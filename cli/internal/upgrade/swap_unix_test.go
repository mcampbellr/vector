//go:build !windows

package upgrade

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestSwapBinaryAtomicWhileOpen simulates the running binary with an open file
// descriptor: after the swap the old descriptor still reads the old bytes (old
// inode) while the path serves the new binary.
func TestSwapBinaryAtomicWhileOpen(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "vector")
	if err := os.WriteFile(targetPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	running, err := os.Open(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()

	newPath := filepath.Join(t.TempDir(), "vector")
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := swapBinary(newPath, targetPath); err != nil {
		t.Fatal(err)
	}

	oldBytes, err := io.ReadAll(running)
	if err != nil {
		t.Fatal(err)
	}
	if string(oldBytes) != "old" {
		t.Errorf("running descriptor reads %q, want old", oldBytes)
	}
	newBytes, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(newBytes) != "new" {
		t.Errorf("path serves %q, want new", newBytes)
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", info.Mode().Perm())
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".vector-upgrade-*"))
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
	if err := CleanupStaleWindowsBinary(targetPath); err != nil {
		t.Errorf("unix cleanup must be a no-op, got %v", err)
	}
}
