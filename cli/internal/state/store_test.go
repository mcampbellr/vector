package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 6, 23, 14, 0, 0, 0, time.UTC)
}

func TestCreateSpecWritesStateAndEvent(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	spec, err := store.CreateSpec(CreateSpecParams{
		Title: "New checkout flow",
		Repo:  "cdr",
		Body:  "# New checkout flow\n\nraw spec body\n",
		Actor: "tester",
		Now:   fixedNow(),
	})
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}

	// Invariants on the returned state.
	if spec.ID != "new-checkout-flow" {
		t.Errorf("ID = %q, want new-checkout-flow", spec.ID)
	}
	if spec.Status != StatusOpen {
		t.Errorf("Status = %q, want open", spec.Status)
	}
	if spec.Priority != PriorityNormal {
		t.Errorf("Priority = %q, want normal (default)", spec.Priority)
	}
	if !spec.CreatedAt.Equal(fixedNow()) || !spec.UpdatedAt.Equal(fixedNow()) {
		t.Errorf("timestamps not set to Now")
	}
	if spec.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", spec.SchemaVersion, SchemaVersion)
	}

	// state.json round-trips from disk (single source of truth on disk).
	onDisk, err := store.ReadSpec(spec.ID)
	if err != nil {
		t.Fatalf("ReadSpec: %v", err)
	}
	if onDisk.Title != "New checkout flow" || onDisk.Status != StatusOpen {
		t.Errorf("on-disk spec mismatch: %+v", onDisk)
	}

	// spec.md written.
	body, err := os.ReadFile(filepath.Join(root, ".vector", "specs", spec.ID, "spec.md"))
	if err != nil {
		t.Fatalf("read spec.md: %v", err)
	}
	if len(body) == 0 {
		t.Error("spec.md is empty")
	}

	// activity.jsonl has exactly one spec.created event with the expected shape.
	events := readEvents(t, filepath.Join(root, ".vector", "local", "activity.jsonl"))
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Type != EvtSpecCreated || ev.SpecID != spec.ID || ev.Actor != "tester" || ev.V != EventVersion {
		t.Errorf("unexpected event envelope: %+v", ev)
	}
	var data SpecCreatedData
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("decode event data: %v", err)
	}
	if data.Source != "raw" || data.Template != "idea" || data.Title != "New checkout flow" {
		t.Errorf("unexpected event data: %+v", data)
	}
}

func TestCreateSpecRejectsDuplicate(t *testing.T) {
	store, _ := Open(t.TempDir())
	params := CreateSpecParams{Title: "Dup", Actor: "t", Now: fixedNow()}
	if _, err := store.CreateSpec(params); err != nil {
		t.Fatalf("first CreateSpec: %v", err)
	}
	if _, err := store.CreateSpec(params); err == nil {
		t.Fatal("expected error creating duplicate spec, got nil")
	}
}

func TestCreateSpecValidatesInputs(t *testing.T) {
	store, _ := Open(t.TempDir())

	if _, err := store.CreateSpec(CreateSpecParams{Title: "   ", Now: fixedNow()}); err == nil {
		t.Error("expected error for empty-derived id")
	}
	if _, err := store.CreateSpec(CreateSpecParams{ID: "Not Kebab", Now: fixedNow()}); err == nil {
		t.Error("expected error for non-kebab id")
	}
	if _, err := store.CreateSpec(CreateSpecParams{Title: "x", Priority: "huge", Now: fixedNow()}); err == nil {
		t.Error("expected error for invalid priority")
	}
}

func TestListSpecs(t *testing.T) {
	store, _ := Open(t.TempDir())
	for _, title := range []string{"Alpha", "Beta"} {
		if _, err := store.CreateSpec(CreateSpecParams{Title: title, Now: fixedNow()}); err != nil {
			t.Fatalf("CreateSpec(%q): %v", title, err)
		}
	}
	specs, err := store.ListSpecs()
	if err != nil {
		t.Fatalf("ListSpecs: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("ListSpecs len = %d, want 2", len(specs))
	}
	// ReadDir returns sorted names: alpha, beta.
	if specs[0].ID != "alpha" || specs[1].ID != "beta" {
		t.Errorf("unexpected order: %q, %q", specs[0].ID, specs[1].ID)
	}
}

func readEvents(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open activity log: %v", err)
	}
	defer f.Close()
	var events []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("decode event line: %v", err)
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan activity log: %v", err)
	}
	return events
}
