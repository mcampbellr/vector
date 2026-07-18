package main

import (
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

func TestResolveRepoRootExplicitStaysFinal(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	nested := filepath.Join(base, "website")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	// An explicit --repo-root skips the walk-up entirely: precedence is unchanged.
	root, strays, err := resolveRepoRootStrays(nested)
	if err != nil {
		t.Fatalf("resolveRepoRootStrays: %v", err)
	}
	if root != nested {
		t.Errorf("root = %q, want %q", root, nested)
	}
	if strays != nil {
		t.Errorf("strays = %v, want nil for an explicit root", strays)
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

func TestInitRefusesNestedStoreBelowAncestor(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	nested := filepath.Join(base, "website")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	err := runInit([]string{"--repo-root", nested})
	if err == nil {
		t.Fatal("expected init to refuse the nested store")
	}
	if !strings.Contains(err.Error(), base) || !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should name the ancestor and suggest --force, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(nested, ".vector")); statErr == nil {
		t.Error("init wrote a nested .vector despite refusing")
	}
}

func TestInitForceCreatesNestedStore(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	nested := filepath.Join(base, "website")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runInit([]string{"--repo-root", nested, "--force"}); err != nil {
		t.Fatalf("init --force: %v", err)
	}
	if !config.Exists(nested) {
		t.Error("init --force did not create the nested store")
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
