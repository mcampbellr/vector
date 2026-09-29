package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/mariocampbell/vector/internal/check"
	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
	"github.com/mariocampbell/vector/internal/ui"
	"github.com/spf13/cobra"
)

// checkDescriptionWidth truncates the human table's description column so a long
// finding does not wrap the table past a normal terminal width.
const checkDescriptionWidth = 90

// newCheckCmd is the read-only reconciliation sweep: it reports where each
// non-archived card's recorded status drifts from the state machine, its ticket's
// shape, git/GitHub (branches, PRs, worktrees) and the OpenSpec changes tree. It
// never contacts Jira and never writes .vector/ — /vector:check layers the Jira
// cross-check, the reporter prose and any confirmed --fix on top of this JSON.
func newCheckCmd() *cobra.Command {
	var (
		repoRoot   string
		jsonOut    bool
		skipRemote bool
	)
	cmd := &cobra.Command{
		Use:   "check",
		Short: "reconcile the board against git, PRs, OpenSpec and the state machine (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCheckBody(repoRoot, jsonOut, skipRemote)
		},
	}
	f := cmd.Flags()
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", true, "emit the CheckReport as JSON (default true)")
	f.BoolVar(&skipRemote, "skip-remote", false, "skip git/gh checks (local state, ticket and tasks checks only)")
	return cmd
}

func runCheckBody(repoRoot string, jsonOut, skipRemote bool) error {
	root, err := resolveRepoRoot(repoRoot)
	if err != nil {
		return err
	}
	store, err := state.Open(root)
	if err != nil {
		return err
	}
	// A repo without .vector/config.json still runs the local checks; it just
	// loses the prose Language and never assumes a branch/worktree layout it did
	// not declare, so git/gh checks are skipped. A present-but-invalid config is
	// a real error and propagates.
	cfg, err := config.Load(root)
	switch {
	case errors.Is(err, os.ErrNotExist):
		fmt.Fprintln(os.Stderr, "warning: no .vector/config.json — running local checks only (no git/gh checks, no language)")
		cfg = &config.Config{}
		skipRemote = true
	case err != nil:
		return err
	}

	report, err := check.Run(store, cfg, root, skipRemote)
	if err != nil {
		return err
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}

	if jsonOut {
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal check report: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}
	printCheckHuman(report)
	return nil
}

// printCheckHuman is the standalone terminal fallback (--json=false): one table
// per severity plus the notes. /vector:check uses --json + the reporter agent.
func printCheckHuman(report check.CheckReport) {
	if len(report.Discrepancies) == 0 {
		fmt.Println(ui.Success("no discrepancies found"))
	}
	for _, severity := range []check.Severity{check.SeverityHigh, check.SeverityMedium, check.SeverityLow} {
		var rows [][]string
		for _, d := range report.Discrepancies {
			if d.Severity == severity {
				rows = append(rows, []string{d.SpecID, string(d.Type), truncate(d.Description, checkDescriptionWidth)})
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Printf("\n%s (%d)\n", ui.Bold(string(severity)), len(rows))
		fmt.Println(ui.Table([]string{"id", "type", "description"}, rows))
	}
	if len(report.Notes) > 0 {
		fmt.Printf("\n%s\n", ui.Bold("notes"))
		for _, n := range report.Notes {
			if n.SpecID != "" {
				fmt.Println(ui.Info(n.SpecID + ": " + n.Description))
			} else {
				fmt.Println(ui.Info(n.Description))
			}
		}
	}
	fmt.Println(ui.Dim("\nRun /vector:check for the full report (Jira cross-check + remediation)."))
}
