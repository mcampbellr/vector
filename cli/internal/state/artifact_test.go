package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// writeRepoFile writes content at a repo-relative path under root, creating dirs.
func writeRepoFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestReadSpecArtifactSpecDoc(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.CreateSpec(CreateSpecParams{
		Title: "Alpha",
		Body:  "# Alpha\n\nauthored spec body\n",
		Now:   fixedNow(),
	}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}

	b, err := store.ReadSpecArtifact("alpha", "spec")
	if err != nil {
		t.Fatalf("ReadSpecArtifact spec: %v", err)
	}
	if string(b) != "# Alpha\n\nauthored spec body\n" {
		t.Errorf("spec body = %q", string(b))
	}
}

func TestReadSpecArtifactConventionStore(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Convention store: the SpecDoc lives outside .vector/ (under the repo's
	// configured spec-path). CreateSpec writes the body there and records the
	// repo-relative pointer; the file preview must be able to read it back.
	const rel = "code/main/docs/specs/alpha/spec.md"
	if _, err := store.CreateSpec(CreateSpecParams{
		ID:             "alpha",
		Title:          "Alpha",
		Body:           "# Alpha\n\nconvention spec body\n",
		Now:            fixedNow(),
		SpecDocRel:     rel,
		SpecDocAbsPath: filepath.Join(root, filepath.FromSlash(rel)),
	}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}

	b, err := store.ReadSpecArtifact("alpha", "spec")
	if err != nil {
		t.Fatalf("ReadSpecArtifact spec (convention store): %v", err)
	}
	if string(b) != "# Alpha\n\nconvention spec body\n" {
		t.Errorf("spec body = %q", string(b))
	}
}

func TestReadSpecArtifactOpenSpecGatedByFlags(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// proposal exists + flagged; design file exists but flag is OFF; tasks flagged
	// but file missing.
	writeRepoFile(t, root, "openspec/changes/alpha/proposal.md", "# Proposal")
	writeRepoFile(t, root, "openspec/changes/alpha/design.md", "# Design")
	if _, err := store.CreateSpec(CreateSpecParams{
		Title:    "Alpha",
		Body:     "# Alpha\n",
		Now:      fixedNow(),
		OpenSpec: &OpenSpec{Change: "alpha", Artifacts: ArtifactSet{Proposal: true, Tasks: true}},
	}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}

	b, err := store.ReadSpecArtifact("alpha", "proposal")
	if err != nil {
		t.Fatalf("ReadSpecArtifact proposal: %v", err)
	}
	if string(b) != "# Proposal" {
		t.Errorf("proposal = %q", string(b))
	}

	// Flag off → not-found even though design.md is on disk.
	if _, err := store.ReadSpecArtifact("alpha", "design"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("design (flag off): err = %v, want fs.ErrNotExist", err)
	}
	// Flag on but file missing → not-found.
	if _, err := store.ReadSpecArtifact("alpha", "tasks"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("tasks (file missing): err = %v, want fs.ErrNotExist", err)
	}
}

func TestReadSpecArtifactUnknownSpecAndKey(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.ReadSpecArtifact("ghost", "spec"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("unknown spec: err = %v, want fs.ErrNotExist", err)
	}
	if _, err := store.CreateSpec(CreateSpecParams{Title: "Alpha", Body: "x", Now: fixedNow()}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if _, err := store.ReadSpecArtifact("alpha", "bogus"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("unknown key: err = %v, want fs.ErrNotExist", err)
	}
}

func TestReadSpecArtifactRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// A crafted change name with .. would resolve outside openspec/changes/. The
	// prefix/root check must reject it as a non-not-exist error (→ 500), never 404.
	writeRepoFile(t, root, "openspec/changes/alpha/proposal.md", "# Proposal")
	if _, err := store.CreateSpec(CreateSpecParams{
		Title:    "Alpha",
		Body:     "# Alpha\n",
		Now:      fixedNow(),
		OpenSpec: &OpenSpec{Change: "../../../etc", Artifacts: ArtifactSet{Proposal: true}},
	}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	_, err = store.ReadSpecArtifact("alpha", "proposal")
	if err == nil {
		t.Fatal("expected an error for a traversal change name")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("traversal err = %v, want a non-not-exist (500-class) error", err)
	}
}
