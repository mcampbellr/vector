package check

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/gitexec"
	"github.com/mariocampbell/vector/internal/state"
)

// git.go holds the card↔branch/PR matching cascade. Every probe degrades to "no
// signal" (plus one stderr warning) instead of aborting the sweep: git missing,
// root not a repo, `git fetch` failing, or `gh` missing/unauthenticated each skip
// only the checks that depend on them.

// ghPRListLimit bounds `gh pr list --state all`; gh's own default (30) would hide
// most PRs of an active repo from the by-id search.
const ghPRListLimit = "1000"

// pullRequest is the subset of `gh pr view|list --json` fields the cascade reads.
type pullRequest struct {
	URL         string `json:"url"`
	Number      int    `json:"number"`
	State       string `json:"state"` // OPEN | MERGED | CLOSED
	HeadRefName string `json:"headRefName"`
}

// worktree is one entry of `git worktree list --porcelain`.
type worktree struct {
	path     string
	prunable bool
}

// gitProbe carries the per-run git/gh facts, resolved once up front (fetch,
// remote heads, worktree list) or lazily once (the PR list).
type gitProbe struct {
	gitRoot string
	base    string
	prefix  string

	gitOK       bool                // git on PATH and gitRoot is a repo
	fresh       bool                // `git fetch origin <base>` succeeded this run
	remoteHeads map[string]struct{} // branch names on origin; nil when unknown
	worktrees   map[string]worktree // checked-out branch → worktree

	ghOK       bool
	prList     []pullRequest
	prListRead bool
	prListOK   bool
}

// newGitProbe resolves the repo to inspect — the bare+worktree container
// (cfg.WorktreeRoot()) when the layout declares one, else root — then fetches the
// base branch once and indexes remote heads and worktrees. Degradations are
// recorded as report warnings.
func newGitProbe(cfg *config.Config, root string, report *CheckReport) *gitProbe {
	p := &gitProbe{
		gitRoot: root,
		base:    cfg.BaseBranchOrDefault(),
		prefix:  cfg.BranchPrefixOrDefault(),
	}
	if cfg.HasBranchPlaceholder() && cfg.WorktreeRoot() != "" {
		p.gitRoot = filepath.Join(root, filepath.FromSlash(cfg.WorktreeRoot()))
	}

	if _, err := exec.LookPath("gh"); err == nil {
		p.ghOK = true
	} else {
		report.warn("gh not found in PATH; skipping PR/merge checks (merged-but-not-closed, pr-closed-unmerged, branch-without-pr, review-without-code)")
	}

	if _, err := exec.LookPath("git"); err != nil {
		report.warn("git not found in PATH; skipping branch, ancestry and worktree checks")
		return p
	}
	if _, err := gitexec.Output(p.gitRoot, "rev-parse", "--git-dir"); err != nil {
		report.warn("%s is not a git repository; skipping branch, ancestry, worktree and PR-search-by-id checks", p.gitRoot)
		return p
	}
	p.gitOK = true
	p.worktrees = listWorktrees(p.gitRoot)

	refspec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", p.base, p.base)
	if _, err := networkGit(p.gitRoot, "fetch", "--quiet", "origin", refspec); err != nil {
		report.warn("git fetch origin %s failed (%v); skipping checks that need fresh refs (remote branches, ancestry vs origin/%s, review-without-code, branch-without-pr)", p.base, err, p.base)
		return p
	}
	p.fresh = true
	if out, err := networkGit(p.gitRoot, "ls-remote", "--heads", "origin"); err == nil {
		p.remoteHeads = parseRemoteHeads(out)
	} else {
		report.warn("git ls-remote origin failed (%v); remote branch existence is unknown", err)
	}
	return p
}

// checkSpec runs the per-card checks: the dirty-worktree signal (any status) and,
// for review cards only, the PR/branch cascade.
func (p *gitProbe) checkSpec(report *CheckReport, spec *state.SpecState) {
	branch := p.prefix + spec.ID
	p.checkWorktree(report, spec, branch)
	if spec.Status != state.StatusReview {
		return
	}

	pr, outcome := p.findPR(report, spec)
	switch outcome {
	case prAmbiguous, prUnknown:
		return
	case prFound:
		classifyPR(report, spec, pr)
		return
	}
	p.classifyBranch(report, spec, branch)
}

// checkWorktree is independent of the PR cascade: a dirty worktree is expected on
// in-progress (Note) and drift on review, where it can coexist with
// branch-without-pr or review-without-code.
func (p *gitProbe) checkWorktree(report *CheckReport, spec *state.SpecState, branch string) {
	if !p.gitOK || (spec.Status != state.StatusInProgress && spec.Status != state.StatusReview) {
		return
	}
	wt, ok := p.worktrees[branch]
	if !ok || wt.prunable {
		return
	}
	if info, err := os.Stat(wt.path); err != nil || !info.IsDir() {
		return // deleted worktree → treated as "no local worktree"
	}
	out, err := gitexec.Output(wt.path, "status", "--porcelain")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return
	}
	if spec.Status == state.StatusInProgress {
		report.note(spec.ID, "in-progress card has an uncommitted worktree at %s (expected)", wt.path)
		return
	}
	report.add(Discrepancy{
		SpecID:      spec.ID,
		Type:        TypeReviewWithUncommittedWork,
		Severity:    SeverityMedium,
		Description: fmt.Sprintf("card is in review but its worktree %s has uncommitted changes", wt.path),
		Suggestion:  fmt.Sprintf("vector spec status %s in-progress", spec.ID),
	})
}

type prOutcome int

const (
	prNone      prOutcome = iota // gh answered: no PR in any state for the card
	prFound                      // exactly one PR (recorded or matched by id)
	prAmbiguous                  // more than one candidate matched the id
	prUnknown                    // gh unavailable/failed: "no PR" cannot be asserted
)

// findPR runs cascade steps 1 and 3: the PR recorded on the card, else a single
// `gh pr list --state all` candidate whose head branch contains the id.
func (p *gitProbe) findPR(report *CheckReport, spec *state.SpecState) (pullRequest, prOutcome) {
	if !p.ghOK {
		return pullRequest{}, prUnknown
	}
	if spec.PR != nil && spec.PR.URL != "" {
		var pr pullRequest
		out, err := p.gh("pr", "view", spec.PR.URL, "--json", "url,number,state,headRefName")
		if err == nil {
			err = json.Unmarshal(out, &pr)
		}
		if err != nil {
			report.note(spec.ID, "could not read the recorded PR %s via gh (%v); PR checks skipped for this card", spec.PR.URL, err)
			return pullRequest{}, prUnknown
		}
		return pr, prFound
	}

	if !p.loadPRList(report) {
		return pullRequest{}, prUnknown
	}
	var candidates []pullRequest
	for _, pr := range p.prList {
		if strings.Contains(pr.HeadRefName, spec.ID) {
			candidates = append(candidates, pr)
		}
	}
	switch len(candidates) {
	case 0:
		return pullRequest{}, prNone
	case 1:
		return candidates[0], prFound
	}
	refs := make([]string, 0, len(candidates))
	for _, pr := range candidates {
		refs = append(refs, fmt.Sprintf("#%d %s (%s)", pr.Number, pr.HeadRefName, pr.State))
	}
	report.note(spec.ID, "ambiguous PR match: %d PRs have a head branch containing %q (%s); record the right one with `vector spec pr %s <url>`", len(candidates), spec.ID, strings.Join(refs, ", "), spec.ID)
	return pullRequest{}, prAmbiguous
}

// loadPRList fetches every PR once per run (lazily: only when a review card
// without a recorded PR needs it).
func (p *gitProbe) loadPRList(report *CheckReport) bool {
	if p.prListRead {
		return p.prListOK
	}
	p.prListRead = true
	if !p.gitOK {
		return false // gh resolves the GitHub repo from git remotes; already warned
	}
	out, err := p.gh("pr", "list", "--state", "all", "--limit", ghPRListLimit, "--json", "url,number,state,headRefName")
	if err == nil {
		err = json.Unmarshal(out, &p.prList)
	}
	if err != nil {
		report.warn("gh pr list failed (%v); skipping PR search by spec id", err)
		return false
	}
	p.prListOK = true
	return true
}

// classifyPR maps a found PR's state onto the review card.
func classifyPR(report *CheckReport, spec *state.SpecState, pr pullRequest) {
	switch pr.State {
	case "MERGED":
		suggestion := fmt.Sprintf("vector spec close %s --resolution done", spec.ID)
		if spec.PR == nil {
			suggestion = fmt.Sprintf("vector spec pr %s %s --number %d && %s", spec.ID, pr.URL, pr.Number, suggestion)
		}
		report.add(Discrepancy{
			SpecID:      spec.ID,
			Type:        TypeMergedButNotClosed,
			Severity:    SeverityHigh,
			Description: fmt.Sprintf("PR #%d (%s) is merged but the card is still %q", pr.Number, pr.URL, spec.Status),
			Suggestion:  suggestion,
		})
	case "CLOSED":
		report.add(Discrepancy{
			SpecID:      spec.ID,
			Type:        TypePRClosedUnmerged,
			Severity:    SeverityHigh,
			Description: fmt.Sprintf("PR #%d (%s) was closed without merging but the card is still %q", pr.Number, pr.URL, spec.Status),
			Suggestion:  fmt.Sprintf("vector spec status %s needs-attention --category decision --summary %q", spec.ID, fmt.Sprintf("PR #%d closed without merging", pr.Number)),
		})
	}
}

// classifyBranch is cascade step 2 + resolution when gh confirmed no PR exists:
// no branch anywhere → review-without-code; a branch not yet in origin/<base> →
// branch-without-pr; a branch already in origin/<base> → Note (manual merge).
func (p *gitProbe) classifyBranch(report *CheckReport, spec *state.SpecState, branch string) {
	if !p.gitOK {
		return
	}
	localRef := "refs/heads/" + branch
	_, localErr := gitexec.Output(p.gitRoot, "rev-parse", "--verify", "--quiet", localRef)
	hasLocal := localErr == nil
	_, hasRemote := p.remoteHeads[branch]

	if !hasLocal && !hasRemote {
		if p.remoteHeads == nil {
			return // remote branches unknown: absence cannot be asserted
		}
		report.add(Discrepancy{
			SpecID:      spec.ID,
			Type:        TypeReviewWithoutCode,
			Severity:    SeverityHigh,
			Description: fmt.Sprintf("card is in review but no PR (any state) and no branch %s (local or on origin) exist", branch),
			Suggestion:  fmt.Sprintf("vector spec status %s needs-attention --category decision --summary %q", spec.ID, "In review but no branch or PR exists"),
		})
		return
	}
	if !p.fresh {
		return
	}

	ref := localRef
	if !hasLocal {
		ref = "refs/remotes/origin/" + branch
		if _, err := networkGit(p.gitRoot, "fetch", "--quiet", "origin", fmt.Sprintf("+refs/heads/%s:%s", branch, ref)); err != nil {
			report.note(spec.ID, "branch %s exists only on origin and could not be fetched (%v); ancestry not verified", branch, err)
			return
		}
	}
	ancestor, err := isAncestor(p.gitRoot, ref, "refs/remotes/origin/"+p.base)
	if err != nil {
		report.note(spec.ID, "could not verify whether %s is merged into origin/%s (%v)", branch, p.base, err)
		return
	}
	if ancestor {
		report.note(spec.ID, "branch %s is already in origin/%s but no PR references it — the code appears merged without a recorded PR; review manually (e.g. `vector spec close %s --resolution done`)", branch, p.base, spec.ID)
		return
	}
	report.add(Discrepancy{
		SpecID:      spec.ID,
		Type:        TypeBranchWithoutPR,
		Severity:    SeverityMedium,
		Description: fmt.Sprintf("branch %s exists and is not an ancestor of origin/%s; no PR (open, merged or closed) references it", branch, p.base),
		Suggestion:  "/vector:ship " + spec.ID,
	})
}

// gh runs `gh <args>` non-interactively from the inspected repo so gh resolves
// the GitHub repo from its remotes.
func (p *gitProbe) gh(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = p.gitRoot
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args[:2], " "), err)
	}
	return out, nil
}

// networkGit runs a git subcommand that talks to the remote with terminal
// credential prompts disabled, so an offline or unauthenticated remote fails fast
// (degrading the sweep) instead of blocking it.
func networkGit(repoRoot string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// isAncestor wraps `git merge-base --is-ancestor`: exit 0 → true, exit 1 → false,
// anything else (unknown ref, missing object) → error.
func isAncestor(repoRoot, ref, base string) (bool, error) {
	cmd := exec.Command("git", "-C", repoRoot, "merge-base", "--is-ancestor", ref, base)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %w", ref, base, err)
}

// parseRemoteHeads reads `git ls-remote --heads` output ("<sha>\trefs/heads/<b>").
func parseRemoteHeads(out []byte) map[string]struct{} {
	heads := map[string]struct{}{}
	for _, line := range strings.Split(string(out), "\n") {
		_, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			heads[name] = struct{}{}
		}
	}
	return heads
}

// listWorktrees indexes `git worktree list --porcelain` by checked-out branch.
func listWorktrees(repoRoot string) map[string]worktree {
	out, err := gitexec.Output(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	byBranch := map[string]worktree{}
	for _, block := range strings.Split(string(out), "\n\n") {
		var (
			wt     worktree
			branch string
		)
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				wt.path = strings.TrimPrefix(line, "worktree ")
			case strings.HasPrefix(line, "branch refs/heads/"):
				branch = strings.TrimPrefix(line, "branch refs/heads/")
			case strings.HasPrefix(line, "prunable"):
				wt.prunable = true
			}
		}
		if branch != "" && wt.path != "" {
			byBranch[branch] = wt
		}
	}
	return byBranch
}
