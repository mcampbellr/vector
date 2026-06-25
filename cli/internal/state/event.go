package state

import (
	"encoding/json"
	"time"
)

// EventVersion guards migrations of the activity log line format.
const EventVersion = 1

// EventType enumerates the events appended to the local activity log.
type EventType string

const (
	EvtSpecCreated   EventType = "spec.created"
	EvtSpecLinked    EventType = "spec.linked"
	EvtStatusChanged EventType = "status.changed"
	EvtNoteAdded     EventType = "note.added"
	EvtReminderSet   EventType = "reminder.set"
	EvtSpecProposed  EventType = "spec.proposed"
	EvtSpecApplied   EventType = "spec.applied"
	EvtSpecClosed    EventType = "spec.closed"
	EvtSpecArchived  EventType = "spec.archived"
	EvtBoardMoved    EventType = "board.moved"
	EvtAgentRouted   EventType = "agent.routed" // feeds the Token Savings Meter
)

// Event is one line of .vector/local/activity.jsonl (append-only, gitignored,
// personal). Data carries a type-specific payload (decode by Type).
type Event struct {
	V      int             `json:"v"`
	TS     time.Time       `json:"ts"`
	Type   EventType       `json:"type"`
	SpecID string          `json:"specId,omitempty"`
	Repo   string          `json:"repo,omitempty"`
	Actor  string          `json:"actor"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// SpecCreatedData is the payload for EvtSpecCreated.
type SpecCreatedData struct {
	Title    string `json:"title"`
	Source   string `json:"source"`             // "raw"
	Template string `json:"template,omitempty"` // "idea"
}

// StatusChangedData is the payload for EvtStatusChanged.
type StatusChangedData struct {
	From    Status `json:"from"`
	To      Status `json:"to"`
	Trigger string `json:"trigger"` // "command" | "hook" | "apply" | "sync"
	Reason  string `json:"reason,omitempty"`
}

// ProposedData is the payload for EvtSpecProposed: the OpenSpec change a draft
// spec was formalized into and which artifacts were created.
type ProposedData struct {
	Change    string      `json:"change"`
	Artifacts ArtifactSet `json:"artifacts"`
}

// AppliedData is the payload for EvtSpecApplied: the OpenSpec change being
// implemented (empty for a native spec with no change).
type AppliedData struct {
	Change string `json:"change,omitempty"`
}

// SpecLinkedData is the payload for EvtSpecLinked.
type SpecLinkedData struct {
	Provider TicketProvider `json:"provider"`
	Key      string         `json:"key"`
	URL      string         `json:"url"`
	Auto     bool           `json:"auto"`
}

// AgentRoutedData is the payload for EvtAgentRouted — the commercialization
// wedge: every cheap-agent route records what the baseline model would have cost.
type AgentRoutedData struct {
	Task      string  `json:"task"`
	Model     string  `json:"model"`
	Baseline  string  `json:"baseline"`
	TokensIn  int     `json:"tokensIn"`
	TokensOut int     `json:"tokensOut"`
	CostUSD   float64 `json:"costUsd"`
	SavedUSD  float64 `json:"savedUsd"`
}
