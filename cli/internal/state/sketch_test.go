package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sampleSketch = `{"type":"excalidraw","version":2,"elements":[]}`

// TestSpecStateSketchesRoundTrip covers task 8.1: Sketches serializes when present,
// is omitted when empty, and a legacy state without the field loads as nil.
func TestSpecStateSketchesRoundTrip(t *testing.T) {
	now := fixedNow()
	spec := SpecState{
		SchemaVersion: SchemaVersion,
		ID:            "alpha",
		Title:         "Alpha",
		Status:        StatusDraft,
		Priority:      PriorityNormal,
		CreatedAt:     now,
		UpdatedAt:     now,
		Sketches:      []SketchRef{{Name: "sketch.excalidraw", CreatedAt: now}},
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back SpecState
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.Sketches) != 1 || back.Sketches[0].Name != "sketch.excalidraw" {
		t.Fatalf("Sketches round-trip = %+v", back.Sketches)
	}

	// Empty Sketches is omitted from the JSON (byte-compatibility with legacy specs).
	spec.Sketches = nil
	b, err = json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	if bytesContains(b, "sketches") {
		t.Errorf("empty Sketches must be omitted, got %s", b)
	}

	// A legacy state.json without the field loads as nil.
	var legacy SpecState
	if err := json.Unmarshal([]byte(`{"id":"a","title":"A","status":"draft","priority":"normal"}`), &legacy); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if legacy.Sketches != nil {
		t.Errorf("legacy Sketches = %+v, want nil", legacy.Sketches)
	}
}

// TestAttachSketchPersistsAndProjects covers task 8.3 (store) + 8.2 (artifact path):
// a valid attach writes the file, appends the ref, bumps UpdatedAt, and the artifact
// path resolves; a missing sketch is fs.ErrNotExist.
func TestAttachSketchPersistsAndProjects(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.CreateSpec(CreateSpecParams{Title: "Alpha", Body: "x", Now: fixedNow()}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}

	// No sketch yet → artifact path is fs.ErrNotExist.
	if _, err := store.ReadSpecArtifact("alpha", "sketch"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("no sketch: err = %v, want fs.ErrNotExist", err)
	}

	created := fixedNow().Add(time.Hour)
	if _, err := store.AttachSketch("alpha", []byte(sampleSketch), SketchRef{Name: "board.excalidraw", CreatedAt: created}, "cli"); err != nil {
		t.Fatalf("AttachSketch: %v", err)
	}

	// File written under the spec's sketches shard.
	onDisk := filepath.Join(root, ".vector", "specs", "alpha", "sketches", "board.excalidraw")
	if b, err := os.ReadFile(onDisk); err != nil || string(b) != sampleSketch {
		t.Fatalf("sketch file = %q, err = %v", string(b), err)
	}

	// State updated: ref appended, UpdatedAt bumped to the ref's time.
	spec, err := store.ReadSpec("alpha")
	if err != nil {
		t.Fatalf("ReadSpec: %v", err)
	}
	if len(spec.Sketches) != 1 || spec.Sketches[0].Name != "board.excalidraw" {
		t.Fatalf("Sketches = %+v", spec.Sketches)
	}
	if !spec.UpdatedAt.Equal(created.UTC()) {
		t.Errorf("UpdatedAt = %v, want %v", spec.UpdatedAt, created.UTC())
	}

	// artifactRelPath resolves and the artifact reads back.
	b, err := store.ReadSpecArtifact("alpha", "sketch")
	if err != nil {
		t.Fatalf("ReadSpecArtifact sketch: %v", err)
	}
	if string(b) != sampleSketch {
		t.Errorf("sketch bytes = %q", string(b))
	}
}

// TestAttachSketchEmitsEvent verifies AttachSketch appends a sketch.attached event
// to the activity log so the attach leaves a trace (state-sync-discipline).
func TestAttachSketchEmitsEvent(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root)
	if _, err := store.CreateSpec(CreateSpecParams{Title: "Alpha", Body: "x", Now: fixedNow()}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	created := fixedNow().Add(time.Hour)
	if _, err := store.AttachSketch("alpha", []byte(sampleSketch), SketchRef{Name: "board.excalidraw", CreatedAt: created}, "cli"); err != nil {
		t.Fatalf("AttachSketch: %v", err)
	}

	events, err := store.ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	var found *Event
	for i := range events {
		if events[i].Type == EvtSketchAttached {
			found = &events[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no sketch.attached event emitted; events = %+v", events)
	}
	if found.SpecID != "alpha" || found.Actor != "cli" || !found.TS.Equal(created.UTC()) {
		t.Errorf("event fields = specId %q actor %q ts %v", found.SpecID, found.Actor, found.TS)
	}
	var data SketchAttachedData
	if err := json.Unmarshal(found.Data, &data); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if data.Name != "board.excalidraw" {
		t.Errorf("payload Name = %q, want board.excalidraw", data.Name)
	}
}

// TestAttachSketchReAttachOverwrites verifies re-attaching the same name overwrites
// the file and refreshes the ref rather than duplicating it.
func TestAttachSketchReAttachOverwrites(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root)
	if _, err := store.CreateSpec(CreateSpecParams{Title: "Alpha", Body: "x", Now: fixedNow()}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	ref := SketchRef{Name: "s.excalidraw", CreatedAt: fixedNow()}
	if _, err := store.AttachSketch("alpha", []byte(sampleSketch), ref, "cli"); err != nil {
		t.Fatalf("AttachSketch 1: %v", err)
	}
	updated := `{"type":"excalidraw","version":2,"elements":[1]}`
	ref.CreatedAt = fixedNow().Add(2 * time.Hour)
	if _, err := store.AttachSketch("alpha", []byte(updated), ref, "cli"); err != nil {
		t.Fatalf("AttachSketch 2: %v", err)
	}
	spec, _ := store.ReadSpec("alpha")
	if len(spec.Sketches) != 1 {
		t.Fatalf("re-attach duplicated: %+v", spec.Sketches)
	}
	b, _ := store.ReadSpecArtifact("alpha", "sketch")
	if string(b) != updated {
		t.Errorf("re-attach did not overwrite: %q", string(b))
	}
}

// TestAttachSketchRejectsBadNameAndMissingSpec covers the store's defensive guards.
func TestAttachSketchRejectsBadNameAndMissingSpec(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root)
	if _, err := store.CreateSpec(CreateSpecParams{Title: "Alpha", Body: "x", Now: fixedNow()}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	// A non-empty --name override must be a bare file name (empty is not here: it
	// means "let the binary name it canonically", covered by TestAttachSketchCanonicalName).
	for _, name := range []string{".", "..", "a/b.excalidraw", "../escape"} {
		if _, err := store.AttachSketch("alpha", []byte(sampleSketch), SketchRef{Name: name, CreatedAt: fixedNow()}, "cli"); err == nil {
			t.Errorf("AttachSketch name %q: want error", name)
		}
	}
	// Unknown spec → error (ReadSpec wraps fs.ErrNotExist).
	if _, err := store.AttachSketch("ghost", []byte(sampleSketch), SketchRef{Name: "s.excalidraw", CreatedAt: fixedNow()}, "cli"); err == nil {
		t.Error("AttachSketch on missing spec: want error")
	}
}

// TestAttachSketchCanonicalName verifies the binary-authoritative naming: an empty
// ref.Name yields <id>{-<ticket>}-sketch{-<n>}.excalidraw, the first sketch is
// unsuffixed, and the ticket key sits between the id and "-sketch".
func TestAttachSketchCanonicalName(t *testing.T) {
	t.Run("no ticket, first then second", func(t *testing.T) {
		root := t.TempDir()
		store, _ := Open(root)
		if _, err := store.CreateSpec(CreateSpecParams{ID: "evolvs-pdp-editorial", Title: "PDP", Body: "x", Now: fixedNow()}); err != nil {
			t.Fatalf("CreateSpec: %v", err)
		}
		first, err := store.AttachSketch("evolvs-pdp-editorial", []byte(sampleSketch), SketchRef{CreatedAt: fixedNow()}, "cli")
		if err != nil {
			t.Fatalf("AttachSketch 1: %v", err)
		}
		if first != "evolvs-pdp-editorial-sketch.excalidraw" {
			t.Errorf("first name = %q, want evolvs-pdp-editorial-sketch.excalidraw", first)
		}
		second, err := store.AttachSketch("evolvs-pdp-editorial", []byte(sampleSketch), SketchRef{CreatedAt: fixedNow().Add(time.Hour)}, "cli")
		if err != nil {
			t.Fatalf("AttachSketch 2: %v", err)
		}
		if second != "evolvs-pdp-editorial-sketch-1.excalidraw" {
			t.Errorf("second name = %q, want evolvs-pdp-editorial-sketch-1.excalidraw", second)
		}
		spec, _ := store.ReadSpec("evolvs-pdp-editorial")
		if len(spec.Sketches) != 2 {
			t.Fatalf("Sketches = %+v, want 2", spec.Sketches)
		}
		// The stored ref name matches the file actually written to disk.
		if _, err := os.Stat(filepath.Join(root, ".vector", "specs", "evolvs-pdp-editorial", "sketches", second)); err != nil {
			t.Errorf("second sketch file missing: %v", err)
		}
	})

	t.Run("with ticket", func(t *testing.T) {
		root := t.TempDir()
		store, _ := Open(root)
		if _, err := store.CreateSpec(CreateSpecParams{
			ID: "evolvs-pdp-editorial", Title: "PDP", Body: "x", Now: fixedNow(),
			Ticket: &Ticket{Provider: TicketJira, Key: "EV-398", URL: "https://example.test/EV-398"},
		}); err != nil {
			t.Fatalf("CreateSpec: %v", err)
		}
		name, err := store.AttachSketch("evolvs-pdp-editorial", []byte(sampleSketch), SketchRef{CreatedAt: fixedNow()}, "cli")
		if err != nil {
			t.Fatalf("AttachSketch: %v", err)
		}
		if name != "evolvs-pdp-editorial-EV-398-sketch.excalidraw" {
			t.Errorf("name = %q, want evolvs-pdp-editorial-EV-398-sketch.excalidraw", name)
		}
	})
}

// bytesContains is a tiny substring check to avoid importing strings for one call.
func bytesContains(b []byte, sub string) bool {
	return len(sub) == 0 || indexOf(string(b), sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
