package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
)

// fakeGhScript answers `gh pr list` from $FAKE_GH_LIST and `gh pr view <url>`
// from $FAKE_GH_VIEW (a JSON object per line, "<url> <json>"), mimicking the
// `--json` output shape the cascade reads.
const fakeGhScript = `#!/bin/sh
PATH=/usr/bin:/bin
if [ "$1 $2" = "pr list" ]; then cat "$FAKE_GH_LIST"; exit 0; fi
if [ "$1 $2" = "pr view" ]; then
  line=$(grep -F "$3 " "$FAKE_GH_VIEW") || { echo "no pull requests found" >&2; exit 1; }
  echo "${line#* }"; exit 0
fi
exit 1
`

// toolPath builds a PATH holding the real git and, when withGh, the fake gh.
func toolPath(t *testing.T, withGh bool) {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	bin := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	if withGh {
		if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGhScript), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func commitFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", name)
	run(t, dir, "commit", "--quiet", "-m", "add "+name)
}

// newRepoWithOrigin creates a repo on main with one commit pushed to a local bare
// origin. .vector/ is git-ignored so the store never dirties the root worktree.
func newRepoWithOrigin(t *testing.T) (root, origin string) {
	t.Helper()
	base := t.TempDir()
	origin = filepath.Join(base, "origin.git")
	root = filepath.Join(base, "repo")
	run(t, base, "init", "--quiet", "--bare", "--initial-branch=main", origin)
	run(t, base, "init", "--quiet", "--initial-branch=main", root)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".vector/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "add", ".gitignore")
	run(t, root, "commit", "--quiet", "-m", "init")
	run(t, root, "remote", "add", "origin", origin)
	run(t, root, "push", "--quiet", "origin", "main")
	return root, origin
}

func writeGhFixtures(t *testing.T, list, views string) {
	t.Helper()
	dir := t.TempDir()
	listPath, viewPath := filepath.Join(dir, "list.json"), filepath.Join(dir, "views.txt")
	if err := os.WriteFile(listPath, []byte(list), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(viewPath, []byte(views), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_GH_LIST", listPath)
	t.Setenv("FAKE_GH_VIEW", viewPath)
}

func TestGitCascade(t *testing.T) {
	toolPath(t, true)
	root, _ := newRepoWithOrigin(t)

	// merged-into-base: a branch already an ancestor of origin/main, no PR → Note.
	run(t, root, "branch", "feat/manual-merge")
	// unshipped: local branch with an extra commit, no PR → branch-without-pr.
	run(t, root, "checkout", "--quiet", "-b", "feat/unshipped")
	commitFile(t, root, "unshipped.txt")
	// remote-only: pushed branch with an extra commit, no local ref → fetched, then
	// branch-without-pr.
	run(t, root, "checkout", "--quiet", "-b", "feat/remote-only", "main")
	commitFile(t, root, "remote.txt")
	run(t, root, "push", "--quiet", "origin", "feat/remote-only")
	run(t, root, "checkout", "--quiet", "main")
	run(t, root, "branch", "-D", "--quiet", "feat/remote-only")
	// dirty-review: unshipped branch in its own worktree with uncommitted work →
	// branch-without-pr AND review-with-uncommitted-work (independent signals).
	run(t, root, "branch", "feat/dirty-review")
	run(t, root, "-c", "core.hooksPath=/dev/null", "checkout", "--quiet", "feat/dirty-review")
	commitFile(t, root, "dirty.txt")
	run(t, root, "checkout", "--quiet", "main")
	dirtyWT := filepath.Join(t.TempDir(), "dirty-review")
	run(t, root, "worktree", "add", "--quiet", dirtyWT, "feat/dirty-review")
	if err := os.WriteFile(filepath.Join(dirtyWT, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// dirty-wip: in-progress card with a dirty worktree → Note only.
	run(t, root, "branch", "feat/dirty-wip")
	wipWT := filepath.Join(t.TempDir(), "dirty-wip")
	run(t, root, "worktree", "add", "--quiet", wipWT, "feat/dirty-wip")
	if err := os.WriteFile(filepath.Join(wipWT, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeGhFixtures(t,
		`[
  {"url":"https://github.com/o/r/pull/1","number":1,"state":"MERGED","headRefName":"feat/merged-card"},
  {"url":"https://github.com/o/r/pull/2","number":2,"state":"CLOSED","headRefName":"feat/closed-card"},
  {"url":"https://github.com/o/r/pull/3","number":3,"state":"OPEN","headRefName":"feat/open-card"},
  {"url":"https://github.com/o/r/pull/4","number":4,"state":"OPEN","headRefName":"feat/twin"},
  {"url":"https://github.com/o/r/pull/5","number":5,"state":"CLOSED","headRefName":"fix/twin-v2"}
]`,
		`https://github.com/o/r/pull/9 {"url":"https://github.com/o/r/pull/9","number":9,"state":"MERGED","headRefName":"feat/recorded-other-name"}
`)

	store := openTestStore(t, root)
	createSpecs(t, store,
		specFixture{id: "merged-card", status: state.StatusReview},
		specFixture{id: "closed-card", status: state.StatusReview},
		specFixture{id: "open-card", status: state.StatusReview},
		specFixture{id: "twin", status: state.StatusReview},
		specFixture{id: "recorded", status: state.StatusReview},
		specFixture{id: "manual-merge", status: state.StatusReview},
		specFixture{id: "unshipped", status: state.StatusReview},
		specFixture{id: "remote-only", status: state.StatusReview},
		specFixture{id: "no-code", status: state.StatusReview},
		specFixture{id: "dirty-review", status: state.StatusReview},
		specFixture{id: "dirty-wip", status: state.StatusInProgress},
	)
	if _, err := store.RecordPR("recorded", "https://github.com/o/r/pull/9", 9, false, "t", testNow); err != nil {
		t.Fatal(err)
	}

	report, err := Run(store, &config.Config{}, root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", report.Warnings)
	}

	expect := []struct {
		specID     string
		kind       DiscrepancyType
		severity   Severity
		fix        bool
		suggestion string
	}{
		{"merged-card", TypeMergedButNotClosed, SeverityHigh, true, "vector spec pr merged-card https://github.com/o/r/pull/1 --number 1 && vector spec close merged-card --resolution done"},
		{"recorded", TypeMergedButNotClosed, SeverityHigh, true, "vector spec close recorded --resolution done"},
		{"closed-card", TypePRClosedUnmerged, SeverityHigh, false, ""},
		{"no-code", TypeReviewWithoutCode, SeverityHigh, true, ""},
		{"unshipped", TypeBranchWithoutPR, SeverityMedium, false, "/vector:ship unshipped"},
		{"remote-only", TypeBranchWithoutPR, SeverityMedium, false, "/vector:ship remote-only"},
		{"dirty-review", TypeBranchWithoutPR, SeverityMedium, false, "/vector:ship dirty-review"},
		{"dirty-review", TypeReviewWithUncommittedWork, SeverityMedium, true, "vector spec status dirty-review in-progress"},
	}
	for _, e := range expect {
		t.Run(e.specID+"/"+string(e.kind), func(t *testing.T) {
			d, ok := findDiscrepancy(report, e.specID, e.kind)
			if !ok {
				t.Fatalf("missing %s for %s; got %+v / notes %+v", e.kind, e.specID, report.Discrepancies, report.Notes)
			}
			if d.Severity != e.severity || d.FixEligible != e.fix {
				t.Errorf("got severity=%s fix=%v, want %s/%v", d.Severity, d.FixEligible, e.severity, e.fix)
			}
			if e.suggestion != "" && d.Suggestion != e.suggestion {
				t.Errorf("suggestion = %q, want %q", d.Suggestion, e.suggestion)
			}
		})
	}

	// Cards that must produce no discrepancy at all (only notes, or nothing).
	for _, id := range []string{"open-card", "twin", "manual-merge", "dirty-wip"} {
		for _, d := range report.Discrepancies {
			if d.SpecID == id {
				t.Errorf("%s must not produce a discrepancy, got %+v", id, d)
			}
		}
	}
	for _, id := range []string{"twin", "manual-merge", "dirty-wip"} {
		found := false
		for _, n := range report.Notes {
			found = found || n.SpecID == id
		}
		if !found {
			t.Errorf("%s: expected a note, got %+v", id, report.Notes)
		}
	}
	for _, d := range report.Discrepancies {
		if d.Type == TypeBranchWithoutPR && d.FixEligible {
			t.Errorf("branch-without-pr must never be fix-eligible: %+v", d)
		}
	}
}

func TestGitCascadeWithoutGh(t *testing.T) {
	toolPath(t, false)
	root, _ := newRepoWithOrigin(t)
	store := openTestStore(t, root)
	createSpecs(t, store, specFixture{id: "no-code", status: state.StatusReview})

	report, err := Run(store, &config.Config{}, root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Discrepancies) != 0 {
		t.Errorf("without gh, 'no PR' cannot be asserted — got %+v", report.Discrepancies)
	}
	if !strings.Contains(strings.Join(report.Warnings, "\n"), "gh not found") {
		t.Errorf("warnings = %v, want a gh-missing warning", report.Warnings)
	}
}

func TestGitCascadeFetchFailure(t *testing.T) {
	toolPath(t, true)
	root, origin := newRepoWithOrigin(t)
	run(t, root, "checkout", "--quiet", "-b", "feat/unshipped")
	commitFile(t, root, "unshipped.txt")
	run(t, root, "checkout", "--quiet", "main")
	if err := os.RemoveAll(origin); err != nil { // origin unreachable → fetch fails
		t.Fatal(err)
	}
	writeGhFixtures(t, "[]", "")

	store := openTestStore(t, root)
	createSpecs(t, store,
		specFixture{id: "unshipped", status: state.StatusReview},
		specFixture{id: "no-code", status: state.StatusReview},
	)

	report, err := Run(store, &config.Config{}, root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Discrepancies) != 0 {
		t.Errorf("a failed fetch must skip ancestry/absence assertions, got %+v", report.Discrepancies)
	}
	if !strings.Contains(strings.Join(report.Warnings, "\n"), "git fetch origin main failed") {
		t.Errorf("warnings = %v, want a fetch-failure warning", report.Warnings)
	}
}

func TestGitCascadeNotARepo(t *testing.T) {
	toolPath(t, true)
	root := t.TempDir()
	writeGhFixtures(t, "[]", "")
	store := openTestStore(t, root)
	createSpecs(t, store, specFixture{id: "no-code", status: state.StatusReview})

	report, err := Run(store, &config.Config{}, root, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Discrepancies) != 0 {
		t.Errorf("a non-repo root yields no git signal, got %+v", report.Discrepancies)
	}
	if !strings.Contains(strings.Join(report.Warnings, "\n"), "is not a git repository") {
		t.Errorf("warnings = %v, want a not-a-repo warning", report.Warnings)
	}
}

func TestParseRemoteHeads(t *testing.T) {
	heads := parseRemoteHeads([]byte("abc\trefs/heads/main\ndef\trefs/heads/feat/x\nbogus line\n"))
	for _, want := range []string{"main", "feat/x"} {
		if _, ok := heads[want]; !ok {
			t.Errorf("missing head %q in %v", want, heads)
		}
	}
	if len(heads) != 2 {
		t.Errorf("heads = %v, want 2", heads)
	}
}
