package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
)

// leadingID pulls an id passed as the first positional argument, even when flags
// follow it (Go's flag package stops parsing at the first non-flag arg).
func leadingID(args []string) (id string, rest []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

// runSpecApply starts work on an open spec: open → in-progress. The /vector:apply
// command calls this to flip board state before implementing the change.
func runSpecApply(args []string) error {
	id, rest := leadingID(args)
	fs := flag.NewFlagSet("spec apply", flag.ContinueOnError)
	idFlag := fs.String("id", "", "spec id to apply (or pass it as the first argument)")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if id == "" {
		id = *idFlag
	}
	if id == "" {
		return fmt.Errorf("usage: vector spec apply <id>")
	}

	store, err := openStore(*repoRoot)
	if err != nil {
		return err
	}
	// Carry the OpenSpec change into the spec.applied event when present.
	change := id
	if spec, rerr := store.ReadSpec(id); rerr == nil && spec.OpenSpec != nil && spec.OpenSpec.Change != "" {
		change = spec.OpenSpec.Change
	}
	updated, err := store.ApplySpec(id, change, resolveActor(), time.Now())
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]string{"id": updated.ID, "status": string(updated.Status), "change": change})
	}
	fmt.Printf("applied spec %q (status: open → in-progress)\n  change: %s\n", updated.ID, change)
	return nil
}

// runSpecLink links a spec to an external ticket (Jira/Linear/GitHub). It parses
// the ref, infers the provider (or honors --provider), and persists a manual link
// (auto:false) via Store.LinkSpec — metadata only, no status change. An ambiguous
// bare key with no --provider yields an actionable error rather than a guess.
func runSpecLink(args []string) error {
	id, rest := leadingID(args)
	// A second leading positional is the ticket ref (e.g. `spec link feat ACME-1`).
	var ref string
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		ref, rest = rest[0], rest[1:]
	}
	fs := flag.NewFlagSet("spec link", flag.ContinueOnError)
	idFlag := fs.String("id", "", "spec id to link (or pass it as the first argument)")
	refFlag := fs.String("ref", "", "ticket ref: a URL, <provider>:<key>, or a bare key with --provider")
	provider := fs.String("provider", "", "force the tracker provider: jira|linear|github|other")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if id == "" {
		id = *idFlag
	}
	if ref == "" {
		ref = *refFlag
	}
	if id == "" || ref == "" {
		return fmt.Errorf("usage: vector spec link <id> <ref> [--provider jira|linear|github|other]")
	}

	// When --provider is omitted, fall back to the repo's defaultTicketProvider so
	// a bare key (e.g. MH-1592) resolves instead of erroring as ambiguous. A missing
	// config is fine (no default); an invalid/corrupt one surfaces.
	forced := *provider
	if forced == "" {
		root, rerr := resolveRepoRoot(*repoRoot)
		if rerr != nil {
			return rerr
		}
		switch cfg, cerr := config.Load(root); {
		case cerr == nil:
			forced = string(cfg.ResolvedDefaultTicketProvider())
		case !errors.Is(cerr, os.ErrNotExist):
			return cerr
		}
	}

	ticket, err := parseRef(ref, forced)
	if err != nil {
		return err
	}
	ticket.Auto = false // manual links are always authoritative

	store, err := openStore(*repoRoot)
	if err != nil {
		return err
	}
	changed, err := store.LinkSpec(id, ticket, resolveActor(), time.Now())
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]string{
			"id":       id,
			"provider": string(ticket.Provider),
			"key":      ticket.Key,
			"url":      ticket.URL,
			"changed":  fmt.Sprintf("%t", changed),
		})
	}
	if !changed {
		fmt.Printf("spec %q already linked to %s %s (no change)\n", id, ticket.Provider, ticket.Key)
		return nil
	}
	fmt.Printf("linked spec %q → %s %s\n", id, ticket.Provider, ticket.Key)
	return nil
}

// runSpecRelate adds one cause→bug relation to a spec (/vector:bug records the
// prior work that caused a bug). It is metadata only — like link, it never changes
// the spec's lifecycle status. The relation is idempotent on {kind,ref}; a
// duplicate is a no-op. A kind:spec ref must point to an existing spec, else the op
// is rejected (no implicit card creation).
func runSpecRelate(args []string) error {
	id, rest := leadingID(args)
	fs := flag.NewFlagSet("spec relate", flag.ContinueOnError)
	idFlag := fs.String("id", "", "spec id to relate (or pass it as the first argument)")
	kind := fs.String("kind", "", "relation kind: spec|ticket")
	ref := fs.String("ref", "", "the cause ref: a Vector spec id (kind=spec) or provider:key (kind=ticket)")
	source := fs.String("source", "manual", "how the relation was found: blame|manual")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if id == "" {
		id = *idFlag
	}
	if id == "" {
		return fmt.Errorf("usage: vector spec relate <id> --kind spec|ticket --ref <ref> [--source blame|manual]")
	}

	item, err := parseRelateFlags(*kind, *ref, *source)
	if err != nil {
		return err
	}

	store, err := openStore(*repoRoot)
	if err != nil {
		return err
	}
	changed, err := store.RelateSpec(id, item, resolveActor(), time.Now())
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSONValue(map[string]any{
			"id":      id,
			"kind":    string(item.Kind),
			"ref":     item.Ref,
			"source":  string(item.Source),
			"changed": changed,
		})
	}
	if !changed {
		fmt.Printf("spec %q already related to %s:%s (no change)\n", id, item.Kind, item.Ref)
		return nil
	}
	fmt.Printf("related spec %q → %s:%s (%s)\n", id, item.Kind, item.Ref, item.Source)
	return nil
}

// runSpecStatus is the generic transition command (/vector:status): it moves a
// spec to a target status if the move is legal. Dedicated transitions (open,
// closed, archived) are routed to propose/close/archive by SetStatus.
func runSpecStatus(args []string) error {
	id, rest := leadingID(args)
	// A second leading positional is the target status (e.g. `spec status feat review`).
	var target string
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		target, rest = rest[0], rest[1:]
	}
	fs := flag.NewFlagSet("spec status", flag.ContinueOnError)
	statusFlag := fs.String("status", "", "target status")
	reason := fs.String("reason", "", "reason (required when entering needs-attention)")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if target == "" {
		target = *statusFlag
	}
	if id == "" || target == "" {
		return fmt.Errorf("usage: vector spec status <id> <status> [--reason ...]")
	}

	store, err := openStore(*repoRoot)
	if err != nil {
		return err
	}
	updated, err := store.SetStatus(id, state.Status(target), *reason, resolveActor(), time.Now())
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]string{"id": updated.ID, "status": string(updated.Status)})
	}
	fmt.Printf("spec %q → %s\n", updated.ID, updated.Status)
	return nil
}

// runSpecClose / runSpecArchive are the closing transitions.
func runSpecClose(args []string) error {
	return runClosingTransition(args, "close", func(store *state.Store, id string) (*state.SpecState, error) {
		return store.CloseSpec(id, resolveActor(), time.Now())
	})
}

func runSpecArchive(args []string) error {
	return runClosingTransition(args, "archive", func(store *state.Store, id string) (*state.SpecState, error) {
		return store.ArchiveSpec(id, resolveActor(), time.Now())
	})
}

func runClosingTransition(args []string, name string, do func(*state.Store, string) (*state.SpecState, error)) error {
	id, rest := leadingID(args)
	fs := flag.NewFlagSet("spec "+name, flag.ContinueOnError)
	idFlag := fs.String("id", "", "spec id")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if id == "" {
		id = *idFlag
	}
	if id == "" {
		return fmt.Errorf("usage: vector spec %s <id>", name)
	}
	store, err := openStore(*repoRoot)
	if err != nil {
		return err
	}
	updated, err := do(store, id)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]string{"id": updated.ID, "status": string(updated.Status)})
	}
	fmt.Printf("%sd spec %q (status: %s)\n", name, updated.ID, updated.Status)
	return nil
}

// runSpecNext recommends the next work-item using Vector's tracked status +
// priority signal — the plus over OpenSpec that powers /vector:apply selection.
func runSpecNext(args []string) error {
	fs := flag.NewFlagSet("spec next", flag.ContinueOnError)
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
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
	// applyMode steers how /vector:apply uses this pick; default ask when unset.
	// applyModel controls which model tier /vector:apply uses for implementation;
	// default opus (current behavior) when unset.
	mode := config.ApplyModeAsk
	applyModel := config.ApplyModelOpus
	if cfg, cerr := config.Load(root); cerr == nil {
		mode = cfg.ResolvedApplyMode()
		applyModel = cfg.ResolvedApplyModel()
	}

	pick := state.SelectNext(specs)
	if pick == nil {
		if *jsonOut {
			return printJSON(map[string]string{"id": "", "applyMode": string(mode), "applyModel": string(applyModel), "note": "nothing actionable"})
		}
		fmt.Println("no actionable spec (only draft/closed/archived remain)")
		return nil
	}
	if *jsonOut {
		return printJSON(map[string]string{
			"id": pick.ID, "status": string(pick.Status), "priority": string(pick.Priority),
			"title": pick.Title, "applyMode": string(mode), "applyModel": string(applyModel),
		})
	}
	fmt.Printf("next: %s  (%s · %s)  [applyMode: %s]  [applyModel: %s]\n  %s\n", pick.ID, pick.Status, pick.Priority, mode, applyModel, pick.Title)
	return nil
}

// runSpecFix records a /vector:fix correction as a spec.fixed event. It never
// transitions status — the command orchestrates lifecycle moves through `vector
// spec status` (the LOCKED machine), so this stays a single additive write and the
// binary remains the sole state writer. The classification is the refiner's
// verdict; --validation-result is informational metadata, not a gate.
func runSpecFix(args []string) error {
	id, rest := leadingID(args)
	fs := flag.NewFlagSet("spec fix", flag.ContinueOnError)
	idFlag := fs.String("id", "", "spec id (or pass it as the first argument)")
	classification := fs.String("classification", "", "correction class: spec-only|code-only|spec+code")
	artifacts := fs.String("artifacts", "", "comma list of OpenSpec artifacts amended: proposal,design,tasks")
	files := fs.String("files", "", "comma-separated code files touched")
	validationResult := fs.String("validation-result", "", "implementer validation outcome: pass|fail (informational)")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if id == "" {
		id = *idFlag
	}
	if id == "" {
		return errors.New("usage: vector spec fix <id> --classification spec-only|code-only|spec+code [--artifacts proposal,design,tasks] [--files ...] [--validation-result pass|fail]")
	}
	if id != state.Slug(id) {
		return fmt.Errorf("invalid spec id %q: must be kebab-case", id)
	}

	class := strings.TrimSpace(*classification)
	switch class {
	case "spec-only", "code-only", "spec+code":
	case "":
		return errors.New("--classification is required (spec-only|code-only|spec+code)")
	default:
		return fmt.Errorf("invalid --classification %q: allowed spec-only|code-only|spec+code", class)
	}

	validation := strings.TrimSpace(*validationResult)
	switch validation {
	case "", "pass", "fail":
	default:
		return fmt.Errorf("invalid --validation-result %q: allowed pass|fail", validation)
	}

	arts, err := parseFixArtifacts(*artifacts)
	if err != nil {
		return err
	}
	touched := splitCSV(*files)

	store, err := openStore(*repoRoot)
	if err != nil {
		return err
	}
	spec, err := store.FixSpec(id, class, validation, arts, touched, resolveActor(), time.Now())
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(map[string]string{
			"id":               spec.ID,
			"status":           string(spec.Status),
			"classification":   class,
			"validationResult": validation,
			"artifacts":        fmt.Sprintf("%d", len(arts)),
			"files":            fmt.Sprintf("%d", len(touched)),
		})
	}
	fmt.Printf("recorded fix for spec %q (%s; %d artifact(s), %d file(s))\n", spec.ID, class, len(arts), len(touched))
	return nil
}

// parseFixArtifacts splits a comma list of amended OpenSpec artifacts, rejecting
// names outside proposal,design,tasks. Casing and an optional .md suffix are
// tolerated (via canonicalArtifact); the returned slice holds the canonical
// names (lowercase, no .md), so persisted state never depends on the input format.
func parseFixArtifacts(list string) ([]string, error) {
	vals := splitCSV(list)
	if len(vals) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		name, ok := canonicalArtifact(v)
		if !ok {
			return nil, fmt.Errorf("invalid --artifacts %q: allowed proposal,design,tasks", v)
		}
		out = append(out, name)
	}
	return out, nil
}

// openStore resolves the repo root and opens the state store — shared by the
// transition subcommands.
func openStore(repoRoot string) (*state.Store, error) {
	root, err := resolveRepoRoot(repoRoot)
	if err != nil {
		return nil, err
	}
	return state.Open(root)
}
