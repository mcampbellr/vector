# Tasks — add-spec-file-viewer

## 1. State — read-only artifact resolution

- [x] 1.1 `cli/internal/state/artifact.go`: `func (s *Store) ReadSpecArtifact(specID, artifact string) ([]byte, error)` — `ReadSpec(specID)` (propagate not-found); resolve key→repo-relative path (`spec` → `SpecDoc`, empty → wrapped `fs.ErrNotExist`; `proposal`/`design`/`tasks` → require `OpenSpec != nil` + matching `Artifacts` flag, else wrapped `fs.ErrNotExist`; path `openspec/changes/<change>/<key>.md`).
- [x] 1.2 Defense in depth: `repoRoot := filepath.Dir(s.root)`; `abs := filepath.Clean(filepath.Join(repoRoot, rel))`; verify `abs` stays under `repoRoot` and under `.vector/specs/<id>/` or `openspec/changes/<change>/`; violation → non-`fs.ErrNotExist` error (→ 500). `os.ReadFile`: wrap not-exist as `fs.ErrNotExist`, return other errors as-is.
- [x] 1.3 Tests: `spec` key resolves `SpecDoc` and reads it; `proposal`/`design`/`tasks` gated by flags (flag off → `fs.ErrNotExist`); absent/unknown spec → not-found; crafted `OpenSpec.Change` with `..` rejected by the prefix/root check.

## 2. Board — Source + Card projection + handler

- [x] 2.1 `cli/internal/board/board.go`: add `ReadSpecArtifact(specID, artifact string) ([]byte, error)` to the `Source` interface (`*state.Store` satisfies it once `artifact.go` exists).
- [x] 2.2 `cli/internal/board/board.go`: add `SpecDoc string \`json:"specDoc,omitempty"\`` to `Card`; set it from the spec's `SpecDoc` in the per-spec projection (`toCard`). Pure projection, no I/O.
- [x] 2.3 `cli/internal/board/server.go`: register `mux.HandleFunc("/api/file", s.handleFile)`; `handleFile` validates `spec` (400 if empty) + `artifact` against `{spec, proposal, design, tasks}` (400 if missing/unknown), calls `ReadSpecArtifact`, maps `fs.ErrNotExist`→404 / other→500 (via `writeJSONError`), success → raw bytes, `Content-Type: text/markdown; charset=utf-8`, `Cache-Control: no-store`.
- [x] 2.4 Handler tests: 200 + `text/markdown` for an existing artifact; 400 (missing `spec`, missing/unknown `artifact`); 404 (absent spec/artifact/file); 500 (read error).

## 3. Web — Files section + preview modal

- [x] 3.1 `web/src/types/board.ts`: add `specDoc?: string` to `Card` (mirrors the Go `Card.SpecDoc`).
- [x] 3.2 `web/src/api/useFileContent.ts`: `useFileContent(spec, artifact)` → `GET /api/file?spec=&artifact=`; mirror `useFetchJSON`'s `{data, loading, error, reload}` but resolve `res.text()`; lazy (`null` artifact → idle); `ArtifactKey = 'spec'|'proposal'|'design'|'tasks'`.
- [x] 3.3 `web/src/components/SpecDetailsDrawer/SpecArtifactBrowser.tsx`: build the list from `card.specDoc` (→ `spec`) and `card.artifacts?.{proposal,design,tasks}`; clickable/keyboard-activable `<button>` items; empty state `"No source files available."`; own `selected artifact` state; render `FilePreviewModal` on selection.
- [x] 3.4 `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx`: overlay above the drawer (`role="dialog"`, `aria-modal`, labelled by file name, focus close button on open); `useFileContent`; loading / error+`Retry` / content states; `<Suspense>` + `React.lazy` `MarkdownView`; close on button/`Escape`/overlay with `stopPropagation` so the drawer stays open; restore focus on close (best-effort).
- [x] 3.5 `web/src/components/SpecDetailsDrawer/MarkdownView.tsx`: default export, thin `react-markdown` + `remark-gfm` wrapper; no `rehype-raw` / raw HTML; the only importer of the new dep; loaded via `React.lazy`.
- [x] 3.6 `web/src/components/SpecDetailsDrawer/index.tsx`: add a `Files` section after `Activity` rendering `SpecArtifactBrowser`; `SpecDetailsDrawer.module.css`: file-list item styles + modal overlay/panel/markdown styles (reuse existing tokens).
- [x] 3.7 `web/package.json`: add `react-markdown` + `remark-gfm` (pinned, React-19-compatible) — the single approved dependency addition.

## 4. Verification

- [x] 4.1 `go -C cli vet ./...` and `go -C cli test ./...` green.
- [x] 4.2 `npm --prefix web run typecheck` and `npm --prefix web run build` green; verify the Markdown renderer is code-split (not in the main entry chunk).
- [x] 4.3 Rebuild + reinstall the `vector` binary to `~/.local/bin/vector` (dogfooding).
