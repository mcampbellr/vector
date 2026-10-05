package standup

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

func ts(day, hour int) time.Time {
	return time.Date(2026, 6, day, hour, 0, 0, 0, time.UTC)
}

func statusEvent(t *testing.T, specID string, at time.Time, from, to state.Status) state.Event {
	t.Helper()
	data, err := json.Marshal(state.StatusChangedData{From: from, To: to, Trigger: "apply"})
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	return state.Event{V: 1, TS: at, Type: state.EvtStatusChanged, SpecID: specID, Data: data}
}

func workEvent(t *testing.T, specID string, at time.Time, files, tasks []string, note string) state.Event {
	t.Helper()
	data, err := json.Marshal(state.WorkLoggedData{FilesTouched: files, TasksCompleted: tasks, Note: note})
	if err != nil {
		t.Fatalf("marshal work: %v", err)
	}
	return state.Event{V: 1, TS: at, Type: state.EvtWorkLogged, SpecID: specID, Data: data}
}

func fixedEvent(t *testing.T, specID string, at time.Time, classification string, files []string) state.Event {
	t.Helper()
	data, err := json.Marshal(state.FixedData{Classification: classification, Files: files})
	if err != nil {
		t.Fatalf("marshal fixed: %v", err)
	}
	return state.Event{V: 1, TS: at, Type: state.EvtSpecFixed, SpecID: specID, Data: data}
}

// openWindow is the zero upper bound: an open-ended [since, ∞) window, matching
// the board/summarize callers and preserving the pre-half-open test behavior.
var openWindow = time.Time{}

func TestProjectFiltersBySinceAndGroupsBySpec(t *testing.T) {
	events := []state.Event{
		statusEvent(t, "alpha", ts(20, 9), state.StatusOpen, state.StatusInProgress), // before since
		statusEvent(t, "alpha", ts(24, 10), state.StatusInProgress, state.StatusReview),
		workEvent(t, "alpha", ts(24, 11), []string{"a.go", "b.go"}, []string{"DTO mapper"}, "wired"),
		statusEvent(t, "beta", ts(24, 12), state.StatusOpen, state.StatusInProgress),
	}
	proj := Project(events, ts(24, 0), openWindow)

	if proj.Totals.Specs != 2 {
		t.Fatalf("specs = %d, want 2", proj.Totals.Specs)
	}
	if proj.Totals.Changes != 3 {
		t.Errorf("changes = %d, want 3 (pre-since event excluded)", proj.Totals.Changes)
	}
	// perSpec is sorted by id, so alpha is first.
	alpha := proj.PerSpec[0]
	if alpha.ID != "alpha" || alpha.ChangeCount != 2 {
		t.Errorf("alpha = %+v, want id alpha with 2 changes", alpha)
	}
	if alpha.LastStatus != "review" {
		t.Errorf("alpha lastStatus = %q, want review", alpha.LastStatus)
	}
	if len(alpha.Work) != 1 || alpha.Work[0].Note != "wired" {
		t.Errorf("alpha work = %+v, want one work entry", alpha.Work)
	}
	// Project is store-free: the ticket is filled by the caller's enrichProjection,
	// never by the pure projection.
	if alpha.Ticket != nil {
		t.Errorf("alpha ticket = %+v, want nil (Project must stay store-free)", alpha.Ticket)
	}
	if proj.Totals.ByStatus["review"] != 1 || proj.Totals.ByStatus["in-progress"] != 1 {
		t.Errorf("byStatus = %+v, want review:1 in-progress:1", proj.Totals.ByStatus)
	}
}

func TestProjectEmptyPeriodNoPanic(t *testing.T) {
	proj := Project(nil, ts(24, 0), openWindow)
	if proj.Totals.Specs != 0 || len(proj.PerSpec) != 0 {
		t.Errorf("empty projection should be empty, got %+v", proj)
	}
	// All-filtered-out is also empty, not a panic.
	proj = Project([]state.Event{statusEvent(t, "x", ts(1, 0), state.StatusOpen, state.StatusInProgress)}, ts(24, 0), openWindow)
	if proj.Totals.Specs != 0 {
		t.Errorf("everything before since should project empty, got %+v", proj)
	}
}

func TestProjectTitleFromSpecCreated(t *testing.T) {
	created, err := json.Marshal(state.SpecCreatedData{Title: "Alpha Feature", Source: "raw"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	events := []state.Event{
		{V: 1, TS: ts(24, 8), Type: state.EvtSpecCreated, SpecID: "alpha", Data: created},
		statusEvent(t, "alpha", ts(24, 9), state.StatusOpen, state.StatusInProgress),
	}
	proj := Project(events, ts(24, 0), openWindow)
	if proj.PerSpec[0].Title != "Alpha Feature" {
		t.Errorf("title = %q, want Alpha Feature", proj.PerSpec[0].Title)
	}
}

func TestParseSince(t *testing.T) {
	now := ts(24, 12)
	cases := []struct {
		window string
		want   time.Time
		err    bool
	}{
		{"24h", now.Add(-24 * time.Hour), false},
		{"7d", now.Add(-7 * 24 * time.Hour), false},
		{"today", ts(24, 0), false},
		{"", time.Time{}, true},
		{"36h", time.Time{}, true},
		{"yesterday", time.Time{}, true},
	}
	for _, c := range cases {
		got, err := ParseSince(c.window, now)
		if c.err {
			if !errors.Is(err, ErrInvalidSince) {
				t.Errorf("ParseSince(%q) err = %v, want ErrInvalidSince", c.window, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSince(%q) unexpected err: %v", c.window, err)
		}
		if !got.Equal(c.want) {
			t.Errorf("ParseSince(%q) = %v, want %v", c.window, got, c.want)
		}
	}
}

func TestTimelineFlattensSpecEvents(t *testing.T) {
	events := []state.Event{
		statusEvent(t, "alpha", ts(24, 9), state.StatusOpen, state.StatusInProgress),
		workEvent(t, "alpha", ts(24, 10), []string{"a.go"}, []string{"task"}, "note"),
		statusEvent(t, "beta", ts(24, 11), state.StatusOpen, state.StatusInProgress), // other spec
		statusEvent(t, "alpha", ts(20, 0), state.StatusOpen, state.StatusInProgress), // before since
	}
	tl := Timeline(events, "alpha", ts(24, 0), openWindow)
	if len(tl) != 2 {
		t.Fatalf("timeline len = %d, want 2", len(tl))
	}
	if tl[0].Type != "status.changed" || tl[0].To != "in-progress" {
		t.Errorf("first event = %+v, want status.changed to in-progress", tl[0])
	}
	if tl[1].Type != "work.logged" || tl[1].Note != "note" || len(tl[1].FilesTouched) != 1 {
		t.Errorf("second event = %+v, want work.logged with files and note", tl[1])
	}
}

// TestProjectDecodesSpecFixed asserts a spec.fixed event both counts (ChangeCount)
// and surfaces its Classification/Files in Fixed — so a spec whose only activity
// was a /vector:fix is not reduced to a bare counter in the digest (leak #1).
func TestProjectDecodesSpecFixed(t *testing.T) {
	events := []state.Event{
		fixedEvent(t, "alpha", ts(24, 10), "code-only", []string{"a.go", "b.go"}),
	}
	proj := Project(events, ts(24, 0), openWindow)

	if len(proj.PerSpec) != 1 {
		t.Fatalf("perSpec len = %d, want 1", len(proj.PerSpec))
	}
	alpha := proj.PerSpec[0]
	if alpha.ChangeCount != 1 {
		t.Errorf("changeCount = %d, want 1", alpha.ChangeCount)
	}
	if len(alpha.Fixed) != 1 {
		t.Fatalf("fixed = %+v, want one entry", alpha.Fixed)
	}
	if alpha.Fixed[0].Classification != "code-only" || len(alpha.Fixed[0].Files) != 2 {
		t.Errorf("fixed[0] = %+v, want code-only with 2 files", alpha.Fixed[0])
	}
	// spec.fixed contributes no work.logged entry (it is its own field).
	if len(alpha.Work) != 0 {
		t.Errorf("work = %+v, want empty (spec.fixed is not work.logged)", alpha.Work)
	}
}

// TestTimelineDecodesSpecFixed asserts Timeline flattens a spec.fixed event with
// its type and the file/classification detail.
func TestTimelineDecodesSpecFixed(t *testing.T) {
	events := []state.Event{
		fixedEvent(t, "alpha", ts(24, 10), "spec+code", []string{"a.go"}),
	}
	tl := Timeline(events, "alpha", ts(24, 0), openWindow)
	if len(tl) != 1 {
		t.Fatalf("timeline len = %d, want 1", len(tl))
	}
	if tl[0].Type != "spec.fixed" {
		t.Errorf("type = %q, want spec.fixed", tl[0].Type)
	}
	if tl[0].Classification != "spec+code" || len(tl[0].FilesTouched) != 1 {
		t.Errorf("entry = %+v, want spec+code with 1 file", tl[0])
	}
}

// TestProjectHalfOpenBoundary asserts the window is [from, to): an event at from
// is included, one strictly before to is included, one exactly at to is excluded,
// and one after to is excluded — so a boundary event is neither dropped nor
// double-counted across adjacent periods (leak #2).
func TestProjectHalfOpenBoundary(t *testing.T) {
	from := ts(24, 0)
	to := ts(24, 12)
	events := []state.Event{
		statusEvent(t, "alpha", from, state.StatusOpen, state.StatusInProgress),         // == from → included
		statusEvent(t, "alpha", ts(24, 11), state.StatusInProgress, state.StatusReview), // before to → included
		statusEvent(t, "alpha", to, state.StatusReview, state.StatusClosed),             // == to → excluded
		statusEvent(t, "alpha", ts(24, 13), state.StatusClosed, state.StatusOpen),       // after to → excluded
	}
	proj := Project(events, from, to)
	if len(proj.PerSpec) != 1 {
		t.Fatalf("perSpec len = %d, want 1", len(proj.PerSpec))
	}
	if got := proj.PerSpec[0].ChangeCount; got != 2 {
		t.Errorf("changeCount = %d, want 2 (from and before-to only)", got)
	}
	if !proj.Until.Equal(to.UTC()) {
		t.Errorf("until = %v, want %v", proj.Until, to.UTC())
	}

	// The event at exactly `to` lands in the next period, whose from == to. The
	// next window stops before ts(24,13) to isolate the single boundary event.
	next := Project(events, to, ts(24, 13))
	if got := next.PerSpec[0].ChangeCount; got != 1 {
		t.Errorf("next-period changeCount = %d, want 1 (the boundary event)", got)
	}
}
