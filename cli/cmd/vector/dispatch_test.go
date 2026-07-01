package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestDispatchExitCodes locks in the legacy exit-code contract (design.md §11)
// through the cobra tree: the mapping must survive the flag → cobra migration.
func TestDispatchExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no args", nil, 2},
		{"unknown command", []string{"bogus"}, 2},
		{"spec no subverb", []string{"spec"}, 1},
		{"spec unknown subverb", []string{"spec", "bogus"}, 1},
		{"spec create unknown flag", []string{"spec", "create", "--bogus"}, 1},
		{"version subcommand", []string{"version"}, 0},
		{"--version flag", []string{"--version"}, 0},
		{"-v flag", []string{"-v"}, 0},
		{"-v after subcommand", []string{"spec", "list", "-v"}, 0},
		{"help", []string{"help"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Silence the command's own stdout/stderr; we only assert the exit code.
			got := captureStdout(t, func() error {
				if code := dispatch(tc.args); code != tc.want {
					t.Errorf("dispatch(%v) = %d, want %d", tc.args, code, tc.want)
				}
				return nil
			})
			_ = got
		})
	}
}

// TestContextJSONDefaultsTrue asserts the `--json` flag on context defaults to
// true (a behavior the pre-cobra flag.Bool set and must be preserved).
func TestContextJSONDefaultsTrue(t *testing.T) {
	cmd := newContextCmd()
	v, err := cmd.Flags().GetBool("json")
	if err != nil {
		t.Fatalf("json flag missing: %v", err)
	}
	if !v {
		t.Error("context --json should default to true")
	}
}

// TestServePortChangedDetection asserts the serve factory registers --port and
// that Changed() reflects whether it was explicitly set (the replacement for the
// old fs.Visit port-detection).
func TestServePortChangedDetection(t *testing.T) {
	cmd := newServeCmd()
	if cmd.Flags().Changed("port") {
		t.Error("port should not be Changed before parsing")
	}
	if err := cmd.Flags().Parse([]string{"--port", "9999"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cmd.Flags().Changed("port") {
		t.Error("port should be Changed after explicit --port")
	}
	cmd2 := newServeCmd()
	if err := cmd2.Flags().Parse([]string{"--host", "0.0.0.0"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cmd2.Flags().Changed("port") {
		t.Error("port should NOT be Changed when only --host is set (free-port fallback path)")
	}
}

// TestVersionLdflag builds the binary with -X main.version and asserts the
// injected version is printed verbatim (the ldflag wiring must survive the
// migration). Skipped in -short mode (it invokes `go build`).
func TestVersionLdflag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build-based ldflag test in -short mode")
	}
	bin := t.TempDir() + "/vector"
	build := exec.Command("go", "build", "-ldflags", "-X main.version=v9.9.9-test", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		out, err := exec.Command(bin, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("run %v: %v\n%s", args, err, out)
		}
		if got := strings.TrimSpace(string(out)); got != "vector v9.9.9-test" {
			t.Errorf("version %v = %q, want %q", args, got, "vector v9.9.9-test")
		}
	}
}

// TestNoUIInJSONBranch is a source-level guard (spec §7.4): no ui.* call may sit
// inside an `if jsonOut {`/`if *jsonOut {` branch, or the --json stdout could gain
// styling bytes and break the byte-identical contract. It scans each command
// source, tracks brace depth from the start of a jsonOut branch, and fails if a
// `ui.` token appears before that branch closes. (ui styling is allowed elsewhere
// — the human branch — which is exactly what this permits.)
func TestNoUIInJSONBranch(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		inJSON := false
		depth := 0 // brace depth relative to the jsonOut branch's opening brace
		for i, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimSpace(line)
			if !inJSON && (strings.HasPrefix(trimmed, "if jsonOut {") || strings.HasPrefix(trimmed, "if *jsonOut {")) {
				inJSON = true
				depth = 1
				continue
			}
			if inJSON {
				depth += strings.Count(line, "{") - strings.Count(line, "}")
				if strings.Contains(line, "ui.") {
					t.Errorf("%s:%d — ui.* call inside a jsonOut branch would corrupt the byte-identical --json contract", name, i+1)
				}
				if depth <= 0 {
					inJSON = false
				}
			}
		}
	}
}
