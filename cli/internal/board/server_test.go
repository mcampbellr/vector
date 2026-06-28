package board

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

func TestHandleBoardReturnsJSON(t *testing.T) {
	src := fakeSource{specs: []*state.SpecState{
		{ID: "a", Title: "A", Status: state.StatusOpen, Priority: state.PriorityNormal},
	}}
	srv := NewServer(src, "demo")

	req := httptest.NewRequest(http.MethodGet, "/api/board", nil)
	rec := httptest.NewRecorder()
	srv.Routes(nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	var b Board
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("response is not valid Board JSON: %v", err)
	}
	if b.Repo != "demo" {
		t.Errorf("repo = %q, want demo", b.Repo)
	}
	if b.Totals.Specs != 1 {
		t.Errorf("totals.specs = %d, want 1", b.Totals.Specs)
	}
}

func TestNeedsUATSerialization(t *testing.T) {
	// True → present in the JSON; false → omitted (omitempty), so the web client
	// reads it as undefined and shows no badge. Guards against a silent regression.
	flagged := fakeSource{specs: []*state.SpecState{
		{ID: "a", Title: "A", Status: state.StatusReview, Priority: state.PriorityNormal, NeedsUAT: true},
	}}
	clean := fakeSource{specs: []*state.SpecState{
		{ID: "b", Title: "B", Status: state.StatusReview, Priority: state.PriorityNormal, NeedsUAT: false},
	}}

	if body := boardJSON(t, flagged); !strings.Contains(body, `"needsUat":true`) {
		t.Errorf("expected needsUat:true in response, got: %s", body)
	}
	if body := boardJSON(t, clean); strings.Contains(body, "needsUat") {
		t.Errorf("expected needsUat omitted when false, got: %s", body)
	}
}

func boardJSON(t *testing.T, src fakeSource) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/board", nil)
	rec := httptest.NewRecorder()
	NewServer(src, "demo").Routes(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

func TestHandleStandupReturnsEmptyWhenNeverRun(t *testing.T) {
	src := fakeSource{} // ReadStandup → zero-value digest (never committed)
	rec := getJSON(t, src, "/api/standup")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "{}" {
		t.Errorf("body = %q, want {}", body)
	}
}

func TestHandleStandupReturnsDigest(t *testing.T) {
	src := fakeSource{standup: &state.StandupDigest{
		SchemaVersion: state.StandupSchemaVersion,
		GeneratedAt:   time.Date(2026, 6, 25, 15, 0, 0, 0, time.UTC),
		Global:        "shipped alpha",
		PerSpec:       []state.StandupSpecDigest{{ID: "alpha", Title: "Alpha", Status: "review", Summary: "done", ChangeCount: 3}},
		Totals:        state.StandupTotals{Specs: 1, Changes: 3, ByStatus: map[string]int{"review": 1}},
	}}
	rec := getJSON(t, src, "/api/standup")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"global":"shipped alpha"`) {
		t.Errorf("digest body missing global prose: %s", rec.Body.String())
	}
}

func TestHandleActivityProjectsTimeline(t *testing.T) {
	now := time.Now().UTC()
	statusData, _ := json.Marshal(state.StatusChangedData{From: state.StatusOpen, To: state.StatusInProgress, Trigger: "apply"})
	workData, _ := json.Marshal(state.WorkLoggedData{FilesTouched: []string{"a.go"}, Note: "wired"})
	src := fakeSource{
		specs: []*state.SpecState{{ID: "alpha", Title: "Alpha", Status: state.StatusInProgress}},
		events: []state.Event{
			{V: 1, TS: now.Add(-time.Hour), Type: state.EvtStatusChanged, SpecID: "alpha", Data: statusData},
			{V: 1, TS: now.Add(-30 * time.Minute), Type: state.EvtWorkLogged, SpecID: "alpha", Data: workData},
		},
	}
	rec := getJSON(t, src, "/api/activity?spec=alpha&since=24h")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"spec":"alpha"`) || !strings.Contains(body, `"type":"work.logged"`) {
		t.Errorf("unexpected activity body: %s", body)
	}
}

func TestHandleActivityInvalidSince(t *testing.T) {
	src := fakeSource{specs: []*state.SpecState{{ID: "alpha", Status: state.StatusOpen}}}
	rec := getJSON(t, src, "/api/activity?spec=alpha&since=36h")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid since") {
		t.Errorf("expected invalid-since error body, got: %s", rec.Body.String())
	}
}

func TestHandleActivityUnknownSpec(t *testing.T) {
	src := fakeSource{specs: []*state.SpecState{{ID: "alpha", Status: state.StatusOpen}}}
	rec := getJSON(t, src, "/api/activity?spec=ghost")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `not found`) {
		t.Errorf("expected not-found error body, got: %s", rec.Body.String())
	}
}

func TestHandleSummaryReturnsPersisted(t *testing.T) {
	src := fakeSource{summaries: map[string]state.SpecSummary{
		"alpha": {SchemaVersion: 1, ID: "alpha", Summary: "applied the change", Action: "apply"},
	}}
	rec := getJSON(t, src, "/api/summary?spec=alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"summary":"applied the change"`) || !strings.Contains(body, `"action":"apply"`) {
		t.Errorf("unexpected summary body: %s", body)
	}
}

func TestHandleSummaryEmptyWhenAbsent(t *testing.T) {
	src := fakeSource{}
	rec := getJSON(t, src, "/api/summary?spec=ghost")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "{}" {
		t.Errorf("body = %q, want {}", got)
	}
}

func TestHandleSummaryMissingParam(t *testing.T) {
	src := fakeSource{}
	rec := getJSON(t, src, "/api/summary")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "missing spec") {
		t.Errorf("expected missing-spec error body, got: %s", rec.Body.String())
	}
}

func TestHandleFileServesMarkdown(t *testing.T) {
	src := fakeSource{
		specs:     []*state.SpecState{{ID: "alpha", Status: state.StatusOpen}},
		artifacts: map[string][]byte{"alpha/proposal": []byte("# Proposal\n\nbody")},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/file?spec=alpha&artifact=proposal", nil)
	rec := httptest.NewRecorder()
	NewServer(src, "demo").Routes(nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
		t.Errorf("content-type = %q, want text/markdown; charset=utf-8", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("cache-control = %q, want no-store", cc)
	}
	if body := rec.Body.String(); body != "# Proposal\n\nbody" {
		t.Errorf("body = %q, want the raw markdown", body)
	}
}

func TestHandleFileBadParams(t *testing.T) {
	src := fakeSource{specs: []*state.SpecState{{ID: "alpha", Status: state.StatusOpen}}}
	for _, path := range []string{
		"/api/file?artifact=proposal",           // missing spec
		"/api/file?spec=alpha",                  // missing artifact
		"/api/file?spec=alpha&artifact=secrets", // unknown artifact
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		NewServer(src, "demo").Routes(nil).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", path, rec.Code)
		}
	}
}

func TestHandleFileNotFound(t *testing.T) {
	src := fakeSource{specs: []*state.SpecState{{ID: "alpha", Status: state.StatusOpen}}}
	// No artifacts registered → fakeSource returns fs.ErrNotExist.
	req := httptest.NewRequest(http.MethodGet, "/api/file?spec=ghost&artifact=design", nil)
	rec := httptest.NewRecorder()
	NewServer(src, "demo").Routes(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not found") {
		t.Errorf("expected not-found error body, got: %s", rec.Body.String())
	}
}

func TestHandleFileReadError(t *testing.T) {
	src := fakeSource{
		specs:    []*state.SpecState{{ID: "alpha", Status: state.StatusOpen}},
		artifErr: errors.New("disk on fire"),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/file?spec=alpha&artifact=tasks", nil)
	rec := httptest.NewRecorder()
	NewServer(src, "demo").Routes(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func getJSON(t *testing.T, src fakeSource, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	NewServer(src, "demo").Routes(nil).ServeHTTP(rec, req)
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	return rec
}

func TestHandleEventsStreamsInitialBoard(t *testing.T) {
	src := fakeSource{specs: []*state.SpecState{
		{ID: "a", Title: "A", Status: state.StatusReview, Priority: state.PriorityHigh},
	}}
	srv := NewServer(src, "demo")

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()
	// handleEvents writes the initial board, then blocks on the request context.
	// A pre-cancelled context makes it return right after that first frame.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	srv.handleEvents(rec, req.WithContext(ctx))

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("content-type = %q, want text/event-stream", got)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "event: board\ndata: ") {
		t.Fatalf("expected an initial 'event: board' frame, got: %q", body)
	}
}
