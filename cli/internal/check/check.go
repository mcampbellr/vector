// Package check runs Vector's read-only reconciliation sweep: it compares the
// status each spec persists in .vector/specs/<id>/state.json against external
// reality — the state machine, the linked ticket's shape, git branches/worktrees,
// GitHub PRs (via the user's `gh`) and the OpenSpec changes tree — and emits a
// CheckReport. It never contacts Jira (that lives exclusively in the /vector:check
// command, which reads tickets via the Jira MCP and appends its own
// discrepancies) and never writes .vector/. The only git subcommand it runs that
// touches refs is `git fetch origin <base>`, required before any merge/ancestry
// assertion so the sweep never reasons off a stale base ref.
package check

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
)

// Severity ranks how much a discrepancy matters. The command groups the report by
// these values; the reporter agent never re-judges them.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// DiscrepancyType enumerates the drift kinds the binary detects. The two
// Jira-derived kinds (ticket-closed-while-open, broken-ticket-link) are NOT
// declared here — the command constructs them after reading Jira; this package
// has no notion of Jira.
type DiscrepancyType string

const (
	// TypeInvalidState: the status is outside the known lifecycle enum.
	TypeInvalidState DiscrepancyType = "invalid-state"
	// TypeNoTicket: no external ticket is linked (advisory; linking is optional).
	TypeNoTicket DiscrepancyType = "no-ticket"
	// TypeMalformedTicketRef: a ticket is linked but lacks a provider or key.
	TypeMalformedTicketRef DiscrepancyType = "malformed-ticket-ref"
	// TypeMergedButNotClosed: the card's PR is merged but the card is still in review.
	TypeMergedButNotClosed DiscrepancyType = "merged-but-not-closed"
	// TypeReviewWithoutCode: a review card with no PR and no branch anywhere.
	TypeReviewWithoutCode DiscrepancyType = "review-without-code"
	// TypeReviewWithUncommittedWork: a review card whose worktree is dirty.
	TypeReviewWithUncommittedWork DiscrepancyType = "review-with-uncommitted-work"
	// TypePRClosedUnmerged: the card's PR was closed without merging.
	TypePRClosedUnmerged DiscrepancyType = "pr-closed-unmerged"
	// TypeReviewWithPendingTasks: a review card whose tasks.md still has real
	// (non-verification) work pending — what sync --reconcile used to silently
	// regress to in-progress.
	TypeReviewWithPendingTasks DiscrepancyType = "review-with-pending-tasks"
	// TypeOpenSpecChangeWithoutCard: an active OpenSpec change no card references.
	TypeOpenSpecChangeWithoutCard DiscrepancyType = "openspec-change-without-card"
	// TypeCardWithoutChange: a card references an OpenSpec change that exists nowhere.
	TypeCardWithoutChange DiscrepancyType = "card-without-change"
	// TypeBranchWithoutPR: a review card's branch exists, is not an ancestor of
	// origin/<base>, and no PR in any state references it.
	TypeBranchWithoutPR DiscrepancyType = "branch-without-pr"
)

// Discrepancy is one detected drift between a spec's recorded state and reality.
// Suggestion is the exact remediation the reporter surfaces verbatim; FixEligible
// marks the types /vector:check --fix may apply after per-batch confirmation.
type Discrepancy struct {
	SpecID      string          `json:"specId"`
	Type        DiscrepancyType `json:"type"`
	Severity    Severity        `json:"severity"`
	Description string          `json:"description"`
	Suggestion  string          `json:"suggestion"`
	FixEligible bool            `json:"fixEligible"`
}

// Note is an informational observation that is expected or needs a human call,
// not a typed problem — e.g. a dirty worktree on an in-progress card, or an
// ambiguous PR match. Never carries a severity and is never fix-eligible.
type Note struct {
	SpecID      string `json:"specId,omitempty"`
	Description string `json:"description"`
}

// SpecCheckable is a spec carrying a well-formed ticket the command must
// cross-check against Jira. Only ticketed specs appear here.
type SpecCheckable struct {
	ID     string        `json:"id"`
	Title  string        `json:"title"`
	Status state.Status  `json:"status"`
	Ticket *state.Ticket `json:"ticket,omitempty"`
}

// CheckReport is the sweep's full output: the specs to cross-check against Jira,
// the discrepancies and notes found, and the resolved prose Language for the
// reporter agent. Warnings lists each degradation (git/gh missing, fetch failed)
// for the caller to print on stderr; it is not part of the JSON contract.
type CheckReport struct {
	GeneratedAt   time.Time       `json:"generatedAt"`
	Language      string          `json:"language,omitempty"`
	Specs         []SpecCheckable `json:"specs"`
	Discrepancies []Discrepancy   `json:"discrepancies"`
	Notes         []Note          `json:"notes"`
	Warnings      []string        `json:"-"`
}

// Run performs the reconciliation sweep over all non-archived specs. Local checks
// (state machine, ticket shape, review-with-pending-tasks) always run; when
// skipRemote is false it also runs the git/gh matching cascade and the OpenSpec
// change↔card drift. It is read-only with respect to .vector/.
func Run(store *state.Store, cfg *config.Config, root string, skipRemote bool) (CheckReport, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	specs, err := store.ListSpecs()
	if err != nil {
		return CheckReport{}, fmt.Errorf("list specs: %w", err)
	}

	report := CheckReport{
		GeneratedAt:   time.Now().UTC(),
		Language:      cfg.ResolvedLanguage(),
		Specs:         []SpecCheckable{},
		Discrepancies: []Discrepancy{},
		Notes:         []Note{},
	}

	active := make([]*state.SpecState, 0, len(specs))
	for _, spec := range specs {
		if spec.Status != state.StatusArchived {
			active = append(active, spec)
		}
	}

	changes, err := loadChanges(cfg, root)
	if err != nil {
		report.warn("could not read OpenSpec changes (%v); skipping review-with-pending-tasks and OpenSpec drift", err)
	}

	var probe *gitProbe
	if !skipRemote {
		probe = newGitProbe(cfg, root, &report)
	}

	var ticketless []string
	for _, spec := range active {
		checkState(&report, spec)
		if ticketlessSpec := checkTicket(&report, spec); ticketlessSpec {
			ticketless = append(ticketless, spec.ID)
		}
		if changes != nil {
			checkPendingTasks(&report, spec, changes)
		}
		if probe != nil {
			probe.checkSpec(&report, spec)
		}
	}
	resolveNoTicket(&report, ticketless)

	if !skipRemote && changes != nil {
		checkOpenSpecDrift(&report, specs, changes)
	}

	sortReport(&report)
	return report, nil
}

// checkState flags a status outside the lifecycle enum (a hand-edited or
// badly-merged state.json).
func checkState(report *CheckReport, spec *state.SpecState) {
	if spec.Status.Valid() {
		return
	}
	report.add(Discrepancy{
		SpecID:      spec.ID,
		Type:        TypeInvalidState,
		Severity:    SeverityHigh,
		Description: fmt.Sprintf("status %q is not a known lifecycle state", spec.Status),
		Suggestion:  fmt.Sprintf("vector spec status %s <open|in-progress|needs-attention|review|closed>", spec.ID),
	})
}

// checkTicket classifies the linked ticket: malformed → High discrepancy,
// well-formed → queued in Specs for the command's Jira pass. It reports whether
// the spec has no ticket at all (decided in bulk by resolveNoTicket).
func checkTicket(report *CheckReport, spec *state.SpecState) (ticketless bool) {
	switch {
	case spec.Ticket == nil:
		return true
	case strings.TrimSpace(string(spec.Ticket.Provider)) == "" || strings.TrimSpace(spec.Ticket.Key) == "":
		report.add(Discrepancy{
			SpecID:      spec.ID,
			Type:        TypeMalformedTicketRef,
			Severity:    SeverityHigh,
			Description: "linked ticket is missing a provider or key and cannot be resolved",
			Suggestion:  fmt.Sprintf("vector spec link %s <ticket-ref>", spec.ID),
		})
	default:
		report.Specs = append(report.Specs, SpecCheckable{
			ID:     spec.ID,
			Title:  spec.Title,
			Status: spec.Status,
			Ticket: spec.Ticket,
		})
	}
	return false
}

// resolveNoTicket decides how ticketless specs surface. When NO spec links a
// ticket the project simply doesn't use a tracker — that is not per-spec drift, so
// it collapses to one advisory Note instead of flooding the report with one Low
// discrepancy per card. When some specs are ticketed, a ticketless one is a real
// outlier and stays a Low discrepancy.
func resolveNoTicket(report *CheckReport, ticketless []string) {
	if len(ticketless) == 0 {
		return
	}
	if len(report.Specs) == 0 {
		report.Notes = append(report.Notes, Note{
			Description: fmt.Sprintf("no spec links an external ticket (%d spec(s)); linking is optional — use `vector spec link <id> <ticket-ref>` if you adopt a tracker", len(ticketless)),
		})
		return
	}
	for _, id := range ticketless {
		report.add(Discrepancy{
			SpecID:      id,
			Type:        TypeNoTicket,
			Severity:    SeverityLow,
			Description: "no external ticket is linked to this spec",
			Suggestion:  fmt.Sprintf("vector spec link %s <ticket-ref>", id),
		})
	}
}

// checkPendingTasks flags a review card whose change still has real
// (non-verification) tasks unchecked. Report-only: moving it back is a human call.
func checkPendingTasks(report *CheckReport, spec *state.SpecState, changes changeIndex) {
	if spec.Status != state.StatusReview || spec.OpenSpec == nil {
		return
	}
	change, ok := changes.active(spec.OpenSpec.Change)
	if !ok || change.PendingReal == 0 {
		return
	}
	report.add(Discrepancy{
		SpecID:      spec.ID,
		Type:        TypeReviewWithPendingTasks,
		Severity:    SeverityMedium,
		Description: fmt.Sprintf("card is in review but %s still has %d pending implementation task(s)", change.Dir+"/tasks.md", change.PendingReal),
		Suggestion:  fmt.Sprintf("vector spec status %s in-progress", spec.ID),
	})
}

func (r *CheckReport) add(d Discrepancy) {
	d.FixEligible = fixEligible(d.Type)
	r.Discrepancies = append(r.Discrepancies, d)
}

func (r *CheckReport) note(specID, format string, args ...any) {
	r.Notes = append(r.Notes, Note{SpecID: specID, Description: fmt.Sprintf(format, args...)})
}

func (r *CheckReport) warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// fixEligible is the single source of truth for which local types /vector:check
// --fix may apply. pr-closed-unmerged, review-with-pending-tasks and
// branch-without-pr are always report-only.
func fixEligible(t DiscrepancyType) bool {
	switch t {
	case TypeMergedButNotClosed, TypeReviewWithoutCode, TypeReviewWithUncommittedWork, TypeOpenSpecChangeWithoutCard:
		return true
	}
	return false
}

// severityRank orders high → medium → low for a deterministic report.
func severityRank(s Severity) int {
	switch s {
	case SeverityHigh:
		return 0
	case SeverityMedium:
		return 1
	}
	return 2
}

// sortReport makes --json byte-stable for an unchanged repo (except generatedAt):
// discrepancies by severity, spec id, then type; notes by spec id.
func sortReport(report *CheckReport) {
	sort.SliceStable(report.Discrepancies, func(i, j int) bool {
		a, b := report.Discrepancies[i], report.Discrepancies[j]
		if ra, rb := severityRank(a.Severity), severityRank(b.Severity); ra != rb {
			return ra < rb
		}
		if a.SpecID != b.SpecID {
			return a.SpecID < b.SpecID
		}
		return a.Type < b.Type
	})
	sort.SliceStable(report.Notes, func(i, j int) bool {
		return report.Notes[i].SpecID < report.Notes[j].SpecID
	})
}
