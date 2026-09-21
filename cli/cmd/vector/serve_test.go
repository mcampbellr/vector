package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mariocampbell/vector/internal/config"
)

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
