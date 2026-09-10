package main

import (
	"strings"
	"testing"

	"github.com/mariocampbell/vector/internal/config"
)

// TestRunConfigSetShip covers incremental per-field writes, idempotency, the
// no-flags error, and the invalid --mode error of `vector config set-ship`.
func TestRunConfigSetShip(t *testing.T) {
	root := t.TempDir()
	if err := config.Write(root, config.Resolve(root)); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	// Error when no configurable flag is given.
	if err := runConfigSetShip([]string{"--repo-root", root}); err == nil {
		t.Error("expected error when no flags are set")
	}

	// Invalid --mode is rejected before writing.
	if err := runConfigSetShip([]string{"--mode", "bogus", "--repo-root", root}); err == nil {
		t.Error("expected error for invalid --mode")
	}

	// Set the base branch only; other fields keep their (unset) values.
	if err := runConfigSetShip([]string{"--base-branch", "develop", "--repo-root", root}); err != nil {
		t.Fatalf("set base-branch: %v", err)
	}
	cfg, _ := config.Load(root)
	if cfg.Ship == nil || cfg.Ship.BaseBranch != "develop" {
		t.Fatalf("base-branch not written: %+v", cfg.Ship)
	}
	if cfg.Ship.Mode != "" {
		t.Errorf("mode should be untouched, got %q", cfg.Ship.Mode)
	}

	// Incrementally set the mode; base branch must survive.
	if err := runConfigSetShip([]string{"--mode", "auto", "--repo-root", root}); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	cfg, _ = config.Load(root)
	if cfg.Ship.BaseBranch != "develop" || cfg.Ship.Mode != config.ShipModeAuto {
		t.Errorf("incremental merge lost a field: %+v", cfg.Ship)
	}

	// Idempotent: re-setting the same mode reports changed:false and writes nothing new.
	out := captureStdout(t, func() error {
		return runConfigSetShip([]string{"--base-branch", "develop", "--repo-root", root, "--json"})
	})
	if !strings.Contains(out, `"changed": false`) {
		t.Errorf("expected changed:false on idempotent re-set, got %s", out)
	}
}

// TestConfigCommandRegistered guards the wiring: `config` and `config set-ship`
// must be reachable through the real dispatch path (exit code ≠ 2 = "unknown
// command"). Regression guard for the release blocker where config.go existed but
// was never hung off newRootCmd, so `vector config set-ship` returned exit 2.
func TestConfigCommandRegistered(t *testing.T) {
	root := t.TempDir()
	if err := config.Write(root, config.Resolve(root)); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	// A valid invocation through dispatch must succeed (exit 0), proving the command
	// is registered and reachable — not routed to the unknown-command exit-2 path.
	if code := dispatch([]string{"config", "set-ship", "--mode", "auto", "--repo-root", root}); code != 0 {
		t.Fatalf("`vector config set-ship` exit code = %d, want 0 (must be a registered command)", code)
	}
	// `vector config` with no subverb is a usage error (exit 1), never exit 2.
	if code := dispatch([]string{"config"}); code != 1 {
		t.Errorf("`vector config` (no subverb) exit code = %d, want 1 (usage error, registered)", code)
	}
}
