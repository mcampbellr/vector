package gitexec

import (
	"os/exec"
	"strings"
	"testing"
)

func TestOutputInRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if _, err := Output(dir, "init", "--quiet", "--initial-branch=main"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	out, err := Output(dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "true" {
		t.Errorf("rev-parse --is-inside-work-tree = %q, want true", got)
	}
}

func TestOutputNotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	_, err := Output(t.TempDir(), "rev-parse", "--git-dir")
	if err == nil {
		t.Fatal("expected an error outside a git repo")
	}
	if !strings.HasPrefix(err.Error(), "git rev-parse --git-dir: ") {
		t.Errorf("error = %q, want the wrapped `git <args>: ...` form", err)
	}
}
