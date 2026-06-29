package intel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// writeRepoFile writes content to repoRoot/rel, creating parent dirs.
func writeRepoFile(t *testing.T, repoRoot, rel, content string) {
	t.Helper()
	abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// readFingerprints loads fingerprints.json from a repo's cache.
func readFingerprints(t *testing.T, repoRoot string) Fingerprints {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(CacheDir(repoRoot), fingerprintsFile))
	if err != nil {
		t.Fatalf("read fingerprints: %v", err)
	}
	var fp Fingerprints
	if err := json.Unmarshal(b, &fp); err != nil {
		t.Fatalf("parse fingerprints: %v", err)
	}
	return fp
}

// seedGoRepo writes a minimal Go+Node polyglot repo and returns its root.
func seedGoRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeRepoFile(t, root, "cli/go.mod", "module example.com/x\n\ngo 1.26\n")
	writeRepoFile(t, root, "cli/go.sum", "")
	writeRepoFile(t, root, "cli/cmd/app/main.go", "package main\nfunc main() {}\n")
	writeRepoFile(t, root, "web/package.json", `{"name":"web","main":"src/main.tsx"}`)
	writeRepoFile(t, root, "web/src/main.tsx", "export {}\n")
	writeRepoFile(t, root, "web/tsconfig.json", "{}")
	return root
}

func TestDigestDomainDeterministic(t *testing.T) {
	root := seedGoRepo(t)
	for _, d := range AllDomains {
		a, err := DigestDomain(root, d)
		if err != nil {
			t.Fatalf("digest %s: %v", d, err)
		}
		b, err := DigestDomain(root, d)
		if err != nil {
			t.Fatalf("digest %s (2nd): %v", d, err)
		}
		if a != b {
			t.Errorf("domain %s digest not deterministic: %s != %s", d, a, b)
		}
		if a == "" || a == "sha256:" {
			t.Errorf("domain %s produced empty digest %q", d, a)
		}
	}
}

func TestDigestDomainDetectsWorkingTreeEdit(t *testing.T) {
	root := seedGoRepo(t)
	before, _ := DigestDomain(root, DomainStack)
	// Edit a stack source without any commit (working-tree edit).
	writeRepoFile(t, root, "web/tsconfig.json", `{"compilerOptions":{"strict":true}}`)
	after, _ := DigestDomain(root, DomainStack)
	if before == after {
		t.Errorf("stack digest unchanged after working-tree edit: %s", after)
	}
	// An unrelated domain (deps) must NOT change from a tsconfig edit.
	depsBefore, _ := DigestDomain(root, DomainDeps)
	writeRepoFile(t, root, "web/tsconfig.json", `{"compilerOptions":{"strict":false}}`)
	depsAfter, _ := DigestDomain(root, DomainDeps)
	if depsBefore != depsAfter {
		t.Errorf("deps digest changed from a stack-only edit: %s != %s", depsBefore, depsAfter)
	}
}

func TestInvalidatedByDAG(t *testing.T) {
	tests := []struct {
		domain Domain
		want   []Domain
	}{
		{DomainStack, []Domain{DomainDeps}},
		{DomainDeps, nil},
		{DomainStructure, nil},
		{DomainBuild, nil},
		{DomainWorkspace, nil},
	}
	for _, tt := range tests {
		got := InvalidatedBy(tt.domain)
		if len(got) != len(tt.want) {
			t.Errorf("InvalidatedBy(%s) = %v, want %v", tt.domain, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("InvalidatedBy(%s)[%d] = %s, want %s", tt.domain, i, got[i], tt.want[i])
			}
		}
	}
}

func TestResolveGeneratesAllArtifacts(t *testing.T) {
	root := seedGoRepo(t)
	cache, err := Resolve(root, "v1", AllDomains, true)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for _, name := range []string{fingerprintsFile, repoIntelFile, structureFile} {
		if _, err := os.Stat(filepath.Join(CacheDir(root), name)); err != nil {
			t.Errorf("artifact %s not generated: %v", name, err)
		}
	}
	fp := readFingerprints(t, root)
	if fp.SchemaVersion != CacheSchemaVersion {
		t.Errorf("schemaVersion = %d, want %d", fp.SchemaVersion, CacheSchemaVersion)
	}
	if fp.KitVersion != "v1" {
		t.Errorf("kitVersion = %q, want v1", fp.KitVersion)
	}
	for _, d := range AllDomains {
		if fp.Domains[d].Digest == "" {
			t.Errorf("domain %s has no digest", d)
		}
	}
	// repo-intel reflects the Go-primary polyglot repo.
	if cache.RepoIntel.PackageManager != "go-modules" {
		t.Errorf("packageManager = %q, want go-modules", cache.RepoIntel.PackageManager)
	}
	if cache.RepoIntel.Runtime.Name != "go" || cache.RepoIntel.Runtime.Version != "1.26" {
		t.Errorf("runtime = %+v, want go/1.26", cache.RepoIntel.Runtime)
	}
	// structure classifies cli (go-module) and web (node) with entry points.
	kinds := map[string]Workspace{}
	for _, ws := range cache.Structure.Workspaces {
		kinds[ws.Path] = ws
	}
	if ws, ok := kinds["cli"]; !ok || ws.Kind != "go-module" {
		t.Errorf("cli workspace missing or wrong kind: %+v", ws)
	} else if !contains(ws.EntryPoints, "cmd/app/main.go") {
		t.Errorf("cli entry points = %v, want cmd/app/main.go", ws.EntryPoints)
	}
	if ws, ok := kinds["web"]; !ok || ws.Kind != "node" {
		t.Errorf("web workspace missing or wrong kind: %+v", ws)
	} else if !contains(ws.EntryPoints, "src/main.tsx") {
		t.Errorf("web entry points = %v, want src/main.tsx", ws.EntryPoints)
	}
}

func TestResolveCacheHitDoesNotRewrite(t *testing.T) {
	root := seedGoRepo(t)
	if _, err := Resolve(root, "v1", AllDomains, false); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	// Plant a sentinel; a cache hit must not overwrite the artifact.
	sentinel := `{"sentinel":true}`
	structPath := filepath.Join(CacheDir(root), structureFile)
	if err := os.WriteFile(structPath, []byte(sentinel), 0o644); err != nil {
		t.Fatalf("plant sentinel: %v", err)
	}
	if _, err := Resolve(root, "v1", AllDomains, false); err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	got, _ := os.ReadFile(structPath)
	if string(got) != sentinel {
		t.Errorf("cache hit rewrote structure-index.json: got %s", got)
	}
}

func TestResolveInvalidatesStaleDomain(t *testing.T) {
	root := seedGoRepo(t)
	if _, err := Resolve(root, "v1", AllDomains, false); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	before := readFingerprints(t, root).Domains[DomainStack].Digest
	// Mutate a stack source; the next read must regenerate the stack domain.
	writeRepoFile(t, root, "web/tsconfig.json", `{"compilerOptions":{"strict":true}}`)
	if _, err := Resolve(root, "v1", AllDomains, false); err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	after := readFingerprints(t, root).Domains[DomainStack].Digest
	if before == after {
		t.Errorf("stack digest not refreshed after source edit: %s", after)
	}
}

func TestResolveVersionBumpInvalidatesAll(t *testing.T) {
	root := seedGoRepo(t)
	if _, err := Resolve(root, "v1", AllDomains, false); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	sentinel := `{"sentinel":true}`
	structPath := filepath.Join(CacheDir(root), structureFile)
	if err := os.WriteFile(structPath, []byte(sentinel), 0o644); err != nil {
		t.Fatalf("plant sentinel: %v", err)
	}
	// A kitVersion bump must invalidate the whole cache and regenerate.
	if _, err := Resolve(root, "v2", AllDomains, false); err != nil {
		t.Fatalf("resolve after bump: %v", err)
	}
	got, _ := os.ReadFile(structPath)
	if string(got) == sentinel {
		t.Errorf("version bump did not regenerate structure-index.json")
	}
	if kv := readFingerprints(t, root).KitVersion; kv != "v2" {
		t.Errorf("kitVersion = %q, want v2", kv)
	}
}

func TestResolveRefreshRegeneratesAll(t *testing.T) {
	root := seedGoRepo(t)
	if _, err := Resolve(root, "v1", []Domain{DomainStack}, false); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	// --refresh validates ALL domains even when fewer are requested.
	if _, err := Resolve(root, "v1", []Domain{DomainStack}, true); err != nil {
		t.Fatalf("refresh resolve: %v", err)
	}
	fp := readFingerprints(t, root)
	for _, d := range AllDomains {
		if fp.Domains[d].Digest == "" {
			t.Errorf("refresh left domain %s unvalidated", d)
		}
	}
}

func TestResolveLazyOnlyValidatesRequested(t *testing.T) {
	root := seedGoRepo(t)
	if _, err := Resolve(root, "v1", []Domain{DomainBuild}, false); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	fp := readFingerprints(t, root)
	if fp.Domains[DomainBuild].Digest == "" {
		t.Error("build domain not validated")
	}
	// structure was not requested → not validated (lazy).
	if _, ok := fp.Domains[DomainStructure]; ok {
		t.Error("structure domain validated though not requested (not lazy)")
	}
}

func TestResolveCorruptCacheRegenerates(t *testing.T) {
	root := seedGoRepo(t)
	if err := os.MkdirAll(CacheDir(root), 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(CacheDir(root), fingerprintsFile), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	cache, err := Resolve(root, "v1", AllDomains, false)
	if err != nil {
		t.Fatalf("resolve over corrupt cache: %v", err)
	}
	if cache.Fingerprints.Domains[DomainStack].Digest == "" {
		t.Error("corrupt cache not regenerated")
	}
}

func TestResolveRepoWithoutManifests(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, "README.md", "# empty\n")
	cache, err := Resolve(root, "v1", AllDomains, false)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cache.RepoIntel.PackageManager != "" || cache.RepoIntel.Runtime.Name != "" {
		t.Errorf("expected empty stack, got %+v", cache.RepoIntel)
	}
	if !cache.Structure.Available {
		t.Error("structure should be available even with no manifests")
	}
	if len(cache.Structure.Workspaces) != 0 {
		t.Errorf("expected no workspaces, got %v", cache.Structure.Workspaces)
	}
}

func TestResolveNoGitStructureFallback(t *testing.T) {
	// t.TempDir() is not a git repo, so digestStructure exercises the fallback.
	root := seedGoRepo(t)
	dig, err := DigestDomain(root, DomainStructure)
	if err != nil {
		t.Fatalf("structure digest without git: %v", err)
	}
	if dig == "" || dig == "sha256:" {
		t.Errorf("structure fallback produced empty digest %q", dig)
	}
}

func TestResolveConcurrentAtomicWrite(t *testing.T) {
	root := seedGoRepo(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = Resolve(root, "v1", AllDomains, true)
		}()
	}
	wg.Wait()
	// The fingerprints file must be valid JSON (no torn write).
	fp := readFingerprints(t, root)
	if fp.SchemaVersion != CacheSchemaVersion {
		t.Errorf("concurrent writes corrupted fingerprints: %+v", fp)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
