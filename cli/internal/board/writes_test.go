package board

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

const testListenAddr = "127.0.0.1:8787"

// newWriteServer builds a Server over a real Store (the writes must go through
// the Store mutators) with writes enabled for testListenAddr, seeded with one
// open spec "login".
func newWriteServer(t *testing.T) (*Server, *state.Store) {
	t.Helper()
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "login", Title: "Login", Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(store, "demo")
	if err := srv.EnableWrites(store, "board-test", testListenAddr); err != nil {
		t.Fatal(err)
	}
	return srv, store
}

type writeRequest struct {
	method  string
	path    string
	body    string
	headers map[string]string
}

// do sends a same-origin JSON request by default; headers override/extend it.
func do(t *testing.T, srv *Server, req writeRequest) *httptest.ResponseRecorder {
	t.Helper()
	httpReq := httptest.NewRequest(req.method, req.path, strings.NewReader(req.body))
	httpReq.Host = testListenAddr
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Origin", "http://"+testListenAddr)
	for key, value := range req.headers {
		if value == "" {
			httpReq.Header.Del(key)
			continue
		}
		if key == "Host" {
			httpReq.Host = value
			continue
		}
		httpReq.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	srv.Routes(nil).ServeHTTP(rec, httpReq)
	return rec
}

func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %q", rec.Body.String())
	}
	return body.Error
}

func TestFocusEndpointPersistsThroughStore(t *testing.T) {
	srv, store := newWriteServer(t)

	rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":true}`})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body)
	}
	var resp FocusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp != (FocusResponse{ID: "login", Focus: true, Changed: true}) {
		t.Fatalf("response = %+v (%v)", resp, err)
	}
	spec, _ := store.ReadSpec("login")
	if !spec.Focus {
		t.Fatal("focus not persisted")
	}
	events, _ := store.ReadEvents()
	last := events[len(events)-1]
	if last.Type != state.EvtSpecFocused || last.Actor != "board-test" {
		t.Errorf("last event = %+v, want spec.focused by board-test", last)
	}

	rec = do(t, srv, writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":false}`})
	if rec.Code != http.StatusOK {
		t.Fatalf("unfocus status = %d", rec.Code)
	}
	if spec, _ := store.ReadSpec("login"); spec.Focus {
		t.Error("unfocus not persisted")
	}
}

func TestWriteBroadcastsFreshBoard(t *testing.T) {
	srv, _ := newWriteServer(t)
	client := make(chan []byte, 1)
	srv.subscribe(client)
	defer srv.unsubscribe(client)

	if rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":true}`}); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	select {
	case frame := <-client:
		if !strings.Contains(string(frame), `"focus":true`) {
			t.Errorf("broadcast board lacks the write: %s", frame)
		}
	default:
		t.Fatal("a successful write must push the board to SSE clients")
	}
}

func TestWriteGuards(t *testing.T) {
	cases := []struct {
		name       string
		req        writeRequest
		wantStatus int
		wantError  string
	}{
		{"wrong method", writeRequest{method: http.MethodGet, path: "/api/specs/login/focus"}, http.StatusMethodNotAllowed, "method GET not allowed"},
		{"foreign origin", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":true}`, headers: map[string]string{"Origin": "https://evil.example"}}, http.StatusForbidden, "cross-origin"},
		{"null origin", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":true}`, headers: map[string]string{"Origin": "null"}}, http.StatusForbidden, "cross-origin"},
		{"other port origin", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":true}`, headers: map[string]string{"Origin": "http://127.0.0.1:5173"}}, http.StatusForbidden, "cross-origin"},
		{"rebinding host", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":true}`, headers: map[string]string{"Host": "evil.example:8787", "Origin": "http://evil.example:8787"}}, http.StatusForbidden, "not this board"},
		{"cross-site fetch metadata", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":true}`, headers: map[string]string{"Origin": "", "Sec-Fetch-Site": "cross-site"}}, http.StatusForbidden, "cross-site"},
		{"form content type", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `focus=true`, headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"}}, http.StatusUnsupportedMediaType, "application/json"},
		{"malformed json", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":`}, http.StatusBadRequest, "invalid JSON"},
		{"unknown field", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":true,"priority":"urgent"}`}, http.StatusBadRequest, "unknown field"},
		{"trailing data", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{"focus":true}{}`}, http.StatusBadRequest, "trailing data"},
		{"missing focus", writeRequest{method: http.MethodPost, path: "/api/specs/login/focus", body: `{}`}, http.StatusBadRequest, `missing "focus"`},
		{"unknown spec", writeRequest{method: http.MethodPost, path: "/api/specs/ghost/focus", body: `{"focus":true}`}, http.StatusNotFound, "ghost"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, store := newWriteServer(t)
			rec := do(t, srv, tc.req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body)
			}
			if msg := errorMessage(t, rec); !strings.Contains(msg, tc.wantError) {
				t.Errorf("error = %q, want it to mention %q", msg, tc.wantError)
			}
			if spec, _ := store.ReadSpec("login"); spec.Focus {
				t.Error("a rejected write must not mutate state")
			}
		})
	}
}

func TestWritesDisabledWithoutEnableWrites(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(store, "demo") // read-only server
	rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics", body: `{"title":"X"}`})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 when writes are not enabled", rec.Code)
	}
	if epics, _ := store.ListEpics(); len(epics) != 0 {
		t.Error("a read-only server created an epic")
	}
}

func TestAllowedWriteHosts(t *testing.T) {
	hosts, err := allowedWriteHosts("0.0.0.0:9000")
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"localhost:9000", "127.0.0.1:9000", "[::1]:9000"} {
		if !hosts[host] {
			t.Errorf("%s should be allowed", host)
		}
	}
	if hosts["0.0.0.0:9000"] || hosts["localhost:8787"] {
		t.Errorf("unspecified bind or another port must not be allowed: %v", hosts)
	}
	specific, _ := allowedWriteHosts("192.168.1.20:9000")
	if !specific["192.168.1.20:9000"] {
		t.Error("a specific bound interface should be allowed")
	}
	if _, err := allowedWriteHosts("no-port"); err == nil {
		t.Error("an address without a port must be rejected")
	}
}

func TestEpicEndpoints(t *testing.T) {
	srv, store := newWriteServer(t)

	rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics", body: `{"title":"App Mobile","description":"iOS","color":"blue"}`})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body)
	}
	var created state.Epic
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID != "app-mobile" || created.Color != state.EpicColorBlue {
		t.Fatalf("created = %+v (%v)", created, err)
	}

	if rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics", body: `{"title":"App Mobile"}`}); rec.Code != http.StatusConflict {
		t.Errorf("duplicate create status = %d, want 409", rec.Code)
	}
	if rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics", body: `{"title":"X","color":"neon"}`}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad color status = %d, want 400", rec.Code)
	}
	if rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics", body: `{"title":""}`}); rec.Code != http.StatusBadRequest {
		t.Errorf("empty title status = %d, want 400", rec.Code)
	}
	if rec := do(t, srv, writeRequest{method: http.MethodPut, path: "/api/epics", body: `{}`}); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT /api/epics status = %d, want 405", rec.Code)
	}

	rec = do(t, srv, writeRequest{method: http.MethodPatch, path: "/api/epics/app-mobile", body: `{"title":"Mobile","color":""}`})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", rec.Code, rec.Body)
	}
	epic, _ := store.GetEpic("app-mobile")
	if epic.Title != "Mobile" || epic.Color != "" || epic.Description != "iOS" {
		t.Errorf("patched epic = %+v", epic)
	}
	if rec := do(t, srv, writeRequest{method: http.MethodPatch, path: "/api/epics/ghost", body: `{"title":"X"}`}); rec.Code != http.StatusNotFound {
		t.Errorf("patch unknown epic status = %d, want 404", rec.Code)
	}
	if rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/epics/app-mobile", body: `{"title":"X"}`}); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/epics/{id} status = %d, want 405", rec.Code)
	}
}

func TestSpecEpicEndpoint(t *testing.T) {
	srv, store := newWriteServer(t)
	if _, err := store.CreateEpic(state.CreateEpicParams{Title: "App Mobile", Now: time.Now()}); err != nil {
		t.Fatal(err)
	}

	rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/specs/login/epic", body: `{"epic":"app-mobile"}`})
	if rec.Code != http.StatusOK {
		t.Fatalf("assign status = %d body=%s", rec.Code, rec.Body)
	}
	if spec, _ := store.ReadSpec("login"); spec.Epic != "app-mobile" {
		t.Fatalf("epic not persisted: %q", spec.Epic)
	}

	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"unknown epic", `{"epic":"nope"}`, http.StatusBadRequest},
		{"wrong type", `{"epic":5}`, http.StatusBadRequest},
		{"missing field", `{}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if rec := do(t, srv, writeRequest{method: http.MethodPost, path: "/api/specs/login/epic", body: tc.body}); rec.Code != tc.wantStatus {
			t.Errorf("%s: status = %d, want %d", tc.name, rec.Code, tc.wantStatus)
		}
	}

	rec = do(t, srv, writeRequest{method: http.MethodPost, path: "/api/specs/login/epic", body: `{"epic":null}`})
	if rec.Code != http.StatusOK {
		t.Fatalf("clear status = %d body=%s", rec.Code, rec.Body)
	}
	var resp EpicAssignResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp != (EpicAssignResponse{ID: "login", Epic: "", Changed: true}) {
		t.Errorf("clear response = %+v (%v)", resp, err)
	}
	if spec, _ := store.ReadSpec("login"); spec.Epic != "" {
		t.Errorf("epic not cleared: %q", spec.Epic)
	}
}
