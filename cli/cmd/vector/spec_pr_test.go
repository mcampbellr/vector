package main

import (
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

// TestRunSpecPR exercises `vector spec pr` argument parsing and confirms a valid run
// records the PR (idempotent on the URL) without transitioning status.
func TestRunSpecPR(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "add-foo", Title: "Add foo", Status: state.StatusReview, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"missing url", []string{"add-foo", "--repo-root", root}, true},
		{"missing id", []string{"--url", "https://github.com/a/b/pull/1", "--repo-root", root}, true},
		{"non-kebab id", []string{"Add_Foo", "https://github.com/a/b/pull/1", "--repo-root", root}, true},
		{"empty url flag", []string{"add-foo", "--url", "", "--repo-root", root}, true},
		{"valid positional", []string{"add-foo", "https://github.com/a/b/pull/7", "--number", "7", "--draft", "--repo-root", root}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runSpecPR(tt.args)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	// The valid run recorded the PR, kept status at review, and emitted pr.opened.
	spec, err := store.ReadSpec("add-foo")
	if err != nil {
		t.Fatal(err)
	}
	if spec.PR == nil || spec.PR.Number != 7 || !spec.PR.Draft {
		t.Fatalf("PR not recorded: %+v", spec.PR)
	}
	if spec.Status != state.StatusReview {
		t.Errorf("status = %q, want review (pr never transitions)", spec.Status)
	}

	// Idempotent via the CLI: re-recording the same URL emits no second event.
	if err := runSpecPR([]string{"add-foo", "https://github.com/a/b/pull/7", "--number", "7", "--draft", "--repo-root", root}); err != nil {
		t.Fatalf("idempotent run: %v", err)
	}
	events, err := store.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	opened := 0
	for _, e := range events {
		if e.Type == state.EvtPROpened {
			opened++
		}
	}
	if opened != 1 {
		t.Errorf("pr.opened count = %d, want 1 (idempotent re-record emits nothing)", opened)
	}
}
