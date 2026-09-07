package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mariocampbell/vector/internal/board"
	"github.com/mariocampbell/vector/internal/state"
	"github.com/mariocampbell/vector/internal/webui"
	"github.com/spf13/cobra"
)

// runServe starts the local board panel: the read-only HTTP API (/api/board,
// /api/events SSE) plus the embedded web UI. It is an ephemeral local server —
// it runs only while the dev manages Vector. It binds 8787 by default (the port
// the Vite dev proxy targets), falling back to a free port if 8787 is taken
// unless --port was given explicitly (architecture/distribution-packaging.md).
func newServeCmd() *cobra.Command {
	var (
		port     int
		host     string
		webDir   string
		repoRoot string
		pollMs   int
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "start the local board panel (HTTP API + SSE + embedded web UI)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Detect whether the user explicitly set --port so we know whether to fall
			// back silently or hard-fail on a bind error.
			portSet := cmd.Flags().Changed("port")

			root, strays, err := resolveRepoRootStrays(repoRoot)
			if err != nil {
				return err
			}
			warnStrayStores(strays, root) // serve has no --json branch
			store, err := state.Open(root)
			if err != nil {
				return err
			}

			static, uiSource, err := webui.Resolve(webDir, root, strings.HasSuffix(version, "-dev"))
			if err != nil {
				return fmt.Errorf("init web ui: %w", err)
			}
			srv := board.NewServer(store, filepath.Base(root))
			httpServer := &http.Server{Handler: withCORS(srv.Routes(static))}

			addr := fmt.Sprintf("%s:%d", host, port)
			listener, listenErr := net.Listen("tcp", addr)
			if listenErr != nil {
				// If the user did not explicitly set --port and 8787 is in use, retry on a
				// free port and warn that the Vite proxy will not reach this instance.
				if !portSet && errors.Is(listenErr, syscall.EADDRINUSE) {
					listener, err = net.Listen("tcp", fmt.Sprintf("%s:0", host))
					if err != nil {
						return fmt.Errorf("listen on %s:0 (fallback): %w", host, err)
					}
					fmt.Fprintf(os.Stderr, "warning: port 8787 is in use; serving on %s instead.\n", listener.Addr())
					fmt.Fprintf(os.Stderr, "         The Vite dev proxy (which targets 8787) will NOT reach this instance.\n")
					fmt.Fprintf(os.Stderr, "         Free port 8787 and restart, or pass --port 8787, or set VECTOR_API for Vite.\n")
				} else {
					return fmt.Errorf("listen on %s: %w", addr, listenErr)
				}
			}

			return runServeLoop(root, httpServer, listener, uiSource, pollMs, srv.Broadcast)
		},
	}
	f := cmd.Flags()
	f.IntVar(&port, "port", 8787, "port to listen on (default 8787; 0 picks a free port)")
	f.StringVar(&host, "host", "127.0.0.1", "interface to bind")
	f.StringVar(&webDir, "web-dir", "", "serve the panel from this dir instead of the embedded build (dev)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.IntVar(&pollMs, "poll", 1000, "state poll interval in ms for live updates")
	return cmd
}

// runServeLoop wires the signal-driven shutdown and the state watcher around the
// bound listener, then blocks until Ctrl-C or a serve error. Split out of the
// factory so the RunE stays a thin flag adapter.
func runServeLoop(root string, httpServer *http.Server, listener net.Listener, uiSource string, pollMs int, broadcast func()) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go watchState(ctx, root, time.Duration(pollMs)*time.Millisecond, broadcast)

	url := fmt.Sprintf("http://%s", listener.Addr().String())
	fmt.Printf("vector serve: %s\n", root)
	fmt.Printf("  board:  %s\n", url)
	fmt.Printf("  api:    %s/api/board\n", url)
	fmt.Printf("  events: %s/api/events (SSE)\n", url)
	if strings.Contains(uiSource, "stale") {
		fmt.Printf("  ui:     %s\n", uiSource)
		fmt.Fprintf(os.Stderr, "note: serving web/dist from disk (embedded UI is stale); re-embed and recompile to bake it into the binary.\n")
	} else if uiSource != "embedded" {
		fmt.Printf("  ui:     %s\n", uiSource)
	} else if missing := webui.EmbeddedAssetsMissing(); len(missing) > 0 {
		// The embed is broken: index.html references assets that were never built
		// into the binary (built from a worktree with no web build — dist/assets is
		// gitignored). Warn LOUD instead of serving a blank board silently.
		fmt.Fprintf(os.Stderr, "\nWARNING: the embedded board is broken — index.html references assets not in this binary:\n")
		for _, ref := range missing {
			fmt.Fprintf(os.Stderr, "           %s\n", ref)
		}
		fmt.Fprintf(os.Stderr, "         The board will render BLANK. Rebuild the web panel and re-embed before reinstalling:\n")
		fmt.Fprintf(os.Stderr, "           npm --prefix web run build && rm -rf cli/internal/webui/dist/assets cli/internal/webui/dist/index.html && cp -R web/dist/. cli/internal/webui/dist/\n")
		fmt.Fprintf(os.Stderr, "         then rebuild the binary. (See .claude/rules/architecture/distribution-packaging.md.)\n\n")
	}
	fmt.Println("\nPress Ctrl-C to stop.")

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.Serve(listener) }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutCtx)
		fmt.Println("\nvector serve: stopped")
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// watchState polls the .vector tree's fingerprint and calls broadcast whenever it
// changes, so the board pushes live updates over SSE. Polling (stdlib only) avoids
// an fsnotify dependency — the tree is tiny and the interval is coarse.
func watchState(ctx context.Context, root string, interval time.Duration, broadcast func()) {
	dir := filepath.Join(root, ".vector")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	last := fingerprint(dir)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if fp := fingerprint(dir); fp != last {
				last = fp
				broadcast()
			}
		}
	}
}

// fingerprint summarizes the .vector tree as a count+size+latest-mtime signature.
// Cheap to compute and changes on any spec or activity-log write.
func fingerprint(dir string) string {
	var count int
	var totalSize int64
	var latest int64
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		count++
		totalSize += info.Size()
		if m := info.ModTime().UnixNano(); m > latest {
			latest = m
		}
		return nil
	})
	return fmt.Sprintf("%d:%d:%d", count, totalSize, latest)
}

// withCORS allows the Vite dev server (a different origin) to call the API during
// development. The server binds to localhost and is ephemeral, so this is safe.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
