package board

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
