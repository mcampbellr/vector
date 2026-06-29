package main

import (
	"reflect"
	"testing"
)

// TestParseFixArtifacts mirrors TestParseArtifacts and additionally asserts the
// returned slice holds canonical names (lowercase, no .md) regardless of the
// input format, and that an empty input yields a nil slice.
func TestParseFixArtifacts(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"empty string", "", nil, false},
		{"bare design", "design", []string{"design"}, false},
		{"canonical from md", "Proposal.md", []string{"proposal"}, false},
		{"uppercase md suffix", "Tasks.MD", []string{"tasks"}, false},
		{"mixed casing", "DESIGN", []string{"design"}, false},
		{"trim spaces", "  tasks  ", []string{"tasks"}, false},
		{"mixed list canonical", "proposal.md,Design,tasks", []string{"proposal", "design", "tasks"}, false},
		{"empty segment tolerated", "design,,tasks", []string{"design", "tasks"}, false},
		{"md alone invalid", ".md", nil, true},
		{"double md invalid", "design.md.md", nil, true},
		{"unknown name invalid", "readme", nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseFixArtifacts(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseFixArtifacts(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFixArtifacts(%q) unexpected error: %v", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseFixArtifacts(%q) = %#v, want %#v", tc.input, got, tc.want)
			}
		})
	}
}
