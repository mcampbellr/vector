package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMigratesFromProjectStructure(t *testing.T) {
	root := t.TempDir()
	ps := "# comment\nspec-path: code/[branch]/docs/specs/<slug>/\nspec-filename: spec.md\nrun:\n  - name: web\n    cmd: pnpm dev\n"
	if err := os.WriteFile(filepath.Join(root, ".project-structure"), []byte(ps), 0o644); err != nil {
		t.Fatal(err)
	}

	c := Resolve(root)
	if c.SpecPath != "code/[branch]/docs/specs/<slug>/" {
		t.Fatalf("SpecPath = %q", c.SpecPath)
	}
	if c.SpecFilename != "spec.md" {
		t.Fatalf("SpecFilename = %q", c.SpecFilename)
	}
	if c.SpecStore != StoreConvention {
		t.Fatalf("SpecStore = %q", c.SpecStore)
	}
	if c.Source != SourceProjectStructure {
		t.Fatalf("Source = %q", c.Source)
	}
}

func TestResolveDetectsConvention(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}

	c := Resolve(root)
	if c.SpecPath != "docs/specs/<slug>/" || c.Source != SourceDetected {
		t.Fatalf("got SpecPath=%q Source=%q", c.SpecPath, c.Source)
	}
}

func TestResolveFallsBackToVector(t *testing.T) {
	root := t.TempDir()

	c := Resolve(root)
	if c.SpecPath != VectorFallbackSpecPath {
		t.Fatalf("SpecPath = %q, want %q", c.SpecPath, VectorFallbackSpecPath)
	}
	if c.SpecStore != StoreVector || c.Source != SourceDefault {
		t.Fatalf("got SpecStore=%q Source=%q", c.SpecStore, c.Source)
	}
}

func TestProjectStructureWinsOverDetection(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".project-structure"), []byte("spec-path: openspec/changes/<slug>/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := Resolve(root)
	if c.Source != SourceProjectStructure || c.SpecPath != "openspec/changes/<slug>/" {
		t.Fatalf(".project-structure should win: got Source=%q SpecPath=%q", c.Source, c.SpecPath)
	}
}

func TestFindSpecDocs(t *testing.T) {
	root := t.TempDir()
	for _, slug := range []string{"add-auth", "dark-mode"} {
		dir := filepath.Join(root, "docs", "specs", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := &Config{SpecPath: "docs/specs/<slug>/", SpecFilename: "spec.md", SpecStore: StoreConvention}

	docs, err := c.FindSpecDocs(root)
	if err != nil {
		t.Fatalf("FindSpecDocs: %v", err)
	}
	got := map[string]string{}
	for _, d := range docs {
		got[d.Slug] = d.Rel
	}
	if got["add-auth"] != "docs/specs/add-auth/spec.md" || got["dark-mode"] != "docs/specs/dark-mode/spec.md" {
		t.Fatalf("unexpected docs: %+v", got)
	}
}

func TestFindSpecDocsWorktreeTemplate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "code", "feat-x", "docs", "specs", "baz")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Config{SpecPath: "code/[branch]/docs/specs/<slug>/", SpecFilename: "spec.md", SpecStore: StoreConvention, Branch: "feat-x"}

	docs, err := c.FindSpecDocs(root)
	if err != nil {
		t.Fatalf("FindSpecDocs: %v", err)
	}
	if len(docs) != 1 || docs[0].Slug != "baz" || docs[0].Rel != "code/feat-x/docs/specs/baz/spec.md" {
		t.Fatalf("worktree template extraction failed: %+v", docs)
	}
}

func TestFindSpecDocsCollapsesWorktreesPreferBranch(t *testing.T) {
	root := t.TempDir()
	for _, wt := range []string{"main", "feat-a", "feat-b"} {
		dir := filepath.Join(root, "code", wt, "docs", "specs", "shared")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# shared"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := &Config{SpecPath: "code/[branch]/docs/specs/<slug>/", SpecFilename: "spec.md", SpecStore: StoreConvention, Branch: "main"}

	docs, err := c.FindSpecDocs(root)
	if err != nil {
		t.Fatalf("FindSpecDocs: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("3 worktree copies should collapse to 1 doc, got %d: %+v", len(docs), docs)
	}
	if docs[0].Branch != "main" || docs[0].Rel != "code/main/docs/specs/shared/spec.md" {
		t.Fatalf("canonical should prefer main: %+v", docs[0])
	}
}

func TestFindSpecDocsSeesInProgressWorktree(t *testing.T) {
	root := t.TempDir()
	// slug "feat-x" lives only in worktree "feat-x" (not yet merged to main).
	dir := filepath.Join(root, "code", "feat-x", "docs", "specs", "feat-x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Config{SpecPath: "code/[branch]/docs/specs/<slug>/", SpecFilename: "spec.md", SpecStore: StoreConvention, Branch: "main"}

	docs, err := c.FindSpecDocs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].Branch != "feat-x" {
		t.Fatalf("an in-progress spec must be visible in its own worktree even when Branch=main: %+v", docs)
	}
}

func TestFindSpecDocsSupersededFrontmatter(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "specs", "old-spec")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("---\nsupersededBy: the-change\n---\n# old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Config{SpecPath: "docs/specs/<slug>/", SpecFilename: "spec.md", SpecStore: StoreConvention}

	docs, err := c.FindSpecDocs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || !docs[0].Superseded || docs[0].SupersededBy != "the-change" {
		t.Fatalf("supersededBy frontmatter not parsed: %+v", docs)
	}
}

func TestChangesDirsAcrossWorktrees(t *testing.T) {
	root := t.TempDir()
	for _, wt := range []string{"main", "feat-a"} {
		if err := os.MkdirAll(filepath.Join(root, "code", wt, "openspec", "changes"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	c := &Config{ChangesPath: "code/[branch]/openspec/changes"}

	dirs, err := c.ChangesDirs(root)
	if err != nil {
		t.Fatalf("ChangesDirs: %v", err)
	}
	if len(dirs) != 2 || dirs[0].Branch != "feat-a" || dirs[1].Branch != "main" {
		t.Fatalf("dirs = %+v, want feat-a then main", dirs)
	}
	if dirs[1].Dir != filepath.Join(root, "code", "main", "openspec", "changes") {
		t.Fatalf("main changes dir = %q", dirs[1].Dir)
	}
}

func TestChangesDirsSimpleRepo(t *testing.T) {
	root := t.TempDir()
	c := &Config{} // no ChangesPath → default openspec/changes, single dir, no [branch]
	dirs, err := c.ChangesDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 1 || dirs[0].Branch != "" || dirs[0].Dir != filepath.Join(root, "openspec", "changes") {
		t.Fatalf("simple repo dirs = %+v", dirs)
	}
}

func TestFindSpecDocsSkipsVectorStore(t *testing.T) {
	root := t.TempDir()
	c := &Config{SpecPath: VectorFallbackSpecPath, SpecFilename: "spec.md", SpecStore: StoreVector}
	docs, err := c.FindSpecDocs(root)
	if err != nil {
		t.Fatalf("FindSpecDocs: %v", err)
	}
	if docs != nil {
		t.Errorf("vector store should return no docs, got %v", docs)
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := Resolve(root)
	if err := Write(root, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !Exists(root) {
		t.Fatal("Exists = false after Write")
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if *got != *want {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}
