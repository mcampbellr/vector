package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mariocampbell/vector/internal/ui"
	"github.com/mariocampbell/vector/internal/updatecheck"
	"github.com/mariocampbell/vector/internal/upgrade"
	"github.com/spf13/cobra"
)

// updateResultKey stores the per-invocation *updatecheck.Result in the command
// context so `vector context` reuses it without a second cache read.
type updateResultKey struct{}

// checkForUpdate is the version check; tests swap it out.
var checkForUpdate = updatecheck.Check

// checkForUpdateBeforeRun is the root PersistentPreRunE hook. Released builds
// read the global update cache (refreshed inline at most once a day, bounded to
// ~1s), print a one-line banner to stderr when a newer release exists, and
// stash the result in the command context. On Windows it also removes a `.old` binary left by a
// previous upgrade. A "dev" build does nothing at all.
func checkForUpdateBeforeRun(cmd *cobra.Command) {
	if version == updatecheck.DevVersion {
		return
	}
	if runtime.GOOS == "windows" {
		cleanupStaleWindowsBinary()
	}
	result := checkForUpdate(version)
	if result == nil {
		return
	}
	// `vector upgrade` is the remedy the banner points to; skip it there.
	if result.Available && cmd.Name() != "upgrade" {
		fmt.Fprintln(os.Stderr, ui.Warning(fmt.Sprintf("vector %s available (you have %s) — upgrade with `vector upgrade`", result.Latest, result.Current)))
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, updateResultKey{}, result))
}

// updateInfoFromContext projects the stored result for `vector context --json`:
// present only when an update is available.
func updateInfoFromContext(ctx context.Context) *UpdateInfo {
	if ctx == nil {
		return nil
	}
	result, ok := ctx.Value(updateResultKey{}).(*updatecheck.Result)
	if !ok || result == nil || !result.Available {
		return nil
	}
	return &UpdateInfo{Available: true, Current: result.Current, Latest: result.Latest}
}

// cleanupStaleWindowsBinary best-effort removes `<binary>.exe.old`.
func cleanupStaleWindowsBinary() {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	if realPath, err := filepath.EvalSymlinks(executable); err == nil {
		executable = realPath
	}
	_ = upgrade.CleanupStaleWindowsBinary(executable)
}
