package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StandupSchemaVersion guards the on-disk format of the persisted standup digest.
const StandupSchemaVersion = 1

// StandupDigest is the persisted "last standup" record at
// .vector/local/standup.json (personal, gitignored). It is both the unit
// WriteStandup persists and the exact body served at GET /api/standup. The
// natural-language fields (Global, per-spec Summary) are produced by the
// vector-standup-writer agent; the structural fields come from the projection.
type StandupDigest struct {
	SchemaVersion int                 `json:"schemaVersion"`
	GeneratedAt   time.Time           `json:"generatedAt"`
	Since         time.Time           `json:"since"`
	MarkerAt      time.Time           `json:"markerAt"`
	Global        string              `json:"global"`
	PerSpec       []StandupSpecDigest `json:"perSpec"`
	Totals        StandupTotals       `json:"totals"`
}

// StandupSpecDigest is one spec's line in the persisted digest.
type StandupSpecDigest struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Summary     string `json:"summary"`
	ChangeCount int    `json:"changeCount"`
}

// StandupTotals are the period-wide counters baked into the digest.
type StandupTotals struct {
	Specs    int            `json:"specs"`
	Changes  int            `json:"changes"`
	ByStatus map[string]int `json:"byStatus"`
}

func (s *Store) standupPath() string { return filepath.Join(s.root, "local", "standup.json") }

// ReadStandup loads the persisted standup digest. A missing file is not an error:
// it returns a zero-value digest (MarkerAt is the zero time, so the next standup
// window covers all history) so callers can resolve "since" without special-casing.
func (s *Store) ReadStandup() (*StandupDigest, error) {
	b, err := os.ReadFile(s.standupPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &StandupDigest{}, nil
		}
		return nil, fmt.Errorf("read standup digest: %w", err)
	}
	var d StandupDigest
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("parse standup digest: %w", err)
	}
	return &d, nil
}

// WriteStandup persists the digest and advances the marker to markerAt (the
// window boundary for the next standup). Writing is serialized through the store
// mutex and atomic, like the rest of the store. The activity log retains every
// event, so advancing the marker never destroys history.
func (s *Store) WriteStandup(digest StandupDigest, markerAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	digest.SchemaVersion = StandupSchemaVersion
	digest.MarkerAt = markerAt.UTC()
	b, err := json.MarshalIndent(digest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal standup digest: %w", err)
	}
	return writeFileAtomic(s.standupPath(), append(b, '\n'))
}

// WorkLog appends a work.logged event for an existing spec. It enriches the
// activity trace without touching state.json (the digest is derived, never
// canonical). It errors if the spec does not exist.
func (s *Store) WorkLog(id string, data WorkLoggedData, actor string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	spec, err := s.ReadSpec(id)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal work.logged data: %w", err)
	}
	return s.appendEvent(Event{
		V:      EventVersion,
		TS:     now.UTC(),
		Type:   EvtWorkLogged,
		SpecID: id,
		Repo:   spec.Repo,
		Actor:  actor,
		Data:   payload,
	})
}
