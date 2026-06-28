package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/standup"
	"github.com/mariocampbell/vector/internal/state"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it
// printed, so we can assert on the JSON projection emitted by --json commands.
func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	runErr := fn()
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("command returned error: %v", runErr)
	}
	return string(out)
}

func TestSummarizeProjectionShape(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha", Status: state.StatusInProgress, Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.WorkLog("alpha", state.WorkLoggedData{Change: "alpha", FilesTouched: []string{"a.go"}, Note: "did work"}, "tester", now); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() error {
		return runSpecSummarize([]string{"alpha", "--json", "--repo-root", root})
	})

	var proj summarizeProjection
	if err := json.Unmarshal([]byte(out), &proj); err != nil {
		t.Fatalf("projection not valid JSON: %v\n%s", err, out)
	}
	if proj.ID != "alpha" || proj.Title != "Alpha" || proj.Status != "in-progress" {
		t.Errorf("projection identity wrong: %+v", proj)
	}
	if len(proj.Events) == 0 {
		t.Errorf("expected recent events in projection, got none")
	}
}

func TestSummarizeProjectionIncludesPriorSummary(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha", Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteSummary("alpha", "earlier prose", "propose", now); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() error {
		return runSpecSummarize([]string{"alpha", "--json", "--repo-root", root})
	})
	var proj summarizeProjection
	if err := json.Unmarshal([]byte(out), &proj); err != nil {
		t.Fatalf("projection not valid JSON: %v", err)
	}
	if proj.PriorSummary != "earlier prose" {
		t.Errorf("priorSummary = %q, want %q", proj.PriorSummary, "earlier prose")
	}
}

func TestSummarizeCommitPersists(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha", Now: time.Now()}); err != nil {
		t.Fatal(err)
	}

	file := writeTempSummary(t, `{"summary":"applied the change end to end"}`)
	if err := runSpecSummarizeCommit("", []string{"alpha", "--action", "apply", "--summary-file", file, "--repo-root", root}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got, err := store.ReadSummary("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Summary != "applied the change end to end" || got.Action != "apply" {
		t.Fatalf("persisted summary = %+v", got)
	}
}

// TestSummarizeDispatchIdThenCommit covers the `summarize <id> commit …`
// ordering the kit commands use, routed through the runSpecSummarize dispatcher.
func TestSummarizeDispatchIdThenCommit(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha", Now: time.Now()}); err != nil {
		t.Fatal(err)
	}

	file := writeTempSummary(t, `{"summary":"committed via id-then-commit ordering"}`)
	if err := runSpecSummarize([]string{"alpha", "commit", "--action", "close", "--summary-file", file, "--repo-root", root}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	got, err := store.ReadSummary("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Action != "close" {
		t.Fatalf("persisted summary = %+v", got)
	}
}

func TestSummarizeCommitEmptyWritesNothing(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha", Now: time.Now()}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		body string
	}{
		{"empty prose", `{"summary":"   "}`},
		{"invalid json", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := writeTempSummary(t, tc.body)
			err := runSpecSummarizeCommit("", []string{"alpha", "--action", "apply", "--summary-file", file, "--repo-root", root})
			if err == nil {
				t.Fatalf("expected an error for %s", tc.name)
			}
			sum, rerr := store.ReadSummary("alpha")
			if rerr != nil {
				t.Fatal(rerr)
			}
			if sum != nil {
				t.Errorf("expected nothing written, got %+v", sum)
			}
		})
	}
}

// TestSummarizeCommitClosePreservesPriorWhenNoNewWork is the deterministic
// safeguard: a close/archive with no work.logged recorded after the last summary
// has nothing fresh to describe, so the binary preserves the rich prior summary
// instead of letting a regenerated (possibly degenerate) line overwrite it.
func TestSummarizeCommitClosePreservesPriorWhenNoNewWork(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha", Now: t0}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteSummary("alpha", "Built the CopyableSlug component and wired it into the card and drawer.", "apply", t0); err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"close", "archive"} {
		file := writeTempSummary(t, `{"summary":"Closed after review. Implementation finalized."}`)
		if err := runSpecSummarizeCommit("", []string{"alpha", "--action", action, "--summary-file", file, "--repo-root", root}); err != nil {
			t.Fatalf("commit %s: %v", action, err)
		}
		got, err := store.ReadSummary("alpha")
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || got.Action != "apply" || got.Summary != "Built the CopyableSlug component and wired it into the card and drawer." {
			t.Fatalf("%s should have preserved the prior summary, got %+v", action, got)
		}
	}
}

// TestSummarizeCommitCloseOverwritesAfterNewWork verifies the safeguard does not
// over-preserve: when real work (work.logged) was recorded after the prior
// summary, a close summary has something new to say and overwrites as usual.
func TestSummarizeCommitCloseOverwritesAfterNewWork(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha", Now: t0}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteSummary("alpha", "earlier prose", "apply", t0); err != nil {
		t.Fatal(err)
	}
	// New work after the prior summary → there is something fresh to describe.
	if err := store.WorkLog("alpha", state.WorkLoggedData{FilesTouched: []string{"b.go"}, Note: "more work"}, "tester", t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	file := writeTempSummary(t, `{"summary":"new substance from the latest run"}`)
	if err := runSpecSummarizeCommit("", []string{"alpha", "--action", "close", "--summary-file", file, "--repo-root", root}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	got, err := store.ReadSummary("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Action != "close" || got.Summary != "new substance from the latest run" {
		t.Fatalf("close after new work should overwrite, got %+v", got)
	}
}

// TestSummarizeProjectionHasWorkFalseWhenNoWorkLogged verifies that a window with
// only a status.changed event (and no work.logged) produces hasWork=false and a
// non-empty templateSummary.
func TestSummarizeProjectionHasWorkFalseWhenNoWorkLogged(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "beta", Title: "Beta", Now: now}); err != nil {
		t.Fatal(err)
	}
	// Append a status.changed event without any work.logged.
	payload, _ := json.Marshal(state.StatusChangedData{
		From:    state.StatusDraft,
		To:      state.StatusOpen,
		Trigger: "command",
	})
	if err := store.AppendEvent(state.Event{
		V:      1,
		TS:     now,
		Type:   state.EvtStatusChanged,
		SpecID: "beta",
		Actor:  "tester",
		Data:   payload,
	}); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() error {
		return runSpecSummarize([]string{"beta", "--json", "--repo-root", root})
	})

	var proj summarizeProjection
	if err := json.Unmarshal([]byte(out), &proj); err != nil {
		t.Fatalf("projection not valid JSON: %v\n%s", err, out)
	}
	if proj.HasWork {
		t.Errorf("hasWork = true; want false (no work.logged in window)")
	}
	if proj.TemplateSummary == "" {
		t.Errorf("templateSummary is empty; want a non-empty deterministic string")
	}
}

// TestSummarizeProjectionHasWorkTrueWhenWorkLogged verifies that a window with a
// work.logged event produces hasWork=true and an empty (omitted) templateSummary.
func TestSummarizeProjectionHasWorkTrueWhenWorkLogged(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "gamma", Title: "Gamma", Status: state.StatusInProgress, Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.WorkLog("gamma", state.WorkLoggedData{Note: "implemented something"}, "tester", now); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() error {
		return runSpecSummarize([]string{"gamma", "--json", "--repo-root", root})
	})

	var proj summarizeProjection
	if err := json.Unmarshal([]byte(out), &proj); err != nil {
		t.Fatalf("projection not valid JSON: %v\n%s", err, out)
	}
	if !proj.HasWork {
		t.Errorf("hasWork = false; want true (work.logged present in window)")
	}
	if proj.TemplateSummary != "" {
		t.Errorf("templateSummary = %q; want empty when hasWork is true", proj.TemplateSummary)
	}
}

// TestBuildTemplateSummary covers the five canonical cases plus the title-empty
// edge case of buildTemplateSummary.
func TestBuildTemplateSummary(t *testing.T) {
	makeEvent := func(evtType, from, to string) standup.TimelineEvent {
		return standup.TimelineEvent{Type: evtType, From: from, To: to}
	}

	cases := []struct {
		name   string
		id     string
		title  string
		events []standup.TimelineEvent
		want   string
	}{
		{
			name:   "spec.proposed uses title label",
			id:     "my-spec",
			title:  "My Spec",
			events: []standup.TimelineEvent{makeEvent("spec.proposed", "", "")},
			want:   "My Spec proposed (draft → open)",
		},
		{
			name:   "spec.closed uses title label",
			id:     "my-spec",
			title:  "My Spec",
			events: []standup.TimelineEvent{makeEvent("spec.closed", "", "")},
			want:   "My Spec closed",
		},
		{
			name:   "spec.archived uses title label",
			id:     "my-spec",
			title:  "My Spec",
			events: []standup.TimelineEvent{makeEvent("spec.archived", "", "")},
			want:   "My Spec archived",
		},
		{
			name:   "status.changed last match",
			id:     "my-spec",
			title:  "My Spec",
			events: []standup.TimelineEvent{makeEvent("status.changed", "in-progress", "review")},
			want:   "My Spec: moved from in-progress to review",
		},
		{
			name:   "no events fallback",
			id:     "my-spec",
			title:  "My Spec",
			events: []standup.TimelineEvent{},
			want:   `spec "my-spec": no recent activity`,
		},
		{
			name:   "empty title falls back to id as label",
			id:     "bare-id",
			title:  "",
			events: []standup.TimelineEvent{makeEvent("spec.closed", "", "")},
			want:   "bare-id closed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildTemplateSummary(tc.id, tc.title, tc.events)
			if got != tc.want {
				t.Errorf("buildTemplateSummary(%q, %q, ...) = %q; want %q", tc.id, tc.title, got, tc.want)
			}
		})
	}
}

func writeTempSummary(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "summary.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
