package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMigratesFromProjectStructure(t *testing.T) {
	root := t.TempDir()
	ps := "# comment\nspec-path: code/[branch]/docs/specs/<slug>/\nspec-filename: spec.md\nrun:\n  - name: web\n    cmd: pnpm dev\n"
	if err := os.WriteFile(filepath.Join(root, ".project-structure"), []byte(ps), 0o644); err != nil {
		t.Fatal(err)
	}

	c := Resolve(root)
	if c.SpecPath != "code/[branch]/docs/specs/<slug>/" {
		t.Fatalf("SpecPath = %q", c.SpecPath)
	}
	if c.SpecFilename != "spec.md" {
		t.Fatalf("SpecFilename = %q", c.SpecFilename)
	}
	if c.SpecStore != StoreConvention {
		t.Fatalf("SpecStore = %q", c.SpecStore)
	}
	if c.Source != SourceProjectStructure {
		t.Fatalf("Source = %q", c.Source)
	}
}

func TestResolveDetectsConvention(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}

	c := Resolve(root)
	if c.SpecPath != "docs/specs/<slug>/" || c.Source != SourceDetected {
		t.Fatalf("got SpecPath=%q Source=%q", c.SpecPath, c.Source)
	}
}

func TestResolveFallsBackToVector(t *testing.T) {
	root := t.TempDir()

	c := Resolve(root)
	if c.SpecPath != VectorFallbackSpecPath {
		t.Fatalf("SpecPath = %q, want %q", c.SpecPath, VectorFallbackSpecPath)
	}
	if c.SpecStore != StoreVector || c.Source != SourceDefault {
		t.Fatalf("got SpecStore=%q Source=%q", c.SpecStore, c.Source)
	}
}

func TestProjectStructureWinsOverDetection(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".project-structure"), []byte("spec-path: openspec/changes/<slug>/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := Resolve(root)
	if c.Source != SourceProjectStructure || c.SpecPath != "openspec/changes/<slug>/" {
		t.Fatalf(".project-structure should win: got Source=%q SpecPath=%q", c.Source, c.SpecPath)
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := Resolve(root)
	if err := Write(root, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !Exists(root) {
		t.Fatal("Exists = false after Write")
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if *got != *want {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}
