package state

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// seedNeedsAttention creates a spec and walks it to needs-attention the way the
// reported repro did (open → in-progress → needs-attention).
func seedNeedsAttention(t *testing.T, store *Store, id string) {
	t.Helper()
	newSpec(t, store, id, StatusOpen, PriorityNormal)
	if _, err := store.ApplySpec(id, id, "tester", fixedNow()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := store.SetStatus(id, StatusNeedsAttention, "blocked on API", "tester", fixedNow()); err != nil {
		t.Fatalf("needs-attention: %v", err)
	}
}

// TestNeedsAttentionTransitionsRepro reproduces the reported bug: closing a
// needs-attention spec failed without guidance, then `spec status X open`
// answered "spec is already open" although the spec was needs-attention.
func TestNeedsAttentionTransitionsRepro(t *testing.T) {
	store := openStore(t)
	seedNeedsAttention(t, store, "feat")

	_, closeErr := store.CloseSpec("feat", "tester", fixedNow())
	var illegal *IllegalTransitionError
	if !errors.As(closeErr, &illegal) {
		t.Fatalf("close from needs-attention err = %v, want *IllegalTransitionError", closeErr)
	}
	if !errors.Is(closeErr, ErrConflict) {
		t.Error("an illegal transition must classify as ErrConflict")
	}
	if want := []Status{StatusInProgress, StatusReview}; !reflect.DeepEqual(illegal.Legal, want) {
		t.Errorf("legal targets = %v, want %v", illegal.Legal, want)
	}
	for _, fragment := range []string{"in-progress", "review", "vector spec status feat in-progress"} {
		if !strings.Contains(closeErr.Error(), fragment) {
			t.Errorf("close error %q does not mention %q", closeErr, fragment)
		}
	}

	_, openErr := store.SetStatus("feat", StatusOpen, "", "tester", fixedNow())
	if openErr == nil || strings.Contains(openErr.Error(), "already open") {
		t.Fatalf("status open on a needs-attention spec err = %v, must not claim it is already open", openErr)
	}
	if !errors.As(openErr, &illegal) || illegal.From != StatusNeedsAttention || illegal.To != StatusOpen {
		t.Errorf("status open err = %v, want illegal needs-attention → open", openErr)
	}

	for name, attempt := range map[string]func() error{
		"status closed":  func() error { _, err := store.SetStatus("feat", StatusClosed, "", "tester", fixedNow()); return err },
		"status archive": func() error { _, err := store.SetStatus("feat", StatusArchived, "", "tester", fixedNow()); return err },
		"archive":        func() error { _, err := store.ArchiveSpec("feat", "tester", fixedNow()); return err },
	} {
		if err := attempt(); !errors.As(err, &illegal) || !strings.Contains(err.Error(), "in-progress") {
			t.Errorf("%s: err = %v, want illegal transition listing the legal targets", name, err)
		}
	}

	// Nothing above may have written: the spec is still needs-attention with its flag.
	spec, err := store.ReadSpec("feat")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Status != StatusNeedsAttention || spec.Flag == nil {
		t.Errorf("spec after rejected moves = %q flag=%v, want needs-attention with flag", spec.Status, spec.Flag)
	}

	// The documented way out works.
	if _, err := store.SetStatus("feat", StatusInProgress, "", "tester", fixedNow()); err != nil {
		t.Fatalf("resolve to in-progress: %v", err)
	}
	if _, err := store.CloseSpec("feat", "tester", fixedNow()); err != nil {
		t.Fatalf("close after resolving: %v", err)
	}
}

func TestSetStatusAlreadyAndDedicatedWriters(t *testing.T) {
	store := openStore(t)
	newSpec(t, store, "feat", StatusOpen, PriorityNormal)

	if _, err := store.SetStatus("feat", StatusOpen, "", "tester", fixedNow()); err == nil || !strings.Contains(err.Error(), "already") {
		t.Errorf("open → open err = %v, want an 'already' error", err)
	}
	// A legal close through the generic command points to the dedicated verb.
	if _, err := store.SetStatus("feat", StatusClosed, "", "tester", fixedNow()); err == nil || !strings.Contains(err.Error(), "vector spec close feat") {
		t.Errorf("status closed err = %v, want a pointer to `vector spec close feat`", err)
	}
	if _, err := store.SetStatus("feat", StatusReview, "", "tester", fixedNow()); !errors.As(err, new(*IllegalTransitionError)) || !strings.Contains(err.Error(), "vector spec apply feat") {
		t.Errorf("open → review err = %v, want illegal with `vector spec apply feat` hint", err)
	}
}

// TestLegalTargetsDerivedFromTable pins LegalTargets to the transition table.
func TestLegalTargetsDerivedFromTable(t *testing.T) {
	for _, from := range lifecycleOrder {
		got := LegalTargets(from)
		want := make([]Status, 0)
		for _, to := range lifecycleOrder {
			if CanTransition(from, to) {
				want = append(want, to)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LegalTargets(%s) = %v, want %v", from, got, want)
		}
	}
	err := &IllegalTransitionError{ID: "x", From: StatusArchived, To: StatusOpen, Legal: LegalTargets(StatusArchived)}
	if !strings.Contains(err.Error(), "final") {
		t.Errorf("archived error = %q, want it to say the status is final", err)
	}
}

func TestCloseResolutionPersistsThroughArchive(t *testing.T) {
	store := openStore(t)
	newSpec(t, store, "feat", StatusOpen, PriorityNormal)

	if _, err := store.CloseSpecWith("feat", Resolution("bogus"), "", "tester", fixedNow()); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid resolution err = %v, want ErrInvalidInput", err)
	}
	if spec, _ := store.ReadSpec("feat"); spec.Status != StatusOpen {
		t.Fatalf("an invalid resolution must not write (status %q)", spec.Status)
	}

	closed, err := store.CloseSpecWith("feat", ResolutionDuplicate, "  dup of login-v2 ", "tester", fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if closed.Resolution != ResolutionDuplicate || closed.ResolutionNote != "dup of login-v2" {
		t.Errorf("closed resolution = %q/%q", closed.Resolution, closed.ResolutionNote)
	}
	archived, err := store.ArchiveSpec("feat", "tester", fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if archived.Resolution != ResolutionDuplicate || archived.ResolutionNote != "dup of login-v2" {
		t.Errorf("archive must keep the close-time resolution, got %q/%q", archived.Resolution, archived.ResolutionNote)
	}
	onDisk, _ := store.ReadSpec("feat")
	if onDisk.Resolution != ResolutionDuplicate {
		t.Errorf("persisted resolution = %q", onDisk.Resolution)
	}

	events, _ := store.ReadEvents()
	var closedData ResolutionData
	for _, event := range events {
		if event.Type == EvtSpecClosed {
			if err := json.Unmarshal(event.Data, &closedData); err != nil {
				t.Fatal(err)
			}
		}
	}
	if closedData.Resolution != ResolutionDuplicate || closedData.Note != "dup of login-v2" {
		t.Errorf("spec.closed data = %+v", closedData)
	}
}

func TestCloseDefaultsToDoneAndArchiveOverride(t *testing.T) {
	store := openStore(t)
	newSpec(t, store, "feat", StatusOpen, PriorityNormal)
	closed, err := store.CloseSpec("feat", "tester", fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if closed.Resolution != ResolutionDone || closed.ResolutionNote != "" {
		t.Errorf("default resolution = %q/%q, want done", closed.Resolution, closed.ResolutionNote)
	}
	if _, err := store.ArchiveSpecWith("feat", "", "a note", "tester", fixedNow()); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("note without resolution err = %v, want ErrInvalidInput", err)
	}
	archived, err := store.ArchiveSpecWith("feat", ResolutionSuperseded, "by v2", "tester", fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if archived.Resolution != ResolutionSuperseded || archived.ResolutionNote != "by v2" {
		t.Errorf("archive override = %q/%q", archived.Resolution, archived.ResolutionNote)
	}
}

func TestEffectiveResolutionLegacyRule(t *testing.T) {
	cases := []struct {
		spec SpecState
		want Resolution
	}{
		{SpecState{Status: StatusOpen}, ""},
		{SpecState{Status: StatusClosed}, ResolutionDone},
		{SpecState{Status: StatusArchived}, ""},
		{SpecState{Status: StatusArchived, Resolution: ResolutionDone}, ResolutionDone},
		{SpecState{Status: StatusClosed, Resolution: ResolutionObsolete}, ResolutionObsolete},
	}
	for _, tc := range cases {
		if got := tc.spec.EffectiveResolution(); got != tc.want {
			t.Errorf("%s/%q: EffectiveResolution = %q, want %q", tc.spec.Status, tc.spec.Resolution, got, tc.want)
		}
	}
}

func TestEpicOrderSortingAndValidation(t *testing.T) {
	store := openStore(t)
	createEpic(t, store, CreateEpicParams{Title: "Zeta"})
	createEpic(t, store, CreateEpicParams{Title: "alpha"})
	createEpic(t, store, CreateEpicParams{Title: "Web", Order: 2})
	createEpic(t, store, CreateEpicParams{Title: "App Mobile", Order: 1})

	if _, err := store.CreateEpic(CreateEpicParams{Title: "Bad", Order: -1, Now: fixedNow()}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("negative order err = %v, want ErrInvalidInput", err)
	}
	ids := func() string {
		epics, err := store.ListEpics()
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(epics))
		for _, epic := range epics {
			out = append(out, epic.ID)
		}
		return strings.Join(out, ",")
	}
	if got, want := ids(), "app-mobile,web,alpha,zeta"; got != want {
		t.Errorf("order = %s, want %s (ordered first, then unordered by title)", got, want)
	}

	zero, three := 0, 3
	if _, changed, err := store.UpdateEpic("zeta", UpdateEpicParams{Order: &three}, "tester", fixedNow()); err != nil || !changed {
		t.Fatalf("set order: changed=%v err=%v", changed, err)
	}
	if _, changed, err := store.UpdateEpic("app-mobile", UpdateEpicParams{Order: &zero}, "tester", fixedNow()); err != nil || !changed {
		t.Fatalf("unset order: changed=%v err=%v", changed, err)
	}
	if got, want := ids(), "web,zeta,alpha,app-mobile"; got != want {
		t.Errorf("order after update = %s, want %s", got, want)
	}
	if _, changed, _ := store.UpdateEpic("web", UpdateEpicParams{Order: new(int)}, "tester", fixedNow()); !changed {
		t.Error("unsetting a set order must report a change")
	}
}

func TestSelectNextPrecedence(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	epics := []*Epic{
		{ID: "first", Title: "First", Order: 1},
		{ID: "second", Title: "Second", Order: 2},
		{ID: "loose", Title: "Loose"},
		{ID: "hot", Title: "Hot", Focus: true},
	}
	cases := []struct {
		name  string
		specs []*SpecState
		want  string
	}{
		{"status tier beats focus and epic order", []*SpecState{
			{ID: "wip", Status: StatusInProgress, Priority: PriorityLow},
			{ID: "focused-open", Status: StatusOpen, Priority: PriorityUrgent, Focus: true, Epic: "first"},
		}, "wip"},
		{"own focus beats epic order", []*SpecState{
			{ID: "ordered", Status: StatusOpen, Priority: PriorityUrgent, Epic: "first"},
			{ID: "focused", Status: StatusOpen, Priority: PriorityLow, Focus: true},
		}, "focused"},
		{"inherited focus beats epic order", []*SpecState{
			{ID: "ordered", Status: StatusOpen, Priority: PriorityUrgent, Epic: "first"},
			{ID: "inherits", Status: StatusOpen, Priority: PriorityLow, Epic: "hot"},
		}, "inherits"},
		{"epic order beats priority", []*SpecState{
			{ID: "second-urgent", Status: StatusOpen, Priority: PriorityUrgent, Epic: "second"},
			{ID: "first-low", Status: StatusOpen, Priority: PriorityLow, Epic: "first"},
		}, "first-low"},
		{"ordered epic beats no epic and unordered epic", []*SpecState{
			{ID: "no-epic", Status: StatusOpen, Priority: PriorityUrgent},
			{ID: "loose", Status: StatusOpen, Priority: PriorityUrgent, Epic: "loose"},
			{ID: "second", Status: StatusOpen, Priority: PriorityLow, Epic: "second"},
		}, "second"},
		{"no epic and unordered epic tie, priority decides", []*SpecState{
			{ID: "loose-low", Status: StatusOpen, Priority: PriorityLow, Epic: "loose"},
			{ID: "no-epic-high", Status: StatusOpen, Priority: PriorityHigh},
		}, "no-epic-high"},
		{"recency breaks the final tie", []*SpecState{
			{ID: "older", Status: StatusOpen, Priority: PriorityNormal, UpdatedAt: now},
			{ID: "newer", Status: StatusOpen, Priority: PriorityNormal, UpdatedAt: now.Add(time.Hour)},
		}, "newer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if pick := SelectNext(tc.specs, epics); pick == nil || pick.ID != tc.want {
				t.Fatalf("SelectNext = %v, want %s", pick, tc.want)
			}
		})
	}
}

func TestEpicIndexInheritedFocus(t *testing.T) {
	index := NewEpicIndex([]*Epic{{ID: "hot", Focus: true}, {ID: "cold"}})
	cases := []struct {
		spec          SpecState
		inherits, eff bool
	}{
		{SpecState{Status: StatusOpen, Epic: "hot"}, true, true},
		{SpecState{Status: StatusOpen, Epic: "hot", Focus: true}, false, true}, // own focus wins, not "inherited"
		{SpecState{Status: StatusClosed, Epic: "hot"}, false, false},           // terminal never inherits
		{SpecState{Status: StatusArchived, Epic: "hot"}, false, false},
		{SpecState{Status: StatusOpen, Epic: "cold"}, false, false},
		{SpecState{Status: StatusOpen, Epic: "cold", Focus: true}, false, true},
		{SpecState{Status: StatusOpen}, false, false},
		{SpecState{Status: StatusOpen, Epic: "ghost"}, false, false},
	}
	for _, tc := range cases {
		spec := tc.spec
		if got := index.InheritsFocus(&spec); got != tc.inherits {
			t.Errorf("%+v InheritsFocus = %v, want %v", spec, got, tc.inherits)
		}
		if got := index.EffectiveFocus(&spec); got != tc.eff {
			t.Errorf("%+v EffectiveFocus = %v, want %v", spec, got, tc.eff)
		}
	}
}

func TestSetEpicFocusIsIdempotentAndNeverTouchesSpecs(t *testing.T) {
	store := openStore(t)
	createEpic(t, store, CreateEpicParams{Title: "App Mobile"})
	newSpec(t, store, "login", StatusOpen, PriorityNormal)
	if _, err := store.AssignEpic("login", "app-mobile", "tester", fixedNow()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(store.statePath("login"))

	if changed, err := store.SetEpicFocus("app-mobile", true, "tester", fixedNow()); err != nil || !changed {
		t.Fatalf("focus: changed=%v err=%v", changed, err)
	}
	if changed, err := store.SetEpicFocus("app-mobile", true, "tester", fixedNow()); err != nil || changed {
		t.Fatalf("re-focus must be a no-op: changed=%v err=%v", changed, err)
	}
	epic, _ := store.GetEpic("app-mobile")
	if !epic.Focus || epic.FocusedAt == nil {
		t.Errorf("epic focus not persisted: %+v", epic)
	}
	after, _ := os.ReadFile(store.statePath("login"))
	if string(before) != string(after) {
		t.Error("focusing an epic must not rewrite its specs (focus is derived)")
	}
	if changed, err := store.SetEpicFocus("app-mobile", false, "tester", fixedNow()); err != nil || !changed {
		t.Fatalf("unfocus: changed=%v err=%v", changed, err)
	}
	if epic, _ := store.GetEpic("app-mobile"); epic.Focus || epic.FocusedAt != nil {
		t.Errorf("unfocus not persisted: %+v", epic)
	}
	if _, err := store.SetEpicFocus("ghost", true, "tester", fixedNow()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("unknown epic err = %v, want not-exist", err)
	}
	events, _ := store.ReadEvents()
	counts := map[EventType]int{}
	for _, event := range events {
		counts[event.Type]++
	}
	if counts[EvtEpicFocused] != 1 || counts[EvtEpicUnfocused] != 1 {
		t.Errorf("focus events = %v, want one focused + one unfocused", counts)
	}
}

func TestAssignEpicBulk(t *testing.T) {
	store := openStore(t)
	createEpic(t, store, CreateEpicParams{Title: "App Mobile"})
	createEpic(t, store, CreateEpicParams{Title: "Web"})
	for _, id := range []string{"a", "b", "c"} {
		newSpec(t, store, id, StatusOpen, PriorityNormal)
	}
	if _, err := store.AssignEpic("c", "web", "tester", fixedNow()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AssignEpic("b", "app-mobile", "tester", fixedNow()); err != nil {
		t.Fatal(err)
	}
	eventsBefore, _ := store.ReadEvents()

	// Any unknown id fails the whole batch before writing anything.
	_, err := store.AssignEpicBulk([]string{"a", "ghost", "Bad Id", "c"}, "app-mobile", "tester", fixedNow())
	if !errors.Is(err, ErrInvalidInput) || !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "Bad Id") {
		t.Fatalf("unknown ids err = %v, want ErrInvalidInput naming ghost and Bad Id", err)
	}
	for id, want := range map[string]string{"a": "", "b": "app-mobile", "c": "web"} {
		if spec, _ := store.ReadSpec(id); spec.Epic != want {
			t.Errorf("%s epic = %q after a failed batch, want %q (no writes)", id, spec.Epic, want)
		}
	}
	if eventsAfter, _ := store.ReadEvents(); len(eventsAfter) != len(eventsBefore) {
		t.Errorf("a failed batch appended %d events", len(eventsAfter)-len(eventsBefore))
	}
	if _, err := store.AssignEpicBulk([]string{"a"}, "nope", "tester", fixedNow()); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("unknown epic err = %v, want ErrInvalidInput", err)
	}

	results, err := store.AssignEpicBulk([]string{"a", "b", "c", "a"}, "app-mobile", "tester", fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	want := []EpicAssignResult{
		{ID: "a", Previous: "", Changed: true},
		{ID: "b", Previous: "app-mobile", Changed: false},
		{ID: "c", Previous: "web", Changed: true},
	}
	if !reflect.DeepEqual(results, want) {
		t.Errorf("results = %+v, want %+v", results, want)
	}

	cleared, err := store.AssignEpicBulk([]string{"a", "b"}, "", "tester", fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared) != 2 || !cleared[0].Changed || !cleared[1].Changed {
		t.Errorf("clear results = %+v", cleared)
	}
	if _, err := store.AssignEpicBulk(nil, "web", "tester", fixedNow()); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("empty batch err = %v, want ErrInvalidInput", err)
	}
}
