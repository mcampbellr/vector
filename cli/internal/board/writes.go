package board

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

// Writer is the subset of *state.Store the board's write endpoints call. Every
// board write goes through these Store mutators — the same ones the CLI uses — so
// the single-writer guarantee (CLI-owns-writes) holds: the server never touches a
// state file directly.
type Writer interface {
	SetFocus(id string, focus bool, actor string, now time.Time) (bool, error)
	AssignEpic(specID, epicID, actor string, now time.Time) (bool, error)
	CreateEpic(p state.CreateEpicParams) (*state.Epic, error)
	UpdateEpic(id string, p state.UpdateEpicParams, actor string, now time.Time) (*state.Epic, bool, error)
	SetEpicFocus(id string, focus bool, actor string, now time.Time) (bool, error)
}

// maxWriteBody caps a write request body; every payload is a handful of short
// strings.
const maxWriteBody = 64 << 10

// EnableWrites turns on the board's write endpoints. listenAddr is the address
// the server is bound to (host:port); it defines which Host headers a write may
// carry (see checkSameOrigin). actor labels the activity events the writes emit.
// Without this call every write endpoint answers 403, so a Server built only for
// reading (tests, tooling) can never mutate state.
func (s *Server) EnableWrites(writer Writer, actor, listenAddr string) error {
	hosts, err := allowedWriteHosts(listenAddr)
	if err != nil {
		return err
	}
	s.writer = writer
	s.actor = actor
	s.writeHosts = hosts
	return nil
}

// allowedWriteHosts derives the Host header values a write may carry from the
// bound address: the loopback names on the bound port, plus the bound host itself
// when it is a specific interface. Requiring a known Host (not merely Origin ==
// Host) is what defeats DNS rebinding, where a hostile page reaches the local
// port under its own hostname.
func allowedWriteHosts(listenAddr string) (map[string]bool, error) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil, fmt.Errorf("parse listen address %q: %w", listenAddr, err)
	}
	hosts := map[string]bool{}
	for _, name := range []string{"localhost", "127.0.0.1", "::1"} {
		hosts[strings.ToLower(net.JoinHostPort(name, port))] = true
	}
	if ip := net.ParseIP(host); host != "" && (ip == nil || !ip.IsUnspecified()) {
		hosts[strings.ToLower(net.JoinHostPort(host, port))] = true
	}
	return hosts, nil
}

// checkSameOrigin rejects a write that did not come from this board's own page.
// The Host must be one this server answers to (anti-DNS-rebinding); a browser
// Origin, when present, must be exactly http://<that host>; and a Sec-Fetch-Site
// other than same-origin/none is refused. A request without Origin (curl, tests)
// passes: browsers always send Origin on POST/PATCH, so CSRF cannot omit it.
func (s *Server) checkSameOrigin(r *http.Request) error {
	if len(s.writeHosts) == 0 {
		return errors.New("board writes are disabled on this server")
	}
	host := strings.ToLower(r.Host)
	if !s.writeHosts[host] {
		return fmt.Errorf("request host %q is not this board", r.Host)
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme != "http" || strings.ToLower(parsed.Host) != host {
			return fmt.Errorf("cross-origin write from %q refused", origin)
		}
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return errors.New("cross-site write refused")
	}
	return nil
}

// decodeWrite runs the shared guards of every write endpoint — method, same
// origin, JSON content type, bounded body, strict decoding — and reports whether
// the handler may continue (on false the error response is already written).
func (s *Server) decodeWrite(w http.ResponseWriter, r *http.Request, method string, dst any) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeJSONError(w, http.StatusMethodNotAllowed, fmt.Sprintf("method %s not allowed; use %s", r.Method, method))
		return false
	}
	if err := s.checkSameOrigin(r); err != nil {
		writeJSONError(w, http.StatusForbidden, err.Error())
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSONError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxWriteBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body: trailing data after the object")
		return false
	}
	return true
}

// writeStoreError maps a Store mutator error to its HTTP status: a missing
// spec/epic → 404, caller input → 400, a state clash → 409, anything else → 500.
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		writeJSONError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, state.ErrInvalidInput):
		writeJSONError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, state.ErrConflict):
		writeJSONError(w, http.StatusConflict, err.Error())
	default:
		writeJSONError(w, http.StatusInternalServerError, err.Error())
	}
}

// writeJSON writes a JSON success body and pushes the fresh board to every SSE
// client, so the write is visible immediately rather than on the next watcher
// poll (the watcher still fires afterwards; a duplicate board push is harmless).
func (s *Server) writeJSON(w http.ResponseWriter, status int, body any) {
	b, err := json.Marshal(body)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "could not encode response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	w.Write(b)
	s.Broadcast()
}

// focusRequest is the body of POST /api/specs/{id}/focus. Focus is a pointer so
// an absent field is a 400, not a silent "unfocus".
type focusRequest struct {
	Focus *bool `json:"focus"`
}

// FocusResponse is the result of POST /api/specs/{id}/focus.
type FocusResponse struct {
	ID      string `json:"id"`
	Focus   bool   `json:"focus"`
	Changed bool   `json:"changed"`
}

// handleSpecFocus toggles a spec's focus marker (POST /api/specs/{id}/focus).
func (s *Server) handleSpecFocus(w http.ResponseWriter, r *http.Request) {
	var req focusRequest
	if !s.decodeWrite(w, r, http.MethodPost, &req) {
		return
	}
	if req.Focus == nil {
		writeJSONError(w, http.StatusBadRequest, `missing "focus" (true|false)`)
		return
	}
	id := r.PathValue("id")
	changed, err := s.writer.SetFocus(id, *req.Focus, s.actor, time.Now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, FocusResponse{ID: id, Focus: *req.Focus, Changed: changed})
}

// epicAssignRequest is the body of POST /api/specs/{id}/epic. Epic is a
// json.RawMessage so the three cases stay distinct: a string assigns, null (or
// "") clears, an absent field is a 400.
type epicAssignRequest struct {
	Epic json.RawMessage `json:"epic"`
}

// EpicAssignResponse is the result of POST /api/specs/{id}/epic ("" = no epic).
type EpicAssignResponse struct {
	ID      string `json:"id"`
	Epic    string `json:"epic"`
	Changed bool   `json:"changed"`
}

// handleSpecEpic assigns or clears a spec's epic (POST /api/specs/{id}/epic).
func (s *Server) handleSpecEpic(w http.ResponseWriter, r *http.Request) {
	var req epicAssignRequest
	if !s.decodeWrite(w, r, http.MethodPost, &req) {
		return
	}
	if len(req.Epic) == 0 {
		writeJSONError(w, http.StatusBadRequest, `missing "epic" (an epic id, or null to clear)`)
		return
	}
	var epicID string
	if string(req.Epic) != "null" {
		if err := json.Unmarshal(req.Epic, &epicID); err != nil {
			writeJSONError(w, http.StatusBadRequest, `"epic" must be a string or null`)
			return
		}
	}
	id := r.PathValue("id")
	changed, err := s.writer.AssignEpic(id, epicID, s.actor, time.Now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, EpicAssignResponse{ID: id, Epic: strings.TrimSpace(epicID), Changed: changed})
}

// epicCreateRequest is the body of POST /api/epics.
type epicCreateRequest struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Order       int    `json:"order"`
}

// handleEpicCreate creates an epic (POST /api/epics) → 201 with the epic.
func (s *Server) handleEpicCreate(w http.ResponseWriter, r *http.Request) {
	var req epicCreateRequest
	if !s.decodeWrite(w, r, http.MethodPost, &req) {
		return
	}
	epic, err := s.writer.CreateEpic(state.CreateEpicParams{
		ID:          req.ID,
		Title:       req.Title,
		Description: req.Description,
		Color:       state.EpicColor(req.Color),
		Order:       req.Order,
		Actor:       s.actor,
		Now:         time.Now(),
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, epic)
}

// epicUpdateRequest is the body of PATCH /api/epics/{id}; absent fields are left
// untouched, "" clears description/color, order 0 unsets the order.
type epicUpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	Order       *int    `json:"order"`
}

// handleEpicUpdate edits an epic's title/description/color (PATCH /api/epics/{id}).
func (s *Server) handleEpicUpdate(w http.ResponseWriter, r *http.Request) {
	var req epicUpdateRequest
	if !s.decodeWrite(w, r, http.MethodPatch, &req) {
		return
	}
	params := state.UpdateEpicParams{Title: req.Title, Description: req.Description, Order: req.Order}
	if req.Color != nil {
		color := state.EpicColor(*req.Color)
		params.Color = &color
	}
	epic, _, err := s.writer.UpdateEpic(r.PathValue("id"), params, s.actor, time.Now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, epic)
}

// EpicFocusResponse is the result of POST /api/epics/{id}/focus.
type EpicFocusResponse struct {
	ID      string `json:"id"`
	Focus   bool   `json:"focus"`
	Changed bool   `json:"changed"`
}

// handleEpicFocus toggles an epic's focus marker (POST /api/epics/{id}/focus),
// mirroring the spec focus endpoint. The epic's specs inherit it at projection
// time; none of them is rewritten.
func (s *Server) handleEpicFocus(w http.ResponseWriter, r *http.Request) {
	var req focusRequest
	if !s.decodeWrite(w, r, http.MethodPost, &req) {
		return
	}
	if req.Focus == nil {
		writeJSONError(w, http.StatusBadRequest, `missing "focus" (true|false)`)
		return
	}
	id := r.PathValue("id")
	changed, err := s.writer.SetEpicFocus(id, *req.Focus, s.actor, time.Now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, EpicFocusResponse{ID: id, Focus: *req.Focus, Changed: changed})
}
