package board

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/mariocampbell/vector/internal/standup"
)

// Server exposes the board as a read-only HTTP API with a live SSE stream. It is
// the cli/ → web/ contract: web/ consumes these endpoints and owns no canonical
// state (architecture/state-model.md).
type Server struct {
	src  Source
	repo string

	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

// NewServer builds a Server over a board source (a *state.Store) labelled by repo.
func NewServer(src Source, repo string) *Server {
	return &Server{src: src, repo: repo, clients: make(map[chan []byte]struct{})}
}

// Routes returns the API mux with the panel served by static for everything else.
func (s *Server) Routes(static http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/board", s.handleBoard)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/standup", s.handleStandup)
	mux.HandleFunc("/api/activity", s.handleActivity)
	mux.HandleFunc("/api/summary", s.handleSummary)
	mux.HandleFunc("/api/file", s.handleFile)
	if static != nil {
		mux.Handle("/", static)
	}
	return mux
}

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	b, err := s.render()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

// handleStandup serves the persisted standup digest (GET /api/standup). When no
// digest has been committed yet it returns {} (200), never 500.
func (s *Server) handleStandup(w http.ResponseWriter, r *http.Request) {
	digest, err := s.src.ReadStandup()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "could not read standup digest")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	// A never-run standup has no generatedAt; serialize {} so the client shows empty.
	if digest == nil || digest.GeneratedAt.IsZero() {
		w.Write([]byte("{}"))
		return
	}
	b, err := json.Marshal(digest)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "could not encode standup digest")
		return
	}
	w.Write(b)
}

// handleActivity serves a spec's projected timeline (GET
// /api/activity?spec=<id>&since=<dur>). Read-only: 400 on an invalid since, 404
// on an unknown spec, 500 on a log read error.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	specID := r.URL.Query().Get("spec")
	if specID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing spec query parameter")
		return
	}
	window := r.URL.Query().Get("since")
	if window == "" {
		window = "24h"
	}
	from, err := standup.ParseSince(window, time.Now())
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	specs, err := s.src.ListSpecs()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "could not read specs")
		return
	}
	found := false
	for _, sp := range specs {
		if sp.ID == specID {
			found = true
			break
		}
	}
	if !found {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("spec %q not found", specID))
		return
	}

	events, err := s.src.ReadEvents()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "could not read activity log")
		return
	}
	timeline := standup.Timeline(events, specID, from)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	b, err := json.Marshal(struct {
		Spec   string                  `json:"spec"`
		Since  string                  `json:"since"`
		Events []standup.TimelineEvent `json:"events"`
	}{Spec: specID, Since: window, Events: timeline})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "could not encode activity")
		return
	}
	w.Write(b)
}

// handleSummary serves a spec's persisted post-action summary (GET
// /api/summary?spec=<id>). Read-only: 400 on a missing spec param, {} (200) when
// no summary has been generated yet, 500 on a read error. It does not 404 on an
// unknown spec — an absent summary and an unknown spec both surface as {}.
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	specID := r.URL.Query().Get("spec")
	if specID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing spec query parameter")
		return
	}
	summary, err := s.src.ReadSummary(specID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "could not read summary")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if summary == nil {
		w.Write([]byte("{}"))
		return
	}
	b, err := json.Marshal(summary)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "could not encode summary")
		return
	}
	w.Write(b)
}

// handleFile serves a spec's source document as raw Markdown (GET
// /api/file?spec=<id>&artifact=<key>). The client sends a spec id and an
// artifact enum — never a path — so traversal is removed by design; path
// resolution + I/O live in state.ReadSpecArtifact. Mapping: missing spec or
// unknown/missing artifact → 400; absent spec/artifact/file → 404; read error →
// 500; success → raw bytes as text/markdown (no JSON envelope).
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	specID := r.URL.Query().Get("spec")
	if specID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing spec query parameter")
		return
	}
	artifact := r.URL.Query().Get("artifact")
	if !validArtifact(artifact) {
		writeJSONError(w, http.StatusBadRequest, "missing or unknown artifact query parameter")
		return
	}
	b, err := s.src.ReadSpecArtifact(specID, artifact)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeJSONError(w, http.StatusNotFound, fmt.Sprintf("artifact %q for spec %q not found", artifact, specID))
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "could not read artifact")
		return
	}
	if artifact == "sketch" {
		// A sketch is a binary design artifact, served as a download rather than
		// inline Markdown. The stored file name is looked up from committed state;
		// the internal absolute path is never leaked in the header.
		w.Header().Set("Content-Type", "application/octet-stream")
		if name := s.sketchFileName(specID); name != "" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
		} else {
			w.Header().Set("Content-Disposition", "attachment")
		}
	} else {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

// sketchFileName resolves the first sketch's stored file name for a spec (V1 serves
// spec.Sketches[0]), for the download's Content-Disposition. It reads committed
// state through the board Source; "" when the spec has no sketch or is unknown.
func (s *Server) sketchFileName(specID string) string {
	specs, err := s.src.ListSpecs()
	if err != nil {
		return ""
	}
	for _, sp := range specs {
		if sp.ID == specID && len(sp.Sketches) > 0 {
			return sp.Sketches[0].Name
		}
	}
	return ""
}

// validArtifact gates the artifact query parameter to the known enum.
func validArtifact(artifact string) bool {
	switch artifact {
	case "spec", "proposal", "design", "tasks", "sketch":
		return true
	}
	return false
}

// writeJSONError writes a {"error": msg} body with the given status, matching the
// shape the web hooks parse (GET-only local API: 400/404/500).
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	b, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: msg})
	w.Write(b)
}

// handleEvents streams the board to a client over Server-Sent Events: the current
// board immediately, then a fresh board whenever Broadcast fires.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 1)
	s.subscribe(ch)
	defer s.unsubscribe(ch)

	if b, err := s.render(); err == nil {
		writeSSE(w, b)
		flusher.Flush()
	}

	ctx := r.Context()
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-ch:
			writeSSE(w, b)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// Broadcast re-renders the board and pushes it to every connected SSE client.
// The caller (the file watcher) invokes this when on-disk state changes.
func (s *Server) Broadcast() {
	b, err := s.render()
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		// Drop the newest if a slow client's buffer is full — it gets the next one.
		select {
		case ch <- b:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- b:
			default:
			}
		}
	}
}

func (s *Server) render() ([]byte, error) {
	b, err := Build(s.src, s.repo, time.Now())
	if err != nil {
		return nil, err
	}
	return json.Marshal(b)
}

func (s *Server) subscribe(ch chan []byte) {
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
}

func (s *Server) unsubscribe(ch chan []byte) {
	s.mu.Lock()
	delete(s.clients, ch)
	s.mu.Unlock()
}

func writeSSE(w http.ResponseWriter, data []byte) {
	fmt.Fprint(w, "event: board\ndata: ")
	w.Write(data)
	fmt.Fprint(w, "\n\n")
}
