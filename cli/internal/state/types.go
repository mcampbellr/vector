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
	case StatusOpen, StatusInProgress, StatusNeedsAttention, StatusReview, StatusClosed, StatusArchived:
		return true
	}
	return false
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

// TicketProvider identifies the external tracker a spec is linked to.
type TicketProvider string

const (
	TicketJira   TicketProvider = "jira"
	TicketLinear TicketProvider = "linear"
	TicketGitHub TicketProvider = "github"
	TicketOther  TicketProvider = "other"
)

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
	Stage         string   `json:"stage,omitempty"`
	Assignee      string   `json:"assignee,omitempty"`
	Labels        []string `json:"labels,omitempty"`
	EstimateMin   int      `json:"estimateMinutes,omitempty"`

	Ticket   *Ticket    `json:"ticket,omitempty"`
	OpenSpec *OpenSpec  `json:"openspec,omitempty"`
	Flag     *Attention `json:"needsAttention,omitempty"`

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
