package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mariocampbell/vector/internal/state"
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

// TestBuildStructuredAttention covers the CLI-side validation for the structured
// needs-attention flags: target guard, --reason mutual exclusion, required
// --summary, enum membership, and --detail/--detail-file exclusion + happy path.
func TestBuildStructuredAttention(t *testing.T) {
	na := state.StatusNeedsAttention
	tests := []struct {
		name         string
		target       state.Status
		reason       string
		category     string
		summary      string
		detail       string
		detailFile   string
		wantErr      bool
		wantCategory state.AttentionCategory
		wantDetail   string
	}{
		{name: "happy path", target: na, category: "dependency", summary: "creds pending", detail: "PR #367", wantCategory: state.AttentionDependency, wantDetail: "PR #367"},
		{name: "empty category defaults to other", target: na, summary: "waiting on a decision", wantCategory: state.AttentionOther, wantDetail: ""},
		{name: "wrong target rejected", target: state.StatusReview, summary: "x", wantErr: true},
		{name: "reason mutually exclusive", target: na, reason: "legacy text", summary: "x", wantErr: true},
		{name: "summary required", target: na, category: "env", wantErr: true},
		{name: "invalid category", target: na, category: "bogus", summary: "x", wantErr: true},
		{name: "detail and detail-file exclusive", target: na, summary: "x", detail: "a", detailFile: "b", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			att, err := buildStructuredAttention(tc.target, tc.reason, tc.category, tc.summary, tc.detail, tc.detailFile)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("buildStructuredAttention() = %+v, want error", att)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildStructuredAttention() unexpected error: %v", err)
			}
			if att.Category != tc.wantCategory {
				t.Errorf("Category = %q, want %q", att.Category, tc.wantCategory)
			}
			if att.Summary != tc.summary {
				t.Errorf("Summary = %q, want %q", att.Summary, tc.summary)
			}
			if att.Detail != tc.wantDetail {
				t.Errorf("Detail = %q, want %q", att.Detail, tc.wantDetail)
			}
		})
	}
}

// TestBuildStructuredAttentionDetailFile verifies --detail-file is read from disk
// into the overlay's Detail.
func TestBuildStructuredAttentionDetailFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "detail.md")
	body := "## Blocker\n\n- fill `TODO(MH-1582)`\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	att, err := buildStructuredAttention(state.StatusNeedsAttention, "", "external", "creds pending", "", path)
	if err != nil {
		t.Fatalf("buildStructuredAttention: %v", err)
	}
	if att.Detail != body {
		t.Errorf("Detail = %q, want the file contents %q", att.Detail, body)
	}

	// A missing --detail-file surfaces a read error.
	if _, err := buildStructuredAttention(state.StatusNeedsAttention, "", "external", "x", "", filepath.Join(t.TempDir(), "missing.md")); err == nil {
		t.Error("missing --detail-file should error")
	}
}
