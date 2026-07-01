# Tasks — add-ui-sketch-generation

## 1. State model (`cli/internal/state`)

- [x] 1.1 `types.go`: add `SketchRef` struct (`Name string \`json:"name"\``, `CreatedAt time.Time
      \`json:"createdAt"\``) + `Sketches []SketchRef \`json:"sketches,omitempty"\`` on `SpecState`
      (next to `QuickWin`). `SchemaVersion` stays 1; legacy state loads with `Sketches == nil`.
- [x] 1.2 `artifact.go`: add case `"sketch"` to `artifactRelPath` → path under
      `.vector/specs/<id>/sketches/<Sketches[0].Name>`; return `fs.ErrNotExist` when `Sketches` is
      empty. Verify `verifyArtifactPath`'s `isUnder` covers `sketches/` under the existing
      `.vector/specs/<id>/` prefix; add an explicit prefix only if not covered.
- [x] 1.3 `store.go`: add `Store.AttachSketch(id string, file []byte, ref SketchRef) error` —
      `MkdirAll` the `sketches/` dir, atomically write the bytes, read the `SpecState`, append `ref`,
      persist `state.json` atomically. Mirror `RouteAgent`/`CreateSpec`. No LLM calls.

## 2. CLI (`cli/cmd/vector`)

- [x] 2.1 `sketch.go` (NEW): `runSpecAttachSketch(args)` — flags `--file` (required), `--name`
      (default `filepath.Base(--file)`), `--repo-root`, `--json`; parse leading id; read file;
      `json.Unmarshal`; verify `type`/`version`/`elements` keys; sanitize `--name` (reject `/`, `..`,
      dangerous chars); `openStore`; `AttachSketch`; output text or `{"id":…,"sketch":…}`. Mirror
      `route.go`.
- [x] 2.2 `main.go`: register `case "attach-sketch": err = runSpecAttachSketch(args[1:])` in the
      internal `runSpec` dispatch. Do not touch other subcommands.

## 3. Config (`cli/internal/config`)

- [x] 3.1 `config.go`: add `SketchEnabled *bool \`json:"sketchEnabled,omitempty"\`` to `Config` +
      `IsSketchEnabled() bool` (`nil` or `true` = enabled; only explicit `false` disables). Do not bump
      the config `SchemaVersion`.

## 4. Board projection + web (read-only download)

- [x] 4.1 `cli/internal/board/board.go`: add `Sketches []state.SketchRef \`json:"sketches,omitempty"\``
      to `Card`; propagate in `toCard`. Confirm whether `board.SchemaVersion` (2) needs a bump (open
      question) — default keep at 2 (additive/omitempty).
- [x] 4.2 `cli/internal/board/server.go`: add `case "sketch": return true` to `validArtifact`; make
      `handleFile` Content-Type conditional — `application/octet-stream` +
      `Content-Disposition: attachment; filename="<sketchName>"` for sketch, `text/markdown` otherwise.
      Do not leak the internal absolute path in headers.
- [x] 4.3 `web/src/types/board.ts`: add `SketchRef` interface (`name`, `createdAt`) + `sketches?:
      SketchRef[]` on `Card`.
- [x] 4.4 `web/src/api/useFileContent.ts`: extend `ArtifactKey` with `| 'sketch'` (type completeness
      only; no blob/fetch — download is a native `<a href download>`).
- [x] 4.5 `web/src/components/SpecDetailsDrawer/entries.ts`: add `download?: boolean` to `ArtifactEntry`;
      in `entriesFor`, push one `{ key: 'sketch', label: sketch.name, download: true }` per
      `card.sketches`.
- [x] 4.6 `web/src/components/SpecDetailsDrawer/SpecArtifactBrowser.tsx`: for `entry.download === true`,
      render a real `<a href="/api/file?spec=${id}&artifact=sketch" download={entry.label}>` with the
      `Download` icon (aria-hidden) + explicit `aria-label`; no `setSelected`/`FilePreviewModal`. Other
      artifacts unchanged.

## 5. Agent (Sonnet)

- [x] 5.1 `kit/agents/vector-ui-ux-designer.md` (NEW): frontmatter `name: vector-ui-ux-designer`,
      `model: sonnet`, `tools: Read, Write, Bash`; read `.claude/agents/_shared/prose-rules.md`; vendor
      the `.excalidraw` format knowledge (`{type, version, elements, appState, files}` + element
      shapes); input `SPEC_PATH`/`SPEC_ID`/`OUTPUT_PATH`; `Write` pure JSON to the temp path and call
      `vector spec attach-sketch`. Hard rules: no `~/.claude/`/MCP assumption, no network, only writes
      the temp file under `.vector/tmp/` (never `.vector/specs/` or `state.json`). Pattern:
      `kit/agents/vector-standup-writer.md` + `vector-spec-composer.md`.

## 6. Commands (kit)

- [x] 6.1 `kit/commands/vector/raw.md`: add tail **step 12 — Sketch Excalidraw (opt-in)** after report:
      opt-out check (`--no-sketch` / `sketchEnabled === false`) → UI heuristic over the composed spec →
      `AskUserQuestion` → async spawn of `vector-ui-ux-designer` (pass `SPEC_PATH`/`SPEC_ID`) → register
      routing. Do not reorder steps 1–11.
- [x] 6.2 `kit/commands/vector/research.md`: add the same logic as tail **step 15** after report. Do not
      reorder steps 0–14.

## 7. Vendoring / scaffold (`cli/internal/scaffold`)

- [x] 7.1 `go generate ./internal/scaffold` copies `vector-ui-ux-designer.md` into
      `assets/agents/` (`//go:generate` + `//go:embed all:assets`). Never edit `assets/` by hand.
- [x] 7.2 `scaffold_test.go`: `TestAssetsMatchKit` stays green (include the new agent in the expected
      set if it enumerates one).

## 8. Tests

- [x] 8.1 `state/types`: `SketchRef`/`Sketches` JSON round-trip (set/omitted); legacy `SpecState`
      without the field loads as `Sketches == nil`.
- [x] 8.2 `state/artifact`: `artifactRelPath("sketch", …)` → correct path with a sketch present;
      `fs.ErrNotExist` when none.
- [x] 8.3 `state/store` (or `cmd/vector/sketch_test.go`): `AttachSketch`/`runSpecAttachSketch` — valid
      JSON persists file + updates state; invalid JSON → descriptive error; missing spec → error.
- [x] 8.4 `board/board`: `Build` projects `Card.Sketches` when present; absent when the spec has none.
- [x] 8.5 `entries.test.ts` (vitest): card with sketches → entries with `download: true`; empty/no
      sketches → no sketch entries; two sketches → two entries. Existing tests stay green.

## 9. Docs

- [x] 9.1 Update `docs/schemas/state-and-activity.md` (the `sketches` field + `SketchRef`),
      `docs/plugin-and-commands.md` (the tail sketch step + `vector spec attach-sketch` + the new
      agent), and `docs/domain-contract.md` if the artifact contract is documented there.

## 10. Verification gate

- [x] 10.1 `go -C cli generate ./internal/scaffold` → `gofmt -l cli` (empty) → `go -C cli vet ./...` →
      `go -C cli test ./...` → `go -C cli build ./...` all green.
- [x] 10.2 `npm --prefix web run typecheck && npm --prefix web run lint && npm --prefix web test &&
      npm --prefix web run build` all green (no anomalous bundle-size warnings).
- [x] 10.3 `vector init`/`update` in a clean repo seeds `vector-ui-ux-designer.md`.
