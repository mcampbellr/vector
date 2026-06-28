package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store is the single writer of Vector's on-disk state, rooted at a repo's
// .vector directory. All write methods serialize through mu.
type Store struct {
	root string // absolute path to the .vector directory
	mu   sync.Mutex
}

// Open returns a Store for the .vector directory under repoRoot, creating the
// base directory layout if needed.
func Open(repoRoot string) (*Store, error) {
	root := filepath.Join(repoRoot, ".vector")
	for _, dir := range []string{filepath.Join(root, "specs"), filepath.Join(root, "local")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return &Store{root: root}, nil
}

func (s *Store) specDir(id string) string   { return filepath.Join(s.root, "specs", id) }
func (s *Store) statePath(id string) string { return filepath.Join(s.specDir(id), "state.json") }
func (s *Store) bodyPath(id string) string  { return filepath.Join(s.specDir(id), "spec.md") }
func (s *Store) activityPath() string       { return filepath.Join(s.root, "local", "activity.jsonl") }

// StatePath returns the absolute path to a spec's state.json (for reporting).
func (s *Store) StatePath(id string) string { return s.statePath(id) }

// CreateSpecParams are the inputs to CreateSpec.
type CreateSpecParams struct {
	Title    string
	ID       string // optional; derived from Title via Slug if empty
	Repo     string
	Priority Priority // defaults to PriorityNormal if empty
	Status   Status   // defaults to StatusDraft if empty
	Source   string   // activity source: "raw" (default) | "sync"
	Body     string   // spec doc content; skipped if empty
	Actor    string
	Now      time.Time

	// SpecDocAbsPath is where Body is written; empty means the .vector fallback
	// (.vector/specs/<id>/spec.md). SpecDocRel is the repo-relative pointer stored
	// in state.json; empty (with an empty AbsPath) means the .vector fallback path.
	SpecDocAbsPath string
	SpecDocRel     string

	// OpenSpec, when set, records the source change (used by `vector sync` to mark
	// a card as derived from an OpenSpec change rather than a /vector:raw draft).
	OpenSpec *OpenSpec

	// NeedsUAT marks the card as awaiting manual UAT. Only honored when Status is
	// review (the invariant: the flag is a refinement of review, nothing else).
	NeedsUAT bool

	// Ticket, when set, seeds the spec's external-tracker link at creation time
	// (e.g. a ref detected in a /vector:raw idea, or sync's conservative scan). It
	// is persisted on state.json and also emits a spec.linked event alongside
	// spec.created. Auto-detected seeds carry Auto:true.
	Ticket *Ticket

	// RelatedTo seeds cause→bug relations at creation time (used by /vector:bug).
	// Each item is normalized and validated; an invalid item fails CreateSpec, so
	// the caller that wants degrade semantics validates/filters before calling.
	// Each persisted relation also emits a spec.related event.
	RelatedTo []RelatedItem
}

// normalizeRelated lowercases the kind/source, trims the ref, and defaults an
// empty source to manual. It does not validate (see validateRelated).
func normalizeRelated(item RelatedItem) RelatedItem {
	item.Kind = RelatedKind(strings.ToLower(strings.TrimSpace(string(item.Kind))))
	item.Ref = strings.TrimSpace(item.Ref)
	item.Source = RelatedSource(strings.ToLower(strings.TrimSpace(string(item.Source))))
	if item.Source == "" {
		item.Source = RelatedManual
	}
	return item
}

// validateRelated checks a normalized relation's structure and, for a spec
// relation, that the referenced spec exists. The caller holds s.mu.
func (s *Store) validateRelated(item RelatedItem) error {
	if !item.Kind.Valid() {
		return fmt.Errorf("invalid relation kind %q: allowed spec,ticket", item.Kind)
	}
	if item.Ref == "" {
		return errors.New("relation requires a non-empty ref")
	}
	if !item.Source.Valid() {
		return fmt.Errorf("invalid relation source %q: allowed blame,manual", item.Source)
	}
	if item.Kind == RelatedSpec {
		if _, err := os.Stat(s.statePath(item.Ref)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("related spec %q does not exist", item.Ref)
			}
			return fmt.Errorf("stat related spec %q: %w", item.Ref, err)
		}
	}
	return nil
}

// containsRelation reports whether items already holds a relation with the same
// kind and ref (idempotency is keyed on {kind,ref}, ignoring source).
func containsRelation(items []RelatedItem, item RelatedItem) bool {
	for _, existing := range items {
		if existing.Kind == item.Kind && existing.Ref == item.Ref {
			return true
		}
	}
	return false
}

// RelateSpec adds one cause→bug relation to a spec: lock → read → validate →
// idempotency on {kind,ref} → atomic write + spec.related event. A duplicate
// relation is a no-op returning (false, nil). The spec's lifecycle status is
// untouched — relating is metadata, not a transition.
func (s *Store) RelateSpec(id string, item RelatedItem, actor string, now time.Time) (bool, error) {
	item = normalizeRelated(item)
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateRelated(item); err != nil {
		return false, err
	}
	spec, err := s.ReadSpec(id)
	if err != nil {
		return false, err
	}
	if containsRelation(spec.RelatedTo, item) {
		return false, nil
	}

	now = now.UTC()
	spec.RelatedTo = append(spec.RelatedTo, item)
	spec.UpdatedAt = now
	if err := writeSpecFile(s.statePath(id), spec); err != nil {
		return false, err
	}
	if err := s.appendRelatedEvent(spec.ID, spec.Repo, item, now, actor); err != nil {
		return false, err
	}
	return true, nil
}

// appendRelatedEvent appends a spec.related event. The caller holds s.mu.
func (s *Store) appendRelatedEvent(specID, repo string, item RelatedItem, now time.Time, actor string) error {
	data, err := json.Marshal(SpecRelatedData{Kind: item.Kind, Ref: item.Ref, Source: item.Source})
	if err != nil {
		return fmt.Errorf("marshal spec.related data: %w", err)
	}
	return s.appendEvent(Event{
		V:      EventVersion,
		TS:     now,
		Type:   EvtSpecRelated,
		SpecID: specID,
		Repo:   repo,
		Actor:  actor,
		Data:   data,
	})
}

// CreateSpec writes a new spec's state.json (status open) and spec.md, and
// appends a spec.created event. It fails if the spec already exists.
func (s *Store) CreateSpec(p CreateSpecParams) (*SpecState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := p.ID
	if id == "" {
		id = Slug(p.Title)
	}
	if id == "" {
		return nil, errors.New("empty spec id: provide --id or a non-empty title")
	}
	if id != Slug(id) {
		return nil, fmt.Errorf("invalid spec id %q: must be kebab-case", id)
	}

	priority := p.Priority
	if priority == "" {
		priority = PriorityNormal
	}
	if !priority.Valid() {
		return nil, fmt.Errorf("invalid priority %q", priority)
	}

	status := p.Status
	if status == "" {
		status = StatusDraft
	}
	if !status.Valid() {
		return nil, fmt.Errorf("invalid status %q", status)
	}

	if _, err := os.Stat(s.statePath(id)); err == nil {
		return nil, fmt.Errorf("spec %q already exists", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat spec %q: %w", id, err)
	}

	// Normalize, validate and de-duplicate seed relations before constructing the
	// spec so an invalid relation fails the create (the CLI degrades by filtering
	// invalid relations before calling, per the --related contract).
	related := make([]RelatedItem, 0, len(p.RelatedTo))
	for _, item := range p.RelatedTo {
		item = normalizeRelated(item)
		if err := s.validateRelated(item); err != nil {
			return nil, err
		}
		if !containsRelation(related, item) {
			related = append(related, item)
		}
	}

	// Resolve where the spec doc lives: the caller's path (repo convention or an
	// OpenSpec change), or the .vector fallback when neither is given.
	docAbs, docRel := p.SpecDocAbsPath, p.SpecDocRel
	if docAbs == "" && docRel == "" {
		docAbs = s.bodyPath(id)
		docRel = filepath.ToSlash(filepath.Join(".vector", "specs", id, "spec.md"))
	}

	now := p.Now.UTC()
	spec := &SpecState{
		SchemaVersion: SchemaVersion,
		ID:            id,
		Title:         strings.TrimSpace(p.Title),
		Status:        status,
		Priority:      priority,
		Repo:          p.Repo,
		SpecDoc:       docRel,
		OpenSpec:      p.OpenSpec,
		NeedsUAT:      p.NeedsUAT && status == StatusReview,
		Ticket:        p.Ticket,
		RelatedTo:     related,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if len(spec.RelatedTo) == 0 {
		spec.RelatedTo = nil // keep omitempty byte-compatibility for relation-less specs
	}
	setStatusTimestamp(spec, status, now)

	if err := os.MkdirAll(s.specDir(id), 0o755); err != nil {
		return nil, fmt.Errorf("create spec dir: %w", err)
	}
	if err := writeSpecFile(s.statePath(id), spec); err != nil {
		return nil, err
	}
	if p.Body != "" && docAbs != "" {
		if err := os.MkdirAll(filepath.Dir(docAbs), 0o755); err != nil {
			return nil, fmt.Errorf("create spec doc dir: %w", err)
		}
		if err := writeFileAtomic(docAbs, []byte(p.Body)); err != nil {
			return nil, err
		}
	}

	source, template := p.Source, ""
	if source == "" {
		if p.OpenSpec != nil {
			source = "sync"
		} else {
			source, template = "raw", "idea"
		}
	}
	data, err := json.Marshal(SpecCreatedData{Title: spec.Title, Source: source, Template: template})
	if err != nil {
		return nil, fmt.Errorf("marshal spec.created data: %w", err)
	}
	if err := s.appendEvent(Event{
		V:      EventVersion,
		TS:     now,
		Type:   EvtSpecCreated,
		SpecID: id,
		Repo:   p.Repo,
		Actor:  p.Actor,
		Data:   data,
	}); err != nil {
		return nil, err
	}
	if spec.Ticket != nil {
		if err := s.appendLinkedEvent(spec, now, p.Actor); err != nil {
			return nil, err
		}
	}
	for _, item := range spec.RelatedTo {
		if err := s.appendRelatedEvent(spec.ID, spec.Repo, item, now, p.Actor); err != nil {
			return nil, err
		}
	}
	return spec, nil
}

// LinkSpec links a spec to an external ticket: lock → read → idempotency &
// precedence → atomic write + spec.linked event. Re-linking the same
// provider+key+url is a no-op returning (false, nil). An auto-detected ticket
// (ticket.Auto) never overwrites a manually-linked one; a manual link always
// wins and may replace an existing link. The spec's lifecycle status is
// untouched — linking is metadata, not a transition.
func (s *Store) LinkSpec(id string, ticket Ticket, actor string, now time.Time) (bool, error) {
	if ticket.Provider == "" || ticket.Key == "" {
		return false, errors.New("link requires a ticket provider and key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	spec, err := s.ReadSpec(id)
	if err != nil {
		return false, err
	}
	if cur := spec.Ticket; cur != nil {
		// Idempotent: identical link is a no-op (no re-emit).
		if cur.Provider == ticket.Provider && cur.Key == ticket.Key && cur.URL == ticket.URL {
			return false, nil
		}
		// Precedence: auto never clobbers a manual link.
		if ticket.Auto && !cur.Auto {
			return false, nil
		}
	}

	now = now.UTC()
	linked := ticket
	spec.Ticket = &linked
	spec.UpdatedAt = now
	if err := writeSpecFile(s.statePath(id), spec); err != nil {
		return false, err
	}
	if err := s.appendLinkedEvent(spec, now, actor); err != nil {
		return false, err
	}
	return true, nil
}

// appendLinkedEvent appends a spec.linked event for spec.Ticket. The caller
// holds s.mu (it runs inside CreateSpec/LinkSpec while the lock is held).
func (s *Store) appendLinkedEvent(spec *SpecState, now time.Time, actor string) error {
	data, err := json.Marshal(SpecLinkedData{
		Provider: spec.Ticket.Provider,
		Key:      spec.Ticket.Key,
		URL:      spec.Ticket.URL,
		Auto:     spec.Ticket.Auto,
	})
	if err != nil {
		return fmt.Errorf("marshal spec.linked data: %w", err)
	}
	return s.appendEvent(Event{
		V:      EventVersion,
		TS:     now,
		Type:   EvtSpecLinked,
		SpecID: spec.ID,
		Repo:   spec.Repo,
		Actor:  actor,
		Data:   data,
	})
}

// ReconcileStatus updates a sync-owned spec's status and OpenSpec artifacts to
// match its source change, appending a status.changed event (trigger "sync"). It
// is a no-op returning (false, nil) when nothing differs.
func (s *Store) ReconcileStatus(id string, status Status, openSpec *OpenSpec, needsUAT bool, actor string, now time.Time) (bool, error) {
	if !status.Valid() {
		return false, fmt.Errorf("invalid status %q", status)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	spec, err := s.ReadSpec(id)
	if err != nil {
		return false, err
	}
	// closed/archived are terminal lifecycle states a human set deliberately after
	// review. A tasks.md-derived status (open/in-progress/review) must never pull a
	// card back out of them, so keep the existing status and only refresh the
	// OpenSpec provenance below. Without this guard, re-running sync --reconcile
	// regresses every closed card to review (all tasks done → review).
	if spec.Status.IsTerminal() {
		status = spec.Status
	}
	// NeedsUAT is a refinement of review; it never persists outside it.
	wantUAT := needsUAT && status == StatusReview
	if spec.Status == status && openSpecEqual(spec.OpenSpec, openSpec) && spec.NeedsUAT == wantUAT {
		return false, nil
	}

	from := spec.Status
	now = now.UTC()
	spec.Status = status
	spec.OpenSpec = openSpec
	spec.NeedsUAT = wantUAT
	spec.UpdatedAt = now
	setStatusTimestamp(spec, status, now)
	if err := writeSpecFile(s.statePath(id), spec); err != nil {
		return false, err
	}

	data, err := json.Marshal(StatusChangedData{From: from, To: status, Trigger: "sync"})
	if err != nil {
		return false, fmt.Errorf("marshal status.changed data: %w", err)
	}
	if err := s.appendEvent(Event{
		V:      EventVersion,
		TS:     now,
		Type:   EvtStatusChanged,
		SpecID: id,
		Repo:   spec.Repo,
		Actor:  actor,
		Data:   data,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// ProposeSpec formalizes a draft spec: it records the OpenSpec change provenance
// and transitions draft → open, appending spec.proposed + status.changed events.
// It does NOT stamp StartedAt — open means the change exists but work has not
// started; StartedAt is set later at /vector:apply (in-progress). Errors if the
// spec is not in draft.
func (s *Store) ProposeSpec(id string, openSpec *OpenSpec, actor string, now time.Time) (*SpecState, error) {
	if openSpec == nil || openSpec.Change == "" {
		return nil, errors.New("propose requires an OpenSpec change reference")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	spec, err := s.ReadSpec(id)
	if err != nil {
		return nil, err
	}
	if spec.Status != StatusDraft {
		return nil, fmt.Errorf("spec %q is %q, not draft (only a draft can be proposed)", id, spec.Status)
	}

	now = now.UTC()
	spec.Status = StatusOpen
	spec.OpenSpec = openSpec
	spec.UpdatedAt = now
	if err := writeSpecFile(s.statePath(id), spec); err != nil {
		return nil, err
	}

	proposed, err := json.Marshal(ProposedData{Change: openSpec.Change, Artifacts: openSpec.Artifacts})
	if err != nil {
		return nil, fmt.Errorf("marshal spec.proposed data: %w", err)
	}
	if err := s.appendEvent(Event{V: EventVersion, TS: now, Type: EvtSpecProposed, SpecID: id, Repo: spec.Repo, Actor: actor, Data: proposed}); err != nil {
		return nil, err
	}
	changed, err := json.Marshal(StatusChangedData{From: StatusDraft, To: StatusOpen, Trigger: "command"})
	if err != nil {
		return nil, fmt.Errorf("marshal status.changed data: %w", err)
	}
	if err := s.appendEvent(Event{V: EventVersion, TS: now, Type: EvtStatusChanged, SpecID: id, Repo: spec.Repo, Actor: actor, Data: changed}); err != nil {
		return nil, err
	}
	return spec, nil
}

// setStatusTimestamp stamps the lifecycle timestamp implied by a status, without
// overwriting one already set.
func setStatusTimestamp(spec *SpecState, status Status, now time.Time) {
	switch status {
	case StatusInProgress:
		if spec.StartedAt == nil {
			spec.StartedAt = &now
		}
	case StatusReview:
		if spec.StartedAt == nil {
			spec.StartedAt = &now
		}
		if spec.ReviewAt == nil {
			spec.ReviewAt = &now
		}
	case StatusClosed:
		if spec.ClosedAt == nil {
			spec.ClosedAt = &now
		}
	case StatusArchived:
		if spec.ArchivedAt == nil {
			spec.ArchivedAt = &now
		}
	}
}

func openSpecEqual(a, b *OpenSpec) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Change == b.Change && a.Artifacts == b.Artifacts
}

// ReadSpec loads a spec's state.json.
func (s *Store) ReadSpec(id string) (*SpecState, error) {
	b, err := os.ReadFile(s.statePath(id))
	if err != nil {
		return nil, fmt.Errorf("read spec %q: %w", id, err)
	}
	var spec SpecState
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("parse spec %q: %w", id, err)
	}
	return &spec, nil
}

// ListSpecs returns every spec under .vector/specs, sorted by id.
func (s *Store) ListSpecs() ([]*SpecState, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "specs"))
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}
	specs := make([]*SpecState, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		spec, err := s.ReadSpec(e.Name())
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// ReadEvents returns every event in the local activity log, in file order.
// A missing log is not an error: it returns an empty slice. Malformed lines are
// skipped (the log is append-only and crash-safe, but tolerate partial tails).
func (s *Store) ReadEvents() ([]Event, error) {
	b, err := os.ReadFile(s.activityPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read activity log: %w", err)
	}
	lines := strings.Split(string(b), "\n")
	events := make([]Event, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // tolerate a torn final line
		}
		events = append(events, e)
	}
	return events, nil
}

// AppendEvent appends an event to the local activity log (serialized).
func (s *Store) AppendEvent(e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendEvent(e)
}

// appendEvent assumes the caller holds s.mu.
func (s *Store) appendEvent(e Event) error {
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	f, err := os.OpenFile(s.activityPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open activity log: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write activity log: %w", err)
	}
	return nil
}

func writeSpecFile(path string, spec *SpecState) error {
	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal spec state: %w", err)
	}
	return writeFileAtomic(path, append(b, '\n'))
}

// writeFileAtomic writes data to path via a temp file in the same directory and
// an atomic rename, so readers never observe a partial file.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file into place: %w", err)
	}
	return nil
}
