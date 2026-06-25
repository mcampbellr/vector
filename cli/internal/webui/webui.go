// Package webui embeds the built web/ panel and serves it as a single-page app.
// The release pipeline builds web/ into ./dist (Vite outDir) before the binary
// is compiled, so the panel ships inside the binary with no Node runtime
// (architecture/distribution-packaging.md). A committed placeholder index.html
// keeps the embed — and the build — valid before the first web build.
package webui

import (
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
