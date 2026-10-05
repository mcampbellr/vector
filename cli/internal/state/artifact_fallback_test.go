package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// worktreeSpecDoc is the per-spec worktree layout (code/[branch]/…) whose folder is
// removed after the branch merges, leaving the recorded SpecDoc pointer dangling.
const worktreeSpecDoc = "code/alpha/openspec/changes/alpha/spec.md"

// createWorktreeSpec creates spec "alpha" whose doc lives in its own worktree.
func createWorktreeSpec(t *testing.T, store *Store, root, body string, openSpec *OpenSpec) {
	t.Helper()
	if _, err := store.CreateSpec(CreateSpecParams{
		ID:             "alpha",
		Title:          "Alpha",
		Body:           body,
		Now:            fixedNow(),
		SpecDocRel:     worktreeSpecDoc,
		SpecDocAbsPath: filepath.Join(root, filepath.FromSlash(worktreeSpecDoc)),
		OpenSpec:       openSpec,
	}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
}

func removeWorktree(t *testing.T, root string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, "code", "alpha")); err != nil {
		t.Fatalf("remove worktree: %v", err)
	}
}

func readArtifact(t *testing.T, store *Store, artifact string) string {
	t.Helper()
	b, err := store.ReadSpecArtifact("alpha", artifact)
	if err != nil {
		t.Fatalf("ReadSpecArtifact %s: %v", artifact, err)
	}
	return string(b)
}

func TestCreateSpecSnapshotsWorktreeSpecDoc(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	createWorktreeSpec(t, store, root, "# Alpha\n", nil)

	snapshot, err := os.ReadFile(filepath.Join(root, ".vector", "specs", "alpha", "spec.md"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(snapshot) != "# Alpha\n" {
		t.Errorf("snapshot = %q", string(snapshot))
	}

	removeWorktree(t, root)
	if got := readArtifact(t, store, "spec"); got != "# Alpha\n" {
		t.Errorf("spec after worktree removal = %q, want the snapshot", got)
	}
}

func TestTransitionRefreshesSpecDocSnapshot(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	createWorktreeSpec(t, store, root, "# Alpha v1\n", nil)
	writeRepoFile(t, root, worktreeSpecDoc, "# Alpha v2\n")

	if _, err := store.CloseSpec("alpha", "test", fixedNow()); err != nil {
		t.Fatalf("CloseSpec: %v", err)
	}
	removeWorktree(t, root)
	if got := readArtifact(t, store, "spec"); got != "# Alpha v2\n" {
		t.Errorf("spec after close + removal = %q, want the refreshed snapshot", got)
	}
}

func TestSnapshotKeptWhenSpecDocMissing(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	createWorktreeSpec(t, store, root, "# Alpha\n", nil)
	removeWorktree(t, root)

	// A transition after the worktree is gone must not fail nor drop the snapshot.
	if _, err := store.CloseSpec("alpha", "test", fixedNow()); err != nil {
		t.Fatalf("CloseSpec with a missing spec doc: %v", err)
	}
	if got := readArtifact(t, store, "spec"); got != "# Alpha\n" {
		t.Errorf("spec = %q, want the preserved snapshot", got)
	}
}

func TestVectorStoreSpecDocIsNotDuplicated(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// The .vector fallback store: SpecDoc already is .vector/specs/<id>/spec.md.
	spec, err := store.CreateSpec(CreateSpecParams{Title: "Alpha", Body: "# Alpha\n", Now: fixedNow()})
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if store.specDocSnapshotPath(spec) != "" {
		t.Errorf("snapshot path = %q, want none for the .vector store", store.specDocSnapshotPath(spec))
	}
}

func TestReadSpecArtifactFallsBackToChangesDirs(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	baseChanges := filepath.Join(root, "code", "main", "openspec", "changes")
	rootChanges := filepath.Join(root, "openspec", "changes")
	store.SetArtifactFallbacks(ArtifactFallbacks{ChangesDirs: func() []string {
		return []string{baseChanges, rootChanges}
	}})

	// proposal survives in the base worktree; design only in the archive, where the
	// newest dated folder wins; tasks has no copy anywhere.
	writeRepoFile(t, root, "code/main/openspec/changes/alpha/proposal.md", "# Proposal (main)")
	writeRepoFile(t, root, "openspec/changes/archive/2026-01-01-alpha/design.md", "# Design (old)")
	writeRepoFile(t, root, "openspec/changes/archive/2026-03-01-alpha/design.md", "# Design (new)")
	createWorktreeSpec(t, store, root, "# Alpha\n", &OpenSpec{
		Change:    "alpha",
		Artifacts: ArtifactSet{Proposal: true, Design: true, Tasks: true},
	})

	if got := readArtifact(t, store, "proposal"); got != "# Proposal (main)" {
		t.Errorf("proposal = %q", got)
	}
	if got := readArtifact(t, store, "design"); got != "# Design (new)" {
		t.Errorf("design = %q, want the newest archived copy", got)
	}
	if _, err := store.ReadSpecArtifact("alpha", "tasks"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("tasks (no copy): err = %v, want fs.ErrNotExist", err)
	}
}

func TestReadSpecArtifactSpecDocFallbackOrder(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	baseChanges := filepath.Join(root, "code", "main", "openspec", "changes")
	store.SetArtifactFallbacks(ArtifactFallbacks{ChangesDirs: func() []string { return []string{baseChanges} }})

	// Recorded without a body (e.g. a spec created before snapshots existed): no
	// snapshot, so the base worktree copy is next, then the composer output.
	if _, err := store.CreateSpec(CreateSpecParams{
		ID: "alpha", Title: "Alpha", Now: fixedNow(), SpecDocRel: worktreeSpecDoc,
	}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	writeRepoFile(t, root, ".vector/tmp/alpha/spec.md", "# Alpha (composer)")
	if got := readArtifact(t, store, "spec"); got != "# Alpha (composer)" {
		t.Errorf("spec = %q, want the composer output", got)
	}

	writeRepoFile(t, root, "code/main/openspec/changes/alpha/spec.md", "# Alpha (main)")
	if got := readArtifact(t, store, "spec"); got != "# Alpha (main)" {
		t.Errorf("spec = %q, want the base worktree copy over the composer output", got)
	}
}

func TestReadSpecArtifactNoSurvivingCopy(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	store.SetArtifactFallbacks(ArtifactFallbacks{ChangesDirs: func() []string {
		return []string{filepath.Join(root, "openspec", "changes")}
	}})
	if _, err := store.CreateSpec(CreateSpecParams{
		ID: "alpha", Title: "Alpha", Now: fixedNow(), SpecDocRel: worktreeSpecDoc,
	}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if _, err := store.ReadSpecArtifact("alpha", "spec"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestArtifactChangeNameRejectsUnsafeSegments(t *testing.T) {
	cases := []struct {
		name   string
		change string
		want   string
	}{
		{name: "plain slug", change: "alpha", want: "alpha"},
		{name: "traversal", change: "..", want: ""},
		{name: "nested", change: "a/b", want: ""},
		{name: "glob metachar", change: "al*ha", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := &SpecState{ID: "alpha", OpenSpec: &OpenSpec{Change: tc.change}}
			if got := artifactChangeName(spec, false, ""); got != tc.want {
				t.Errorf("artifactChangeName(%q) = %q, want %q", tc.change, got, tc.want)
			}
		})
	}
}
