package state

import (
	"os"
	"testing"
	"time"
)

func TestWorkLogAppendsEventWithoutMutatingState(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	spec, err := store.CreateSpec(CreateSpecParams{Title: "Alpha", Status: StatusInProgress, Actor: "tester", Now: fixedNow()})
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	stateBefore, err := os.ReadFile(store.statePath(spec.ID))
	if err != nil {
		t.Fatalf("read state before: %v", err)
	}

	at := fixedNow().Add(time.Hour)
	data := WorkLoggedData{Change: spec.ID, FilesTouched: []string{"a.go"}, TasksCompleted: []string{"mapper"}, Note: "done"}
	if err := store.WorkLog(spec.ID, data, "tester", at); err != nil {
		t.Fatalf("WorkLog: %v", err)
	}

	// state.json untouched.
	stateAfter, err := os.ReadFile(store.statePath(spec.ID))
	if err != nil {
		t.Fatalf("read state after: %v", err)
	}
	if string(stateBefore) != string(stateAfter) {
		t.Errorf("WorkLog mutated state.json")
	}

	events, err := store.ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	var work *Event
	for i := range events {
		if events[i].Type == EvtWorkLogged {
			work = &events[i]
		}
	}
	if work == nil {
		t.Fatal("no work.logged event appended")
	}
	if work.SpecID != spec.ID || !work.TS.Equal(at.UTC()) {
		t.Errorf("work event envelope = %+v", work)
	}
}

func TestWorkLogUnknownSpecErrors(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.WorkLog("ghost", WorkLoggedData{}, "tester", fixedNow()); err == nil {
		t.Fatal("expected error for unknown spec, got nil")
	}
}

func TestReadStandupMissingReturnsZeroValue(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	d, err := store.ReadStandup()
	if err != nil {
		t.Fatalf("ReadStandup: %v", err)
	}
	if !d.MarkerAt.IsZero() || !d.GeneratedAt.IsZero() {
		t.Errorf("missing digest should be zero-value, got %+v", d)
	}
}

func TestWriteStandupRoundTripAndMarkerAdvance(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	marker := fixedNow()
	digest := StandupDigest{
		GeneratedAt: marker,
		Since:       marker.Add(-24 * time.Hour),
		Global:      "shipped alpha",
		PerSpec:     []StandupSpecDigest{{ID: "alpha", Title: "Alpha", Status: "review", Summary: "done", ChangeCount: 3}},
		Totals:      StandupTotals{Specs: 1, Changes: 3, ByStatus: map[string]int{"review": 1}},
	}
	if err := store.WriteStandup(digest, marker); err != nil {
		t.Fatalf("WriteStandup: %v", err)
	}

	got, err := store.ReadStandup()
	if err != nil {
		t.Fatalf("ReadStandup: %v", err)
	}
	if got.SchemaVersion != StandupSchemaVersion {
		t.Errorf("schemaVersion = %d, want %d", got.SchemaVersion, StandupSchemaVersion)
	}
	if got.Global != "shipped alpha" || len(got.PerSpec) != 1 || got.PerSpec[0].ID != "alpha" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if !got.MarkerAt.Equal(marker.UTC()) {
		t.Errorf("markerAt = %v, want %v", got.MarkerAt, marker.UTC())
	}

	// Advance the marker on a second write.
	later := marker.Add(24 * time.Hour)
	if err := store.WriteStandup(StandupDigest{GeneratedAt: later}, later); err != nil {
		t.Fatalf("WriteStandup (advance): %v", err)
	}
	got, err = store.ReadStandup()
	if err != nil {
		t.Fatalf("ReadStandup (advance): %v", err)
	}
	if !got.MarkerAt.Equal(later.UTC()) {
		t.Errorf("marker did not advance: %v, want %v", got.MarkerAt, later.UTC())
	}
}
