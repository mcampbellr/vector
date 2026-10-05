package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// specWindowOption is the tmux user option that tags a window with its spec id.
// Idempotency keys on it rather than on the window name, which the user may rename.
const specWindowOption = "@vector-spec"

// tmuxTargetFormat makes new-window/new-session print a "<session>:<window_id>"
// target, which addresses the new window for set-option/attach-session and also
// carries the session name for the report.
const tmuxTargetFormat = "#{session_name}:#{window_id}"

// windowNameMaxSlug is how many slug characters the window name keeps before "…".
const windowNameMaxSlug = 16

// tmuxWindowSpec is the input of buildTmuxArgs: everything that decides how the
// spec's window (or pane) is created.
type tmuxWindowSpec struct {
	insideTmux    bool   // $TMUX is set: create the window in the current session
	split         bool   // split the current window instead of creating one (inside tmux only)
	sessionExists bool   // outside tmux: the repo session already exists
	session       string // outside tmux: the repo session name
	windowName    string
	cwd           string
	shellCommand  string // "" starts the user's default shell
}

// buildTmuxArgs returns the tmux argv (without the "tmux" program name) that
// creates the spec's window or pane. It is pure — no I/O — so every layout
// (inside tmux, outside with a new or existing session, split) is testable
// without a tmux server. Outside tmux it never uses `new-session -A`: with an
// existing session -A degrades to attach-session and ignores -n, -c and the
// command, so the window is created detached (-d), tagged, then attached.
func buildTmuxArgs(spec tmuxWindowSpec) []string {
	cwd := escapeTmuxFormat(spec.cwd)
	var args []string
	switch {
	case spec.split:
		args = []string{"split-window", "-h", "-c", cwd}
	case spec.insideTmux:
		args = []string{"new-window", "-P", "-F", tmuxTargetFormat, "-n", spec.windowName, "-c", cwd}
	case spec.sessionExists:
		args = []string{"new-window", "-d", "-P", "-F", tmuxTargetFormat, "-t", exactSession(spec.session) + ":", "-n", spec.windowName, "-c", cwd}
	default:
		args = []string{"new-session", "-d", "-P", "-F", tmuxTargetFormat, "-s", spec.session, "-n", spec.windowName, "-c", cwd}
	}
	if spec.shellCommand != "" {
		args = append(args, spec.shellCommand)
	}
	return args
}

// escapeTmuxFormat doubles '#' so tmux, which expands -c start-directory as a
// format, takes the path literally: an unescaped "#(cmd)" in a directory name
// would run cmd.
func escapeTmuxFormat(value string) string {
	return strings.ReplaceAll(value, "#", "##")
}

// listSpecWindowsArgs lists every window of every session with its spec tag.
func listSpecWindowsArgs() []string {
	return []string{"list-windows", "-a", "-F", "#{session_name}:#{window_index} #{" + specWindowOption + "}"}
}

// hasSessionArgs checks whether the repo session exists (exact name match).
func hasSessionArgs(session string) []string {
	return []string{"has-session", "-t", exactSession(session)}
}

// tagWindowArgs tags a window with its spec id.
func tagWindowArgs(target, id string) []string {
	return []string{"set-option", "-w", "-t", target, specWindowOption, id}
}

// attachArgs attaches the terminal to target (session:window) in the foreground.
func attachArgs(target string) []string {
	return []string{"attach-session", "-t", target}
}

// focusWindowArgs focuses an already-open spec window: from inside tmux the
// client switches session (a no-op when already there) and selects the window;
// from outside, the terminal attaches to it.
func focusWindowArgs(target string, insideTmux bool) [][]string {
	if insideTmux {
		return [][]string{
			{"switch-client", "-t", target},
			{"select-window", "-t", target},
		}
	}
	return [][]string{attachArgs(target)}
}

// findSpecWindow parses `tmux list-windows -a` output (listSpecWindowsArgs) and
// returns the "<session>:<index>" target of the window tagged with id. The tag is
// the last space-separated field (spec ids never contain spaces), so session
// names with spaces still parse.
func findSpecWindow(listing, id string) (string, bool) {
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimRight(line, "\r")
		cut := strings.LastIndex(line, " ")
		if cut <= 0 {
			continue
		}
		if line[cut+1:] == id {
			return line[:cut], true
		}
	}
	return "", false
}

// specWindowName is "◆ " plus the first 16 characters of the slug, with "…" when
// the slug is longer.
func specWindowName(id string) string {
	if utf8.RuneCountInString(id) <= windowNameMaxSlug {
		return "◆ " + id
	}
	return "◆ " + string([]rune(id)[:windowNameMaxSlug]) + "…"
}

// repoSessionName is the tmux session used outside tmux: the repo root's
// basename, with the characters tmux rejects in session names ('.' and ':')
// replaced the way tmux itself does.
func repoSessionName(repoRoot string) string {
	name := strings.NewReplacer(".", "_", ":", "_").Replace(filepath.Base(repoRoot))
	if name == "" || name == "_" || name == string(filepath.Separator) {
		return "vector"
	}
	return name
}

// exactSession prefixes a session name with "=" so tmux matches it exactly
// instead of by prefix.
func exactSession(session string) string { return "=" + session }

// sessionOfTarget returns the session part of a "<session>:<window>" target.
func sessionOfTarget(target string) string {
	if cut := strings.LastIndex(target, ":"); cut >= 0 {
		return target[:cut]
	}
	return target
}

// claudeShellCommand is the shell command tmux runs in the new window: Claude
// Code with the slash command as its first message, safely quoted.
func claudeShellCommand(slash string) string {
	return "claude " + shellQuote(slash)
}

// shellQuote single-quotes s for a POSIX shell unless it only holds characters
// that never need quoting.
func shellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool { return !isShellSafe(r) }) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func isShellSafe(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	// '=' is excluded: zsh expands a word starting with '=' (EQUALS option).
	return strings.ContainsRune("@%+:,./_-", r)
}

// renderCommand renders an argv as a copy-pasteable shell line.
func renderCommand(program string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, program)
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

// renderCapture renders `VAR=$(program args…)` for --print, so the printed
// sequence stays runnable when a later command targets the captured window.
func renderCapture(variable, program string, args []string) string {
	return fmt.Sprintf("%s=$(%s)", variable, renderCommand(program, args))
}
