package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/mariocampbell/vector/internal/ui"
	"github.com/spf13/cobra"
)

// rootLong is the banner shown at the top of `vector --help`, mirroring the first
// line of the old hand-rolled usage().
const rootLong = "vector organizes specs on a kanban board and is the sole writer of Vector's on-disk state. The /vector:* project commands invoke this binary rather than editing state directly."

// errVersionRequested is the sentinel returned by the persistent -v/--version
// handling so Execute() short-circuits with exit 0 after printing the version.
var errVersionRequested = errors.New("version requested")

// errNoCommand is returned by the root's RunE when `vector` is invoked with no
// subcommand (and no version flag); main() maps it to the legacy exit code 2.
var errNoCommand = errors.New("no command")

// newRootCmd builds a fresh command tree per call (not a package-level singleton),
// so tests can execute commands in isolation without shared state. It disables
// cobra's auto --version (a different format from the legacy "vector <version>"),
// wires a persistent -v/--version and an explicit `version` subcommand, installs
// the styled help, and registers every subcommand.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "vector",
		Short:         "developer-focused spec/kanban companion for Claude Code",
		Long:          rootLong,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	// Disable cobra's auto-generated `completion` subcommand; Vector ships its own
	// (completion.go) with the legacy shell set and no hidden default.
	root.CompletionOptions.DisableDefaultCmd = true

	// Hand-rolled version handling. cobra's automatic --version prints
	// "<name> version <v>"; the legacy format is "vector <version>". A persistent
	// bool set anywhere in the invocation prints the legacy line and short-circuits
	// (errVersionRequested → exit 0), matching `version`/`--version`/`-v` in any
	// position.
	var showVersion bool
	root.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "print the vector version and exit")
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if showVersion {
			fmt.Println("vector", version)
			return errVersionRequested
		}
		return nil
	}
	// Runnable root so `vector -v`/`vector --version` (no subcommand) still triggers
	// PersistentPreRunE; without a version flag it is the legacy no-args case.
	root.RunE = func(_ *cobra.Command, _ []string) error {
		return errNoCommand
	}

	root.AddCommand(
		newInitCmd(),
		newUpdateCmd(),
		newContextCmd(),
		newSyncCmd(),
		newServeCmd(),
		newStandupCmd(),
		newSpecCmd(),
		newDetectTicketCmd(),
		newDoctorCmd(),
		newVersionCmd(),
		newCompletionCmd(),
	)

	ui.ApplyCustomHelp(root)
	return root
}

// newVersionCmd is the explicit `vector version` subcommand; it prints the same
// legacy line as the -v/--version flag.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print the vector version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Println("vector", version)
			return nil
		},
	}
}

// isKnownCommand reports whether name is a registered subcommand (or alias, or the
// built-in help command). Used by main() to preserve the legacy "unknown command →
// exit 2" behavior, which cobra would otherwise map to exit 1.
func isKnownCommand(root *cobra.Command, name string) bool {
	for _, c := range root.Commands() {
		if c.Name() == name {
			return true
		}
		for _, alias := range c.Aliases {
			if alias == name {
				return true
			}
		}
	}
	return name == "help"
}

// mapExitError maps the error returned by rootCmd.Execute() to a process exit code
// following the legacy contract (design.md §11): nil / version-requested → 0;
// everything else (business error, unknown subcommand, unknown flag, spec with no
// subverb) → 1. The unknown-top-level-command → 2 case is handled in main() before
// Execute. On a real error the styled message is written to stderr (SilenceErrors
// keeps cobra from printing it itself).
func mapExitError(err error) int {
	switch {
	case err == nil, errors.Is(err, errVersionRequested):
		return 0
	default:
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		return 1
	}
}
