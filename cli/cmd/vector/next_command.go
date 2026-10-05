package main

import "github.com/mariocampbell/vector/internal/state"

// nextCommandFor returns the slash command the user should run next for a spec,
// or ("", false) when none applies. It is a 1:1 port of the board's
// web/src/components/SpecCard/nextCommandFor.ts so the terminal and the board
// never disagree on what comes next; archived and the legacy draft status
// (unknown to the web Status type) are treated like closed.
//
// A spec in review needs shipping until /vector:ship records its PR; once the
// PR exists the next step is /vector:close (run after the merge). The PR is the
// only signal: a draft PR still reads as shipped, and merge is not detected.
func nextCommandFor(spec *state.SpecState) (string, bool) {
	switch spec.Status {
	case state.StatusOpen, state.StatusInProgress, state.StatusNeedsAttention:
		return "/vector:apply " + spec.ID, true
	case state.StatusReview:
		if spec.PR != nil {
			return "/vector:close " + spec.ID, true
		}
		return "/vector:ship " + spec.ID, true
	default:
		// closed, archived, legacy draft.
		return "", false
	}
}
