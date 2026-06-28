package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

const (
	rawCommand        = ".claude/commands/vector/raw.md"
	bugCommand        = ".claude/commands/vector/bug.md"
	bugRefiner        = ".claude/agents/vector-bug-refiner.md"
	specComposerAgent = ".claude/agents/vector-spec-composer.md"
)

func TestSeedCommandsCreatesUnderClaude(t *testing.T) {
	root := t.TempDir()

	results, err := SeedCommands(root, SeedOptions{})
	if err != nil {
		t.Fatalf("SeedCommands: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one seeded file")
	}

	target := filepath.Join(root, rawCommand)
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected %s to exist: %v", rawCommand, err)
	}
	if got := actionFor(results, rawCommand); got != ActionCreated {
		t.Fatalf("raw.md action = %q, want %q", got, ActionCreated)
	}
}

func TestSeedCommandsSkipsExistingByDefault(t *testing.T) {
	root := t.TempDir()
	if _, err := SeedCommands(root, SeedOptions{}); err != nil {
		t.Fatalf("first seed: %v", err)
	}

	target := filepath.Join(root, rawCommand)
	if err := os.WriteFile(target, []byte("user edits"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := SeedCommands(root, SeedOptions{})
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if got := actionFor(results, rawCommand); got != ActionSkipped {
		t.Fatalf("action = %q, want %q", got, ActionSkipped)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "user edits" {
		t.Fatalf("user edits were clobbered: %q", got)
	}
}

func TestSeedCommandsForceOverwrites(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, rawCommand)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := SeedCommands(root, SeedOptions{Force: true})
	if err != nil {
		t.Fatalf("force seed: %v", err)
	}
	if got := actionFor(results, rawCommand); got != ActionOverwritten {
		t.Fatalf("action = %q, want %q", got, ActionOverwritten)
	}
	got, _ := os.ReadFile(target)
	if string(got) == "stale" {
		t.Fatal("force did not overwrite the file")
	}
}

func TestSeedCommandsDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()

	results, err := SeedCommands(root, SeedOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run seed: %v", err)
	}
	if got := actionFor(results, rawCommand); got != ActionCreated {
		t.Fatalf("dry-run action = %q, want %q", got, ActionCreated)
	}
	if _, err := os.Stat(filepath.Join(root, rawCommand)); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote a file (stat err = %v)", err)
	}
}

// TestSeedCommandsSeedsBugCommandAndRefiner guards that `vector init` writes the
// /vector:bug command and its Haiku refiner agent — both must be vendored
// (go generate) and embedded so the command never assumes a global skill exists.
func TestSeedCommandsSeedsBugCommandAndRefiner(t *testing.T) {
	root := t.TempDir()

	results, err := SeedCommands(root, SeedOptions{})
	if err != nil {
		t.Fatalf("SeedCommands: %v", err)
	}
	for _, rel := range []string{bugCommand, bugRefiner} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s to be seeded: %v", rel, err)
		}
		if got := actionFor(results, rel); got != ActionCreated {
			t.Fatalf("%s action = %q, want %q", rel, got, ActionCreated)
		}
	}
}

// TestSeedCommandsSeedsSpecComposerAgent guards that `vector init` writes the
// vector-spec-composer agent — it must be vendored (go generate) and embedded so the
// command never assumes a global agent exists in ~/.claude/agents/.
func TestSeedCommandsSeedsSpecComposerAgent(t *testing.T) {
	root := t.TempDir()

	results, err := SeedCommands(root, SeedOptions{})
	if err != nil {
		t.Fatalf("SeedCommands: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, specComposerAgent)); err != nil {
		t.Fatalf("expected %s to be seeded: %v", specComposerAgent, err)
	}
	if got := actionFor(results, specComposerAgent); got != ActionCreated {
		t.Fatalf("%s action = %q, want %q", specComposerAgent, got, ActionCreated)
	}
}

func TestCommandPathsNonEmpty(t *testing.T) {
	paths, err := CommandPaths()
	if err != nil {
		t.Fatalf("CommandPaths: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no embedded commands — go generate vendoring is broken")
	}
}

// TestAssetsMatchKit verifies that every file embedded in assets/ is byte-for-byte
// identical to its counterpart in kit/. If this test fails, run:
//
//	go generate ./internal/scaffold   (from cli/)
//
// then recommit assets/ before merging.
func TestAssetsMatchKit(t *testing.T) {
	t.Helper()

	// Walking from cli/internal/scaffold/, kit/ is three levels up.
	kitRoot := filepath.Join("..", "..", "..", "kit")

	err := fs.WalkDir(assets, embedRoot, func(embPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// embPath is like "assets/agents/vector-bug-refiner.md"
		// strip the "assets/" prefix to get the kit-relative subpath
		rel := embPath[len(embedRoot)+1:] // e.g. "agents/vector-bug-refiner.md"

		embBytes, err := assets.ReadFile(embPath)
		if err != nil {
			t.Errorf("read embedded %s: %v", embPath, err)
			return nil
		}

		kitPath := filepath.Join(kitRoot, filepath.FromSlash(rel))
		kitBytes, err := os.ReadFile(kitPath)
		if err != nil {
			t.Errorf("read kit file %s: %v — does kit/ have this file?", kitPath, err)
			return nil
		}

		if string(embBytes) != string(kitBytes) {
			t.Errorf("assets/%s differs from kit/%s — run `go generate ./internal/scaffold` from cli/", rel, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk assets: %v", err)
	}
}

func actionFor(results []FileResult, repoRelPath string) Action {
	want := filepath.FromSlash(repoRelPath)
	for _, r := range results {
		if r.Path == want {
			return r.Action
		}
	}
	return ""
}
