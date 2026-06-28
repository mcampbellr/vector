# Design — add-spec-file-viewer

## Context

The board projects committed `state` read-only and already serves `/api/board` + `/api/events`
(SSE), `/api/summary`, `/api/activity`, and `/api/standup` — every endpoint is GET, with
`Cache-Control: no-store` and a shared `writeJSONError` helper (`cli/internal/board/server.go`). The
board `Source` interface (`board.go:131`) is the only read seam web consumes; `*state.Store`
satisfies it, and `s.root` is the `.vector` dir (so the repo root is `filepath.Dir(s.root)`). The
`SpecDetailsDrawer` is already a folder component with composable sections and an overlay/dialog
pattern (`role="dialog"`, `aria-modal`, Escape-to-close, `stopPropagation`). There is no surface
that reads a spec's source documents.

## Goals / Non-Goals

**Goals:**
- A read-only artifact endpoint that resolves a file from `spec id + artifact key`, server-side,
  from trusted committed state — never from a client-supplied path.
- A `Files` section in the drawer listing the spec's available docs (derived from state, no disk
  scan) and an in-board modal that previews one file as rendered Markdown.
- Keep the Markdown renderer out of the initial bundle (lazy, code-split).

**Non-Goals:**
- Editing, creating, or uploading files; any web write surface (board stays read-only).
- Opening files in a new browser tab; custom doc discovery / filesystem scanning.
- Free-form path access; rendering raw HTML; syntax highlighting / file tree / line numbers / diffing.
- Crediting any token route (no agent runs; nothing appended to `activity.jsonl`).

## Decisions

- **Path resolution + I/O live in cli/state**, not the HTTP handler. `ReadSpecArtifact` on
  `*state.Store` resolves the key (`spec` → `SpecState.SpecDoc`; `proposal`/`design`/`tasks` →
  `openspec/changes/<change>/<key>.md`, only when `OpenSpec != nil` and the matching `Artifacts`
  flag is set), joins with the repo root, and reads with `os.ReadFile`.
- **The client never sends a path.** `GET /api/file` takes `spec` + an `artifact` **enum**
  (`{spec, proposal, design, tasks}`); traversal is removed by design. **Defense in depth**: the
  resolved path is `filepath.Clean`ed and verified to stay under the repo root and under the expected
  `.vector/specs/<id>/` or `openspec/changes/<change>/` prefix, even though both inputs come from
  trusted committed state.
- **Error mapping**: wrapped `fs.ErrNotExist` (artifact flag unset / file missing / spec absent) →
  404; other errors → 500; bad/missing/unknown param → 400. Success writes **raw bytes** as
  `text/markdown; charset=utf-8` with `Cache-Control: no-store` (not a JSON envelope).
- **The file list is static**, derived from the already-loaded `Card` (`specDoc` + `artifacts`
  flags); it triggers no fetch. Content is fetched **lazily on click**.
- **`useFileContent` returns text, not JSON.** The existing `useFetchJSON` parses JSON and is not
  reusable for a Markdown body; the new hook mirrors its `{data, loading, error, reload}` shape but
  resolves `res.text()`, and is lazy on a `null` artifact.
- **The Markdown renderer is isolated + code-split.** `MarkdownView` is the only module importing
  `react-markdown` + `remark-gfm`, loaded via `React.lazy` from the modal so it stays out of the
  initial board bundle (the key mitigation for the new dependency). No `rehype-raw`, no raw HTML.
- **Modal stacks above the drawer and closes independently.** Escape is handled at the modal first
  (`stopPropagation`) so the drawer's own Escape handler does not also fire; with no modal open,
  Escape closes the drawer (existing behavior).

## Open questions (carried from the spec)

- **Go version target** → unchanged (read from `cli/go.mod`; not modified here).
- **`react-markdown` / `remark-gfm` exact versions** → pin the resolved React-19-compatible versions
  in `web/package.json` (floors `^9.0.0` / `^4.0.0`); verify the peer range at install time.
- **Optional size cap on served files** → none in this phase; files are served in full
  (`react-markdown` parses client-side). A cap is a later perf consideration, not a correctness one.
- **Exact visible copy** (`"Files"`, `"No source files available."`, `"loading file…"`,
  `"could not load file: "` + `"Retry"`, close `aria-label`) → as specified; adjustable later.
