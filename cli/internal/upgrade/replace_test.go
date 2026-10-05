package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReplaceViaRenameOld exercises the Windows swap sequence on every platform:
// rename-old → write-new → `.old` left behind for deferred cleanup.
func TestReplaceViaRenameOld(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "vector.exe")
	if err := os.WriteFile(targetPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A leftover .old from a previous upgrade is replaced, not an error.
	if err := os.WriteFile(targetPath+staleSuffix, []byte("older"), 0o755); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(t.TempDir(), "vector.exe")
	if err := os.WriteFile(newPath, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceViaRenameOld(newPath, targetPath); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(targetPath); string(got) != "new" {
		t.Errorf("target = %q, want new", got)
	}
	if got, _ := os.ReadFile(targetPath + staleSuffix); string(got) != "old" {
		t.Errorf(".old = %q, want old", got)
	}
}

// TestReplaceViaRenameOldRestoresOnFailure: when the new binary cannot be
// installed, the old one is renamed back under the original name.
func TestReplaceViaRenameOldRestoresOnFailure(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "vector.exe")
	if err := os.WriteFile(targetPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	missingSource := filepath.Join(t.TempDir(), "does-not-exist")
	if err := replaceViaRenameOld(missingSource, targetPath); err == nil {
		t.Fatal("expected an error for a missing source")
	}
	if got, _ := os.ReadFile(targetPath); string(got) != "old" {
		t.Errorf("target = %q, want the restored old binary", got)
	}
	if _, err := os.Stat(targetPath + staleSuffix); !os.IsNotExist(err) {
		t.Errorf(".old should be renamed back, stat err = %v", err)
	}
}
