package state

import "time"

// SetFocus turns the developer's "work on this first" marker on or off for a
// spec: lock → read → idempotency → atomic write + spec.focused/spec.unfocused
// event. Setting the value it already has is a no-op returning (false, nil) (no
// duplicate event). Focusing a closed/archived spec is refused (ErrConflict) —
// there is nothing left to work on; unfocusing is always allowed. The spec's
// lifecycle status and priority are untouched: focus is metadata, not a
// transition, and a separate axis from priority.
func (s *Store) SetFocus(id string, focus bool, actor string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	spec, err := s.ReadSpec(id)
	if err != nil {
		return false, err
	}
	if spec.Focus == focus {
		return false, nil
	}
	if focus && spec.Status.IsTerminal() {
		return false, conflictf("spec %q is %s: only an active spec can be focused", id, spec.Status)
	}

	now = now.UTC()
	spec.Focus = focus
	if focus {
		spec.FocusedAt = &now
	} else {
		spec.FocusedAt = nil
	}
	spec.UpdatedAt = now
	if err := writeSpecFile(s.statePath(id), spec); err != nil {
		return false, err
	}

	eventType := EvtSpecUnfocused
	if focus {
		eventType = EvtSpecFocused
	}
	if err := s.appendEvent(Event{V: EventVersion, TS: now, Type: eventType, SpecID: id, Repo: spec.Repo, Actor: actor}); err != nil {
		return false, err
	}
	return true, nil
}
