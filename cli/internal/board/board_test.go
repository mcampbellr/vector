package board

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

type fakeSource struct {
	specs   []*state.SpecState
	events  []state.Event
	standup *state.StandupDigest
}

func (f fakeSource) ListSpecs() ([]*state.SpecState, error) { return f.specs, nil }
func (f fakeSource) ReadEvents() ([]state.Event, error)     { return f.events, nil }
func (f fakeSource) ReadStandup() (*state.StandupDigest, error) {
	if f.standup == nil {
		return &state.StandupDigest{}, nil
	}
	return f.standup, nil
}

func routedEvent(t *testing.T, specID, model, baseline string, saved, cost float64) state.Event {
	t.Helper()
	data, err := json.Marshal(state.AgentRoutedData{
		Task: "summarize", Model: model, Baseline: baseline,
		TokensIn: 1000, TokensOut: 100, CostUSD: cost, SavedUSD: saved,
	})
	if err != nil {
		t.Fatalf("marshal routed data: %v", err)
	}
	return state.Event{V: 1, Type: state.EvtAgentRouted, SpecID: specID, Data: data}
}

func TestBuildGroupsByStatusAndOrdersByPriority(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{specs: []*state.SpecState{
		{ID: "a", Title: "A", Status: state.StatusOpen, Priority: state.PriorityNormal, UpdatedAt: now},
		{ID: "b", Title: "B", Status: state.StatusOpen, Priority: state.PriorityUrgent, UpdatedAt: now},
		{ID: "c", Title: "C", Status: state.StatusArchived, Priority: state.PriorityLow, UpdatedAt: now},
		{ID: "d", Title: "D", Status: state.StatusReview, Priority: state.PriorityHigh, UpdatedAt: now},
	}}

	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if got := len(b.Columns); got != len(columnOrder) {
		t.Fatalf("columns = %d, want %d", got, len(columnOrder))
	}
	open := columnByStatus(t, b, "open")
	if open.Count != 2 {
		t.Fatalf("open count = %d, want 2", open.Count)
	}
	if open.Cards[0].ID != "b" {
		t.Errorf("urgent card should sort first, got %q", open.Cards[0].ID)
	}
	if b.Totals.Specs != 4 {
		t.Errorf("totals.specs = %d, want 4 (archived counted in totals)", b.Totals.Specs)
	}
	// Archived must not appear in any column.
	for _, col := range b.Columns {
		for _, card := range col.Cards {
			if card.ID == "c" {
				t.Errorf("archived spec leaked into column %q", col.Status)
			}
		}
	}
}

func TestBuildRollsUpTokenSavings(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{
		specs: []*state.SpecState{
			{ID: "a", Title: "A", Status: state.StatusInProgress, Priority: state.PriorityHigh, UpdatedAt: now},
		},
		events: []state.Event{
			routedEvent(t, "a", "haiku", "opus", 0.31, 0.02),
			routedEvent(t, "a", "haiku", "opus", 0.10, 0.01),
			routedEvent(t, "", "sonnet", "opus", 0.05, 0.04), // no spec attribution
		},
	}

	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	s := b.TokenSavings
	if s.Routes != 3 {
		t.Errorf("routes = %d, want 3", s.Routes)
	}
	if !almostEqual(s.TotalSavedUSD, 0.46) {
		t.Errorf("totalSaved = %v, want 0.46", s.TotalSavedUSD)
	}
	if !almostEqual(s.BaselineUSD, s.TotalSpentUSD+s.TotalSavedUSD) {
		t.Errorf("baseline should equal spent+saved, got %v", s.BaselineUSD)
	}
	if len(s.ByModel) != 2 {
		t.Fatalf("byModel groups = %d, want 2", len(s.ByModel))
	}
	if s.ByModel[0].Model != "haiku" {
		t.Errorf("top saver should be haiku, got %q", s.ByModel[0].Model)
	}

	card := columnByStatus(t, b, "in-progress").Cards[0]
	if card.Routes != 2 {
		t.Errorf("card routes = %d, want 2", card.Routes)
	}
	if !almostEqual(card.SavedUSD, 0.41) {
		t.Errorf("card savedUsd = %v, want 0.41", card.SavedUSD)
	}
}

func columnByStatus(t *testing.T, b *Board, status string) Column {
	t.Helper()
	for _, c := range b.Columns {
		if c.Status == status {
			return c
		}
	}
	t.Fatalf("column %q not found", status)
	return Column{}
}

func almostEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}
