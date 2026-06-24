// Package config owns Vector's per-repo configuration at .vector/config.json —
// the source of truth for repo conventions, starting with where authored spec
// docs live. It is Vector's own successor to .project-structure: `vector init`
// migrates from that legacy file once, after which Vector reads only its own
// config. The binary is the sole writer (CLI-owns-writes).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SchemaVersion guards migrations of the on-disk config format.
const SchemaVersion = 1

const defaultSpecFilename = "spec.md"

// VectorFallbackSpecPath stores spec docs inside .vector when the repo declares
// no spec convention of its own. Placeholders: <slug> (and [branch], resolved
// by the spec-authoring flow).
const VectorFallbackSpecPath = ".vector/specs/<slug>/"

// SpecStore says where a spec's authored doc lives.
type SpecStore string

const (
	StoreConvention SpecStore = "convention" // repo declares/uses its own spec location
	StoreVector     SpecStore = "vector"     // no repo convention; specs live under .vector
)

// Source records how SpecPath was resolved, for transparency on re-init.
type Source string

const (
	SourceProjectStructure Source = "project-structure" // migrated from .project-structure
	SourceDetected         Source = "detected"          // auto-detected a known convention
	SourceDefault          Source = "default"           // no convention; .vector fallback
)

// Config is Vector's per-repo config at .vector/config.json.
type Config struct {
	SchemaVersion int       `json:"schemaVersion"`
	SpecPath      string    `json:"specPath"` // dir template; may contain <slug> and [branch]
	SpecFilename  string    `json:"specFilename"`
	SpecStore     SpecStore `json:"specStore"`
	Source        Source    `json:"source"`
	// ChangesPath is the OpenSpec changes root template (may contain [branch]);
	// defaults to "openspec/changes". Branch resolves the [branch] placeholder in
	// ChangesPath and SpecPath to a concrete worktree, for bare+worktree layouts.
	ChangesPath string `json:"changesPath,omitempty"`
	Branch      string `json:"branch,omitempty"`
	// KitVersion records the binary/kit version that last seeded this repo's
	// .claude artifacts, so `vector update` can report staleness. Stamped by
	// `vector init` and `vector update`.
	KitVersion string `json:"kitVersion,omitempty"`
}

// DefaultChangesPath is used when no convention declares one.
const DefaultChangesPath = "openspec/changes"

const branchPlaceholder = "[branch]"

// changesTemplate returns ChangesPath or the default.
func (c *Config) changesTemplate() string {
	if c.ChangesPath != "" {
		return c.ChangesPath
	}
	return DefaultChangesPath
}

// NeedsBranch reports whether any path template uses the [branch] placeholder,
// so a concrete Branch must be resolved before scanning.
func (c *Config) NeedsBranch() bool {
	return strings.Contains(c.changesTemplate(), branchPlaceholder) ||
		strings.Contains(c.SpecPath, branchPlaceholder)
}

// ChangesDir returns the absolute OpenSpec changes directory, resolving [branch]
// via Branch. Callers must ensure Branch is set when NeedsBranch is true.
func (c *Config) ChangesDir(repoRoot string) string {
	p := strings.ReplaceAll(c.changesTemplate(), branchPlaceholder, c.Branch)
	return filepath.Join(repoRoot, filepath.FromSlash(p))
}

// BranchCandidates lists worktree branches that have an OpenSpec changes tree,
// by globbing the [branch] placeholder in ChangesPath. Empty when ChangesPath
// has no [branch] placeholder. Sorted and de-duplicated.
func (c *Config) BranchCandidates(repoRoot string) ([]string, error) {
	tmpl := c.changesTemplate()
	if !strings.Contains(tmpl, branchPlaceholder) {
		return nil, nil
	}
	glob := strings.ReplaceAll(tmpl, branchPlaceholder, "*")
	re, err := compileTemplate(tmpl)
	if err != nil {
		return nil, err
	}
	idx := re.SubexpIndex("branch")
	if idx < 0 {
		return nil, nil
	}
	matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(glob)))
	if err != nil {
		return nil, fmt.Errorf("glob changes dirs: %w", err)
	}
	set := map[string]bool{}
	for _, m := range matches {
		rel, err := filepath.Rel(repoRoot, m)
		if err != nil {
			continue
		}
		if sm := re.FindStringSubmatch(filepath.ToSlash(rel)); sm != nil {
			set[sm[idx]] = true
		}
	}
	out := make([]string, 0, len(set))
	for b := range set {
		out = append(out, b)
	}
	sort.Strings(out)
	return out, nil
}

// Path returns the absolute path to a repo's .vector/config.json.
func Path(repoRoot string) string {
	return filepath.Join(repoRoot, ".vector", "config.json")
}

// SpecDocPath returns where a spec's authored doc lives for the given slug: the
// repo-relative path (stored in state as a pointer) and the absolute path (where
// the doc is written). Placeholders <slug> and [branch] both resolve to slug —
// matching the worktree==slug convention; a precise branch resolver can come later.
func (c *Config) SpecDocPath(repoRoot, slug string) (rel, abs string) {
	dir := strings.ReplaceAll(c.SpecPath, "<slug>", slug)
	dir = strings.ReplaceAll(dir, "[branch]", slug)
	filename := c.SpecFilename
	if filename == "" {
		filename = defaultSpecFilename
	}
	rel = filepath.ToSlash(filepath.Join(filepath.FromSlash(dir), filename))
	abs = filepath.Join(repoRoot, filepath.FromSlash(rel))
	return rel, abs
}

// Exists reports whether a config already exists for repoRoot.
func Exists(repoRoot string) bool {
	_, err := os.Stat(Path(repoRoot))
	return err == nil
}

// Load reads an existing config.
func Load(repoRoot string) (*Config, error) {
	b, err := os.ReadFile(Path(repoRoot))
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &c, nil
}

// Resolve builds a config for repoRoot: migrate from .project-structure if it
// declares a spec location, else auto-detect a known convention, else fall back
// to .vector storage. It never reads or writes Vector's config; callers persist
// with Write.
func Resolve(repoRoot string) *Config {
	if c := fromProjectStructure(repoRoot); c != nil {
		return c
	}
	if c := detect(repoRoot); c != nil {
		return c
	}
	return &Config{
		SchemaVersion: SchemaVersion,
		SpecPath:      VectorFallbackSpecPath,
		SpecFilename:  defaultSpecFilename,
		SpecStore:     StoreVector,
		Source:        SourceDefault,
	}
}

// SpecDoc is an authored spec document found at the repo's spec convention.
type SpecDoc struct {
	Slug string // the <slug> segment of SpecPath
	Rel  string // repo-relative path to the doc
}

// FindSpecDocs scans the repo's spec convention for authored spec docs (e.g.
// /idea specs with no OpenSpec change) and returns each with its slug. Returns
// nil unless SpecStore is convention — the .vector fallback has no external specs
// to discover beyond what /vector:raw already tracked.
func (c *Config) FindSpecDocs(repoRoot string) ([]SpecDoc, error) {
	if c.SpecStore != StoreConvention {
		return nil, nil
	}
	if strings.Contains(c.SpecPath, branchPlaceholder) && c.Branch == "" {
		return nil, fmt.Errorf("specPath uses [branch] but no branch is resolved (set it in config or pass --branch)")
	}
	filename := c.SpecFilename
	if filename == "" {
		filename = defaultSpecFilename
	}
	// Resolve [branch] to the concrete worktree first, so the match is deterministic.
	tmpl := strings.ReplaceAll(path.Join(c.SpecPath, filename), branchPlaceholder, c.Branch)

	glob := strings.ReplaceAll(tmpl, "<slug>", "*")
	re, err := compileTemplate(tmpl)
	if err != nil {
		return nil, err
	}
	slugIdx := re.SubexpIndex("slug")
	if slugIdx < 0 {
		return nil, nil // no <slug> placeholder → nothing to extract
	}

	matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(glob)))
	if err != nil {
		return nil, fmt.Errorf("glob spec docs: %w", err)
	}
	var docs []SpecDoc
	for _, m := range matches {
		rel, err := filepath.Rel(repoRoot, m)
		if err != nil {
			continue
		}
		relSlash := filepath.ToSlash(rel)
		sm := re.FindStringSubmatch(relSlash)
		if sm == nil {
			continue
		}
		docs = append(docs, SpecDoc{Slug: sm[slugIdx], Rel: relSlash})
	}
	return docs, nil
}

// compileTemplate compiles a forward-slash path template into an anchored regex
// with named captures for whichever placeholders it contains: <slug> → "slug"
// and [branch] → "branch". Absent placeholders simply have no group.
func compileTemplate(tmpl string) (*regexp.Regexp, error) {
	var sb strings.Builder
	sb.WriteByte('^')
	for i := 0; i < len(tmpl); {
		switch {
		case strings.HasPrefix(tmpl[i:], "<slug>"):
			sb.WriteString(`(?P<slug>[^/]+)`)
			i += len("<slug>")
		case strings.HasPrefix(tmpl[i:], branchPlaceholder):
			sb.WriteString(`(?P<branch>[^/]+)`)
			i += len(branchPlaceholder)
		default:
			sb.WriteString(regexp.QuoteMeta(tmpl[i : i+1]))
			i++
		}
	}
	sb.WriteByte('$')
	return regexp.Compile(sb.String())
}

// Write persists the config to .vector/config.json via an atomic rename.
func Write(repoRoot string, c *Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return writeFileAtomic(Path(repoRoot), append(b, '\n'))
}

// fromProjectStructure migrates spec-path / spec-filename out of a legacy
// .project-structure file, if present and declaring spec-path. Only top-level
// `key: value` lines are read; nested blocks (run:, tunnel:) are ignored.
func fromProjectStructure(repoRoot string) *Config {
	b, err := os.ReadFile(filepath.Join(repoRoot, ".project-structure"))
	if err != nil {
		return nil
	}
	specPath, specFile, changesPath, branch := "", "", "", ""
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue // skip comments, blanks, and nested (indented) entries
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "spec-path":
			specPath = strings.TrimSpace(val)
		case "spec-filename":
			specFile = strings.TrimSpace(val)
		case "changes-path":
			changesPath = strings.TrimSpace(val)
		case "base-branch":
			branch = strings.TrimSpace(val)
		}
	}
	if specPath == "" {
		return nil
	}
	if specFile == "" {
		specFile = defaultSpecFilename
	}
	if changesPath == "" {
		changesPath = deriveChangesPath(specPath)
	}
	return &Config{
		SchemaVersion: SchemaVersion,
		SpecPath:      specPath,
		SpecFilename:  specFile,
		SpecStore:     StoreConvention,
		Source:        SourceProjectStructure,
		ChangesPath:   changesPath,
		Branch:        branch,
	}
}

// deriveChangesPath infers the OpenSpec changes root from a spec-path template:
// the worktree root (everything through the [branch] segment) + openspec/changes,
// or the plain default for non-worktree repos. Generic — no repo-specific names.
func deriveChangesPath(specPath string) string {
	if i := strings.Index(specPath, branchPlaceholder); i >= 0 {
		root := specPath[:i+len(branchPlaceholder)] // e.g. "code/[branch]"
		return path.Join(root, DefaultChangesPath)
	}
	return DefaultChangesPath
}

// detect looks for a well-known spec directory convention in the repo, in
// priority order. Returns nil if none is found.
func detect(repoRoot string) *Config {
	for _, cand := range []struct{ dir, path string }{
		{"docs/specs", "docs/specs/<slug>/"},
		{"openspec/changes", "openspec/changes/<slug>/"},
		{"specs", "specs/<slug>/"},
	} {
		if info, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(cand.dir))); err == nil && info.IsDir() {
			return &Config{
				SchemaVersion: SchemaVersion,
				SpecPath:      cand.path,
				SpecFilename:  defaultSpecFilename,
				SpecStore:     StoreConvention,
				Source:        SourceDetected,
			}
		}
	}
	return nil
}

// writeFileAtomic writes data via a temp file in the same directory and an
// atomic rename, so readers never observe a partial file. (Mirrors the helper
// in internal/state and internal/scaffold; a shared internal/fsutil is a
// reasonable future cleanup.)
func writeFileAtomic(targetPath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
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
