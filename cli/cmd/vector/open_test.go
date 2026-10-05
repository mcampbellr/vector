package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
)

// fakeOpenRunner records every external invocation and answers from scripted
// handlers; it never spawns a process, so no test touches real tmux, git, fzf or
// claude. Unscripted calls get the defaults of a machine with no tmux server.
type fakeOpenRunner struct {
	missing     map[string]bool
	listWindows string // list-windows stdout; "" + listErr → no server
	listErr     error
	hasSession  bool
	createOut   string
	worktrees   string // `git worktree list --porcelain` stdout
	addErr      error
	tagErr      error
	refs        map[string]bool // git refs that exist (show-ref)
	repos       map[string]bool // directories git resolves to a repo; nil → every one

	listWorktreesErr error
	pickOut          string
	pickErr          error

	calls       [][]string // Output calls: program + args
	interactive [][]string
	picks       [][]string
}

func newFakeOpenRunner() *fakeOpenRunner {
	return &fakeOpenRunner{
		listErr:   &commandError{Name: "tmux", Stderr: "no server running on /tmp/tmux-1000/default", Err: errors.New("exit status 1")},
		createOut: "vector:@7\n",
	}
}

func (f *fakeOpenRunner) LookPath(file string) (string, error) {
	if f.missing[file] {
		return "", errors.New("executable file not found in $PATH")
	}
	return "/usr/bin/" + file, nil
}

func (f *fakeOpenRunner) Output(name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	joined := strings.Join(args, " ")
	if name == "git" && !f.gitRunsIn(args) {
		// Reproduces the failure that motivated gitRoot: git aborts whenever it
		// is pointed at a directory it cannot resolve to a repository.
		return "", &commandError{Name: "git", Stderr: "fatal: not a git repository (or any parent up to mount point /home)", Err: errors.New("exit status 128")}
	}
	switch {
	case name == "tmux" && args[0] == "list-windows":
		if f.listWindows != "" {
			return f.listWindows, nil
		}
		return "", f.listErr
	case name == "tmux" && args[0] == "has-session":
		if f.hasSession {
			return "", nil
		}
		return "", &commandError{Name: "tmux", Stderr: "can't find session", Err: errors.New("exit status 1")}
	case name == "tmux" && (args[0] == "new-window" || args[0] == "new-session" || args[0] == "split-window"):
		return f.createOut, nil
	case name == "tmux" && args[0] == "set-option":
		return "", f.tagErr
	case name == "git" && strings.Contains(joined, " worktree list"):
		return f.worktrees, f.listWorktreesErr
	case name == "git" && strings.Contains(joined, " worktree add "):
		return "", f.addErr
	case name == "git" && strings.Contains(joined, " show-ref "):
		if f.refs[args[len(args)-1]] {
			return "", nil
		}
		return "", &commandError{Name: "git", Err: errors.New("exit status 1")}
	}
	return "", nil
}

// gitRunsIn reports whether this git invocation's `-C` directory is one of the
// scripted repos. With no scripted repos every directory answers, so tests that
// do not care about git discovery stay unaffected.
func (f *fakeOpenRunner) gitRunsIn(args []string) bool {
	if f.repos == nil {
		return true
	}
	for index, arg := range args {
		if arg == "-C" && index+1 < len(args) {
			return f.repos[args[index+1]]
		}
	}
	return true
}

func (f *fakeOpenRunner) Interactive(name string, args ...string) error {
	f.interactive = append(f.interactive, append([]string{name}, args...))
	return nil
}

func (f *fakeOpenRunner) Pick(input, name string, args ...string) (string, error) {
	f.picks = append(f.picks, append([]string{name, input}, args...))
	return f.pickOut, f.pickErr
}

// calledWith reports whether any Output call was `program sub …`.
func (f *fakeOpenRunner) calledWith(program, sub string) bool {
	for _, call := range f.calls {
		if call[0] == program && len(call) > 1 && strings.Contains(strings.Join(call[1:], " "), sub) {
			return true
		}
	}
	return false
}

// launchedClaude reports whether any tmux call ran Claude as the window command.
// It inspects the last argv element only: temp paths embed the test name.
func (f *fakeOpenRunner) launchedClaude() bool {
	for _, call := range f.calls {
		if call[0] == "tmux" && strings.HasPrefix(call[len(call)-1], "claude ") {
			return true
		}
	}
	return false
}

// openTestEnv is one test's world: the repo root, the fake runner and the deps.
type openTestEnv struct {
	root   string
	home   string
	runner *fakeOpenRunner
	stdout *bytes.Buffer
	deps   openDeps
	env    map[string]string
}

var openTestClock = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

// newOpenTestEnv seeds a repo with the given specs (id → status) and optional
// config, and returns deps wired to the fake runner, a temp HOME and no TTY.
func newOpenTestEnv(t *testing.T, cfg *config.Config, specs map[string]state.Status) *openTestEnv {
	t.Helper()
	root := t.TempDir()
	if cfg != nil {
		if err := config.Write(root, cfg); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}
	store, err := state.Open(root)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	for id, status := range specs {
		if _, err := store.CreateSpec(state.CreateSpecParams{ID: id, Title: "Title of " + id, Status: status, Priority: state.PriorityNormal, Actor: "tester", Now: openTestClock}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	env := &openTestEnv{root: root, home: t.TempDir(), runner: newFakeOpenRunner(), stdout: &bytes.Buffer{}, env: map[string]string{}}
	env.deps = openDeps{
		runner:  env.runner,
		getenv:  func(key string) string { return env.env[key] },
		stdin:   strings.NewReader(""),
		stdout:  env.stdout,
		homeDir: func() (string, error) { return env.home, nil },
		now:     func() time.Time { return openTestClock },
	}
	return env
}

// enterTmux simulates running inside a live tmux client with one untagged window.
func (env *openTestEnv) enterTmux() {
	env.env["TMUX"] = "/tmp/tmux-1000/default,1,0"
	env.runner.listWindows = "main:1 \n"
}

func (env *openTestEnv) run(opts openOptions) error {
	opts.repoRoot = env.root
	return runOpen(opts, env.deps)
}

func worktreeConfig() *config.Config {
	return &config.Config{
		SchemaVersion: config.SchemaVersion,
		SpecPath:      "code/[branch]/.vector/specs/<slug>/",
		SpecFilename:  "spec.md",
		SpecStore:     config.StoreVector,
		BaseBranch:    "main",
		BranchPrefix:  "feat/",
	}
}

func TestNextCommandFor(t *testing.T) {
	pr := &state.PullRequest{URL: "https://github.com/o/r/pull/7", Number: 7}
	tests := []struct {
		name    string
		status  state.Status
		pr      *state.PullRequest
		want    string
		wantHas bool
	}{
		{"open", state.StatusOpen, nil, "/vector:apply demo", true},
		{"in-progress", state.StatusInProgress, nil, "/vector:apply demo", true},
		{"needs-attention", state.StatusNeedsAttention, nil, "/vector:apply demo", true},
		// A spec in review needs shipping until /vector:ship records its PR.
		{"review without a PR", state.StatusReview, nil, "/vector:ship demo", true},
		{"review with a PR", state.StatusReview, pr, "/vector:close demo", true},
		{"closed", state.StatusClosed, nil, "", false},
		{"closed with a PR", state.StatusClosed, pr, "", false},
		{"archived", state.StatusArchived, nil, "", false},
		{"legacy draft", state.StatusLegacyDraft, nil, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, has := nextCommandFor(&state.SpecState{ID: "demo", Status: tc.status, PR: tc.pr})
			if got != tc.want || has != tc.wantHas {
				t.Fatalf("nextCommandFor(%s) = (%q, %v), want (%q, %v)", tc.status, got, has, tc.want, tc.wantHas)
			}
		})
	}
}

func TestBuildTmuxArgs(t *testing.T) {
	claude := claudeShellCommand("/vector:apply demo")
	tests := []struct {
		name string
		spec tmuxWindowSpec
		want []string
	}{
		{
			name: "inside tmux",
			spec: tmuxWindowSpec{insideTmux: true, windowName: "◆ demo", cwd: "/repo", shellCommand: claude},
			want: []string{"new-window", "-P", "-F", tmuxTargetFormat, "-n", "◆ demo", "-c", "/repo", "claude '/vector:apply demo'"},
		},
		{
			name: "outside tmux, new session",
			spec: tmuxWindowSpec{session: "repo", windowName: "◆ demo", cwd: "/repo", shellCommand: claude},
			want: []string{"new-session", "-d", "-P", "-F", tmuxTargetFormat, "-s", "repo", "-n", "◆ demo", "-c", "/repo", "claude '/vector:apply demo'"},
		},
		{
			name: "outside tmux, existing session",
			spec: tmuxWindowSpec{session: "repo", sessionExists: true, windowName: "◆ demo", cwd: "/repo", shellCommand: claude},
			want: []string{"new-window", "-d", "-P", "-F", tmuxTargetFormat, "-t", "=repo:", "-n", "◆ demo", "-c", "/repo", "claude '/vector:apply demo'"},
		},
		{
			name: "split",
			spec: tmuxWindowSpec{insideTmux: true, split: true, windowName: "◆ demo", cwd: "/repo", shellCommand: claude},
			want: []string{"split-window", "-h", "-c", "/repo", "claude '/vector:apply demo'"},
		},
		{
			name: "no claude starts the default shell",
			spec: tmuxWindowSpec{insideTmux: true, windowName: "◆ demo", cwd: "/repo"},
			want: []string{"new-window", "-P", "-F", tmuxTargetFormat, "-n", "◆ demo", "-c", "/repo"},
		},
		{
			name: "custom --cmd is quoted",
			spec: tmuxWindowSpec{insideTmux: true, windowName: "◆ demo", cwd: "/repo", shellCommand: claudeShellCommand("/vector:fix it's")},
			want: []string{"new-window", "-P", "-F", tmuxTargetFormat, "-n", "◆ demo", "-c", "/repo", `claude '/vector:fix it'\''s'`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildTmuxArgs(tc.spec); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("buildTmuxArgs = %#v\nwant %#v", got, tc.want)
			}
		})
	}
}

func TestSpecWindowName(t *testing.T) {
	tests := map[string]string{
		"short":                   "◆ short",
		"exactly-sixteen1":        "◆ exactly-sixteen1",
		"add-vector-open-command": "◆ add-vector-open-…",
	}
	for id, want := range tests {
		if got := specWindowName(id); got != want {
			t.Errorf("specWindowName(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"/vector:apply":  "/vector:apply",
		"a b":            "'a b'",
		"it's":           `'it'\''s'`,
		"$(rm -rf /)":    "'$(rm -rf /)'",
		"":               "''",
		"#{window_id} x": "'#{window_id} x'",
	}
	for input, want := range tests {
		if got := shellQuote(input); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSuggestSpecIDs(t *testing.T) {
	specs := []*state.SpecState{{ID: "add-dark-mode"}, {ID: "add-vector-open-command"}, {ID: "fix-login"}}
	tests := map[string][]string{
		"add-vector":     {"add-vector-open-command"},
		"add-vector-opn": {"add-vector-open-command"},
		"add-nothing":    {"add-dark-mode", "add-vector-open-command"},
		"zzz":            nil,
	}
	for input, want := range tests {
		if got := suggestSpecIDs(specs, input); !reflect.DeepEqual(got, want) {
			t.Errorf("suggestSpecIDs(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestFindSpecWindow(t *testing.T) {
	listing := "main:1 \nmain:2 other-spec\nmy session:3 demo\n"
	if target, found := findSpecWindow(listing, "demo"); !found || target != "my session:3" {
		t.Fatalf("findSpecWindow = (%q, %v), want (my session:3, true)", target, found)
	}
	if _, found := findSpecWindow(listing, "missing"); found {
		t.Fatal("findSpecWindow found a window for an untagged id")
	}
}

func TestOpenInsideTmuxCreatesTaggedWindow(t *testing.T) {
	env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusInProgress})
	env.enterTmux()
	env.runner.createOut = "main:@4\n"

	if err := env.run(openOptions{id: "demo"}); err != nil {
		t.Fatal(err)
	}
	wantCalls := [][]string{
		append([]string{"tmux"}, listSpecWindowsArgs()...),
		{"tmux", "new-window", "-P", "-F", tmuxTargetFormat, "-n", "◆ demo", "-c", env.root, "claude '/vector:apply demo'"},
		{"tmux", "set-option", "-w", "-t", "main:@4", "@vector-spec", "demo"},
	}
	if !reflect.DeepEqual(env.runner.calls, wantCalls) {
		t.Fatalf("calls = %#v\nwant %#v", env.runner.calls, wantCalls)
	}
	if len(env.runner.interactive) != 0 {
		t.Fatalf("inside tmux must not attach, got %v", env.runner.interactive)
	}
	out := env.stdout.String()
	for _, want := range []string{"no worktree, using the repo root", "status in-progress · next /vector:apply demo", "tmux window ◆ demo in session main"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
}

func TestOpenOutsideTmux(t *testing.T) {
	tests := []struct {
		name        string
		hasSession  bool
		wantCreate  string
		wantSession string
	}{
		{"new session", false, "new-session", "-s"},
		{"existing session", true, "new-window", "-t"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusOpen})
			env.runner.hasSession = tc.hasSession
			env.runner.createOut = "repo:@9\n"
			if err := env.run(openOptions{id: "demo"}); err != nil {
				t.Fatal(err)
			}
			var create []string
			for _, call := range env.runner.calls {
				if call[1] == "new-session" || call[1] == "new-window" {
					create = call
				}
			}
			if create == nil || create[1] != tc.wantCreate || !containsArg(create, "-d") || !containsArg(create, tc.wantSession) || containsArg(create, "-A") {
				t.Fatalf("create call = %v, want detached %s without -A", create, tc.wantCreate)
			}
			if !env.runner.calledWith("tmux", "set-option -w -t repo:@9 @vector-spec demo") {
				t.Fatalf("window not tagged: %v", env.runner.calls)
			}
			wantAttach := [][]string{{"tmux", "attach-session", "-t", "repo:@9"}}
			if !reflect.DeepEqual(env.runner.interactive, wantAttach) {
				t.Fatalf("interactive = %v, want %v", env.runner.interactive, wantAttach)
			}
		})
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestOpenFocusesExistingWindow(t *testing.T) {
	t.Run("inside tmux", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusInProgress})
		env.enterTmux()
		env.runner.listWindows = "other:2 demo\n"
		if err := env.run(openOptions{id: "demo"}); err != nil {
			t.Fatal(err)
		}
		if env.runner.calledWith("tmux", "new-window") {
			t.Fatal("an existing window must be focused, not recreated")
		}
		if !env.runner.calledWith("tmux", "switch-client -t other:2") || !env.runner.calledWith("tmux", "select-window -t other:2") {
			t.Fatalf("calls = %v, want switch-client + select-window", env.runner.calls)
		}
		if !strings.Contains(env.stdout.String(), "existing window focused (other:2)") {
			t.Errorf("report = %s", env.stdout.String())
		}
	})
	t.Run("outside tmux", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusInProgress})
		env.runner.listWindows = "repo:3 demo\n"
		if err := env.run(openOptions{id: "demo"}); err != nil {
			t.Fatal(err)
		}
		want := [][]string{{"tmux", "attach-session", "-t", "repo:3"}}
		if !reflect.DeepEqual(env.runner.interactive, want) || env.runner.calledWith("tmux", "new-") {
			t.Fatalf("interactive = %v calls = %v, want only attach", env.runner.interactive, env.runner.calls)
		}
	})
}

func TestOpenResolvesCwd(t *testing.T) {
	t.Run("existing worktree is reused", func(t *testing.T) {
		env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
		worktree := filepath.Join(env.root, "code", "demo")
		if err := os.MkdirAll(worktree, 0o755); err != nil {
			t.Fatal(err)
		}
		env.runner.worktrees = "worktree " + filepath.Join(env.root, "code", "main") + "\n\nworktree " + worktree + "\nbranch refs/heads/feat/demo\n"
		if err := env.run(openOptions{id: "demo"}); err != nil {
			t.Fatal(err)
		}
		if env.runner.calledWith("git", "worktree add") {
			t.Fatal("an existing worktree must not be recreated")
		}
		if !env.runner.calledWith("tmux", "-c "+worktree) {
			t.Fatalf("window cwd is not the worktree: %v", env.runner.calls)
		}
		if !strings.Contains(env.stdout.String(), "worktree code/demo (reused)") {
			t.Errorf("report = %s", env.stdout.String())
		}
	})
	t.Run("non-worktree layout uses the repo root", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusOpen})
		if err := env.run(openOptions{id: "demo"}); err != nil {
			t.Fatal(err)
		}
		if env.runner.calledWith("git", "worktree") {
			t.Fatal("a non-worktree layout must not query git worktrees")
		}
		if !env.runner.calledWith("tmux", "-c "+env.root) {
			t.Fatalf("window cwd is not the repo root: %v", env.runner.calls)
		}
	})
}

func TestOpenCreatesMissingWorktree(t *testing.T) {
	tests := []struct {
		name       string
		opts       openOptions
		tty        bool
		answer     string
		addErr     error
		wantAdd    bool
		wantErr    string
		wantWindow bool
	}{
		{name: "tty yes creates", tty: true, answer: "y\n", wantAdd: true, wantWindow: true},
		{name: "tty default declines", tty: true, answer: "\n", wantErr: "declined"},
		{name: "tty no declines", tty: true, answer: "n\n", wantErr: "declined"},
		{name: "--yes creates without asking", opts: openOptions{yes: true}, wantAdd: true, wantWindow: true},
		{name: "no tty and no --yes errors", wantErr: "--yes"},
		{name: "git failure is verbatim", opts: openOptions{yes: true}, addErr: &commandError{Name: "git", Stderr: "fatal: 'code/demo' already exists", Err: errors.New("exit status 128")}, wantAdd: true, wantErr: "fatal: 'code/demo' already exists"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
			env.deps.stdinTTY = tc.tty
			env.deps.stdin = strings.NewReader(tc.answer)
			env.runner.addErr = tc.addErr
			opts := tc.opts
			opts.id = "demo"
			err := env.run(opts)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			wantArgs := "worktree add " + filepath.Join(env.root, "code", "demo") + " -b feat/demo main"
			if got := env.runner.calledWith("git", wantArgs); got != tc.wantAdd {
				t.Fatalf("worktree add called = %v, want %v (calls %v)", got, tc.wantAdd, env.runner.calls)
			}
			if got := env.runner.calledWith("tmux", "new-"); got != tc.wantWindow {
				t.Fatalf("window created = %v, want %v", got, tc.wantWindow)
			}
		})
	}
}

func TestOpenPrintNeverExecutes(t *testing.T) {
	env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
	if err := env.run(openOptions{id: "demo", printOnly: true}); err != nil {
		t.Fatal(err)
	}
	for _, call := range env.runner.calls {
		switch {
		case call[0] == "git" && containsArg(call, "add"),
			call[0] == "tmux" && (call[1] == "new-window" || call[1] == "new-session" || call[1] == "set-option"):
			t.Fatalf("--print executed %v", call)
		}
	}
	if len(env.runner.interactive) != 0 {
		t.Fatalf("--print attached: %v", env.runner.interactive)
	}
	if _, err := os.Stat(filepath.Join(env.home, ".vector")); !os.IsNotExist(err) {
		t.Fatal("--print wrote active.json")
	}
	lines := strings.Split(strings.TrimSpace(env.stdout.String()), "\n")
	wantPrefixes := []string{
		"git -C " + env.root + " worktree add " + filepath.Join(env.root, "code", "demo") + " -b feat/demo main",
		"WIN=$(tmux new-session -d -P -F '" + tmuxTargetFormat + "' -s ",
		`tmux set-option -w -t "$WIN" @vector-spec demo`,
		`tmux attach-session -t "$WIN"`,
	}
	if len(lines) != len(wantPrefixes) {
		t.Fatalf("--print output = %q", lines)
	}
	for index, prefix := range wantPrefixes {
		if !strings.HasPrefix(lines[index], prefix) {
			t.Errorf("line %d = %q, want prefix %q", index, lines[index], prefix)
		}
	}
}

func TestOpenActiveSpecFile(t *testing.T) {
	tests := []struct {
		name      string
		status    state.Status
		opts      openOptions
		wantWrite bool
	}{
		{name: "claude launched", status: state.StatusInProgress, wantWrite: true},
		{name: "--cmd on a closed spec launches claude", status: state.StatusClosed, opts: openOptions{cmdOverride: "/vector:fix demo"}, wantWrite: true},
		{name: "--no-claude", status: state.StatusInProgress, opts: openOptions{noClaude: true}},
		{name: "--print", status: state.StatusInProgress, opts: openOptions{printOnly: true}},
		{name: "closed", status: state.StatusClosed},
		{name: "archived", status: state.StatusArchived},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": tc.status})
			env.enterTmux()
			opts := tc.opts
			opts.id = "demo"
			if err := env.run(opts); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(env.home, ".vector", filepath.Base(env.root), "active.json")
			body, err := os.ReadFile(path)
			if !tc.wantWrite {
				if err == nil {
					t.Fatalf("active.json written: %s", body)
				}
				return
			}
			if err != nil {
				t.Fatalf("active.json not written: %v", err)
			}
			var got activeSpecFile
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			want := activeSpecFile{ID: "demo", RepoRoot: env.root, SetAt: "2026-09-28T12:00:00Z"}
			if got != want {
				t.Fatalf("active.json = %+v, want %+v", got, want)
			}
		})
	}
}

func TestOpenClosedSpecOpensShellWithNotice(t *testing.T) {
	env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusClosed})
	env.enterTmux()
	env.runner.missing = map[string]bool{"claude": true}
	if err := env.run(openOptions{id: "demo"}); err != nil {
		t.Fatalf("closed spec without claude installed: %v", err)
	}
	if env.runner.launchedClaude() {
		t.Fatalf("closed spec launched claude: %v", env.runner.calls)
	}
	if !strings.Contains(env.stdout.String(), "no next command (closed)") {
		t.Errorf("report = %s", env.stdout.String())
	}
}

// The fixture spec is in review with no recorded PR, so its next command is
// /vector:ship (see nextCommandFor); this test only pins the report's language.
func TestOpenReportFollowsLanguage(t *testing.T) {
	cfg := &config.Config{SchemaVersion: config.SchemaVersion, SpecPath: config.VectorFallbackSpecPath, SpecStore: config.StoreVector, Language: "es"}
	env := newOpenTestEnv(t, cfg, map[string]state.Status{"add-vector-open-command": state.StatusReview})
	env.enterTmux()
	env.runner.createOut = "vector:@2\n"
	if err := env.run(openOptions{id: "add-vector-open-command"}); err != nil {
		t.Fatal(err)
	}
	out := env.stdout.String()
	for _, want := range []string{"sin worktree, uso raíz del repo", "estado review · próximo /vector:ship add-vector-open-command", "tmux ventana ◆ add-vector-open-… en sesión vector"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
}

func TestOpenWithoutID(t *testing.T) {
	specs := map[string]state.Status{"working": state.StatusInProgress, "waiting": state.StatusOpen, "blocked": state.StatusNeedsAttention}
	t.Run("fzf picker selects", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, specs)
		env.enterTmux()
		env.deps.stdinTTY, env.deps.stdoutTTY = true, true
		env.runner.pickOut = "working\tin-progress\tTitle of working\n"
		if err := env.run(openOptions{}); err != nil {
			t.Fatal(err)
		}
		if len(env.runner.picks) != 1 || strings.Contains(env.runner.picks[0][1], "blocked") || strings.Contains(env.runner.picks[0][1], "waiting") {
			t.Fatalf("picker input = %v, want only in-progress/focused specs", env.runner.picks)
		}
		if !env.runner.calledWith("tmux", "set-option -w -t vector:@7 @vector-spec working") {
			t.Fatalf("selected spec not opened: %v", env.runner.calls)
		}
	})
	t.Run("picker cancelled", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, specs)
		env.deps.stdinTTY, env.deps.stdoutTTY = true, true
		env.runner.pickErr = errors.New("exit status 130")
		if err := env.run(openOptions{}); err == nil || !strings.Contains(err.Error(), "no spec selected") {
			t.Fatalf("err = %v, want no spec selected", err)
		}
		if env.runner.calledWith("tmux", "new-") {
			t.Fatal("cancelled picker opened a window")
		}
	})
	t.Run("no fzf lists and fails", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, specs)
		env.deps.stdinTTY, env.deps.stdoutTTY = true, true
		env.runner.missing = map[string]bool{"fzf": true}
		err := env.run(openOptions{})
		if err == nil || !strings.Contains(err.Error(), "pass a spec id") {
			t.Fatalf("err = %v, want a request for the id", err)
		}
		if !strings.Contains(env.stdout.String(), "vector open working") || strings.Contains(env.stdout.String(), "blocked") {
			t.Fatalf("listing = %s", env.stdout.String())
		}
		if len(env.runner.picks) != 0 {
			t.Fatal("picker ran without fzf")
		}
	})
	t.Run("no tty lists and fails", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, specs)
		if err := env.run(openOptions{}); err == nil {
			t.Fatal("want an error without an id")
		}
		if len(env.runner.picks) != 0 {
			t.Fatal("picker ran without a TTY")
		}
	})
}

func TestOpenErrors(t *testing.T) {
	tests := []struct {
		name    string
		opts    openOptions
		tmux    bool
		missing map[string]bool
		wantErr string
	}{
		{name: "tmux missing", opts: openOptions{id: "demo"}, missing: map[string]bool{"tmux": true}, wantErr: "tmux not found"},
		{name: "claude missing", opts: openOptions{id: "demo"}, missing: map[string]bool{"claude": true}, wantErr: "claude not found"},
		{name: "unknown id with suggestion", opts: openOptions{id: "dem"}, wantErr: "spec `dem` not found\ndid you mean: `demo`?"},
		{name: "--split outside tmux", opts: openOptions{id: "demo", split: true}, wantErr: "`--split` requires running inside tmux"},
		{name: "--cmd without slash", opts: openOptions{id: "demo", cmdOverride: "vector:apply demo"}, wantErr: "`--cmd` must start with `/`"},
		{name: "invalid id", opts: openOptions{id: "../etc"}, wantErr: "invalid spec id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusOpen})
			env.runner.missing = tc.missing
			err := env.run(tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
			for _, call := range env.runner.calls {
				// The existing-window lookup is a read-only query that runs
				// before claude is required; nothing else may run.
				if call[0] != "tmux" || call[1] != "list-windows" {
					t.Fatalf("a failed validation invoked %v", call)
				}
			}
			if len(env.runner.interactive) != 0 {
				t.Fatalf("a failed validation attached: %v", env.runner.interactive)
			}
		})
	}
	t.Run("--no-claude does not require claude", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusOpen})
		env.enterTmux()
		env.runner.missing = map[string]bool{"claude": true}
		if err := env.run(openOptions{id: "demo", noClaude: true}); err != nil {
			t.Fatal(err)
		}
		if env.runner.launchedClaude() {
			t.Fatal("--no-claude launched claude")
		}
	})
	t.Run("tmux server not answering inside tmux", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusOpen})
		env.env["TMUX"] = "/tmp/tmux-dead,1,0"
		if err := env.run(openOptions{id: "demo"}); err == nil || !strings.Contains(err.Error(), "tmux is not responding") {
			t.Fatalf("err = %v, want a not-responding error", err)
		}
	})
}

func TestOpenWorktreeSources(t *testing.T) {
	tests := []struct {
		name     string
		refs     map[string]bool
		wantTail string
	}{
		{name: "new spec forks from base", wantTail: "-b feat/demo main"},
		{name: "existing local branch is checked out without -b", refs: map[string]bool{"refs/heads/feat/demo": true}, wantTail: "feat/demo"},
		{name: "remote-only branch is tracked", refs: map[string]bool{"refs/remotes/origin/feat/demo": true}, wantTail: "-b feat/demo --track origin/feat/demo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusInProgress})
			env.runner.refs = tc.refs
			if err := env.run(openOptions{id: "demo", yes: true}); err != nil {
				t.Fatal(err)
			}
			var add []string
			for _, call := range env.runner.calls {
				if call[0] == "git" && containsArg(call, "add") {
					add = call
				}
			}
			want := "worktree add " + filepath.Join(env.root, "code", "demo") + " " + tc.wantTail
			if got := strings.Join(add, " "); !strings.HasSuffix(got, want) {
				t.Fatalf("worktree add = %q, want suffix %q", got, want)
			}
		})
	}
}

func TestResolveGitRoot(t *testing.T) {
	// seed builds a workspace: "dir/" entries are directories, "dir/.git" with a
	// trailing "!" is a gitfile (what a bare+worktree workspace puts beside .bare).
	seed := func(t *testing.T, entries ...string) string {
		t.Helper()
		root := t.TempDir()
		for _, entry := range entries {
			if gitfile, isFile := strings.CutSuffix(entry, "!"); isFile {
				if err := os.MkdirAll(filepath.Join(root, filepath.Dir(gitfile)), 0o755); err != nil {
					t.Fatalf("seed %s: %v", entry, err)
				}
				if err := os.WriteFile(filepath.Join(root, gitfile), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
					t.Fatalf("seed %s: %v", entry, err)
				}
				continue
			}
			if err := os.MkdirAll(filepath.Join(root, entry), 0o755); err != nil {
				t.Fatalf("seed %s: %v", entry, err)
			}
		}
		return root
	}
	// A trailing slash never reaches resolveGitRoot: config.WorktreeRoot() trims it.
	tests := []struct {
		name         string
		entries      []string
		worktreeRoot string
		want         string // relative to the seeded root; "" → the root itself
	}{
		{name: "a board root that is a checkout keeps git in itself", entries: []string{".git", "code/.bare"}, worktreeRoot: "code"},
		{name: "a non-repo workspace root uses the worktree root's gitfile", entries: []string{"code/.git!", "code/.bare"}, worktreeRoot: "code", want: "code"},
		{name: "a worktree root with only a bare repo runs git inside it", entries: []string{"code/.bare"}, worktreeRoot: "code", want: "code/.bare"},
		{name: "a nested worktree root resolves too", entries: []string{"code/worktrees/.bare"}, worktreeRoot: "code/worktrees", want: "code/worktrees/.bare"},
		{name: "a board root beside its own bare repo needs no worktree root", entries: []string{".bare"}, worktreeRoot: "", want: ".bare"},
		{name: "no git anywhere keeps the root, for git's upward discovery", entries: []string{"code"}, worktreeRoot: "code"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := seed(t, tc.entries...)
			want := root
			if tc.want != "" {
				want = filepath.Join(root, filepath.FromSlash(tc.want))
			}
			if got := resolveGitRoot(root, tc.worktreeRoot); got != want {
				t.Fatalf("resolveGitRoot = %q, want %q", got, want)
			}
		})
	}
	t.Run("a worktree root symlinked out of the workspace is never used", func(t *testing.T) {
		root := seed(t)
		elsewhere := t.TempDir()
		if err := os.MkdirAll(filepath.Join(elsewhere, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, filepath.Join(root, "code")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if got := resolveGitRoot(root, "code"); got != root {
			t.Fatalf("resolveGitRoot = %q, want the root %q: a symlinked worktree root must not move git to another repo", got, root)
		}
	})
}

// TestOpenWorktreeWorkspaceRootIsNotARepo covers the bare+worktree workspace
// where the Vector board sits at a non-git root and the repo lives under the
// worktree root. The fake answers git only for the directories scripted as
// repos, so picking the wrong one fails the way real git does.
func TestOpenWorktreeWorkspaceRootIsNotARepo(t *testing.T) {
	// workspace seeds the layout and returns the worktree root and the spec's
	// worktree path; bare adds `code/.bare` instead of the `code/.git` gitfile.
	workspace := func(t *testing.T, env *openTestEnv, bare bool) (string, string) {
		t.Helper()
		code := filepath.Join(env.root, "code")
		if err := os.MkdirAll(code, 0o755); err != nil {
			t.Fatal(err)
		}
		gitRoot := code
		if bare {
			gitRoot = filepath.Join(code, bareRepoDir)
			if err := os.MkdirAll(gitRoot, 0o755); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(filepath.Join(code, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		env.runner.repos = map[string]bool{gitRoot: true}
		return gitRoot, filepath.Join(code, "demo")
	}
	registered := func(gitRoot, worktree string) string {
		return "worktree " + filepath.Join(gitRoot, bareRepoDir) + "\nbare\n\nworktree " + worktree + "\nbranch refs/heads/feat/demo\n"
	}

	t.Run("the worktree root's gitfile is where git runs", func(t *testing.T) {
		env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
		gitRoot, worktree := workspace(t, env, false)
		if err := os.MkdirAll(worktree, 0o755); err != nil {
			t.Fatal(err)
		}
		env.runner.worktrees = registered(gitRoot, worktree)
		if err := env.run(openOptions{id: "demo"}); err != nil {
			t.Fatal(err)
		}
		if !env.runner.calledWith("git", "-C "+gitRoot+" worktree list") {
			t.Fatalf("git did not run in the worktree root: %v", env.runner.calls)
		}
		if env.runner.calledWith("git", "worktree add") {
			t.Fatal("an existing worktree must not be recreated")
		}
		if !env.runner.calledWith("tmux", "-c "+worktree) {
			t.Fatalf("window cwd is not the worktree: %v", env.runner.calls)
		}
	})

	t.Run("a worktree root with only a bare repo runs git inside it", func(t *testing.T) {
		env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
		gitRoot, worktree := workspace(t, env, true)
		if err := os.MkdirAll(worktree, 0o755); err != nil {
			t.Fatal(err)
		}
		env.runner.worktrees = "worktree " + gitRoot + "\nbare\n\nworktree " + worktree + "\nbranch refs/heads/feat/demo\n"
		if err := env.run(openOptions{id: "demo"}); err != nil {
			t.Fatal(err)
		}
		if !env.runner.calledWith("git", "-C "+gitRoot+" worktree list") {
			t.Fatalf("git did not run inside the bare repo: %v", env.runner.calls)
		}
	})

	t.Run("creation also runs in the worktree root", func(t *testing.T) {
		env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
		gitRoot, worktree := workspace(t, env, false)
		if err := env.run(openOptions{id: "demo", yes: true}); err != nil {
			t.Fatal(err)
		}
		if !env.runner.calledWith("git", "-C "+gitRoot+" show-ref") {
			t.Fatalf("ref lookup did not run in the worktree root: %v", env.runner.calls)
		}
		if !env.runner.calledWith("git", "-C "+gitRoot+" worktree add "+worktree+" -b feat/demo main") {
			t.Fatalf("worktree add did not run in the worktree root with an absolute target: %v", env.runner.calls)
		}
	})

	t.Run("no repository anywhere reports git's own failure against the board root", func(t *testing.T) {
		env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
		env.runner.repos = map[string]bool{}
		err := env.run(openOptions{id: "demo", yes: true})
		if err == nil || !strings.Contains(err.Error(), "not a git repository") {
			t.Fatalf("err = %v, want git's own failure", err)
		}
		if !strings.Contains(err.Error(), env.root) {
			t.Fatalf("err = %v, want it to name the board root %q it fell back to", err, env.root)
		}
		if env.runner.calledWith("git", "worktree add") {
			t.Fatal("a failed listing must not be followed by a worktree add")
		}
	})
}

func TestOpenWorktreeGuards(t *testing.T) {
	t.Run("registered but missing directory asks for prune", func(t *testing.T) {
		env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
		env.runner.worktrees = "worktree " + filepath.Join(env.root, "code", "demo") + "\nbranch refs/heads/feat/demo\n"
		err := env.run(openOptions{id: "demo", yes: true})
		if err == nil || !strings.Contains(err.Error(), "worktree prune") {
			t.Fatalf("err = %v, want a prune hint", err)
		}
		if env.runner.calledWith("git", "worktree add") {
			t.Fatal("a stale registration must not trigger a doomed worktree add")
		}
	})
	t.Run("branch checked out elsewhere is reused", func(t *testing.T) {
		env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
		elsewhere := filepath.Join(env.root, "code", "demo-old")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatal(err)
		}
		env.runner.worktrees = "worktree " + elsewhere + "\nbranch refs/heads/feat/demo\n"
		if err := env.run(openOptions{id: "demo"}); err != nil {
			t.Fatal(err)
		}
		if env.runner.calledWith("git", "worktree add") || !env.runner.calledWith("tmux", "-c "+elsewhere) {
			t.Fatalf("calls = %v, want the existing checkout reused", env.runner.calls)
		}
		if !strings.Contains(env.stdout.String(), "worktree code/demo-old (reused)") {
			t.Errorf("report = %s", env.stdout.String())
		}
	})
	t.Run("unregistered directory is not overwritten", func(t *testing.T) {
		env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
		if err := os.MkdirAll(filepath.Join(env.root, "code", "demo"), 0o755); err != nil {
			t.Fatal(err)
		}
		err := env.run(openOptions{id: "demo", yes: true})
		if err == nil || !strings.Contains(err.Error(), "not a registered git worktree") || env.runner.calledWith("git", "worktree add") {
			t.Fatalf("err = %v calls = %v", err, env.runner.calls)
		}
	})
	t.Run("git worktree list failure is surfaced", func(t *testing.T) {
		env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusOpen})
		env.runner.listWorktreesErr = &commandError{Name: "git", Stderr: "fatal: not a git repository", Err: errors.New("exit status 128")}
		err := env.run(openOptions{id: "demo", yes: true})
		if err == nil || !strings.Contains(err.Error(), "fatal: not a git repository") || env.runner.calledWith("git", "worktree add") {
			t.Fatalf("err = %v calls = %v", err, env.runner.calls)
		}
	})
	unsafe := []struct {
		name   string
		mutate func(cfg *config.Config)
		want   string
	}{
		{"tmux format in worktree root", func(cfg *config.Config) { cfg.SpecPath = "#(touch pwned)/[branch]/specs/" }, "worktree root"},
		{"worktree root escapes the repo", func(cfg *config.Config) { cfg.SpecPath = "../[branch]/specs/" }, "inside the repo"},
		{"git option as base branch", func(cfg *config.Config) { cfg.BaseBranch = "--force" }, "baseBranch"},
		{"git option as branch prefix", func(cfg *config.Config) { cfg.BranchPrefix = "-x/" }, "branchPrefix"},
	}
	for _, tc := range unsafe {
		t.Run(tc.name, func(t *testing.T) {
			cfg := worktreeConfig()
			tc.mutate(cfg)
			env := newOpenTestEnv(t, cfg, map[string]state.Status{"demo": state.StatusOpen})
			err := env.run(openOptions{id: "demo", yes: true})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if env.runner.calledWith("git", "worktree") || env.runner.calledWith("tmux", "new-") {
				t.Fatalf("unsafe config reached git/tmux: %v", env.runner.calls)
			}
		})
	}
}

func TestBuildTmuxArgsEscapesFormatInCwd(t *testing.T) {
	for _, spec := range []tmuxWindowSpec{
		{insideTmux: true, windowName: "◆ demo", cwd: "/repo/#(touch x)"},
		{insideTmux: true, split: true, cwd: "/repo/#(touch x)"},
		{session: "repo", cwd: "/repo/#(touch x)"},
	} {
		args := buildTmuxArgs(spec)
		if !containsArg(args, "/repo/##(touch x)") {
			t.Errorf("buildTmuxArgs(%+v) = %v, want the cwd with '#' escaped", spec, args)
		}
	}
}

func TestOpenRejectsMismatchedStateID(t *testing.T) {
	env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusOpen})
	rewriteSpecState(t, env.root, "demo", func(fields map[string]any) { fields["id"] = "#(touch pwned)" })
	err := env.run(openOptions{id: "demo"})
	if err == nil || !strings.Contains(err.Error(), "different id") || len(env.runner.calls) != 0 {
		t.Fatalf("err = %v calls = %v, want a mismatched-id error before any process", err, env.runner.calls)
	}
}

// rewriteSpecState edits a seeded state.json in place, simulating a hand-edited
// or legacy committed file.
func rewriteSpecState(t *testing.T, root, id string, mutate func(map[string]any)) {
	t.Helper()
	path := filepath.Join(root, ".vector", "specs", id, "state.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]any{}
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	mutate(fields)
	if body, err = json.Marshal(fields); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenFocusSkipsClaudeAndWorktree(t *testing.T) {
	env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusInProgress})
	env.enterTmux()
	env.runner.listWindows = "main:2 demo\n"
	env.runner.missing = map[string]bool{"claude": true}
	if err := env.run(openOptions{id: "demo"}); err != nil {
		t.Fatalf("focusing an open window needs neither claude nor the worktree: %v", err)
	}
	if env.runner.calledWith("git", "worktree") {
		t.Fatalf("focus resolved the worktree: %v", env.runner.calls)
	}
}

func TestOpenActiveSpecFailureIsAWarning(t *testing.T) {
	env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusInProgress})
	env.runner.createOut = "repo:@1\n"
	env.deps.homeDir = func() (string, error) { return "", errors.New("$HOME is not defined") }
	if err := env.run(openOptions{id: "demo"}); err != nil {
		t.Fatalf("an active.json failure must not abort after the window exists: %v", err)
	}
	if len(env.runner.interactive) != 1 {
		t.Fatalf("session not attached after the warning: %v", env.runner.interactive)
	}
	if !strings.Contains(env.stdout.String(), "active spec not recorded") {
		t.Errorf("report = %s", env.stdout.String())
	}
}

func TestOpenTagFailureNamesTheWindow(t *testing.T) {
	env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusInProgress})
	env.enterTmux()
	env.runner.createOut = "main:@5\n"
	env.runner.tagErr = &commandError{Name: "tmux", Stderr: "invalid option", Err: errors.New("exit status 1")}
	err := env.run(openOptions{id: "demo"})
	if err == nil || !strings.Contains(err.Error(), "main:@5 was created but could not be tagged") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenLegacyDraftHasNoNextCommand(t *testing.T) {
	env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusOpen})
	rewriteSpecState(t, env.root, "demo", func(fields map[string]any) { fields["status"] = "draft" })
	env.enterTmux()
	if err := env.run(openOptions{id: "demo"}); err != nil {
		t.Fatal(err)
	}
	if env.runner.launchedClaude() {
		t.Fatal("a legacy draft launched claude")
	}
	if _, err := os.Stat(filepath.Join(env.home, ".vector")); !os.IsNotExist(err) {
		t.Fatal("a legacy draft wrote active.json")
	}
}

func TestOpenLeavesVectorStateUntouched(t *testing.T) {
	env := newOpenTestEnv(t, worktreeConfig(), map[string]state.Status{"demo": state.StatusInProgress})
	before := snapshotDir(t, filepath.Join(env.root, ".vector"))
	if err := env.run(openOptions{id: "demo", yes: true}); err != nil {
		t.Fatal(err)
	}
	if after := snapshotDir(t, filepath.Join(env.root, ".vector")); !reflect.DeepEqual(before, after) {
		t.Fatal("vector open modified .vector/")
	}
}

// snapshotDir maps every file under dir to its contents.
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		files[path] = string(body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestOpenPrintVariants(t *testing.T) {
	t.Run("existing window prints the focus commands", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusOpen})
		env.enterTmux()
		env.runner.listWindows = "main:2 demo\n"
		if err := env.run(openOptions{id: "demo", printOnly: true}); err != nil {
			t.Fatal(err)
		}
		want := "tmux switch-client -t main:2\ntmux select-window -t main:2\n"
		if env.stdout.String() != want {
			t.Fatalf("--print = %q, want %q", env.stdout.String(), want)
		}
	})
	t.Run("split prints one untagged command", func(t *testing.T) {
		env := newOpenTestEnv(t, nil, map[string]state.Status{"demo": state.StatusOpen})
		env.enterTmux()
		if err := env.run(openOptions{id: "demo", printOnly: true, split: true}); err != nil {
			t.Fatal(err)
		}
		out := strings.TrimSpace(env.stdout.String())
		if strings.Count(out, "\n") != 0 || !strings.HasPrefix(out, "tmux split-window -h -c ") {
			t.Fatalf("--print --split = %q", out)
		}
	})
	t.Run("session target is zsh-safe", func(t *testing.T) {
		if got := shellQuote("=repo:"); got != "'=repo:'" {
			t.Fatalf("shellQuote(=repo:) = %q", got)
		}
	})
}

func TestRepoSessionName(t *testing.T) {
	tests := map[string]string{"/work/my.repo": "my_repo", "/work/a:b": "a_b", "/": "vector"}
	for root, want := range tests {
		if got := repoSessionName(root); got != want {
			t.Errorf("repoSessionName(%q) = %q, want %q", root, got, want)
		}
	}
}
