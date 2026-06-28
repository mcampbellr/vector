package main

import (
	"testing"

	"github.com/mariocampbell/vector/internal/config"
)

// runInitQuiet runs runInit with stdout suppressed (it prints a report we don't
// assert on here) and fails the test on error.
func runInitQuiet(t *testing.T, args []string) {
	t.Helper()
	captureStdout(t, func() error { return runInit(args) })
}

func runUpdateQuiet(t *testing.T, args []string) {
	t.Helper()
	captureStdout(t, func() error { return runUpdate(args) })
}

// TestInitLanguageFlag covers `vector init --language`: set, omit, --force
// preservation, and --force overwrite.
func TestInitLanguageFlag(t *testing.T) {
	t.Run("sets language", func(t *testing.T) {
		root := t.TempDir()
		runInitQuiet(t, []string{"--repo-root", root, "--language", "  es  "})
		cfg, err := config.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Language != "es" {
			t.Errorf("Language = %q, want trimmed es", cfg.Language)
		}
	})

	t.Run("omits language without flag", func(t *testing.T) {
		root := t.TempDir()
		runInitQuiet(t, []string{"--repo-root", root})
		cfg, err := config.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Language != "" {
			t.Errorf("Language = %q, want empty", cfg.Language)
		}
	})

	t.Run("force without flag preserves existing", func(t *testing.T) {
		root := t.TempDir()
		runInitQuiet(t, []string{"--repo-root", root, "--language", "es"})
		runInitQuiet(t, []string{"--repo-root", root, "--force"})
		cfg, err := config.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Language != "es" {
			t.Errorf("force re-init dropped language: got %q, want es", cfg.Language)
		}
	})

	t.Run("force with flag overwrites", func(t *testing.T) {
		root := t.TempDir()
		runInitQuiet(t, []string{"--repo-root", root, "--language", "es"})
		runInitQuiet(t, []string{"--repo-root", root, "--force", "--language", "en"})
		cfg, err := config.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Language != "en" {
			t.Errorf("Language = %q, want en", cfg.Language)
		}
	})
}

// TestUpdateLanguageFlag covers `vector update --language`: sets/changes the
// language while preserving the rest of the config; absent, it leaves it as-is.
func TestUpdateLanguageFlag(t *testing.T) {
	root := t.TempDir()
	runInitQuiet(t, []string{"--repo-root", root})
	before, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	runUpdateQuiet(t, []string{"--repo-root", root, "--language", "fr"})
	after, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if after.Language != "fr" {
		t.Errorf("Language = %q, want fr", after.Language)
	}
	if after.SpecPath != before.SpecPath || after.SpecStore != before.SpecStore {
		t.Errorf("update altered unrelated config: %+v vs %+v", after, before)
	}

	// update without --language leaves the configured language untouched.
	runUpdateQuiet(t, []string{"--repo-root", root})
	kept, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Language != "fr" {
		t.Errorf("update without flag cleared language: got %q, want fr", kept.Language)
	}
}
