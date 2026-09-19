//go:build !windows

package upgrade

// swapBinary installs newBinaryPath over targetPath: the new binary is copied to
// a temp file in targetPath's directory (same filesystem) and renamed over the
// target atomically. The running process keeps executing the old inode through
// its open descriptor; the next `vector` invocation picks up the new one.
func swapBinary(newBinaryPath, targetPath string) error {
	return installViaSiblingRename(newBinaryPath, targetPath)
}

// CleanupStaleWindowsBinary is a no-op outside Windows (no deferred `.old`).
func CleanupStaleWindowsBinary(_ string) error {
	return nil
}
