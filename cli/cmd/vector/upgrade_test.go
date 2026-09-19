package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mariocampbell/vector/internal/upgrade"
)

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = orig
	out, _ := io.ReadAll(r)
	return string(out)
}

// upgradeHarness swaps the upgrade command's injection points for one test.
type upgradeHarness struct {
	calls   int
	options upgrade.Options
	report  *upgrade.Report
	// confirmed records the Confirm outcome when the fake invoked it.
	confirmed *bool
}

func newUpgradeHarness(t *testing.T, terminal bool, promptInput string) *upgradeHarness {
	t.Helper()
	harness := &upgradeHarness{}
	origTerminal, origInput, origRun := stdinIsTerminal, upgradePromptInput, runUpgrade
	t.Cleanup(func() { stdinIsTerminal, upgradePromptInput, runUpgrade = origTerminal, origInput, origRun })
	stdinIsTerminal = func() bool { return terminal }
	upgradePromptInput = strings.NewReader(promptInput)
	runUpgrade = func(_ context.Context, opts upgrade.Options, current string) (*upgrade.Report, error) {
		harness.calls++
		harness.options = opts
		report := &upgrade.Report{CurrentVersion: current, TargetTag: "v1.1.0", Asset: "vector_1.1.0_linux_amd64.tar.gz", Platform: "linux/amd64", ChecksumVerified: true, DryRun: opts.DryRun}
		if opts.Confirm != nil {
			ok, err := opts.Confirm(current, report.TargetTag)
			if err != nil {
				return nil, err
			}
			harness.confirmed = &ok
			if !ok {
				report.Cancelled = true
				harness.report = report
				return report, nil
			}
		}
		report.Replaced = !opts.DryRun
		harness.report = report
		return report, nil
	}
	return harness
}

func TestUpgradeNonTTYWithoutYesRefuses(t *testing.T) {
	harness := newUpgradeHarness(t, false, "")
	_, err := execCmd(t, newUpgradeCmd)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("err = %v, want a refusal pointing at --yes", err)
	}
	if harness.calls != 0 {
		t.Error("upgrade must not run without confirmation in a non-TTY")
	}
}

func TestUpgradeNonTTYWithYesProceeds(t *testing.T) {
	for _, flag := range []string{"--yes", "-y"} {
		t.Run(flag, func(t *testing.T) {
			harness := newUpgradeHarness(t, false, "")
			if _, err := execCmd(t, newUpgradeCmd, flag); err != nil {
				t.Fatal(err)
			}
			if harness.calls != 1 || harness.options.Confirm != nil {
				t.Errorf("calls = %d, confirm set = %v", harness.calls, harness.options.Confirm != nil)
			}
			if !harness.report.Replaced {
				t.Error("expected the upgrade to proceed")
			}
		})
	}
}

func TestUpgradeDryRunNeedsNoConfirmation(t *testing.T) {
	harness := newUpgradeHarness(t, false, "")
	if _, err := execCmd(t, newUpgradeCmd, "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if harness.calls != 1 || !harness.options.DryRun {
		t.Errorf("calls = %d, options = %+v", harness.calls, harness.options)
	}
}

func TestUpgradeTTYPrompt(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"\n", false},
		{"", false},
		{"maybe\n", false},
	}
	for _, tc := range cases {
		t.Run(strings.TrimSpace(tc.input), func(t *testing.T) {
			harness := newUpgradeHarness(t, true, tc.input)
			var stdout string
			stderr := captureStderr(t, func() {
				var err error
				stdout, err = execCmd(t, newUpgradeCmd)
				if err != nil {
					t.Fatal(err)
				}
			})
			if harness.confirmed == nil || *harness.confirmed != tc.want {
				t.Fatalf("confirmed = %v, want %v", harness.confirmed, tc.want)
			}
			if !strings.Contains(stderr, "[y/N]") {
				t.Errorf("prompt not written to stderr: %q", stderr)
			}
			if tc.want && !strings.Contains(stdout, "Upgraded vector") {
				t.Errorf("stdout = %q", stdout)
			}
			if !tc.want && !strings.Contains(stdout, "cancelled") {
				t.Errorf("stdout = %q, want a cancellation notice", stdout)
			}
		})
	}
}

func TestUpgradeInvalidTargetFailsFast(t *testing.T) {
	harness := newUpgradeHarness(t, false, "")
	_, err := execCmd(t, newUpgradeCmd, "--yes", "--target", "latest")
	if err == nil || !strings.Contains(err.Error(), "invalid --target") {
		t.Fatalf("err = %v", err)
	}
	if harness.calls != 0 {
		t.Error("invalid --target must fail before running the upgrade")
	}
}

func TestUpgradeJSONShape(t *testing.T) {
	newUpgradeHarness(t, false, "")
	out, err := execCmd(t, newUpgradeCmd, "--yes", "--json", "--target", "v1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &fields); err != nil {
		t.Fatalf("stdout is not pure JSON: %v\n%s", err, out)
	}
	for _, key := range []string{"currentVersion", "targetTag", "asset", "platform", "checksumVerified", "replaced", "dryRun", "cancelled"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("missing key %q in %s", key, out)
		}
	}
}

func TestUpgradeKeepsPersistentVersionFlag(t *testing.T) {
	harness := newUpgradeHarness(t, false, "")
	code := 0
	captureStdout(t, func() error {
		code = dispatch([]string{"upgrade", "-v"})
		return nil
	})
	if code != 0 || harness.calls != 0 {
		t.Errorf("`upgrade -v` exit = %d, upgrade calls = %d; want 0 and 0 (prints the version)", code, harness.calls)
	}
}
