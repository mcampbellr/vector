package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mariocampbell/vector/internal/config"
)

// ContextOutput is the JSON output of `vector context --json`. It bundles
// the repo's setup context — spec example path, prose language, build/lint/test
// commands, apply mode, and ticket detection status — so /vector:* commands can
// consume all of it from a single binary invocation without per-command
// re-derivation.
type ContextOutput struct {
	ExamplePath    string `json:"examplePath"`
	Language       string `json:"language"`
	BuildCmd       string `json:"buildCmd"`
	LintCmd        string `json:"lintCmd"`
	TestCmd        string `json:"testCmd"`
	ApplyMode      string `json:"applyMode"`
	TicketDetected bool   `json:"ticketDetected"`
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
