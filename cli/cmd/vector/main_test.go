package main

import (
	"testing"

	"github.com/mariocampbell/vector/internal/state"
)

// TestParseArtifacts exercises the tolerant normalization of parseArtifacts:
// bare names, .md suffix, casing, surrounding whitespace, mixed lists, empty
// segments, and the invalid cases (.md alone, double .md, unknown names).
func TestParseArtifacts(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    state.ArtifactSet
		wantErr bool
	}{
		{"empty string", "", state.ArtifactSet{}, false},
		{"bare proposal", "proposal", state.ArtifactSet{Proposal: true}, false},
		{"md suffix", "proposal.md", state.ArtifactSet{Proposal: true}, false},
		{"uppercase md suffix", "Design.MD", state.ArtifactSet{Design: true}, false},
		{"mixed casing", "TASKS", state.ArtifactSet{Tasks: true}, false},
		{"trim spaces", "  proposal  ", state.ArtifactSet{Proposal: true}, false},
		{"mixed list", "proposal.md,Design,tasks", state.ArtifactSet{Proposal: true, Design: true, Tasks: true}, false},
		{"empty segment tolerated", "proposal,,tasks", state.ArtifactSet{Proposal: true, Tasks: true}, false},
		{"md alone invalid", ".md", state.ArtifactSet{}, true},
		{"double md invalid", "proposal.md.md", state.ArtifactSet{}, true},
		{"unknown name invalid", "readme", state.ArtifactSet{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseArtifacts(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseArtifacts(%q) = %+v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArtifacts(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseArtifacts(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}
}
