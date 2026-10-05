package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mariocampbell/vector/internal/state"
)

func openTestStore(t *testing.T, root string) *state.Store {
	t.Helper()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

// TestSpecStatusOpenOnNeedsAttentionListsLegalTargets is the CLI-level repro of
// the reported bug (`spec close` then `spec status X open` on a needs-attention
// spec answered "already open").
func TestSpecStatusOpenOnNeedsAttentionListsLegalTargets(t *testing.T) {
	root := seedSpecStatus("alpha", state.StatusInProgress)(t)
	if _, err := execCmd(t, newSpecStatusCmd, "alpha", "needs-attention", "--reason", "blocked", "--repo-root", root); err != nil {
		t.Fatal(err)
	}
	_, closeErr := execCmd(t, newSpecCloseCmd, "alpha", "--repo-root", root)
	if closeErr == nil || !strings.Contains(closeErr.Error(), "legal next statuses") || !strings.Contains(closeErr.Error(), "vector spec status alpha in-progress") {
		t.Errorf("close err = %v, want the legal targets with their commands", closeErr)
	}
	_, openErr := execCmd(t, newSpecStatusCmd, "alpha", "open", "--repo-root", root)
	if openErr == nil || strings.Contains(openErr.Error(), "already open") || !strings.Contains(openErr.Error(), "review") {
		t.Errorf("status open err = %v, want an illegal-transition error listing in-progress/review", openErr)
	}
}

func TestSpecCloseAndArchiveResolution(t *testing.T) {
	root := seedSpecStatus("alpha", state.StatusReview)(t)
	if _, err := execCmd(t, newSpecCloseCmd, "alpha", "--resolution", "nope", "--repo-root", root); err == nil || !strings.Contains(err.Error(), "invalid --resolution") {
		t.Fatalf("invalid resolution err = %v", err)
	}
	out, err := execCmd(t, newSpecCloseCmd, "alpha", "--resolution", "duplicate", "--note", "dup of beta", "--json", "--repo-root", root)
	if err != nil {
		t.Fatal(err)
	}
	var closed map[string]string
	if err := json.Unmarshal([]byte(out), &closed); err != nil {
		t.Fatal(err)
	}
	if closed["resolution"] != "duplicate" || closed["resolutionNote"] != "dup of beta" || closed["status"] != "closed" {
		t.Errorf("close json = %v", closed)
	}
	out, err = execCmd(t, newSpecArchiveCmd, "alpha", "--json", "--repo-root", root)
	if err != nil {
		t.Fatal(err)
	}
	var archived map[string]string
	if err := json.Unmarshal([]byte(out), &archived); err != nil {
		t.Fatal(err)
	}
	if archived["status"] != "archived" || archived["resolution"] != "duplicate" {
		t.Errorf("archive json = %v, want the close-time resolution kept", archived)
	}
}

// seedFilterRepo: epics app-mobile (focused) and web; specs alpha (open,
// app-mobile), beta (in-progress, no epic, own focus), gamma (needs-attention,
// web), delta (closed obsolete, app-mobile).
func seedFilterRepo(t *testing.T) string {
	t.Helper()
	root := seedEpicRepo(t) // app-mobile, web; alpha (app-mobile, focused), beta (no epic)
	store := openTestStore(t, root)
	if _, err := store.SetFocus("alpha", false, "tester", goldenClock); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetEpicFocus("app-mobile", true, "tester", goldenClock); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplySpec("beta", "beta", "tester", goldenClock); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetFocus("beta", true, "tester", goldenClock); err != nil {
		t.Fatal(err)
	}
	for _, params := range []state.CreateSpecParams{
		{ID: "gamma", Title: "Gamma", Status: state.StatusInProgress, Priority: state.PriorityHigh, Epic: "web"},
		{ID: "delta", Title: "Delta", Status: state.StatusOpen, Priority: state.PriorityNormal, Epic: "app-mobile"},
	} {
		params.Actor, params.Now = "tester", goldenClock
		if _, err := store.CreateSpec(params); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.SetStatusAttention("gamma", state.StatusNeedsAttention, state.Attention{Category: state.AttentionDependency, Summary: "waiting on API"}, "tester", goldenClock); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CloseSpecWith("delta", state.ResolutionObsolete, "", "tester", goldenClock); err != nil {
		t.Fatal(err)
	}
	return root
}

func listIDs(t *testing.T, root string, args ...string) ([]map[string]any, string) {
	t.Helper()
	out, err := execCmd(t, newSpecListCmd, append(args, "--json", "--repo-root", root)...)
	if err != nil {
		t.Fatalf("spec list %v: %v", args, err)
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry["id"].(string))
	}
	return entries, strings.Join(ids, ",")
}

func TestSpecListFilters(t *testing.T) {
	root := seedFilterRepo(t)
	cases := []struct {
		args []string
		want string
	}{
		{nil, "alpha,beta,delta,gamma"},
		{[]string{"--epic", "app-mobile"}, "alpha,delta"},
		{[]string{"--no-epic"}, "beta"},
		{[]string{"--status", "in-progress,needs-attention"}, "beta,gamma"},
		{[]string{"--focus"}, "alpha,beta"}, // alpha inherits from app-mobile; delta is closed → no inheritance
		{[]string{"--epic", "app-mobile", "--focus"}, "alpha"},
		{[]string{"--epic", "web", "--status", "open"}, ""},
	}
	for _, tc := range cases {
		if _, got := listIDs(t, root, tc.args...); got != tc.want {
			t.Errorf("spec list %v = %q, want %q", tc.args, got, tc.want)
		}
	}

	entries, _ := listIDs(t, root)
	byID := map[string]map[string]any{}
	for _, entry := range entries {
		byID[entry["id"].(string)] = entry
	}
	if byID["alpha"]["focusInherited"] != true || byID["alpha"]["focus"] != nil {
		t.Errorf("alpha = %v, want focusInherited only", byID["alpha"])
	}
	attention, ok := byID["gamma"]["needsAttention"].(map[string]any)
	if !ok || attention["summary"] != "waiting on API" || attention["category"] != "dependency" {
		t.Errorf("gamma needsAttention = %v", byID["gamma"]["needsAttention"])
	}
	if byID["delta"]["resolution"] != "obsolete" {
		t.Errorf("delta = %v, want resolution obsolete", byID["delta"])
	}
	if _, has := byID["beta"]["needsAttention"]; has {
		t.Error("needsAttention must be absent on unflagged specs")
	}

	for _, bad := range [][]string{
		{"--epic", "ghost"},
		{"--epic", "web", "--no-epic"},
		{"--status", "draft"},
	} {
		if _, err := execCmd(t, newSpecListCmd, append(bad, "--repo-root", root)...); err == nil {
			t.Errorf("spec list %v: want an error", bad)
		}
	}
}

func TestSpecEpicBulkAssign(t *testing.T) {
	root := seedFilterRepo(t)
	store := openTestStore(t, root)
	eventsBefore, _ := store.ReadEvents()

	if _, err := execCmd(t, newSpecEpicCmd, "--epic", "web", "alpha", "ghost", "--repo-root", root); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("unknown id err = %v", err)
	}
	if spec, _ := store.ReadSpec("alpha"); spec.Epic != "app-mobile" {
		t.Fatalf("a failed batch wrote alpha: epic %q", spec.Epic)
	}
	if eventsAfter, _ := store.ReadEvents(); len(eventsAfter) != len(eventsBefore) {
		t.Fatal("a failed batch appended events")
	}

	out, err := execCmd(t, newSpecEpicCmd, "--epic", "web", "alpha", "beta", "gamma", "--json", "--repo-root", root)
	if err != nil {
		t.Fatal(err)
	}
	var result epicBulkJSON
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	want := epicBulkJSON{Epic: "web", Changed: 2, Unchanged: 1, Specs: []epicBulkSpecJSON{
		{ID: "alpha", Changed: true, Previous: "app-mobile"},
		{ID: "beta", Changed: true},
		{ID: "gamma", Changed: false, Previous: "web"},
	}}
	if !jsonEqual(t, result, want) {
		t.Errorf("bulk result = %+v, want %+v", result, want)
	}

	// --stdin + --clear.
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	writer.WriteString("alpha\n\n# comment\nbeta\n")
	writer.Close()
	origStdin := os.Stdin
	os.Stdin = reader
	out, err = execCmd(t, newSpecEpicCmd, "--clear", "--stdin", "--json", "--repo-root", root)
	os.Stdin = origStdin
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.Epic != "" || result.Changed != 2 || len(result.Specs) != 2 {
		t.Errorf("stdin clear result = %+v", result)
	}

	// The legacy single form keeps its original shape.
	out, err = execCmd(t, newSpecEpicCmd, "alpha", "web", "--json", "--repo-root", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"changed": true`) || !strings.Contains(out, `"id": "alpha"`) || strings.Contains(out, `"specs"`) {
		t.Errorf("legacy form output = %s", out)
	}
	if _, err := execCmd(t, newSpecEpicCmd, "alpha", "beta", "web", "--repo-root", root); err == nil || !strings.Contains(err.Error(), "--epic") {
		t.Errorf("3 positionals without --epic err = %v, want a pointer to --epic", err)
	}
	if _, err := execCmd(t, newSpecEpicCmd, "--epic", "web", "--clear", "alpha", "--repo-root", root); err == nil {
		t.Error("--epic with --clear must fail")
	}
}

func jsonEqual(t *testing.T, got, want any) bool {
	t.Helper()
	left, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	right, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	return string(left) == string(right)
}

func TestEpicOrderAndFocusCommands(t *testing.T) {
	root := seedEpicRepo(t)
	if _, err := execCmd(t, newEpicCreateCmd, "--title", "Backend", "--order", "1", "--repo-root", root); err != nil {
		t.Fatal(err)
	}
	if _, err := execCmd(t, newEpicUpdateCmd, "web", "--order", "2", "--repo-root", root); err != nil {
		t.Fatal(err)
	}
	if _, err := execCmd(t, newEpicUpdateCmd, "web", "--order", "-1", "--repo-root", root); err == nil {
		t.Error("negative --order must fail")
	}
	out, err := execCmd(t, newEpicListCmd, "--json", "--repo-root", root)
	if err != nil {
		t.Fatal(err)
	}
	var epics []epicJSON
	if err := json.Unmarshal([]byte(out), &epics); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(epics))
	for _, epic := range epics {
		ids = append(ids, epic.ID)
	}
	if got := strings.Join(ids, ","); got != "backend,web,app-mobile" || epics[0].Order != 1 {
		t.Errorf("epic list order = %s (%+v)", got, epics)
	}
	if _, err := execCmd(t, newEpicUpdateCmd, "web", "--order", "0", "--repo-root", root); err != nil {
		t.Fatal(err)
	}
	if epic, _ := openTestStore(t, root).GetEpic("web"); epic.Order != 0 {
		t.Errorf("--order 0 must unset, got %d", epic.Order)
	}

	out, err = execCmd(t, newEpicFocusCmd, "app-mobile", "--json", "--repo-root", root)
	if err != nil || !strings.Contains(out, `"changed": true`) || !strings.Contains(out, `"focus": true`) {
		t.Fatalf("epic focus = %s (%v)", out, err)
	}
	out, err = execCmd(t, newEpicShowCmd, "app-mobile", "--json", "--repo-root", root)
	if err != nil || !strings.Contains(out, `"focus": true`) {
		t.Errorf("epic show after focus = %s (%v)", out, err)
	}
	out, err = execCmd(t, newEpicUnfocusCmd, "app-mobile", "--json", "--repo-root", root)
	if err != nil || !strings.Contains(out, `"focus": false`) || !strings.Contains(out, `"changed": true`) {
		t.Errorf("epic unfocus = %s (%v)", out, err)
	}
}

// TestSpecNextUsesEpicOrderAndInheritedFocus: beta (normal, no epic) would win
// over alpha (low, app-mobile) on priority; ordering app-mobile makes alpha win,
// and focusing web makes its spec win over both.
func TestSpecNextUsesEpicOrderAndInheritedFocus(t *testing.T) {
	root := seedEpicRepo(t)
	store := openTestStore(t, root)
	if _, err := store.SetFocus("alpha", false, "tester", goldenClock); err != nil {
		t.Fatal(err)
	}
	next := func() map[string]string {
		out, err := execCmd(t, newSpecNextCmd, "--json", "--repo-root", root)
		if err != nil {
			t.Fatal(err)
		}
		var pick map[string]string
		if err := json.Unmarshal([]byte(out), &pick); err != nil {
			t.Fatal(err)
		}
		return pick
	}
	if pick := next(); pick["id"] != "beta" {
		t.Fatalf("baseline pick = %v, want beta (priority)", pick)
	}
	one := 1
	if _, _, err := store.UpdateEpic("app-mobile", state.UpdateEpicParams{Order: &one}, "tester", goldenClock); err != nil {
		t.Fatal(err)
	}
	if pick := next(); pick["id"] != "alpha" {
		t.Fatalf("pick with ordered epic = %v, want alpha", pick)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "gamma", Title: "Gamma", Status: state.StatusOpen, Priority: state.PriorityLow, Epic: "web", Actor: "tester", Now: goldenClock}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetEpicFocus("web", true, "tester", goldenClock); err != nil {
		t.Fatal(err)
	}
	if pick := next(); pick["id"] != "gamma" || pick["focusInherited"] != "true" || pick["focus"] != "" {
		t.Fatalf("pick with focused epic = %v, want gamma with focusInherited", pick)
	}
}
