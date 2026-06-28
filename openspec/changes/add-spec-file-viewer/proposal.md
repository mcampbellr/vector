# Spec artifact file viewer in the details drawer

## Why

The board's details drawer surfaces a spec's summary, activity, and useful commands, but never the
spec's actual source documents. To read the authored `spec.md` or the OpenSpec change artifacts
(`proposal.md`, `design.md`, `tasks.md`) a developer must leave the board and open them on disk.
There is no in-board way to read the very documents the board is built around.

## What changes

- **Read-only artifact endpoint (cli)** — a new `Source.ReadSpecArtifact(specID, artifact)` on
  `*state.Store` resolves an artifact key (`spec`/`proposal`/`design`/`tasks`) to an on-disk path
  **server-side** from committed spec state (`SpecDoc` for `spec`; `openspec/changes/<change>/<key>.md`
  gated by the `Artifacts` flags), defends the resolved path against escaping the repo root, and
  reads the bytes. A new `GET /api/file?spec=<id>&artifact=<key>` handler validates the params,
  serves the raw bytes as `text/markdown` (no JSON envelope), and maps absent→404 / read-error→500 /
  bad-param→400. The client never sends a path, so traversal is removed by design.
- **Files section + preview modal (web)** — the `SpecDetailsDrawer` gains a `Files` section listing
  the spec's available documents (built from the new `Card.specDoc` field + the existing `artifacts`
  flags, no filesystem scan). Clicking a file opens a `FilePreviewModal` stacked above the drawer
  that fetches the file via a new text-fetch `useFileContent` hook and renders the Markdown through a
  **lazily code-split** `MarkdownView` (`react-markdown` + `remark-gfm`). The modal closes
  independently of the drawer (button / `Escape` / overlay). The board stays read-only.

## Capabilities

### New Capabilities
- `spec-file-viewer`: a read-only `GET /api/file` endpoint plus a `Files` section + in-board preview
  modal that lets a developer read a spec's `spec.md` and OpenSpec artifacts (`proposal`/`design`/
  `tasks`) as rendered Markdown without leaving the board; paths are resolved server-side from
  committed state (no client path, no disk scan).

### Modified Capabilities
- `spec-details-drawer`: the drawer gains a `Files` section after `Activity` (additive; the existing
  Summary / Next command / Useful commands / Activity sections are unchanged).

## Impact

- `cli/internal/state` (new `artifact.go`: `ReadSpecArtifact` — key→path resolution + traversal
  defense), `cli/internal/board` (`Source.ReadSpecArtifact`, `Card.SpecDoc` projection, `handleFile`
  + `/api/file` route).
- `web/`: new `SpecArtifactBrowser`, `FilePreviewModal`, `MarkdownView` (lazy) under
  `SpecDetailsDrawer/` + the `Files` section; new `useFileContent` text-fetch hook; `Card.specDoc` in
  `web/src/types/board.ts`; drawer CSS for the file list + modal.
- **One new web dependency**: `react-markdown` + `remark-gfm`, a documented user-approved deviation
  from the "no new libraries / light bundle" rule, mitigated by isolating it in `MarkdownView` and
  code-splitting it via `React.lazy`. No new Go dependency. The board stays read-only.

Authored spec: `.vector/specs/add-spec-file-viewer/spec.md`.
