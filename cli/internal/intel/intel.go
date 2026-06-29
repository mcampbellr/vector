// Package intel owns Vector's class-C knowledge cache: derived repo intelligence
// (techstack, runtime, framework, structure index, entry points) under
// .vector/cache/ (gitignored, regenerable). The Go binary is the sole producer
// and consumer (CLI-owns-writes); agents never read the cache directly — they
// receive a projected slice from `vector context`.
//
// Validity is governed by a per-domain sha256 content fingerprint over the
// working tree (not HEAD, not mtime): each of the five fixed domains (stack,
// deps, build, workspace, structure) is validated and regenerated independently,
// with a small invalidation DAG between them. A schemaVersion/kitVersion bump
// invalidates everything. See docs/knowledge-architecture.md for the full design.
package intel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheSchemaVersion guards migrations of the on-disk cache format. Independent
// of config.SchemaVersion and board.SchemaVersion: a bump invalidates the whole
// cache (every domain), so a per-domain schema version would be redundant.
const CacheSchemaVersion = 1

const (
	fingerprintsFile = "fingerprints.json"
	repoIntelFile    = "repo-intel.json"
	structureFile    = "structure-index.json"
)

// DomainFingerprint is a single domain's validity record.
type DomainFingerprint struct {
	Digest      string `json:"digest"`
	GeneratedAt string `json:"generatedAt"`
}

// Fingerprints is the validity oracle (fingerprints.json). schemaVersion and
// kitVersion live at the root: a bump of either invalidates all domains at once.
type Fingerprints struct {
	SchemaVersion int                          `json:"schemaVersion"`
	KitVersion    string                       `json:"kitVersion"`
	Domains       map[Domain]DomainFingerprint `json:"domains"`
}

// Runtime is the detected language runtime (name + optional version).
type Runtime struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// RepoIntel is repo-intel.json: stack/runtime/framework detail derived from the
// stack/deps/workspace domains.
type RepoIntel struct {
	SchemaVersion  int      `json:"schemaVersion"`
	PackageManager string   `json:"packageManager,omitempty"`
	Runtime        Runtime  `json:"runtime"`
	Frameworks     []string `json:"frameworks"`
	TsconfigPaths  []string `json:"tsconfigPaths"`
	GeneratedAt    string   `json:"generatedAt"`
}

// Workspace is one classified workspace in the structure index.
type Workspace struct {
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	EntryPoints []string `json:"entryPoints"`
}

// StructureIndex is structure-index.json: the workspace tree + entry points
// derived from the structure domain. Available is false only when no index could
// be built at all (Note explains why); Truncated marks a capped large repo.
type StructureIndex struct {
	SchemaVersion int         `json:"schemaVersion"`
	Available     bool        `json:"available"`
	Workspaces    []Workspace `json:"workspaces"`
	Truncated     bool        `json:"truncated,omitempty"`
	Note          string      `json:"note,omitempty"`
	GeneratedAt   string      `json:"generatedAt"`
}

// Cache bundles the three loaded/regenerated artifacts of .vector/cache/.
type Cache struct {
	Fingerprints Fingerprints
	RepoIntel    RepoIntel
	Structure    StructureIndex
}

// CacheDir returns the absolute path to a repo's .vector/cache/ directory.
func CacheDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".vector", "cache")
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// loadCache reads the three artifacts, tolerating absence and corruption: a
// missing or malformed file loads as a zero value (a cache miss that the next
// validation step regenerates). Never errors — a cold cache is the normal start.
// The bool reports whether a valid fingerprints.json was loaded, so the version
// guard only fires against an existing cache (a cold cache is not a "mismatch").
func loadCache(repoRoot string) (*Cache, bool) {
	c := &Cache{Fingerprints: Fingerprints{Domains: map[Domain]DomainFingerprint{}}}
	loaded := false
	dir := CacheDir(repoRoot)
	if b, err := os.ReadFile(filepath.Join(dir, fingerprintsFile)); err == nil {
		var fp Fingerprints
		if json.Unmarshal(b, &fp) == nil {
			if fp.Domains == nil {
				fp.Domains = map[Domain]DomainFingerprint{}
			}
			c.Fingerprints = fp
			loaded = true
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, repoIntelFile)); err == nil {
		_ = json.Unmarshal(b, &c.RepoIntel)
	}
	if b, err := os.ReadFile(filepath.Join(dir, structureFile)); err == nil {
		_ = json.Unmarshal(b, &c.Structure)
	}
	return c, loaded
}

// Resolve validates the requested domains against the cache, regenerating any
// stale ones (and their DAG dependents), and returns the resulting Cache. With
// refresh=true — or when the cache's schemaVersion/kitVersion no longer match —
// all five domains are regenerated. A pure cache hit performs zero writes.
//
// Domains not in the requested set are left untouched (lazy validation): a trust
// command passing no domains never touches the cache. kitVersion is the running
// binary's kit version, compared against the cached one for the version guard.
func Resolve(repoRoot, kitVersion string, domains []Domain, refresh bool) (*Cache, error) {
	c, loaded := loadCache(repoRoot)

	// The version guard fires only against an existing cache: a cold cache is a
	// plain miss (its domains regenerate on their own digest mismatch), not a
	// whole-cache invalidation that would defeat lazy per-domain validation.
	versionMismatch := loaded && (c.Fingerprints.SchemaVersion != CacheSchemaVersion ||
		c.Fingerprints.KitVersion != kitVersion)

	check := domains
	if refresh || versionMismatch {
		check = AllDomains
	}
	if len(check) == 0 {
		return c, nil // trust tier: nothing to validate, nothing to write
	}

	current, _ := DigestDomains(repoRoot, check)

	stale := map[Domain]bool{}
	for _, d := range check {
		if refresh || versionMismatch || c.Fingerprints.Domains[d].Digest != current[d] {
			stale[d] = true
		}
	}
	// Expand via the invalidation DAG; dependents may fall outside the checked set,
	// so compute their current digests too before recording fingerprints.
	for d := range stale {
		for _, dep := range InvalidatedBy(d) {
			stale[dep] = true
		}
	}
	if len(stale) == 0 {
		return c, nil // cache hit across all requested domains → no write
	}
	missing := make([]Domain, 0)
	for d := range stale {
		if _, ok := current[d]; !ok {
			missing = append(missing, d)
		}
	}
	if len(missing) > 0 {
		extra, _ := DigestDomains(repoRoot, missing)
		for d, dig := range extra {
			current[d] = dig
		}
	}

	// Regenerate the artifacts whose contributing domains went stale.
	if needsRepoIntel(stale) {
		c.RepoIntel = BuildRepoIntel(repoRoot)
	}
	if stale[DomainStructure] {
		c.Structure = BuildStructureIndex(repoRoot)
	}

	// Record fingerprints: stale domains get a fresh timestamp; first-seen checked
	// domains that matched still get persisted so subsequent runs validate cheaply.
	now := nowStamp()
	if c.Fingerprints.Domains == nil {
		c.Fingerprints.Domains = map[Domain]DomainFingerprint{}
	}
	c.Fingerprints.SchemaVersion = CacheSchemaVersion
	c.Fingerprints.KitVersion = kitVersion
	for d := range stale {
		c.Fingerprints.Domains[d] = DomainFingerprint{Digest: current[d], GeneratedAt: now}
	}
	for _, d := range check {
		if _, ok := c.Fingerprints.Domains[d]; !ok {
			c.Fingerprints.Domains[d] = DomainFingerprint{Digest: current[d], GeneratedAt: now}
		}
	}

	if err := c.persist(repoRoot); err != nil {
		return c, err
	}
	return c, nil
}

// needsRepoIntel reports whether any repo-intel contributing domain is stale.
// repo-intel.json derives from stack/deps/workspace; the build domain feeds
// config.json (not the cache) and the structure domain its own artifact.
func needsRepoIntel(stale map[Domain]bool) bool {
	return stale[DomainStack] || stale[DomainDeps] || stale[DomainWorkspace]
}

// persist writes fingerprints + the regenerated artifacts atomically.
func (c *Cache) persist(repoRoot string) error {
	dir := CacheDir(repoRoot)
	if err := writeJSONAtomic(filepath.Join(dir, fingerprintsFile), c.Fingerprints); err != nil {
		return err
	}
	if err := writeJSONAtomic(filepath.Join(dir, repoIntelFile), c.RepoIntel); err != nil {
		return err
	}
	if err := writeJSONAtomic(filepath.Join(dir, structureFile), c.Structure); err != nil {
		return err
	}
	return nil
}

// writeJSONAtomic marshals v and writes it via a temp file + rename, so a
// concurrent reader never observes a partial artifact (mirrors config.Write).
func writeJSONAtomic(targetPath string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(targetPath), err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("rename temp file into place: %w", err)
	}
	return nil
}
