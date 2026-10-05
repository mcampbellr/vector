package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// allowedTransitions encodes the LOCKED spec state machine
// (docs/domain-contract.md §1). Legacy draft is intentionally absent: it is
// migrated to open by `vector update` or consumed compatibly by ApplySpec.
var allowedTransitions = map[Status]map[Status]bool{
	StatusOpen:           {StatusInProgress: true, StatusClosed: true},
	StatusInProgress:     {StatusReview: true, StatusNeedsAttention: true, StatusClosed: true},
	StatusReview:         {StatusInProgress: true, StatusNeedsAttention: true, StatusClosed: true},
	StatusNeedsAttention: {StatusInProgress: true, StatusReview: true},
	StatusClosed:         {StatusArchived: true},
	StatusArchived:       {},
}

// CanTransition reports whether from→to is a legal state-machine move.
func CanTransition(from, to Status) bool {
	return allowedTransitions[from][to]
}

// lifecycleOrder is the canonical display order of statuses, used to list legal
// targets deterministically (the transition table is a map).
var lifecycleOrder = []Status{
	StatusOpen, StatusInProgress, StatusNeedsAttention, StatusReview, StatusClosed, StatusArchived,
}

// LegalTargets returns the statuses reachable from `from` in one legal move, in
// lifecycle order, derived from the transition table. Empty for a final status.
func LegalTargets(from Status) []Status {
	targets := make([]Status, 0, len(allowedTransitions[from]))
	for _, candidate := range lifecycleOrder {
		if allowedTransitions[from][candidate] {
			targets = append(targets, candidate)
		}
	}
	return targets
}

// TransitionCommand is the CLI invocation that performs from→to for spec id —
// the dedicated verb when one owns the move (apply/close/archive), else the
// generic `vector spec status`.
func TransitionCommand(id string, from, to Status) string {
	switch {
	case to == StatusClosed:
		return fmt.Sprintf("vector spec close %s", id)
	case to == StatusArchived:
		return fmt.Sprintf("vector spec archive %s", id)
	case from == StatusOpen && to == StatusInProgress:
		return fmt.Sprintf("vector spec apply %s", id)
	default:
		return fmt.Sprintf("vector spec status %s %s", id, to)
	}
}

// IllegalTransitionError reports a move the state machine forbids, carrying the
// legal targets from the current status so the message tells the caller what it
// can do instead. It matches ErrConflict (well-formed request, clashes with the
// spec's current state).
type IllegalTransitionError struct {
	ID    string
	From  Status
	To    Status
	Legal []Status
}

func (e *IllegalTransitionError) Error() string {
	if len(e.Legal) == 0 {
		return fmt.Sprintf("illegal transition %q → %q for spec %q: %q is final (no legal transitions)", e.From, e.To, e.ID, e.From)
	}
	options := make([]string, 0, len(e.Legal))
	for _, target := range e.Legal {
		options = append(options, fmt.Sprintf("%s (`%s`)", target, TransitionCommand(e.ID, e.From, target)))
	}
	return fmt.Sprintf("illegal transition %q → %q for spec %q; legal next statuses from %q: %s",
		e.From, e.To, e.ID, e.From, strings.Join(options, ", "))
}

// Is lets callers classify the error with errors.Is(err, ErrConflict).
func (e *IllegalTransitionError) Is(target error) bool { return target == ErrConflict }

// checkTransition validates from→to for spec id: a same-status move is an
// "already" conflict, a forbidden one an *IllegalTransitionError.
func checkTransition(id string, from, to Status) error {
	if from == to {
		return conflictf("spec %q is already %q", id, to)
	}
	if !CanTransition(from, to) {
		return &IllegalTransitionError{ID: id, From: from, To: to, Legal: LegalTargets(from)}
	}
	return nil
}

// transitionOpts configure a single state-machine move.
type transitionOpts struct {
	to        Status
	trigger   string     // status.changed trigger: command | apply | hook
	reason    string     // legacy free-text; used when entering needs-attention and att is nil
	att       *Attention // structured needs-attention overlay; when set it wins over reason
	source    string     // needs-attention source: hook | command
	extraType EventType  // optional domain event emitted alongside status.changed
	extraData any        // payload for extraType
	// resolution/resolutionNote: set on entering closed (resolution defaults to
	// done); on entering archived they override the stored values only when
	// resolution is non-empty.
	resolution     Resolution
	resolutionNote string
	actor          string
	now            time.Time
}

// attentionSummaryMax bounds the one-liner summary derived from a legacy --reason
// during on-write migration (the card renders it; the drawer shows the full detail).
const attentionSummaryMax = 80

// truncateAttentionSummary returns reason clipped to attentionSummaryMax runes,
// appending an ellipsis only when it actually cut the string.
func truncateAttentionSummary(reason string) string {
	runes := []rune(reason)
	if len(runes) <= attentionSummaryMax {
		return reason
	}
	return string(runes[:attentionSummaryMax]) + "…"
}

// buildAttention constructs the needs-attention overlay from a transition.
// Structured path (opts.att != nil): the caller supplies Category/Summary/Detail;
// Category defaults to "other", Detail falls back to Summary, and Reason is fixed
// equal to Summary. Legacy path (opts.reason only): migrate on write —
// Category="other", Summary=truncated reason, Detail=Reason=reason.
func buildAttention(opts transitionOpts, now time.Time) *Attention {
	source := opts.source
	if source == "" {
		source = "command"
	}
	if opts.att != nil {
		category := opts.att.Category
		if category == "" {
			category = AttentionOther
		}
		detail := opts.att.Detail
		if detail == "" {
			detail = opts.att.Summary
		}
		return &Attention{
			Reason:   opts.att.Summary,
			Category: category,
			Summary:  opts.att.Summary,
			Detail:   detail,
			Since:    now,
			Source:   source,
		}
	}
	return &Attention{
		Reason:   opts.reason,
		Category: AttentionOther,
		Summary:  truncateAttentionSummary(opts.reason),
		Detail:   opts.reason,
		Since:    now,
		Source:   source,
	}
}

// applyTransition is the shared write primitive for state-machine moves: it
// validates from→to, stamps the lifecycle timestamp, maintains the
// needs-attention flag, persists, and appends status.changed plus an optional
// domain event. It is a no-op error if the move is illegal.
func (s *Store) applyTransition(id string, opts transitionOpts) (*SpecState, error) {
	if !opts.to.Valid() {
		return nil, fmt.Errorf("invalid status %q", opts.to)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	spec, err := s.ReadSpec(id)
	if err != nil {
		return nil, err
	}
	from := spec.Status
	if err := checkTransition(id, from, opts.to); err != nil {
		return nil, err
	}
	if opts.resolution != "" && !opts.resolution.Valid() {
		return nil, invalidInputf("invalid resolution %q: allowed %s", opts.resolution, resolutionList())
	}
	if opts.to == StatusNeedsAttention {
		if opts.att != nil {
			if opts.att.Summary == "" {
				return nil, errors.New("entering needs-attention requires a summary")
			}
			if opts.att.Category != "" && !opts.att.Category.Valid() {
				return nil, fmt.Errorf("invalid attention category %q", opts.att.Category)
			}
		} else if opts.reason == "" {
			return nil, errors.New("entering needs-attention requires a reason")
		}
	}

	now := opts.now.UTC()
	spec.Status = opts.to
	spec.UpdatedAt = now
	setStatusTimestamp(spec, opts.to, now)

	// Maintain the attention overlay: set on entry, clear on resolution.
	switch {
	case opts.to == StatusNeedsAttention:
		spec.Flag = buildAttention(opts, now)
	case from == StatusNeedsAttention:
		spec.Flag = nil
	}
	// Focus means "work on this first"; a closed/archived spec has no work left, so
	// the marker is dropped with the terminal move rather than lingering as noise.
	if opts.to.IsTerminal() {
		spec.Focus = false
		spec.FocusedAt = nil
	}
	// Resolution: always stamped on close (default done); an archive keeps the
	// close-time resolution unless the caller overrides it; leaving the terminal
	// states (not a legal move today) drops it.
	switch {
	case opts.to == StatusClosed:
		spec.Resolution = opts.resolution
		if spec.Resolution == "" {
			spec.Resolution = ResolutionDone
		}
		spec.ResolutionNote = strings.TrimSpace(opts.resolutionNote)
	case opts.to == StatusArchived && opts.resolution != "":
		spec.Resolution = opts.resolution
		spec.ResolutionNote = strings.TrimSpace(opts.resolutionNote)
	case !opts.to.IsTerminal():
		spec.Resolution = ""
		spec.ResolutionNote = ""
	}

	if err := s.writeSpecState(spec); err != nil {
		return nil, err
	}

	eventReason := opts.reason
	if opts.att != nil {
		eventReason = opts.att.Summary
	}
	changed, err := json.Marshal(StatusChangedData{From: from, To: opts.to, Trigger: opts.trigger, Reason: eventReason})
	if err != nil {
		return nil, fmt.Errorf("marshal status.changed data: %w", err)
	}
	if err := s.appendEvent(Event{V: EventVersion, TS: now, Type: EvtStatusChanged, SpecID: id, Repo: spec.Repo, Actor: opts.actor, Data: changed}); err != nil {
		return nil, err
	}
	if opts.extraType != "" {
		var data json.RawMessage
		if opts.extraData != nil {
			b, mErr := json.Marshal(opts.extraData)
			if mErr != nil {
				return nil, fmt.Errorf("marshal %s data: %w", opts.extraType, mErr)
			}
			data = b
		}
		if err := s.appendEvent(Event{V: EventVersion, TS: now, Type: opts.extraType, SpecID: id, Repo: spec.Repo, Actor: opts.actor, Data: data}); err != nil {
			return nil, err
		}
	}
	return spec, nil
}

// ApplySpec starts work on an open spec. It also consumes a pre-v0.8 legacy
// draft atomically: records compatibility OpenSpec provenance, opens it, and then
// starts work. This keeps stale cards recoverable without exposing a draft lane
// or a separate formalization command to users.
func (s *Store) ApplySpec(id, change, actor string, now time.Time) (*SpecState, error) {
	if spec, err := s.ReadSpec(id); err == nil && spec.Status == StatusLegacyDraft {
		if change == "" {
			change = id
		}
		if _, err := s.ProposeSpec(id, &OpenSpec{Change: change}, actor, now); err != nil {
			return nil, err
		}
	}
	return s.applyTransition(id, transitionOpts{
		to:        StatusInProgress,
		trigger:   "apply",
		extraType: EvtSpecApplied,
		extraData: AppliedData{Change: change},
		actor:     actor,
		now:       now,
	})
}

// CloseSpec transitions a spec to closed (from open, in-progress or review)
// with resolution done, emitting spec.closed + status.changed.
func (s *Store) CloseSpec(id, actor string, now time.Time) (*SpecState, error) {
	return s.CloseSpecWith(id, ResolutionDone, "", actor, now)
}

// CloseSpecWith closes a spec recording why: resolution ("" = done) and an
// optional note are persisted on the spec and carried by the spec.closed event.
func (s *Store) CloseSpecWith(id string, resolution Resolution, note, actor string, now time.Time) (*SpecState, error) {
	if resolution == "" {
		resolution = ResolutionDone
	}
	return s.applyTransition(id, transitionOpts{
		to:             StatusClosed,
		trigger:        "command",
		extraType:      EvtSpecClosed,
		extraData:      ResolutionData{Resolution: resolution, Note: strings.TrimSpace(note)},
		resolution:     resolution,
		resolutionNote: note,
		actor:          actor,
		now:            now,
	})
}

// ArchiveSpec transitions a closed spec to archived, keeping its close-time
// resolution, emitting spec.archived + status.changed.
func (s *Store) ArchiveSpec(id, actor string, now time.Time) (*SpecState, error) {
	return s.ArchiveSpecWith(id, "", "", actor, now)
}

// ArchiveSpecWith archives a closed spec. A non-empty resolution (and its note)
// overrides the one recorded at close — the way to resolve a legacy closed spec
// that predates resolutions; "" keeps whatever the spec already carries.
func (s *Store) ArchiveSpecWith(id string, resolution Resolution, note, actor string, now time.Time) (*SpecState, error) {
	if resolution == "" && strings.TrimSpace(note) != "" {
		return nil, invalidInputf("a resolution note requires a resolution")
	}
	opts := transitionOpts{
		to:             StatusArchived,
		trigger:        "command",
		extraType:      EvtSpecArchived,
		resolution:     resolution,
		resolutionNote: note,
		actor:          actor,
		now:            now,
	}
	if resolution != "" {
		opts.extraData = ResolutionData{Resolution: resolution, Note: strings.TrimSpace(note)}
	}
	return s.applyTransition(id, opts)
}

// resolutionList renders the known resolutions for error messages.
func resolutionList() string {
	names := make([]string, 0, len(Resolutions))
	for _, resolution := range Resolutions {
		names = append(names, string(resolution))
	}
	return strings.Join(names, "|")
}

// SetStatus is the generic /vector:status transition (trigger command). It
// covers the moves without a dedicated command (review↔in-progress, resolving
// needs-attention). reason is required only when entering needs-attention. Use
// ApplySpec/CloseSpec/ArchiveSpec/ProposeSpec for moves that carry extra
// semantics; SetStatus rejects those to keep one writer per transition.
func (s *Store) SetStatus(id string, to Status, reason, actor string, now time.Time) (*SpecState, error) {
	switch to {
	case StatusOpen, StatusClosed, StatusArchived:
		// These targets have a dedicated writer (or, for open, no inbound move at
		// all). Validate against the spec's real current status first, so the
		// caller gets "already X" / the legal targets instead of a blind message.
		spec, err := s.ReadSpec(id)
		if err != nil {
			return nil, err
		}
		if err := checkTransition(id, spec.Status, to); err != nil {
			return nil, err
		}
		return nil, invalidInputf("use `%s` to move spec %q to %s", TransitionCommand(id, spec.Status, to), id, to)
	}
	return s.applyTransition(id, transitionOpts{
		to:      to,
		trigger: "command",
		reason:  reason,
		source:  "command",
		actor:   actor,
		now:     now,
	})
}

// SetStatusAttention is the structured needs-attention transition: instead of a
// single free-text reason it carries a categorized, summarized, markdown-detailed
// overlay. Only needs-attention is a valid target (the other moves have no
// structured payload); it delegates to the same applyTransition as SetStatus, so
// the state machine and Flag lifecycle are identical. att.Summary is required.
func (s *Store) SetStatusAttention(id string, to Status, att Attention, actor string, now time.Time) (*SpecState, error) {
	if to != StatusNeedsAttention {
		return nil, errors.New("structured attention is only valid for the needs-attention transition")
	}
	return s.applyTransition(id, transitionOpts{
		to:      to,
		trigger: "command",
		att:     &att,
		source:  "command",
		actor:   actor,
		now:     now,
	})
}

// selectionRank orders the work queue: continue what's started, then unblock,
// then close out, then pick up fresh work (docs/apply-design.md §3).
var selectionRank = map[Status]int{
	StatusInProgress:     0,
	StatusNeedsAttention: 1,
	StatusReview:         2,
	StatusOpen:           3,
}

// SelectNext returns the recommended next work-item across specs, using Vector's
// tracked status + priority signal (the plus over OpenSpec): in-progress >
// needs-attention > review > open; within each status tier specs with effective
// focus come first (the spec's own focus, or inherited from a focused epic — see
// EpicIndex.EffectiveFocus), then specs of lower-order epics (epic order; specs
// with no epic or an unordered epic rank after every ordered epic), then
// priority, then most-recently-updated. Neither focus nor epic order lifts a
// spec across status tiers, so finishing started work still wins over a focused
// open spec. epics may be nil (no epic focus/order signal).
// Returns nil when nothing is actionable (only closed/archived remain).
func SelectNext(specs []*SpecState, epics []*Epic) *SpecState {
	index := NewEpicIndex(epics)
	candidates := make([]*SpecState, 0, len(specs))
	for _, spec := range specs {
		if _, ok := selectionRank[spec.Status]; ok {
			candidates = append(candidates, spec)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		ri, rj := selectionRank[left.Status], selectionRank[right.Status]
		if ri != rj {
			return ri < rj
		}
		fi, fj := index.EffectiveFocus(left), index.EffectiveFocus(right)
		if fi != fj {
			return fi
		}
		ei, ej := index.OrderRank(left.Epic), index.OrderRank(right.Epic)
		if ei != ej {
			return ei < ej
		}
		pi, pj := priorityRank(left.Priority), priorityRank(right.Priority)
		if pi != pj {
			return pi < pj
		}
		return left.UpdatedAt.After(right.UpdatedAt)
	})
	return candidates[0]
}

func priorityRank(p Priority) int {
	switch p {
	case PriorityUrgent:
		return 0
	case PriorityHigh:
		return 1
	case PriorityNormal:
		return 2
	case PriorityLow:
		return 3
	default:
		return 4
	}
}
