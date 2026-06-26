// Package webui embeds the built web/ panel and serves it as a single-page app.
// The release pipeline builds web/ into ./dist (Vite outDir) before the binary
// is compiled, so the panel ships inside the binary with no Node runtime
// (architecture/distribution-packaging.md). A committed placeholder index.html
// keeps the embed — and the build — valid before the first web build.
package webui

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

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
		// Unknown path → SPA entry point (client router resolves it).
		h.serveFile(w, r, "index.html")
		return
	}
	f.Close()
	h.serveFile(w, r, name)
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
