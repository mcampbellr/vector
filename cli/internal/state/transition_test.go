package state

import (
	"testing"
	"time"
)

func newSpec(t *testing.T, store *Store, id string, status Status, priority Priority) {
	t.Helper()
	if _, err := store.CreateSpec(CreateSpecParams{
		ID: id, Title: id, Status: status, Priority: priority, Actor: "tester", Now: time.Now(),
	}); err != nil {
		t.Fatalf("create spec %q: %v", id, err)
	}
}

func TestApplyAndCloseAndArchiveHappyPath(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusOpen, PriorityNormal)
	now := time.Now()

	applied, err := store.ApplySpec("feat", "feat", "tester", now)
	if err != nil {
		t.Fatalf("ApplySpec: %v", err)
	}
	if applied.Status != StatusInProgress {
		t.Fatalf("status = %q, want in-progress", applied.Status)
	}
	if applied.StartedAt == nil {
		t.Error("ApplySpec should stamp StartedAt")
	}

	reviewed, err := store.SetStatus("feat", StatusReview, "", "tester", now)
	if err != nil {
		t.Fatalf("SetStatus review: %v", err)
	}
	if reviewed.ReviewAt == nil {
		t.Error("review should stamp ReviewAt")
	}

	closed, err := store.CloseSpec("feat", "tester", now)
	if err != nil {
		t.Fatalf("CloseSpec: %v", err)
	}
	if closed.Status != StatusClosed || closed.ClosedAt == nil {
		t.Errorf("close: status=%q closedAt=%v", closed.Status, closed.ClosedAt)
	}

	archived, err := store.ArchiveSpec("feat", "tester", now)
	if err != nil {
		t.Fatalf("ArchiveSpec: %v", err)
	}
	if archived.Status != StatusArchived || archived.ArchivedAt == nil {
		t.Errorf("archive: status=%q archivedAt=%v", archived.Status, archived.ArchivedAt)
	}

	// spec.applied + spec.closed + spec.archived + 4 status.changed = 7 events,
	// plus the spec.created from CreateSpec.
	events, err := store.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[EventType]int{}
	for _, e := range events {
		counts[e.Type]++
	}
	if counts[EvtStatusChanged] != 4 {
		t.Errorf("status.changed events = %d, want 4", counts[EvtStatusChanged])
	}
	if counts[EvtSpecApplied] != 1 || counts[EvtSpecClosed] != 1 || counts[EvtSpecArchived] != 1 {
		t.Errorf("domain events: applied=%d closed=%d archived=%d", counts[EvtSpecApplied], counts[EvtSpecClosed], counts[EvtSpecArchived])
	}
}

func TestApplyConsumesLegacyDraftInOneAction(t *testing.T) {
	store, _ := Open(t.TempDir())
	makeLegacyDraft(t, store, "legacy")

	applied, err := store.ApplySpec("legacy", "legacy", "tester", time.Now())
	if err != nil {
		t.Fatalf("ApplySpec legacy: %v", err)
	}
	if applied.Status != StatusInProgress || applied.OpenSpec == nil || applied.OpenSpec.Change != "legacy" {
		t.Fatalf("legacy apply did not formalize and start: %+v", applied)
	}
	events, err := store.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	var proposed, appliedEvent bool
	for _, event := range events {
		proposed = proposed || event.Type == EvtSpecProposed
		appliedEvent = appliedEvent || event.Type == EvtSpecApplied
	}
	if !proposed || !appliedEvent {
		t.Fatalf("legacy apply events: proposed=%t applied=%t", proposed, appliedEvent)
	}
}

func TestIllegalTransitionRejected(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusOpen, PriorityNormal)

	// open → review is not in the machine (must apply first).
	if _, err := store.SetStatus("feat", StatusReview, "", "tester", time.Now()); err == nil {
		t.Fatal("expected illegal transition open → review to fail")
	}
	// open → archived is illegal.
	if _, err := store.ArchiveSpec("feat", "tester", time.Now()); err == nil {
		t.Fatal("expected illegal transition open → archived to fail")
	}
}

func TestSetStatusRoutesAwayFromDedicatedCommands(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusInProgress, PriorityNormal)
	for _, to := range []Status{StatusOpen, StatusClosed, StatusArchived} {
		if _, err := store.SetStatus("feat", to, "", "tester", time.Now()); err == nil {
			t.Errorf("SetStatus to %q should be rejected (dedicated command exists)", to)
		}
	}
}

func TestNeedsAttentionSetsAndClearsFlag(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusInProgress, PriorityNormal)
	now := time.Now()

	if _, err := store.SetStatus("feat", StatusNeedsAttention, "", "tester", now); err == nil {
		t.Fatal("needs-attention without a reason should fail")
	}
	flagged, err := store.SetStatus("feat", StatusNeedsAttention, "blocked on DTO", "tester", now)
	if err != nil {
		t.Fatalf("SetStatus needs-attention: %v", err)
	}
	if flagged.Flag == nil || flagged.Flag.Reason != "blocked on DTO" {
		t.Fatalf("attention flag not set: %+v", flagged.Flag)
	}
	resolved, err := store.SetStatus("feat", StatusInProgress, "", "tester", now)
	if err != nil {
		t.Fatalf("resolve needs-attention: %v", err)
	}
	if resolved.Flag != nil {
		t.Error("resolving needs-attention should clear the flag")
	}
}

func TestSelectNextRanksByStatusThenPriority(t *testing.T) {
	now := time.Now()
	specs := []*SpecState{
		{ID: "open-high", Status: StatusOpen, Priority: PriorityHigh, UpdatedAt: now},
		{ID: "review", Status: StatusReview, Priority: PriorityLow, UpdatedAt: now},
		{ID: "wip", Status: StatusInProgress, Priority: PriorityLow, UpdatedAt: now},
		{ID: "closed", Status: StatusClosed, Priority: PriorityUrgent, UpdatedAt: now},
	}
	got := SelectNext(specs, nil)
	if got == nil || got.ID != "wip" {
		t.Fatalf("SelectNext = %v, want in-progress 'wip' first", got)
	}

	// With only terminal cards left, nothing is actionable.
	if SelectNext([]*SpecState{{ID: "c", Status: StatusClosed}}, nil) != nil {
		t.Error("SelectNext should return nil when nothing is actionable")
	}
}

// TestSelectNextFocusWithinStatusTier pins the ordering rule: within a status tier
// a focused spec beats any priority, but focus never lifts a spec across tiers
// (continuing started work still wins over a focused open spec).
func TestSelectNextFocusWithinStatusTier(t *testing.T) {
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		specs []*SpecState
		want  string
	}{
		{
			name: "focused low beats unfocused urgent in the same tier",
			specs: []*SpecState{
				{ID: "urgent-open", Status: StatusOpen, Priority: PriorityUrgent, UpdatedAt: base.Add(2)},
				{ID: "focused-low", Status: StatusOpen, Priority: PriorityLow, Focus: true, UpdatedAt: base},
			},
			want: "focused-low",
		},
		{
			name: "status tier still wins over focus",
			specs: []*SpecState{
				{ID: "focused-open", Status: StatusOpen, Priority: PriorityUrgent, Focus: true, UpdatedAt: base.Add(5)},
				{ID: "started", Status: StatusInProgress, Priority: PriorityLow, UpdatedAt: base},
			},
			want: "started",
		},
		{
			name: "among focused specs priority decides",
			specs: []*SpecState{
				{ID: "focused-normal", Status: StatusOpen, Priority: PriorityNormal, Focus: true, UpdatedAt: base.Add(5)},
				{ID: "focused-high", Status: StatusOpen, Priority: PriorityHigh, Focus: true, UpdatedAt: base},
			},
			want: "focused-high",
		},
		{
			name: "same focus and priority falls back to recency",
			specs: []*SpecState{
				{ID: "older", Status: StatusOpen, Priority: PriorityHigh, Focus: true, UpdatedAt: base},
				{ID: "newer", Status: StatusOpen, Priority: PriorityHigh, Focus: true, UpdatedAt: base.Add(1)},
			},
			want: "newer",
		},
		{
			name: "a focused closed spec is not actionable",
			specs: []*SpecState{
				{ID: "closed", Status: StatusClosed, Priority: PriorityUrgent, Focus: true, UpdatedAt: base},
				{ID: "open", Status: StatusOpen, Priority: PriorityLow, UpdatedAt: base},
			},
			want: "open",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pick := SelectNext(tc.specs, nil)
			if pick == nil || pick.ID != tc.want {
				t.Fatalf("SelectNext = %v, want %s", pick, tc.want)
			}
		})
	}
}
