# Canonical Excalidraw sketch filenames (binary-authoritative)

## Change Type
refactor

## What Changes
`vector spec attach-sketch` no longer stores the agent's temp filename. When `--name`
is omitted, the binary (`Store.AttachSketch`) computes a canonical name from the spec:

    <spec-slug>{-<ticketKey>}-sketch{-<count>}.excalidraw

- With ticket:    evolvs-pdp-editorial-EV-398-sketch.excalidraw
- Without ticket: evolvs-pdp-editorial-sketch.excalidraw
- Nth (count>0):  evolvs-pdp-editorial-sketch-1.excalidraw  (first is unsuffixed)

`--name` stays as an explicit override (canonical is the default). The
`vector-ui-ux-designer` agent already invokes attach-sketch without `--name`, so its
sketches become canonical automatically.

## Why
CLI-owns-writes: the binary knows the spec id, ticket, and existing sketch count under
its lock, so it — not the agent's arbitrary temp basename — should name the artifact.
Deterministic names make sketches discoverable and stable across tooling.

## Files to Touch
- cli/internal/state/store.go — AttachSketch: canonical name when ref.Name=="" (+ helper, collision-safe)
- cli/cmd/vector/sketch.go — pass empty name when --name omitted (no file-basename fallback); keep --name override
- cli/internal/state/sketch_test.go — drop "" from bad-name guard; add canonical-name test (ticket/no-ticket/count)
- cli/cmd/vector/sketch_test.go — sanitizeSketchName empty→""; TestRunSpecAttachSketch expects add-ui-sketch.excalidraw
- cli/cmd/vector/testdata/golden/spec-attach-sketch.json — regenerate (alpha-sketch.excalidraw)

## Acceptance
- attach-sketch without --name persists <slug>{-<ticket>}-sketch{-N}.excalidraw and returns it in --json
- explicit --name still overrides and still rejects unsafe names
- second sketch on a spec gets -1 suffix; SketchRef.Name matches the file on disk
- go test ./... green; golden regenerated

## Risks
- Behavior change: agent-produced sketch filenames change (intended).
- Re-running raw appends a new suffixed sketch instead of overwriting (matches "si hay varios").