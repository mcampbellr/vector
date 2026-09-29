// Package gitexec is the single, shared entrypoint for running `git` subprocesses
// against a repo root. It exists so unrelated domains (intel's fingerprint, check)
// reuse one helper instead of each re-declaring the same `git -C` shell-out with the
// same "git missing / not-a-repo → wrapped error, caller degrades" contract.
package gitexec

import (
	"fmt"
	"os/exec"
	"strings"
)

// Output runs `git -C repoRoot <args...>` and returns stdout. An error (git
// absent, not a repo, a failing subcommand) is returned wrapped so callers can
// fall back rather than abort.
func Output(repoRoot string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
