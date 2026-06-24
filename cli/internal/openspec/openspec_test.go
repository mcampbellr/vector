package openspec

import (
	"os"
	"path/filepath"
	"testing"
)

func writeChange(t *testing.T, root, rel string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func find(changes []Change, name string) (Change, bool) {
	for _, c := range changes {
		if c.Name == name {
			return c, true
		}
	}
	return Change{}, false
}

func TestReadChangesActiveAndArchived(t *testing.T) {
	root := t.TempDir()
	// active: some tasks done
	writeChange(t, root, "openspec/changes/add-auth", map[string]string{
		"proposal.md": "## Why\n",
		"design.md":   "x",
		"tasks.md":    "- [x] 1.1 done\n- [ ] 1.2 todo\n- [ ] 1.3 todo\n",
	})
	// active: no tasks
	writeChange(t, root, "openspec/changes/no-tasks", map[string]string{
		"proposal.md": "## Why\n",
	})
	// archived: date-prefixed dir
	writeChange(t, root, "openspec/changes/archive/2026-05-11-old-fix", map[string]string{
		"proposal.md": "x",
		"tasks.md":    "- [x] done\n",
	})

	changes, err := ReadChanges(root)
	if err != nil {
		t.Fatalf("ReadChanges: %v", err)
	}
	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3", len(changes))
	}

	auth, ok := find(changes, "add-auth")
	if !ok {
		t.Fatal("add-auth not found")
	}
	if auth.Archived || auth.TasksTotal != 3 || auth.TasksDone != 1 {
		t.Errorf("add-auth: archived=%v total=%d done=%d", auth.Archived, auth.TasksTotal, auth.TasksDone)
	}
	if !auth.HasProposal || !auth.HasDesign || !auth.HasTasks {
		t.Errorf("add-auth artifacts: %+v", auth)
	}
	if auth.ProposalRel != "openspec/changes/add-auth/proposal.md" {
		t.Errorf("ProposalRel = %q", auth.ProposalRel)
	}

	old, ok := find(changes, "old-fix")
	if !ok {
		t.Fatal("archived change should drop the date prefix → old-fix")
	}
	if !old.Archived {
		t.Error("old-fix should be archived")
	}
}

func TestPendingRealIgnoresManualQA(t *testing.T) {
	root := t.TempDir()
	writeChange(t, root, "openspec/changes/almost-done", map[string]string{
		"tasks.md": "- [x] 1.1 build it\n- [x] 1.2 wire it\n- [ ] 7.7 Manual check — ready for manual QA\n",
	})
	changes, err := ReadChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := find(changes, "almost-done")
	if !ok {
		t.Fatal("almost-done not found")
	}
	if c.TasksTotal != 3 || c.TasksDone != 2 {
		t.Fatalf("total=%d done=%d", c.TasksTotal, c.TasksDone)
	}
	if c.PendingReal != 0 {
		t.Fatalf("PendingReal = %d, want 0 (only manual QA left)", c.PendingReal)
	}
}

func TestDetected(t *testing.T) {
	root := t.TempDir()
	if Detected(root) {
		t.Error("Detected = true on empty repo")
	}
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !Detected(root) {
		t.Error("Detected = false with openspec/changes present")
	}
}

func TestReadChangesAbsentDir(t *testing.T) {
	changes, err := ReadChanges(t.TempDir())
	if err != nil {
		t.Fatalf("ReadChanges on absent dir: %v", err)
	}
	if changes != nil {
		t.Errorf("want nil, got %v", changes)
	}
}
