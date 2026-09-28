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
	EvtSpecCreated    EventType = "spec.created"
	EvtSpecLinked     EventType = "spec.linked"
	EvtSpecRelated    EventType = "spec.related" // cause→bug relation recorded on a spec
	EvtStatusChanged  EventType = "status.changed"
	EvtNoteAdded      EventType = "note.added"
	EvtReminderSet    EventType = "reminder.set"
	EvtSpecProposed   EventType = "spec.proposed"
	EvtSpecApplied    EventType = "spec.applied"
	EvtSpecFixed      EventType = "spec.fixed" // a /vector:fix correction (additive; never transitions)
	EvtSpecClosed     EventType = "spec.closed"
	EvtSpecArchived   EventType = "spec.archived"
	EvtBoardMoved     EventType = "board.moved"
	EvtAgentRouted    EventType = "agent.routed"    // feeds the Token Savings Meter
	EvtWorkLogged     EventType = "work.logged"     // enriched apply trace for the standup digest
	EvtSketchAttached EventType = "sketch.attached" // a UI wireframe was attached to a spec
	EvtPROpened       EventType = "pr.opened"       // a /vector:ship pull request (additive; never transitions)
	EvtSpecFocused    EventType = "spec.focused"    // the developer marked a spec to work on first (never transitions)
	EvtSpecUnfocused  EventType = "spec.unfocused"  // the focus marker was removed (never transitions)
	EvtEpicAssigned   EventType = "spec.epic-assigned"
	EvtEpicCreated    EventType = "epic.created"
	EvtEpicUpdated    EventType = "epic.updated"
	EvtEpicDeleted    EventType = "epic.deleted"
	EvtEpicFocused    EventType = "epic.focused"   // the epic's specs inherit focus (never transitions)
	EvtEpicUnfocused  EventType = "epic.unfocused" // the epic's focus marker was removed
)

// ResolutionData is the payload of EvtSpecClosed (always, since a close records
// its resolution) and of EvtSpecArchived when the archive overrides the
// resolution. Additive: spec.closed events written before resolutions existed
// carry no data.
type ResolutionData struct {
	Resolution Resolution `json:"resolution"`
	Note       string     `json:"note,omitempty"`
}

// EpicAssignedData is the payload for EvtEpicAssigned: the epic a spec now belongs
// to (Epic, "" when cleared) and the one it left (Previous, "" when it had none).
// Purely additive — assigning an epic never transitions status.
type EpicAssignedData struct {
	Epic     string `json:"epic"`
	Previous string `json:"previous,omitempty"`
}

// EpicEventData is the payload for EvtEpicCreated/EvtEpicUpdated/EvtEpicDeleted.
// Epic events carry no SpecID (an epic is not a spec), so spec-scoped projections
// such as the standup digest and the per-spec timeline skip them.
type EpicEventData struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	Color EpicColor `json:"color,omitempty"`
}

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

// WorkLoggedData is the payload for EvtWorkLogged: the concrete work done during
// a /vector:apply run — the "what was done" the standup digest needs that
// status.changed alone cannot express. Purely additive; a consumer that does not
// know work.logged ignores it.
type WorkLoggedData struct {
	Change         string   `json:"change,omitempty"`
	FilesTouched   []string `json:"filesTouched,omitempty"`
	TasksCompleted []string `json:"tasksCompleted,omitempty"`
	Note           string   `json:"note,omitempty"`
}

// FixedData is the payload for EvtSpecFixed: a /vector:fix correction recorded on
// an in-flight spec. Classification is the refiner's verdict (spec-only|code-only|
// spec+code); ValidationResult is the implementer's informational outcome
// (pass|fail, optional); Artifacts lists the OpenSpec artifacts amended and Files
// the code files touched. Purely additive — it never transitions status.
type FixedData struct {
	Classification   string   `json:"classification"`
	ValidationResult string   `json:"validationResult,omitempty"`
	Artifacts        []string `json:"artifacts,omitempty"`
	Files            []string `json:"files,omitempty"`
}

// SpecRelatedData is the payload for EvtSpecRelated: a cause→bug relation added to
// a spec. Mirrors RelatedItem; purely additive for the timeline/standup.
type SpecRelatedData struct {
	Kind   RelatedKind   `json:"kind"`
	Ref    string        `json:"ref"`
	Source RelatedSource `json:"source"`
}

// PROpenedData is the payload for EvtPROpened: the pull request /vector:ship opened
// for a spec. Purely additive — recording a PR never transitions status, so a
// consumer that does not know pr.opened simply ignores it.
type PROpenedData struct {
	URL    string `json:"url"`
	Number int    `json:"number,omitempty"`
	Draft  bool   `json:"draft"`
}

// SpecLinkedData is the payload for EvtSpecLinked.
type SpecLinkedData struct {
	Provider TicketProvider `json:"provider"`
	Key      string         `json:"key"`
	URL      string         `json:"url"`
	Auto     bool           `json:"auto"`
}

// SketchAttachedData is the payload for EvtSketchAttached: a UI wireframe attached
// to a spec via Store.AttachSketch. Purely additive for the timeline/standup; a
// consumer that does not know sketch.attached ignores it.
type SketchAttachedData struct {
	Name string `json:"name"`
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
	// Precision is the data quality of the token counts:
	//   "actual"    = token counts reported by the harness (exact measurement).
	//   "estimated" = self-reported by the orchestrating command (default).
	// Absent in events written before this field was introduced; the board
	// rollup treats "" as "estimated" for backward compatibility.
	Precision string `json:"precision,omitempty"`
}
