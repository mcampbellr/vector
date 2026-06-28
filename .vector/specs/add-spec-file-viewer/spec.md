# Spec: Spec artifact file viewer in the details drawer

## 1. Goal

Add a **Files** section to the existing `SpecDetailsDrawer` (web) that lists a spec's source
documents and lets the developer **read them in an in-board preview modal** without leaving the
board or navigating the filesystem. The files are the authored **`spec.md`** plan and the OpenSpec
change artifacts **`proposal.md`**, **`design.md`**, **`tasks.md`** (each shown only when it
exists for that spec).

Clicking a file opens a **modal** (a higher-stacked overlay over the drawer) that fetches the file
from a new **read-only** CLI endpoint and renders its Markdown. The list itself is derived from the
spec's projected state (the new `specDoc` field + the existing `artifacts` flags) — no filesystem
scan. The endpoint **resolves the on-disk path server-side from the spec id + a fixed artifact
key**; the client never sends a path, which removes path traversal as an attack surface by design.

Decisions already made by the user (Section 10): **in-board modal** (not a new tab); **rendered
Markdown** (not raw text), accepting a single new dependency (`react-markdown` + `remark-gfm`),
**lazily code-split** so the board's initial bundle stays light; scope is **`spec.md` + the three
OpenSpec artifacts only** (fixed whitelist, no custom docs, no disk scan).

## 2. Scope

### Included in this phase

**A. Read-only artifact endpoint (cli)**

- **New `Source` method** `ReadSpecArtifact(specID, artifact string) ([]byte, error)` on
  `*state.Store`, added to the board `Source` interface (`cli/internal/board/board.go:131`). It:
  - resolves the spec via `ReadSpec(specID)` (returns a not-found error if absent);
  - maps the artifact key to an on-disk file — `spec` → `SpecState.SpecDoc`; `proposal`/`design`/
    `tasks` → `openspec/changes/<OpenSpec.Change>/<artifact>.md`, **only when** `OpenSpec != nil`
    and the matching `Artifacts` flag is set;
  - joins the repo-relative path with the **repo root** (`filepath.Dir(s.root)`, since `s.root` is
    the `.vector` dir) and reads it with `os.ReadFile`;
  - **defends in depth**: `filepath.Clean`s the resolved path and verifies it stays under the repo
    root and under the expected `.vector/specs/<id>/` or `openspec/changes/<change>/` prefix even
    though both inputs come from trusted committed state;
  - returns a wrapped `fs.ErrNotExist` when the artifact flag is unset or the file is missing (the
    handler maps this to 404), other errors unwrapped (→ 500).
- **New HTTP handler** `handleFile` + route `GET /api/file?spec=<id>&artifact=<key>`
  (`cli/internal/board/server.go`): validates `spec` (400 if empty) and `artifact` against the
  fixed enum `{spec, proposal, design, tasks}` (400 if missing/unknown), calls `ReadSpecArtifact`,
  maps `fs.ErrNotExist` → 404 and other errors → 500 (reusing `writeJSONError`), and on success
  writes the **raw file bytes** with `Content-Type: text/markdown; charset=utf-8` and
  `Cache-Control: no-store`.

**B. Files section + preview modal (web)**

- **New projected field** `specDoc` on the board `Card` (`cli/internal/board/board.go` `Card` +
  `web/src/types/board.ts` `Card`): the repo-relative spec-doc path (display label basename +
  existence signal). Additive, `omitempty`.
- **New text-fetch hook** `useFileContent(spec, artifact)` (`web/src/api/useFileContent.ts`):
  mirrors `useFetchJSON`'s shape (`{ data, loading, error, reload }`) but resolves the response as
  **text** (`res.text()`), since the file body is Markdown, not JSON. Lazy: a `null` artifact skips
  the fetch.
- **New component** `web/src/components/SpecDetailsDrawer/SpecArtifactBrowser.tsx`: builds the file
  list from `card` (entry for `spec.md` when `card.specDoc`; `proposal`/`design`/`tasks` when
  `card.artifacts?.*`), renders each as a clickable, keyboard-activable item, and owns the
  `selected artifact` state that opens the modal.
- **New component** `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx`: an overlay (higher
  z-index than the drawer) that, given `{ specId, artifact, fileName, onClose }`, fetches via
  `useFileContent` and renders the Markdown with a **lazily imported** `MarkdownView` so the parser
  is code-split out of the initial bundle. Loading / error+retry / content states; `role="dialog"`,
  `aria-modal`, focus the close button on open, close on its own button / `Escape` / overlay click
  **without** also closing the drawer (stop propagation; Escape handled at the modal first).
- **New component** `web/src/components/SpecDetailsDrawer/MarkdownView.tsx`: thin wrapper over
  `react-markdown` + `remark-gfm` (GFM needed for `tasks.md` checkboxes/tables). Default-safe (no
  raw HTML). This is the only module that imports the new dependency, and it is loaded via
  `React.lazy`.
- **Drawer wiring** (`web/src/components/SpecDetailsDrawer/index.tsx`): render a `Files` section
  (after `Activity`) containing `SpecArtifactBrowser`.
- **Tests**: Go tests for `ReadSpecArtifact` (key→path resolution, flag-gating, traversal defense)
  and `handleFile` (200/400/404/500); web typecheck + build green.

### Out of scope

- **Editing, creating, or uploading files** from the web. The board stays **read-only** (no
  POST/PUT) — consistent with `add-spec-summary-drawer` and `architecture/state-model.md`.
- **Opening files in a new browser tab/window.** The user chose the in-board modal; no new-tab
  affordance is built here (could be a later addition).
- **Custom doc discovery / filesystem scanning.** Only `spec.md` + the three OpenSpec artifacts,
  derived from committed spec state. A file present on disk but not reflected in state is not shown.
- **Free-form file path access.** The client never sends a path; only the fixed artifact enum.
- **Rendering arbitrary embedded HTML or executing scripts** from the Markdown (no
  `rehype-raw`/`dangerouslySetInnerHTML`).
- **Syntax highlighting, a file tree, line numbers, search-in-file, or diffing.**
- **Watching/live-reloading** open file content (artifacts are effectively static while the modal
  is open).
- **Crediting any token route** — no agent runs here; nothing is appended to `activity.jsonl`.
- **Changing other drawer sections** (Summary, Next command, Useful commands, Activity) beyond
  adding the new section.

The agent must not implement anything outside the scope above, even if it seems related.

---

## 3. Project technologies and conventions

### Stack

- **Go** (single module, stdlib only) for `cli/`: `net/http`, `os.ReadFile`, `path/filepath`,
  `errors`/`io/fs` for the endpoint and path defense. No new Go dependency.
- **TypeScript + React 19 + Vite** for `web/`: CSS Modules + CSS variables
  (`web/src/styles/tokens.css`); icons via `lucide-react`.
- **One new web dependency**: `react-markdown` + `remark-gfm` for Markdown rendering. This is a
  **documented, user-approved deviation** from the repo's "no new libraries / light bundle" rule
  (`standards/typescript-react.md`, `architecture/distribution-packaging.md`). It is mitigated by
  importing the renderer only inside `MarkdownView`, loaded via `React.lazy`, so it is **code-split
  out of the initial board bundle** and pulled in only when a file modal opens.
- Testing: Go `testing` (table-driven); `web/` typecheck + build as the gate.
- No agent tier involved — this feature runs no model.

### Relevant versions

- Go: `TBD — ver Open questions` (confirm in `cli/go.mod`; not changed here).
- React: 19 (`web/package.json`; not changed here).
- `react-markdown` (floor `^9.0.0`) / `remark-gfm` (floor `^4.0.0`) — both React-19-compatible;
  pin the exact resolved version in `web/package.json` and verify the peer range at install time.

### Existing patterns to respect

- **Read-only HTTP server**: every endpoint is GET; `{}`/`{error}` JSON envelopes; `Cache-Control:
  no-store` (`cli/internal/board/server.go` `handleSummary`/`handleActivity`). `handleFile` follows
  the same error-writing helper (`writeJSONError`) but returns raw bytes (not JSON) on success.
- **Board `Source` interface** is the only read seam web consumes (`board.go:131`); `*state.Store`
  satisfies it. `ReadSpecArtifact` is added there, keeping all file I/O and path resolution inside
  the cli/state layer.
- **Store path helpers**: `s.root` is the `.vector` dir; `specDir(id)` etc. join from it
  (`cli/internal/state/store.go:33`). The repo root is `filepath.Dir(s.root)`.
- **Web lazy-fetch on open**: the drawer only mounts when a card is selected; `useSpecSummary`/
  `useSpecActivity` fetch on mount and tear down on close. `useFileContent` follows this but
  fetches **text**, since the existing `useFetchJSON` parses JSON and is therefore **not reusable**
  for a Markdown body.
- **Component layout**: one component per file under `SpecDetailsDrawer/` (it is already a folder
  with `CopyableCommand`, `UsefulCommands`, `usefulCommandsFor`); semantic names
  (`SpecArtifactBrowser`, `FilePreviewModal`, `MarkdownView`). Strong typing from the API contract;
  no `any`.
- **Modal/overlay pattern**: `SpecDetailsDrawer/index.tsx` already implements an overlay with
  `role="dialog"`, `aria-modal`, Escape-to-close, and `stopPropagation` on the panel — the file
  modal mirrors it at a higher stacking level.

No new patterns beyond those already in the project, except the single approved Markdown dependency.

---

## 4. Prerequisites

Before starting, the following must already exist (all verified present):

- [x] `SpecDetailsDrawer` as a folder component with composable sections and an overlay/dialog
      pattern (`web/src/components/SpecDetailsDrawer/index.tsx:42-115`).
- [x] Board `Source` interface satisfied by `*state.Store`
      (`cli/internal/board/board.go:131-136`).
- [x] Spec state carries `SpecDoc` (repo-relative) and `OpenSpec{Change, Artifacts{Proposal,
      Design, Tasks}}` (`cli/internal/state/types.go:79-131`).
- [x] `*state.Store` with `root` (the `.vector` dir), `ReadSpec(id)`
      (`cli/internal/state/store.go:17,376`), so the repo root is `filepath.Dir(s.root)`.
- [x] HTTP server with `Routes` registration and `writeJSONError`
      (`cli/internal/board/server.go:30-41,165`); `handleSummary` as the closest mirror
      (`server.go:138`).
- [x] Board `Card` projection (`cli/internal/board/board.go` `Card`) and the hand-mirrored web
      `Card` type (`web/src/types/board.ts:28-46`) carrying `hasOpenSpec` + `artifacts`.
- [x] Web fetch helpers `useFetchJSON` (`web/src/api/useFetchJSON.ts`) and the `useSpecSummary`/
      `useSpecActivity` hooks as the lazy-fetch shape to mirror.
- [x] `vector serve` wires `board.NewServer(store, filepath.Base(root))`
      (`cli/cmd/vector/serve.go:52-61`), so the server already holds the store as its `Source`.

If a prerequisite is missing, stop and report exactly what is absent. Do not invent contracts.

---

## 5. Architecture

### Pattern to use

**Read-only artifact projection + lazy client fetch + modal preview**, mirroring the existing
summary/activity surfaces. The web never reads the filesystem; it asks the cli for a file by
**spec id + artifact key**, and the cli resolves the path from trusted committed state. The board
performs no mutation; selection and modal state are local UI state only.

### Affected layers

- **presentation (web)**: yes — new `SpecArtifactBrowser`, `FilePreviewModal`, `MarkdownView`; a new
  `Files` section in the drawer; new `useFileContent` hook; `Card` gains `specDoc`.
- **application/use-cases (cli)**: yes — new `handleFile`; new `ReadSpecArtifact` on the `Source`
  interface; `Card` projection gains `SpecDoc`.
- **domain (cli state)**: no — file paths derive from existing `SpecState` fields; the state
  machine, `state.json`, and event log are unchanged.
- **data/infrastructure (cli)**: yes — file I/O with path resolution + traversal defense inside
  `*state.Store.ReadSpecArtifact`.
- **kit**: no.
- **shared/common**: no.

### Expected flow

1. The user clicks a `SpecCard`; `KanbanBoard` opens `SpecDetailsDrawer` (existing behavior).
2. The drawer renders the `Files` section → `SpecArtifactBrowser`, which builds a static list from
   `card.specDoc` and `card.artifacts` (no fetch yet).
3. The user clicks a file (e.g. `proposal.md`); `SpecArtifactBrowser` sets the selected artifact and
   renders `FilePreviewModal`.
4. `FilePreviewModal` calls `useFileContent(card.id, 'proposal')` →
   `GET /api/file?spec=<id>&artifact=proposal`.
5. `handleFile` validates `spec` + the `artifact` enum, calls `ReadSpecArtifact`, which resolves
   `openspec/changes/<change>/proposal.md` from state, defends the path, reads it, and returns the
   bytes as `text/markdown`.
6. The modal lazily mounts `MarkdownView` (code-split) and renders the Markdown; loading / error+
   retry handled inline.
7. The user closes the modal (button / `Escape` / overlay) — the drawer stays open. Closing the
   drawer unmounts everything (fetches torn down).

### Location of new files

```txt
cli/
  internal/state/artifact.go         # ReadSpecArtifact (resolution + traversal defense) — or co-locate in store.go
web/src/components/SpecDetailsDrawer/
  SpecArtifactBrowser.tsx            # the file list + selection state
  FilePreviewModal.tsx               # the overlay that fetches + previews one file
  MarkdownView.tsx                   # react-markdown + remark-gfm wrapper (lazy-imported)
web/src/api/useFileContent.ts        # text-fetch hook (mirrors useFetchJSON, returns string)
```

Modified: `cli/internal/board/board.go` (Source + Card.SpecDoc), `cli/internal/board/server.go`
(handleFile + route), `web/src/types/board.ts` (Card.specDoc), `web/src/components/
SpecDetailsDrawer/index.tsx` (Files section) and its `.module.css` (file-list + modal styles).
No new Go packages; reuse `internal/state`, `internal/board`.

---

## 6. Files to create or modify

| Path | Action | Purpose | Project example to follow |
|---|---|---|---|
| `cli/internal/state/artifact.go` | NUEVO | `ReadSpecArtifact(specID, artifact)` — resolve key→path from `SpecDoc`/`OpenSpec`, defend traversal, read bytes; wrapped `fs.ErrNotExist` when absent | `cli/internal/state/store.go` (path helpers, `ReadSpec`) |
| `cli/internal/board/board.go` | MODIFY | Add `ReadSpecArtifact(specID, artifact string) ([]byte, error)` to `Source`; add `SpecDoc string \`json:"specDoc,omitempty"\`` to `Card` and fill it in `Build` | `cli/internal/board/board.go:131-136` (interface), `Card` projection |
| `cli/internal/board/server.go` | MODIFY | Add `handleFile` + route `/api/file`; validate `spec`+`artifact` enum; map `fs.ErrNotExist`→404, else 500; serve raw bytes as `text/markdown` | `cli/internal/board/server.go:138-161` (`handleSummary`) |
| `web/src/types/board.ts` | MODIFY | Add `specDoc?: string` to `Card` | `web/src/types/board.ts:28-46` |
| `web/src/api/useFileContent.ts` | NUEVO | `useFileContent(spec, artifact)` → `GET /api/file?...`; text body; lazy on null artifact; `{data,loading,error,reload}` | `web/src/api/useFetchJSON.ts`, `useSpecSummary.ts` |
| `web/src/components/SpecDetailsDrawer/SpecArtifactBrowser.tsx` | NUEVO | Build file list from `card.specDoc`/`card.artifacts`; clickable items; own selected-artifact state; render `FilePreviewModal` | `web/src/components/SpecDetailsDrawer/UsefulCommands.tsx` |
| `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx` | NUEVO | Overlay; `useFileContent`; lazy `MarkdownView`; loading/error/content; a11y + Escape/overlay close without closing the drawer | `web/src/components/SpecDetailsDrawer/index.tsx:42-59` (overlay/dialog pattern) |
| `web/src/components/SpecDetailsDrawer/MarkdownView.tsx` | NUEVO | `react-markdown` + `remark-gfm` wrapper; the only importer of the new dep; lazy-loaded | new (smallest possible wrapper) |
| `web/src/components/SpecDetailsDrawer/index.tsx` | MODIFY | Add a `Files` section after `Activity` rendering `SpecArtifactBrowser` | `web/src/components/SpecDetailsDrawer/index.tsx:111-114` |
| `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css` | MODIFY | File-list item styles + modal overlay/panel/markdown styles | `SpecDetailsDrawer.module.css` (existing `.overlay`/`.drawer`/`.section`) |
| `web/package.json` | MODIFY | Add `react-markdown` + `remark-gfm` (pinned) | existing deps block |

### Detail per file

#### cli/internal/state/artifact.go

Action: NUEVO. `func (s *Store) ReadSpecArtifact(specID, artifact string) ([]byte, error)`:

- `spec, err := s.ReadSpec(specID)` — propagate not-found.
- Resolve a **repo-relative** path by key:
  - `"spec"` → `spec.SpecDoc`; if empty → wrapped `fs.ErrNotExist`.
  - `"proposal"|"design"|"tasks"` → require `spec.OpenSpec != nil` and the matching
    `Artifacts.{Proposal,Design,Tasks}` true, else wrapped `fs.ErrNotExist`; path
    `filepath.Join("openspec", "changes", spec.OpenSpec.Change, artifact+".md")`.
  - default → an error the handler treats as 400 (unknown key; but the handler validates the enum
    first, so this is defensive).
- `repoRoot := filepath.Dir(s.root)`; `abs := filepath.Join(repoRoot, rel)`; `abs =
  filepath.Clean(abs)`. Verify `abs` is within `repoRoot` (`strings.HasPrefix(abs, repoRoot+
  string(os.PathSeparator))`) **and** under `.vector/specs/<id>/` or `openspec/changes/<change>/`.
  On violation → a non-`fs.ErrNotExist` error (→ 500; should never happen with trusted state).
- `b, err := os.ReadFile(abs)`: wrap `os.IsNotExist`/`fs.ErrNotExist` as not-found; return other
  errors as-is. Return `b, nil`.

Restrictions: read-only; do not touch `state.json` or the state machine. Resolution uses only
committed spec state; no filesystem scan.

#### cli/internal/board/board.go

Action: MODIFY.

- Add `ReadSpecArtifact(specID, artifact string) ([]byte, error)` to the `Source` interface
  (`*state.Store` satisfies it once `artifact.go` exists).
- Add `SpecDoc string \`json:"specDoc,omitempty"\`` to `Card`; set it from the spec's `SpecDoc` in
  `toCard` (`cli/internal/board/board.go:190-221`, the per-spec projection called from `Build`).
  Pure projection; no I/O.

#### cli/internal/board/server.go

Action: MODIFY.

- Register `mux.HandleFunc("/api/file", s.handleFile)` in `Routes` (line 36 area).
- `handleFile(w, r)`:
  - `spec := r.URL.Query().Get("spec")`; 400 `"missing spec query parameter"` if empty.
  - `artifact := r.URL.Query().Get("artifact")`; 400 `"missing artifact query parameter"` if empty;
    validate against `{spec, proposal, design, tasks}` → 400 `"unknown artifact"` otherwise.
  - `b, err := s.src.ReadSpecArtifact(spec, artifact)`; `errors.Is(err, fs.ErrNotExist)` → 404
    `"file not found"`; other err → 500 `"could not read file"`.
  - Success: `Content-Type: text/markdown; charset=utf-8`, `Cache-Control: no-store`, `w.Write(b)`.

#### web/src/types/board.ts

Action: MODIFY. Add `specDoc?: string` to `Card` (mirrors the new Go `Card.SpecDoc`).

#### web/src/api/useFileContent.ts

Action: NUEVO. `useFileContent(spec: string | null, artifact: ArtifactKey | null):
AsyncState<string>`. URL `null` when either is null (lazy). Mirror `useFetchJSON` internals but
`res.ok ? res.text() : throw parseError(res)`. `ArtifactKey = 'spec'|'proposal'|'design'|'tasks'`.
Export `AsyncState`-compatible `{ data, loading, error, reload }`.

#### web/src/components/SpecDetailsDrawer/SpecArtifactBrowser.tsx

Action: NUEVO. Props `{ card: Card }`. Compute the list:

- `card.specDoc` → `{ key: 'spec', name: basename(card.specDoc) || 'spec.md' }`.
- `card.artifacts?.proposal` → `{ key: 'proposal', name: 'proposal.md' }`; same for `design`,
  `tasks`.

Render a vertical list of buttons (file icon + name), keyboard-activable (`<button>`); empty state
`"No source files available."` when the list is empty. Own `const [open, setOpen] =
useState<ArtifactKey | null>(null)`; on click set it; render `{open && <FilePreviewModal
specId={card.id} artifact={open} fileName={...} onClose={() => setOpen(null)} />}`.

#### web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx

Action: NUEVO. Props `{ specId: string; artifact: ArtifactKey; fileName: string; onClose: () =>
void }`. `useFileContent(specId, artifact)`. Overlay > drawer (higher `z-index`), `role="dialog"`,
`aria-modal="true"`, `aria-label={fileName}`. Header: `fileName` + close button (focus on open).
Body: loading (`loading file…`), error (`could not load file: {error}` + `Retry` → `reload`),
content (`<Suspense fallback={<p className={styles.muted}>loading file…</p>}><MarkdownView
source={data} /></Suspense>` with `MarkdownView` = `React.lazy(() => import('./MarkdownView'))` —
the chunk-loading fallback reuses the same "loading file…" string as the fetch-loading state). Close
on button / `Escape` / overlay click;
`stopPropagation` on the panel and **handle Escape locally + stopPropagation** so the drawer's
Escape handler does not also fire. Restore focus to the triggering list item on close (best-effort).

#### web/src/components/SpecDetailsDrawer/MarkdownView.tsx

Action: NUEVO. Default export. `function MarkdownView({ source }: { source: string }) { return
<ReactMarkdown remarkPlugins={[remarkGfm]}>{source}</ReactMarkdown> }`. No `rehype-raw`, no raw
HTML. The only module importing `react-markdown`/`remark-gfm`; loaded via `React.lazy` from the
modal to keep it out of the initial bundle.

#### web/src/components/SpecDetailsDrawer/index.tsx

Action: MODIFY. After the `Activity` section (line 114) add:

```tsx
<section className={styles.section}>
  <h3 className={styles.sectionTitle}>Files</h3>
  <SpecArtifactBrowser card={card} />
</section>
```

No change to the other sections.

#### web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css

Action: MODIFY. Add file-list item styles (button reset, hover/focus) and modal styles (overlay,
centered panel, header, scrollable markdown body with the existing tokens). Reuse the existing
`.overlay`/`.drawer` variables for consistency.

#### web/package.json

Action: MODIFY. Add `react-markdown` and `remark-gfm` at pinned, React-19-compatible versions.
This is the single approved dependency addition (Section 10).

---

## 7. API Contract

The Go structs are the source of truth; `web/src/types/*` mirrors them by hand. Changes:

- **New** `GET /api/file?spec=<id>&artifact=<key>` where `key ∈ {spec, proposal, design, tasks}`.
  - `200` → **raw file bytes**, `Content-Type: text/markdown; charset=utf-8`,
    `Cache-Control: no-store`. (Not a JSON envelope — the body is the Markdown.)
  - `400` `{"error":"missing spec query parameter"}` / `{"error":"missing artifact query
    parameter"}` / `{"error":"unknown artifact"}`.
  - `404` `{"error":"file not found"}` — spec absent, artifact flag unset, or file missing on disk.
  - `500` `{"error":"could not read file"}`.
- **New** `Card.specDoc` (optional, `omitempty`) on `GET /api/board` and the SSE board stream — the
  repo-relative spec-doc path. Additive; existing fields unchanged.
- `GET /api/summary` / `GET /api/activity` / `GET /api/standup` / `GET /api/board` (other fields)
  are **unchanged**.

CLI surface: no new subcommand (the endpoint is web-only; the binary already serves files via the
running `vector serve`).

### Endpoints involved

- GET /api/file?spec=<id>&artifact=<key> (new)
- GET /api/board + SSE (existing; gains `Card.specDoc`)

---

## 8. Success criteria

The implementation is correct when:

- [ ] `ReadSpecArtifact` resolves `spec`→`SpecDoc` and `proposal`/`design`/`tasks`→
      `openspec/changes/<change>/<name>.md`, gating on the `Artifacts` flags; returns wrapped
      `fs.ErrNotExist` when absent; defends the resolved path against escaping the repo root /
      expected prefixes.
- [ ] `GET /api/file` returns 200 + Markdown for an existing artifact, 400 for missing/unknown
      params, 404 for absent spec/artifact/file, 500 for a read error; success is `text/markdown`,
      not JSON.
- [ ] `Card.specDoc` is projected by `Build` and present in `web/src/types/board.ts`.
- [ ] `useFileContent` fetches text lazily (null artifact → idle) with loading/error/reload.
- [ ] `SpecArtifactBrowser` lists exactly the files that exist for the spec (`spec.md` when
      `specDoc`; each OpenSpec artifact per its flag); empty state otherwise.
- [ ] Clicking a file opens `FilePreviewModal`, which renders the Markdown (GFM checkboxes/tables in
      `tasks.md` render); the renderer is code-split (not in the initial chunk).
- [ ] The modal closes on button/`Escape`/overlay **without** closing the drawer; the drawer's own
      Escape still works when no modal is open.
- [ ] No regression: existing drawer sections, `/api/board`, SSE, summary/activity/standup intact.
- [ ] `go vet`/`go test` green; web typecheck + build succeed; binary rebuilt + reinstalled.

### Required tests

- [ ] `ReadSpecArtifact`: `spec` key resolves `SpecDoc` and reads it; `proposal`/`design`/`tasks`
      gated by flags (flag off → `fs.ErrNotExist`); unknown/absent spec → not-found; a crafted
      `OpenSpec.Change` containing `..` is rejected by the prefix/root check (defense in depth).
- [ ] `handleFile`: 200 (existing), 400 (missing `spec`, missing/unknown `artifact`), 404 (absent),
      and the success `Content-Type` is `text/markdown`.
- [ ] (web) typecheck passes with the new types/hook/components; build succeeds; the Markdown chunk
      is split (verify the renderer is not in the main entry chunk).

### Verification commands

```bash
go -C cli vet ./...
go -C cli test ./...
npm --prefix web run typecheck
npm --prefix web run build
```

The phase is not complete if any of these fail.

---

## 9. UX criteria

### Files section (in the drawer)

- A `Files` section, visually consistent with the other sections (same `sectionTitle`, spacing).
- One clickable item per available file (file icon + name); hover/focus affordance; keyboard
  activable (`<button>`, Enter/Space).
- Empty state `"No source files available."` (muted) when the spec has no `specDoc` and no
  artifacts.

### Preview modal

- Opens centered over a dim overlay, stacked **above** the drawer.
- Header: the file name + a close button (`aria-label="Close file"`), focused on open.
- Body: while loading, `loading file…`; on error, `could not load file: <msg>` + `Retry`; on
  success, the rendered Markdown in a scrollable region.
- **Close**: close button, `Escape`, and overlay click — each closes **only the modal**, leaving the
  drawer open; focus returns to the file item.
- Content scrolls inside the modal; comfortable hit target for the close button on mobile.

### Accessibility

- Modal: `role="dialog"`, `aria-modal="true"`, labelled by the file name; focus enters on open and
  is restored on close; the close button has an `aria-label`.
- Rendered Markdown preserves heading/list/code/table semantics (via `react-markdown` + GFM).

---

## 10. Decisions made

Settled by the user — do not re-litigate:

- **In-board modal** preview (not a new browser tab). The modal stacks above the drawer and closes
  independently of it.
- **Rendered Markdown** (not raw text), via **`react-markdown` + `remark-gfm`**. This is a
  **deliberate, approved deviation** from the "no new libraries / light bundle" rule, mitigated by
  isolating the dep in `MarkdownView` and **lazy-loading** it (code-split) so it is excluded from
  the initial board bundle.
- **Scope = `spec.md` + the three OpenSpec artifacts only.** Fixed whitelist; no custom docs, no
  filesystem scan; the list is derived from committed spec state.
- **The client never sends a path.** The endpoint takes `spec` + an `artifact` **enum**; the binary
  resolves the on-disk path server-side from trusted state — path traversal is removed by design,
  with an extra defensive root/prefix check.
- **Read-only.** No editing/uploading; the board exposes no write surface (consistent with
  `add-spec-summary-drawer`).
- **Path resolution + I/O live in the cli/state layer** (`ReadSpecArtifact` on `*state.Store`,
  exposed through the `Source` interface), not inline in the HTTP handler.

If the agent sees a seemingly better alternative, report it as an observation; do not implement it.

---

## 11. Edge cases

### Spec has no `specDoc`

- The `spec.md` entry is omitted. In practice every spec gets a `SpecDoc` at create time, so this is
  rare; handled gracefully (no entry, not an error).

### Spec has OpenSpec but a given artifact is absent

- Only artifacts whose `Artifacts` flag is set are listed; requesting an unset one returns 404
  (defense even though the UI won't offer it).

### Draft spec (no OpenSpec change yet)

- Only `spec.md` is listed (no proposal/design/tasks until `/vector:propose`). Common for fresh
  `/vector:raw` cards.

### File listed in state but missing on disk

- `os.ReadFile` fails with not-exist → 404 `"file not found"`; the modal shows the error + `Retry`.

### `OpenSpec.Change` malformed / containing `..`

- Defense-in-depth: the resolved path must stay under the repo root and the expected
  `openspec/changes/<change>/` prefix; a violation returns a 500 (not a file leak). Should not occur
  with trusted committed state.

### Empty file (0 bytes)

- 200 with an empty body; `MarkdownView` renders nothing; the modal shows an empty (but valid)
  content area.

### Very large file

- Served in full (no truncation). `react-markdown` parses client-side; a pathologically large
  `spec.md` is a perf consideration, not a correctness one (`TBD — ver Open questions` on an
  optional size cap).

### Modal open while the drawer would close on Escape

- Escape is handled at the modal first (`stopPropagation`); it closes the modal only. With no modal
  open, Escape closes the drawer (existing behavior).

### Another card selected while a modal is open

- The drawer remounts for the new card; the modal is local to the browser instance and unmounts with
  the old drawer. No stale fetch (cancellation on unmount).

### Clipboard / popup blockers

- N/A — no new tab, no clipboard use in this feature.

### Board offline / `vector serve` down

- `GET /api/file` doesn't resolve → the modal shows its error + `Retry` (same handling as
  `useSpecActivity`). No new dependency on availability.

### Markdown with embedded raw HTML / scripts

- Not rendered as HTML (no `rehype-raw`); treated as text. No XSS surface even though content is the
  user's own local files.

### Timeout (file fetch)

- `useFileContent` (like `useFetchJSON`) sets no explicit request timeout; a stalled `GET /api/file`
  resolves as a fetch error. The modal shows the inline error + `Retry` — identical to the offline
  case. No new behavior is introduced.

### Double submit

- N/A — every server endpoint is GET (read-only); there is no web form and no mutation to
  double-submit. Re-clicking the same file re-opens the modal idempotently; `Retry` just re-issues
  the same GET.

### HTTP error codes (400/401/403/404/409/422/429/500)

- Local, auth-free, read-only server. `/api/file`: 400 (missing/unknown param), 404 (absent),
  500 (read error). No auth/conflict/validation codes apply.

---

## 12. Required UI states

| State | What is shown | What the user can do |
|---|---|---|
| files section, has files | one button per available file | click/Enter to open a file |
| files section, empty | "No source files available." | read other sections; close drawer |
| modal, loading | "loading file…" | wait / close |
| modal, content | rendered Markdown (scrollable) | read; scroll; close |
| modal, error / 404 | "could not load file: …" + Retry | retry / close |
| offline (`vector serve` down) | modal shows the fetch error + Retry | retry / close |

---

## 13. Validations

Read-only; no user-facing forms. Server-side validation only:

| Input | Rule | Message / status |
|---|---|---|
| `spec` query param | required, non-empty | `400 missing spec query parameter` |
| `artifact` query param | required | `400 missing artifact query parameter` |
| `artifact` value | ∈ {spec, proposal, design, tasks} | `400 unknown artifact` |
| resolved path | within repo root + expected prefix | `500 could not read file` (defensive) |
| file existence | must exist on disk | `404 file not found` |

---

## 14. Security and permissions

- **No path from the client.** The endpoint accepts only a spec id and an artifact enum; the path is
  computed server-side from committed state, so classic `?path=../../etc/passwd` traversal is
  structurally impossible.
- **Defense in depth.** Even the state-derived path is `Clean`ed and checked to remain under the
  repo root and the expected `.vector/specs/<id>/` / `openspec/changes/<change>/` prefix.
- **No HTML injection.** Markdown is rendered without raw-HTML support; no `dangerouslySetInnerHTML`.
- **No secrets.** Content is the user's own spec/OpenSpec docs; the local server is auth-free and
  read-only and already exposes the board. No new write surface, no credential exposure.

---

## 15. Observability and logging

- No new event types in `activity.jsonl`; reading a file is a diagnostic GET, not a domain action.
- `handleFile` surfaces errors to the client (400/404/500) via `writeJSONError`; no extra logging
  infrastructure. Reuse the existing read-error paths.

---

## 16. i18n / visible text

Vector has no i18n layer; CLI/web strings are English in code. New visible strings:

| Key | Text |
|---|---|
| drawer.files.title | "Files" |
| drawer.files.empty | "No source files available." |
| modal.file.loading | "loading file…" |
| modal.file.error | "could not load file: " + "Retry" |
| modal.file.close | aria-label "Close file" |

File names are derived from the artifact (`spec.md`, `proposal.md`, `design.md`, `tasks.md`).
Exact wording is `TBD — ver Open questions` if the user wants different copy.

---

## 17. Performance

- The file list is **static** (derived from the already-loaded card); it triggers no fetch.
- File content is fetched **lazily on click**, and the `react-markdown` renderer is **code-split**
  via `React.lazy` so it loads only when the first modal opens — the initial board bundle stays
  light (the key mitigation for the new dependency).
- `useFileContent` cancels in-flight fetches on unmount (mirrors the existing hooks).
- A single `os.ReadFile` per request; no caching layer (artifacts are small and local).

---

## 18. Restrictions

The agent must not:

- Add web write endpoints or mutate state (board stays read-only).
- Accept a client-supplied file path; only the spec id + artifact enum.
- Serve anything beyond `spec.md` + the three OpenSpec artifacts; no filesystem scan, no custom
  docs.
- Add any web dependency other than `react-markdown` + `remark-gfm`; do not enable `rehype-raw` or
  raw-HTML rendering.
- Import the Markdown renderer eagerly (it must stay code-split out of the initial bundle).
- Put file I/O or path resolution in the HTTP handler instead of the cli/state layer.
- Touch `state.json`, the state machine, the event log, or the standup/summary contracts.
- Use `any` / `interface{}` outside justified (de)serialization boundaries.
- Refactor unrelated code or change other drawer/board/standup views beyond what is listed.
- Ignore lint/vet/typecheck/test failures.

---

## 19. Deliverables

On completion:

- [ ] `ReadSpecArtifact` on `*state.Store` (resolution + traversal defense) with tests.
- [ ] `Source.ReadSpecArtifact` + `Card.SpecDoc` projection.
- [ ] `GET /api/file` handler with tests (200/400/404/500, `text/markdown` success).
- [ ] `Card.specDoc` in `web/src/types/board.ts`; `useFileContent` text-fetch hook.
- [ ] `SpecArtifactBrowser`, `FilePreviewModal`, `MarkdownView` (lazy) + the `Files` section in the
      drawer + CSS.
- [ ] `react-markdown` + `remark-gfm` pinned in `web/package.json`, isolated + code-split.
- [ ] Gate green: `go vet`, `go test`, web typecheck, web build.
- [ ] Binary rebuilt + reinstalled to `~/.local/bin/vector` (dogfooding uses the PATH binary).

---

## 20. Final checklist for the agent

- [ ] Read this whole spec.
- [ ] Confirmed the reuse seams (`Source` interface, `writeJSONError`, the drawer overlay pattern,
      the lazy-fetch hooks) — no new layers beyond those listed.
- [ ] `ReadSpecArtifact` resolves only from committed state, gates on the artifact flags, and
      defends the path against escaping the repo root / expected prefixes.
- [ ] `/api/file` takes `spec` + `artifact` enum (no path), serves `text/markdown`, maps
      absent→404 / read-error→500 / bad-param→400.
- [ ] `Card.specDoc` added on both sides; `useFileContent` returns text, not JSON.
- [ ] The renderer dep is isolated in `MarkdownView` and **lazy-loaded** (verified code-split).
- [ ] The modal stacks above the drawer and closes independently (Escape handled locally).
- [ ] Added Go tests (resolution, flag-gating, traversal defense, handler codes); web typecheck +
      build green.
- [ ] Ran `go vet`, `go test`, web typecheck, web build.
- [ ] Rebuilt and reinstalled the `vector` binary.
- [ ] Left no temporary logs or unjustified TODOs.
