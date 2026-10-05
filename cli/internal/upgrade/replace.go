package upgrade

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// staleSuffix names the renamed-away binary a Windows swap leaves behind.
const staleSuffix = ".old"

// backupSuffix names the pre-swap copy used for rollback.
const backupSuffix = ".bak"

// installViaSiblingRename copies sourcePath to a temp file in targetPath's
// directory and renames it over targetPath. The rename stays within one
// filesystem, so the target is replaced atomically — it is never missing or
// half-written. Shared by the unix swap and by rollback on every platform.
func installViaSiblingRename(sourcePath, targetPath string) error {
	tmpPath, err := copyToSiblingTemp(sourcePath, filepath.Dir(targetPath))
	if err != nil {
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace %s: %w", targetPath, err)
	}
	return nil
}

// replaceViaRenameOld is the Windows swap sequence, kept platform-neutral so it
// is testable everywhere: rename targetPath to `<target>.old` (allowed for a
// running .exe, unlike overwriting it), then install the new binary under the
// original name. If the install fails, the old binary is renamed back.
func replaceViaRenameOld(newBinaryPath, targetPath string) error {
	stalePath := targetPath + staleSuffix
	if err := os.Remove(stalePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove leftover %s (still in use? retry after closing other vector processes): %w", filepath.Base(stalePath), err)
	}
	if err := os.Rename(targetPath, stalePath); err != nil {
		return fmt.Errorf("move the running binary aside: %w", err)
	}
	if err := installViaSiblingRename(newBinaryPath, targetPath); err != nil {
		if restoreErr := os.Rename(stalePath, targetPath); restoreErr != nil {
			return fmt.Errorf("%w (and restoring %s failed: %v)", err, targetPath, restoreErr)
		}
		return err
	}
	return nil
}

// copyToSiblingTemp copies sourcePath into a new executable temp file inside dir
// and returns its path.
func copyToSiblingTemp(sourcePath, dir string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", sourcePath, err)
	}
	defer source.Close()
	tmp, err := os.CreateTemp(dir, ".vector-upgrade-*")
	if err != nil {
		return "", fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, source); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("copy binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("chmod temp file: %w", err)
	}
	return tmpPath, nil
}

// copyFile copies sourcePath to destPath (created or truncated) with mode.
func copyFile(sourcePath, destPath string, mode os.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", sourcePath, err)
	}
	defer source.Close()
	dest, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	if _, err := io.Copy(dest, source); err != nil {
		dest.Close()
		return fmt.Errorf("copy to %s: %w", destPath, err)
	}
	if err := dest.Close(); err != nil {
		return fmt.Errorf("close %s: %w", destPath, err)
	}
	return nil
}
