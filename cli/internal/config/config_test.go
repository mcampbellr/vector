package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mariocampbell/vector/internal/state"
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
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestLoadValidatesDefaultTicketProvider(t *testing.T) {
	root := t.TempDir()
	cfg := Resolve(root)
	cfg.DefaultTicketProvider = "jirra" // typo
	cfg.TicketKeyPrefixes = []string{" mh ", ""}
	if err := Write(root, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("expected Load to reject an invalid defaultTicketProvider")
	}

	// A valid provider loads, and prefixes normalize to upper/trimmed/non-empty.
	cfg.DefaultTicketProvider = state.TicketJira
	if err := Write(root, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ResolvedDefaultTicketProvider() != state.TicketJira {
		t.Errorf("ResolvedDefaultTicketProvider = %q, want jira", loaded.ResolvedDefaultTicketProvider())
	}
	if got := loaded.NormalizedTicketKeyPrefixes(); len(got) != 1 || got[0] != "MH" {
		t.Errorf("NormalizedTicketKeyPrefixes = %v, want [MH]", got)
	}
}

func TestLanguageRoundTripAndResolve(t *testing.T) {
	root := t.TempDir()
	cfg := Resolve(root)
	cfg.Language = "  es-MX  "
	if err := Write(root, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Language != "  es-MX  " {
		t.Errorf("Language not round-tripped: got %q", loaded.Language)
	}
	if got := loaded.ResolvedLanguage(); got != "es-MX" {
		t.Errorf("ResolvedLanguage() = %q, want trimmed es-MX", got)
	}

	// Omitted when empty: the field carries omitempty, so a language-less config
	// must not serialize a "language" key.
	cfg.Language = ""
	if err := Write(root, cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "language") {
		t.Errorf("empty Language should be omitted from JSON, got: %s", b)
	}
}

func TestLoadLegacyConfigWithoutLanguage(t *testing.T) {
	root := t.TempDir()
	// A legacy config predating the field deserializes cleanly with Language == "".
	legacy := `{"schemaVersion":1,"specPath":".vector/specs/<slug>/","specFilename":"spec.md","specStore":"vector","source":"default"}`
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load legacy: %v", err)
	}
	if loaded.Language != "" || loaded.ResolvedLanguage() != "" {
		t.Errorf("legacy config Language = %q, want empty", loaded.Language)
	}
	if loaded.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1 (no migration)", loaded.SchemaVersion)
	}
}

func TestWorktreeTicketKeys(t *testing.T) {
	root := t.TempDir()
	// Worktree layout: branches one or two levels under the "code" root, with
	// grouping folders (feat/chore) in between and a single-level branch (develop).
	for _, branch := range []string{
		"feat/mh-1592-payments-period-checkout",
		"chore/MH-880-cleanup",
		"develop",                 // single-level branch, no key → not indexed
		"feat/adr-7-architecture", // denylisted prefix → not indexed
		"feat/rfc-3-protocol",     // denylisted prefix → not indexed
		"feat/mh-2001",            // bare key, no slug → not indexed
	} {
		if err := os.MkdirAll(filepath.Join(root, "code", filepath.FromSlash(branch)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &Config{ChangesPath: "code/[branch]/openspec/changes"}
	idx, err := cfg.WorktreeTicketKeys(root)
	if err != nil {
		t.Fatalf("WorktreeTicketKeys: %v", err)
	}
	want := map[string]string{
		"payments-period-checkout": "MH-1592", // upper-normalized from mh-1592
		"cleanup":                  "MH-880",
	}
	if !reflect.DeepEqual(idx, want) {
		t.Fatalf("index = %v, want %v", idx, want)
	}
}

func TestWorktreeTicketKeysDuplicateSlugOmitted(t *testing.T) {
	root := t.TempDir()
	// Two distinct keys claim the same slug → ambiguous, omitted.
	for _, branch := range []string{"feat/mh-1-dashboard", "chore/eng-9-dashboard"} {
		if err := os.MkdirAll(filepath.Join(root, "code", filepath.FromSlash(branch)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &Config{ChangesPath: "code/[branch]/openspec/changes"}
	idx, err := cfg.WorktreeTicketKeys(root)
	if err != nil {
		t.Fatalf("WorktreeTicketKeys: %v", err)
	}
	if _, ok := idx["dashboard"]; ok {
		t.Fatalf("ambiguous slug must be omitted, got %v", idx)
	}
}

func TestWorktreeTicketKeysNoBranchTemplate(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", "MH-1-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Default (non-worktree) template has no [branch] → feature inert, empty map.
	cfg := &Config{}
	idx, err := cfg.WorktreeTicketKeys(root)
	if err != nil {
		t.Fatalf("WorktreeTicketKeys: %v", err)
	}
	if len(idx) != 0 {
		t.Fatalf("expected empty index without [branch], got %v", idx)
	}
}

func TestWorktreeTicketKeysMissingRoot(t *testing.T) {
	root := t.TempDir() // no "code" dir created
	cfg := &Config{ChangesPath: "code/[branch]/openspec/changes"}
	idx, err := cfg.WorktreeTicketKeys(root)
	if err != nil {
		t.Fatalf("a missing worktree root is not an error: %v", err)
	}
	if len(idx) != 0 {
		t.Fatalf("expected empty index, got %v", idx)
	}
}

// TestDetectBuildCmds verifies manifest-based command inference across Go,
// Node, Makefile, Python, and edge-case scenarios.
func TestDetectBuildCmds(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(root string)
		wantBuild string
		wantLint  string
		wantTest  string
	}{
		{
			name: "go_mod_only",
			setup: func(root string) {
				if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.22\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantBuild: "go build ./...",
			wantLint:  "golangci-lint run",
			wantTest:  "go test ./...",
		},
		{
			name: "package_json_with_scripts",
			setup: func(root string) {
				pkg := `{"name":"app","scripts":{"build":"tsc","lint":"eslint .","test":"jest"}}`
				if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkg), 0o644); err != nil {
					t.Fatal(err)
				}
				// npm fallback (no lock file)
			},
			wantBuild: "npm run build",
			wantLint:  "npm run lint",
			wantTest:  "npm run test",
		},
		{
			name: "package_json_with_pnpm_lock",
			setup: func(root string) {
				pkg := `{"scripts":{"build":"vite build","test":"vitest"}}`
				if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkg), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte(""), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantBuild: "pnpm run build",
			wantLint:  "",
			wantTest:  "pnpm run test",
		},
		{
			name: "makefile_with_all_targets",
			setup: func(root string) {
				makefile := ".PHONY: build lint test\nbuild:\n\tgo build ./...\nlint:\n\tgolangci-lint run\ntest:\n\tgo test ./...\n"
				if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(makefile), 0o644); err != nil {
					t.Fatal(err)
				}
				// also go.mod — Makefile must win
				if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.22\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantBuild: "make build",
			wantLint:  "make lint",
			wantTest:  "make test",
		},
		{
			name: "makefile_partial_and_go_fills_gap",
			setup: func(root string) {
				// Makefile has build+test but no lint; go.mod supplies golangci-lint.
				makefile := ".PHONY: build test\nbuild:\n\tgo build ./...\ntest:\n\tgo test ./...\n"
				if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(makefile), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.22\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantBuild: "make build",
			wantLint:  "golangci-lint run",
			wantTest:  "make test",
		},
		{
			name:      "no_manifests_returns_empty",
			setup:     func(root string) { /* nothing */ },
			wantBuild: "",
			wantLint:  "",
			wantTest:  "",
		},
		{
			name: "package_json_without_scripts_field",
			setup: func(root string) {
				if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"app"}`), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantBuild: "",
			wantLint:  "",
			wantTest:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(root)
			gotBuild, gotLint, gotTest := DetectBuildCmds(root)
			if gotBuild != tc.wantBuild {
				t.Errorf("build = %q, want %q", gotBuild, tc.wantBuild)
			}
			if gotLint != tc.wantLint {
				t.Errorf("lint = %q, want %q", gotLint, tc.wantLint)
			}
			if gotTest != tc.wantTest {
				t.Errorf("test = %q, want %q", gotTest, tc.wantTest)
			}
		})
	}
}

func TestResolvedBuildCmdsRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := Resolve(root)
	cfg.BuildCmd = "go build ./..."
	cfg.LintCmd = "golangci-lint run"
	cfg.TestCmd = "go test ./..."
	if err := Write(root, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	gotBuild, gotLint, gotTest := loaded.ResolvedBuildCmds()
	if gotBuild != "go build ./..." || gotLint != "golangci-lint run" || gotTest != "go test ./..." {
		t.Errorf("round trip mismatch: build=%q lint=%q test=%q", gotBuild, gotLint, gotTest)
	}
}

func TestBuildCmdsOmitEmptyInJSON(t *testing.T) {
	root := t.TempDir()
	cfg := Resolve(root)
	// No build commands set — they must be omitted from the JSON.
	if err := Write(root, cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"buildCmd", "lintCmd", "testCmd"} {
		if strings.Contains(string(b), key) {
			t.Errorf("empty %s should be omitted from JSON, got: %s", key, b)
		}
	}
}
