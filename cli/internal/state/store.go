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
	Body     string   // spec doc content; skipped if empty
	Actor    string
	Now      time.Time

	// SpecDocAbsPath is where Body is written; empty means the .vector fallback
	// (.vector/specs/<id>/spec.md). SpecDocRel is the repo-relative pointer stored
	// in state.json; empty means the .vector fallback path.
	SpecDocAbsPath string
	SpecDocRel     string
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

	// Resolve where the spec doc lives: the caller's path (repo convention) or
	// the .vector fallback.
	docAbs, docRel := p.SpecDocAbsPath, p.SpecDocRel
	if docAbs == "" {
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
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := os.MkdirAll(s.specDir(id), 0o755); err != nil {
		return nil, fmt.Errorf("create spec dir: %w", err)
	}
	if err := writeSpecFile(s.statePath(id), spec); err != nil {
		return nil, err
	}
	if p.Body != "" {
		if err := os.MkdirAll(filepath.Dir(docAbs), 0o755); err != nil {
			return nil, fmt.Errorf("create spec doc dir: %w", err)
		}
		if err := writeFileAtomic(docAbs, []byte(p.Body)); err != nil {
			return nil, err
		}
	}

	data, err := json.Marshal(SpecCreatedData{Title: spec.Title, Source: "raw", Template: "idea"})
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
	return spec, nil
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
