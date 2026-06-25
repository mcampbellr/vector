// Command vector is the developer-focused spec/kanban companion CLI for Claude
// Code. It is the sole writer of Vector's on-disk state; the /vector:* project
// commands (seeded by `vector init`) invoke this binary rather than editing
// state directly.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/openspec"
	"github.com/mariocampbell/vector/internal/scaffold"
	"github.com/mariocampbell/vector/internal/state"
)

const version = "0.0.1-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "update":
		err = runUpdate(os.Args[2:])
	case "sync":
		err = runSync(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
	case "standup":
		err = runStandup(os.Args[2:])
	case "spec":
		err = runSpec(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("vector", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runInit seeds the /vector:* project commands into the repo's
// .claude/commands/vector/ and initializes the .vector state skeleton. It is
// additive: nothing else under .claude is touched, and existing command files
// are left intact unless --force is given.
func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	force := fs.Bool("force", false, "overwrite existing /vector:* command files")
	dryRun := fs.Bool("dry-run", false, "show what would be written without writing")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}

	results, err := scaffold.SeedCommands(root, scaffold.SeedOptions{Force: *force, DryRun: *dryRun})
	if err != nil {
		return fmt.Errorf("seed vector commands: %w", err)
	}

	// Resolve the repo config (migrates from .project-structure, else detects,
	// else .vector fallback). Persisted unless it already exists (kept to respect
	// edits) or this is a dry run. cfg is what we'd write / what is in effect.
	cfg := config.Resolve(root)
	cfgExisted := config.Exists(root)
	cfgAction := "written"
	switch {
	case *dryRun:
		cfgAction = "would write"
	case cfgExisted && !*force:
		cfgAction = "skipped (exists)"
		if existing, err := config.Load(root); err == nil {
			cfg = existing // report what's actually in effect
		}
	}

	// Initialize the state skeleton and persist config (unless dry-run).
	if !*dryRun {
		if _, err := state.Open(root); err != nil {
			return fmt.Errorf("init state: %w", err)
		}
		if !cfgExisted || *force {
			cfg.KitVersion = version
			if err := config.Write(root, cfg); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
		}
	}

	if *jsonOut {
		b, err := json.MarshalIndent(struct {
			Root   string                `json:"root"`
			DryRun bool                  `json:"dryRun"`
			Files  []scaffold.FileResult `json:"files"`
			Config *config.Config        `json:"config"`
		}{Root: root, DryRun: *dryRun, Files: results, Config: cfg}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json result: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("vector init: %s\n", root)
	for _, r := range results {
		fmt.Printf("  %-12s %s\n", r.Action, r.Path)
	}
	fmt.Printf("  %-12s .vector/config.json (specPath: %s, source: %s)\n", cfgAction, cfg.SpecPath, cfg.Source)
	if *dryRun {
		fmt.Println("\n(dry run — nothing written; a real init also creates the .vector/ state skeleton)")
		return nil
	}
	if openspec.Detected(root) {
		fmt.Println("\nDetected openspec/ — run `vector sync` to import existing changes onto the board.")
	}
	fmt.Println("\nReload Claude Code (/reload-plugins) to pick up the /vector:* commands.")
	return nil
}

// runUpdate re-seeds the /vector:* kit artifacts (commands, agents, template) to
// match the binary, preserving the repo's config (.vector/config.json) and state
// (.vector/specs, activity). Use it to refresh a repo after upgrading the binary.
func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}
	if !config.Exists(root) {
		return fmt.Errorf("no .vector/config.json in %s — run `vector init` first", root)
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	prev := cfg.KitVersion
	if prev == "" {
		prev = "(unstamped)"
	}

	// Force-overwrite the seeded kit artifacts; never touches config or state.
	results, err := scaffold.SeedCommands(root, scaffold.SeedOptions{Force: true, DryRun: *dryRun})
	if err != nil {
		return fmt.Errorf("re-seed vector kit: %w", err)
	}
	if !*dryRun {
		cfg.KitVersion = version
		if err := config.Write(root, cfg); err != nil {
			return fmt.Errorf("update kit version stamp: %w", err)
		}
	}

	if *jsonOut {
		b, err := json.MarshalIndent(struct {
			Root        string                `json:"root"`
			DryRun      bool                  `json:"dryRun"`
			FromVersion string                `json:"fromVersion"`
			ToVersion   string                `json:"toVersion"`
			Files       []scaffold.FileResult `json:"files"`
		}{Root: root, DryRun: *dryRun, FromVersion: prev, ToVersion: version, Files: results}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json result: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("vector update: %s\n", root)
	fmt.Printf("  kit %s -> %s\n", prev, version)
	for _, r := range results {
		fmt.Printf("  %-12s %s\n", r.Action, r.Path)
	}
	if *dryRun {
		fmt.Println("\n(dry run — nothing written)")
		return nil
	}
	fmt.Println("\nConfig and state preserved. Reload Claude Code (/reload-plugins) to pick up changes.")
	return nil
}

// runSync projects the repo's OpenSpec changes onto the Vector board. It is
// additive and idempotent: new changes become cards (status by task progress),
// existing sync-owned cards are left alone unless --reconcile, and /vector:raw
// drafts are never touched. Applied capability specs (openspec/specs/) are skipped.
func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	branch := fs.String("branch", "", "authoritative worktree for [branch] path templates (bare+worktree layouts); persisted to config")
	reconcile := fs.Bool("reconcile", false, "update status of already-synced cards to match OpenSpec")
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return fmt.Errorf("read .vector/config.json (run `vector init` first): %w", err)
	}
	branchPersisted := resolveSyncBranch(cfg, *branch)

	// Read changes across every worktree (so in-progress changes living only in
	// their own worktree are visible), collapsed to one canonical change per name.
	changes, err := readCanonicalChanges(cfg, root)
	if err != nil {
		return err
	}
	specDocs, err := cfg.FindSpecDocs(root)
	if err != nil {
		return err
	}
	if len(changes) == 0 && len(specDocs) == 0 {
		fmt.Println("nothing to sync (no openspec/changes and no spec docs at the configured spec-path)")
		return nil
	}

	store, err := state.Open(root)
	if err != nil {
		return err
	}
	if branchPersisted && !*dryRun {
		if err := config.Write(root, cfg); err != nil {
			return fmt.Errorf("persist resolved branch: %w", err)
		}
	}
	actor, now := resolveActor(), time.Now()
	seen := make(map[string]bool, len(changes)+len(specDocs))

	// Worktree-folder ticket index (detectTicket's last-resort fallback), computed
	// once per sync. Only meaningful when a default provider is set; empty otherwise
	// or when the layout is not worktree-based.
	var branchKeys map[string]string
	if cfg.ResolvedDefaultTicketProvider() != "" {
		branchKeys, err = cfg.WorktreeTicketKeys(root)
		if err != nil {
			return fmt.Errorf("index worktree ticket keys: %w", err)
		}
	}

	// Index spec docs by slug so a change can point its specDoc at the authoritative
	// doc (same resolved branch), not an arbitrary worktree.
	specBySlug := make(map[string]config.SpecDoc, len(specDocs))
	for _, d := range specDocs {
		specBySlug[d.Slug] = d
	}

	type syncResult struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Action string `json:"action"`
	}
	results := make([]syncResult, 0, len(changes)+len(specDocs))

	for _, c := range changes {
		seen[c.Name] = true
		status := syncStatus(c)
		needsUAT := syncNeedsUAT(c)
		openSpec := &state.OpenSpec{
			Change:    c.Name,
			Artifacts: state.ArtifactSet{Proposal: c.HasProposal, Design: c.HasDesign, Tasks: c.HasTasks},
		}
		specDocRel := c.ProposalRel
		if sd, ok := specBySlug[c.Name]; ok {
			specDocRel = sd.Rel // prefer the authoritative spec doc over the change's proposal
		}
		if specDocRel == "" {
			specDocRel = c.Dir
		}

		// best-effort auto-link (auto:true), nil if none; the default provider +
		// known prefixes enable bare-key detection when configured.
		ticket := detectTicket(c, root, cfg.ResolvedDefaultTicketProvider(), cfg.NormalizedTicketKeyPrefixes(), branchKeys[c.Name])

		existing, rerr := store.ReadSpec(c.Name)
		switch {
		case rerr != nil && !errors.Is(rerr, os.ErrNotExist):
			return rerr
		case rerr != nil: // not found → create
			if !*dryRun {
				if _, err := store.CreateSpec(state.CreateSpecParams{
					ID:         c.Name,
					Title:      humanizeSlug(c.Name),
					Status:     status,
					OpenSpec:   openSpec,
					NeedsUAT:   needsUAT,
					SpecDocRel: specDocRel,
					Ticket:     ticket,
					Actor:      actor,
					Now:        now,
				}); err != nil {
					return err
				}
			}
			results = append(results, syncResult{c.Name, string(status), "created"})
		case existing.OpenSpec == nil: // user-authored (e.g. a /vector:raw draft) — never touch
			results = append(results, syncResult{c.Name, string(existing.Status), "skipped (not sync-owned)"})
		case *reconcile:
			if *dryRun {
				action := "unchanged"
				if existing.Status != status {
					action = "would update"
				}
				results = append(results, syncResult{c.Name, string(status), action})
				break
			}
			changed, err := store.ReconcileStatus(c.Name, status, openSpec, needsUAT, actor, now)
			if err != nil {
				return err
			}
			// Reconcile the ticket too: LinkSpec is idempotent and an auto link
			// never overwrites a manual one.
			if ticket != nil {
				if _, err := store.LinkSpec(c.Name, *ticket, actor, now); err != nil {
					return err
				}
			}
			action := "unchanged"
			if changed {
				action = "updated"
			}
			results = append(results, syncResult{c.Name, string(status), action})
		default:
			results = append(results, syncResult{c.Name, string(existing.Status), "skipped (exists)"})
		}
	}

	// Standalone spec docs with no matching change → import as drafts.
	for _, d := range specDocs {
		if seen[d.Slug] {
			continue // a change with this slug is authoritative
		}
		if d.Superseded {
			// Covered by a change under a different slug (frontmatter supersededBy /
			// status). The change provides the card; do not emit a draft.
			seen[d.Slug] = true
			msg := "skipped (superseded)"
			if d.SupersededBy != "" {
				msg = "skipped (superseded by " + d.SupersededBy + ")"
			}
			results = append(results, syncResult{d.Slug, "—", msg})
			continue
		}
		seen[d.Slug] = true
		existing, rerr := store.ReadSpec(d.Slug)
		switch {
		case rerr != nil && !errors.Is(rerr, os.ErrNotExist):
			return rerr
		case rerr != nil: // not found → create draft
			if !*dryRun {
				if _, err := store.CreateSpec(state.CreateSpecParams{
					ID:         d.Slug,
					Title:      humanizeSlug(d.Slug),
					Status:     state.StatusDraft,
					Source:     "sync",
					SpecDocRel: d.Rel,
					Actor:      actor,
					Now:        now,
				}); err != nil {
					return err
				}
			}
			results = append(results, syncResult{d.Slug, string(state.StatusDraft), "created"})
		default:
			results = append(results, syncResult{d.Slug, string(existing.Status), "skipped (exists)"})
		}
	}

	if *jsonOut {
		b, err := json.MarshalIndent(struct {
			Root   string       `json:"root"`
			DryRun bool         `json:"dryRun"`
			Specs  []syncResult `json:"specs"`
		}{Root: root, DryRun: *dryRun, Specs: results}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json result: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("vector sync: %s (%d changes, %d spec docs)\n", root, len(changes), len(specDocs))
	for _, r := range results {
		fmt.Printf("  %-24s %-12s %s\n", r.Action, r.Status, r.ID)
	}
	if *dryRun {
		fmt.Println("\n(dry run — nothing written)")
	}
	return nil
}

// resolveSyncBranch records a --branch preference (the canonical-copy tie-breaker)
// to config. Branch is a preference, not a filter: sync reads every worktree, so
// an empty branch is never an error. Returns whether the config changed.
func resolveSyncBranch(cfg *config.Config, branchFlag string) bool {
	if branchFlag == "" || cfg.Branch == branchFlag {
		return false
	}
	cfg.Branch = branchFlag
	return true
}

// readCanonicalChanges reads OpenSpec changes from every worktree and collapses
// them to one canonical change per name (so the same change checked out in N
// worktrees is one card, and an in-progress change in its own worktree is seen).
func readCanonicalChanges(cfg *config.Config, root string) ([]openspec.Change, error) {
	dirs, err := cfg.ChangesDirs(root)
	if err != nil {
		return nil, err
	}
	byName := map[string]openspec.Change{}
	for _, bd := range dirs {
		cs, err := openspec.ReadChangesAt(bd.Dir, root)
		if err != nil {
			return nil, err
		}
		for _, ch := range cs {
			ch.Branch = bd.Branch
			if cur, ok := byName[ch.Name]; !ok || moreCanonical(ch, cur, cfg.Branch) {
				byName[ch.Name] = ch
			}
		}
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]openspec.Change, 0, len(names))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out, nil
}

// moreCanonical reports whether candidate should replace current as the canonical
// copy of a change: prefer the configured branch, then a worktree named after the
// change, then the lexically-smaller branch.
func moreCanonical(candidate, current openspec.Change, preferBranch string) bool {
	cs, cur := canonScore(candidate, preferBranch), canonScore(current, preferBranch)
	if cs != cur {
		return cs > cur
	}
	return candidate.Branch < current.Branch
}

func canonScore(c openspec.Change, preferBranch string) int {
	switch {
	case preferBranch != "" && c.Branch == preferBranch:
		return 2
	case c.Branch == c.Name:
		return 1
	default:
		return 0
	}
}

// syncStatus maps an OpenSpec change to a Vector status: archived changes are
// archived; active changes derive from task progress (none done → open; all done,
// or only manual-QA tasks left → review; otherwise in-progress). Changes without
// parseable tasks default to open.
func syncStatus(c openspec.Change) state.Status {
	if c.Archived {
		return state.StatusArchived
	}
	if c.HasTasks && c.TasksTotal > 0 {
		switch {
		case c.TasksDone == 0:
			return state.StatusOpen
		case c.TasksDone >= c.TasksTotal || c.PendingReal == 0:
			// All done, or implementation complete and only manual QA remains.
			return state.StatusReview
		default:
			return state.StatusInProgress
		}
	}
	return state.StatusOpen
}

// syncNeedsUAT reports whether a change reaching review is doing so because only
// manual UAT / verification tasks remain (vs everything done). It is the
// discriminator behind the board's UAT marker, derived from the same task scan
// as syncStatus — no new metadata. False unless work has started and the only
// pending tasks are verification ones (PendingReal == 0 with TasksDone > 0).
func syncNeedsUAT(c openspec.Change) bool {
	return c.HasTasks && c.TasksTotal > 0 &&
		c.TasksDone > 0 && c.TasksDone < c.TasksTotal && c.PendingReal == 0
}

// humanizeSlug turns a kebab-case id into a display title ("billing-v1" → "Billing v1").
func humanizeSlug(slug string) string {
	s := strings.ReplaceAll(slug, "-", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func runSpec(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vector spec <create|list|propose|apply|link|status|close|archive|next|worklog> ...")
	}
	switch args[0] {
	case "create":
		return runSpecCreate(args[1:])
	case "list":
		return runSpecList(args[1:])
	case "propose":
		return runSpecPropose(args[1:])
	case "apply":
		return runSpecApply(args[1:])
	case "link":
		return runSpecLink(args[1:])
	case "status":
		return runSpecStatus(args[1:])
	case "close":
		return runSpecClose(args[1:])
	case "archive":
		return runSpecArchive(args[1:])
	case "next":
		return runSpecNext(args[1:])
	case "worklog":
		return runSpecWorklog(args[1:])
	default:
		return fmt.Errorf("unknown spec subcommand %q", args[0])
	}
}

// runSpecPropose formalizes a draft spec: records the OpenSpec change provenance
// and transitions draft → open. The /vector:propose command creates the actual
// change artifacts (delegated to OpenSpec, or native) and then calls this to flip
// the board state — the binary stays the sole state writer.
func runSpecPropose(args []string) error {
	// Accept the id as a leading positional even when flags follow (Go's flag
	// package stops parsing at the first non-flag arg).
	var leadingID string
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		leadingID, rest = args[0], args[1:]
	}

	fs := flag.NewFlagSet("spec propose", flag.ContinueOnError)
	id := fs.String("id", "", "spec id to propose (or pass it as the first argument)")
	change := fs.String("change", "", "OpenSpec change name (defaults to the spec id)")
	artifacts := fs.String("artifacts", "proposal,design,tasks", "comma list of created artifacts: proposal,design,tasks")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(rest); err != nil {
		return err
	}

	specID := *id
	if specID == "" {
		specID = leadingID
	}
	if specID == "" && fs.NArg() > 0 {
		specID = fs.Arg(0)
	}
	if specID == "" {
		return fmt.Errorf("usage: vector spec propose <id> [--change name] [--artifacts proposal,design,tasks]")
	}
	changeName := *change
	if changeName == "" {
		changeName = specID
	}
	if changeName != state.Slug(changeName) {
		return fmt.Errorf("invalid --change %q: must be kebab-case", changeName)
	}
	arts, err := parseArtifacts(*artifacts)
	if err != nil {
		return err
	}

	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}
	store, err := state.Open(root)
	if err != nil {
		return err
	}

	// Validate up front (covers dry-run and idempotency without writing).
	spec, err := store.ReadSpec(specID)
	if err != nil {
		return err
	}
	if spec.Status == state.StatusOpen {
		if *jsonOut {
			return printJSON(map[string]string{"id": specID, "status": string(spec.Status), "note": "already open"})
		}
		fmt.Printf("spec %q is already open (no change)\n", specID)
		return nil
	}
	if spec.Status != state.StatusDraft {
		return fmt.Errorf("spec %q is %q, not draft (only a draft can be proposed)", specID, spec.Status)
	}

	if *dryRun {
		if *jsonOut {
			return printJSON(map[string]string{"id": specID, "status": "draft", "wouldBe": "open", "change": changeName})
		}
		fmt.Printf("would propose spec %q (draft → open)\n  change: %s\n", specID, changeName)
		return nil
	}

	updated, err := store.ProposeSpec(specID, &state.OpenSpec{Change: changeName, Artifacts: arts}, resolveActor(), time.Now())
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]string{"id": updated.ID, "status": string(updated.Status), "change": changeName, "specDoc": updated.SpecDoc})
	}
	fmt.Printf("proposed spec %q (status: draft → open)\n  change: %s\n  next: /vector:apply %s\n", updated.ID, changeName, updated.ID)
	return nil
}

// parseArtifacts turns a comma list into an ArtifactSet, rejecting unknown names.
func parseArtifacts(list string) (state.ArtifactSet, error) {
	var a state.ArtifactSet
	for _, part := range strings.Split(list, ",") {
		switch strings.TrimSpace(part) {
		case "proposal":
			a.Proposal = true
		case "design":
			a.Design = true
		case "tasks":
			a.Tasks = true
		case "":
			// tolerate empty segments
		default:
			return a, fmt.Errorf("invalid --artifacts %q: allowed proposal,design,tasks", part)
		}
	}
	return a, nil
}

func runSpecCreate(args []string) error {
	fs := flag.NewFlagSet("spec create", flag.ContinueOnError)
	title := fs.String("title", "", "spec title (required unless --id is given)")
	id := fs.String("id", "", "spec id (kebab-case); derived from title if empty")
	repo := fs.String("repo", "", "repo name for the board")
	priority := fs.String("priority", "normal", "urgent|high|normal|low")
	status := fs.String("status", "draft", "draft|open|in-progress|needs-attention|review|closed|archived")
	bodyFile := fs.String("body-file", "", "path to the spec doc body, or - for stdin")
	ticketJSON := fs.String("ticket", "", "seed an external ticket link as JSON {provider,key,url,auto}")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*title) == "" && *id == "" {
		return fmt.Errorf("--title or --id is required")
	}

	body, err := readBody(*bodyFile)
	if err != nil {
		return err
	}
	ticket, err := parseTicketFlag(*ticketJSON)
	if err != nil {
		return err
	}
	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}

	specID := *id
	if specID == "" {
		specID = state.Slug(*title)
	}
	// Resolve the spec doc location from the repo config (migrated by `vector
	// init`). Without a config, CreateSpec falls back to .vector storage.
	var docAbs, docRel string
	if cfg, cfgErr := config.Load(root); cfgErr == nil && specID != "" {
		docRel, docAbs = cfg.SpecDocPath(root, specID)
	}

	store, err := state.Open(root)
	if err != nil {
		return err
	}
	spec, err := store.CreateSpec(state.CreateSpecParams{
		Title:          *title,
		ID:             specID,
		Repo:           *repo,
		Priority:       state.Priority(*priority),
		Status:         state.Status(*status),
		Body:           body,
		Ticket:         ticket,
		Actor:          resolveActor(),
		Now:            time.Now(),
		SpecDocAbsPath: docAbs,
		SpecDocRel:     docRel,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(map[string]string{
			"id":      spec.ID,
			"status":  string(spec.Status),
			"state":   store.StatePath(spec.ID),
			"specDoc": spec.SpecDoc,
		})
	}
	fmt.Printf("created spec %q (status: %s)\n  spec doc: %s\n", spec.ID, spec.Status, spec.SpecDoc)
	return nil
}

func runSpecList(args []string) error {
	fs := flag.NewFlagSet("spec list", flag.ContinueOnError)
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}
	store, err := state.Open(root)
	if err != nil {
		return err
	}
	specs, err := store.ListSpecs()
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		fmt.Println("no specs")
		return nil
	}
	for _, s := range specs {
		fmt.Printf("%-40s %-16s %-8s %s\n", s.ID, s.Status, s.Priority, s.Title)
	}
	return nil
}

func readBody(path string) (string, error) {
	switch path {
	case "":
		return "", nil
	case "-":
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(b), nil
	default:
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read body file: %w", err)
		}
		return string(b), nil
	}
}

// resolveRepoRoot returns the explicit root if given, else the git toplevel,
// else the current working directory.
func resolveRepoRoot(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		if root := strings.TrimSpace(string(out)); root != "" {
			return root, nil
		}
	}
	return os.Getwd()
}

// resolveActor identifies who triggered an action, for the activity log.
func resolveActor() string {
	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name
		}
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "unknown"
}

func printJSON(v map[string]string) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json result: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

func usage() {
	fmt.Fprint(os.Stderr, `vector — developer-focused spec/kanban companion for Claude Code

usage:
  vector init [--repo-root path] [--force] [--dry-run] [--json]
  vector update [--repo-root path] [--dry-run] [--json]
  vector sync [--repo-root path] [--reconcile] [--dry-run] [--json]
  vector serve [--port N] [--host addr] [--web-dir path] [--repo-root path]
  vector standup [--since 24h|today|7d] [--json]
  vector standup commit --digest-file -|path
  vector spec create --title "..." [--id slug] [--repo name] [--priority normal] [--status draft] [--body-file -|path] [--ticket '{"provider":"jira","key":"ACME-1"}'] [--json]
  vector spec propose <id> [--change name] [--artifacts proposal,design,tasks] [--dry-run] [--json]
  vector spec apply <id> [--json]
  vector spec link <id> <ref> [--provider jira|linear|github|other] [--json]
  vector spec status <id> <status> [--reason ...] [--json]
  vector spec close <id> [--json]
  vector spec archive <id> [--json]
  vector spec next [--json]
  vector spec worklog <id> [--files a.go,b.go] [--tasks "DTO mapper"] [--note "..."] [--json]
  vector spec list
  vector version
`)
}
