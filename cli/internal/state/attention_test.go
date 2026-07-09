package state

import (
	"strings"
	"testing"
	"time"
)

// TestSetStatusAttentionStructured verifies the structured path persists the three
// new fields and fixes Reason == Summary so legacy readers still get a one-liner.
func TestSetStatusAttentionStructured(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusInProgress, PriorityNormal)
	now := time.Now()

	att := Attention{Category: AttentionDependency, Summary: "Zoho api_names pending creds", Detail: "PR #367 open; fill `TODO(MH-1582)`"}
	flagged, err := store.SetStatusAttention("feat", StatusNeedsAttention, att, "tester", now)
	if err != nil {
		t.Fatalf("SetStatusAttention: %v", err)
	}
	if flagged.Flag == nil {
		t.Fatal("attention flag not set")
	}
	if got := flagged.Flag.Category; got != AttentionDependency {
		t.Errorf("Category = %q, want dependency", got)
	}
	if got := flagged.Flag.Summary; got != att.Summary {
		t.Errorf("Summary = %q, want %q", got, att.Summary)
	}
	if got := flagged.Flag.Detail; got != att.Detail {
		t.Errorf("Detail = %q, want %q", got, att.Detail)
	}
	if flagged.Flag.Reason != att.Summary {
		t.Errorf("Reason = %q, want it fixed to Summary %q", flagged.Flag.Reason, att.Summary)
	}

	// Reload from disk to confirm the fields persist (not just the in-memory return).
	reread, err := store.ReadSpec("feat")
	if err != nil {
		t.Fatal(err)
	}
	if reread.Flag == nil || reread.Flag.Summary != att.Summary || reread.Flag.Detail != att.Detail || reread.Flag.Category != AttentionDependency {
		t.Errorf("persisted flag = %+v, want the structured fields", reread.Flag)
	}
}

// TestSetStatusAttentionDefaults verifies Category defaults to "other" and Detail
// falls back to Summary when the caller omits them.
func TestSetStatusAttentionDefaults(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusInProgress, PriorityNormal)

	flagged, err := store.SetStatusAttention("feat", StatusNeedsAttention, Attention{Summary: "waiting on a decision"}, "tester", time.Now())
	if err != nil {
		t.Fatalf("SetStatusAttention: %v", err)
	}
	if flagged.Flag.Category != AttentionOther {
		t.Errorf("Category = %q, want other (default)", flagged.Flag.Category)
	}
	if flagged.Flag.Detail != "waiting on a decision" {
		t.Errorf("Detail = %q, want it to fall back to Summary", flagged.Flag.Detail)
	}
}

// TestSetStatusAttentionRejectsInvalid covers the guards: missing summary, an
// unknown category enum, and a non-needs-attention target.
func TestSetStatusAttentionRejectsInvalid(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusInProgress, PriorityNormal)
	now := time.Now()

	if _, err := store.SetStatusAttention("feat", StatusNeedsAttention, Attention{Summary: ""}, "tester", now); err == nil {
		t.Error("missing summary should fail")
	}
	if _, err := store.SetStatusAttention("feat", StatusNeedsAttention, Attention{Category: "bogus", Summary: "x"}, "tester", now); err == nil {
		t.Error("invalid category enum should fail")
	}
	if _, err := store.SetStatusAttention("feat", StatusReview, Attention{Summary: "x"}, "tester", now); err == nil {
		t.Error("structured attention on a non-needs-attention target should fail")
	}
}

// TestSetStatusLegacyMigratesAttention verifies the legacy --reason path migrates
// on write: Category="other", Summary truncated, Detail=Reason=reason.
func TestSetStatusLegacyMigratesAttention(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusInProgress, PriorityNormal)

	longReason := strings.Repeat("blocked on the upstream DTO rename that has not landed yet ", 4) // > 80 runes
	flagged, err := store.SetStatus("feat", StatusNeedsAttention, longReason, "tester", time.Now())
	if err != nil {
		t.Fatalf("SetStatus needs-attention (legacy): %v", err)
	}
	if flagged.Flag.Category != AttentionOther {
		t.Errorf("Category = %q, want other", flagged.Flag.Category)
	}
	if flagged.Flag.Reason != longReason || flagged.Flag.Detail != longReason {
		t.Errorf("Reason/Detail should keep the full reason; got Reason=%q Detail=%q", flagged.Flag.Reason, flagged.Flag.Detail)
	}
	summaryRunes := []rune(flagged.Flag.Summary)
	if len(summaryRunes) > attentionSummaryMax+1 { // +1 for the ellipsis
		t.Errorf("Summary length = %d runes, want <= %d", len(summaryRunes), attentionSummaryMax+1)
	}
	if !strings.HasSuffix(flagged.Flag.Summary, "…") {
		t.Errorf("Summary %q should be ellipsized when the reason exceeds the bound", flagged.Flag.Summary)
	}

	// A short reason is not truncated and carries no ellipsis.
	newSpec(t, store, "feat2", StatusInProgress, PriorityNormal)
	short, err := store.SetStatus("feat2", StatusNeedsAttention, "blocked on DTO", "tester", time.Now())
	if err != nil {
		t.Fatalf("SetStatus short reason: %v", err)
	}
	if short.Flag.Summary != "blocked on DTO" {
		t.Errorf("short Summary = %q, want the reason verbatim", short.Flag.Summary)
	}
}
