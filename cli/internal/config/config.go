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

// SpecDoc is an authored spec document found at the repo's spec convention. A
// slug maps to ONE SpecDoc even when the file is checked out in several worktrees
// (Rel/Branch are the canonical copy).
type SpecDoc struct {
	Slug         string // the <slug> segment of SpecPath
	Rel          string // repo-relative path to the canonical copy
	Branch       string // worktree of the canonical copy ("" for non-worktree repos)
	SupersededBy string // frontmatter supersededBy: this spec is covered by that change
	Superseded   bool   // SupersededBy set, or frontmatter status is superseded/implemented
}

// FindSpecDocs scans the repo's spec convention for authored spec docs and
// collapses worktree copies of the same slug into ONE canonical SpecDoc (identity
// is the slug, never (worktree, slug)). The canonical copy prefers the configured
// Branch, then a worktree named after the slug (an in-progress idea/change not yet
// merged), then the lexically-first one — so a spec that only lives in its own
// worktree is never hidden. Frontmatter supersededBy/status is parsed so callers
// can suppress specs already represented by a change. Returns nil unless SpecStore
// is convention.
func (c *Config) FindSpecDocs(repoRoot string) ([]SpecDoc, error) {
	if c.SpecStore != StoreConvention {
		return nil, nil
	}
	filename := c.SpecFilename
	if filename == "" {
		filename = defaultSpecFilename
	}
	tmpl := path.Join(c.SpecPath, filename)
	glob := strings.NewReplacer("<slug>", "*", branchPlaceholder, "*").Replace(tmpl)
	re, err := compileTemplate(tmpl)
	if err != nil {
		return nil, err
	}
	slugIdx := re.SubexpIndex("slug")
	if slugIdx < 0 {
		return nil, nil // no <slug> placeholder → nothing to extract
	}
	branchIdx := re.SubexpIndex("branch")

	matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(glob)))
	if err != nil {
		return nil, fmt.Errorf("glob spec docs: %w", err)
	}

	bySlug := map[string][]specCopy{}
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
		branch := ""
		if branchIdx >= 0 {
			branch = sm[branchIdx]
		}
		bySlug[sm[slugIdx]] = append(bySlug[sm[slugIdx]], specCopy{branch: branch, rel: relSlash})
	}

	slugs := make([]string, 0, len(bySlug))
	for s := range bySlug {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)

	docs := make([]SpecDoc, 0, len(slugs))
	for _, slug := range slugs {
		canon := pickCanonical(bySlug[slug], c.Branch, slug)
		sup, superseded := parseSupersede(filepath.Join(repoRoot, filepath.FromSlash(canon.rel)))
		docs = append(docs, SpecDoc{Slug: slug, Rel: canon.rel, Branch: canon.branch, SupersededBy: sup, Superseded: superseded})
	}
	return docs, nil
}

// specCopy is one physical copy of a spec doc (in a given worktree).
type specCopy struct{ branch, rel string }

// pickCanonical chooses the canonical copy of a slug: prefer the configured
// branch, then a worktree named after the slug, then the lexically-first branch.
func pickCanonical(copies []specCopy, preferBranch, slug string) specCopy {
	sort.Slice(copies, func(i, j int) bool { return copies[i].branch < copies[j].branch })
	if preferBranch != "" {
		for _, c := range copies {
			if c.branch == preferBranch {
				return c
			}
		}
	}
	for _, c := range copies {
		if c.branch == slug {
			return c
		}
	}
	return copies[0]
}

// parseSupersede reads a spec doc's leading YAML frontmatter for supersededBy /
// status. No frontmatter (or absent keys) → not superseded.
func parseSupersede(docPath string) (supersededBy string, superseded bool) {
	b, err := os.ReadFile(docPath)
	if err != nil {
		return "", false
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "supersededBy":
			supersededBy = strings.TrimSpace(val)
		case "status":
			switch strings.ToLower(strings.TrimSpace(val)) {
			case "superseded", "implemented":
				superseded = true
			}
		}
	}
	if supersededBy != "" {
		superseded = true
	}
	return supersededBy, superseded
}

// ChangesDir is one OpenSpec changes directory (one per worktree under [branch]).
type ChangesDir struct {
	Branch string // worktree name ("" for non-worktree repos)
	Dir    string // absolute changes directory
}

// ChangesDirs returns each OpenSpec changes directory: one per worktree when the
// changes template uses [branch] (so in-progress changes living only in their own
// worktree are visible), else a single directory.
func (c *Config) ChangesDirs(repoRoot string) ([]ChangesDir, error) {
	tmpl := c.changesTemplate()
	if !strings.Contains(tmpl, branchPlaceholder) {
		return []ChangesDir{{Dir: filepath.Join(repoRoot, filepath.FromSlash(tmpl))}}, nil
	}
	glob := strings.ReplaceAll(tmpl, branchPlaceholder, "*")
	re, err := compileTemplate(tmpl)
	if err != nil {
		return nil, err
	}
	idx := re.SubexpIndex("branch")
	matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(glob)))
	if err != nil {
		return nil, fmt.Errorf("glob changes dirs: %w", err)
	}
	out := make([]ChangesDir, 0, len(matches)+1)
	for _, m := range matches {
		branch := ""
		if idx >= 0 {
			if rel, err := filepath.Rel(repoRoot, m); err == nil {
				if sm := re.FindStringSubmatch(filepath.ToSlash(rel)); sm != nil {
					branch = sm[idx]
				}
			}
		}
		out = append(out, ChangesDir{Branch: branch, Dir: m})
	}
	// Also include a root-level openspec/changes tree when present: in bare+worktree
	// layouts the archived/historical changes accumulate at the root while active
	// ones live in the worktrees, so reading only [branch] would miss the archive.
	if rootDir := filepath.Join(repoRoot, filepath.FromSlash(DefaultChangesPath)); isDir(rootDir) {
		dup := false
		for _, d := range out {
			if d.Dir == rootDir {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, ChangesDir{Branch: "", Dir: rootDir})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Branch < out[j].Branch })
	return out, nil
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
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
