package intel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Domain is one fingerprint domain: a slice of repo knowledge with its own
// authoritative source set and digest. A domain is validated (and regenerated)
// independently, so a change to one set of sources never invalidates unrelated
// knowledge (docs/knowledge-architecture.md §5).
type Domain string

const (
	// DomainStack protects techstack, framework and runtime (manifests, tsconfig,
	// framework configs).
	DomainStack Domain = "stack"
	// DomainDeps protects the dependency graph (lockfiles only).
	DomainDeps Domain = "deps"
	// DomainBuild protects build/lint/test/format commands (Makefile, package.json
	// scripts, turbo.json).
	DomainBuild Domain = "build"
	// DomainWorkspace protects the mono/micro layout (workspace configs + root manifest).
	DomainWorkspace Domain = "workspace"
	// DomainStructure protects the tree index and entry points (git ls-files digest +
	// untracked-not-ignored count + submodule SHAs).
	DomainStructure Domain = "structure"
)

// AllDomains is the canonical ordered set of the five fixed domains.
var AllDomains = []Domain{DomainStack, DomainDeps, DomainBuild, DomainWorkspace, DomainStructure}

// domainDependents encodes the invalidation DAG between domains: invalidating a
// key invalidates (transitively) its dependents (docs/knowledge-architecture.md
// §5). stack → deps (a manifest change can shift resolved dependencies). The
// structure → entry-points edge lives inside the structure artifact itself
// (entry points are regenerated with structure-index.json), so it needs no
// separate domain edge here.
var domainDependents = map[Domain][]Domain{
	DomainStack: {DomainDeps},
}

// InvalidatedBy returns the transitive closure of domains that must be
// invalidated when d is invalidated, excluding d itself, sorted for determinism.
func InvalidatedBy(d Domain) []Domain {
	seen := map[Domain]bool{}
	var walk func(Domain)
	walk = func(x Domain) {
		for _, dep := range domainDependents[x] {
			if !seen[dep] {
				seen[dep] = true
				walk(dep)
			}
		}
	}
	walk(d)
	out := make([]Domain, 0, len(seen))
	for dep := range seen {
		out = append(out, dep)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// matcher selects authoritative source files by base-name glob. rootOnly limits
// the match to files directly at the repo root (no slash in the relative path).
type matcher struct {
	base     string
	rootOnly bool
}

func (m matcher) match(rel string) bool {
	if m.rootOnly && strings.Contains(rel, "/") {
		return false
	}
	ok, _ := filepath.Match(m.base, path.Base(rel))
	return ok
}

// domainSources lists the authoritative source globs per content domain
// (docs/knowledge-architecture.md §5, spec §7). DomainStructure is not listed:
// its digest is computed from git metadata, not file contents.
var domainSources = map[Domain][]matcher{
	DomainStack: {
		{base: "package.json"}, {base: "go.mod"}, {base: "pyproject.toml"},
		{base: "Cargo.toml"}, {base: "tsconfig*.json"},
		{base: "next.config.*"}, {base: "vite.config.*"},
	},
	DomainDeps: {
		{base: "pnpm-lock.yaml"}, {base: "package-lock.json"}, {base: "yarn.lock"},
		{base: "go.sum"}, {base: "Cargo.lock"}, {base: "poetry.lock"},
	},
	DomainBuild: {
		{base: "Makefile"}, {base: "package.json"}, {base: "turbo.json"},
	},
	DomainWorkspace: {
		{base: "pnpm-workspace.yaml"}, {base: "turbo.json"}, {base: "nx.json"},
		{base: "go.work"}, {base: "package.json", rootOnly: true}, {base: "go.mod", rootOnly: true},
	},
}

// skipDirs are directory names never walked when enumerating sources: heavy
// generated trees that never hold authoritative manifests. Hidden directories
// (".git", ".vector", ".claude", …) are skipped separately by name prefix.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true,
	"build": true, "target": true,
}

// collectFiles returns every non-ignored repo-relative file path (slash form),
// sorted lexicographically. It skips heavy generated directories and hidden
// directories so the walk is bounded and deterministic. A subtree that cannot be
// read is skipped rather than aborting the walk.
func collectFiles(repoRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry → skip it, keep walking
		}
		if d.IsDir() {
			if p == repoRoot {
				return nil
			}
			name := d.Name()
			if skipDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, p)
		if relErr != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)
	return files, err
}

// sourcesFor returns the authoritative source files of a content domain, in
// canonical (sorted) order, filtered from a pre-collected file list.
func sourcesFor(domain Domain, files []string) []string {
	ms := domainSources[domain]
	if len(ms) == 0 {
		return nil
	}
	var out []string
	for _, rel := range files {
		for _, m := range ms {
			if m.match(rel) {
				out = append(out, rel)
				break
			}
		}
	}
	return out
}

// DigestDomains computes the working-tree content digest of each requested
// domain in parallel (goroutines + sync.WaitGroup). Content domains share a
// single file walk; the structure domain reads git metadata. Returns the first
// error encountered (digests for the other domains are still populated).
func DigestDomains(repoRoot string, domains []Domain) (map[Domain]string, error) {
	files, _ := collectFiles(repoRoot) // a partial walk still yields useful digests

	out := make(map[Domain]string, len(domains))
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
	)
	for _, d := range domains {
		wg.Add(1)
		go func(dom Domain) {
			defer wg.Done()
			var (
				dig string
				err error
			)
			if dom == DomainStructure {
				dig, err = digestStructure(repoRoot)
			} else {
				dig = digestContent(repoRoot, dom, files)
			}
			mu.Lock()
			out[dom] = dig
			if err != nil && firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
		}(d)
	}
	wg.Wait()
	return out, firstErr
}

// DigestDomain computes the working-tree content digest of a single domain.
func DigestDomain(repoRoot string, domain Domain) (string, error) {
	m, err := DigestDomains(repoRoot, []Domain{domain})
	return m[domain], err
}

// digestContent hashes a domain's authoritative sources over their working-tree
// content in canonical (sorted-path) order, prefixing the result with "sha256:".
// Unreadable sources are skipped — a missing manifest simply does not contribute,
// so a repo with no manifests yields a stable empty-set digest.
func digestContent(repoRoot string, domain Domain, files []string) string {
	h := sha256.New()
	for _, rel := range sourcesFor(domain, files) {
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(content)
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// digestStructure hashes the repo's file-set identity over the working tree:
// tracked files (git ls-files), the count of untracked-not-ignored files, and
// submodule SHAs. When git is unavailable it falls back to a filesystem-walk
// digest so the domain never crashes (spec §11).
func digestStructure(repoRoot string) (string, error) {
	h := sha256.New()
	tracked, err := gitOutput(repoRoot, "ls-files")
	if err != nil {
		// Fallback: no git → digest the bounded filesystem walk instead.
		files, _ := collectFiles(repoRoot)
		h.Write([]byte("walk\n"))
		for _, f := range files {
			h.Write([]byte(f))
			h.Write([]byte{0})
		}
		return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
	}
	h.Write([]byte("tracked\n"))
	h.Write(tracked)
	if others, err := gitOutput(repoRoot, "ls-files", "--others", "--exclude-standard"); err == nil {
		fmt.Fprintf(h, "untracked:%d\n", countNonEmptyLines(others))
	}
	if sub, err := gitOutput(repoRoot, "submodule", "status"); err == nil {
		h.Write([]byte("submodules\n"))
		h.Write(sub)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// gitOutput runs `git -C repoRoot <args...>` and returns stdout. An error
// (git absent, not a repo) is returned so callers can fall back.
func gitOutput(repoRoot string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// countNonEmptyLines counts the non-empty newline-separated lines in b.
func countNonEmptyLines(b []byte) int {
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}
