package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/standup"
	"github.com/mariocampbell/vector/internal/state"
)

// TestEnrichProjectionSetsTicket asserts enrichProjection copies a linked spec's
// ticket into the projection and leaves an unlinked spec's ticket nil.
func TestEnrichProjectionSetsTicket(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "linked", Title: "Linked", Now: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "plain", Title: "Plain", Now: now}); err != nil {
		t.Fatal(err)
	}
	ticket := state.Ticket{Provider: state.TicketJira, Key: "ACME-123", URL: "https://acme.atlassian.net/browse/ACME-123"}
	if _, err := store.LinkSpec("linked", ticket, "tester", now); err != nil {
		t.Fatal(err)
	}

	proj := standup.Projection{PerSpec: []standup.SpecActivity{{ID: "linked"}, {ID: "plain"}}}
	enrichProjection(store, &proj)

	if got := proj.PerSpec[0].Ticket; got == nil || got.Key != "ACME-123" || got.Provider != state.TicketJira {
		t.Fatalf("linked spec ticket = %+v, want key ACME-123", got)
	}
	if got := proj.PerSpec[1].Ticket; got != nil {
		t.Fatalf("unlinked spec ticket = %+v, want nil", got)
	}
}

// TestStandupCommitRoundTripsTicket asserts runStandupCommit copies the projected
// ticket into the persisted digest, recoverable via ReadStandup.
func TestStandupCommitRoundTripsTicket(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	// CreateSpec emits a spec.created event, so the spec falls in the projection
	// window (marker is the zero time on a first run).
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "linked", Title: "Linked", Now: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "plain", Title: "Plain", Now: now}); err != nil {
		t.Fatal(err)
	}
	ticket := state.Ticket{Provider: state.TicketJira, Key: "ACME-123", URL: "https://acme.atlassian.net/browse/ACME-123"}
	if _, err := store.LinkSpec("linked", ticket, "tester", now); err != nil {
		t.Fatal(err)
	}

	digestFile := writeTempDigest(t, `{"global":"two specs moved","perSpec":[{"id":"linked","summary":"ACME-123 (linked) created"},{"id":"plain","summary":"plain created"}]}`)
	if err := runStandupCommit([]string{"--digest-file", digestFile, "--repo-root", root}); err != nil {
		t.Fatalf("runStandupCommit: %v", err)
	}

	got, err := store.ReadStandup()
	if err != nil {
		t.Fatal(err)
	}
	bySpec := make(map[string]state.StandupSpecDigest, len(got.PerSpec))
	for _, sd := range got.PerSpec {
		bySpec[sd.ID] = sd
	}
	if tk := bySpec["linked"].Ticket; tk == nil || tk.Key != "ACME-123" || tk.URL != ticket.URL {
		t.Fatalf("persisted linked ticket = %+v, want key ACME-123", tk)
	}
	if tk := bySpec["plain"].Ticket; tk != nil {
		t.Fatalf("persisted plain ticket = %+v, want nil", tk)
	}
}

// TestEnrichProjectionSetsPriorSummary asserts enrichProjection copies a spec's
// persisted summary into the projection and leaves a spec without one empty.
func TestEnrichProjectionSetsPriorSummary(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "summed", Title: "Summed", Now: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "plain", Title: "Plain", Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteSummary("summed", "did the work", "apply", now); err != nil {
		t.Fatal(err)
	}

	proj := standup.Projection{PerSpec: []standup.SpecActivity{{ID: "summed"}, {ID: "plain"}}}
	enrichProjection(store, &proj)

	if got := proj.PerSpec[0].PriorSummary; got != "did the work" {
		t.Fatalf("summed priorSummary = %q, want %q", got, "did the work")
	}
	if got := proj.PerSpec[1].PriorSummary; got != "" {
		t.Fatalf("plain priorSummary = %q, want empty", got)
	}
}

// TestStandupJSONSurfacesLanguage asserts `vector standup --json` carries the
// repo's configured prose language, and omits it when none is configured.
func TestStandupJSONSurfacesLanguage(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha", Now: time.Now()}); err != nil {
		t.Fatal(err)
	}

	// No config → no language key (config.Load error is ignored).
	out := captureStdout(t, func() error { return runStandup([]string{"--repo-root", root, "--json"}) })
	var proj standup.Projection
	if err := json.Unmarshal([]byte(out), &proj); err != nil {
		t.Fatalf("unmarshal projection: %v\n%s", err, out)
	}
	if proj.Language != "" {
		t.Errorf("language without config = %q, want empty", proj.Language)
	}
	if strings.Contains(out, `"language"`) {
		t.Errorf("omitempty violated: %s", out)
	}

	// Configured language → surfaced (and trimmed).
	cfg := config.Resolve(root)
	cfg.Language = " es "
	if err := config.Write(root, cfg); err != nil {
		t.Fatal(err)
	}
	out = captureStdout(t, func() error { return runStandup([]string{"--repo-root", root, "--json"}) })
	if err := json.Unmarshal([]byte(out), &proj); err != nil {
		t.Fatalf("unmarshal projection: %v\n%s", err, out)
	}
	if proj.Language != "es" {
		t.Errorf("language with config = %q, want es", proj.Language)
	}
}

// TestEnrichProjectionSetsReviewAndBlockedSignals asserts enrichProjection copies
// the deterministic template signals: needsUat for a review card, and the
// needs-attention reason for a blocked card. A plain card carries neither.
func TestEnrichProjectionSetsReviewAndBlockedSignals(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	// A review card awaiting UAT (needsUat is only honored at review).
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "reviewing", Title: "Reviewing", Status: state.StatusReview, NeedsUAT: true, Now: now}); err != nil {
		t.Fatal(err)
	}
	// A blocked card: created in-progress, then flagged needs-attention with a reason.
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "blocked", Title: "Blocked", Status: state.StatusInProgress, Now: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetStatus("blocked", state.StatusNeedsAttention, "waiting on the upstream API", "tester", now); err != nil {
		t.Fatal(err)
	}
	// A plain in-progress card with neither signal.
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "plain", Title: "Plain", Status: state.StatusInProgress, Now: now}); err != nil {
		t.Fatal(err)
	}

	proj := standup.Projection{PerSpec: []standup.SpecActivity{{ID: "reviewing"}, {ID: "blocked"}, {ID: "plain"}}}
	enrichProjection(store, &proj)

	if !proj.PerSpec[0].NeedsUAT {
		t.Errorf("reviewing needsUat = false, want true")
	}
	if got := proj.PerSpec[1].AttentionReason; got != "waiting on the upstream API" {
		t.Errorf("blocked attentionReason = %q, want the flag reason", got)
	}
	if proj.PerSpec[2].NeedsUAT || proj.PerSpec[2].AttentionReason != "" {
		t.Errorf("plain card leaked signals: needsUat=%v reason=%q", proj.PerSpec[2].NeedsUAT, proj.PerSpec[2].AttentionReason)
	}
}

func writeTempDigest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "digest.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
