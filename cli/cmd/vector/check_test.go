package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mariocampbell/vector/internal/check"
	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/state"
)

func seedCheckRepo(t *testing.T, withConfig bool) string {
	t.Helper()
	root := t.TempDir()
	if withConfig {
		cfg := &config.Config{
			SchemaVersion: config.SchemaVersion,
			SpecPath:      config.VectorFallbackSpecPath,
			SpecFilename:  "spec.md",
			SpecStore:     config.StoreVector,
			Source:        config.SourceDefault,
			Language:      "es",
		}
		if err := config.Write(root, cfg); err != nil {
			t.Fatal(err)
		}
	}
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "add-auth", Title: "Add auth", Status: state.StatusReview}); err != nil {
		t.Fatal(err)
	}
	return root
}

func decodeCheckReport(t *testing.T, out string) check.CheckReport {
	t.Helper()
	var report check.CheckReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse vector check --json: %v\n%s", err, out)
	}
	return report
}

// snapshotVector returns every file under .vector/ with its contents, to prove
// `vector check` never writes the store.
func snapshotVector(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join(root, ".vector"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		files[path] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestCheckJSONReport(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no git, no gh: remote checks degrade
	root := seedCheckRepo(t, true)
	before := snapshotVector(t, root)

	out, err := execCmd(t, newCheckCmd, "--repo-root", root)
	if err != nil {
		t.Fatalf("vector check: %v", err)
	}
	report := decodeCheckReport(t, out)
	if report.Language != "es" {
		t.Errorf("language = %q, want es", report.Language)
	}
	if report.Discrepancies == nil || report.Specs == nil || report.Notes == nil {
		t.Errorf("arrays must serialize as [], got %+v", report)
	}

	after := snapshotVector(t, root)
	if len(before) != len(after) {
		t.Fatalf(".vector file count changed: %d → %d", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("vector check modified %s", path)
		}
	}
}

func TestCheckWithoutConfigRunsLocalOnly(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := seedCheckRepo(t, false)

	out, err := execCmd(t, newCheckCmd, "--repo-root", root)
	if err != nil {
		t.Fatalf("vector check without config: %v", err)
	}
	report := decodeCheckReport(t, out)
	if report.Language != "" {
		t.Errorf("language = %q, want empty without config", report.Language)
	}
	if len(report.Discrepancies) != 0 {
		t.Errorf("without config only local checks run, got %+v", report.Discrepancies)
	}
}

func TestCheckInvalidConfigFails(t *testing.T) {
	root := seedCheckRepo(t, false)
	if err := os.WriteFile(config.Path(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := execCmd(t, newCheckCmd, "--repo-root", root); err == nil {
		t.Fatal("a present-but-invalid config must fail, not degrade")
	}
}

func TestCheckWithoutGhDegrades(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := seedCheckRepo(t, true)

	out, err := execCmd(t, newCheckCmd, "--repo-root", root, "--json=false")
	if err != nil {
		t.Fatalf("vector check --json=false without gh: %v", err)
	}
	if out == "" {
		t.Error("expected the human report on stdout")
	}
}
