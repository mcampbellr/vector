package main

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/board"
	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
)

// TestServeUntilCanceledReturnsQuicklyWithOpenSSEClient proves Ctrl-C does not wait
// on an open /api/events stream: the request context derives from the canceled
// serve context, so the SSE handler returns and Shutdown completes well inside
// the grace period (which is set long on purpose, so only BaseContext can make
// this pass — not the Close fallback).
func TestServeUntilCanceledReturnsQuicklyWithOpenSSEClient(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	httpServer := &http.Server{Handler: board.NewServer(store, "demo").Routes(nil)}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()

	type serveResult struct {
		stoppedBySignal bool
		err             error
	}
	resultCh := make(chan serveResult, 1)
	go func() {
		stoppedBySignal, serveErr := serveUntilCanceled(serveCtx, httpServer, listener, 5*time.Second)
		resultCh <- serveResult{stoppedBySignal: stoppedBySignal, err: serveErr}
	}()

	response, err := http.Get("http://" + listener.Addr().String() + "/api/events")
	if err != nil {
		t.Fatalf("open SSE stream: %v", err)
	}
	defer response.Body.Close()
	firstLine, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read first SSE line: %v", err)
	}
	if !strings.HasPrefix(firstLine, "event: board") {
		t.Fatalf("first SSE line = %q, want the board event frame (stream not established)", firstLine)
	}

	cancelStart := time.Now()
	cancelServe()

	select {
	case result := <-resultCh:
		elapsed := time.Since(cancelStart)
		if result.err != nil {
			t.Fatalf("serveUntilCanceled error: %v", result.err)
		}
		if !result.stoppedBySignal {
			t.Errorf("stoppedBySignal = false, want true after context cancellation")
		}
		if elapsed > 250*time.Millisecond {
			t.Errorf("shutdown with an open SSE client took %v, want < 250ms", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serveUntilCanceled did not return within 3s of cancellation")
	}
}

// TestPrintServeBannerRoutesAndContent asserts the banner keeps every piece of
// information and the stdout (header, URLs, hint) vs stderr (warnings) routing.
func TestPrintServeBannerRoutesAndContent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	printServeBanner(&stdout, &stderr, serveBanner{
		root:                  "/work/repos/cdr",
		address:               "127.0.0.1:36785",
		uiSource:              "embedded",
		requestedPort:         8787,
		portFallback:          true,
		missingEmbeddedAssets: []string{"/assets/index-abc.js"},
	})

	for _, want := range []string{
		"vector serve", "cdr", "/work/repos/cdr",
		"board", "http://127.0.0.1:36785",
		"api", "http://127.0.0.1:36785/api/board",
		"events", "http://127.0.0.1:36785/api/events", "(SSE)",
		"Ctrl+C to stop",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	for _, want := range []string{
		"port 8787 is in use", "serving on 127.0.0.1:36785 instead",
		"Vite dev proxy (which targets 8787) will NOT reach this instance",
		"pass --port 8787", "VECTOR_API",
		"embedded board is broken", "/assets/index-abc.js", "BLANK",
		"npm --prefix web run build", "distribution-packaging.md",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
	if strings.Contains(stdout.String(), "in use") || strings.Contains(stdout.String(), "broken") {
		t.Errorf("warnings leaked to stdout:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "  ui ") {
		t.Errorf("embedded UI must not print a ui line:\n%s", stdout.String())
	}
}

// TestPrintServeBannerStaleUI asserts the stale-UI source is listed on stdout and
// its note goes to stderr.
func TestPrintServeBannerStaleUI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	printServeBanner(&stdout, &stderr, serveBanner{
		root:          "/work/repos/cdr",
		address:       "127.0.0.1:8787",
		uiSource:      "/work/repos/cdr/web/dist (embedded UI is stale)",
		requestedPort: 8787,
	})
	if !strings.Contains(stdout.String(), "web/dist (embedded UI is stale)") {
		t.Errorf("stdout missing ui source:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "embedded UI is stale") || !strings.Contains(stderr.String(), "Re-embed and recompile") {
		t.Errorf("stderr missing stale note:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "in use") {
		t.Errorf("no port fallback happened, but stderr warns about it:\n%s", stderr.String())
	}
}

// TestPrintServeStopped asserts the carriage return that overwrites the tty's
// "^C" echo is emitted only on a terminal.
func TestPrintServeStopped(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		onTerminal bool
		wantPrefix bool
	}{
		{name: "terminal", onTerminal: true, wantPrefix: true},
		{name: "piped", onTerminal: false, wantPrefix: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var stdout bytes.Buffer
			printServeStopped(&stdout, testCase.onTerminal)
			out := stdout.String()
			if !strings.Contains(out, "stopped") {
				t.Errorf("output = %q, want it to contain %q", out, "stopped")
			}
			if got := strings.HasPrefix(out, "\r"); got != testCase.wantPrefix {
				t.Errorf("carriage-return prefix = %v, want %v (output %q)", got, testCase.wantPrefix, out)
			}
		})
	}
}

func TestOrderedChangesDirsRanksBranchWorktreesThenRoot(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"code/feature-a/openspec/changes",
		"code/main/openspec/changes",
		"code/zeta/openspec/changes",
		"openspec/changes",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
	}
	cfg := &config.Config{ChangesPath: "code/[branch]/openspec/changes/", Branch: "main"}

	got := orderedChangesDirs(cfg, root)
	want := []string{
		filepath.Join(root, "code", "main", "openspec", "changes"),
		filepath.Join(root, "code", "feature-a", "openspec", "changes"),
		filepath.Join(root, "code", "zeta", "openspec", "changes"),
		filepath.Join(root, "openspec", "changes"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("orderedChangesDirs =\n  %v\nwant\n  %v", got, want)
	}
}

func TestArtifactFallbacksWithoutConfigIsInert(t *testing.T) {
	fallbacks := artifactFallbacks(t.TempDir())
	if fallbacks.ChangesDirs != nil {
		t.Error("ChangesDirs set without a config, want nil (fallbacks disabled)")
	}
}
