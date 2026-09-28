package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EpicSchemaVersion guards migrations of the on-disk Epic format.
const EpicSchemaVersion = 1

// EpicColor is a token from the small fixed palette the board renders epic chips
// with. Persisting a token (not a hex value) keeps the file theme-independent: the
// web panel maps each token to a light and a dark value.
type EpicColor string

const (
	EpicColorSlate  EpicColor = "slate"
	EpicColorBlue   EpicColor = "blue"
	EpicColorTeal   EpicColor = "teal"
	EpicColorGreen  EpicColor = "green"
	EpicColorAmber  EpicColor = "amber"
	EpicColorOrange EpicColor = "orange"
	EpicColorRed    EpicColor = "red"
	EpicColorPink   EpicColor = "pink"
	EpicColorViolet EpicColor = "violet"
)

// EpicColors lists the palette in display order (used for validation messages
// and CLI help).
var EpicColors = []EpicColor{
	EpicColorSlate, EpicColorBlue, EpicColorTeal, EpicColorGreen, EpicColorAmber,
	EpicColorOrange, EpicColorRed, EpicColorPink, EpicColorViolet,
}

// Valid reports whether c is a palette token. The empty color is valid at the
// call sites that treat it as "no color" (the board falls back to neutral).
func (c EpicColor) Valid() bool {
	for _, known := range EpicColors {
		if c == known {
			return true
		}
	}
	return false
}

// epicColorList renders the palette for error messages ("slate,blue,…").
func epicColorList() string {
	names := make([]string, 0, len(EpicColors))
	for _, color := range EpicColors {
		names = append(names, string(color))
	}
	return strings.Join(names, ",")
}

// Epic groups specs under a larger initiative (e.g. "App Mobile"). It is a
// first-class entity persisted at .vector/epics/<id>.json (committed, one file per
// epic like specs are sharded, so edits stay merge-local). Specs point to an epic
// through SpecState.Epic; the epic itself stores no membership list, so there is a
// single source of truth for which specs belong to it.
type Epic struct {
	SchemaVersion int       `json:"schemaVersion"`
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description,omitempty"`
	Color         EpicColor `json:"color,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// CreateEpicParams are the inputs to CreateEpic.
type CreateEpicParams struct {
	Title       string
	ID          string // optional; derived from Title via Slug if empty
	Description string
	Color       EpicColor // optional; must be a palette token when set
	Actor       string
	Now         time.Time
}

// UpdateEpicParams carries the fields to change; a nil pointer leaves the field
// untouched. An empty Description or Color clears it; an empty Title is refused.
type UpdateEpicParams struct {
	Title       *string
	Description *string
	Color       *EpicColor
}

// EpicCounts is the membership roll-up of one epic: how many specs point to it,
// how many are done (closed + archived) and the breakdown by status.
type EpicCounts struct {
	Total    int
	Done     int
	ByStatus map[Status]int
}

func (s *Store) epicsDir() string          { return filepath.Join(s.root, "epics") }
func (s *Store) epicPath(id string) string { return filepath.Join(s.epicsDir(), id+".json") }

// validateEpicID rejects anything that is not a non-empty kebab-case slug. The id
// becomes a file name, so this is also the path-traversal guard.
func validateEpicID(id string) error {
	if id == "" {
		return invalidInputf("empty epic id")
	}
	if id != Slug(id) {
		return invalidInputf("invalid epic id %q: must be kebab-case", id)
	}
	return nil
}

func validateEpicColor(color EpicColor) error {
	if color != "" && !color.Valid() {
		return invalidInputf("invalid epic color %q: allowed %s", color, epicColorList())
	}
	return nil
}

// CreateEpic writes a new epic file and appends an epic.created event. The id is
// derived from the title when not given; it fails (ErrConflict) if the epic
// already exists.
func (s *Store) CreateEpic(p CreateEpicParams) (*Epic, error) {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		return nil, invalidInputf("epic title is required")
	}
	id := strings.TrimSpace(p.ID)
	if id == "" {
		id = Slug(title)
	}
	if err := validateEpicID(id); err != nil {
		return nil, err
	}
	if err := validateEpicColor(p.Color); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.epicPath(id)); err == nil {
		return nil, conflictf("epic %q already exists", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat epic %q: %w", id, err)
	}

	now := p.Now.UTC()
	epic := &Epic{
		SchemaVersion: EpicSchemaVersion,
		ID:            id,
		Title:         title,
		Description:   strings.TrimSpace(p.Description),
		Color:         p.Color,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := os.MkdirAll(s.epicsDir(), 0o755); err != nil {
		return nil, fmt.Errorf("create epics dir: %w", err)
	}
	if err := writeEpicFile(s.epicPath(id), epic); err != nil {
		return nil, err
	}
	if err := s.appendEpicEvent(EvtEpicCreated, epic, now, p.Actor); err != nil {
		return nil, err
	}
	return epic, nil
}

// GetEpic loads one epic. A missing epic is a wrapped fs.ErrNotExist.
func (s *Store) GetEpic(id string) (*Epic, error) {
	if err := validateEpicID(id); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.epicPath(id))
	if err != nil {
		return nil, fmt.Errorf("read epic %q: %w", id, err)
	}
	var epic Epic
	if err := json.Unmarshal(b, &epic); err != nil {
		return nil, fmt.Errorf("parse epic %q: %w", id, err)
	}
	return &epic, nil
}

// ListEpics returns every epic under .vector/epics, sorted by id. A repo that has
// never created an epic has no epics dir; that is an empty list, not an error.
func (s *Store) ListEpics() ([]*Epic, error) {
	entries, err := os.ReadDir(s.epicsDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []*Epic{}, nil
		}
		return nil, fmt.Errorf("list epics: %w", err)
	}
	epics := make([]*Epic, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
			continue // skip dirs and atomic-write temp files
		}
		epic, err := s.GetEpic(strings.TrimSuffix(name, ".json"))
		if err != nil {
			return nil, err
		}
		epics = append(epics, epic)
	}
	sort.Slice(epics, func(i, j int) bool { return epics[i].ID < epics[j].ID })
	return epics, nil
}

// UpdateEpic changes an epic's title/description/color: lock → read → apply the
// non-nil fields → atomic write + epic.updated event. When nothing differs it is a
// no-op returning (epic, false, nil).
func (s *Store) UpdateEpic(id string, p UpdateEpicParams, actor string, now time.Time) (*Epic, bool, error) {
	if p.Title != nil && strings.TrimSpace(*p.Title) == "" {
		return nil, false, invalidInputf("epic title cannot be empty")
	}
	if p.Color != nil {
		if err := validateEpicColor(*p.Color); err != nil {
			return nil, false, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	epic, err := s.GetEpic(id)
	if err != nil {
		return nil, false, err
	}
	updated := *epic
	if p.Title != nil {
		updated.Title = strings.TrimSpace(*p.Title)
	}
	if p.Description != nil {
		updated.Description = strings.TrimSpace(*p.Description)
	}
	if p.Color != nil {
		updated.Color = *p.Color
	}
	if updated == *epic {
		return epic, false, nil
	}

	now = now.UTC()
	updated.UpdatedAt = now
	if err := writeEpicFile(s.epicPath(id), &updated); err != nil {
		return nil, false, err
	}
	if err := s.appendEpicEvent(EvtEpicUpdated, &updated, now, actor); err != nil {
		return nil, false, err
	}
	return &updated, true, nil
}

// DeleteEpic removes an epic file and appends epic.deleted. It refuses
// (ErrConflict) while any spec still points to the epic, naming them, so a delete
// never leaves dangling references behind.
func (s *Store) DeleteEpic(id, actor string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	epic, err := s.GetEpic(id)
	if err != nil {
		return err
	}
	specs, err := s.ListSpecs()
	if err != nil {
		return err
	}
	members := make([]string, 0)
	for _, spec := range specs {
		if spec.Epic == id {
			members = append(members, spec.ID)
		}
	}
	if len(members) > 0 {
		return conflictf("epic %q is still assigned to %d spec(s): %s — clear them first with `vector spec epic <spec-id> --clear`",
			id, len(members), strings.Join(members, ", "))
	}

	if err := os.Remove(s.epicPath(id)); err != nil {
		return fmt.Errorf("delete epic %q: %w", id, err)
	}
	return s.appendEpicEvent(EvtEpicDeleted, epic, now.UTC(), actor)
}

// AssignEpic points a spec at an epic (epicID "" clears it): lock → read → validate
// the epic exists → idempotency → atomic write + spec.epic-assigned event.
// Re-assigning the same epic is a no-op returning (false, nil). The spec's
// lifecycle status is untouched — grouping is metadata, not a transition.
func (s *Store) AssignEpic(specID, epicID, actor string, now time.Time) (bool, error) {
	epicID = strings.TrimSpace(epicID)
	s.mu.Lock()
	defer s.mu.Unlock()

	if epicID != "" {
		if err := s.requireEpic(epicID); err != nil {
			return false, err
		}
	}
	spec, err := s.ReadSpec(specID)
	if err != nil {
		return false, err
	}
	if spec.Epic == epicID {
		return false, nil
	}

	now = now.UTC()
	previous := spec.Epic
	spec.Epic = epicID
	spec.UpdatedAt = now
	if err := writeSpecFile(s.statePath(specID), spec); err != nil {
		return false, err
	}
	if err := s.appendEpicAssignedEvent(spec, previous, now, actor); err != nil {
		return false, err
	}
	return true, nil
}

// requireEpic checks that an epic exists, reporting a missing one as caller input
// to fix (ErrInvalidInput) rather than a not-found resource: the resource being
// written is the spec. The caller holds s.mu.
func (s *Store) requireEpic(epicID string) error {
	if err := validateEpicID(epicID); err != nil {
		return err
	}
	if _, err := os.Stat(s.epicPath(epicID)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return invalidInputf("epic %q does not exist (create it with `vector epic create --id %s --title ...`)", epicID, epicID)
		}
		return fmt.Errorf("stat epic %q: %w", epicID, err)
	}
	return nil
}

// CountEpicSpecs rolls specs up per epic id (specs without an epic are skipped).
// Done counts closed + archived, the "finished" states. Shared by the board
// projection and `vector epic list|show` so both report the same progress.
func CountEpicSpecs(specs []*SpecState) map[string]EpicCounts {
	counts := make(map[string]EpicCounts)
	for _, spec := range specs {
		if spec.Epic == "" {
			continue
		}
		entry := counts[spec.Epic]
		if entry.ByStatus == nil {
			entry.ByStatus = make(map[Status]int)
		}
		entry.Total++
		entry.ByStatus[spec.Status]++
		if spec.Status.IsTerminal() {
			entry.Done++
		}
		counts[spec.Epic] = entry
	}
	return counts
}

// appendEpicEvent appends an epic.* event (no SpecID). The caller holds s.mu.
func (s *Store) appendEpicEvent(eventType EventType, epic *Epic, now time.Time, actor string) error {
	data, err := json.Marshal(EpicEventData{ID: epic.ID, Title: epic.Title, Color: epic.Color})
	if err != nil {
		return fmt.Errorf("marshal %s data: %w", eventType, err)
	}
	return s.appendEvent(Event{V: EventVersion, TS: now, Type: eventType, Actor: actor, Data: data})
}

// appendEpicAssignedEvent appends a spec.epic-assigned event. The caller holds s.mu.
func (s *Store) appendEpicAssignedEvent(spec *SpecState, previous string, now time.Time, actor string) error {
	data, err := json.Marshal(EpicAssignedData{Epic: spec.Epic, Previous: previous})
	if err != nil {
		return fmt.Errorf("marshal spec.epic-assigned data: %w", err)
	}
	return s.appendEvent(Event{V: EventVersion, TS: now, Type: EvtEpicAssigned, SpecID: spec.ID, Repo: spec.Repo, Actor: actor, Data: data})
}

func writeEpicFile(path string, epic *Epic) error {
	b, err := json.MarshalIndent(epic, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal epic: %w", err)
	}
	return writeFileAtomic(path, append(b, '\n'))
}
