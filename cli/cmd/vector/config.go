package main

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/mariocampbell/vector/internal/config"
)

// runConfig dispatches the `vector config <subcommand>` family. The binary is the
// sole writer of .vector/config.json (CLI-owns-writes); these subcommands are the
// only supported way to mutate it after `vector init`.
func runConfig(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: vector config <set-ship> ...")
	}
	switch args[0] {
	case "set-ship":
		return runConfigSetShip(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

// runConfigSetShip writes the Ship block of .vector/config.json incrementally: each
// flag is optional and touches only its own field ("empty = don't change"), so a
// caller can set the base branch today and the mode tomorrow without clobbering the
// other. It requires an existing config (`vector init` first), validates --mode
// before applying, and writes only when a field actually changed (idempotent).
func runConfigSetShip(args []string) error {
	fs := flag.NewFlagSet("config set-ship", flag.ContinueOnError)
	baseBranch := fs.String("base-branch", "", "branch the PR targets and is rebased onto (empty = leave as-is)")
	mode := fs.String("mode", "", "PR-open mode: ask|auto (empty = leave as-is)")
	draft := fs.String("draft", "", "open the PR as a draft: true|false (empty = leave as-is)")
	exclude := fs.String("exclude", "", "comma-separated extra commit-exclude globs (empty = leave as-is)")
	authBootstrap := fs.String("auth-bootstrap", "", "opt-in auth bootstrap spec: a path to source or an SSH alias (empty = leave as-is)")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// "empty = don't change" per field. A flag counts as provided only when it was
	// passed on the command line (Visit reports set flags), so an explicit --draft=false
	// or --exclude= (clear) is honored while an omitted flag leaves the field untouched.
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if !set["base-branch"] && !set["mode"] && !set["draft"] && !set["exclude"] && !set["auth-bootstrap"] {
		return errors.New("nothing to set: pass at least one of --base-branch, --mode, --draft, --exclude, --auth-bootstrap")
	}

	// Validate --mode before touching anything, so a bad value never half-writes.
	var wantMode config.ShipMode
	if set["mode"] {
		wantMode = config.ShipMode(strings.TrimSpace(*mode))
		if !wantMode.Valid() {
			return fmt.Errorf("invalid --mode %q: allowed ask,auto", *mode)
		}
	}
	// Parse --draft only when provided and non-empty.
	var wantDraft *bool
	if set["draft"] && strings.TrimSpace(*draft) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(*draft))
		if err != nil {
			return fmt.Errorf("invalid --draft %q: allowed true,false", *draft)
		}
		wantDraft = &parsed
	}

	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return fmt.Errorf("read .vector/config.json (run `vector init` first): %w", err)
	}
	if cfg.Ship == nil {
		cfg.Ship = &config.ShipConfig{}
	}

	// Merge incrementally; track whether any field actually changed.
	changed := false
	if set["base-branch"] {
		if v := strings.TrimSpace(*baseBranch); cfg.Ship.BaseBranch != v {
			cfg.Ship.BaseBranch = v
			changed = true
		}
	}
	if set["mode"] {
		if cfg.Ship.Mode != wantMode {
			cfg.Ship.Mode = wantMode
			changed = true
		}
	}
	if set["draft"] {
		if !draftEqual(cfg.Ship.Draft, wantDraft) {
			cfg.Ship.Draft = wantDraft
			changed = true
		}
	}
	if set["exclude"] {
		globs := splitCSV(*exclude)
		if !stringsEqual(cfg.Ship.ExcludeGlobs, globs) {
			cfg.Ship.ExcludeGlobs = globs
			changed = true
		}
	}
	if set["auth-bootstrap"] {
		if v := strings.TrimSpace(*authBootstrap); cfg.Ship.AuthBootstrap != v {
			cfg.Ship.AuthBootstrap = v
			changed = true
		}
	}

	if changed {
		if err := config.Write(root, cfg); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
	}

	if *jsonOut {
		return printJSONValue(map[string]any{
			"baseBranch":    cfg.Ship.BaseBranch,
			"mode":          string(cfg.ResolvedShipMode()),
			"draft":         cfg.ResolvedShipDraft(),
			"excludeGlobs":  cfg.ResolvedShipExcludeGlobs(),
			"authBootstrap": cfg.Ship.AuthBootstrap,
			"changed":       changed,
		})
	}
	if !changed {
		fmt.Println("ship config unchanged")
		return nil
	}
	fmt.Printf("ship config updated (mode: %s, draft: %t)\n", cfg.ResolvedShipMode(), cfg.ResolvedShipDraft())
	return nil
}

// draftEqual reports whether two *bool draft values are equivalent (both nil, or
// both non-nil with the same value).
func draftEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// stringsEqual reports whether two string slices hold the same elements in order.
func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
