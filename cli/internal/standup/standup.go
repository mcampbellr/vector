// Package standup projects the append-only activity log into a per-spec,
// per-period view for the scrum standup digest. It is a pure, deterministic
// read-only projection: it filters by each event's own timestamp (never the wall
// clock), groups by spec, and never writes, calls an LLM, or touches the network.
// The natural-language prose is produced outside the binary by the
// vector-standup-writer agent (product/token-routing.md).
package standup

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

// ErrInvalidSince is returned by ParseSince for an unsupported window keyword.
// Callers format their own user-facing message (CLI "--since" vs API "since").
var ErrInvalidSince = errors.New("invalid since: use 24h, today or 7d")

// ParseSince resolves a window keyword to its lower bound relative to now. The
// boundary computation is the only use of now; membership is decided by each
// event's timestamp in Project/Timeline. Supported: "24h", "today" (UTC
// midnight), "7d". An empty or unknown keyword is an error — the caller decides
// what "no window" means (e.g. the marker).
func ParseSince(window string, now time.Time) (time.Time, error) {
	now = now.UTC()
	switch window {
	case "24h":
		return now.Add(-24 * time.Hour), nil
	case "7d":
		return now.Add(-7 * 24 * time.Hour), nil
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	default:
		return time.Time{}, ErrInvalidSince
	}
}

// Projection is the structured standup view returned for a period.
type Projection struct {
	Since   time.Time      `json:"since"`
	PerSpec []SpecActivity `json:"perSpec"`
	Totals  Totals         `json:"totals"`
}

// SpecActivity is one spec's activity within the window.
type SpecActivity struct {
	ID          string                    `json:"id"`
	Title       string                    `json:"title,omitempty"`
	LastStatus  string                    `json:"lastStatus,omitempty"`
	LastChanged time.Time                 `json:"lastChanged"`
	ChangeCount int                       `json:"changeCount"`
	Work        []state.WorkLoggedData    `json:"work,omitempty"`
	Transitions []state.StatusChangedData `json:"transitions,omitempty"`
}

// Totals are the period-wide counters.
type Totals struct {
	Specs    int            `json:"specs"`
	Changes  int            `json:"changes"`
	ByStatus map[string]int `json:"byStatus"`
}

// Project groups every event with TS >= since by spec. Title is best-effort from
// a spec.created event seen in the window (callers with a store enrich the rest);
// LastStatus is the latest status.changed.To in the window. byStatus counts specs
// by that last-seen status. Malformed payloads are skipped, never fatal.
func Project(events []state.Event, since time.Time) Projection {
	since = since.UTC()
	order := make([]string, 0)
	bySpec := make(map[string]*SpecActivity)
	total := 0

	for _, e := range events {
		if e.SpecID == "" || e.TS.Before(since) {
			continue
		}
		sa := bySpec[e.SpecID]
		if sa == nil {
			sa = &SpecActivity{ID: e.SpecID}
			bySpec[e.SpecID] = sa
			order = append(order, e.SpecID)
		}
		sa.ChangeCount++
		total++
		if e.TS.After(sa.LastChanged) {
			sa.LastChanged = e.TS.UTC()
		}
		switch e.Type {
		case state.EvtSpecCreated:
			var d state.SpecCreatedData
			if json.Unmarshal(e.Data, &d) == nil && d.Title != "" {
				sa.Title = d.Title
			}
		case state.EvtStatusChanged:
			var d state.StatusChangedData
			if json.Unmarshal(e.Data, &d) == nil {
				sa.Transitions = append(sa.Transitions, d)
				sa.LastStatus = string(d.To)
			}
		case state.EvtWorkLogged:
			var d state.WorkLoggedData
			if json.Unmarshal(e.Data, &d) == nil {
				sa.Work = append(sa.Work, d)
			}
		}
	}

	sort.Strings(order)
	perSpec := make([]SpecActivity, 0, len(order))
	byStatus := make(map[string]int)
	for _, id := range order {
		sa := bySpec[id]
		if sa.LastStatus != "" {
			byStatus[sa.LastStatus]++
		}
		perSpec = append(perSpec, *sa)
	}

	return Projection{
		Since:   since,
		PerSpec: perSpec,
		Totals:  Totals{Specs: len(perSpec), Changes: total, ByStatus: byStatus},
	}
}

// TimelineEvent is one flattened entry of a spec's activity timeline, covering
// both status.changed and work.logged shapes (omitempty keeps each line minimal).
type TimelineEvent struct {
	TS             time.Time `json:"ts"`
	Type           string    `json:"type"`
	From           string    `json:"from,omitempty"`
	To             string    `json:"to,omitempty"`
	Trigger        string    `json:"trigger,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	FilesTouched   []string  `json:"filesTouched,omitempty"`
	TasksCompleted []string  `json:"tasksCompleted,omitempty"`
	Note           string    `json:"note,omitempty"`
}

// Timeline projects a single spec's events (TS >= since) into a flat, ordered
// list for the board's SpecTimeline. Events are returned in file order (the
// activity log is append-only, so that is chronological).
func Timeline(events []state.Event, specID string, since time.Time) []TimelineEvent {
	since = since.UTC()
	out := make([]TimelineEvent, 0)
	for _, e := range events {
		if e.SpecID != specID || e.TS.Before(since) {
			continue
		}
		te := TimelineEvent{TS: e.TS.UTC(), Type: string(e.Type)}
		switch e.Type {
		case state.EvtStatusChanged:
			var d state.StatusChangedData
			if json.Unmarshal(e.Data, &d) == nil {
				te.From, te.To, te.Trigger, te.Reason = string(d.From), string(d.To), d.Trigger, d.Reason
			}
		case state.EvtWorkLogged:
			var d state.WorkLoggedData
			if json.Unmarshal(e.Data, &d) == nil {
				te.FilesTouched, te.TasksCompleted, te.Note = d.FilesTouched, d.TasksCompleted, d.Note
			}
		}
		out = append(out, te)
	}
	return out
}
