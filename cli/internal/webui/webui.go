// Package webui embeds the built web/ panel and serves it as a single-page app.
// The canonical install path (make install / scripts/dev-install.sh) builds web/
// into ./dist (Vite outDir) and re-embeds it before the binary is compiled, so the
// panel ships inside the binary with no Node runtime
// (architecture/distribution-packaging.md). The real, hashed dist/index.html is a
// build product and is gitignored; a separate committed dist/index.placeholder.html
// (which references no /assets/*) is embedded instead when web/ has not been built,
// so a clean checkout embeds a valid, integrity-checkable board and serves an honest
// "board not built" page rather than a blank one.
package webui

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// assetRefRe extracts the hashed asset paths (/assets/index-XXXX.js|css) that
// index.html loads. It underpins ValidateAssets — the guard against shipping a
// binary whose embedded index references assets that were never built into the
// embed (the classic "blank board from a fresh worktree" break: dist/assets is
// gitignored, so `go build` in a worktree with no web build embeds only index.html).
var assetRefRe = regexp.MustCompile(`/assets/[A-Za-z0-9._-]+\.(?:js|css)`)

// ValidateAssets reports the /assets/* paths that index.html references but that are
// absent from fsys — i.e. the frontend the binary would serve is broken (the browser
// would receive the SPA index.html in place of the missing JS/CSS and render blank).
// An empty result means the embed is internally consistent. A missing/unreadable
// index.html returns nil (nothing to validate — the caller decides if that matters).
func ValidateAssets(fsys fs.FS) []string {
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil
	}
	var missing []string
	seen := map[string]bool{}
	for _, ref := range assetRefRe.FindAllString(string(index), -1) {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		if _, err := fs.Stat(fsys, strings.TrimPrefix(ref, "/")); err != nil {
			missing = append(missing, ref)
		}
	}
	return missing
}

// EmbeddedBoardBuilt reports whether the embed carries a real web build (a
// dist/index.html produced by `web build` and re-embedded) rather than only the
// tracked dist/index.placeholder.html. `vector serve` uses it to tell the honest
// "never built" state apart from a working board, so the not-built case gets its
// own notice instead of passing silently through the asset-integrity guard (which
// has nothing to validate when index.html is absent).
func EmbeddedBoardBuilt() bool {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return false
	}
	_, err = fs.Stat(sub, "index.html")
	return err == nil
}

// EmbeddedAssetsMissing runs ValidateAssets against the embedded dist — the guard
// `vector serve` calls at startup so a binary built without a web build fails LOUD
// (a warning to rebuild web) instead of silently serving a blank board.
func EmbeddedAssetsMissing() []string {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil
	}
	return ValidateAssets(sub)
}

// Handler serves the panel. When dir is non-empty it serves from that directory
// on disk (development: a freshly built web/dist without recompiling the binary);
// otherwise it serves the embedded assets. Unknown paths fall back to index.html
// so client-side routing works.
func Handler(dir string) (http.Handler, error) {
	if dir != "" {
		return spaHandler{fsys: os.DirFS(dir)}, nil
	}
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	return spaHandler{fsys: sub}, nil
}

// Resolve picks the panel source and serves it. An explicit dir (from --web-dir)
// always wins. Otherwise, when allowDevDir is true (the binary is a dev build) and
// a freshly built <repoRoot>/web/dist exists whose index.html differs from the
// embedded one, it serves web/dist so `vector serve` reflects the latest frontend
// without a recompile. Otherwise it serves the embedded build. The returned source
// string is a short human label for logging ("embedded", the dir path, or the dir
// path annotated as stale).
func Resolve(explicitDir, repoRoot string, allowDevDir bool) (handler http.Handler, source string, err error) {
	if explicitDir != "" {
		h, herr := Handler(explicitDir)
		if herr != nil {
			return nil, "", herr
		}
		return h, explicitDir, nil
	}

	if allowDevDir {
		candidate := filepath.Join(repoRoot, "web", "dist")
		candidateIndex := filepath.Join(candidate, "index.html")
		diskBytes, readErr := os.ReadFile(candidateIndex)
		if readErr == nil {
			embeddedBytes, embErr := embedded.ReadFile("dist/index.html")
			if embErr == nil && !bytes.Equal(diskBytes, embeddedBytes) {
				h, herr := Handler(candidate)
				if herr != nil {
					return nil, "", herr
				}
				return h, candidate + " (embedded UI is stale)", nil
			}
		}
		// Any read/stat error or identical bytes → fall through to embedded.
	}

	h, herr := Handler("")
	if herr != nil {
		return nil, "", herr
	}
	return h, "embedded", nil
}

type spaHandler struct {
	fsys fs.FS
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	f, err := h.fsys.Open(name)
	if err != nil {
		// A missing static asset must fail LOUD (real 404), never fall back to
		// index.html: serving HTML where the browser requested /assets/*.js renders
		// the board blank with a 200 and no error — the exact silent break a
		// stale/empty embed causes. SPA fallback is only for app routes (no /assets/
		// prefix), so client-side routing still works.
		if strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)
			return
		}
		// Unknown app path → SPA entry point (client router resolves it).
		h.serveIndex(w, r)
		return
	}
	f.Close()
	if name == "index.html" {
		// Route the entry point through serveIndex so the placeholder fallback
		// also applies to a bare "/" (and explicit /index.html) request.
		h.serveIndex(w, r)
		return
	}
	h.serveFile(w, r, name)
}

// serveIndex serves the SPA entry point, with a placeholder fallback. When a real
// index.html is present in the embed (a proper web build was re-embedded) it is
// served as-is. When it is absent — a binary built from a worktree that never ran
// `web build`, whose embed carries only the tracked index.placeholder.html — the
// placeholder is served (200, text/html) so "/" shows an honest "board not built"
// page instead of a blank board or a confusing 404. This path is never reached for
// /assets/* (those 404 above), preserving the loud-failure guarantee for missing JS/CSS.
func (h spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	if f, err := h.fsys.Open("index.html"); err == nil {
		f.Close()
		h.serveFile(w, r, "index.html")
		return
	}
	h.serveFile(w, r, "index.placeholder.html")
}

func (h spaHandler) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	f, err := h.fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "asset not seekable", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, name, info.ModTime(), rs)
}
