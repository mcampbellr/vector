package board

import (
	"encoding/json"
	"io/fs"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

type fakeSource struct {
	specs     []*state.SpecState
	events    []state.Event
	standup   *state.StandupDigest
	summaries map[string]state.SpecSummary
	artifacts map[string][]byte // key: "<spec>/<artifact>"
	artifErr  error             // when set, ReadSpecArtifact returns it
}

func (f fakeSource) ListSpecs() ([]*state.SpecState, error) { return f.specs, nil }
func (f fakeSource) ReadEvents() ([]state.Event, error)     { return f.events, nil }
func (f fakeSource) ReadStandup() (*state.StandupDigest, error) {
	if f.standup == nil {
		return &state.StandupDigest{}, nil
	}
	return f.standup, nil
}
func (f fakeSource) ReadSummary(id string) (*state.SpecSummary, error) {
	if sum, ok := f.summaries[id]; ok {
		return &sum, nil
	}
	return nil, nil
}
func (f fakeSource) ReadSpecArtifact(specID, artifact string) ([]byte, error) {
	if f.artifErr != nil {
		return nil, f.artifErr
	}
	if b, ok := f.artifacts[specID+"/"+artifact]; ok {
		return b, nil
	}
	return nil, fs.ErrNotExist
}

func routedEvent(t *testing.T, specID, model, baseline string, saved, cost float64) state.Event {
	t.Helper()
	return routedEventWithPrecision(t, specID, model, baseline, saved, cost, "")
}

func routedEventWithPrecision(t *testing.T, specID, model, baseline string, saved, cost float64, precision string) state.Event {
	t.Helper()
	data, err := json.Marshal(state.AgentRoutedData{
		Task: "summarize", Model: model, Baseline: baseline,
		TokensIn: 1000, TokensOut: 100, CostUSD: cost, SavedUSD: saved,
		Precision: precision,
	})
	if err != nil {
		t.Fatalf("marshal routed data: %v", err)
	}
	return state.Event{V: 1, Type: state.EvtAgentRouted, SpecID: specID, Data: data}
}

func TestBuildGroupsByStatusAndOrdersByPriority(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{specs: []*state.SpecState{
		{ID: "a", Title: "A", Status: state.StatusOpen, Priority: state.PriorityNormal, UpdatedAt: now},
		{ID: "b", Title: "B", Status: state.StatusOpen, Priority: state.PriorityUrgent, UpdatedAt: now},
		{ID: "c", Title: "C", Status: state.StatusArchived, Priority: state.PriorityLow, UpdatedAt: now},
		{ID: "d", Title: "D", Status: state.StatusReview, Priority: state.PriorityHigh, UpdatedAt: now},
	}}

	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if got := len(b.Columns); got != len(columnOrder) {
		t.Fatalf("columns = %d, want %d", got, len(columnOrder))
	}
	open := columnByStatus(t, b, "open")
	if open.Count != 2 {
		t.Fatalf("open count = %d, want 2", open.Count)
	}
	if open.Cards[0].ID != "b" {
		t.Errorf("urgent card should sort first, got %q", open.Cards[0].ID)
	}
	if b.Totals.Specs != 4 {
		t.Errorf("totals.specs = %d, want 4 (archived counted in totals)", b.Totals.Specs)
	}
	// Archived must not appear in any column.
	for _, col := range b.Columns {
		for _, card := range col.Cards {
			if card.ID == "c" {
				t.Errorf("archived spec leaked into column %q", col.Status)
			}
		}
	}
}

func TestBuildRollsUpTokenSavings(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{
		specs: []*state.SpecState{
			{ID: "a", Title: "A", Status: state.StatusInProgress, Priority: state.PriorityHigh, UpdatedAt: now},
		},
		events: []state.Event{
			routedEvent(t, "a", "haiku", "opus", 0.31, 0.02),
			routedEvent(t, "a", "haiku", "opus", 0.10, 0.01),
			routedEvent(t, "", "sonnet", "opus", 0.05, 0.04), // no spec attribution
		},
	}

	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	s := b.TokenSavings
	if s.Routes != 3 {
		t.Errorf("routes = %d, want 3", s.Routes)
	}
	if !almostEqual(s.TotalSavedUSD, 0.46) {
		t.Errorf("totalSaved = %v, want 0.46", s.TotalSavedUSD)
	}
	if !almostEqual(s.BaselineUSD, s.TotalSpentUSD+s.TotalSavedUSD) {
		t.Errorf("baseline should equal spent+saved, got %v", s.BaselineUSD)
	}
	if len(s.ByModel) != 2 {
		t.Fatalf("byModel groups = %d, want 2", len(s.ByModel))
	}
	if s.ByModel[0].Model != "haiku" {
		t.Errorf("top saver should be haiku, got %q", s.ByModel[0].Model)
	}

	card := columnByStatus(t, b, "in-progress").Cards[0]
	if card.Routes != 2 {
		t.Errorf("card routes = %d, want 2", card.Routes)
	}
	if !almostEqual(card.SavedUSD, 0.41) {
		t.Errorf("card savedUsd = %v, want 0.41", card.SavedUSD)
	}

	// Per-spec token totals: spec "a" has 2 routed events (1000 in / 100 out each).
	if card.TokensIn != 2000 {
		t.Errorf("card tokensIn = %d, want 2000", card.TokensIn)
	}
	if card.TokensOut != 200 {
		t.Errorf("card tokensOut = %d, want 200", card.TokensOut)
	}
	if len(card.ByModel) != 1 {
		t.Fatalf("card byModel groups = %d, want 1", len(card.ByModel))
	}
	cm := card.ByModel[0]
	if cm.Model != "haiku" || cm.Baseline != "opus" {
		t.Errorf("card byModel pair = %s→%s, want haiku→opus", cm.Model, cm.Baseline)
	}
	if cm.Routes != 2 || cm.TokensIn != 2000 || cm.TokensOut != 200 {
		t.Errorf("card byModel haiku→opus = {routes:%d, in:%d, out:%d}, want {2, 2000, 200}", cm.Routes, cm.TokensIn, cm.TokensOut)
	}

	// Global per-model breakdown must also carry token counts (not just routes/USD).
	if s.ByModel[0].TokensIn != 2000 || s.ByModel[0].TokensOut != 200 {
		t.Errorf("global byModel[0] tokens = {in:%d, out:%d}, want {2000, 200}", s.ByModel[0].TokensIn, s.ByModel[0].TokensOut)
	}
}

func TestBuildProjectsRelatedTo(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{specs: []*state.SpecState{{
		ID: "fix-loop", Title: "Fix loop", Status: state.StatusOpen, Priority: state.PriorityNormal, UpdatedAt: now,
		RelatedTo: []state.RelatedItem{
			{Kind: state.RelatedSpec, Ref: "add-login", Source: state.RelatedBlame},
			{Kind: state.RelatedTicket, Ref: "jira:ACME-7", Source: state.RelatedManual},
		},
	}}}

	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	card := columnByStatus(t, b, "open").Cards[0]
	if len(card.RelatedTo) != 2 {
		t.Fatalf("card relatedTo len = %d, want 2: %+v", len(card.RelatedTo), card.RelatedTo)
	}
	if card.RelatedTo[0].Kind != "spec" || card.RelatedTo[0].Ref != "add-login" || card.RelatedTo[0].Source != "blame" {
		t.Errorf("unexpected first relation projection: %+v", card.RelatedTo[0])
	}

	// A relation-less spec must omit relatedTo from the JSON contract.
	plain := fakeSource{specs: []*state.SpecState{{ID: "p", Title: "P", Status: state.StatusOpen, Priority: state.PriorityNormal, UpdatedAt: now}}}
	pb, _ := Build(plain, "demo", now)
	raw, err := json.Marshal(columnByStatus(t, pb, "open").Cards[0])
	if err != nil {
		t.Fatal(err)
	}
	if containsField(raw, "relatedTo") {
		t.Errorf("relatedTo present for a relation-less card: %s", raw)
	}
}

// TestBuildProjectsQuickWin verifies the Card projection carries quickWin and that
// a non-quick-win card omits the field from the JSON contract (omitempty).
func TestBuildProjectsQuickWin(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{specs: []*state.SpecState{{
		ID: "extract-timeouts", Title: "Extract timeouts", Status: state.StatusInProgress,
		Priority: state.PriorityNormal, QuickWin: true, UpdatedAt: now,
	}}}

	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	card := columnByStatus(t, b, "in-progress").Cards[0]
	if !card.QuickWin {
		t.Fatalf("card.QuickWin = false, want true: %+v", card)
	}

	// A non-quick-win card must omit quickWin from the JSON contract.
	plain := fakeSource{specs: []*state.SpecState{{ID: "p", Title: "P", Status: state.StatusOpen, Priority: state.PriorityNormal, UpdatedAt: now}}}
	pb, _ := Build(plain, "demo", now)
	raw, err := json.Marshal(columnByStatus(t, pb, "open").Cards[0])
	if err != nil {
		t.Fatal(err)
	}
	if containsField(raw, "quickWin") {
		t.Errorf("quickWin present for a non-quick-win card: %s", raw)
	}
}

// TestBuildProjectsAttention verifies toCard flattens a full structured Attention
// into the four card fields, and that a legacy Attention (only Reason/Since/Source)
// projects only attentionReason with the structured fields omitted from the JSON.
func TestBuildProjectsAttention(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{specs: []*state.SpecState{{
		ID: "blocked", Title: "Blocked", Status: state.StatusNeedsAttention, Priority: state.PriorityNormal,
		Flag: &state.Attention{
			Reason:   "Zoho api_names pending creds",
			Category: state.AttentionDependency,
			Summary:  "Zoho api_names pending creds",
			Detail:   "PR #367 open; fill `TODO(MH-1582)`",
			Since:    now,
			Source:   "command",
		},
		UpdatedAt: now,
	}}}
	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	card := columnByStatus(t, b, "needs-attention").Cards[0]
	if card.AttentionCategory != "dependency" || card.AttentionSummary != "Zoho api_names pending creds" {
		t.Errorf("category/summary = %q/%q, want dependency/…", card.AttentionCategory, card.AttentionSummary)
	}
	if card.AttentionDetail != "PR #367 open; fill `TODO(MH-1582)`" {
		t.Errorf("detail = %q, want the markdown detail", card.AttentionDetail)
	}
	if card.AttentionReason != "Zoho api_names pending creds" {
		t.Errorf("reason = %q, want it kept for legacy readers", card.AttentionReason)
	}

	// A legacy Attention (pre-migration on disk: only Reason/Since/Source) projects
	// just attentionReason; the structured fields stay omitted from the contract.
	legacy := fakeSource{specs: []*state.SpecState{{
		ID: "old", Title: "Old", Status: state.StatusNeedsAttention, Priority: state.PriorityNormal,
		Flag:      &state.Attention{Reason: "blocked on DTO", Since: now, Source: "command"},
		UpdatedAt: now,
	}}}
	lb, _ := Build(legacy, "demo", now)
	lcard := columnByStatus(t, lb, "needs-attention").Cards[0]
	if lcard.AttentionReason != "blocked on DTO" {
		t.Errorf("legacy reason = %q, want blocked on DTO", lcard.AttentionReason)
	}
	raw, err := json.Marshal(lcard)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"attentionCategory", "attentionSummary", "attentionDetail"} {
		if containsField(raw, field) {
			t.Errorf("%s present for a legacy attention card: %s", field, raw)
		}
	}
}

// TestBuildProjectsSketches verifies the Card projection carries sketches when the
// spec has them, and omits the field (omitempty) when it has none.
func TestBuildProjectsSketches(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{specs: []*state.SpecState{{
		ID: "add-ui", Title: "Add UI", Status: state.StatusInProgress, Priority: state.PriorityNormal,
		Sketches: []state.SketchRef{{Name: "board.excalidraw", CreatedAt: now}}, UpdatedAt: now,
	}}}
	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	card := columnByStatus(t, b, "in-progress").Cards[0]
	if len(card.Sketches) != 1 || card.Sketches[0].Name != "board.excalidraw" {
		t.Fatalf("card.Sketches = %+v, want one board.excalidraw", card.Sketches)
	}

	// A card without sketches must omit the field from the JSON contract.
	plain := fakeSource{specs: []*state.SpecState{{ID: "p", Title: "P", Status: state.StatusOpen, Priority: state.PriorityNormal, UpdatedAt: now}}}
	pb, _ := Build(plain, "demo", now)
	raw, err := json.Marshal(columnByStatus(t, pb, "open").Cards[0])
	if err != nil {
		t.Fatal(err)
	}
	if containsField(raw, "sketches") {
		t.Errorf("sketches present for a card with none: %s", raw)
	}
}

func containsField(b []byte, field string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return false
	}
	_, ok := m[field]
	return ok
}

func columnByStatus(t *testing.T, b *Board, status string) Column {
	t.Helper()
	for _, c := range b.Columns {
		if c.Status == status {
			return c
		}
	}
	t.Fatalf("column %q not found", status)
	return Column{}
}

func almostEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

// TestRollupSavings_AllActual: every event has precision "actual" → rollup is "actual".
func TestRollupSavings_AllActual(t *testing.T) {
	events := []state.Event{
		routedEventWithPrecision(t, "a", "haiku", "opus", 0.10, 0.01, "actual"),
		routedEventWithPrecision(t, "b", "haiku", "opus", 0.20, 0.02, "actual"),
	}
	s, _ := rollupSavings(events)
	if s.Precision != "actual" {
		t.Errorf("Precision = %q, want \"actual\"", s.Precision)
	}
}

// TestRollupSavings_AllEstimated: every event has precision "estimated" → rollup is "estimated".
func TestRollupSavings_AllEstimated(t *testing.T) {
	events := []state.Event{
		routedEventWithPrecision(t, "a", "haiku", "opus", 0.10, 0.01, "estimated"),
		routedEventWithPrecision(t, "b", "haiku", "opus", 0.20, 0.02, "estimated"),
	}
	s, _ := rollupSavings(events)
	if s.Precision != "estimated" {
		t.Errorf("Precision = %q, want \"estimated\"", s.Precision)
	}
}

// TestRollupSavings_Mixed: one "actual" + one "estimated" → worst-case "estimated".
func TestRollupSavings_Mixed(t *testing.T) {
	events := []state.Event{
		routedEventWithPrecision(t, "a", "haiku", "opus", 0.10, 0.01, "actual"),
		routedEventWithPrecision(t, "b", "haiku", "opus", 0.20, 0.02, "estimated"),
	}
	s, _ := rollupSavings(events)
	if s.Precision != "estimated" {
		t.Errorf("Precision = %q, want \"estimated\" (worst-case)", s.Precision)
	}
}

// TestRollupSavings_OldEvents: events with empty Precision field (pre-feature) → "estimated".
func TestRollupSavings_OldEvents(t *testing.T) {
	events := []state.Event{
		routedEvent(t, "a", "haiku", "opus", 0.10, 0.01), // no Precision field
	}
	s, _ := rollupSavings(events)
	if s.Precision != "estimated" {
		t.Errorf("Precision = %q, want \"estimated\" (old event with absent field)", s.Precision)
	}
}

// TestRollupSavings_Empty: no routed events → precision is "" (no badge).
func TestRollupSavings_Empty(t *testing.T) {
	s, _ := rollupSavings(nil)
	if s.Precision != "" {
		t.Errorf("Precision = %q, want \"\" (empty meter has no precision)", s.Precision)
	}
	if s.Routes != 0 {
		t.Errorf("Routes = %d, want 0", s.Routes)
	}
}
