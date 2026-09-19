package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mariocampbell/vector/internal/ui"
	"github.com/mariocampbell/vector/internal/upgrade"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// errUpgradeNeedsConfirmation is the non-TTY refusal: replacing the binary is
// destructive, so a script/CI/Claude invocation must pass --yes explicitly.
var errUpgradeNeedsConfirmation = errors.New("refusing to upgrade without confirmation: stdin is not a terminal — re-run with --yes to confirm")

// Injection points for tests (TTY detection, prompt input, the upgrade itself).
var (
	stdinIsTerminal = func() bool {
		return isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	}
	upgradePromptInput io.Reader = os.Stdin
	runUpgrade                   = upgrade.Run
)

// newUpgradeCmd is `vector upgrade`: replace the installed binary with a GitHub
// release after checksum verification. Not to be confused with `vector update`,
// which re-seeds the kit into a repo. The tag flag is --target (not --version):
// the root's persistent -v/--version must keep working in any position.
func newUpgradeCmd() *cobra.Command {
	var (
		assumeYes bool
		target    string
		force     bool
		dryRun    bool
		jsonOut   bool
	)
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "replace the vector binary with the latest (or a given) GitHub release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if target != "" {
				if err := upgrade.ValidateTarget(target); err != nil {
					return err
				}
			}
			interactive := stdinIsTerminal()
			if !assumeYes && !dryRun && !interactive {
				return errUpgradeNeedsConfirmation
			}

			opts := upgrade.Options{Target: target, Force: force, DryRun: dryRun}
			if !assumeYes {
				opts.Confirm = promptUpgradeConfirmation
			}
			if !jsonOut {
				opts.Progress = func(message string) { fmt.Fprintln(os.Stderr, ui.Info(message)) }
			}

			report, err := runUpgrade(cmd.Context(), opts, version)
			if err != nil {
				return err
			}
			if jsonOut {
				payload, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal json: %w", err)
				}
				fmt.Println(string(payload))
				return nil
			}
			printUpgradeReport(report)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&assumeYes, "yes", "y", false, "confirm without prompting (required when stdin is not a terminal)")
	flags.StringVar(&target, "target", "", "release tag to install (default: latest release), e.g. vMAJOR.MINOR.PATCH")
	flags.BoolVar(&force, "force", false, "allow reinstalling the same version or downgrading")
	flags.BoolVar(&dryRun, "dry-run", false, "resolve, download and verify the release without replacing the binary")
	flags.BoolVar(&jsonOut, "json", false, "emit the upgrade report as JSON")
	return cmd
}

// promptUpgradeConfirmation asks y/N on stderr (stdout stays clean for --json).
// Only "y"/"yes" (any case) confirms; empty input or anything else declines.
func promptUpgradeConfirmation(current, target string) (bool, error) {
	fmt.Fprintf(os.Stderr, "Upgrade vector %s -> %s? [y/N]: ", current, target)
	scanner := bufio.NewScanner(upgradePromptInput)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("read confirmation: %w", err)
		}
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}

// printUpgradeReport renders the human outcome.
func printUpgradeReport(report *upgrade.Report) {
	switch {
	case report.DryRun:
		fmt.Println(ui.Success("Dry run: " + report.TargetTag + " is ready to install — nothing was changed."))
		fmt.Println(ui.KeyValue("current", report.CurrentVersion))
		fmt.Println(ui.KeyValue("target", report.TargetTag))
		fmt.Println(ui.KeyValue("asset", report.Asset))
		fmt.Println(ui.KeyValue("platform", report.Platform))
		fmt.Println(ui.KeyValue("checksum", "verified (SHA256)"))
	case report.Cancelled:
		fmt.Println(ui.Info("Upgrade cancelled — nothing was changed."))
	default:
		fmt.Println(ui.Success(fmt.Sprintf("Upgraded vector %s → %s (%s)", report.CurrentVersion, report.TargetTag, report.BinaryPath)))
		if report.BackupPath != "" {
			fmt.Println(ui.KeyValue("backup", report.BackupPath))
		}
	}
}
