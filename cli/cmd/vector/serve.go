package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/mariocampbell/vector/internal/board"
	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
	"github.com/mariocampbell/vector/internal/ui"
	"github.com/mariocampbell/vector/internal/webui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// runServe starts the local board panel: the HTTP API (/api/board, /api/events
// SSE, and the same-origin write endpoints for focus/epics) plus the embedded web
// UI. It is an ephemeral local server —
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
			store.SetArtifactFallbacks(artifactFallbacks(root))

			static, uiSource, err := webui.Resolve(webDir, root, strings.HasSuffix(version, "-dev"))
			if err != nil {
				return fmt.Errorf("init web ui: %w", err)
			}
			srv := board.NewServer(store, filepath.Base(root))
			httpServer := &http.Server{Handler: withCORS(srv.Routes(static))}

			addr := fmt.Sprintf("%s:%d", host, port)
			portFallback := false
			listener, listenErr := net.Listen("tcp", addr)
			if listenErr != nil {
				// If the user did not explicitly set --port and 8787 is in use, retry on a
				// free port; the banner warns that the Vite proxy will not reach it.
				if !portSet && errors.Is(listenErr, syscall.EADDRINUSE) {
					listener, err = net.Listen("tcp", fmt.Sprintf("%s:0", host))
					if err != nil {
						return fmt.Errorf("listen on %s:0 (fallback): %w", host, err)
					}
					portFallback = true
				} else {
					return fmt.Errorf("listen on %s: %w", addr, listenErr)
				}
			}

			// Writes are only accepted for the address actually bound (the fallback
			// port included), so the same-origin guard matches what the browser uses.
			if err := srv.EnableWrites(store, resolveActor(), listener.Addr().String()); err != nil {
				listener.Close()
				return fmt.Errorf("enable board writes: %w", err)
			}

			banner := serveBanner{
				root:          root,
				address:       listener.Addr().String(),
				uiSource:      uiSource,
				requestedPort: port,
				portFallback:  portFallback,
			}
			if uiSource == "embedded" {
				banner.boardNotBuilt = !webui.EmbeddedBoardBuilt()
				banner.missingEmbeddedAssets = webui.EmbeddedAssetsMissing()
			}
			return runServeLoop(root, httpServer, listener, banner, pollMs, srv.Broadcast)
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

// artifactFallbacks lists the OpenSpec changes roots the file preview searches
// when a spec's recorded artifact path is gone (typically a removed per-spec
// worktree): the configured branch's worktree first, then every other worktree,
// then the root-level changes tree that holds the archive. The roots are globbed
// per lookup so worktrees added while the board runs are found. A missing or
// invalid config only disables the fallbacks — the board still serves recorded paths.
func artifactFallbacks(root string) state.ArtifactFallbacks {
	cfg, err := config.Load(root)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: file preview fallbacks disabled: %v\n", err)
		}
		return state.ArtifactFallbacks{}
	}
	return state.ArtifactFallbacks{ChangesDirs: func() []string { return orderedChangesDirs(cfg, root) }}
}

// orderedChangesDirs returns cfg's changes roots ranked for fallback lookups:
// configured branch, other worktrees, then the root-level tree. A glob error
// yields no roots (the lookup degrades to 404, never 500).
func orderedChangesDirs(cfg *config.Config, root string) []string {
	dirs, err := cfg.ChangesDirs(root)
	if err != nil {
		return nil
	}
	rootChangesDir := filepath.Join(root, filepath.FromSlash(config.DefaultChangesPath))
	// Compare resolved directories rather than ChangesDir.Branch: the branch capture
	// is empty for templates with a trailing slash (e.g. "code/[branch]/…/changes/").
	branchChangesDir := ""
	if cfg.Branch != "" {
		template := cfg.ChangesPath
		if template == "" {
			template = config.DefaultChangesPath
		}
		branchChangesDir = filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(template, "[branch]", cfg.Branch)))
	}
	rank := func(dir config.ChangesDir) int {
		switch {
		case branchChangesDir != "" && dir.Dir == branchChangesDir:
			return 0
		case dir.Dir == rootChangesDir:
			return 2
		default:
			return 1
		}
	}
	sort.SliceStable(dirs, func(i, j int) bool { return rank(dirs[i]) < rank(dirs[j]) })
	changesDirs := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		changesDirs = append(changesDirs, dir.Dir)
	}
	return changesDirs
}

// serveShutdownGracePeriod bounds the graceful Shutdown after Ctrl-C. Request
// contexts are already canceled by then (BaseContext), so SSE handlers return at
// once; the grace only covers in-flight plain requests before Close drops them.
const serveShutdownGracePeriod = 500 * time.Millisecond

// runServeLoop wires the signal-driven shutdown and the state watcher around the
// bound listener, then blocks until Ctrl-C or a serve error. Split out of the
// factory so the RunE stays a thin flag adapter.
func runServeLoop(root string, httpServer *http.Server, listener net.Listener, banner serveBanner, pollMs int, broadcast func()) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// After the first signal, restore the default handlers so a second Ctrl-C
	// terminates the process immediately instead of waiting on the shutdown.
	go func() {
		<-ctx.Done()
		stop()
	}()

	go watchState(ctx, root, time.Duration(pollMs)*time.Millisecond, broadcast)

	printServeBanner(os.Stdout, os.Stderr, banner)

	stoppedBySignal, err := serveUntilCanceled(ctx, httpServer, listener, serveShutdownGracePeriod)
	if err != nil {
		return err
	}
	if stoppedBySignal {
		printServeStopped(os.Stdout, isTerminal(os.Stdout))
	}
	return nil
}

// serveUntilCanceled serves on listener until ctx is canceled or Serve fails. Every
// request context derives from ctx (http.Server.BaseContext), so cancellation ends
// long-lived handlers such as the /api/events SSE stream right away. It then runs a
// Shutdown bounded by gracePeriod and falls back to Close if connections are still
// open. stoppedBySignal reports whether ctx cancellation ended the loop.
func serveUntilCanceled(ctx context.Context, httpServer *http.Server, listener net.Listener, gracePeriod time.Duration) (stoppedBySignal bool, err error) {
	httpServer.BaseContext = func(net.Listener) context.Context { return ctx }

	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- httpServer.Serve(listener) }()

	select {
	case serveErr := <-serveErrCh:
		if errors.Is(serveErr, http.ErrServerClosed) {
			return false, nil
		}
		return false, serveErr
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracePeriod)
	defer cancel()
	if shutdownErr := httpServer.Shutdown(shutdownCtx); shutdownErr != nil {
		// The grace period elapsed with connections still active: drop them. The
		// process is exiting either way, so a Close error is not actionable.
		_ = httpServer.Close()
	}
	return true, nil
}

// serveBanner is everything the startup banner reports about a running panel.
type serveBanner struct {
	root                  string
	address               string
	uiSource              string
	requestedPort         int
	portFallback          bool
	boardNotBuilt         bool
	missingEmbeddedAssets []string
}

// printServeBanner writes the human startup banner: header and URL block to
// stdout, warnings and notes to stderr (the same stream routing as before the
// restyle). Styling comes from internal/ui, which degrades to plain text when the
// output is not a terminal or NO_COLOR is set.
func printServeBanner(stdout, stderr io.Writer, banner serveBanner) {
	url := "http://" + banner.address

	fmt.Fprintf(stdout, "%s %s %s\n", ui.Title("vector serve"), ui.Dim("·"), ui.Bold(filepath.Base(banner.root)))
	fmt.Fprintf(stdout, "  %s\n\n", ui.Dim(banner.root))
	fmt.Fprintln(stdout, ui.KeyValue("board", ui.Bold(url)))
	fmt.Fprintln(stdout, ui.KeyValue("api", url+"/api/board"))
	fmt.Fprintln(stdout, ui.KeyValue("events", url+"/api/events "+ui.Dim("(SSE)")))
	if banner.uiSource != "embedded" {
		fmt.Fprintln(stdout, ui.KeyValue("ui", banner.uiSource))
	}
	fmt.Fprintln(stdout)

	if banner.portFallback {
		fmt.Fprintln(stderr, ui.WarningBlock(
			fmt.Sprintf("port %d is in use — serving on %s instead", banner.requestedPort, banner.address),
			fmt.Sprintf("The Vite dev proxy (which targets %d) will NOT reach this instance.", banner.requestedPort),
			fmt.Sprintf("Free port %d and restart, or pass --port %d, or set VECTOR_API for Vite.", banner.requestedPort, banner.requestedPort),
		))
		fmt.Fprintln(stderr)
	}
	if strings.Contains(banner.uiSource, "stale") {
		fmt.Fprintln(stderr, ui.WarningBlock(
			"serving web/dist from disk (embedded UI is stale)",
			"Re-embed and recompile to bake it into the binary.",
		))
		fmt.Fprintln(stderr)
	}
	if banner.boardNotBuilt {
		// Placeholder-only embed: this binary was compiled without a web build, so
		// there is no board to serve. The placeholder page says so in the browser;
		// say it here too, with the one command that fixes it.
		fmt.Fprintln(stderr, ui.WarningBlock(
			"this binary embeds no web board — it was compiled without a web build",
			"The board shows a \"not built\" page. Reinstall through the canonical path, which",
			"always rebuilds + re-embeds the panel before compiling:",
			"  "+ui.Cyan("make install")+ui.Dim("   # or: scripts/dev-install.sh"),
			"then restart this server. (See .claude/rules/architecture/distribution-packaging.md.)",
		))
		fmt.Fprintln(stderr)
	}
	if len(banner.missingEmbeddedAssets) > 0 {
		// The embed is broken: index.html references assets that are not in the
		// binary (a partial/stale re-embed — dist/assets is gitignored). Warn LOUD
		// instead of serving a blank board silently.
		details := make([]string, 0, len(banner.missingEmbeddedAssets)+4)
		for _, ref := range banner.missingEmbeddedAssets {
			details = append(details, "  "+ui.Dim(ref))
		}
		details = append(details,
			"The board will render "+ui.Bold("BLANK")+". Reinstall through the canonical path, which",
			"rebuilds the web panel, re-embeds it and runs the integrity guard:",
			"  "+ui.Cyan("make install")+ui.Dim("   # or: scripts/dev-install.sh"),
			"then restart this server. (See .claude/rules/architecture/distribution-packaging.md.)",
		)
		fmt.Fprintln(stderr, ui.WarningBlock(
			"WARNING: the embedded board is broken — index.html references assets not in this binary:",
			details...,
		))
		fmt.Fprintln(stderr)
	}

	fmt.Fprintf(stdout, "  %s\n", ui.Dim("Ctrl+C to stop"))
}

// printServeStopped writes the stop confirmation. On a terminal it starts with a
// carriage return so the line overwrites the "^C" the tty echoes for Ctrl-C;
// piped output gets no control characters.
func printServeStopped(stdout io.Writer, onTerminal bool) {
	prefix := ""
	if onTerminal {
		prefix = "\r"
	}
	fmt.Fprintf(stdout, "%s%s\n", prefix, ui.Success("stopped"))
}

// isTerminal reports whether file is an interactive terminal.
func isTerminal(file *os.File) bool {
	return isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
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

// withCORS allows the Vite dev server (a different origin) to call the read API
// during development. The server binds to localhost and is ephemeral, so this is
// safe for GETs. Writes are NOT opened to other origins: POST/PATCH with a JSON
// body need a preflight that this header set does not grant, and the write
// handlers additionally enforce same-origin (board.Server.checkSameOrigin). The
// Vite proxy rewrites Origin to the API target so dev writes pass that check.
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
