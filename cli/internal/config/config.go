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
	"sync"

	"github.com/mariocampbell/vector/internal/state"
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
	// ProposeBranch overrides which worktree /vector:propose creates a change in
	// (bare+worktree layouts); falls back to Branch when empty.
	ProposeBranch string `json:"proposeBranch,omitempty"`
	// ApplyMode controls how much /vector:apply decides vs asks when selecting the
	// next work-item: "auto" (pick and start), "ask" (propose a pick, confirm) or
	// "always-ask" (always show candidates). Empty defaults to ApplyModeAsk.
	ApplyMode ApplyMode `json:"applyMode,omitempty"`
	// ApplyModel controls which model tier /vector:apply uses for the implementation
	// step: "opus" (or empty) keeps the current inline behavior; "sonnet" always
	// delegates to a Sonnet subagent; "conditional" evaluates mechanical signals and
	// routes to Sonnet only when the change qualifies. Empty = ApplyModelOpus (no
	// regression). Not written by vector init/update — strictly opt-in.
	ApplyModel ApplyModel `json:"applyModel,omitempty"`
	// Language is the prose language Vector agents write in (a BCP-47 tag like "es"
	// or a plain name like "Spanish" — free pass-through, no allow-list). Empty =
	// agents match the conversation language (current behavior). Set via
	// `vector init --language` / `vector update --language`. Additive and
	// backward-compatible: a legacy config without the field loads as "".
	Language string `json:"language,omitempty"`
	// KitVersion records the binary/kit version that last seeded this repo's
	// .claude artifacts, so `vector update` can report staleness. Stamped by
	// `vector init` and `vector update`.
	KitVersion string `json:"kitVersion,omitempty"`
	// DefaultTicketProvider, when set (jira|linear|github|other), is the fallback
	// provider for ambiguous bare ticket keys: keys detected by detectTicket during
	// sync/raw, and the key passed to `vector spec link` without --provider. Empty
	// disables that fallback (detection stays conservative — see ticket.go).
	DefaultTicketProvider state.TicketProvider `json:"defaultTicketProvider,omitempty"`
	// TicketKeyPrefixes lists project key prefixes (e.g. ["MH"]) that mark a bare
	// key as a ticket with high confidence anywhere in prose, complementing the
	// cue-word scan. Compared case-insensitively (see NormalizedTicketKeyPrefixes).
	TicketKeyPrefixes []string `json:"ticketKeyPrefixes,omitempty"`
	// BuildCmd, LintCmd, TestCmd cache the repo's detected build, lint, and test
	// commands (set by `vector init`/`vector update` via DetectBuildCmds). Empty
	// when no manifest was recognized or detection was not run; `vector context`
	// re-detects at runtime without persisting when these are empty. Additive and
	// backward-compatible: a legacy config without these fields loads them as "".
	BuildCmd string `json:"buildCmd,omitempty"`
	LintCmd  string `json:"lintCmd,omitempty"`
	TestCmd  string `json:"testCmd,omitempty"`
	// BaseBranch is the fork point for new per-spec worktrees in bare+worktree
	// layouts (the base passed to `git worktree add <path> -b <branch> <base>`).
	// Empty defaults to "main". Consulted only when the repo declares a [branch]
	// layout (HasBranchPlaceholder); additive and backward-compatible.
	BaseBranch string `json:"baseBranch,omitempty"`
	// BranchPrefix is the prefix for per-spec feature branch names in bare+worktree
	// layouts ("feat/" → "feat/<slug>"). Empty defaults to "feat/". Consulted only
	// when the repo declares a [branch] layout; additive and backward-compatible.
	BranchPrefix string `json:"branchPrefix,omitempty"`
	// SketchEnabled globally gates the opt-in Excalidraw sketch step at the tail of
	// /vector:raw and /vector:research. nil (absent) or true = enabled (the command
	// may prompt on a strong UI signal); only an explicit false suppresses the prompt
	// repo-wide. A pointer so absent and false are distinguishable; additive and
	// backward-compatible (a legacy config loads it as nil = enabled). Not written by
	// vector init/update — set it manually to opt out.
	SketchEnabled *bool `json:"sketchEnabled,omitempty"`
	// Ship carries the /vector:ship orchestration knobs (base branch, ask|auto mode,
	// default draft-PR, extra commit-exclude globs, opt-in auth bootstrap). Optional
	// and omitempty — a legacy config loads it as nil and every resolver below is
	// nil-safe. Written only by `vector config set-ship` (strictly opt-in, never by
	// init/update). SchemaVersion stays 1 (additive).
	Ship *ShipConfig `json:"ship,omitempty"`
}

// IsSketchEnabled reports whether the tail sketch step is enabled for this repo:
// true unless SketchEnabled is explicitly false. nil (absent) defaults to enabled,
// so the feature is on by default and only an explicit opt-out disables it.
func (c *Config) IsSketchEnabled() bool {
	return c.SketchEnabled == nil || *c.SketchEnabled
}

// ApplyMode controls /vector:apply autonomy (docs/apply-design.md §3).
type ApplyMode string

const (
	ApplyModeAuto      ApplyMode = "auto"       // pick the work-item and start, no prompt
	ApplyModeAsk       ApplyMode = "ask"        // propose a pick and confirm (default)
	ApplyModeAlwaysAsk ApplyMode = "always-ask" // always show candidates and choose
)

// Valid reports whether m is a known apply mode.
func (m ApplyMode) Valid() bool {
	switch m {
	case ApplyModeAuto, ApplyModeAsk, ApplyModeAlwaysAsk:
		return true
	}
	return false
}

// ResolvedApplyMode returns the configured mode or the ApplyModeAsk default.
func (c *Config) ResolvedApplyMode() ApplyMode {
	if c.ApplyMode.Valid() {
		return c.ApplyMode
	}
	return ApplyModeAsk
}

// ApplyModel controls which model tier /vector:apply uses for the implementation
// step (docs/apply-design.md §3). "opus" (or empty) keeps the current Opus-inline
// behavior; "sonnet" always delegates to a Sonnet subagent; "conditional" evaluates
// five mechanical signals and routes to Sonnet only when the change is mechanical.
// The field is opt-in: vector init/update never writes it, so existing configs keep
// the current behavior unchanged.
type ApplyModel string

const (
	ApplyModelOpus        ApplyModel = "opus"        // default; implements inline (Opus)
	ApplyModelSonnet      ApplyModel = "sonnet"      // always delegates to Sonnet subagent
	ApplyModelConditional ApplyModel = "conditional" // evaluates mechanical signals first
)

// Valid reports whether m is a known apply model value.
func (m ApplyModel) Valid() bool {
	switch m {
	case ApplyModelOpus, ApplyModelSonnet, ApplyModelConditional:
		return true
	}
	return false
}

// ResolvedApplyModel returns the configured model tier or ApplyModelOpus when the
// field is empty or invalid — the safe default preserves current Opus behavior.
func (c *Config) ResolvedApplyModel() ApplyModel {
	if c.ApplyModel.Valid() {
		return c.ApplyModel
	}
	return ApplyModelOpus
}

// ShipMode controls whether /vector:ship opens the pull request after confirmation
// ("ask") or without prompting ("auto"). Empty resolves to ShipModeAsk.
type ShipMode string

const (
	ShipModeAsk  ShipMode = "ask"  // confirm before opening the PR (default)
	ShipModeAuto ShipMode = "auto" // open the PR without prompting
)

// Valid reports whether m is a known ship mode.
func (m ShipMode) Valid() bool {
	switch m {
	case ShipModeAsk, ShipModeAuto:
		return true
	}
	return false
}

// DefaultShipExcludeGlobs is the static set of paths /vector:ship never commits
// when shipping a spec — OpenSpec change artifacts are shipped separately from the
// implementation. The spec's own authored doc is excluded dynamically (resolved via
// SpecDocPath), not listed here. Extra globs come from ShipConfig.ExcludeGlobs.
var DefaultShipExcludeGlobs = []string{"openspec/"}

// ShipConfig holds the /vector:ship orchestration knobs. Every field is optional
// (pass-through, merged incrementally by `vector config set-ship`); the resolvers on
// Config apply the defaults, so a nil ShipConfig behaves as all-defaults. Draft is a
// pointer so an explicit false is distinguishable from "unset" (which defaults true).
type ShipConfig struct {
	// BaseBranch overrides the branch a PR targets and is rebased onto. Empty falls
	// back to the worktree base branch (see ResolvedShipBaseBranch).
	BaseBranch string `json:"baseBranch,omitempty"`
	// Mode is ask|auto (see ShipMode). Empty defaults to ShipModeAsk.
	Mode ShipMode `json:"mode,omitempty"`
	// Draft, when set, forces the PR's draft state; nil defaults to true (open drafts).
	Draft *bool `json:"draft,omitempty"`
	// ExcludeGlobs are extra commit-exclude globs added on top of DefaultShipExcludeGlobs.
	ExcludeGlobs []string `json:"excludeGlobs,omitempty"`
	// AuthBootstrap is an opt-in spec (a path to source or an SSH alias) that /vector:ship
	// uses to resolve git/gh auth in a non-interactive shell. Empty = never bootstrap.
	AuthBootstrap string `json:"authBootstrap,omitempty"`
}

// ResolvedShipMode returns the configured ship mode or the ShipModeAsk default.
// Nil-safe: a nil Ship resolves to ask.
func (c *Config) ResolvedShipMode() ShipMode {
	if c.Ship != nil && c.Ship.Mode.Valid() {
		return c.Ship.Mode
	}
	return ShipModeAsk
}

// ResolvedShipDraft reports whether /vector:ship opens the PR as a draft. Nil-safe:
// an unset Ship or Ship.Draft defaults to true (drafts by default).
func (c *Config) ResolvedShipDraft() bool {
	if c.Ship != nil && c.Ship.Draft != nil {
		return *c.Ship.Draft
	}
	return true
}

// ResolvedShipExcludeGlobs returns the effective commit-exclude globs: the static
// DefaultShipExcludeGlobs plus any configured extras, de-duplicated and trimmed. A
// nil Ship yields exactly the static defaults (["openspec/"]). The result is a fresh
// slice — the caller may not mutate DefaultShipExcludeGlobs through it.
func (c *Config) ResolvedShipExcludeGlobs() []string {
	out := make([]string, 0, len(DefaultShipExcludeGlobs)+2)
	seen := map[string]bool{}
	add := func(glob string) {
		if glob = strings.TrimSpace(glob); glob != "" && !seen[glob] {
			seen[glob] = true
			out = append(out, glob)
		}
	}
	for _, glob := range DefaultShipExcludeGlobs {
		add(glob)
	}
	if c.Ship != nil {
		for _, glob := range c.Ship.ExcludeGlobs {
			add(glob)
		}
	}
	return out
}

// ResolvedShipBaseBranch returns the branch a PR targets: the configured ship base
// branch, else the provided fallback (typically the worktree base branch), else the
// "main" default. Nil-safe.
func (c *Config) ResolvedShipBaseBranch(fallback string) string {
	if c.Ship != nil {
		if b := strings.TrimSpace(c.Ship.BaseBranch); b != "" {
			return b
		}
	}
	if b := strings.TrimSpace(fallback); b != "" {
		return b
	}
	return DefaultBaseBranch
}

// ResolvedShipAuthBootstrap returns the configured opt-in auth bootstrap spec, or ""
// when none is set (ship never bootstraps auth implicitly). Nil-safe.
func (c *Config) ResolvedShipAuthBootstrap() string {
	if c.Ship != nil {
		return strings.TrimSpace(c.Ship.AuthBootstrap)
	}
	return ""
}

// ResolvedLanguage returns the configured prose language trimmed of surrounding
// whitespace, or "" when none is set (agents fall back to the conversation language).
func (c *Config) ResolvedLanguage() string {
	return strings.TrimSpace(c.Language)
}

// ResolvedBuildCmds returns the configured build, lint, and test commands.
// All three may be empty if DetectBuildCmds found nothing or was not run
// during init/update. Empty means "not configured" — callers may fall back to
// DetectBuildCmds or ask the user.
func (c *Config) ResolvedBuildCmds() (build, lint, test string) {
	return c.BuildCmd, c.LintCmd, c.TestCmd
}

// makefileTargets records which standard targets a Makefile declares.
type makefileTargets struct{ build, lint, test bool }

// nodeScripts holds the npm/yarn/pnpm run-script commands for the three
// standard lifecycle operations (empty string = script absent in package.json).
type nodeScripts struct{ build, lint, test string }

// DetectBuildCmds infers build, lint, and test commands from the repo's manifest
// files concurrently. Returns empty strings when no command can be inferred with
// confidence. Priority per field: an explicit Makefile target wins; absent that,
// go.mod (Go), package.json scripts (Node), or pyproject.toml/setup.py (Python)
// contribute. A repo may mix sources (e.g. Makefile without a lint target +
// go.mod contributes golangci-lint run). Goroutines that fail return empty values
// without propagating errors to the caller.
func DetectBuildCmds(repoRoot string) (build, lint, test string) {
	var (
		mk   makefileTargets
		isGo bool
		node nodeScripts
		isPy bool
	)

	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		mk = parseMakefile(repoRoot)
	}()
	go func() {
		defer wg.Done()
		_, err := os.Stat(filepath.Join(repoRoot, "go.mod"))
		isGo = err == nil
	}()
	go func() {
		defer wg.Done()
		node = parsePackageJSON(repoRoot)
	}()
	go func() {
		defer wg.Done()
		_, e1 := os.Stat(filepath.Join(repoRoot, "pyproject.toml"))
		_, e2 := os.Stat(filepath.Join(repoRoot, "setup.py"))
		isPy = e1 == nil || e2 == nil
	}()
	wg.Wait()

	// Build command: Makefile target wins; then language-specific manifest.
	switch {
	case mk.build:
		build = "make build"
	case isGo:
		build = "go build ./..."
	case node.build != "":
		build = node.build
	case isPy:
		build = "python -m build"
	}

	// Lint command.
	switch {
	case mk.lint:
		lint = "make lint"
	case isGo:
		lint = "golangci-lint run"
	case node.lint != "":
		lint = node.lint
	}

	// Test command.
	switch {
	case mk.test:
		test = "make test"
	case isGo:
		test = "go test ./..."
	case node.test != "":
		test = node.test
	case isPy:
		test = "pytest"
	}

	return
}

// parseMakefile reads the root Makefile (if present) and reports which of the
// three standard targets (build, lint, test) it declares. A target line starts
// with its name followed by a colon with no leading whitespace. Errors silently
// return all-false so callers keep building from other sources.
func parseMakefile(repoRoot string) (mt makefileTargets) {
	b, err := os.ReadFile(filepath.Join(repoRoot, "Makefile"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
			continue // recipe line, not a target declaration
		}
		name, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(name) {
		case "build":
			mt.build = true
		case "lint":
			mt.lint = true
		case "test":
			mt.test = true
		}
	}
	return
}

// detectNodeRunner identifies the package manager in use by looking for
// lock-file markers. Defaults to "npm" when none is found.
func detectNodeRunner(repoRoot string) string {
	for _, candidate := range []struct{ file, runner string }{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	} {
		if _, err := os.Stat(filepath.Join(repoRoot, candidate.file)); err == nil {
			return candidate.runner
		}
	}
	return "npm"
}

// parsePackageJSON reads package.json (if present) and returns run-script
// commands for the three standard lifecycle scripts. Scripts not present in
// package.json produce an empty string. Errors silently return empty nodeScripts.
func parsePackageJSON(repoRoot string) (ns nodeScripts) {
	b, err := os.ReadFile(filepath.Join(repoRoot, "package.json"))
	if err != nil {
		return
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return
	}
	if len(pkg.Scripts) == 0 {
		return
	}
	runner := detectNodeRunner(repoRoot)
	if _, ok := pkg.Scripts["build"]; ok {
		ns.build = runner + " run build"
	}
	if _, ok := pkg.Scripts["lint"]; ok {
		ns.lint = runner + " run lint"
	}
	if _, ok := pkg.Scripts["test"]; ok {
		ns.test = runner + " run test"
	}
	return
}

// ResolvedDefaultTicketProvider returns the configured default ticket provider,
// or "" when none is set (which disables the bare-key detection fallback).
func (c *Config) ResolvedDefaultTicketProvider() state.TicketProvider {
	if c.DefaultTicketProvider.Valid() {
		return c.DefaultTicketProvider
	}
	return ""
}

// NormalizedTicketKeyPrefixes returns the configured key prefixes trimmed and
// upper-cased, dropping empties — the form used for case-insensitive matching.
func (c *Config) NormalizedTicketKeyPrefixes() []string {
	out := make([]string, 0, len(c.TicketKeyPrefixes))
	for _, p := range c.TicketKeyPrefixes {
		if p = strings.ToUpper(strings.TrimSpace(p)); p != "" {
			out = append(out, p)
		}
	}
	return out
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

// Defaults for the per-spec worktree the /vector:raw and /vector:bug orchestration
// creates on bare+worktree layouts.
const (
	DefaultBaseBranch   = "main"
	DefaultBranchPrefix = "feat/"
)

// HasBranchPlaceholder reports whether the repo declares a bare+worktree layout:
// the [branch] placeholder is present in the resolved spec-path or changes-path
// template. This is the signal that gates the worktree-resolve/create step in
// /vector:raw and /vector:bug; false means that step is inert (non-worktree repos).
func (c *Config) HasBranchPlaceholder() bool {
	return strings.Contains(c.SpecPath, branchPlaceholder) ||
		strings.Contains(c.changesTemplate(), branchPlaceholder)
}

// WorktreeRoot returns the literal template prefix that precedes the [branch]
// placeholder — the directory under which per-spec worktrees live (e.g. "code"
// for a "code/[branch]/..." spec-path), with any trailing slash trimmed. Empty
// when the repo declares no worktree layout. Prefers SpecPath, falling back to
// the changes template (either may carry the placeholder).
func (c *Config) WorktreeRoot() string {
	for _, tmpl := range []string{c.SpecPath, c.changesTemplate()} {
		if i := strings.Index(tmpl, branchPlaceholder); i >= 0 {
			return strings.TrimRight(tmpl[:i], "/")
		}
	}
	return ""
}

// BaseBranchOrDefault returns the configured base branch (worktree fork point) or
// the "main" default.
func (c *Config) BaseBranchOrDefault() string {
	if b := strings.TrimSpace(c.BaseBranch); b != "" {
		return b
	}
	return DefaultBaseBranch
}

// BranchPrefixOrDefault returns the configured per-spec branch prefix or the
// "feat/" default.
func (c *Config) BranchPrefixOrDefault() string {
	if p := strings.TrimSpace(c.BranchPrefix); p != "" {
		return p
	}
	return DefaultBranchPrefix
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

// FindAncestorConfig walks up from startDir looking for the nearest directory
// that holds a valid .vector/config.json — the canonical store to anchor to. The
// walk starts at startDir itself and stops at the filesystem root.
//
// A .vector/ directory WITHOUT a loadable config.json is a stray: it is recorded
// in strayDirs (so the caller can warn) but never adopted, and the walk keeps
// going up — an intermediate stray must not hide the real ancestor.
//
// Nearest wins: in a bare+worktree layout where a worktree carries its own
// .vector/config.json under a workspace that also has one, a command run inside
// the worktree anchors to the worktree, never to the workspace above it.
//
// It is read-only: no writes, no side effects, and no opinion on git/worktree
// boundaries. When nothing is found it returns ("", strayDirs, false) and the
// caller keeps its existing resolution.
func FindAncestorConfig(startDir string) (root string, strayDirs []string, found bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", nil, false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".vector")); err == nil {
			if _, err := Load(dir); err == nil {
				return dir, strayDirs, true
			}
			strayDirs = append(strayDirs, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", strayDirs, false
		}
		dir = parent
	}
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
	if c.DefaultTicketProvider != "" && !c.DefaultTicketProvider.Valid() {
		return nil, fmt.Errorf("invalid defaultTicketProvider %q: allowed jira,linear,github,other", c.DefaultTicketProvider)
	}
	if c.ApplyModel != "" && !c.ApplyModel.Valid() {
		return nil, fmt.Errorf("invalid applyModel %q: allowed opus,sonnet,conditional", c.ApplyModel)
	}
	if c.Ship != nil && c.Ship.Mode != "" && !c.Ship.Mode.Valid() {
		return nil, fmt.Errorf("invalid ship.mode %q: allowed ask,auto", c.Ship.Mode)
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

// worktreeMaxDepth bounds how many directory levels below the worktree root
// WorktreeTicketKeys descends. Bare+worktree layouts nest a branch one or two
// levels under the root — grouping folders (feat/chore/fix/docs) may sit in
// between (e.g. code/feat/MH-1592-payments) — so 3 covers those without walking
// into each worktree's own file tree (a perf and noise guard, not a magic number).
const worktreeMaxDepth = 3

// worktreeKeyRe matches a branch folder basename of the form <KEY>-<slug>,
// capturing the universal ticket key (<project>-<number>) and the slug. A bare
// <KEY> folder (no slug) does not match — it is intentionally not indexed.
var worktreeKeyRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]*-\d+)-(.+)$`)

// worktreeKeyDenylist mirrors ticket detection's denylist: prefixes that are
// documentation conventions (ADR, RFC), never tickets. Keys are uppercased before
// the lookup (see worktreeBranchKey).
var worktreeKeyDenylist = map[string]bool{"ADR": true, "RFC": true}

// WorktreeTicketKeys indexes ticket keys carried by worktree folder names, for
// detectTicket's last-resort fallback during sync. In a bare+worktree layout each
// branch lives in a folder named <KEY>-<slug> (e.g. code/feat/MH-1592-payments) and
// the change's own artifacts often never mention the key, so the folder name is the
// highest-recall deterministic signal available. The worktree root is the literal
// prefix of the changes template before [branch] (code/[branch]/openspec/changes →
// code); without a [branch] placeholder the layout is not worktree-based and the map
// is empty (the feature is inert — no regression on the non-worktree repos). The scan
// is read-only and depth-bounded (worktreeMaxDepth), tolerating grouping folders
// (feat/chore/fix/docs) and single-level branches (develop). Keys are normalized to
// uppercase; ADR/RFC are dropped. Identity is the slug (== change name after stripping
// the <KEY>- prefix); a slug claimed by two distinct keys is ambiguous and omitted. A
// permission/IO error on a subtree skips only that subtree (the index keeps building);
// an error reading the worktree root itself is propagated (a missing root is not an
// error — it just yields an empty map).
func (c *Config) WorktreeTicketKeys(repoRoot string) (map[string]string, error) {
	tmpl := c.changesTemplate()
	i := strings.Index(tmpl, branchPlaceholder)
	if i < 0 {
		return map[string]string{}, nil // not a worktree layout; feature inert
	}
	rootDir := repoRoot
	if prefix := strings.Trim(tmpl[:i], "/"); prefix != "" {
		rootDir = filepath.Join(repoRoot, filepath.FromSlash(prefix))
	}

	rootEntries, err := os.ReadDir(rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil // no worktree root yet → nothing to index
		}
		return nil, fmt.Errorf("read worktree root %q: %w", rootDir, err)
	}

	index := map[string]string{}
	conflicted := map[string]bool{}
	register := func(name string) {
		key, slug, ok := worktreeBranchKey(name)
		if !ok {
			return
		}
		switch {
		case conflicted[slug]:
			// already ambiguous; stays omitted
		case index[slug] == "":
			index[slug] = key
		case index[slug] != key:
			delete(index, slug) // two distinct keys claim this slug → ambiguous
			conflicted[slug] = true
		}
	}

	var descend func(dir string, depth int)
	descend = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return // permission/IO error on a subtree → skip it; the index continues
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			register(e.Name())
			if depth < worktreeMaxDepth {
				descend(filepath.Join(dir, e.Name()), depth+1)
			}
		}
	}
	for _, e := range rootEntries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		register(e.Name())
		descend(filepath.Join(rootDir, e.Name()), 2)
	}
	return index, nil
}

// worktreeBranchKey parses a branch folder basename of the form <KEY>-<slug>,
// returning the uppercased ticket key and the slug. ok is false for a bare key
// (no slug), a non-matching name (grouping folders like feat, single-level branches
// like develop) or a denylisted key (ADR/RFC).
func worktreeBranchKey(name string) (key, slug string, ok bool) {
	m := worktreeKeyRe.FindStringSubmatch(name)
	if m == nil {
		return "", "", false
	}
	key = strings.ToUpper(m[1])
	if i := strings.IndexByte(key, '-'); i > 0 && worktreeKeyDenylist[key[:i]] {
		return "", "", false
	}
	return key, m[2], true
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
