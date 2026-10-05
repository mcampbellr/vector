package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
	"github.com/mariocampbell/vector/internal/ui"
	"github.com/spf13/cobra"
)

// openOptions are the flags and positional of `vector open`.
type openOptions struct {
	id          string
	repoRoot    string
	cmdOverride string
	noClaude    bool
	split       bool
	printOnly   bool
	yes         bool
}

// openDeps are the process-level collaborators of `vector open`, injected so
// tests drive the whole flow without a terminal, tmux, git, fzf or claude.
type openDeps struct {
	runner    openRunner
	getenv    func(string) string
	stdin     io.Reader
	stdout    io.Writer
	stdinTTY  bool
	stdoutTTY bool
	homeDir   func() (string, error)
	now       func() time.Time
}

func defaultOpenDeps() openDeps {
	return openDeps{
		runner:    execRunner{},
		getenv:    os.Getenv,
		stdin:     os.Stdin,
		stdout:    os.Stdout,
		stdinTTY:  isTerminal(os.Stdin),
		stdoutTTY: isTerminal(os.Stdout),
		homeDir:   os.UserHomeDir,
		now:       time.Now,
	}
}

// specIDPattern bounds what `vector open` accepts as an id before touching the
// store or building a shell command (no path separators, no leading dot/dash).
var specIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// newOpenCmd is `vector open [id]`: open (or focus) the spec's tmux window, with
// its worktree as cwd, running Claude Code with the spec's next slash command.
// It is read-only on Vector's state; its only repo mutation is the consented
// `git worktree add` when a worktree layout's spec worktree is missing.
func newOpenCmd() *cobra.Command {
	var opts openOptions
	cmd := &cobra.Command{
		Use:   "open [id]",
		Short: "open a spec in its tmux window with Claude running its next command",
		Long: "Open (or focus) a tmux window named after the spec, with cwd in the spec's worktree, " +
			"running Claude Code with the next command for its status (/vector:apply, /vector:close). " +
			"Without an id, pick among in-progress and focused specs (fzf when available).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.id = args[0]
			}
			return runOpen(opts, defaultOpenDeps())
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.cmdOverride, "cmd", "", "slash command to run instead of the computed next one (e.g. /vector:fix <id>)")
	f.BoolVar(&opts.noClaude, "no-claude", false, "open the window with your shell only, without Claude")
	f.BoolVar(&opts.split, "split", false, "split the current tmux window instead of opening a new one (inside tmux only)")
	f.BoolVar(&opts.printOnly, "print", false, "print the commands that would run; only read-only tmux/git queries execute")
	f.BoolVar(&opts.yes, "yes", false, "create the spec's missing worktree without asking")
	f.StringVar(&opts.repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	return cmd
}

// openRun carries the resolved inputs shared by the steps of one invocation.
type openRun struct {
	opts       openOptions
	deps       openDeps
	msg        openMessages
	root       string // Vector board root: state store, display paths, tmux session name
	gitRoot    string // only ever passed to `git -C`; never join paths onto it
	cfg        *config.Config
	spec       *state.SpecState
	insideTmux bool
	slash      string // slash command Claude starts with ("" → no Claude)
	report     []string
	printLines []string
}

func runOpen(opts openOptions, deps openDeps) error {
	insideTmux := deps.getenv("TMUX") != ""
	if opts.cmdOverride != "" && !strings.HasPrefix(opts.cmdOverride, "/") {
		return errors.New("`--cmd` must start with `/` (e.g. `/vector:apply <id>`)")
	}
	if opts.split && !insideTmux {
		return errors.New("`--split` requires running inside tmux (start tmux, or drop --split to open a new window)")
	}
	root, err := resolveRepoRoot(opts.repoRoot)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(filepath.Join(root, ".vector", "specs")); statErr != nil || !info.IsDir() {
		return fmt.Errorf("no Vector board at %s (run `vector init`, or pass --repo-root)", root)
	}
	cfg, err := loadOpenConfig(root)
	if err != nil {
		return err
	}
	store, err := state.Open(root)
	if err != nil {
		return err
	}
	run := &openRun{opts: opts, deps: deps, msg: openMessagesFor(cfg.ResolvedLanguage()), root: root, gitRoot: root, cfg: cfg, insideTmux: insideTmux}

	id := opts.id
	if id == "" {
		if id, err = run.pickSpec(store); err != nil {
			return err
		}
	}
	if !specIDPattern.MatchString(id) {
		return fmt.Errorf("invalid spec id %q", id)
	}
	spec, err := store.ReadSpec(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return specNotFoundError(store, id)
		}
		return err
	}
	// Everything downstream (worktree path, tmux tag, active.json) keys on
	// spec.ID, which comes from a committed file: it must be the validated id.
	if spec.ID != id {
		return fmt.Errorf("state.json of spec %q declares a different id %q: fix the file (e.g. `vector spec list`) before opening it", id, spec.ID)
	}
	run.spec = spec
	return run.execute()
}

// loadOpenConfig reads the repo config; a repo without one behaves as a
// non-worktree layout.
func loadOpenConfig(root string) (*config.Config, error) {
	cfg, err := config.Load(root)
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return &config.Config{}, nil
	}
	return nil, err
}

func (run *openRun) execute() error {
	next, hasNext := nextCommandFor(run.spec)
	if run.opts.cmdOverride != "" {
		next, hasNext = run.opts.cmdOverride, true
	}
	if hasNext && !run.opts.noClaude {
		run.slash = next
	}
	if !run.opts.printOnly {
		if _, err := run.deps.runner.LookPath("tmux"); err != nil {
			return errors.New("tmux not found in PATH: install tmux to use `vector open`")
		}
	}

	// An already-open window is only focused: it needs neither claude nor the
	// worktree, so it is looked up before either is checked or created.
	target, found, err := run.findExistingWindow()
	if err != nil {
		return err
	}
	if found {
		return run.focusExisting(target, next, hasNext)
	}

	if !run.opts.printOnly && run.slash != "" {
		if _, err := run.deps.runner.LookPath("claude"); err != nil {
			return errors.New("claude not found in PATH: install Claude Code, or pass --no-claude to open a plain shell")
		}
	}
	cwd, err := run.resolveCwd()
	if err != nil {
		return err
	}
	return run.openNew(cwd, next, hasNext)
}

// worktreeRefPattern bounds the config-provided branch prefix and base branch
// passed to git: ref-like characters only, so a committed config can never
// smuggle a git option (leading "-") or other syntax into `git worktree add`.
var worktreeRefPattern = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/-]*$`)

// validateWorktreeLayout rejects committed config values that would escape the
// repo or be interpreted by tmux/git: the worktree root must be a relative path
// without ".." segments or tmux format characters ('#'), and the branch prefix
// and base must be ref-like.
func validateWorktreeLayout(worktreeRoot, branchPrefix, base string) error {
	if worktreeRoot == "" || filepath.IsAbs(worktreeRoot) || strings.Contains(worktreeRoot, "#") {
		return fmt.Errorf("unsupported worktree root %q in .vector/config.json: it must be a relative directory without '#'", worktreeRoot)
	}
	for _, segment := range strings.Split(filepath.ToSlash(worktreeRoot), "/") {
		if segment == ".." {
			return fmt.Errorf("unsupported worktree root %q in .vector/config.json: it must stay inside the repo", worktreeRoot)
		}
	}
	if !worktreeRefPattern.MatchString(branchPrefix) {
		return fmt.Errorf("unsupported branchPrefix %q in .vector/config.json", branchPrefix)
	}
	if !worktreeRefPattern.MatchString(base) {
		return fmt.Errorf("unsupported baseBranch %q in .vector/config.json", base)
	}
	return nil
}

// bareRepoDir is the bare repository a bare+worktree workspace keeps beside its
// worktrees (`code/.bare`), as the kit's workspace scaffolding creates it.
const bareRepoDir = ".bare"

// resolveGitRoot returns the directory git subcommands run in on a worktree
// layout. The Vector board can live at the root of a bare+worktree workspace
// that is not itself a repo — the repo is the worktree root, a `code/` holding
// `.bare` plus one worktree per spec — so the board root is kept only when it
// carries git metadata of its own, then the worktree root when it does, then
// that worktree root's bare repo: a `.bare` beside the worktrees is a git
// directory, not a repo top, so git reaches it from inside and never from its
// parent.
//
// Only a directory carrying git metadata itself is accepted, never one git
// would reach by walking up — that is what stops a workspace nested inside an
// unrelated repo from having its spec worktrees created in that repo. With no
// candidate, the board root is returned unchanged, so a board in a subdirectory
// of a plain repo keeps working through git's own upward discovery.
//
// worktreeRoot must have passed validateWorktreeLayout.
func resolveGitRoot(root, worktreeRoot string) string {
	container := root
	if worktreeRoot != "" {
		container = filepath.Join(root, filepath.FromSlash(worktreeRoot))
		// A symlinked worktree root must not move git onto another repository.
		if !isNestedPath(canonicalPath(container), canonicalPath(root)) {
			return root
		}
	}
	if hasGitEntry(root) {
		return root
	}
	if hasGitEntry(container) {
		return container
	}
	if bare := filepath.Join(container, bareRepoDir); isExistingDir(bare) {
		return bare
	}
	return root
}

// hasGitEntry reports whether dir carries git metadata of its own: a ".git"
// directory (a checkout) or ".git" file (a worktree, or the gitfile a
// bare+worktree workspace puts beside its ".bare"). Lstat, so a dangling
// symlink counts as present and git reports the real cause instead of this
// resolver silently falling back.
func hasGitEntry(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil
}

// gitWorktree is one entry of `git worktree list --porcelain`.
type gitWorktree struct {
	path   string
	branch string // full ref (refs/heads/…), "" when detached
}

// parseWorktreeList parses `git worktree list --porcelain` output.
func parseWorktreeList(listing string) []gitWorktree {
	worktrees := make([]gitWorktree, 0)
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimSpace(line)
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			worktrees = append(worktrees, gitWorktree{path: path})
			continue
		}
		if ref, ok := strings.CutPrefix(line, "branch "); ok && len(worktrees) > 0 {
			worktrees[len(worktrees)-1].branch = ref
		}
	}
	return worktrees
}

// resolveCwd returns the window's working directory: on a worktree layout, the
// spec's worktree — reused when git registers one at the path or on the spec
// branch, created with consent when missing — else the repo root.
func (run *openRun) resolveCwd() (string, error) {
	if !run.cfg.HasBranchPlaceholder() {
		run.report = append(run.report, ui.Info(run.msg.worktreeNone))
		return run.root, nil
	}
	id := run.spec.ID
	worktreeRoot := run.cfg.WorktreeRoot()
	branch := run.cfg.BranchPrefixOrDefault() + id
	base := run.cfg.BaseBranchOrDefault()
	if err := validateWorktreeLayout(worktreeRoot, run.cfg.BranchPrefixOrDefault(), base); err != nil {
		return "", err
	}
	run.gitRoot = resolveGitRoot(run.root, worktreeRoot)
	// git runs in gitRoot, which may differ from the board root, so every path
	// handed to git is the absolute `abs`; `rel` exists only for the report,
	// the prompts and the error text the user reads from the board root.
	rel := filepath.ToSlash(filepath.Join(worktreeRoot, id))
	abs := filepath.Join(run.root, filepath.FromSlash(rel))

	listing, err := run.deps.runner.Output("git", "-C", run.gitRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("list git worktrees of %s (the config declares a worktree layout): %s", run.gitRoot, commandStderr(err))
	}
	want := canonicalPath(abs)
	for _, worktree := range parseWorktreeList(listing) {
		if canonicalPath(worktree.path) == want {
			if !isExistingDir(abs) {
				return "", fmt.Errorf("worktree %s is registered in git but its directory is missing: run `git -C %s worktree prune`, then `vector open %s` again; nothing was created", rel, shellQuote(run.gitRoot), id)
			}
			run.report = append(run.report, ui.Success(fmt.Sprintf(run.msg.worktreeReused, rel)))
			return abs, nil
		}
		if worktree.branch == "refs/heads/"+branch && isExistingDir(worktree.path) {
			// The spec branch is already checked out elsewhere: git refuses a
			// second checkout, so that worktree is the spec's worktree.
			run.report = append(run.report, ui.Success(fmt.Sprintf(run.msg.worktreeReused, displayPath(run.root, worktree.path))))
			return worktree.path, nil
		}
	}
	if isExistingDir(abs) {
		return "", fmt.Errorf("%s exists but is not a registered git worktree: move it aside or register it by hand; nothing was created", rel)
	}

	addArgs, source := run.worktreeAddArgs(abs, branch, base)
	if run.opts.printOnly {
		run.printLines = append(run.printLines, renderCommand("git", addArgs))
		return abs, nil
	}
	if !run.opts.yes {
		if !run.deps.stdinTTY {
			return "", fmt.Errorf("worktree %s does not exist: re-run with --yes to create it (%s), or create it by hand; nothing was created", rel, renderCommand("git", addArgs))
		}
		if !run.confirm(fmt.Sprintf(run.msg.createPrompt, rel, source)) {
			return "", errors.New("worktree creation declined: nothing was created")
		}
	}
	if _, err := run.deps.runner.Output("git", addArgs...); err != nil {
		return "", fmt.Errorf("git worktree add failed (nothing was deleted or overwritten):\n%s\nfix: remove or relocate what occupies %s, or run `%s` by hand", commandStderr(err), rel, renderCommand("git", addArgs))
	}
	run.report = append(run.report, ui.Success(fmt.Sprintf(run.msg.worktreeCreated, rel)))
	return abs, nil
}

// worktreeAddArgs picks how to create the spec worktree without losing work:
// an existing local spec branch is checked out as-is, a branch that only exists
// on origin is tracked, and only a brand-new spec forks a branch from base.
// It also returns the localized description of that source for the prompt.
func (run *openRun) worktreeAddArgs(target, branch, base string) ([]string, string) {
	prefix := []string{"-C", run.gitRoot, "worktree", "add", target}
	if run.refExists("refs/heads/" + branch) {
		return append(prefix, branch), fmt.Sprintf(run.msg.sourceLocalBranch, branch)
	}
	remote := "origin/" + branch
	if run.refExists("refs/remotes/" + remote) {
		return append(prefix, "-b", branch, "--track", remote), fmt.Sprintf(run.msg.sourceRemoteBranch, branch, remote)
	}
	return append(prefix, "-b", branch, base), fmt.Sprintf(run.msg.sourceNewBranch, branch, base)
}

// refExists reports whether a git ref exists (read-only query).
func (run *openRun) refExists(ref string) bool {
	_, err := run.deps.runner.Output("git", "-C", run.gitRoot, "show-ref", "--verify", "--quiet", ref)
	return err == nil
}

// displayPath shows path relative to root when it lives inside it.
func displayPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

// confirm asks a [y/N] question on the terminal; anything but yes declines.
func (run *openRun) confirm(question string) bool {
	fmt.Fprint(run.deps.stdout, question)
	answer, _ := bufio.NewReader(run.deps.stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes", "s", "si", "sí":
		return true
	}
	return false
}

// findExistingWindow looks up the window tagged with the spec id. With no tmux
// server running, list-windows fails — that means "no window", except inside
// tmux, where a failure means the server the client belongs to is not answering.
func (run *openRun) findExistingWindow() (string, bool, error) {
	out, err := run.deps.runner.Output("tmux", listSpecWindowsArgs()...)
	if err != nil {
		if run.insideTmux && !run.opts.printOnly {
			return "", false, fmt.Errorf("tmux is not responding although $TMUX is set: %s", commandStderr(err))
		}
		return "", false, nil
	}
	target, found := findSpecWindow(out, run.spec.ID)
	return target, found, nil
}

func (run *openRun) focusExisting(target, next string, hasNext bool) error {
	steps := focusWindowArgs(target, run.insideTmux)
	if run.opts.printOnly {
		for _, step := range steps {
			run.printLines = append(run.printLines, renderCommand("tmux", step))
		}
		return run.flushPrint()
	}
	run.appendStatusReport(next, hasNext)
	run.report = append(run.report, ui.Success(fmt.Sprintf(run.msg.windowFocused, target)))
	if run.insideTmux {
		for _, step := range steps {
			if _, err := run.deps.runner.Output("tmux", step...); err != nil {
				return fmt.Errorf("focus tmux window %s: %s", target, commandStderr(err))
			}
		}
		run.flushReport()
		return nil
	}
	run.flushReport()
	return run.deps.runner.Interactive("tmux", steps[0]...)
}

func (run *openRun) openNew(cwd, next string, hasNext bool) error {
	id := run.spec.ID
	window := tmuxWindowSpec{
		insideTmux: run.insideTmux,
		split:      run.opts.split,
		windowName: specWindowName(id),
		cwd:        cwd,
	}
	if run.slash != "" {
		window.shellCommand = claudeShellCommand(run.slash)
	}
	if !run.insideTmux {
		window.session = repoSessionName(run.root)
		_, err := run.deps.runner.Output("tmux", hasSessionArgs(window.session)...)
		window.sessionExists = err == nil
	}
	createArgs := buildTmuxArgs(window)

	if run.opts.printOnly {
		if window.split {
			run.printLines = append(run.printLines, renderCommand("tmux", createArgs))
			return run.flushPrint()
		}
		run.printLines = append(run.printLines,
			renderCapture("WIN", "tmux", createArgs),
			fmt.Sprintf(`tmux set-option -w -t "$WIN" %s %s`, specWindowOption, shellQuote(id)))
		if !run.insideTmux {
			run.printLines = append(run.printLines, `tmux attach-session -t "$WIN"`)
		}
		return run.flushPrint()
	}

	out, err := run.deps.runner.Output("tmux", createArgs...)
	if err != nil {
		return fmt.Errorf("create tmux window: %s", commandStderr(err))
	}
	target := strings.TrimSpace(out)
	if !window.split {
		if _, err := run.deps.runner.Output("tmux", tagWindowArgs(target, id)...); err != nil {
			return fmt.Errorf("window %s was created but could not be tagged (a later `vector open %s` will not find it; tag it with `%s`): %s", target, id, renderCommand("tmux", tagWindowArgs(target, id)), commandStderr(err))
		}
	}
	// The window already runs Claude here: a failed active.json write is
	// reported, never allowed to abort the attach and orphan the session.
	if run.slash != "" {
		if display, err := writeActiveSpec(run.deps, run.root, id); err != nil {
			run.report = append(run.report, ui.Warning(fmt.Sprintf(run.msg.activeSpecFailed, err)))
		} else {
			run.report = append(run.report, ui.Success(fmt.Sprintf(run.msg.activeSpec, display)))
		}
	}
	run.appendStatusReport(next, hasNext)
	if window.split {
		run.report = append(run.report, ui.Success(run.msg.paneSplit))
	} else {
		run.report = append(run.report, ui.Success(fmt.Sprintf(run.msg.windowCreated, window.windowName, sessionOfTarget(target))))
	}
	run.flushReport()
	if run.insideTmux {
		return nil
	}
	return run.deps.runner.Interactive("tmux", attachArgs(target)...)
}

// appendStatusReport adds the "status · next" line, plus the extra
// "no next command" line for closed/archived/draft specs without --cmd.
func (run *openRun) appendStatusReport(next string, hasNext bool) {
	status := string(run.spec.Status)
	switch {
	case !hasNext:
		run.report = append(run.report,
			ui.Success(fmt.Sprintf(run.msg.statusOnly, status)),
			ui.Info(fmt.Sprintf(run.msg.noNextCommand, status)))
	case run.opts.noClaude:
		run.report = append(run.report, ui.Success(fmt.Sprintf(run.msg.statusNoClaude, status, next)))
	default:
		run.report = append(run.report, ui.Success(fmt.Sprintf(run.msg.statusNext, status, next)))
	}
}

func (run *openRun) flushReport() {
	for _, line := range run.report {
		fmt.Fprintln(run.deps.stdout, line)
	}
}

// flushPrint writes the --print dry-run: only the commands, ready to copy.
func (run *openRun) flushPrint() error {
	for _, line := range run.printLines {
		fmt.Fprintln(run.deps.stdout, line)
	}
	return nil
}

// pickSpec resolves the id when none was passed: an fzf picker over the
// in-progress and focused specs when fzf and a terminal are available, else the
// list of candidates and an error asking for the id.
func (run *openRun) pickSpec(store *state.Store) (string, error) {
	specs, err := store.ListSpecs()
	if err != nil {
		return "", err
	}
	epics, err := store.ListEpics()
	if err != nil {
		return "", err
	}
	index := state.NewEpicIndex(epics)
	candidates := make([]*state.SpecState, 0)
	for _, spec := range specs {
		if spec.Status.IsTerminal() {
			continue
		}
		if spec.Status == state.StatusInProgress || index.EffectiveFocus(spec) {
			candidates = append(candidates, spec)
		}
	}
	if len(candidates) == 0 {
		return "", errors.New("no in-progress or focused specs to pick from: pass a spec id (`vector open <id>`)")
	}

	_, fzfErr := run.deps.runner.LookPath("fzf")
	if fzfErr == nil && run.deps.stdinTTY && run.deps.stdoutTTY {
		var input strings.Builder
		for _, spec := range candidates {
			fmt.Fprintf(&input, "%s\t%s\t%s\n", spec.ID, spec.Status, spec.Title)
		}
		out, err := run.deps.runner.Pick(input.String(), "fzf", "--delimiter", "\t", "--prompt", "vector open> ", "--height", "40%", "--reverse")
		selected, _, _ := strings.Cut(strings.TrimSpace(out), "\t")
		if err != nil || selected == "" {
			return "", errors.New("no spec selected: nothing was opened")
		}
		return selected, nil
	}

	fmt.Fprintln(run.deps.stdout, run.msg.pickHeader)
	for _, spec := range candidates {
		fmt.Fprintf(run.deps.stdout, "  vector open %-40s %-16s %s\n", spec.ID, spec.Status, spec.Title)
	}
	return "", errors.New("pass a spec id: `vector open <id>`")
}

// specNotFoundError reports an unknown id with prefix-based suggestions.
func specNotFoundError(store *state.Store, id string) error {
	message := fmt.Sprintf("spec `%s` not found", id)
	specs, err := store.ListSpecs()
	if err != nil {
		return errors.New(message)
	}
	suggestions := suggestSpecIDs(specs, id)
	if len(suggestions) == 0 {
		return errors.New(message)
	}
	quoted := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		quoted = append(quoted, "`"+suggestion+"`")
	}
	return fmt.Errorf("%s\ndid you mean: %s?", message, strings.Join(quoted, ", "))
}

// maxIDSuggestions caps the "did you mean" list.
const maxIDSuggestions = 5

// suggestSpecIDs returns ids starting with the given one; failing that, ids
// sharing its longest leading run of kebab segments (e.g. "add-vector-" for
// "add-vector-opn", before falling back to "add-").
func suggestSpecIDs(specs []*state.SpecState, id string) []string {
	match := func(prefix string) []string {
		found := make([]string, 0)
		for _, spec := range specs {
			if strings.HasPrefix(spec.ID, prefix) && len(found) < maxIDSuggestions {
				found = append(found, spec.ID)
			}
		}
		return found
	}
	if found := match(id); len(found) > 0 {
		return found
	}
	for cut := strings.LastIndex(id, "-"); cut > 0; cut = strings.LastIndex(id[:cut], "-") {
		if found := match(id[:cut+1]); len(found) > 0 {
			return found
		}
	}
	return nil
}

// activeSpecFile is ~/.vector/<repo-id>/active.json: the developer's personal
// active spec, outside the repo and outside Vector's store.
type activeSpecFile struct {
	ID       string `json:"id"`
	RepoRoot string `json:"repoRoot"`
	SetAt    string `json:"setAt"`
}

// writeActiveSpec atomically writes active.json (temp file + rename) and returns
// the "~/…" path for the report.
func writeActiveSpec(deps openDeps, root, id string) (string, error) {
	home, err := deps.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for active.json: %w", err)
	}
	repoID := filepath.Base(root)
	dir := filepath.Join(home, ".vector", repoID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	body, err := json.MarshalIndent(activeSpecFile{ID: id, RepoRoot: root, SetAt: deps.now().UTC().Format(time.RFC3339)}, "", "  ")
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".active-*.json")
	if err != nil {
		return "", fmt.Errorf("write active.json: %w", err)
	}
	tmpPath := tmp.Name()
	_, writeErr := tmp.Write(append(body, '\n'))
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write active.json: %w", errors.Join(writeErr, closeErr))
	}
	path := filepath.Join(dir, "active.json")
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write active.json: %w", err)
	}
	return filepath.ToSlash(filepath.Join("~", ".vector", repoID, "active.json")), nil
}

// commandStderr extracts an external command's stderr verbatim when available.
func commandStderr(err error) string {
	var cmdErr *commandError
	if errors.As(err, &cmdErr) && strings.TrimSpace(cmdErr.Stderr) != "" {
		return strings.TrimSpace(cmdErr.Stderr)
	}
	return err.Error()
}

func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// canonicalPath resolves symlinks when possible so git's listed paths compare
// equal to the configured ones. A path whose directory was deleted is resolved
// through its parent, so a listed-but-missing worktree still matches.
func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	if parent, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		return filepath.Join(parent, filepath.Base(path))
	}
	return filepath.Clean(path)
}
