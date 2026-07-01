# Generate Excalidraw design sketches for UI specs

## Why

When a dev finishes authoring a UI-facing spec with `/vector:raw` or `/vector:research`, they have a
precise written description of the interface but no visual artifact. Turning that description into a
wireframe means leaving Claude Code, opening a design tool, and modeling the layout by hand — friction
that competes with the token-efficient, in-flow experience Vector aims for.

Vector already vendors cheap/expensive agent routing, a CLI-owns-writes state model, and an artifact
serving path (`/api/file`). None of it is wired to produce a downloadable design artifact. This change
closes that gap: at the **tail** of `/vector:raw` and `/vector:research` (after the draft card is
registered), a hybrid heuristic detects UI work in the composed spec and — opt-in — spawns a new
embedded Sonnet agent, `vector-ui-ux-designer`, that emits a valid `.excalidraw` JSON wireframe. The
binary validates and persists it under `.vector/specs/<id>/sketches/`; the board's `SpecDetailsDrawer`
shows a download button; the existing `vector serve` SSE watcher live-updates the panel with no
restart.

It is opt-in per run (heuristic + `AskUserQuestion`), globally opt-out-able (`--no-sketch` flag +
`sketchEnabled: false` in `.vector/config.json`), and degrades softly: a malformed agent output is
silently rejected, leaving the spec a clean draft with no error state.

## What changes

- **UI detection heuristic (tail of `/vector:raw` + `/vector:research`)**: after the spec body is
  composed, a conservative rule fires a suggestion iff (a) the spec's §12 "Estados de UI" section is
  non-empty (not just "No aplica"), **or** (b) ≥2 layer keywords (`board`, `drawer`, `modal`, `web/`,
  `component`, `UI`, `pantalla`, `formulario`, `card`, `componente`) appear in title+body. A single
  loose keyword is a weak signal → no prompt (false-negative preferred over false-positive).
- **Opt-in confirmation + global opt-out**: on a strong signal and no opt-out, the command asks via
  `AskUserQuestion`. Opt-out is `--no-sketch` (per run) and `SketchEnabled *bool` in `.vector/config.json`
  (nil/absent = enabled; `false` = globally suppressed).
- **New embedded agent `vector-ui-ux-designer` (Sonnet)**: vendors the `.excalidraw` JSON format
  knowledge in its body and emits a valid document from the spec's requirements. Embedded via
  `kit/agents/vector-ui-ux-designer.md` + `go generate ./internal/scaffold` (guarded by
  `TestAssetsMatchKit`).
- **Binary validates before persisting**: `vector spec attach-sketch` verifies the JSON is well-formed
  and carries top-level `type`, `version`, `elements` before copying the file. Malformed → silent
  rejection (soft failure); the spec stays a draft.
- **Storage + additive state**: `.vector/specs/<id>/sketches/<name>.excalidraw`; additive
  `Sketches []SketchRef` on `SpecState` (`omitempty`, `SchemaVersion` stays 1, mirroring `QuickWin bool`).
- **Artifact serving**: `"sketch"` added to `artifactRelPath`, `validArtifact`, and the `/api/file`
  handler with `Content-Type: application/octet-stream` + `Content-Disposition: attachment`. The
  `.vector/specs/<id>/sketches/` prefix is already covered by `verifyArtifactPath`.
- **Board/web**: `board.Card` gains `Sketches []SketchRef` (omitempty); `web/src/types/board.ts`
  mirrors it; `ArtifactKey` gains `'sketch'`; `entries.ts` emits one downloadable entry per sketch;
  `SpecArtifactBrowser` serves a native `<a download>` (no `FilePreviewModal`).
- **Async-in-session + SSE**: the command spawns the agent and returns immediately; on completion the
  agent calls `vector spec attach-sketch`; the `vector serve` watcher broadcasts over the existing SSE
  and the board live-updates.
- **Token routing**: `vector-ui-ux-designer` runs on Sonnet (layout/hierarchy reasoning); cost is
  registered via the existing `vector spec route` mechanism.

## Capabilities

### New Capabilities

- `ui-sketch-generation`: detect UI work at the tail of `/vector:raw` and `/vector:research`,
  opt-in-spawn the embedded Sonnet `vector-ui-ux-designer` agent, and produce a validated
  `.excalidraw` wireframe persisted under the spec — all opt-out-able and soft-failing.
- `sketch-artifact-serving`: persist, project, and serve sketches as a downloadable artifact
  (`vector spec attach-sketch` writes; `GET /api/file?artifact=sketch` serves them as
  `attachment`), CLI-owns-writes.

### Modified Capabilities

- `state-model`: `SpecState` gains the additive `Sketches []SketchRef`; the board `Card` projection
  and `/api/board` gain `sketches` (omitempty). `SchemaVersion` unchanged.
- `spec-cli`: new subcommand `vector spec attach-sketch <id> --file <path>` (validate JSON shape,
  copy the file, update `state.json` via `Store.AttachSketch`).
- `config`: `.vector/config.json` gains `sketchEnabled` (`*bool`, omitempty) + `IsSketchEnabled()`.

## Out of scope

- In-binary pre-render to SVG/PNG (the Go binary is stdlib-only, no JS runtime).
- Live inline render of the sketch on the board (`@excalidraw/excalidraw` ~2–3 MB breaks the
  light-bundle rule).
- A true background daemon/worker (would force the binary to make its own LLM calls; today it makes
  zero).
- Sketches for specs with no UI signal (the heuristic filters; not every spec is prompted).
- Translating `.excalidraw` labels or internal content.
- Figma or other external design-tool integration.
- Sketch version history / multi-revision tracking of the same sketch.
- Dark-mode theming of the `.excalidraw`.
- Inline thumbnails on the board card or drawer header, and inline preview in `FilePreviewModal`.
