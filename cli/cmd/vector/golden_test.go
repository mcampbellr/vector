package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
	"github.com/spf13/cobra"
)

// updateGolden regenerates the testdata/golden/*.json snapshots. The snapshots
// encode the pre-change --json contract: at migration time each was verified
// byte-identical against the pre-cobra HEAD binary on an identically-seeded repo
// (see the change adopt-cobra-lipgloss-cli). Regenerate only when a --json shape
// intentionally changes; an unintended diff here is the hard gate failing.
var updateGolden = flag.Bool("update-golden", false, "regenerate golden --json snapshots")

// TestJSONGoldenUnchanged runs each --json command against a deterministic repo
// and asserts its stdout is byte-identical to the committed golden file.
//
// Scope note (no silent cap): commands whose --json embeds a wall-clock timestamp
// (standup, summarize projections) or the absolute repo root (init, update — they
// echo t.TempDir() paths) are intentionally excluded from byte-exact goldens; they
// are covered by their own dedicated tests (standup_test.go, summarize_test.go,
// init_language_test.go). Every timestamp/path-free --json surface is covered here.
func TestJSONGoldenUnchanged(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T) string // returns repo root
		factory func() *cobra.Command
		args    func(root string) []string
	}{
		{"spec-list", seedTwoSpecs, newSpecListCmd, func(r string) []string { return []string{"--json", "--repo-root", r} }},
		{"spec-next", seedTwoSpecs, newSpecNextCmd, func(r string) []string { return []string{"--json", "--repo-root", r} }},
		{"context", seedConfiguredRepo, newContextCmd, func(r string) []string { return []string{"--json", "--repo-root", r} }},
		{"context-for-apply", seedConfiguredRepo, newContextCmd, func(r string) []string { return []string{"--json", "--for", "apply", "--repo-root", r} }},
		{"context-for-raw", seedConfiguredRepo, newContextCmd, func(r string) []string { return []string{"--json", "--for", "raw", "--repo-root", r} }},
		{"context-for-status", seedConfiguredRepo, newContextCmd, func(r string) []string { return []string{"--json", "--for", "status", "--repo-root", r} }},
		{"detect-ticket", seedConfiguredRepo, newDetectTicketCmd, func(r string) []string { return []string{"--json", "--text-file", os.DevNull, "--repo-root", r} }},
		{"spec-apply", seedSpecStatus("alpha", state.StatusOpen), newSpecApplyCmd, func(r string) []string { return []string{"alpha", "--json", "--repo-root", r} }},
		{"spec-status", seedSpecStatus("alpha", state.StatusInProgress), newSpecStatusCmd, func(r string) []string { return []string{"alpha", "review", "--json", "--repo-root", r} }},
		{"spec-close", seedSpecStatus("alpha", state.StatusReview), newSpecCloseCmd, func(r string) []string { return []string{"alpha", "--json", "--repo-root", r} }},
		{"spec-archive", seedSpecStatus("alpha", state.StatusClosed), newSpecArchiveCmd, func(r string) []string { return []string{"alpha", "--json", "--repo-root", r} }},
		{"spec-link", seedSpecStatus("alpha", state.StatusOpen), newSpecLinkCmd, func(r string) []string { return []string{"alpha", "jira:ACME-1", "--json", "--repo-root", r} }},
		{"spec-worklog", seedSpecStatus("alpha", state.StatusInProgress), newSpecWorklogCmd, func(r string) []string {
			return []string{"alpha", "--files", "a.go,b.go", "--tasks", "DTO mapper", "--json", "--repo-root", r}
		}},
		{"spec-route", seedSpecStatus("alpha", state.StatusInProgress), newSpecRouteCmd, func(r string) []string {
			return []string{"alpha", "--model", "haiku", "--baseline", "opus", "--tokens-in", "1000", "--tokens-out", "500", "--task", "refine", "--json", "--repo-root", r}
		}},
		{"spec-attach-sketch", seedSketchRepo, newSpecAttachSketchCmd, func(r string) []string {
			return []string{"alpha", "--file", filepath.Join(r, "sketch.excalidraw"), "--json", "--repo-root", r}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			out, err := execCmd(t, tc.factory, tc.args(root)...)
			if err != nil {
				t.Fatalf("%s: command error: %v", tc.name, err)
			}
			// Strip the (random) temp root so the snapshot is path-independent. None of
			// the selected commands echo the root in --json, but this is belt-and-braces.
			out = normalizeGolden(out, root)

			goldenPath := filepath.Join("testdata", "golden", tc.name+".json")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, []byte(out), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s (run `go test -run TestJSONGoldenUnchanged -update-golden` to create): %v", goldenPath, err)
			}
			if out != string(want) {
				t.Fatalf("%s: --json output drifted from golden\n--- got ---\n%s\n--- want ---\n%s", tc.name, out, want)
			}
		})
	}
}

// normalizeGolden strips absolute repo-root occurrences so golden files stay
// portable across the random temp dirs each run uses.
func normalizeGolden(s, root string) string {
	if root == "" {
		return s
	}
	// os.TempDir on macOS may resolve through /private; strip both forms.
	for _, r := range []string{"/private" + root, root} {
		s = strings.ReplaceAll(s, r, "")
	}
	return s
}

// --- deterministic seed helpers -------------------------------------------------

// goldenClock is the fixed timestamp seed helpers stamp so any incidental
// time-derived field stays stable (the selected commands don't emit timestamps,
// but the store still records them internally).
var goldenClock = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// seedConfiguredRepo writes a minimal Go repo with a Vector config and two specs
// so context/detect-ticket have a config to read and a spec.md exists for
// examplePath resolution.
func seedConfiguredRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := config.Write(root, config.Resolve(root)); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeFile(t, root, "go.mod", "module example.com/x\n\ngo 1.26\n")
	writeFile(t, root, "cmd/app/main.go", "package main\nfunc main() {}\n")
	store, err := state.Open(root)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha feature", Status: state.StatusOpen, Actor: "tester", Now: goldenClock}); err != nil {
		t.Fatalf("seed alpha: %v", err)
	}
	return root
}

// seedTwoSpecs writes a config plus two specs (alpha open, beta draft) for
// list/next.
func seedTwoSpecs(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := config.Write(root, config.Resolve(root)); err != nil {
		t.Fatalf("write config: %v", err)
	}
	store, err := state.Open(root)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "alpha", Title: "Alpha feature", Status: state.StatusOpen, Priority: state.PriorityNormal, Actor: "tester", Now: goldenClock}); err != nil {
		t.Fatalf("seed alpha: %v", err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "beta", Title: "Beta", Status: state.StatusDraft, Priority: state.PriorityNormal, Actor: "tester", Now: goldenClock}); err != nil {
		t.Fatalf("seed beta: %v", err)
	}
	return root
}

// seedSpecStatus returns a setup that creates a single spec in the given status.
func seedSpecStatus(id string, status state.Status) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		store, err := state.Open(root)
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		if _, err := store.CreateSpec(state.CreateSpecParams{ID: id, Title: "Alpha feature", Status: status, Priority: state.PriorityNormal, Actor: "tester", Now: goldenClock}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
		return root
	}
}

// seedSketchRepo seeds a spec plus a minimal valid .excalidraw file for
// attach-sketch.
func seedSketchRepo(t *testing.T) string {
	t.Helper()
	root := seedSpecStatus("alpha", state.StatusOpen)(t)
	writeFile(t, root, "sketch.excalidraw", `{"type":"excalidraw","version":2,"source":"test","elements":[]}`)
	return root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}
