package state

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

// eventTypes returns the activity log's event types in order.
func eventTypes(t *testing.T, store *Store) []EventType {
	t.Helper()
	events, err := store.ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	types := make([]EventType, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func countEvents(types []EventType, want EventType) int {
	count := 0
	for _, eventType := range types {
		if eventType == want {
			count++
		}
	}
	return count
}

func TestSetFocusTogglesAndIsIdempotent(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusOpen, PriorityLow)
	later := fixedNow().Add(1)

	changed, err := store.SetFocus("feat", true, "tester", later)
	if err != nil || !changed {
		t.Fatalf("SetFocus(on) = %v, %v; want changed", changed, err)
	}
	spec, _ := store.ReadSpec("feat")
	if !spec.Focus || spec.FocusedAt == nil || !spec.FocusedAt.Equal(later) {
		t.Fatalf("focus=%v focusedAt=%v, want true/%v", spec.Focus, spec.FocusedAt, later)
	}
	if !spec.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt = %v, want %v (the watcher needs a fresh state.json)", spec.UpdatedAt, later)
	}
	if spec.Priority != PriorityLow || spec.Status != StatusOpen {
		t.Errorf("focus must not touch priority/status: got %s/%s", spec.Priority, spec.Status)
	}

	// Re-focusing is a no-op: no second event.
	if changed, err := store.SetFocus("feat", true, "tester", later); err != nil || changed {
		t.Fatalf("repeat SetFocus(on) = %v, %v; want no-op", changed, err)
	}
	if changed, err := store.SetFocus("feat", false, "tester", later); err != nil || !changed {
		t.Fatalf("SetFocus(off) = %v, %v; want changed", changed, err)
	}
	spec, _ = store.ReadSpec("feat")
	if spec.Focus || spec.FocusedAt != nil {
		t.Errorf("after unfocus: focus=%v focusedAt=%v", spec.Focus, spec.FocusedAt)
	}

	types := eventTypes(t, store)
	if got := countEvents(types, EvtSpecFocused); got != 1 {
		t.Errorf("spec.focused events = %d, want 1", got)
	}
	if got := countEvents(types, EvtSpecUnfocused); got != 1 {
		t.Errorf("spec.unfocused events = %d, want 1", got)
	}
}

func TestSetFocusRefusesTerminalSpecAndUnknownSpec(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "done", StatusClosed, PriorityNormal)

	if _, err := store.SetFocus("done", true, "tester", fixedNow()); !errors.Is(err, ErrConflict) {
		t.Errorf("focusing a closed spec: err = %v, want ErrConflict", err)
	}
	// Unfocusing a closed spec is a harmless no-op.
	if changed, err := store.SetFocus("done", false, "tester", fixedNow()); err != nil || changed {
		t.Errorf("unfocus closed = %v, %v; want no-op", changed, err)
	}
	if _, err := store.SetFocus("missing", true, "tester", fixedNow()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("unknown spec: err = %v, want not-exist", err)
	}
}

func TestCloseClearsFocus(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "feat", StatusReview, PriorityNormal)
	if _, err := store.SetFocus("feat", true, "tester", fixedNow()); err != nil {
		t.Fatal(err)
	}
	closed, err := store.CloseSpec("feat", "tester", fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if closed.Focus || closed.FocusedAt != nil {
		t.Errorf("closing must drop the focus marker: focus=%v focusedAt=%v", closed.Focus, closed.FocusedAt)
	}
}

// TestFocusAndEpicAreOmittedWhenUnset guards the additive contract: a spec with
// neither marker serializes without the new keys, byte-identical to before.
func TestFocusAndEpicAreOmittedWhenUnset(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	newSpec(t, store, "plain", StatusOpen, PriorityNormal)
	raw, err := os.ReadFile(store.StatePath("plain"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"focus"`, `"focusedAt"`, `"epic"`} {
		if bytes.Contains(raw, []byte(key)) {
			t.Errorf("state.json unexpectedly contains %s:\n%s", key, raw)
		}
	}
}
