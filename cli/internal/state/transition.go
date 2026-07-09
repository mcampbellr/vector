package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// allowedTransitions encodes the LOCKED spec state machine
// (docs/domain-contract.md §1). draft→open is intentionally excluded here: it is
// owned by ProposeSpec, which also records the OpenSpec change. Everything else
// goes through applyTransition.
var allowedTransitions = map[Status]map[Status]bool{
	StatusDraft:          {StatusClosed: true},
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

// transitionOpts configure a single state-machine move.
type transitionOpts struct {
	to        Status
	trigger   string     // status.changed trigger: command | apply | hook
	reason    string     // legacy free-text; used when entering needs-attention and att is nil
	att       *Attention // structured needs-attention overlay; when set it wins over reason
	source    string     // needs-attention source: hook | command
	extraType EventType  // optional domain event emitted alongside status.changed
	extraData any        // payload for extraType
	actor     string
	now       time.Time
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
	if from == opts.to {
		return nil, fmt.Errorf("spec %q is already %q", id, opts.to)
	}
	if !CanTransition(from, opts.to) {
		return nil, fmt.Errorf("illegal transition %q → %q for spec %q", from, opts.to, id)
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

	if err := writeSpecFile(s.statePath(id), spec); err != nil {
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

// ApplySpec starts work on an open spec: open → in-progress, stamping StartedAt
// and emitting spec.applied + status.changed (trigger apply). change is the
// OpenSpec change being implemented (may be empty for native specs).
func (s *Store) ApplySpec(id, change, actor string, now time.Time) (*SpecState, error) {
	return s.applyTransition(id, transitionOpts{
		to:        StatusInProgress,
		trigger:   "apply",
		extraType: EvtSpecApplied,
		extraData: AppliedData{Change: change},
		actor:     actor,
		now:       now,
	})
}

// CloseSpec transitions a spec to closed (from draft, in-progress or review),
// emitting spec.closed + status.changed.
func (s *Store) CloseSpec(id, actor string, now time.Time) (*SpecState, error) {
	return s.applyTransition(id, transitionOpts{
		to:        StatusClosed,
		trigger:   "command",
		extraType: EvtSpecClosed,
		actor:     actor,
		now:       now,
	})
}

// ArchiveSpec transitions a closed spec to archived, emitting spec.archived +
// status.changed.
func (s *Store) ArchiveSpec(id, actor string, now time.Time) (*SpecState, error) {
	return s.applyTransition(id, transitionOpts{
		to:        StatusArchived,
		trigger:   "command",
		extraType: EvtSpecArchived,
		actor:     actor,
		now:       now,
	})
}

// SetStatus is the generic /vector:status transition (trigger command). It
// covers the moves without a dedicated command (review↔in-progress, resolving
// needs-attention). reason is required only when entering needs-attention. Use
// ApplySpec/CloseSpec/ArchiveSpec/ProposeSpec for moves that carry extra
// semantics; SetStatus rejects those to keep one writer per transition.
func (s *Store) SetStatus(id string, to Status, reason, actor string, now time.Time) (*SpecState, error) {
	switch to {
	case StatusOpen:
		return nil, errors.New("use `vector spec propose` to open a draft")
	case StatusClosed:
		return nil, errors.New("use `vector spec close` to close a spec")
	case StatusArchived:
		return nil, errors.New("use `vector spec archive` to archive a spec")
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
// needs-attention > review > open, then by priority, then most-recently-updated.
// Returns nil when nothing is actionable (only draft/closed/archived remain).
func SelectNext(specs []*SpecState) *SpecState {
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
		ri, rj := selectionRank[candidates[i].Status], selectionRank[candidates[j].Status]
		if ri != rj {
			return ri < rj
		}
		pi, pj := priorityRank(candidates[i].Priority), priorityRank(candidates[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
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
