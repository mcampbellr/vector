package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/spf13/cobra"
)

// newConfigCmd is the `config` parent command. It carries the config.* subverbs and
// keeps an explicit RunE so `vector config` with no subverb returns the legacy usage
// error (exit 1), mirroring newSpecCmd. The binary is the sole writer of
// .vector/config.json (CLI-owns-writes); these subcommands are the only supported way
// to mutate it after `vector init`.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "read and mutate .vector/config.json (CLI-owns-writes)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("usage: vector config <set-ship> ...")
		},
	}
	cmd.AddCommand(newConfigSetShipCmd())
	return cmd
}

// newConfigSetShipCmd writes the Ship block of .vector/config.json incrementally:
// each flag is optional and touches only its own field ("empty = don't change"), so a
// caller can set the base branch today and the mode tomorrow without clobbering the
// other. It requires an existing config (`vector init` first), validates --mode before
// applying, and writes only when a field actually changed (idempotent).
func newConfigSetShipCmd() *cobra.Command {
	var (
		baseBranch    string
		mode          string
		draft         string
		exclude       string
		authBootstrap string
		repoRoot      string
		jsonOut       bool
	)
	cmd := &cobra.Command{
		Use:   "set-ship",
		Short: "set the ship block of .vector/config.json (base branch, mode, draft, excludes, auth bootstrap)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// "empty = don't change" per field. A flag counts as provided only when it was
			// passed on the command line (cobra's Changed reports set flags), so an explicit
			// --draft=false or --exclude= (clear) is honored while an omitted flag leaves the
			// field untouched.
			flags := cmd.Flags()
			set := func(name string) bool { return flags.Changed(name) }

			if !set("base-branch") && !set("mode") && !set("draft") && !set("exclude") && !set("auth-bootstrap") {
				return errors.New("nothing to set: pass at least one of --base-branch, --mode, --draft, --exclude, --auth-bootstrap")
			}

			// Validate --mode before touching anything, so a bad value never half-writes.
			var wantMode config.ShipMode
			if set("mode") {
				wantMode = config.ShipMode(strings.TrimSpace(mode))
				if !wantMode.Valid() {
					return fmt.Errorf("invalid --mode %q: allowed ask,auto", mode)
				}
			}
			// Parse --draft only when provided and non-empty.
			var wantDraft *bool
			if set("draft") && strings.TrimSpace(draft) != "" {
				parsed, err := strconv.ParseBool(strings.TrimSpace(draft))
				if err != nil {
					return fmt.Errorf("invalid --draft %q: allowed true,false", draft)
				}
				wantDraft = &parsed
			}

			root, err := resolveRepoRoot(repoRoot)
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
			if set("base-branch") {
				if v := strings.TrimSpace(baseBranch); cfg.Ship.BaseBranch != v {
					cfg.Ship.BaseBranch = v
					changed = true
				}
			}
			if set("mode") {
				if cfg.Ship.Mode != wantMode {
					cfg.Ship.Mode = wantMode
					changed = true
				}
			}
			if set("draft") {
				if !draftEqual(cfg.Ship.Draft, wantDraft) {
					cfg.Ship.Draft = wantDraft
					changed = true
				}
			}
			if set("exclude") {
				globs := splitCSV(exclude)
				if !stringsEqual(cfg.Ship.ExcludeGlobs, globs) {
					cfg.Ship.ExcludeGlobs = globs
					changed = true
				}
			}
			if set("auth-bootstrap") {
				if v := strings.TrimSpace(authBootstrap); cfg.Ship.AuthBootstrap != v {
					cfg.Ship.AuthBootstrap = v
					changed = true
				}
			}

			if changed {
				if err := config.Write(root, cfg); err != nil {
					return fmt.Errorf("write config: %w", err)
				}
			}

			if jsonOut {
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
		},
	}
	f := cmd.Flags()
	f.StringVar(&baseBranch, "base-branch", "", "branch the PR targets and is rebased onto (empty = leave as-is)")
	f.StringVar(&mode, "mode", "", "PR-open mode: ask|auto (empty = leave as-is)")
	f.StringVar(&draft, "draft", "", "open the PR as a draft: true|false (empty = leave as-is)")
	f.StringVar(&exclude, "exclude", "", "comma-separated extra commit-exclude globs (empty = leave as-is)")
	f.StringVar(&authBootstrap, "auth-bootstrap", "", "opt-in auth bootstrap spec: a path to source or an SSH alias (empty = leave as-is)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
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
