package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SummarySchemaVersion guards the on-disk format of the persisted per-spec
// summaries.
const SummarySchemaVersion = 1

// SpecSummary is one spec's persisted "what was done" summary, produced by the
// vector-summary-writer agent after a domain transition. It lives in
// .vector/local/summaries.json (personal, gitignored), keyed by spec id. The
// prose is the agent's; the structural fields (Action, GeneratedAt) are stamped
// by the binary. CLI-owns-writes: the binary never calls an LLM.
type SpecSummary struct {
	SchemaVersion int       `json:"schemaVersion"`
	ID            string    `json:"id"`
	Summary       string    `json:"summary"`
	Action        string    `json:"action"`
	GeneratedAt   time.Time `json:"generatedAt"`
}

func (s *Store) summariesPath() string {
	return filepath.Join(s.root, "local", "summaries.json")
}

// ReadSummaries loads the whole per-spec summary map. A missing file is not an
// error: it returns an empty map so callers can range over it without
// special-casing the first run.
func (s *Store) ReadSummaries() (map[string]SpecSummary, error) {
	b, err := os.ReadFile(s.summariesPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]SpecSummary{}, nil
		}
		return nil, fmt.Errorf("read summaries: %w", err)
	}
	var m map[string]SpecSummary
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse summaries: %w", err)
	}
	if m == nil {
		m = map[string]SpecSummary{}
	}
	return m, nil
}

// ReadSummary returns the summary for one spec, or nil when none is persisted.
// A missing file is not an error (nil, nil).
func (s *Store) ReadSummary(id string) (*SpecSummary, error) {
	all, err := s.ReadSummaries()
	if err != nil {
		return nil, err
	}
	sum, ok := all[id]
	if !ok {
		return nil, nil
	}
	return &sum, nil
}

// WriteSummary upserts one spec's summary (last-writer-wins per id), stamping the
// schema version and a UTC GeneratedAt. Writing is serialized through the store
// mutex and atomic, like the rest of the store, so readers never observe a
// partial file.
func (s *Store) WriteSummary(id, summary, action string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.ReadSummaries()
	if err != nil {
		return err
	}
	all[id] = SpecSummary{
		SchemaVersion: SummarySchemaVersion,
		ID:            id,
		Summary:       summary,
		Action:        action,
		GeneratedAt:   now.UTC(),
	}
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summaries: %w", err)
	}
	return writeFileAtomic(s.summariesPath(), append(b, '\n'))
}
