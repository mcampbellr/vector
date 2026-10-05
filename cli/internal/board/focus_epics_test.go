package board

import (
	"strings"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

// TestSortCardsFocusBeforePriority pins the in-column order: focused cards first
// (any priority), then priority, then recency.
func TestSortCardsFocusBeforePriority(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{specs: []*state.SpecState{
		{ID: "urgent", Title: "U", Status: state.StatusOpen, Priority: state.PriorityUrgent, UpdatedAt: now.Add(3)},
		{ID: "focused-low", Title: "FL", Status: state.StatusOpen, Priority: state.PriorityLow, Focus: true, UpdatedAt: now},
		{ID: "focused-high", Title: "FH", Status: state.StatusOpen, Priority: state.PriorityHigh, Focus: true, UpdatedAt: now},
		{ID: "normal", Title: "N", Status: state.StatusOpen, Priority: state.PriorityNormal, UpdatedAt: now.Add(9)},
	}}
	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatal(err)
	}
	open := columnByStatus(t, b, "open")
	got := make([]string, 0, len(open.Cards))
	for _, card := range open.Cards {
		got = append(got, card.ID)
	}
	want := "focused-high,focused-low,urgent,normal"
	if strings.Join(got, ",") != want {
		t.Fatalf("order = %v, want %s", got, want)
	}
	if !open.Cards[0].Focus || open.Cards[2].Focus {
		t.Errorf("card.Focus not projected: %+v", open.Cards)
	}
}

func TestBuildProjectsEpics(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{
		epics: []*state.Epic{
			{ID: "app-mobile", Title: "App Mobile", Description: "iOS + Android", Color: state.EpicColorBlue, UpdatedAt: now},
			{ID: "empty", Title: "Empty", UpdatedAt: now},
		},
		specs: []*state.SpecState{
			{ID: "a", Title: "A", Status: state.StatusOpen, Priority: state.PriorityNormal, Epic: "app-mobile"},
			{ID: "b", Title: "B", Status: state.StatusClosed, Priority: state.PriorityNormal, Epic: "app-mobile"},
			{ID: "c", Title: "C", Status: state.StatusArchived, Priority: state.PriorityNormal, Epic: "app-mobile", Resolution: state.ResolutionDone},
			{ID: "dup", Title: "Dup", Status: state.StatusClosed, Priority: state.PriorityNormal, Epic: "app-mobile", Resolution: state.ResolutionDuplicate},
			{ID: "d", Title: "D", Status: state.StatusOpen, Priority: state.PriorityNormal},
		},
	}
	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Epics) != 2 {
		t.Fatalf("epics = %+v, want 2", b.Epics)
	}
	mobile := b.Epics[0]
	if mobile.ID != "app-mobile" || mobile.Title != "App Mobile" || mobile.Color != "blue" || mobile.Description != "iOS + Android" {
		t.Errorf("epic identity = %+v", mobile)
	}
	if mobile.Total != 3 || mobile.Done != 2 || mobile.Dropped != 1 || mobile.ByStatus["open"] != 1 || mobile.ByStatus["archived"] != 1 {
		t.Errorf("epic counts = %+v, want total 3 / done 2 incl. archived / dropped 1", mobile)
	}
	if empty := b.Epics[1]; empty.Total != 0 || empty.ByStatus == nil {
		t.Errorf("an epic without specs still serializes an empty byStatus: %+v", empty)
	}

	open := columnByStatus(t, b, "open")
	epicByCard := map[string]string{}
	for _, card := range open.Cards {
		epicByCard[card.ID] = card.Epic
	}
	if epicByCard["a"] != "app-mobile" || epicByCard["d"] != "" {
		t.Errorf("card.Epic projection = %v", epicByCard)
	}
}

func TestBoardJSONEpicsIsAnArrayAndCardKeysAreOmitted(t *testing.T) {
	body := boardJSON(t, fakeSource{specs: []*state.SpecState{
		{ID: "a", Title: "A", Status: state.StatusOpen, Priority: state.PriorityNormal},
	}})
	if !strings.Contains(body, `"epics":[]`) {
		t.Errorf("expected epics:[] with no epics, got: %s", body)
	}
	for _, key := range []string{`"focus"`, `"epic"`} {
		if strings.Contains(body, key) {
			t.Errorf("expected %s omitted on a plain card, got: %s", key, body)
		}
	}
}
