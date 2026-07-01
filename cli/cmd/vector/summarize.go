package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/standup"
	"github.com/mariocampbell/vector/internal/state"
	"github.com/spf13/cobra"
)

// summarizeWindow is the activity window the summarize projection covers. A
// post-action summary only needs the recent slice of a spec's timeline (the work
// the just-finished command did), so a fixed short window keeps the projection
// cheap and the prose focused.
const summarizeWindow = "24h"

// summarizeProjection is the shape the vector-summary-writer agent consumes on
// stdin: the spec's identity, its prior summary as context, and the recent
// activity to describe. It mirrors the standup projection's role but is scoped to
// a single spec.
//
// HasWork (always present) tells the caller whether the window contains at least
// one work.logged event — i.e. real implementation work. TemplateSummary (omitted
// when HasWork is true) is the deterministic one-line summary the binary pre-builds
// for structural-only transitions (archive, close, status-change, propose), saving
// a Haiku spawn + two round-trips when there is nothing substantive to describe.
type summarizeProjection struct {
	ID              string                  `json:"id"`
	Title           string                  `json:"title"`
	Status          string                  `json:"status"`
	Ticket          *state.Ticket           `json:"ticket,omitempty"`
	PriorSummary    string                  `json:"priorSummary,omitempty"`
	Events          []standup.TimelineEvent `json:"events"`
	HasWork         bool                    `json:"hasWork"`
	TemplateSummary string                  `json:"templateSummary,omitempty"`
}

// agentSummary is the minimal shape the vector-summary-writer agent emits on
// stdin at commit: just the prose. The structural fields (id/action/generatedAt)
// are owned by the binary.
type agentSummary struct {
	Summary string `json:"summary"`
}

// runSpecSummarize is the per-spec summary entrypoint, mirroring runStandup: with
// no subcommand it projects a spec's recent activity (for the
// vector-summary-writer agent); `commit` persists the generated prose. The binary
// never generates prose — CLI-owns-writes.
// newSpecSummarizeCmd is a single command (not a parent with a cobra `commit`
// child) so it can preserve BOTH orderings the kit commands rely on:
// `summarize commit <id>` and `summarize <id> commit`. A pure cobra
// AddCommand("commit") would only understand the first. This is a deliberate,
// documented exception to the child-command pattern used everywhere else in the
// tree — the RunE detects "commit" in either position over its positional args.
// The commit-only flags (--action, --summary-file) are registered on this single
// command; they are inert in projection mode.
func newSpecSummarizeCmd() *cobra.Command {
	var (
		idFlag      string
		action      string
		summaryFile string
		repoRoot    string
		jsonOut     bool
	)
	cmd := &cobra.Command{
		Use:   "summarize [id] [commit]",
		Short: "project a spec's recent activity, or commit the agent-generated summary",
		RunE: func(_ *cobra.Command, args []string) error {
			// Two-order commit detection: "commit <id>" or "<id> commit".
			commit := false
			var posID string
			switch {
			case len(args) > 0 && args[0] == "commit":
				commit = true
				if len(args) > 1 {
					posID = args[1]
				}
			case len(args) > 1 && args[1] == "commit":
				commit = true
				posID = args[0]
			case len(args) > 0:
				posID = args[0]
			}
			id := idFlag
			if id == "" {
				id = posID
			}

			if commit {
				return runSpecSummarizeCommitBody(id, action, summaryFile, repoRoot, jsonOut)
			}
			return runSpecSummarizeProjection(id, repoRoot, jsonOut)
		},
	}
	f := cmd.Flags()
	f.StringVar(&idFlag, "id", "", "spec id (or pass it as the first argument)")
	f.StringVar(&action, "action", "", "the command that produced this summary (required for commit)")
	f.StringVar(&summaryFile, "summary-file", "", "path to the summary JSON, or - for stdin (required for commit)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit JSON for tooling / the projection for the summary agent")
	return cmd
}

// runSpecSummarizeProjection projects a spec's recent activity for the
// vector-summary-writer agent.
func runSpecSummarizeProjection(id, repoRoot string, jsonOut bool) error {
	if id == "" {
		return errors.New("usage: vector spec summarize <id> [--json]")
	}

	store, err := openStore(repoRoot)
	if err != nil {
		return err
	}
	spec, err := store.ReadSpec(id)
	if err != nil {
		return err
	}
	from, err := standup.ParseSince(summarizeWindow, time.Now())
	if err != nil {
		return err
	}
	events, err := store.ReadEvents()
	if err != nil {
		return err
	}

	timelineEvents := standup.Timeline(events, id, from)
	proj := summarizeProjection{
		ID:     spec.ID,
		Title:  spec.Title,
		Status: string(spec.Status),
		Ticket: spec.Ticket,
		Events: timelineEvents,
	}
	if prior, err := store.ReadSummary(id); err == nil && prior != nil {
		proj.PriorSummary = prior.Summary
	}

	// Determine whether the window contains substantive work (work.logged events).
	// Structural transitions (archive, close, status-change, propose) produce no
	// work.logged; in those cases the binary pre-builds a deterministic template
	// summary so the caller can skip the Haiku spawn entirely.
	hasWork := false
	for _, te := range timelineEvents {
		if te.Type == string(state.EvtWorkLogged) {
			hasWork = true
			break
		}
	}
	proj.HasWork = hasWork
	if !hasWork {
		proj.TemplateSummary = buildTemplateSummary(spec.ID, spec.Title, timelineEvents)
	}

	if jsonOut {
		b, err := json.MarshalIndent(proj, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal projection: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("summarize — %s (%s), %d recent event(s)\n", proj.ID, proj.Status, len(proj.Events))
	fmt.Println("Run with --json and pipe to the vector-summary-writer agent, then `summarize commit`.")
	return nil
}

// runSpecSummarizeCommitBody persists the agent-generated summary. On any
// validation failure (missing flags, unreadable file, invalid/empty prose) it
// writes nothing and returns a clear error — the run leaves summaries.json
// untouched. id/action/summaryFile arrive already resolved by the RunE's two-order
// detection (the id from the positional or --id flag).
func runSpecSummarizeCommitBody(id, action, summaryFile, repoRoot string, jsonOut bool) error {
	if id == "" {
		return errors.New("usage: vector spec summarize commit <id> --action <name> --summary-file -|path")
	}
	if action == "" {
		return errors.New("usage: vector spec summarize commit <id> --action <name> --summary-file -|path")
	}
	if summaryFile == "" {
		return errors.New("usage: vector spec summarize commit <id> --action <name> --summary-file -|path")
	}

	raw, err := readBody(summaryFile)
	if err != nil {
		return fmt.Errorf("cannot read summary file: %w", err)
	}
	var as agentSummary
	if err := json.Unmarshal([]byte(raw), &as); err != nil {
		return errors.New("invalid summary json (expected {\"summary\": \"...\"})")
	}
	summary := strings.TrimSpace(as.Summary)
	if summary == "" {
		return errors.New("empty summary: nothing written")
	}

	store, err := openStore(repoRoot)
	if err != nil {
		return err
	}
	// Validate the spec exists before writing, so a typo'd id is a clear error,
	// not a stray summaries.json entry.
	if _, err := store.ReadSpec(id); err != nil {
		return err
	}

	// Deterministic safeguard against summary degradation on terminal transitions.
	// A close/archive adds no new work, so re-summarizing has nothing fresh to
	// describe — and the cheap agent, asked to summarize anyway, can collapse a rich
	// prior summary into a generic line ("closed after review") and overwrite it.
	// The agent prompt is hardened to preserve, but Haiku is not deterministic, so
	// this guard is the hard guarantee: for close/archive, if a non-empty prior
	// summary exists and no work.logged event was recorded after it, preserve the
	// prior prose instead of overwriting it (CLI-owns-writes — the binary refuses to
	// let an empty-handed regeneration destroy substance).
	if action == "close" || action == "archive" {
		prior, err := store.ReadSummary(id)
		if err != nil {
			return err
		}
		if prior != nil && strings.TrimSpace(prior.Summary) != "" {
			events, err := store.ReadEvents()
			if err != nil {
				return err
			}
			if !hasWorkLoggedAfter(events, id, prior.GeneratedAt) {
				if jsonOut {
					return printJSON(map[string]string{
						"id":        id,
						"action":    action,
						"preserved": "true",
					})
				}
				fmt.Printf("no new work since the last summary; preserved the prior summary for spec %q (action: %s)\n", id, action)
				return nil
			}
		}
	}

	if err := store.WriteSummary(id, summary, action, time.Now()); err != nil {
		return err
	}

	if jsonOut {
		return printJSON(map[string]string{
			"id":     id,
			"action": action,
		})
	}
	fmt.Printf("summary committed for spec %q (action: %s)\n", id, action)
	return nil
}

// hasWorkLoggedAfter reports whether spec id has any work.logged event recorded
// strictly after t — i.e. real work done since the last summary was generated. It
// is the deterministic signal the close/archive safeguard uses to decide whether a
// regeneration has anything new to describe.
func hasWorkLoggedAfter(events []state.Event, id string, t time.Time) bool {
	for _, e := range events {
		if e.SpecID == id && e.Type == state.EvtWorkLogged && e.TS.After(t) {
			return true
		}
	}
	return false
}

// buildTemplateSummary produces a deterministic one-line summary for transitions
// whose activity window contains no work.logged events. It operates only on the
// Type, From, and To fields of each TimelineEvent and never returns an error —
// the caller never handles a nil or zero value.
//
// Priority order (first match wins):
//  1. spec.proposed  → "<label> proposed (draft → open)"
//  2. spec.closed    → "<label> closed"
//  3. spec.archived  → "<label> archived"
//  4. Last status.changed with From and To both non-empty → "<label>: moved from <from> to <to>"
//  5. Fallback       → "spec \"<id>\": no recent activity"
func buildTemplateSummary(id, title string, events []standup.TimelineEvent) string {
	label := title
	if label == "" {
		label = id
	}

	var lastStatusChanged *standup.TimelineEvent
	for i := range events {
		te := &events[i]
		switch te.Type {
		case string(state.EvtSpecProposed):
			return label + " proposed (draft → open)"
		case string(state.EvtSpecClosed):
			return label + " closed"
		case string(state.EvtSpecArchived):
			return label + " archived"
		case string(state.EvtStatusChanged):
			if te.From != "" && te.To != "" {
				lastStatusChanged = te
			}
		}
	}

	if lastStatusChanged != nil {
		return fmt.Sprintf("%s: moved from %s to %s", label, lastStatusChanged.From, lastStatusChanged.To)
	}
	return fmt.Sprintf("spec %q: no recent activity", id)
}
