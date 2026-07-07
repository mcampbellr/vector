package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// execCmd drives a cobra command factory the way the real dispatch does: it builds
// a fresh command, silences cobra's own error/usage printing (so the returned error
// is the sole signal, mirroring newRootCmd), sets the args, and executes. It
// captures os.Stdout across the run — RunE writes to os.Stdout directly (a
// deliberate design choice keeping the --json contract independent of cobra
// wiring), so nothing is lost. It returns the captured stdout and the run error.
//
// The per-command shims below drive their factory through execCmd, preserving the
// exact runXxx(args) signatures the existing tests call. This is the harness the
// test suite migrated onto: every command is exercised through the cobra factory,
// not the deleted hand-rolled dispatch.
func execCmd(t *testing.T, newCmd func() *cobra.Command, args ...string) (string, error) {
	t.Helper()
	cmd := newCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	runErr := cmd.Execute()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), runErr
}

// runViaCmd executes a factory with args and returns only the run error, letting an
// outer captureStdout redirect stdout. It is the plumbing behind the runXxx shims.
func runViaCmd(newCmd func() *cobra.Command, args []string) error {
	cmd := newCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	return cmd.Execute()
}

// runXxx shims: same signatures the pre-cobra tests used, now driving the cobra
// factories. They keep the ~11 command test files unchanged.

func runInit(args []string) error     { return runViaCmd(newInitCmd, args) }
func runUpdate(args []string) error   { return runViaCmd(newUpdateCmd, args) }
func runContext(args []string) error  { return runViaCmd(newContextCmd, args) }
func runSync(args []string) error     { return runViaCmd(newSyncCmd, args) }
func runStandup(args []string) error  { return runViaCmd(newStandupCmd, args) }
func runSpecFix(args []string) error  { return runViaCmd(newSpecFixCmd, args) }
func runSpecLink(args []string) error { return runViaCmd(newSpecLinkCmd, args) }
func runSpecPR(args []string) error   { return runViaCmd(newSpecPRCmd, args) }

// runStandupCommit drives the standup `commit` child.
func runStandupCommit(args []string) error {
	return runViaCmd(newStandupCmd, append([]string{"commit"}, args...))
}

// runSpecAttachSketch drives the attach-sketch factory.
func runSpecAttachSketch(args []string) error {
	return runViaCmd(newSpecAttachSketchCmd, args)
}

// runSpecSummarize drives the single summarize command (projection or either
// commit ordering — the RunE detects "commit" among the positionals).
func runSpecSummarize(args []string) error {
	return runViaCmd(newSpecSummarizeCmd, args)
}

// runSpecSummarizeCommit preserves the old (presetID, args) entrypoint: presetID
// != "" reproduces the `summarize <id> commit …` ordering; presetID == "" the
// `summarize commit …` ordering (the kit commands use this shape).
func runSpecSummarizeCommit(presetID string, args []string) error {
	argv := []string{"commit"}
	if presetID != "" {
		argv = []string{presetID, "commit"}
	}
	return runViaCmd(newSpecSummarizeCmd, append(argv, args...))
}
