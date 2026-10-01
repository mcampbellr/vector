package check

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
)

var testNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

// emptyPath points PATH at an empty directory so neither git nor gh resolves:
// the remote checks degrade and the test stays hermetic on any machine.
func emptyPath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func openTestStore(t *testing.T, root string) *state.Store {
	t.Helper()
	store, err := state.Open(root)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

type specFixture struct {
	id     string
	status state.Status
	change string        // OpenSpec change name; "" → no OpenSpec provenance
	ticket *state.Ticket // linked after creation
}

func createSpecs(t *testing.T, store *state.Store, fixtures ...specFixture) {
	t.Helper()
	for _, f := range fixtures {
		params := state.CreateSpecParams{ID: f.id, Title: f.id, Status: f.status, Now: testNow, SpecDocRel: "x"}
		if f.change != "" {
			params.OpenSpec = &state.OpenSpec{Change: f.change, Artifacts: state.ArtifactSet{Tasks: true}}
		}
		if _, err := store.CreateSpec(params); err != nil {
			t.Fatalf("create %s: %v", f.id, err)
		}
		if f.ticket != nil {
			if _, err := store.LinkSpec(f.id, *f.ticket, "t", testNow); err != nil {
				t.Fatalf("link %s: %v", f.id, err)
			}
		}
	}
}

// writeChange creates openspec/changes/<name>/tasks.md with the given lines.
func writeChange(t *testing.T, root, name string, taskLines ...string) {
	t.Helper()
	dir := filepath.Join(root, "openspec", "changes", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Tasks\n\n" + strings.Join(taskLines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func jiraTicket(key string) *state.Ticket {
	return &state.Ticket{Provider: state.TicketJira, Key: key, URL: "https://example.atlassian.net/browse/" + key}
}

// editState rewrites a spec's state.json by hand, bypassing the store's
// validation — the only way to build the corrupt fixtures check must detect.
func editState(t *testing.T, store *state.Store, id string, mutate func(*state.SpecState)) {
	t.Helper()
	spec, err := store.ReadSpec(id)
	if err != nil {
		t.Fatal(err)
	}
	mutate(spec)
	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.StatePath(id), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func findDiscrepancy(report CheckReport, specID string, kind DiscrepancyType) (Discrepancy, bool) {
	for _, d := range report.Discrepancies {
		if d.SpecID == specID && d.Type == kind {
			return d, true
		}
	}
	return Discrepancy{}, false
}

func TestRunCleanBoard(t *testing.T) {
	emptyPath(t)
	root := t.TempDir()
	store := openTestStore(t, root)
	createSpecs(t, store,
		specFixture{id: "add-auth", status: state.StatusInProgress, ticket: jiraTicket("ACME-1")},
		specFixture{id: "add-billing", status: state.StatusClosed, ticket: jiraTicket("ACME-2")},
	)

	report, err := Run(store, &config.Config{}, root, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Discrepancies) != 0 {
		t.Fatalf("discrepancies = %+v, want none", report.Discrepancies)
	}
	if len(report.Specs) != 2 {
		t.Errorf("specs = %d, want both ticketed specs queued for Jira", len(report.Specs))
	}
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"discrepancies":[]`) {
		t.Errorf("JSON must carry an empty discrepancies array, got %s", b)
	}
	if strings.Contains(string(b), "Warnings") || strings.Contains(string(b), "warnings") {
		t.Errorf("warnings must stay out of the JSON contract, got %s", b)
	}
}

func TestRunLocalDriftTypes(t *testing.T) {
	emptyPath(t)
	root := t.TempDir()
	store := openTestStore(t, root)
	createSpecs(t, store,
		specFixture{id: "ticketed", status: state.StatusOpen, ticket: jiraTicket("ACME-1")},
		specFixture{id: "unlinked", status: state.StatusOpen},
		specFixture{id: "bad-ticket", status: state.StatusOpen},
		specFixture{id: "pending-review", status: state.StatusReview, change: "pending-review", ticket: jiraTicket("ACME-3")},
		specFixture{id: "uat-review", status: state.StatusReview, change: "uat-review", ticket: jiraTicket("ACME-4")},
		specFixture{id: "orphan-card", status: state.StatusInProgress, change: "orphan-card", ticket: jiraTicket("ACME-5")},
		specFixture{id: "old-work", status: state.StatusArchived, change: "old-work"},
	)
	writeChange(t, root, "pending-review", "- [x] 1.1 implement", "- [ ] 1.2 wire the command")
	writeChange(t, root, "uat-review", "- [x] 1.1 implement", "- [ ] 2.1 Manual QA: verify on staging")
	writeChange(t, root, "stray-change", "- [ ] 1.1 something")
	writeChange(t, root, "old-work", "- [x] 1.1 done")

	// invalid-state and malformed-ticket-ref are only reachable by a hand edit of
	// state.json (the binary's writers validate both), so corrupt the files directly.
	createSpecs(t, store, specFixture{id: "corrupt", status: state.StatusOpen, ticket: jiraTicket("ACME-6")})
	editState(t, store, "corrupt", func(spec *state.SpecState) { spec.Status = "doing" })
	editState(t, store, "bad-ticket", func(spec *state.SpecState) {
		spec.Ticket = &state.Ticket{Provider: state.TicketJira, Key: " "}
	})

	report, err := Run(store, &config.Config{}, root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cases := []struct {
		specID   string
		kind     DiscrepancyType
		severity Severity
		fix      bool
	}{
		{"corrupt", TypeInvalidState, SeverityHigh, false},
		{"bad-ticket", TypeMalformedTicketRef, SeverityHigh, false},
		{"unlinked", TypeNoTicket, SeverityLow, false},
		{"pending-review", TypeReviewWithPendingTasks, SeverityMedium, false},
		{"stray-change", TypeOpenSpecChangeWithoutCard, SeverityMedium, true},
		{"orphan-card", TypeCardWithoutChange, SeverityLow, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			d, ok := findDiscrepancy(report, tc.specID, tc.kind)
			if !ok {
				t.Fatalf("missing %s for %s in %+v", tc.kind, tc.specID, report.Discrepancies)
			}
			if d.Severity != tc.severity {
				t.Errorf("severity = %s, want %s", d.Severity, tc.severity)
			}
			if d.FixEligible != tc.fix {
				t.Errorf("fixEligible = %v, want %v", d.FixEligible, tc.fix)
			}
			if d.Suggestion == "" {
				t.Error("suggestion must be an exact remediation command")
			}
		})
	}

	if _, ok := findDiscrepancy(report, "uat-review", TypeReviewWithPendingTasks); ok {
		t.Error("a review card whose only pending task is manual QA must not be flagged")
	}
	for _, d := range report.Discrepancies {
		if d.SpecID == "old-work" {
			t.Errorf("archived spec produced a discrepancy: %+v", d)
		}
	}
	for _, s := range report.Specs {
		if s.ID == "old-work" || s.ID == "unlinked" || s.ID == "bad-ticket" {
			t.Errorf("specs[] must hold only non-archived, well-ticketed specs; got %s", s.ID)
		}
	}
	if len(report.Warnings) == 0 {
		t.Error("expected degradation warnings with git/gh absent from PATH")
	}
}

func TestRunNoTicketCollapsesOnTrackerlessBoard(t *testing.T) {
	emptyPath(t)
	root := t.TempDir()
	store := openTestStore(t, root)
	createSpecs(t, store,
		specFixture{id: "alpha", status: state.StatusOpen},
		specFixture{id: "beta", status: state.StatusOpen},
	)

	report, err := Run(store, &config.Config{}, root, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Discrepancies) != 0 {
		t.Errorf("tracker-less board must not emit per-spec no-ticket, got %+v", report.Discrepancies)
	}
	if len(report.Notes) != 1 || report.Notes[0].SpecID != "" {
		t.Errorf("notes = %+v, want one board-level advisory note", report.Notes)
	}
}

func TestRunSkipRemoteSkipsOpenSpecDriftAndGit(t *testing.T) {
	emptyPath(t)
	root := t.TempDir()
	store := openTestStore(t, root)
	createSpecs(t, store, specFixture{id: "orphan-card", status: state.StatusReview, change: "orphan-card", ticket: jiraTicket("ACME-1")})
	writeChange(t, root, "stray-change", "- [ ] 1.1 something")

	report, err := Run(store, &config.Config{}, root, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Discrepancies) != 0 {
		t.Errorf("--skip-remote must run only local checks, got %+v", report.Discrepancies)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("--skip-remote must not probe git/gh (no warnings), got %v", report.Warnings)
	}
}

func TestRunNoOpenSpecTree(t *testing.T) {
	emptyPath(t)
	root := t.TempDir()
	store := openTestStore(t, root)
	createSpecs(t, store, specFixture{id: "synced", status: state.StatusReview, change: "synced", ticket: jiraTicket("ACME-1")})

	report, err := Run(store, &config.Config{}, root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok := findDiscrepancy(report, "synced", TypeCardWithoutChange); ok {
		t.Error("a repo without openspec/changes must produce no OpenSpec drift")
	}
}

func TestRunLanguageNilSafe(t *testing.T) {
	emptyPath(t)
	root := t.TempDir()
	store := openTestStore(t, root)

	report, err := Run(store, nil, root, true)
	if err != nil {
		t.Fatalf("Run(nil cfg): %v", err)
	}
	if report.Language != "" {
		t.Errorf("language = %q, want empty without config", report.Language)
	}
	report, err = Run(store, &config.Config{Language: "es"}, root, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Language != "es" {
		t.Errorf("language = %q, want es", report.Language)
	}
}

func TestFixEligibility(t *testing.T) {
	want := map[DiscrepancyType]bool{
		TypeInvalidState:              false,
		TypeNoTicket:                  false,
		TypeMalformedTicketRef:        false,
		TypeMergedButNotClosed:        true,
		TypeReviewWithoutCode:         true,
		TypeReviewWithUncommittedWork: true,
		TypePRClosedUnmerged:          false,
		TypeReviewWithPendingTasks:    false,
		TypeOpenSpecChangeWithoutCard: true,
		TypeCardWithoutChange:         false,
		TypeBranchWithoutPR:           false,
	}
	for kind, eligible := range want {
		if got := fixEligible(kind); got != eligible {
			t.Errorf("fixEligible(%s) = %v, want %v", kind, got, eligible)
		}
	}
}
