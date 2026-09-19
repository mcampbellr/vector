package upgrade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Synthetic versions only — never real release numbers.
const (
	olderTag  = "v1.0.0"
	latestTag = "v1.1.0"
)

// fixture is a fake release server plus an installed binary to upgrade.
type fixture struct {
	upgrader   *runner
	binaryPath string
	newContent []byte
	oldContent []byte
	requests   []string
}

// newFixture serves latestTag's asset + checksums (checksumOverride replaces the
// asset digest when non-empty) and installs an "old" binary in a temp dir.
func newFixture(t *testing.T, checksumOverride string) *fixture {
	t.Helper()
	fx := &fixture{newContent: []byte("new-binary"), oldContent: []byte("old-binary")}
	goos, goarch := runtime.GOOS, runtime.GOARCH
	archive := buildArchive(t, goos, fx.newContent)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fx.requests = append(fx.requests, request.URL.Path)
		parts := strings.Split(strings.TrimPrefix(request.URL.Path, "/"), "/")
		if len(parts) != 2 {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		assetName, err := resolveAssetName(goos, goarch, parts[0])
		if err != nil {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		switch parts[1] {
		case assetName:
			_, _ = writer.Write(archive)
		case checksumsFileName:
			digest := sha256Hex(archive)
			if checksumOverride != "" {
				digest = checksumOverride
			}
			_, _ = fmt.Fprintf(writer, "%s  %s\n", digest, assetName)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	installDir := t.TempDir()
	fx.binaryPath = filepath.Join(installDir, binaryName(goos))
	if err := os.WriteFile(fx.binaryPath, fx.oldContent, 0o755); err != nil {
		t.Fatal(err)
	}
	fx.upgrader = &runner{
		fetchLatestTag:  func(context.Context) (string, error) { return latestTag, nil },
		downloadBaseURL: server.URL,
		httpClient:      server.Client(),
		goos:            goos,
		goarch:          goarch,
		executablePath:  func() (string, error) { return fx.binaryPath, nil },
		verifyBinary:    func(context.Context, string) error { return nil },
	}
	return fx
}

func (fx *fixture) installedContent(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(fx.binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestRunUpgradesToLatest(t *testing.T) {
	fx := newFixture(t, "")
	report, err := fx.upgrader.run(context.Background(), Options{}, olderTag)
	if err != nil {
		t.Fatal(err)
	}
	if got := fx.installedContent(t); got != string(fx.newContent) {
		t.Errorf("installed %q, want the new binary", got)
	}
	if !report.Replaced || !report.ChecksumVerified || report.TargetTag != latestTag || report.CurrentVersion != olderTag {
		t.Errorf("report = %+v", report)
	}
	if runtime.GOOS != "windows" {
		if _, err := os.Stat(fx.binaryPath + backupSuffix); !os.IsNotExist(err) {
			t.Errorf("backup should be deleted on success, stat err = %v", err)
		}
		if report.BackupPath != "" {
			t.Errorf("BackupPath = %q, want empty after cleanup", report.BackupPath)
		}
	}
	if info, err := os.Stat(fx.binaryPath); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0) {
		t.Errorf("installed binary not executable: %v %v", info, err)
	}
}

func TestRunDryRunTouchesNothing(t *testing.T) {
	fx := newFixture(t, "")
	executableCalled := false
	fx.upgrader.executablePath = func() (string, error) { executableCalled = true; return fx.binaryPath, nil }
	report, err := fx.upgrader.run(context.Background(), Options{DryRun: true}, olderTag)
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun || !report.ChecksumVerified || report.Replaced || report.Asset == "" {
		t.Errorf("report = %+v", report)
	}
	if executableCalled {
		t.Error("dry-run must not resolve or touch the installed binary")
	}
	if got := fx.installedContent(t); got != string(fx.oldContent) {
		t.Errorf("dry-run replaced the binary: %q", got)
	}
}

func TestRunRefusesSameOrOlderWithoutForce(t *testing.T) {
	for _, current := range []string{latestTag, "v1.2.0"} {
		t.Run(current, func(t *testing.T) {
			fx := newFixture(t, "")
			_, err := fx.upgrader.run(context.Background(), Options{}, current)
			if err == nil || !strings.Contains(err.Error(), "--force") {
				t.Fatalf("err = %v, want a refusal mentioning --force", err)
			}
			if len(fx.requests) != 0 {
				t.Errorf("refusal must happen before any download, got %v", fx.requests)
			}
			if got := fx.installedContent(t); got != string(fx.oldContent) {
				t.Errorf("binary changed: %q", got)
			}
		})
	}
}

func TestRunDowngradeWithForce(t *testing.T) {
	fx := newFixture(t, "")
	report, err := fx.upgrader.run(context.Background(), Options{Target: olderTag, Force: true}, latestTag)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Replaced || report.TargetTag != olderTag {
		t.Errorf("report = %+v", report)
	}
}

func TestRunDevNeverBlocks(t *testing.T) {
	fx := newFixture(t, "")
	report, err := fx.upgrader.run(context.Background(), Options{}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !report.Replaced || report.CurrentVersion != "dev" {
		t.Errorf("report = %+v", report)
	}
}

func TestRunChecksumMismatchAborts(t *testing.T) {
	fx := newFixture(t, sha256Hex([]byte("tampered")))
	_, err := fx.upgrader.run(context.Background(), Options{}, olderTag)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v, want checksum mismatch", err)
	}
	if got := fx.installedContent(t); got != string(fx.oldContent) {
		t.Errorf("binary replaced despite mismatch: %q", got)
	}
	if _, err := os.Stat(fx.binaryPath + backupSuffix); !os.IsNotExist(err) {
		t.Errorf("no backup should exist, stat err = %v", err)
	}
}

func TestRunInvalidTargetFailsFast(t *testing.T) {
	fx := newFixture(t, "")
	for _, target := range []string{"latest", "v1.2", "v1.2.0-rc.1"} {
		if _, err := fx.upgrader.run(context.Background(), Options{Target: target}, olderTag); err == nil || !strings.Contains(err.Error(), "invalid --target") {
			t.Errorf("target %q: err = %v", target, err)
		}
	}
	if len(fx.requests) != 0 {
		t.Errorf("invalid target must not reach the network: %v", fx.requests)
	}
}

func TestRunLatestResolutionFailure(t *testing.T) {
	fx := newFixture(t, "")
	fx.upgrader.fetchLatestTag = func(context.Context) (string, error) { return "", errors.New("offline") }
	if _, err := fx.upgrader.run(context.Background(), Options{}, olderTag); err == nil || !strings.Contains(err.Error(), "--target") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits are not enforced on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	fx := newFixture(t, "")
	installDir := filepath.Dir(fx.binaryPath)
	if err := os.Chmod(installDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(installDir, 0o755) })

	_, err := fx.upgrader.run(context.Background(), Options{}, olderTag)
	if err == nil || !strings.Contains(err.Error(), "never uses sudo") {
		t.Fatalf("err = %v, want a write-permission refusal", err)
	}
	if got := fx.installedContent(t); got != string(fx.oldContent) {
		t.Errorf("binary changed: %q", got)
	}
	entries, err := os.ReadDir(installDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("partial writes left in the install dir: %v", entries)
	}
}

func TestRunReplacesSymlinkTargetNotLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need extra privileges on windows")
	}
	fx := newFixture(t, "")
	linkPath := filepath.Join(t.TempDir(), "vector")
	if err := os.Symlink(fx.binaryPath, linkPath); err != nil {
		t.Fatal(err)
	}
	fx.upgrader.executablePath = func() (string, error) { return linkPath, nil }

	report, err := fx.upgrader.run(context.Background(), Options{}, olderTag)
	if err != nil {
		t.Fatal(err)
	}
	realBinaryPath, err := filepath.EvalSymlinks(fx.binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if report.BinaryPath != realBinaryPath {
		t.Errorf("BinaryPath = %q, want the resolved target %q", report.BinaryPath, realBinaryPath)
	}
	info, err := os.Lstat(linkPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink was replaced: %v %v", info, err)
	}
	if got := fx.installedContent(t); got != string(fx.newContent) {
		t.Errorf("symlink target not upgraded: %q", got)
	}
}

func TestRunCancelledByConfirm(t *testing.T) {
	fx := newFixture(t, "")
	var asked string
	confirm := func(current, target string) (bool, error) {
		asked = current + "->" + target
		return false, nil
	}
	report, err := fx.upgrader.run(context.Background(), Options{Confirm: confirm}, olderTag)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Cancelled || report.Replaced {
		t.Errorf("report = %+v", report)
	}
	if asked != olderTag+"->"+latestTag {
		t.Errorf("confirm asked %q", asked)
	}
	if got := fx.installedContent(t); got != string(fx.oldContent) {
		t.Errorf("binary changed after a declined confirmation: %q", got)
	}
}

func TestRunVerifyFailureRestoresBackup(t *testing.T) {
	fx := newFixture(t, "")
	fx.upgrader.verifyBinary = func(context.Context, string) error { return errors.New("exec format error") }
	_, err := fx.upgrader.run(context.Background(), Options{}, olderTag)
	if err == nil || !strings.Contains(err.Error(), "previous binary was restored") {
		t.Fatalf("err = %v", err)
	}
	if got := fx.installedContent(t); got != string(fx.oldContent) {
		t.Errorf("binary not restored: %q", got)
	}
	if _, err := os.Stat(fx.binaryPath + backupSuffix); !os.IsNotExist(err) {
		t.Errorf("backup should be consumed by the restore, stat err = %v", err)
	}
}
