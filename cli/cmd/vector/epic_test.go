package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mariocampbell/vector/internal/state"
)

func runEpicCreate(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return execCmd(t, newEpicCreateCmd, args...)
}

func TestEpicCreateCommand(t *testing.T) {
	root := t.TempDir()

	out, err := runEpicCreate(t, "--title", "App Mobile", "--color", "violet", "--description", "native clients", "--json", "--repo-root", root)
	if err != nil {
		t.Fatalf("epic create: %v", err)
	}
	var created epicJSON
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("--json is not an epic: %v\n%s", err, out)
	}
	if created.ID != "app-mobile" || created.Color != "violet" || created.Total != 0 || created.ByStatus == nil {
		t.Errorf("created = %+v", created)
	}

	for name, args := range map[string][]string{
		"missing title": {"--repo-root", root},
		"bad color":     {"--title", "X", "--color", "neon", "--repo-root", root},
		"duplicate":     {"--title", "App Mobile", "--repo-root", root},
		"bad id":        {"--title", "X", "--id", "Not Kebab", "--repo-root", root},
	} {
		if _, err := runEpicCreate(t, args...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestEpicUpdateCommand(t *testing.T) {
	root := seedEpicRepo(t)

	if _, err := execCmd(t, newEpicUpdateCmd, "app-mobile", "--repo-root", root); err == nil ||
		!strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("update with no flags err = %v", err)
	}
	out, err := execCmd(t, newEpicUpdateCmd, "app-mobile", "--title", "Mobile", "--color", "", "--json", "--repo-root", root)
	if err != nil {
		t.Fatalf("epic update: %v", err)
	}
	var updated epicJSON
	if err := json.Unmarshal([]byte(out), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Mobile" || updated.Color != "" || updated.Description != "iOS and Android clients" || updated.Total != 1 {
		t.Errorf("updated = %+v", updated)
	}
	if _, err := execCmd(t, newEpicUpdateCmd, "ghost", "--title", "X", "--repo-root", root); err == nil {
		t.Error("updating a missing epic should fail")
	}
}

func TestEpicDeleteRefusesReferencedEpic(t *testing.T) {
	root := seedEpicRepo(t)
	_, err := execCmd(t, newEpicDeleteCmd, "app-mobile", "--repo-root", root)
	if err == nil || !strings.Contains(err.Error(), "alpha") {
		t.Fatalf("delete referenced epic err = %v, want a refusal naming alpha", err)
	}
}

func TestSpecCreateWithEpicAndSpecEpicErrors(t *testing.T) {
	root := seedEpicRepo(t)

	if _, err := execCmd(t, newSpecCreateCmd, "--title", "Push notifications", "--epic", "app-mobile", "--repo-root", root); err != nil {
		t.Fatalf("spec create --epic: %v", err)
	}
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if spec, err := store.ReadSpec("push-notifications"); err != nil || spec.Epic != "app-mobile" {
		t.Fatalf("created spec epic = %v (%v)", spec, err)
	}
	if _, err := execCmd(t, newSpecCreateCmd, "--title", "Orphan", "--epic", "nope", "--repo-root", root); err == nil {
		t.Error("spec create with an unknown epic should fail")
	}

	for name, args := range map[string][]string{
		"no epic and no clear": {"beta", "--repo-root", root},
		"epic and clear":       {"beta", "web", "--clear", "--repo-root", root},
		"unknown epic":         {"beta", "nope", "--repo-root", root},
	} {
		if _, err := execCmd(t, newSpecEpicCmd, args...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestSpecFocusRefusesClosedSpec(t *testing.T) {
	root := seedSpecStatus("alpha", state.StatusClosed)(t)
	if _, err := execCmd(t, newSpecFocusCmd, "alpha", "--repo-root", root); err == nil {
		t.Fatal("focusing a closed spec should fail")
	}
	if _, err := execCmd(t, newSpecFocusCmd, "--repo-root", root); err == nil {
		t.Fatal("focus without an id should fail")
	}
}
