package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mariocampbell/vector/internal/config"
)

const testConfigBody = `{"schemaVersion":1,"specPath":".vector/specs/<slug>/","specStore":"vector","source":"default"}`

// tempWorkspace returns a symlink-resolved temp dir. On macOS t.TempDir() sits
// under /var (a symlink to /private/var), which os.Getwd resolves but
// filepath.Abs does not — comparing unresolved paths would fail spuriously.
func tempWorkspace(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return base
}

// seedStore creates a .vector directory at dir, with a valid config when body is
// non-empty and a stray (config-less) store when it is empty.
func seedStore(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".vector"), 0o755); err != nil {
		t.Fatal(err)
	}
	if body == "" {
		return
	}
	if err := os.WriteFile(config.Path(dir), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRepoRootWalksUpToAncestorStore(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	nested := filepath.Join(base, "website", "src", "components")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	root, strays, err := resolveRepoRootStrays("")
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if root != base {
		t.Errorf("root = %q, want %q", root, base)
	}
	if len(strays) != 0 {
		t.Errorf("strays = %v, want none", strays)
	}
}

func TestResolveRepoRootRejectsExplicitInnerStore(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	nested := filepath.Join(base, "website")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	_, _, err := resolveRepoRootStrays(nested)
	if err == nil || !strings.Contains(err.Error(), "canonical Vector root") {
		t.Fatalf("resolveRepoRootStrays error = %v, want actionable inner-root rejection", err)
	}
}

func TestResolveRepoRootReportsStraysAndFallsBack(t *testing.T) {
	base := tempWorkspace(t)
	// A stray with no valid ancestor above it: the walk finds nothing, reports the
	// stray, and the legacy fallback (git toplevel, else cwd) applies.
	nested := filepath.Join(base, "website")
	seedStore(t, nested, "")
	if err := os.MkdirAll(filepath.Join(nested, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(nested, "src"))

	_, strays, err := resolveRepoRootStrays("")
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if len(strays) != 1 || strays[0] != nested {
		t.Errorf("strays = %v, want [%s]", strays, nested)
	}
}

func TestInitRejectsNestedStoreBelowAncestorEvenWithForce(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	nested := filepath.Join(base, "website")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"--repo-root", nested}, {"--repo-root", nested, "--force"}} {
		err := runInit(args)
		if err == nil || !strings.Contains(err.Error(), "canonical Vector root") {
			t.Errorf("init %v error = %v, want inner-root rejection", args, err)
		}
	}
	if config.Exists(nested) {
		t.Error("init created a nested .vector store")
	}
}

func TestInitAtCanonicalRootDoesNotTripGuard(t *testing.T) {
	// The guard looks strictly ABOVE the target, so re-running init at the store's
	// own root stays on the existing cfgExisted path.
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)

	if err := runInit([]string{"--repo-root", base}); err != nil {
		t.Fatalf("init at the canonical root: %v", err)
	}
}

func TestInitFromWorktreeUsesWorkspaceStoreEvenWithForce(t *testing.T) {
	workspaceRoot := tempWorkspace(t)
	seedStore(t, workspaceRoot, testConfigBody)
	worktree := filepath.Join(workspaceRoot, "code", "main")
	seedStore(t, worktree, testConfigBody)
	t.Chdir(worktree)

	if err := runInit([]string{"--force"}); err != nil {
		t.Fatalf("init --force from worktree: %v", err)
	}
	if !config.Exists(workspaceRoot) {
		t.Error("init did not retain the workspace-root store")
	}
	if !config.Exists(worktree) {
		t.Error("fixture worktree store unexpectedly disappeared")
	}
}

func TestInitFirstRunWithoutAncestorSucceeds(t *testing.T) {
	base := tempWorkspace(t)
	target := filepath.Join(base, "fresh-repo")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runInit([]string{"--repo-root", target}); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if !config.Exists(target) {
		t.Error("first init did not write a config")
	}
}

func TestSpecCreateFromStraySubdirLandsInCanonicalStore(t *testing.T) {
	// End-to-end regression: with a canonical store at the root and a pre-existing
	// stray below it, creating a spec from either place must write to the root
	// store and leave the stray untouched.
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	strayDir := filepath.Join(base, "website")
	seedStore(t, strayDir, "")
	strayBefore := dirEntryNames(t, filepath.Join(strayDir, ".vector"))

	t.Chdir(base)
	if _, err := execCmd(t, newSpecCreateCmd, "--title", "From the root", "--json"); err != nil {
		t.Fatalf("create from the root: %v", err)
	}
	t.Chdir(strayDir)
	if _, err := execCmd(t, newSpecCreateCmd, "--title", "From the stray subdir", "--json"); err != nil {
		t.Fatalf("create from the stray subdir: %v", err)
	}

	for _, slug := range []string{"from-the-root", "from-the-stray-subdir"} {
		if _, err := os.Stat(filepath.Join(base, ".vector", "specs", slug)); err != nil {
			t.Errorf("spec %s did not land in the canonical store: %v", slug, err)
		}
	}
	if got := dirEntryNames(t, filepath.Join(strayDir, ".vector")); !reflect.DeepEqual(got, strayBefore) {
		t.Errorf("stray store was written to: %v, want %v", got, strayBefore)
	}
}

func TestResolveRepoRootEnvVarOverridesWalkUp(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	pinned := tempWorkspace(t)
	nested := filepath.Join(base, "website")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	t.Setenv("VECTOR_REPO_ROOT", pinned)

	root, strays, err := resolveRepoRootStrays("")
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if root != pinned {
		t.Errorf("root = %q, want the env-pinned %q (walk-up must be skipped)", root, pinned)
	}
	if strays != nil {
		t.Errorf("strays = %v, want nil (walk-up skipped entirely)", strays)
	}
}

func TestResolveRepoRootRejectsEnvVarInsideWorkspace(t *testing.T) {
	workspaceRoot := tempWorkspace(t)
	seedStore(t, workspaceRoot, testConfigBody)
	worktree := filepath.Join(workspaceRoot, "code", "main")
	seedStore(t, worktree, testConfigBody)
	t.Chdir(worktree)
	t.Setenv("VECTOR_REPO_ROOT", worktree)

	_, _, err := resolveRepoRootStrays("")
	if err == nil || !strings.Contains(err.Error(), "VECTOR_REPO_ROOT points inside the workspace") {
		t.Fatalf("resolveRepoRootStrays error = %v, want env inner-root rejection", err)
	}
}

func TestResolveRepoRootExplicitFlagBeatsEnvVar(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	t.Setenv("VECTOR_REPO_ROOT", tempWorkspace(t))

	root, _, err := resolveRepoRootStrays(base)
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if root != base {
		t.Errorf("root = %q, want the explicit %q (flag must beat the env var)", root, base)
	}
}

func TestResolveRepoRootStateRootPinsToPersistentTarget(t *testing.T) {
	// A bare+worktree layout: the workspace root carries the canonical store, and
	// the worktree's own tracked config carries a stateRoot pointing back at it.
	workspaceRoot := tempWorkspace(t)
	seedStore(t, workspaceRoot, testConfigBody)
	worktree := filepath.Join(workspaceRoot, "code", "main")
	worktreeCfg := fmt.Sprintf(`{"schemaVersion":1,"specPath":".vector/specs/<slug>/","specStore":"vector","source":"default","stateRoot":%q}`, workspaceRoot)
	seedStore(t, worktree, worktreeCfg)
	nested := filepath.Join(worktree, "cli")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	root, _, err := resolveRepoRootStrays("")
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if root != workspaceRoot {
		t.Errorf("root = %q, want the stateRoot-pinned workspace root %q", root, workspaceRoot)
	}
}

func TestResolveRepoRootStateRootRelativeToItsOwnConfig(t *testing.T) {
	workspaceRoot := tempWorkspace(t)
	seedStore(t, workspaceRoot, testConfigBody)
	worktree := filepath.Join(workspaceRoot, "code", "main")
	// A relative stateRoot resolves against the directory holding this config, not cwd.
	worktreeCfg := `{"schemaVersion":1,"specPath":".vector/specs/<slug>/","specStore":"vector","source":"default","stateRoot":"../.."}`
	seedStore(t, worktree, worktreeCfg)
	t.Chdir(worktree)

	root, _, err := resolveRepoRootStrays("")
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if root != workspaceRoot {
		t.Errorf("root = %q, want %q (relative stateRoot resolved against its own config dir)", root, workspaceRoot)
	}
}

func TestResolveRepoRootStateRootSelfReferenceIgnored(t *testing.T) {
	base := tempWorkspace(t)
	selfPinCfg := fmt.Sprintf(`{"schemaVersion":1,"specPath":".vector/specs/<slug>/","specStore":"vector","source":"default","stateRoot":%q}`, base)
	seedStore(t, base, selfPinCfg)
	t.Chdir(base)

	root, _, err := resolveRepoRootStrays("")
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if root != base {
		t.Errorf("root = %q, want %q (self-referencing stateRoot ignored, falls through to nearest-wins)", root, base)
	}
}

func TestResolveRepoRootStateRootInvalidTargetIgnored(t *testing.T) {
	base := tempWorkspace(t)
	invalidPinCfg := `{"schemaVersion":1,"specPath":".vector/specs/<slug>/","specStore":"vector","source":"default","stateRoot":"/nonexistent-path-does-not-exist"}`
	seedStore(t, base, invalidPinCfg)
	t.Chdir(base)

	root, _, err := resolveRepoRootStrays("")
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if root != base {
		t.Errorf("root = %q, want %q (invalid stateRoot ignored, falls through to nearest-wins)", root, base)
	}
}

func TestResolveRepoRootUsesOutermostAncestorStoreWithoutPin(t *testing.T) {
	// A worktree-local config cannot shadow the workspace root, even without a
	// stateRoot pin.
	workspaceRoot := tempWorkspace(t)
	seedStore(t, workspaceRoot, testConfigBody)
	worktree := filepath.Join(workspaceRoot, "code", "main")
	seedStore(t, worktree, testConfigBody) // no stateRoot pin
	nested := filepath.Join(worktree, "cli")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	root, _, err := resolveRepoRootStrays("")
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if root != workspaceRoot {
		t.Errorf("root = %q, want the workspace root %q", root, workspaceRoot)
	}
}

func TestResolveRepoRootRejectsStateRootInsideWorkspace(t *testing.T) {
	workspaceRoot := tempWorkspace(t)
	worktree := filepath.Join(workspaceRoot, "code", "main")
	seedStore(t, worktree, testConfigBody)
	workspaceCfg := fmt.Sprintf(`{"schemaVersion":1,"specPath":".vector/specs/<slug>/","specStore":"vector","source":"default","stateRoot":%q}`, worktree)
	seedStore(t, workspaceRoot, workspaceCfg)
	t.Chdir(worktree)

	_, _, err := resolveRepoRootStrays("")
	if err == nil || !strings.Contains(err.Error(), "stateRoot points inside the workspace") {
		t.Fatalf("resolveRepoRootStrays error = %v, want stateRoot rejection", err)
	}
}

// dirEntryNames lists a directory's entries, tolerating a missing directory.
func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
