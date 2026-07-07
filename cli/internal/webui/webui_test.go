package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// embeddedIndexHTML returns the bytes of the embedded dist/index.html so tests
// can construct exact-match and differ scenarios without hardcoding content.
func embeddedIndexHTML(t *testing.T) []byte {
	t.Helper()
	b, err := embedded.ReadFile("dist/index.html")
	if err != nil {
		t.Fatalf("read embedded dist/index.html: %v", err)
	}
	return b
}

func TestResolve(t *testing.T) {
	embeddedBytes := embeddedIndexHTML(t)

	// A small content that is guaranteed different from the embedded placeholder.
	differentContent := []byte(`<!doctype html><html><head><title>Fresh build</title><script src="/assets/main-abc123.js"></script></head><body></body></html>`)
	// Sanity-check: the different content must actually differ from embedded.
	if string(differentContent) == string(embeddedBytes) {
		t.Fatal("test setup error: differentContent equals embedded bytes")
	}

	// writeDist creates a temp repoRoot with web/dist/index.html containing content.
	writeDist := func(t *testing.T, content []byte) string {
		t.Helper()
		repoRoot := t.TempDir()
		distDir := filepath.Join(repoRoot, "web", "dist")
		if err := os.MkdirAll(distDir, 0o755); err != nil {
			t.Fatalf("mkdir web/dist: %v", err)
		}
		if err := os.WriteFile(filepath.Join(distDir, "index.html"), content, 0o644); err != nil {
			t.Fatalf("write index.html: %v", err)
		}
		return repoRoot
	}

	t.Run("explicit dir wins regardless of allowDevDir", func(t *testing.T) {
		explicitDir := t.TempDir()
		// Write a minimal index.html so Handler does not error.
		if err := os.WriteFile(filepath.Join(explicitDir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
			t.Fatal(err)
		}

		for _, allowDev := range []bool{false, true} {
			handler, source, err := Resolve(explicitDir, "", allowDev)
			if err != nil {
				t.Fatalf("allowDevDir=%v: unexpected error: %v", allowDev, err)
			}
			if handler == nil {
				t.Fatalf("allowDevDir=%v: handler is nil", allowDev)
			}
			if source != explicitDir {
				t.Errorf("allowDevDir=%v: source = %q, want %q", allowDev, source, explicitDir)
			}
		}
	})

	t.Run("allowDevDir=false with existing web/dist uses embedded", func(t *testing.T) {
		repoRoot := writeDist(t, differentContent)

		handler, source, err := Resolve("", repoRoot, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if handler == nil {
			t.Fatal("handler is nil")
		}
		if source != "embedded" {
			t.Errorf("source = %q, want %q", source, "embedded")
		}
	})

	t.Run("allowDevDir=true, fresh web/dist with different bytes → stale notice", func(t *testing.T) {
		repoRoot := writeDist(t, differentContent)

		handler, source, err := Resolve("", repoRoot, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if handler == nil {
			t.Fatal("handler is nil")
		}
		candidateDir := filepath.Join(repoRoot, "web", "dist")
		if !strings.HasPrefix(source, candidateDir) {
			t.Errorf("source = %q, want prefix %q", source, candidateDir)
		}
		if !strings.Contains(source, "stale") {
			t.Errorf("source = %q, want to contain %q", source, "stale")
		}
	})

	t.Run("allowDevDir=true, web/dist identical to embedded → embedded", func(t *testing.T) {
		repoRoot := writeDist(t, embeddedBytes)

		handler, source, err := Resolve("", repoRoot, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if handler == nil {
			t.Fatal("handler is nil")
		}
		if source != "embedded" {
			t.Errorf("source = %q, want %q", source, "embedded")
		}
	})

	t.Run("allowDevDir=true, no web/dist → embedded", func(t *testing.T) {
		repoRoot := t.TempDir() // no web/dist written

		handler, source, err := Resolve("", repoRoot, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if handler == nil {
			t.Fatal("handler is nil")
		}
		if source != "embedded" {
			t.Errorf("source = %q, want %q", source, "embedded")
		}
	})
}

// TestValidateAssetsDetectsMissing verifies ValidateAssets flags a referenced asset
// that is absent from the filesystem (the mechanism behind the serve-startup guard).
func TestValidateAssetsDetectsMissing(t *testing.T) {
	dir := t.TempDir()
	// index.html references an asset that does not exist on disk.
	if err := os.WriteFile(filepath.Join(dir, "index.html"),
		[]byte(`<script src="/assets/index-DEADBEEF.js"></script>`), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := ValidateAssets(os.DirFS(dir))
	if len(missing) != 1 || !strings.Contains(missing[0], "index-DEADBEEF.js") {
		t.Fatalf("expected the missing asset to be flagged, got %v", missing)
	}

	// With the asset present, validation passes.
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "index-DEADBEEF.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if missing := ValidateAssets(os.DirFS(dir)); len(missing) != 0 {
		t.Fatalf("expected no missing assets, got %v", missing)
	}
}

// TestAssetPathNeverFallsBackToIndex verifies a missing /assets/* request returns a
// real 404 instead of the SPA index.html — so a broken embed fails loud in the
// browser console, not as HTML masquerading as JS with a 200.
func TestAssetPathNeverFallsBackToIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := Handler(dir)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/assets/index-MISSING.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing asset status = %d, want 404 (no SPA fallback)", rec.Code)
	}

	// An app route (no /assets/ prefix) still falls back to index.html.
	req = httptest.NewRequest("GET", "/some/app/route", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("app route should serve index.html; status=%d body=%q", rec.Code, rec.Body.String())
	}
}
