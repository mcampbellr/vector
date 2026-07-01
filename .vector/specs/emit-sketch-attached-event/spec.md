# Emit sketch.attached event in Store.AttachSketch to close the state-sync audit gap

## Change Type
refactor

## What Changes
`Store.AttachSketch` is the only domain mutation that writes state.json without appending an
event to activity.jsonl. Add an `EvtSketchAttached` event type + `SketchAttachedData` payload
in event.go, give `AttachSketch` an `actor string` param, and emit the event via
`s.appendEvent()` after the state write succeeds (TS = ref.CreatedAt). Update the single caller
(sketch.go) to pass `resolveActor()`, and update sketch_test.go call sites + add an assertion
that the event is emitted.

## Why
Attaching a UI sketch left no trace in the activity log, so it never appeared in the spec
timeline / standup / summary — violating state-sync-discipline ("a domain action that leaves no
trace in the JSON is a bug"). This adds the missing record, consistent with every other Store
write.

## Files to Touch
- cli/internal/state/event.go — EvtSketchAttached + SketchAttachedData{Name}
- cli/internal/state/store.go — AttachSketch: +actor param, append event
- cli/cmd/vector/sketch.go — pass resolveActor() to AttachSketch
- cli/internal/state/sketch_test.go — update call sites + assert event emitted

## Acceptance
- AttachSketch appends a sketch.attached event on success (type + payload with the sketch name).
- activity.jsonl gains a sketch.attached entry after each attach.
- New/updated tests pass; go vet + go test green for cli/.
- No consumer breaks (board ignores non-routed; Timeline includes it generically; web has fallback).

## Out of scope
- Surfacing the sketch nicely in standup prose / timeline UI (that would be /vector:raw).