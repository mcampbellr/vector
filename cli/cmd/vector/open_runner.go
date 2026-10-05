package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// openRunner is the seam between `vector open` and the external processes it
// drives (tmux, git, claude, fzf). The command never calls os/exec directly, so
// tests inject a fake and never spawn a real process.
type openRunner interface {
	// LookPath reports whether an executable is available on the PATH.
	LookPath(file string) (string, error)
	// Output runs a command, returning its stdout; a failure carries stderr
	// verbatim (as *commandError).
	Output(name string, args ...string) (string, error)
	// Interactive runs a command attached to the user's terminal (stdin, stdout
	// and stderr inherited) — used for the foreground `tmux attach-session`.
	Interactive(name string, args ...string) error
	// Pick runs an interactive picker fed with input on stdin, returning its
	// stdout (the selection); stderr stays on the terminal for the picker UI.
	Pick(input, name string, args ...string) (string, error)
}

// commandError is a failed external command with its stderr kept verbatim, so
// callers can surface git/tmux diagnostics unchanged.
type commandError struct {
	Name   string
	Args   []string
	Stderr string
	Err    error
}

func (e *commandError) Error() string {
	detail := strings.TrimSpace(e.Stderr)
	if detail == "" {
		detail = e.Err.Error()
	}
	return fmt.Sprintf("%s %s: %s", e.Name, strings.Join(e.Args, " "), detail)
}

func (e *commandError) Unwrap() error { return e.Err }

// execRunner is the real openRunner backed by os/exec.
type execRunner struct{}

func (execRunner) LookPath(file string) (string, error) { return exec.LookPath(file) }

func (execRunner) Output(name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), &commandError{Name: name, Args: args, Stderr: stderr.String(), Err: err}
	}
	return stdout.String(), nil
}

func (execRunner) Interactive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (execRunner) Pick(input, name string, args ...string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
