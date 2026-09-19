//go:build windows

package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwapBinaryWindowsLeavesOld(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "vector.exe")
	if err := os.WriteFile(targetPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(t.TempDir(), "vector.exe")
	if err := os.WriteFile(newPath, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := swapBinary(newPath, targetPath); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(targetPath); string(got) != "new" {
		t.Errorf("target = %q, want new", got)
	}
	if got, _ := os.ReadFile(targetPath + staleSuffix); string(got) != "old" {
		t.Errorf(".old = %q, want old", got)
	}
	if err := CleanupStaleWindowsBinary(targetPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(targetPath + staleSuffix); !os.IsNotExist(err) {
		t.Errorf(".old not removed: %v", err)
	}
	if err := CleanupStaleWindowsBinary(targetPath); err != nil {
		t.Errorf("cleanup without a .old must succeed, got %v", err)
	}
}
