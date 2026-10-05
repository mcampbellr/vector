package board

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

// TestBuildInheritedFocusAndResolution pins the derived-focus projection: a card
// of a focused epic carries focusInherited (focus stays the spec's own flag),
// sorts with the focused cards, and a closed card never inherits. Non-done
// resolutions are projected on the card.
func TestBuildInheritedFocusAndResolution(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	src := fakeSource{
		epics: []*state.Epic{{ID: "hot", Title: "Hot", Focus: true}},
		specs: []*state.SpecState{
			{ID: "urgent", Title: "U", Status: state.StatusOpen, Priority: state.PriorityUrgent, UpdatedAt: now},
			{ID: "inherits", Title: "I", Status: state.StatusOpen, Priority: state.PriorityLow, Epic: "hot", UpdatedAt: now},
			{ID: "own", Title: "O", Status: state.StatusOpen, Priority: state.PriorityHigh, Focus: true, Epic: "hot", UpdatedAt: now},
			{ID: "done-dup", Title: "D", Status: state.StatusClosed, Priority: state.PriorityNormal, Epic: "hot",
				Resolution: state.ResolutionDuplicate, ResolutionNote: "dup of own", UpdatedAt: now},
		},
	}
	b, err := Build(src, "demo", now)
	if err != nil {
		t.Fatal(err)
	}
	open := columnByStatus(t, b, "open")
	ids := make([]string, 0, len(open.Cards))
	for _, card := range open.Cards {
		ids = append(ids, card.ID)
	}
	if got := strings.Join(ids, ","); got != "own,inherits,urgent" {
		t.Fatalf("open order = %s, want own,inherits,urgent (effective focus first)", got)
	}
	if card := open.Cards[0]; !card.Focus || card.FocusInherited {
		t.Errorf("own-focused card = focus %v inherited %v, want own focus only", card.Focus, card.FocusInherited)
	}
	if card := open.Cards[1]; card.Focus || !card.FocusInherited {
		t.Errorf("inheriting card = focus %v inherited %v, want inherited only", card.Focus, card.FocusInherited)
	}
	closed := columnByStatus(t, b, "closed")
	if card := closed.Cards[0]; card.FocusInherited || card.Resolution != "duplicate" || card.ResolutionNote != "dup of own" {
		t.Errorf("closed card = %+v, want no inherited focus and the duplicate resolution", card)
	}
	if !b.Epics[0].Focus || b.Epics[0].Dropped != 1 || b.Epics[0].Total != 2 {
		t.Errorf("epic summary = %+v, want focus, dropped 1, total 2", b.Epics[0])
	}
}

func TestBuildSortsEpicsByOrder(t *testing.T) {
	src := fakeSource{epics: []*state.Epic{
		{ID: "b-unordered", Title: "B"},
		{ID: "second", Title: "Second", Order: 2},
		{ID: "a-unordered", Title: "A"},
		{ID: "first", Title: "First", Order: 1},
	}}
	b, err := Build(src, "demo", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(b.Epics))
	for _, epic := range b.Epics {
		ids = append(ids, epic.ID)
	}
	if got := strings.Join(ids, ","); got != "first,second,a-unordered,b-unordered" {
		t.Errorf("epic order = %s", got)
	}
	if b.Epics[0].Order != 1 || b.Epics[2].Order != 0 {
		t.Errorf("order not projected: %+v", b.Epics)
	}
}

func TestBoardJSONOmitsNewKeysWhenUnset(t *testing.T) {
	body := boardJSON(t, fakeSource{
		epics: []*state.Epic{{ID: "web", Title: "Web"}},
		specs: []*state.SpecState{{ID: "a", Title: "A", Status: state.StatusOpen, Priority: state.PriorityNormal}},
	})
	for _, key := range []string{`"focusInherited"`, `"resolution"`, `"resolutionNote"`, `"order"`, `"dropped"`, `"focus"`} {
		if strings.Contains(body, key) {
			t.Errorf("expected %s omitted when unset, got: %s", key, body)
		}
	}
}

func TestEpicOrderAndFocusEndpoints(t *testing.T) {
	srv, store := newWriteServer(t)
	rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics", body: `{"title":"App Mobile","order":2}`})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body)
	}
	if epic, _ := store.GetEpic("app-mobile"); epic.Order != 2 {
		t.Fatalf("create order = %d, want 2", epic.Order)
	}

	rec = do(t, srv, writeRequest{method: http.MethodPatch, path: "/api/epics/app-mobile", body: `{"order":1}`})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", rec.Code, rec.Body)
	}
	var patched state.Epic
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil || patched.Order != 1 {
		t.Fatalf("patch response = %+v (%v)", patched, err)
	}
	if rec = do(t, srv, writeRequest{method: http.MethodPatch, path: "/api/epics/app-mobile", body: `{"order":-3}`}); rec.Code != http.StatusBadRequest {
		t.Errorf("negative order status = %d, want 400", rec.Code)
	}
	if rec = do(t, srv, writeRequest{method: http.MethodPatch, path: "/api/epics/app-mobile", body: `{"order":0}`}); rec.Code != http.StatusOK {
		t.Fatalf("unset order status = %d", rec.Code)
	}
	if epic, _ := store.GetEpic("app-mobile"); epic.Order != 0 {
		t.Errorf("order 0 must unset, got %d", epic.Order)
	}

	rec = do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics/app-mobile/focus", body: `{"focus":true}`})
	if rec.Code != http.StatusOK {
		t.Fatalf("epic focus status = %d body=%s", rec.Code, rec.Body)
	}
	var focusResp EpicFocusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &focusResp); err != nil || focusResp != (EpicFocusResponse{ID: "app-mobile", Focus: true, Changed: true}) {
		t.Fatalf("epic focus response = %+v (%v)", focusResp, err)
	}
	if epic, _ := store.GetEpic("app-mobile"); !epic.Focus {
		t.Error("epic focus not persisted")
	}
	if rec = do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics/app-mobile/focus", body: `{}`}); rec.Code != http.StatusBadRequest {
		t.Errorf("missing focus status = %d, want 400", rec.Code)
	}
	if rec = do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics/ghost/focus", body: `{"focus":true}`}); rec.Code != http.StatusNotFound {
		t.Errorf("unknown epic status = %d, want 404", rec.Code)
	}
	if rec = do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics/app-mobile/focus", body: `{"focus":true}`,
		headers: map[string]string{"Origin": "http://evil.example"}}); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin epic focus status = %d, want 403", rec.Code)
	}

	// The inherited focus reaches the served board: login joins the epic.
	if _, err := store.AssignEpic("login", "app-mobile", "tester", time.Now()); err != nil {
		t.Fatal(err)
	}
	b, err := Build(store, "demo", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if card := columnByStatus(t, b, "open").Cards[0]; card.ID != "login" || !card.FocusInherited || card.Focus {
		t.Errorf("login card = %+v, want focusInherited only", card)
	}
}
