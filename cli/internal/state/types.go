// Package state owns Vector's per-spec committed state and the local activity log.
// It is the single writer of these on-disk files; no other component edits them
// directly (CLI-owns-writes — see docs/domain-contract.md and
// .claude/rules/architecture/state-model.md).
package state

import "time"

// SchemaVersion guards migrations of the on-disk SpecState format.
const SchemaVersion = 1

// Status is the lifecycle state of a spec. Board columns map 1:1 to these
// (single-axis), except Archived which lives in a separate view.
type Status string

const (
	// StatusDraft is a captured spec with no OpenSpec change yet (the output of
	// /vector:raw). It precedes StatusOpen, which means the change exists.
	StatusDraft          Status = "draft"
	StatusOpen           Status = "open"
	StatusInProgress     Status = "in-progress"
	StatusNeedsAttention Status = "needs-attention"
	StatusReview         Status = "review"
	StatusClosed         Status = "closed"
	StatusArchived       Status = "archived"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusOpen, StatusInProgress, StatusNeedsAttention, StatusReview, StatusClosed, StatusArchived:
		return true
	}
	return false
}

// IsTerminal reports whether s is a terminal lifecycle state (closed or archived)
// — a deliberate post-review human decision that a tasks.md-derived status must
// never pull a card back out of (see ReconcileStatus and sync --reconcile).
func (s Status) IsTerminal() bool {
	return s == StatusClosed || s == StatusArchived
}

// Priority drives intra-column ordering on the board.
type Priority string

const (
	PriorityUrgent Priority = "urgent"
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

// Valid reports whether p is a known priority.
func (p Priority) Valid() bool {
	switch p {
	case PriorityUrgent, PriorityHigh, PriorityNormal, PriorityLow:
		return true
	}
	return false
}

// RelatedKind is the kind of entity a relation points to. V1 records the cause of
// a bug as either a prior Vector spec or an external ticket; a commit/PR kind is a
// deliberate future addition, out of V1 (see design.md).
type RelatedKind string

const (
	RelatedSpec   RelatedKind = "spec"
	RelatedTicket RelatedKind = "ticket"
)

// Valid reports whether k is a known related kind.
func (k RelatedKind) Valid() bool {
	switch k {
	case RelatedSpec, RelatedTicket:
		return true
	}
	return false
}

// RelatedSource records how a relation was established: deduced from git blame/log
// (an inference signal) or entered/confirmed by the user.
type RelatedSource string

const (
	RelatedBlame  RelatedSource = "blame"
	RelatedManual RelatedSource = "manual"
)

// Valid reports whether s is a known related source.
func (s RelatedSource) Valid() bool {
	switch s {
	case RelatedBlame, RelatedManual:
		return true
	}
	return false
}

// TicketProvider identifies the external tracker a spec is linked to.
type TicketProvider string

const (
	TicketJira   TicketProvider = "jira"
	TicketLinear TicketProvider = "linear"
	TicketGitHub TicketProvider = "github"
	TicketOther  TicketProvider = "other"
)

// Valid reports whether p is a known ticket provider.
func (p TicketProvider) Valid() bool {
	switch p {
	case TicketJira, TicketLinear, TicketGitHub, TicketOther:
		return true
	}
	return false
}

// SpecState is the committed, per-spec source of truth at
// .vector/specs/<id>/state.json. It is slow-changing so merge conflicts stay
// local to the spec being edited. Token economics are NOT stored here — they
// are derived from the activity log.
type SpecState struct {
	SchemaVersion int      `json:"schemaVersion"`
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Status        Status   `json:"status"`
	Priority      Priority `json:"priority"`
	Repo          string   `json:"repo"`
	SpecDoc       string   `json:"specDoc,omitempty"` // repo-relative path to the authored spec doc
	Stage         string   `json:"stage,omitempty"`
	Assignee      string   `json:"assignee,omitempty"`
	Labels        []string `json:"labels,omitempty"`
	EstimateMin   int      `json:"estimateMinutes,omitempty"`

	// NeedsUAT marks a review card whose only remaining work is manual UAT /
	// verification. It is derived from the change's tasks.md (set by sync, not by
	// hand) and cleared whenever the card leaves review. See docs/domain-contract.md.
	NeedsUAT bool `json:"needsUat,omitempty"`

	// QuickWin marks a card created by /vector:quick as a small one-run change
	// (applied directly, no OpenSpec change, no Sonnet validator). Persisted as a
	// read-only marker; surfaced on the board projection as a badge. Backward-
	// compatible (omitempty) so specs without it read/serialize byte-identically.
	QuickWin bool `json:"quickWin,omitempty"`

	Ticket   *Ticket    `json:"ticket,omitempty"`
	OpenSpec *OpenSpec  `json:"openspec,omitempty"`
	Flag     *Attention `json:"needsAttention,omitempty"`

	// RelatedTo records the prior work that caused this spec (used by /vector:bug
	// to trace a bug to its root cause). Optional and omitempty, so specs without
	// relations read/serialize byte-identically. Relating never changes status.
	RelatedTo []RelatedItem `json:"relatedTo,omitempty"`

	// RFC3339 UTC. These cover cycle-time analytics without a transition history
	// array — full history is reconstructable from git log of this file plus the
	// activity log.
	CreatedAt  time.Time  `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	ReviewAt   *time.Time `json:"reviewAt,omitempty"`
	ClosedAt   *time.Time `json:"closedAt,omitempty"`
	ArchivedAt *time.Time `json:"archivedAt,omitempty"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// RelatedItem is one cause→bug relation persisted on a spec. Ref is a Vector spec
// id when Kind is RelatedSpec, or a provider:key (e.g. jira:ACME-12) when Kind is
// RelatedTicket. Source distinguishes a git-deduced relation from a manual one.
type RelatedItem struct {
	Kind   RelatedKind   `json:"kind"`
	Ref    string        `json:"ref"`
	Source RelatedSource `json:"source"`
}

// Ticket links a spec to an external tracker.
type Ticket struct {
	Provider TicketProvider `json:"provider"`
	Key      string         `json:"key"`
	URL      string         `json:"url"`
	Auto     bool           `json:"auto"` // true if auto-detected from raw text
}

// OpenSpec references the OpenSpec change a spec was applied to.
type OpenSpec struct {
	Change    string      `json:"change"`
	Artifacts ArtifactSet `json:"artifacts"`
}

// ArtifactSet tracks which OpenSpec artifacts exist for the change.
type ArtifactSet struct {
	Proposal bool `json:"proposal"`
	Design   bool `json:"design"`
	Tasks    bool `json:"tasks"`
}

// Attention is set when Status is StatusNeedsAttention.
type Attention struct {
	Reason string    `json:"reason"`
	Since  time.Time `json:"since"`
	Source string    `json:"source,omitempty"` // "hook" | "command"
}
