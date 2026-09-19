//go:build windows

package upgrade

import (
	"errors"
	"os"
)

// swapBinary installs newBinaryPath over targetPath on Windows, where a running
// .exe cannot be overwritten but can be renamed: rename the current binary to
// `<target>.old`, write the new one under the original name, and leave `.old`
// for deferred cleanup (CleanupStaleWindowsBinary on the next invocation).
func swapBinary(newBinaryPath, targetPath string) error {
	return replaceViaRenameOld(newBinaryPath, targetPath)
}

// CleanupStaleWindowsBinary removes `<execPath>.old` left by a previous upgrade.
// A missing file is not an error; a still-locked one returns the error, which
// callers ignore (best-effort, retried on the next invocation).
func CleanupStaleWindowsBinary(execPath string) error {
	err := os.Remove(execPath + staleSuffix)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
