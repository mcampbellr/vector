package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedStraySpec creates a stray store at dir holding one spec plus an activity
// line, the shape `doctor adopt` has to migrate.
func seedStraySpec(t *testing.T, dir, slug, ts string) string {
	t.Helper()
	stray := filepath.Join(dir, ".vector")
	specDir := filepath.Join(stray, "specs", slug)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "state.json"), []byte(`{"id":"`+slug+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	localDir := filepath.Join(stray, "local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"v":1,"ts":"` + ts + `","type":"spec.created","specId":"` + slug + `","actor":"tester"}`
	if err := os.WriteFile(filepath.Join(localDir, "activity.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "summaries.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return stray
}

func TestDoctorScanReportsStraysWithoutMutating(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	stray := seedStraySpec(t, filepath.Join(base, "website"), "orphan-spec", "2026-07-01T10:00:00Z")

	out, err := execCmd(t, newDoctorCmd, "--repo-root", base, "--json")
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	var found []strayStore
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		t.Fatalf("parse doctor json: %v (%s)", err, out)
	}
	if len(found) != 1 || found[0].Path != stray {
		t.Fatalf("found = %+v, want one stray at %s", found, stray)
	}
	if len(found[0].Specs) != 1 || found[0].Specs[0] != "orphan-spec" {
		t.Errorf("specs = %v, want [orphan-spec]", found[0].Specs)
	}
	if _, statErr := os.Stat(filepath.Join(stray, "specs", "orphan-spec")); statErr != nil {
		t.Error("scan must not move anything")
	}
}

func TestDoctorScanIgnoresTheCanonicalStore(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)

	out, err := execCmd(t, newDoctorCmd, "--repo-root", base, "--json")
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("want an empty report, got %s", out)
	}
}

func TestDoctorAdoptWithoutForceIsADryRun(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	stray := seedStraySpec(t, filepath.Join(base, "website"), "orphan-spec", "2026-07-01T10:00:00Z")

	out, err := execCmd(t, newDoctorAdoptCmd, stray, "--repo-root", base, "--json")
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	var result adoptResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse adopt json: %v (%s)", err, out)
	}
	if result.Applied || result.Removed {
		t.Errorf("dry run must not apply or remove: %+v", result)
	}
	if len(result.Specs) != 1 || result.Events != 1 {
		t.Errorf("plan = %+v, want 1 spec and 1 event", result)
	}
	if _, statErr := os.Stat(filepath.Join(stray, "specs", "orphan-spec")); statErr != nil {
		t.Error("dry run moved the spec")
	}
	if _, statErr := os.Stat(filepath.Join(base, ".vector", "specs", "orphan-spec")); statErr == nil {
		t.Error("dry run wrote into the canonical store")
	}
}

func TestDoctorAdoptForceMigratesAndRemovesStray(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	canonicalLocal := filepath.Join(base, ".vector", "local")
	if err := os.MkdirAll(canonicalLocal, 0o755); err != nil {
		t.Fatal(err)
	}
	// A later canonical event: the merge must land the stray's earlier line first.
	canonicalLine := `{"v":1,"ts":"2026-07-05T10:00:00Z","type":"spec.created","specId":"existing","actor":"tester"}`
	if err := os.WriteFile(filepath.Join(canonicalLocal, "activity.jsonl"), []byte(canonicalLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stray := seedStraySpec(t, filepath.Join(base, "website"), "orphan-spec", "2026-07-01T10:00:00Z")

	out, err := execCmd(t, newDoctorAdoptCmd, stray, "--repo-root", base, "--force", "--json")
	if err != nil {
		t.Fatalf("adopt --force: %v", err)
	}
	var result adoptResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse adopt json: %v (%s)", err, out)
	}
	if !result.Applied || !result.Removed {
		t.Fatalf("want an applied migration that removed the stray, got %+v", result)
	}
	if _, statErr := os.Stat(filepath.Join(base, ".vector", "specs", "orphan-spec", "state.json")); statErr != nil {
		t.Error("spec was not migrated into the canonical store")
	}
	if _, statErr := os.Stat(stray); statErr == nil {
		t.Error("stray store was not removed")
	}
	if _, statErr := os.Stat(filepath.Join(canonicalLocal, "summaries.json")); statErr != nil {
		t.Error("remaining local state was not moved")
	}

	merged, err := os.ReadFile(filepath.Join(canonicalLocal, "activity.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(merged)), "\n")
	if len(lines) != 2 {
		t.Fatalf("merged log has %d lines, want 2", len(lines))
	}
	if !strings.Contains(lines[0], "orphan-spec") {
		t.Errorf("merge is not chronological: first line is %s", lines[0])
	}
}

func TestDoctorAdoptKeepsStrayOnSlugConflict(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)
	existing := filepath.Join(base, ".vector", "specs", "orphan-spec")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "state.json"), []byte(`{"id":"canonical"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stray := seedStraySpec(t, filepath.Join(base, "website"), "orphan-spec", "2026-07-01T10:00:00Z")

	out, err := execCmd(t, newDoctorAdoptCmd, stray, "--repo-root", base, "--force", "--json")
	if err != nil {
		t.Fatalf("adopt --force: %v", err)
	}
	var result adoptResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse adopt json: %v (%s)", err, out)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0] != "orphan-spec" {
		t.Errorf("conflicts = %v, want [orphan-spec]", result.Conflicts)
	}
	if result.Removed {
		t.Error("a conflicted migration must never delete the stray")
	}
	if _, statErr := os.Stat(filepath.Join(stray, "specs", "orphan-spec")); statErr != nil {
		t.Error("the conflicted spec was moved out of the stray")
	}
	body, err := os.ReadFile(filepath.Join(existing, "state.json"))
	if err != nil || !strings.Contains(string(body), "canonical") {
		t.Errorf("canonical spec was overwritten: %s (%v)", body, err)
	}
}

func TestDoctorAdoptRejectsNonStrayPaths(t *testing.T) {
	base := tempWorkspace(t)
	seedStore(t, base, testConfigBody)

	if _, err := execCmd(t, newDoctorAdoptCmd, filepath.Join(base, ".vector"), "--repo-root", base); err == nil {
		t.Error("adopting the canonical store must fail")
	}
	missing := filepath.Join(base, "nope", ".vector")
	_, err := execCmd(t, newDoctorAdoptCmd, missing, "--repo-root", base)
	if err == nil || !strings.Contains(err.Error(), "not a stray") {
		t.Errorf("want a not-a-stray error, got %v", err)
	}
}
