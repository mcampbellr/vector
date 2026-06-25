package board

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
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
