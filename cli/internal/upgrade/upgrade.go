// Package upgrade replaces the running vector binary with a published GitHub
// release: it resolves the platform asset, downloads it with checksums.txt,
// verifies the SHA256, and swaps the real (symlink-resolved) binary with a
// backup and rollback. It is non-interactive: the confirmation gate is a
// callback owned by cmd/vector. It never elevates privileges.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/updatecheck"
)

// DefaultDownloadBaseURL is where release assets are served from
// (`<base>/<tag>/<asset>`), matching scripts/install.sh.
const DefaultDownloadBaseURL = "https://github.com/" + updatecheck.Repository + "/releases/download"

// downloadTimeout bounds each asset/checksums download.
const downloadTimeout = 5 * time.Minute

// verifyTimeout bounds the post-swap `<binary> version` probe.
const verifyTimeout = 15 * time.Second

// Options configures an upgrade run.
type Options struct {
	// Target is the release tag to install; empty resolves releases/latest.
	Target string
	// Force allows reinstalling the same version or downgrading.
	Force bool
	// DryRun stops after the download + checksum + extraction checks.
	DryRun bool
	// Confirm is asked once every check passed, right before the binary is
	// touched. Returning false cancels without changes. Nil means confirmed.
	Confirm func(current, target string) (bool, error)
	// Progress receives short human progress lines. Nil discards them.
	Progress func(message string)
}

// Report describes what an upgrade run resolved and did.
type Report struct {
	CurrentVersion   string `json:"currentVersion"`
	TargetTag        string `json:"targetTag"`
	Asset            string `json:"asset"`
	Platform         string `json:"platform"`
	BinaryPath       string `json:"binaryPath,omitempty"`
	BackupPath       string `json:"backupPath,omitempty"`
	ChecksumVerified bool   `json:"checksumVerified"`
	Replaced         bool   `json:"replaced"`
	DryRun           bool   `json:"dryRun"`
	Cancelled        bool   `json:"cancelled"`
}

// runner holds the injectable dependencies of an upgrade run.
type runner struct {
	fetchLatestTag  func(ctx context.Context) (string, error)
	downloadBaseURL string
	httpClient      *http.Client
	goos            string
	goarch          string
	executablePath  func() (string, error)
	verifyBinary    func(ctx context.Context, binaryPath string) error
}

// Run upgrades the running binary per opts. currentVersion is the binary's
// stamped version ("dev" skips the newer-than check).
func Run(ctx context.Context, opts Options, currentVersion string) (*Report, error) {
	return defaultRunner().run(ctx, opts, currentVersion)
}

// defaultRunner wires the production dependencies.
func defaultRunner() *runner {
	return &runner{
		fetchLatestTag: func(ctx context.Context) (string, error) {
			release, err := updatecheck.FetchLatestRelease(ctx)
			if err != nil {
				return "", err
			}
			return release.TagName, nil
		},
		downloadBaseURL: DefaultDownloadBaseURL,
		httpClient:      &http.Client{Timeout: downloadTimeout},
		goos:            runtime.GOOS,
		goarch:          runtime.GOARCH,
		executablePath:  os.Executable,
		verifyBinary:    runVersionProbe,
	}
}

// run executes the upgrade flow. Nothing on disk outside a private temp dir is
// modified before every check (target, checksum, extraction, write permission,
// confirmation) has passed.
func (upgrader *runner) run(ctx context.Context, opts Options, currentVersion string) (*Report, error) {
	progress := opts.Progress
	if progress == nil {
		progress = func(string) {}
	}
	report := &Report{
		CurrentVersion: currentVersion,
		Platform:       upgrader.goos + "/" + upgrader.goarch,
		DryRun:         opts.DryRun,
	}
	if currentVersion != updatecheck.DevVersion {
		report.CurrentVersion = updatecheck.NormalizeTag(currentVersion)
	}

	targetTag, err := upgrader.resolveTarget(ctx, opts.Target)
	if err != nil {
		return nil, err
	}
	report.TargetTag = targetTag
	if err := checkNewer(currentVersion, targetTag, opts.Force); err != nil {
		return nil, err
	}

	assetName, err := resolveAssetName(upgrader.goos, upgrader.goarch, targetTag)
	if err != nil {
		return nil, err
	}
	report.Asset = assetName

	workDir, err := os.MkdirTemp("", "vector-upgrade-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	assetPath := filepath.Join(workDir, assetName)
	checksumsPath := filepath.Join(workDir, checksumsFileName)
	releaseURL := strings.TrimSuffix(upgrader.downloadBaseURL, "/") + "/" + targetTag
	progress(fmt.Sprintf("Downloading %s…", assetName))
	if err := downloadFile(ctx, upgrader.httpClient, releaseURL+"/"+assetName, assetPath); err != nil {
		return nil, err
	}
	if err := downloadFile(ctx, upgrader.httpClient, releaseURL+"/"+checksumsFileName, checksumsPath); err != nil {
		return nil, fmt.Errorf("%w — cannot verify integrity", err)
	}
	progress("Verifying checksum…")
	if err := verifyChecksum(assetPath, checksumsPath, assetName); err != nil {
		return nil, err
	}
	report.ChecksumVerified = true

	extractDir := filepath.Join(workDir, "extract")
	if err := os.Mkdir(extractDir, 0o755); err != nil {
		return nil, fmt.Errorf("create extract dir: %w", err)
	}
	newBinaryPath, err := extractArchive(assetPath, extractDir, upgrader.goos)
	if err != nil {
		return nil, err
	}
	if opts.DryRun {
		return report, nil
	}

	binaryPath, err := upgrader.resolveBinaryPath()
	if err != nil {
		return nil, err
	}
	report.BinaryPath = binaryPath
	if err := probeWritable(filepath.Dir(binaryPath)); err != nil {
		return nil, err
	}

	if opts.Confirm != nil {
		confirmed, err := opts.Confirm(report.CurrentVersion, targetTag)
		if err != nil {
			return nil, err
		}
		if !confirmed {
			report.Cancelled = true
			return report, nil
		}
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", binaryPath, err)
	}
	backupPath := binaryPath + backupSuffix
	if err := copyFile(binaryPath, backupPath, info.Mode().Perm()); err != nil {
		return nil, fmt.Errorf("back up the current binary: %w", err)
	}
	report.BackupPath = backupPath

	progress(fmt.Sprintf("Installing %s to %s…", targetTag, binaryPath))
	if err := swapBinary(newBinaryPath, binaryPath); err != nil {
		// A failed swap leaves the original in place: the unix rename is atomic and
		// the Windows sequence renames the old binary back itself.
		os.Remove(backupPath)
		return nil, fmt.Errorf("install the new binary: %w — the current binary was left unchanged", err)
	}
	if err := upgrader.verifyBinary(ctx, binaryPath); err != nil {
		return nil, upgrader.rollback(backupPath, binaryPath, fmt.Errorf("new binary failed its version check: %w", err))
	}
	report.Replaced = true

	if upgrader.goos != "windows" {
		if err := os.Remove(backupPath); err == nil {
			report.BackupPath = ""
		}
	}
	return report, nil
}

// resolveTarget validates an explicit --target or resolves releases/latest.
func (upgrader *runner) resolveTarget(ctx context.Context, target string) (string, error) {
	if target != "" {
		if err := ValidateTarget(target); err != nil {
			return "", err
		}
		return updatecheck.NormalizeTag(target), nil
	}
	tag, err := upgrader.fetchLatestTag(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve the latest release: %w — check your connection or pass --target <tag>", err)
	}
	if _, _, _, pre, ok := updatecheck.ParseVersion(tag); !ok || pre != "" {
		return "", fmt.Errorf("latest release tag %q is not a stable semver version", tag)
	}
	return updatecheck.NormalizeTag(tag), nil
}

// ValidateTarget fails fast on a --target that is not a stable vMAJOR.MINOR.PATCH.
func ValidateTarget(target string) error {
	_, _, _, pre, ok := updatecheck.ParseVersion(target)
	if !ok {
		return fmt.Errorf("invalid --target %q: expected a release tag like vMAJOR.MINOR.PATCH", target)
	}
	if pre != "" {
		return fmt.Errorf("invalid --target %q: pre-release versions are not supported", target)
	}
	return nil
}

// checkNewer refuses a same-version or older target unless forced. A "dev"
// current version has no meaningful baseline and never blocks.
func checkNewer(currentVersion, targetTag string, force bool) error {
	if currentVersion == updatecheck.DevVersion || force {
		return nil
	}
	cmp, ok := updatecheck.CompareVersions(targetTag, currentVersion)
	if !ok {
		return fmt.Errorf("cannot compare the current version %q with %s — pass --force to install anyway", currentVersion, targetTag)
	}
	if cmp <= 0 {
		return fmt.Errorf("vector %s is already at or newer than %s — nothing to upgrade (pass --force to reinstall or downgrade)",
			updatecheck.NormalizeTag(currentVersion), targetTag)
	}
	return nil
}

// resolveBinaryPath returns the real path of the running binary, following
// symlinks so the link itself is never overwritten.
func (upgrader *runner) resolveBinaryPath() (string, error) {
	executable, err := upgrader.executablePath()
	if err != nil {
		return "", fmt.Errorf("locate the running vector binary: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", executable, err)
	}
	return realPath, nil
}

// probeWritable checks the binary's directory accepts a new file (the swap is a
// rename there). No sudo, no elevation: failure is a refusal.
func probeWritable(dir string) error {
	probe, err := os.CreateTemp(dir, ".vector-write-probe-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s (%v) — reinstall vector to a user-writable directory (e.g. ~/.local/bin) or upgrade it with the tool that installed it; vector upgrade never uses sudo", dir, err)
	}
	probePath := probe.Name()
	probe.Close()
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("clean up write probe in %s: %w", dir, err)
	}
	return nil
}

// rollback restores the backup over binaryPath after a failed swap/verify and
// returns cause (annotated when the restore itself fails).
func (upgrader *runner) rollback(backupPath, binaryPath string, cause error) error {
	if err := installViaSiblingRename(backupPath, binaryPath); err != nil {
		return fmt.Errorf("%w; restoring the previous binary also failed (%v) — copy %s to %s manually", cause, err, backupPath, binaryPath)
	}
	os.Remove(backupPath)
	return fmt.Errorf("%w — the previous binary was restored", cause)
}

// runVersionProbe runs `<binary> version` and expects the legacy "vector <v>"
// line on stdout.
func runVersionProbe(ctx context.Context, binaryPath string) error {
	probeCtx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()
	output, err := exec.CommandContext(probeCtx, binaryPath, "version").Output()
	if err != nil {
		return fmt.Errorf("run %s version: %w", binaryPath, err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(output)), "vector ") {
		return errors.New("unexpected `vector version` output")
	}
	return nil
}
