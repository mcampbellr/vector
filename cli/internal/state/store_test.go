package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 6, 23, 14, 0, 0, 0, time.UTC)
}

func TestCreateSpecWritesStateAndEvent(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	spec, err := store.CreateSpec(CreateSpecParams{
		Title: "New checkout flow",
		Repo:  "cdr",
		Body:  "# New checkout flow\n\nraw spec body\n",
		Actor: "tester",
		Now:   fixedNow(),
	})
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}

	// Invariants on the returned state.
	if spec.ID != "new-checkout-flow" {
		t.Errorf("ID = %q, want new-checkout-flow", spec.ID)
	}
	if spec.Status != StatusDraft {
		t.Errorf("Status = %q, want draft (default)", spec.Status)
	}
	if spec.SpecDoc != ".vector/specs/new-checkout-flow/spec.md" {
		t.Errorf("SpecDoc = %q, want .vector fallback path", spec.SpecDoc)
	}
	if spec.Priority != PriorityNormal {
		t.Errorf("Priority = %q, want normal (default)", spec.Priority)
	}
	if !spec.CreatedAt.Equal(fixedNow()) || !spec.UpdatedAt.Equal(fixedNow()) {
		t.Errorf("timestamps not set to Now")
	}
	if spec.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", spec.SchemaVersion, SchemaVersion)
	}

	// state.json round-trips from disk (single source of truth on disk).
	onDisk, err := store.ReadSpec(spec.ID)
	if err != nil {
		t.Fatalf("ReadSpec: %v", err)
	}
	if onDisk.Title != "New checkout flow" || onDisk.Status != StatusDraft {
		t.Errorf("on-disk spec mismatch: %+v", onDisk)
	}

	// spec.md written.
	body, err := os.ReadFile(filepath.Join(root, ".vector", "specs", spec.ID, "spec.md"))
	if err != nil {
		t.Fatalf("read spec.md: %v", err)
	}
	if len(body) == 0 {
		t.Error("spec.md is empty")
	}

	// activity.jsonl has exactly one spec.created event with the expected shape.
	events := readEvents(t, filepath.Join(root, ".vector", "local", "activity.jsonl"))
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Type != EvtSpecCreated || ev.SpecID != spec.ID || ev.Actor != "tester" || ev.V != EventVersion {
		t.Errorf("unexpected event envelope: %+v", ev)
	}
	var data SpecCreatedData
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("decode event data: %v", err)
	}
	if data.Source != "raw" || data.Template != "idea" || data.Title != "New checkout flow" {
		t.Errorf("unexpected event data: %+v", data)
	}
}

func TestCreateSpecRejectsDuplicate(t *testing.T) {
	store, _ := Open(t.TempDir())
	params := CreateSpecParams{Title: "Dup", Actor: "t", Now: fixedNow()}
	if _, err := store.CreateSpec(params); err != nil {
		t.Fatalf("first CreateSpec: %v", err)
	}
	if _, err := store.CreateSpec(params); err == nil {
		t.Fatal("expected error creating duplicate spec, got nil")
	}
}

func TestCreateSpecValidatesInputs(t *testing.T) {
	store, _ := Open(t.TempDir())

	if _, err := store.CreateSpec(CreateSpecParams{Title: "   ", Now: fixedNow()}); err == nil {
		t.Error("expected error for empty-derived id")
	}
	if _, err := store.CreateSpec(CreateSpecParams{ID: "Not Kebab", Now: fixedNow()}); err == nil {
		t.Error("expected error for non-kebab id")
	}
	if _, err := store.CreateSpec(CreateSpecParams{Title: "x", Priority: "huge", Now: fixedNow()}); err == nil {
		t.Error("expected error for invalid priority")
	}
}

func TestCreateSpecWithOpenSpecAndStatus(t *testing.T) {
	store, _ := Open(t.TempDir())
	os := &OpenSpec{Change: "add-auth", Artifacts: ArtifactSet{Proposal: true, Tasks: true}}
	spec, err := store.CreateSpec(CreateSpecParams{
		ID:         "add-auth",
		Title:      "Add auth",
		Status:     StatusReview,
		OpenSpec:   os,
		SpecDocRel: "openspec/changes/add-auth/proposal.md",
		Now:        fixedNow(),
	})
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if spec.Status != StatusReview || spec.OpenSpec == nil || spec.OpenSpec.Change != "add-auth" {
		t.Fatalf("unexpected spec: %+v", spec)
	}
	if spec.SpecDoc != "openspec/changes/add-auth/proposal.md" {
		t.Errorf("SpecDoc = %q (should not fall back to .vector)", spec.SpecDoc)
	}
	if spec.ReviewAt == nil {
		t.Error("ReviewAt should be stamped for a review-status spec")
	}
}

func TestReconcileStatus(t *testing.T) {
	store, _ := Open(t.TempDir())
	os := &OpenSpec{Change: "add-auth", Artifacts: ArtifactSet{Tasks: true}}
	if _, err := store.CreateSpec(CreateSpecParams{ID: "add-auth", Title: "Add auth", Status: StatusOpen, OpenSpec: os, SpecDocRel: "x", Now: fixedNow()}); err != nil {
		t.Fatal(err)
	}

	changed, err := store.ReconcileStatus("add-auth", StatusReview, os, false, "t", fixedNow())
	if err != nil {
		t.Fatalf("ReconcileStatus: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on open→review")
	}
	// idempotent: same status → no change.
	changed, err = store.ReconcileStatus("add-auth", StatusReview, os, false, "t", fixedNow())
	if err != nil {
		t.Fatalf("ReconcileStatus (2): %v", err)
	}
	if changed {
		t.Fatal("expected changed=false when status already matches")
	}

	onDisk, _ := store.ReadSpec("add-auth")
	if onDisk.Status != StatusReview {
		t.Errorf("Status = %q, want review", onDisk.Status)
	}
}

func TestReconcileStatusKeepsTerminalStates(t *testing.T) {
	for _, terminal := range []Status{StatusClosed, StatusArchived} {
		t.Run(string(terminal), func(t *testing.T) {
			store, _ := Open(t.TempDir())
			os := &OpenSpec{Change: "add-auth", Artifacts: ArtifactSet{Tasks: true}}
			if _, err := store.CreateSpec(CreateSpecParams{ID: "add-auth", Title: "Add auth", Status: terminal, OpenSpec: os, SpecDocRel: "x", Now: fixedNow()}); err != nil {
				t.Fatal(err)
			}
			// tasks.md all done → derived status review, but a terminal card must
			// not be pulled back. changed=false, status unchanged.
			changed, err := store.ReconcileStatus("add-auth", StatusReview, os, false, "t", fixedNow())
			if err != nil {
				t.Fatalf("ReconcileStatus: %v", err)
			}
			if changed {
				t.Errorf("expected changed=false reconciling a %s card", terminal)
			}
			onDisk, _ := store.ReadSpec("add-auth")
			if onDisk.Status != terminal {
				t.Errorf("Status = %q, want %q (terminal preserved)", onDisk.Status, terminal)
			}
		})
	}
}

func TestProposeSpec(t *testing.T) {
	store, _ := Open(t.TempDir())
	if _, err := store.CreateSpec(CreateSpecParams{ID: "add-foo", Title: "Add foo", Now: fixedNow()}); err != nil {
		t.Fatal(err)
	}

	os := &OpenSpec{Change: "add-foo", Artifacts: ArtifactSet{Proposal: true, Design: true, Tasks: true}}
	spec, err := store.ProposeSpec("add-foo", os, "tester", fixedNow())
	if err != nil {
		t.Fatalf("ProposeSpec: %v", err)
	}
	if spec.Status != StatusOpen {
		t.Errorf("Status = %q, want open", spec.Status)
	}
	if spec.OpenSpec == nil || spec.OpenSpec.Change != "add-foo" || !spec.OpenSpec.Artifacts.Tasks {
		t.Errorf("OpenSpec provenance not set: %+v", spec.OpenSpec)
	}
	if spec.StartedAt != nil {
		t.Error("StartedAt must not be set on propose (open != started)")
	}

	// Events: spec.created (from CreateSpec) then spec.proposed + status.changed.
	events := readEvents(t, filepath.Join(store.root, "local", "activity.jsonl"))
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3 (created + proposed + status.changed)", len(events))
	}
	if events[1].Type != EvtSpecProposed || events[2].Type != EvtStatusChanged {
		t.Errorf("unexpected events: %s, %s", events[1].Type, events[2].Type)
	}
	var sc StatusChangedData
	if err := json.Unmarshal(events[2].Data, &sc); err != nil {
		t.Fatal(err)
	}
	if sc.From != StatusDraft || sc.To != StatusOpen || sc.Trigger != "command" {
		t.Errorf("unexpected status.changed: %+v", sc)
	}
}

func TestProposeSpecRejectsNonDraft(t *testing.T) {
	store, _ := Open(t.TempDir())
	os := &OpenSpec{Change: "add-foo"}
	if _, err := store.CreateSpec(CreateSpecParams{ID: "add-foo", Title: "x", Now: fixedNow()}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ProposeSpec("add-foo", os, "t", fixedNow()); err != nil {
		t.Fatalf("first propose: %v", err)
	}
	// Now it's open → second propose must fail.
	if _, err := store.ProposeSpec("add-foo", os, "t", fixedNow()); err == nil {
		t.Fatal("expected error proposing a non-draft spec")
	}
}

func TestListSpecs(t *testing.T) {
	store, _ := Open(t.TempDir())
	for _, title := range []string{"Alpha", "Beta"} {
		if _, err := store.CreateSpec(CreateSpecParams{Title: title, Now: fixedNow()}); err != nil {
			t.Fatalf("CreateSpec(%q): %v", title, err)
		}
	}
	specs, err := store.ListSpecs()
	if err != nil {
		t.Fatalf("ListSpecs: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("ListSpecs len = %d, want 2", len(specs))
	}
	// ReadDir returns sorted names: alpha, beta.
	if specs[0].ID != "alpha" || specs[1].ID != "beta" {
		t.Errorf("unexpected order: %q, %q", specs[0].ID, specs[1].ID)
	}
}

func TestCreateSpecWithTicket(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root)
	spec, err := store.CreateSpec(CreateSpecParams{
		Title:  "Linked at birth",
		Actor:  "tester",
		Now:    fixedNow(),
		Ticket: &Ticket{Provider: TicketJira, Key: "ACME-1", URL: "https://acme.atlassian.net/browse/ACME-1", Auto: true},
	})
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if spec.Ticket == nil || spec.Ticket.Key != "ACME-1" || !spec.Ticket.Auto {
		t.Fatalf("ticket not persisted on returned spec: %+v", spec.Ticket)
	}
	onDisk, err := store.ReadSpec(spec.ID)
	if err != nil {
		t.Fatalf("ReadSpec: %v", err)
	}
	if onDisk.Ticket == nil || onDisk.Ticket.Provider != TicketJira {
		t.Fatalf("ticket not on disk: %+v", onDisk.Ticket)
	}

	// CreateSpec emits spec.created AND spec.linked when a ticket is seeded.
	events := readEvents(t, filepath.Join(root, ".vector", "local", "activity.jsonl"))
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2 (created + linked)", len(events))
	}
	if events[0].Type != EvtSpecCreated || events[1].Type != EvtSpecLinked {
		t.Fatalf("event order = [%s,%s], want [spec.created, spec.linked]", events[0].Type, events[1].Type)
	}
	var data SpecLinkedData
	if err := json.Unmarshal(events[1].Data, &data); err != nil {
		t.Fatalf("decode spec.linked: %v", err)
	}
	if data.Provider != TicketJira || data.Key != "ACME-1" || !data.Auto {
		t.Errorf("unexpected spec.linked data: %+v", data)
	}
}

func TestLinkSpec(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root)
	if _, err := store.CreateSpec(CreateSpecParams{ID: "feat", Title: "Feat", Now: fixedNow()}); err != nil {
		t.Fatal(err)
	}

	// Manual link writes the ticket and emits spec.linked.
	manual := Ticket{Provider: TicketLinear, Key: "ENG-7", URL: "https://linear.app/acme/issue/ENG-7", Auto: false}
	changed, err := store.LinkSpec("feat", manual, "tester", fixedNow())
	if err != nil {
		t.Fatalf("LinkSpec: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first link")
	}
	onDisk, _ := store.ReadSpec("feat")
	if onDisk.Ticket == nil || onDisk.Ticket.Key != "ENG-7" {
		t.Fatalf("ticket not persisted: %+v", onDisk.Ticket)
	}

	// Idempotent: identical link is a no-op (no event, changed=false).
	changed, err = store.LinkSpec("feat", manual, "tester", fixedNow())
	if err != nil {
		t.Fatalf("LinkSpec idempotent: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false re-linking identical ticket")
	}

	// Precedence: an auto link never overwrites the manual one.
	auto := Ticket{Provider: TicketJira, Key: "ACME-99", URL: "https://acme.atlassian.net/browse/ACME-99", Auto: true}
	changed, err = store.LinkSpec("feat", auto, "tester", fixedNow())
	if err != nil {
		t.Fatalf("LinkSpec auto: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false: auto must not clobber a manual link")
	}
	onDisk, _ = store.ReadSpec("feat")
	if onDisk.Ticket.Key != "ENG-7" {
		t.Errorf("manual link overwritten by auto: %+v", onDisk.Ticket)
	}

	// A new manual link DOES replace an existing one.
	replacement := Ticket{Provider: TicketGitHub, Key: "acme/api#3", URL: "https://github.com/acme/api/issues/3"}
	changed, err = store.LinkSpec("feat", replacement, "tester", fixedNow())
	if err != nil {
		t.Fatalf("LinkSpec replace: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true replacing with a new manual link")
	}

	// Events: created + linked(manual) + linked(replacement) = 3 (idempotent and
	// precedence-blocked links emit nothing).
	events := readEvents(t, filepath.Join(root, ".vector", "local", "activity.jsonl"))
	linked := 0
	for _, e := range events {
		if e.Type == EvtSpecLinked {
			linked++
		}
	}
	if linked != 2 {
		t.Errorf("spec.linked event count = %d, want 2", linked)
	}
}

func TestLinkSpecValidates(t *testing.T) {
	store, _ := Open(t.TempDir())
	if _, err := store.CreateSpec(CreateSpecParams{ID: "feat", Title: "Feat", Now: fixedNow()}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LinkSpec("feat", Ticket{Provider: TicketJira}, "t", fixedNow()); err == nil {
		t.Error("expected error linking a ticket with no key")
	}
	if _, err := store.LinkSpec("missing", Ticket{Provider: TicketJira, Key: "X-1"}, "t", fixedNow()); err == nil {
		t.Error("expected error linking a nonexistent spec")
	}
}

func TestCreateSpecWithRelated(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root)
	// A kind:spec relation requires the referenced spec to already exist.
	if _, err := store.CreateSpec(CreateSpecParams{ID: "add-login", Title: "Add login", Now: fixedNow()}); err != nil {
		t.Fatalf("seed cause spec: %v", err)
	}

	spec, err := store.CreateSpec(CreateSpecParams{
		ID:    "fix-login-loop",
		Title: "Fix login loop",
		Actor: "tester",
		Now:   fixedNow(),
		RelatedTo: []RelatedItem{
			{Kind: RelatedSpec, Ref: "add-login", Source: RelatedBlame},
			{Kind: RelatedTicket, Ref: "jira:ACME-7"},                   // source defaults to manual
			{Kind: RelatedSpec, Ref: "add-login", Source: RelatedBlame}, // duplicate, deduped
		},
	})
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if len(spec.RelatedTo) != 2 {
		t.Fatalf("relatedTo len = %d, want 2 (deduped): %+v", len(spec.RelatedTo), spec.RelatedTo)
	}
	if spec.RelatedTo[1].Source != RelatedManual {
		t.Errorf("empty source not defaulted to manual: %+v", spec.RelatedTo[1])
	}
	onDisk, _ := store.ReadSpec("fix-login-loop")
	if len(onDisk.RelatedTo) != 2 {
		t.Fatalf("relatedTo not persisted on disk: %+v", onDisk.RelatedTo)
	}

	// One spec.related event per persisted relation, after spec.created.
	events := readEvents(t, filepath.Join(root, ".vector", "local", "activity.jsonl"))
	related := 0
	for _, e := range events {
		if e.Type == EvtSpecRelated {
			related++
		}
	}
	if related != 2 {
		t.Errorf("spec.related event count = %d, want 2", related)
	}
}

func TestCreateSpecRejectsInvalidRelated(t *testing.T) {
	store, _ := Open(t.TempDir())
	cases := []struct {
		name string
		item RelatedItem
	}{
		{"bad kind", RelatedItem{Kind: "commit", Ref: "x", Source: RelatedManual}},
		{"empty ref", RelatedItem{Kind: RelatedSpec, Ref: "  ", Source: RelatedManual}},
		{"bad source", RelatedItem{Kind: RelatedTicket, Ref: "jira:X-1", Source: "robot"}},
		{"missing spec", RelatedItem{Kind: RelatedSpec, Ref: "ghost", Source: RelatedBlame}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := store.CreateSpec(CreateSpecParams{ID: "fix-" + Slug(tc.name), Title: tc.name, Now: fixedNow(), RelatedTo: []RelatedItem{tc.item}})
			if err == nil {
				t.Fatalf("expected error creating spec with invalid relation %+v", tc.item)
			}
		})
	}
}

func TestRelateSpec(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root)
	if _, err := store.CreateSpec(CreateSpecParams{ID: "fix-bug", Title: "Fix bug", Now: fixedNow()}); err != nil {
		t.Fatal(err)
	}

	// First relate writes and emits spec.related.
	changed, err := store.RelateSpec("fix-bug", RelatedItem{Kind: RelatedTicket, Ref: "jira:ACME-1"}, "tester", fixedNow())
	if err != nil {
		t.Fatalf("RelateSpec: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first relate")
	}
	onDisk, _ := store.ReadSpec("fix-bug")
	if len(onDisk.RelatedTo) != 1 || onDisk.RelatedTo[0].Source != RelatedManual {
		t.Fatalf("relation not persisted with defaulted source: %+v", onDisk.RelatedTo)
	}

	// Idempotent on {kind,ref}: a duplicate is a no-op regardless of source.
	changed, err = store.RelateSpec("fix-bug", RelatedItem{Kind: RelatedTicket, Ref: "jira:ACME-1", Source: RelatedBlame}, "tester", fixedNow())
	if err != nil {
		t.Fatalf("RelateSpec idempotent: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false re-relating same {kind,ref}")
	}

	// A distinct relation appends.
	if _, err := store.RelateSpec("fix-bug", RelatedItem{Kind: RelatedTicket, Ref: "jira:ACME-2"}, "tester", fixedNow()); err != nil {
		t.Fatalf("RelateSpec second: %v", err)
	}
	onDisk, _ = store.ReadSpec("fix-bug")
	if len(onDisk.RelatedTo) != 2 {
		t.Fatalf("relatedTo len = %d, want 2", len(onDisk.RelatedTo))
	}

	// Relating never changes lifecycle status.
	if onDisk.Status != StatusDraft {
		t.Errorf("status changed by relate: %s", onDisk.Status)
	}

	events := readEvents(t, filepath.Join(root, ".vector", "local", "activity.jsonl"))
	related := 0
	for _, e := range events {
		if e.Type == EvtSpecRelated {
			related++
		}
	}
	if related != 2 {
		t.Errorf("spec.related event count = %d, want 2 (idempotent emits nothing)", related)
	}
}

func TestRelateSpecRejectsMissingSpec(t *testing.T) {
	store, _ := Open(t.TempDir())
	if _, err := store.RelateSpec("ghost", RelatedItem{Kind: RelatedTicket, Ref: "jira:X-1"}, "t", fixedNow()); err == nil {
		t.Error("expected error relating a nonexistent spec")
	}
}

// TestSpecWithoutRelatedSerializesClean guards backward compatibility: a spec with
// no relations omits relatedTo entirely (omitempty), so existing state.json files
// read and round-trip byte-identically.
func TestSpecWithoutRelatedSerializesClean(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root)
	spec, err := store.CreateSpec(CreateSpecParams{ID: "plain", Title: "Plain", Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	if spec.RelatedTo != nil {
		t.Errorf("relation-less spec has non-nil RelatedTo: %+v", spec.RelatedTo)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".vector", "specs", "plain", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "relatedTo") {
		t.Errorf("relatedTo present in state.json for a relation-less spec:\n%s", raw)
	}
}

func readEvents(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open activity log: %v", err)
	}
	defer f.Close()
	var events []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("decode event line: %v", err)
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan activity log: %v", err)
	}
	return events
}
