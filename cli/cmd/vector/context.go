package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/intel"
)

// ContextOutput is the JSON output of `vector context --json`. It bundles
// the repo's setup context — spec example path, prose language, build/lint/test
// commands, apply mode, and ticket detection status — so /vector:* commands can
// consume all of it from a single binary invocation without per-command
// re-derivation. The optional intel field carries a compact, fingerprint-fresh
// summary of the repo's stack and workspaces (class-C cache); it is additive and
// omitted for callers that don't consume it (backward-compat).
type ContextOutput struct {
	ExamplePath    string        `json:"examplePath"`
	Language       string        `json:"language"`
	BuildCmd       string        `json:"buildCmd"`
	LintCmd        string        `json:"lintCmd"`
	TestCmd        string        `json:"testCmd"`
	ApplyMode      string        `json:"applyMode"`
	TicketDetected bool          `json:"ticketDetected"`
	Intel          *IntelSummary `json:"intel,omitempty"`
}

// IntelSummary is the compact repo-intel projection embedded in ContextOutput:
// the stack summary plus the workspace list (path + kind), without entry points.
type IntelSummary struct {
	Stack      StackSummary       `json:"stack"`
	Workspaces []WorkspaceSummary `json:"workspaces"`
}

// StackSummary is the compact stack projection ({packageManager, runtime,
// frameworks}) drawn from repo-intel.json.
type StackSummary struct {
	PackageManager string        `json:"packageManager,omitempty"`
	Runtime        intel.Runtime `json:"runtime"`
	Frameworks     []string      `json:"frameworks"`
}

// WorkspaceSummary is one workspace's path + kind (no entry points) for the
// embedded intel summary.
type WorkspaceSummary struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// ContextSlice is the scoped projection returned by `vector context --for
// <command>`: only the domains a command consumes, per its validation tier.
type ContextSlice struct {
	Command          string        `json:"command"`
	Tier             string        `json:"tier"`
	ValidatedDomains []string      `json:"validatedDomains"`
	ExamplePath      string        `json:"examplePath,omitempty"`
	Language         string        `json:"language,omitempty"`
	BuildCmd         string        `json:"buildCmd,omitempty"`
	LintCmd          string        `json:"lintCmd,omitempty"`
	TestCmd          string        `json:"testCmd,omitempty"`
	Stack            *StackSummary `json:"stack,omitempty"`
}

// validationTier classifies how much repo validation a command demands
// (docs/knowledge-architecture.md §6).
type validationTier string

const (
	tierTrust validationTier = "trust"         // no repo facts → no validation
	tierLazy  validationTier = "lazy-validate" // generates prose/spec → validate stack+workspace
	tierFull  validationTier = "full-validate" // mutates/runs code → validate build+stack+deps
)

// commandTiers is the static command→tier map (canonical list of
// docs/knowledge-architecture.md §6). Unknown commands are rejected by --for.
var commandTiers = map[string]validationTier{
	"status": tierTrust, "link": tierTrust, "close": tierTrust,
	"archive": tierTrust, "standup": tierTrust, "propose": tierTrust, "sync": tierTrust,
	"raw": tierLazy, "bug": tierLazy,
	"apply": tierFull, "comment": tierFull,
}

// tierDomains maps a tier to the domains it validates.
var tierDomains = map[validationTier][]intel.Domain{
	tierTrust: {},
	tierLazy:  {intel.DomainStack, intel.DomainWorkspace},
	tierFull:  {intel.DomainBuild, intel.DomainStack, intel.DomainDeps},
}

// runContext implements `vector context [--json] [--repo-root path] [--dry-run]`.
// It reads the repo's config, resolves examplePath via a glob over specPath, fills
// build/lint/test from config (with a runtime DetectBuildCmds fallback when
// uncached), and emits a ContextOutput.
//
// Exit 0 on success; exit 1 with an actionable message on stderr when
// .vector/config.json is absent or malformed. Never calls config.Write.
func runContext(args []string) error {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", true, "emit JSON output (default true)")
	refresh := fs.Bool("refresh", false, "force full regeneration of the repo-intel cache")
	forCmd := fs.String("for", "", "project only the context slice a given /vector command consumes")
	_ = fs.Bool("dry-run", false, "no-op; present for interface consistency")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		// Surface a clear, actionable error regardless of the underlying cause.
		return fmt.Errorf("no .vector/config.json in %s — run vector init first", root)
	}

	// Scoped projection: --for <command> returns only that command's slice.
	if *forCmd != "" {
		return runContextFor(cfg, root, *forCmd, *refresh, *jsonOut)
	}

	// Resolve examplePath and (optionally) build commands concurrently: the
	// glob and DetectBuildCmds are independent I/O operations.
	cfgBuild, cfgLint, cfgTest := cfg.ResolvedBuildCmds()
	needDetect := cfgBuild == "" || cfgLint == "" || cfgTest == ""

	var (
		examplePath                               string
		detectedBuild, detectedLint, detectedTest string
	)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		examplePath = resolveExamplePath(cfg, root)
	}()
	if needDetect {
		wg.Add(1)
		go func() {
			defer wg.Done()
			detectedBuild, detectedLint, detectedTest = config.DetectBuildCmds(root)
		}()
	}
	wg.Wait()

	// Fill each field: prefer the cached config value; fall back to runtime detection.
	buildCmd := cfgBuild
	if buildCmd == "" {
		buildCmd = detectedBuild
	}
	lintCmd := cfgLint
	if lintCmd == "" {
		lintCmd = detectedLint
	}
	testCmd := cfgTest
	if testCmd == "" {
		testCmd = detectedTest
	}

	out := ContextOutput{
		ExamplePath:    examplePath,
		Language:       cfg.ResolvedLanguage(),
		BuildCmd:       buildCmd,
		LintCmd:        lintCmd,
		TestCmd:        testCmd,
		ApplyMode:      string(cfg.ResolvedApplyMode()),
		TicketDetected: cfg.ResolvedDefaultTicketProvider() != "",
	}

	// Validate (and lazily regenerate) the full intel cache, attaching a compact
	// summary. Best-effort: a cache failure warns and omits intel rather than
	// breaking the backward-compatible output.
	if cache, err := intel.Resolve(root, version, intel.AllDomains, *refresh); err != nil {
		fmt.Fprintf(os.Stderr, "warning: repo-intel cache: %v\n", err)
	} else {
		out.Intel = &IntelSummary{Stack: stackSummary(cache.RepoIntel), Workspaces: workspaceSummaries(cache.Structure)}
	}

	if *jsonOut {
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	// Human-readable output: one line per field with fixed-width label.
	fmt.Printf("%-16s %s\n", "examplePath", strOr(out.ExamplePath, "(none)"))
	fmt.Printf("%-16s %s\n", "language", strOr(out.Language, "(default)"))
	fmt.Printf("%-16s %s\n", "buildCmd", strOr(out.BuildCmd, "(none)"))
	fmt.Printf("%-16s %s\n", "lintCmd", strOr(out.LintCmd, "(none)"))
	fmt.Printf("%-16s %s\n", "testCmd", strOr(out.TestCmd, "(none)"))
	fmt.Printf("%-16s %s\n", "applyMode", out.ApplyMode)
	fmt.Printf("%-16s %v\n", "ticketDetected", out.TicketDetected)
	return nil
}

// resolveExamplePath returns the repo-relative path of the first spec doc found
// under the configured specPath, sorted lexicographically. Returns "" when none
// exist; emits a warning to stderr on glob/scan error without aborting.
func resolveExamplePath(cfg *config.Config, repoRoot string) string {
	switch cfg.SpecStore {
	case config.StoreVector:
		// .vector/specs/*/spec.md — a single glob level under the local store.
		matches, err := filepath.Glob(filepath.Join(repoRoot, ".vector", "specs", "*", "spec.md"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: glob spec examples: %v\n", err)
			return ""
		}
		if len(matches) == 0 {
			return ""
		}
		// filepath.Glob returns sorted names; matches[0] is lexicographically first.
		rel, err := filepath.Rel(repoRoot, matches[0])
		if err != nil {
			return filepath.ToSlash(matches[0])
		}
		return filepath.ToSlash(rel)

	case config.StoreConvention:
		// Reuse FindSpecDocs which already sorts by slug.
		docs, err := cfg.FindSpecDocs(repoRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: scan spec docs: %v\n", err)
			return ""
		}
		if len(docs) == 0 {
			return ""
		}
		return docs[0].Rel
	}
	return ""
}

// strOr returns a when non-empty, else b.
func strOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// stackSummary projects the compact stack summary from a RepoIntel artifact,
// normalizing a nil frameworks slice to an empty one for stable JSON.
func stackSummary(ri intel.RepoIntel) StackSummary {
	frameworks := ri.Frameworks
	if frameworks == nil {
		frameworks = []string{}
	}
	return StackSummary{
		PackageManager: ri.PackageManager,
		Runtime:        ri.Runtime,
		Frameworks:     frameworks,
	}
}

// workspaceSummaries projects path+kind summaries from a StructureIndex.
func workspaceSummaries(si intel.StructureIndex) []WorkspaceSummary {
	out := make([]WorkspaceSummary, 0, len(si.Workspaces))
	for _, ws := range si.Workspaces {
		out = append(out, WorkspaceSummary{Path: ws.Path, Kind: ws.Kind})
	}
	return out
}

// runContextFor handles `vector context --for <command>`: it resolves the
// command's tier, validates (and lazily regenerates) only the domains that tier
// consumes, and emits the scoped ContextSlice. An unknown command is an
// actionable error (exit 1).
func runContextFor(cfg *config.Config, root, command string, refresh, jsonOut bool) error {
	tier, ok := commandTiers[command]
	if !ok {
		return fmt.Errorf("unknown command %q for --for: known commands are %s", command, knownCommands())
	}
	domains := tierDomains[tier]

	slice := ContextSlice{
		Command:          command,
		Tier:             string(tier),
		ValidatedDomains: domainStrings(domains),
	}

	// Validate the consumed domains; a cache failure warns but still returns the
	// slice (without the stack payload) rather than failing the command.
	var cache *intel.Cache
	if len(domains) > 0 {
		c, err := intel.Resolve(root, version, domains, refresh)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: repo-intel cache: %v\n", err)
		}
		cache = c
	}

	switch tier {
	case tierLazy:
		slice.ExamplePath = resolveExamplePath(cfg, root)
		slice.Language = cfg.ResolvedLanguage()
		if cache != nil {
			s := stackSummary(cache.RepoIntel)
			slice.Stack = &s
		}
	case tierFull:
		build, lint, test := resolvedBuildCmds(cfg, root)
		slice.BuildCmd, slice.LintCmd, slice.TestCmd = build, lint, test
		if cache != nil {
			s := stackSummary(cache.RepoIntel)
			slice.Stack = &s
		}
	}

	if jsonOut {
		b, err := json.MarshalIndent(slice, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("%-16s %s\n", "command", slice.Command)
	fmt.Printf("%-16s %s\n", "tier", slice.Tier)
	fmt.Printf("%-16s %s\n", "validatedDomains", strings.Join(slice.ValidatedDomains, ", "))
	return nil
}

// resolvedBuildCmds returns the repo's build/lint/test commands, preferring the
// cached config values and falling back to runtime detection (same precedence as
// the base output).
func resolvedBuildCmds(cfg *config.Config, root string) (build, lint, test string) {
	cfgBuild, cfgLint, cfgTest := cfg.ResolvedBuildCmds()
	build, lint, test = cfgBuild, cfgLint, cfgTest
	if build == "" || lint == "" || test == "" {
		detectedBuild, detectedLint, detectedTest := config.DetectBuildCmds(root)
		build = strOr(build, detectedBuild)
		lint = strOr(lint, detectedLint)
		test = strOr(test, detectedTest)
	}
	return build, lint, test
}

// domainStrings converts a domain slice to its string form for JSON output.
func domainStrings(domains []intel.Domain) []string {
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		out = append(out, string(d))
	}
	return out
}

// knownCommands returns the sorted set of commands accepted by --for, for error
// messages.
func knownCommands() string {
	names := make([]string, 0, len(commandTiers))
	for name := range commandTiers {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
