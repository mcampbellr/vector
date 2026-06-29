package main

import (
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

// TestRunSpecFixValidation exercises flag/id/classification/validation parsing of
// `vector spec fix`, and confirms a valid run records a spec.fixed event without
// transitioning status.
func TestRunSpecFixValidation(t *testing.T) {
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
		{"missing classification", []string{"add-foo", "--repo-root", root}, true},
		{"invalid classification", []string{"add-foo", "--classification", "bogus", "--repo-root", root}, true},
		{"invalid validation-result", []string{"add-foo", "--classification", "spec-only", "--validation-result", "maybe", "--repo-root", root}, true},
		{"invalid artifacts", []string{"add-foo", "--classification", "spec-only", "--artifacts", "readme", "--repo-root", root}, true},
		{"non-kebab id", []string{"Add_Foo", "--classification", "spec-only", "--repo-root", root}, true},
		{"missing id", []string{"--classification", "spec-only", "--repo-root", root}, true},
		{"unknown spec", []string{"ghost", "--classification", "spec-only", "--repo-root", root}, true},
		{"valid", []string{"add-foo", "--classification", "spec+code", "--artifacts", "design,tasks", "--files", "x.go,y.go", "--validation-result", "pass", "--repo-root", root}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runSpecFix(tt.args)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	// The valid run leaves status unchanged and appends exactly one spec.fixed.
	spec, err := store.ReadSpec("add-foo")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Status != state.StatusReview {
		t.Errorf("status = %q, want review (fix never transitions)", spec.Status)
	}
	events, err := store.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	fixed := 0
	for _, e := range events {
		if e.Type == state.EvtSpecFixed {
			fixed++
		}
	}
	if fixed != 1 {
		t.Errorf("spec.fixed count = %d, want 1", fixed)
	}
}
