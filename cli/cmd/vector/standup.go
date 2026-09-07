package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/standup"
	"github.com/mariocampbell/vector/internal/state"
	"github.com/spf13/cobra"
)

const worklogNoteMax = 280

// runStandup is the standup entrypoint: with no subcommand it projects the
// activity period (for the vector-standup-writer agent); `commit` persists the
// generated digest and advances the marker. The binary never generates prose.
func newStandupCmd() *cobra.Command {
	var (
		since    string
		repoRoot string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "standup",
		Short: "project the activity period for the standup digest agent",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			// Capture the window's upper bound once, up front, and reuse it for both
			// the projection filter and (via the JSON's `until`) the commit's marker.
			// This is the fix for leak #2: a second time.Now() at commit would reopen
			// the race where an event written between generation and commit is either
			// dropped from this period or double-counted in the next.
			to := time.Now().UTC()
			from, err := resolveSince(store, since, to)
			if err != nil {
				return err
			}
			events, err := store.ReadEvents()
			if err != nil {
				return err
			}
			proj := standup.Project(events, from, to)
			enrichProjection(store, &proj)
			// Surface the repo's configured prose language to the digest agent. Resolved
			// here (not in enrichProjection) so the standup package never imports config.
			// A config error is non-fatal: the projection must not fail over a dispensable
			// field — empty language just means the agent falls back to the conversation.
			if root, rerr := resolveRepoRoot(repoRoot); rerr == nil {
				if cfg, cerr := config.Load(root); cerr == nil {
					proj.Language = cfg.ResolvedLanguage()
				}
			}

			if jsonOut {
				b, err := json.MarshalIndent(proj, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal projection: %w", err)
				}
				fmt.Println(string(b))
				return nil
			}

			if len(proj.PerSpec) == 0 {
				fmt.Printf("no activity since %s\n", from.Format(time.RFC3339))
				return nil
			}
			fmt.Printf("standup — %d spec(s), %d change(s) since %s\n", proj.Totals.Specs, proj.Totals.Changes, from.Format(time.RFC3339))
			for _, sa := range proj.PerSpec {
				fmt.Printf("  %-32s %-14s %d change(s)\n", sa.ID, sa.LastStatus, sa.ChangeCount)
			}
			fmt.Println("\nRun /vector:standup to generate and persist the digest.")
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&since, "since", "", "window: 24h|today|7d (default: since the last standup marker)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit the projection as JSON for the digest agent")
	cmd.AddCommand(newStandupCommitCmd())
	return cmd
}

// resolveSince resolves the projection window: an explicit --since keyword, else
// the persisted marker (zero time on a first run, covering all history).
func resolveSince(store *state.Store, window string, now time.Time) (time.Time, error) {
	if window == "" {
		d, err := store.ReadStandup()
		if err != nil {
			return time.Time{}, err
		}
		return d.MarkerAt, nil
	}
	from, err := standup.ParseSince(window, now)
	if err != nil {
		if errors.Is(err, standup.ErrInvalidSince) {
			return time.Time{}, errors.New("invalid --since: use 24h, today or 7d")
		}
		return time.Time{}, err
	}
	return from, nil
}

// enrichProjection fills each spec's display Title (and a fallback current
// status) from the store, which the events alone cannot always supply.
func enrichProjection(store *state.Store, proj *standup.Projection) {
	for i := range proj.PerSpec {
		sa := &proj.PerSpec[i]
		spec, err := store.ReadSpec(sa.ID)
		if err != nil {
			continue
		}
		if sa.Title == "" {
			sa.Title = spec.Title
		}
		if sa.LastStatus == "" {
			sa.LastStatus = string(spec.Status)
		}
		// Surface the spec's linked external ticket (nil for unlinked specs) so
		// the standup digest can name it next to the slug. Additive; the join
		// key stays sa.ID.
		sa.Ticket = spec.Ticket
		// Carry the spec's last post-action summary as context for the digest
		// agent (additive; absent when no summary was ever generated). The
		// projection stays store-free, so this enrichment lives here.
		if sum, err := store.ReadSummary(sa.ID); err == nil && sum != nil {
			sa.PriorSummary = sum.Summary
		}
		// Surface the deterministic signals the digest template depends on: the
		// review/UAT distinction (needsUat), the spec's owner (assignee) and the
		// blocked reason (needs-attention flag). All additive; the join key stays
		// sa.ID and unset fields serialize away via omitempty.
		sa.NeedsUAT = spec.NeedsUAT
		sa.Assignee = spec.Assignee
		if spec.Flag != nil {
			sa.AttentionReason = spec.Flag.Reason
		}
	}
}

// agentDigest is the minimal shape the vector-standup-writer agent emits on
// stdin: the global prose plus a per-spec summary. The structural fields
// (title/status/counts/totals) are rebuilt from a fresh projection at commit.
type agentDigest struct {
	Global  string `json:"global"`
	PerSpec []struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
	} `json:"perSpec"`
}

// runStandupCommit persists the agent-generated digest and advances the marker.
// On any validation failure it writes nothing and leaves the marker untouched —
// the run does not count as a standup.
func newStandupCommitCmd() *cobra.Command {
	var (
		digestFile string
		since      string
		until      string
		repoRoot   string
		jsonOut    bool
	)
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "persist the agent-generated digest and advance the standup marker",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStandupCommitBody(digestFile, since, until, repoRoot, jsonOut)
		},
	}
	f := cmd.Flags()
	f.StringVar(&digestFile, "digest-file", "", "path to the digest JSON, or - for stdin (required)")
	f.StringVar(&since, "since", "", "window the digest covers: 24h|today|7d (default: since the marker)")
	f.StringVar(&until, "until", "", "window upper bound as RFC3339, from `vector standup --json`'s `until` (default: now)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

// runStandupCommitBody reprojects the same window the generation step showed the
// agent and advances the marker to exactly its upper bound. To close leak #2, the
// bound `to` must match the `until` the generation captured: /vector:standup reads
// it from `vector standup --json` and passes it back as --until. When --until is
// absent (standalone invocation), it falls back to a fresh now — consistent within
// this single commit, but it cannot recover the generation's boundary, so the wired
// path (generation → --until → commit) is the one that fully closes the race.
func runStandupCommitBody(digestFile, since, until, repoRoot string, jsonOut bool) error {
	if digestFile == "" {
		return errors.New("usage: vector standup commit --digest-file -|path")
	}

	raw, err := readBody(digestFile)
	if err != nil {
		return fmt.Errorf("cannot read digest file: %w", err)
	}
	var ad agentDigest
	if err := json.Unmarshal([]byte(raw), &ad); err != nil {
		return errors.New("invalid digest json")
	}

	store, err := openStore(repoRoot)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	// The marker advances to `to`, the window's upper bound: the `until` captured at
	// generation (passed back via --until), else a fresh now for a standalone commit.
	to := now
	if until != "" {
		parsed, perr := time.Parse(time.RFC3339, until)
		if perr != nil {
			return fmt.Errorf("invalid --until: %w", perr)
		}
		to = parsed.UTC()
	}
	from, err := resolveSince(store, since, now)
	if err != nil {
		return err
	}
	events, err := store.ReadEvents()
	if err != nil {
		return err
	}
	proj := standup.Project(events, from, to)
	enrichProjection(store, &proj)

	summaries := make(map[string]string, len(ad.PerSpec))
	for _, p := range ad.PerSpec {
		summaries[p.ID] = p.Summary
	}
	perSpec := make([]state.StandupSpecDigest, 0, len(proj.PerSpec))
	for _, sa := range proj.PerSpec {
		perSpec = append(perSpec, state.StandupSpecDigest{
			ID:          sa.ID,
			Title:       sa.Title,
			Status:      sa.LastStatus,
			Summary:     summaries[sa.ID],
			ChangeCount: sa.ChangeCount,
			Ticket:      sa.Ticket,
		})
	}
	digest := state.StandupDigest{
		GeneratedAt: now,
		Since:       proj.Since,
		Global:      ad.Global,
		PerSpec:     perSpec,
		Totals: state.StandupTotals{
			Specs:    proj.Totals.Specs,
			Changes:  proj.Totals.Changes,
			ByStatus: proj.Totals.ByStatus,
		},
	}
	if err := store.WriteStandup(digest, to); err != nil {
		return err
	}

	if jsonOut {
		return printJSON(map[string]string{
			"markerAt": to.Format(time.RFC3339),
			"specs":    fmt.Sprintf("%d", digest.Totals.Specs),
			"changes":  fmt.Sprintf("%d", digest.Totals.Changes),
		})
	}
	fmt.Printf("standup committed — %d spec(s), %d change(s); marker advanced to %s\n",
		digest.Totals.Specs, digest.Totals.Changes, to.Format(time.RFC3339))
	return nil
}

// runSpecWorklog appends a work.logged event enriching the activity trace with
// the concrete work a /vector:apply run did. Additive: it never mutates
// state.json. Invoked by /vector:apply after implementing.
func newSpecWorklogCmd() *cobra.Command {
	var (
		idFlag   string
		files    string
		tasks    string
		note     string
		change   string
		repoRoot string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "worklog [id]",
		Short: "append a work.logged event enriching the activity trace",
		RunE: func(_ *cobra.Command, args []string) error {
			id, _ := leadingID(args)
			if id == "" {
				id = idFlag
			}
			if id == "" {
				return errors.New("usage: vector spec worklog <id> [--files ...] [--tasks ...] [--note ...]")
			}
			changeName := change
			if changeName == "" {
				changeName = id
			}

			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			data := state.WorkLoggedData{
				Change:         changeName,
				FilesTouched:   splitCSV(files),
				TasksCompleted: splitCSV(tasks),
				Note:           truncate(strings.TrimSpace(note), worklogNoteMax),
			}
			if err := store.WorkLog(id, data, resolveActor(), time.Now()); err != nil {
				return err
			}

			if jsonOut {
				return printJSON(map[string]string{
					"id":             id,
					"change":         changeName,
					"filesTouched":   fmt.Sprintf("%d", len(data.FilesTouched)),
					"tasksCompleted": fmt.Sprintf("%d", len(data.TasksCompleted)),
				})
			}
			fmt.Printf("logged work for spec %q (%d file(s), %d task(s))\n", id, len(data.FilesTouched), len(data.TasksCompleted))
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&idFlag, "id", "", "spec id (or pass it as the first argument)")
	f.StringVar(&files, "files", "", "comma-separated files touched")
	f.StringVar(&tasks, "tasks", "", "comma-separated tasks completed")
	f.StringVar(&note, "note", "", "short note (truncated to 280 chars)")
	f.StringVar(&change, "change", "", "OpenSpec change name (defaults to the spec id)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

// splitCSV splits a comma list into trimmed, non-empty values; "" → nil.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// truncate caps s to max runes (so a long note never bloats the log line).
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
