package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func createEpic(t *testing.T, store *Store, p CreateEpicParams) *Epic {
	t.Helper()
	if p.Now.IsZero() {
		p.Now = fixedNow()
	}
	epic, err := store.CreateEpic(p)
	if err != nil {
		t.Fatalf("CreateEpic(%+v): %v", p, err)
	}
	return epic
}

func TestCreateEpicPersistsFileAndEvent(t *testing.T) {
	store := openStore(t)
	epic := createEpic(t, store, CreateEpicParams{Title: "App Mobile", Description: "  iOS + Android  ", Color: EpicColorBlue, Actor: "tester"})

	if epic.ID != "app-mobile" {
		t.Errorf("ID = %q, want app-mobile (slug of the title)", epic.ID)
	}
	if epic.Description != "iOS + Android" {
		t.Errorf("Description = %q, want trimmed", epic.Description)
	}
	if epic.SchemaVersion != EpicSchemaVersion {
		t.Errorf("SchemaVersion = %d", epic.SchemaVersion)
	}

	raw, err := os.ReadFile(filepath.Join(store.root, "epics", "app-mobile.json"))
	if err != nil {
		t.Fatalf("epic file: %v", err)
	}
	var onDisk Epic
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse epic file: %v", err)
	}
	if onDisk != *epic {
		t.Errorf("on-disk epic = %+v, want %+v", onDisk, *epic)
	}

	events, _ := store.ReadEvents()
	if len(events) != 1 || events[0].Type != EvtEpicCreated || events[0].SpecID != "" {
		t.Fatalf("events = %+v, want one epic.created without a spec id", events)
	}
	var data EpicEventData
	if err := json.Unmarshal(events[0].Data, &data); err != nil || data.ID != "app-mobile" || data.Title != "App Mobile" {
		t.Errorf("epic.created data = %+v (%v)", data, err)
	}
}

func TestCreateEpicValidation(t *testing.T) {
	store := openStore(t)
	createEpic(t, store, CreateEpicParams{Title: "App Mobile"})

	cases := []struct {
		name   string
		params CreateEpicParams
		class  error
	}{
		{"empty title", CreateEpicParams{Title: "   "}, ErrInvalidInput},
		{"non-kebab id", CreateEpicParams{Title: "X", ID: "App_Mobile"}, ErrInvalidInput},
		{"traversal id", CreateEpicParams{Title: "X", ID: "../evil"}, ErrInvalidInput},
		{"unknown color", CreateEpicParams{Title: "X", Color: "chartreuse"}, ErrInvalidInput},
		{"duplicate", CreateEpicParams{Title: "App Mobile"}, ErrConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.params.Now = fixedNow()
			if _, err := store.CreateEpic(tc.params); !errors.Is(err, tc.class) {
				t.Fatalf("err = %v, want class %v", err, tc.class)
			}
		})
	}
}

func TestListAndGetEpics(t *testing.T) {
	store := openStore(t)
	if epics, err := store.ListEpics(); err != nil || len(epics) != 0 {
		t.Fatalf("ListEpics on a fresh repo = %v, %v; want empty, no error", epics, err)
	}
	createEpic(t, store, CreateEpicParams{Title: "Web"})
	createEpic(t, store, CreateEpicParams{Title: "App Mobile"})

	epics, err := store.ListEpics()
	if err != nil {
		t.Fatal(err)
	}
	if len(epics) != 2 || epics[0].ID != "app-mobile" || epics[1].ID != "web" {
		t.Fatalf("ListEpics = %+v, want [app-mobile web]", epics)
	}
	if _, err := store.GetEpic("nope"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("GetEpic(missing) err = %v, want not-exist", err)
	}
}

func TestUpdateEpic(t *testing.T) {
	store := openStore(t)
	createEpic(t, store, CreateEpicParams{Title: "App Mobile", Description: "old", Color: EpicColorBlue})
	later := fixedNow().Add(1)

	title := "Mobile app"
	clearedColor := EpicColor("")
	updated, changed, err := store.UpdateEpic("app-mobile", UpdateEpicParams{Title: &title, Color: &clearedColor}, "tester", later)
	if err != nil || !changed {
		t.Fatalf("UpdateEpic = %v, %v", changed, err)
	}
	if updated.Title != "Mobile app" || updated.Color != "" || updated.Description != "old" || !updated.UpdatedAt.Equal(later) {
		t.Errorf("updated = %+v", updated)
	}
	if updated.ID != "app-mobile" {
		t.Errorf("the id is stable across a title change, got %q", updated.ID)
	}

	// Same values → no-op, no event.
	if _, changed, err := store.UpdateEpic("app-mobile", UpdateEpicParams{Title: &title}, "tester", later); err != nil || changed {
		t.Errorf("no-op update = %v, %v", changed, err)
	}
	if got := countEvents(eventTypes(t, store), EvtEpicUpdated); got != 1 {
		t.Errorf("epic.updated events = %d, want 1", got)
	}

	empty := " "
	if _, _, err := store.UpdateEpic("app-mobile", UpdateEpicParams{Title: &empty}, "tester", later); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("empty title err = %v, want ErrInvalidInput", err)
	}
	bad := EpicColor("mauve")
	if _, _, err := store.UpdateEpic("app-mobile", UpdateEpicParams{Color: &bad}, "tester", later); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad color err = %v, want ErrInvalidInput", err)
	}
	if _, _, err := store.UpdateEpic("missing", UpdateEpicParams{Title: &title}, "tester", later); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing epic err = %v, want not-exist", err)
	}
}

func TestAssignEpic(t *testing.T) {
	store := openStore(t)
	createEpic(t, store, CreateEpicParams{Title: "App Mobile"})
	createEpic(t, store, CreateEpicParams{Title: "Web"})
	newSpec(t, store, "login", StatusOpen, PriorityNormal)

	if _, err := store.AssignEpic("login", "nope", "tester", fixedNow()); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("assign unknown epic err = %v, want ErrInvalidInput", err)
	}
	if _, err := store.AssignEpic("ghost", "web", "tester", fixedNow()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("assign to unknown spec err = %v, want not-exist", err)
	}

	if changed, err := store.AssignEpic("login", "app-mobile", "tester", fixedNow()); err != nil || !changed {
		t.Fatalf("assign = %v, %v", changed, err)
	}
	if changed, err := store.AssignEpic("login", "app-mobile", "tester", fixedNow()); err != nil || changed {
		t.Fatalf("re-assign = %v, %v; want no-op", changed, err)
	}
	if changed, err := store.AssignEpic("login", "web", "tester", fixedNow()); err != nil || !changed {
		t.Fatalf("move = %v, %v", changed, err)
	}
	if changed, err := store.AssignEpic("login", "", "tester", fixedNow()); err != nil || !changed {
		t.Fatalf("clear = %v, %v", changed, err)
	}
	spec, _ := store.ReadSpec("login")
	if spec.Epic != "" || spec.Status != StatusOpen {
		t.Errorf("after clear: epic=%q status=%s", spec.Epic, spec.Status)
	}

	events, _ := store.ReadEvents()
	var assigned []EpicAssignedData
	for _, event := range events {
		if event.Type == EvtEpicAssigned {
			var data EpicAssignedData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatal(err)
			}
			assigned = append(assigned, data)
		}
	}
	want := []EpicAssignedData{{Epic: "app-mobile"}, {Epic: "web", Previous: "app-mobile"}, {Epic: "", Previous: "web"}}
	if len(assigned) != len(want) {
		t.Fatalf("spec.epic-assigned events = %+v, want %+v", assigned, want)
	}
	for index := range want {
		if assigned[index] != want[index] {
			t.Errorf("event %d = %+v, want %+v", index, assigned[index], want[index])
		}
	}
}

func TestCreateSpecWithEpic(t *testing.T) {
	store := openStore(t)
	createEpic(t, store, CreateEpicParams{Title: "App Mobile"})

	if _, err := store.CreateSpec(CreateSpecParams{ID: "orphan", Title: "Orphan", Epic: "nope", Now: fixedNow()}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("create with unknown epic err = %v, want ErrInvalidInput", err)
	}
	if _, err := os.Stat(store.StatePath("orphan")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused create must not leave a state.json behind")
	}

	spec, err := store.CreateSpec(CreateSpecParams{ID: "push", Title: "Push notifications", Epic: "app-mobile", Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Epic != "app-mobile" {
		t.Errorf("Epic = %q", spec.Epic)
	}
	types := eventTypes(t, store)
	if countEvents(types, EvtEpicAssigned) != 1 {
		t.Errorf("events = %v, want one spec.epic-assigned", types)
	}
}

func TestDeleteEpicRefusesWhileReferenced(t *testing.T) {
	store := openStore(t)
	createEpic(t, store, CreateEpicParams{Title: "App Mobile"})
	newSpec(t, store, "login", StatusOpen, PriorityNormal)
	if _, err := store.AssignEpic("login", "app-mobile", "tester", fixedNow()); err != nil {
		t.Fatal(err)
	}

	err := store.DeleteEpic("app-mobile", "tester", fixedNow())
	if !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), "login") {
		t.Fatalf("delete referenced epic err = %v, want ErrConflict naming login", err)
	}
	if _, err := store.GetEpic("app-mobile"); err != nil {
		t.Fatalf("a refused delete must keep the epic: %v", err)
	}

	if _, err := store.AssignEpic("login", "", "tester", fixedNow()); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteEpic("app-mobile", "tester", fixedNow()); err != nil {
		t.Fatalf("delete unreferenced epic: %v", err)
	}
	if _, err := store.GetEpic("app-mobile"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("epic still readable after delete: %v", err)
	}
	if countEvents(eventTypes(t, store), EvtEpicDeleted) != 1 {
		t.Error("want one epic.deleted event")
	}
	if err := store.DeleteEpic("app-mobile", "tester", fixedNow()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("deleting twice err = %v, want not-exist", err)
	}
}

func TestCountEpicSpecs(t *testing.T) {
	specs := []*SpecState{
		{ID: "a", Status: StatusOpen, Epic: "mobile"},
		{ID: "b", Status: StatusClosed, Epic: "mobile"},                               // legacy closed → done
		{ID: "c", Status: StatusArchived, Epic: "mobile", Resolution: ResolutionDone}, // explicit done
		{ID: "c2", Status: StatusArchived, Epic: "mobile"},                            // legacy archived → not done
		{ID: "x", Status: StatusClosed, Epic: "mobile", Resolution: ResolutionDuplicate},
		{ID: "y", Status: StatusArchived, Epic: "mobile", Resolution: ResolutionObsolete},
		{ID: "d", Status: StatusInProgress, Epic: "web"},
		{ID: "e", Status: StatusOpen},
	}
	counts := CountEpicSpecs(specs)
	mobile := counts["mobile"]
	if mobile.Total != 4 || mobile.Done != 2 || mobile.Dropped != 2 {
		t.Errorf("mobile = %+v, want total 4 (a,b,c,c2), done 2 (b legacy closed, c done), dropped 2 (x,y)", mobile)
	}
	if mobile.ByStatus[StatusOpen] != 1 || mobile.ByStatus[StatusArchived] != 3 || mobile.ByStatus[StatusClosed] != 2 {
		t.Errorf("byStatus must cover every member, dropped included: %+v", mobile.ByStatus)
	}
	if web := counts["web"]; web.Total != 1 || web.Done != 0 || web.Dropped != 0 {
		t.Errorf("web = %+v", web)
	}
	if _, ok := counts[""]; ok {
		t.Error("specs without an epic must not be counted")
	}
}
