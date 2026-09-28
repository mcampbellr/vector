package main

import "github.com/mariocampbell/vector/internal/state"

// nextCommandFor returns the slash command the user should run next for a spec in
// the given status, or ("", false) when none applies. It is a 1:1 port of the
// board's web/src/components/SpecCard/nextCommandFor.ts so the terminal and the
// board never disagree on what comes next; archived and the legacy draft status
// (unknown to the web Status type) are treated like closed.
func nextCommandFor(status state.Status, id string) (string, bool) {
	switch status {
	case state.StatusOpen, state.StatusInProgress, state.StatusNeedsAttention:
		return "/vector:apply " + id, true
	case state.StatusReview:
		return "/vector:close " + id, true
	default:
		// closed, archived, legacy draft.
		return "", false
	}
}
