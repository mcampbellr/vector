# Fix convention-store file preview returning HTTP 500

> Bug investigation/fix. Frame: the allowlist in `verifyArtifactPath` is out of sync with the
> convention spec store that `CreateSpec` already supports.

## Why

The board's file preview (`GET /api/file?spec=<id>&artifact=spec`, rendered in the
`SpecDetailsDrawer` `FilePreviewModal`) fails with **HTTP 500 "could not read artifact"** for any
spec whose `SpecDoc` lives **outside `.vector/`** — i.e. specs created under `specStore: convention`.
Native-store specs (`SpecDoc` inside `.vector/specs/<id>/`) and OpenSpec changes work; convention
specs never can.

The file exists and is readable. `ReadSpecArtifact` (`cli/internal/state/artifact.go`) resolves the
`SpecDoc` to a correct absolute path, but `verifyArtifactPath` — a defense-in-depth allowlist —
permits only two prefixes:

- `<repoRoot>/.vector/specs/<id>/`
- `<repoRoot>/openspec/changes/<change>/` (only when `OpenSpec` is set)

A convention path such as `code/main/docs/specs/<slug>/spec.md` is under neither, so
`verifyArtifactPath` returns `"artifact path … outside allowed locations"` — a **non-`fs.ErrNotExist`**
error, which `cli/internal/board/server.go`'s `handleFile` maps to **500**, not 404.

This contradicts `CreateSpec` (`cli/internal/state/store.go:113-119`), which legitimately writes the
`SpecDoc` to the caller-supplied convention path. The allowlist was never updated to include the
`SpecDoc`'s own location, so it is out of sync with the convention store. CI never caught it because
`artifact_test.go` only covers the native store (SpecDoc inside `.vector/specs/<id>/`).

## What changes

- **Allowlist includes the `SpecDoc` path (cli)** — in `verifyArtifactPath`, when `spec.SpecDoc != ""`,
  add `filepath.Join(repoRoot, filepath.FromSlash(spec.SpecDoc))` to the allowed list. `isUnder(prefix, abs)`
  returns true when `prefix == abs`, so the exact committed `SpecDoc` file (and only it) is permitted.
  The `repoRoot`-escape check (`isUnder(repoRoot, abs)`) still runs first, so traversal defense is
  unchanged. Update the function's doc comment (and the file-level comment) to record that the
  `SpecDoc`'s own location is an allowed source, since it is already-trusted committed state.
- **Close the test gap (cli)** — add a case to `cli/internal/state/artifact_test.go` that creates a
  spec with a convention `SpecDoc` **outside `.vector/`** (e.g. `code/main/docs/specs/alpha/spec.md`)
  and asserts `ReadSpecArtifact(id, "spec")` returns the body. `TestReadSpecArtifactRejectsTraversal`
  must stay green (the `..`-traversal change name is still rejected by the root-escape check).

## Capabilities

### Modified Capabilities

- `spec-file-viewer`: the read-only `GET /api/file` endpoint now serves a spec's `spec.md` when the
  `SpecDoc` lives outside `.vector/` (convention store), in addition to native-store and OpenSpec
  artifacts. The path is still resolved server-side from committed state; no client path, no disk
  scan; traversal defense (repo-root escape rejection) is unchanged.

## Scope

- `cli/internal/state/artifact.go` — `verifyArtifactPath`: extend the allowlist with the `SpecDoc`
  path; update the doc/file comments.
- `cli/internal/state/artifact_test.go` — add the convention-store coverage case.

## Non-goals

- Changing the security model or weakening the traversal defense (the repo-root escape check stays).
- Allowing access to sibling files in the convention directory — only the exact committed `SpecDoc`
  file is permitted (`isUnder` with `prefix == abs`).
- Touching the convention store feature itself, `CreateSpec`, the `web/` client, or the HTTP
  error-mapping in `server.go`.
- Changing how OpenSpec-change artifacts (`proposal`/`design`/`tasks`) are resolved or gated.

## Reproduction steps

1. Have a spec whose `state.json` carries a convention `SpecDoc` outside `.vector/`, e.g.
   `"specDoc": "code/main/docs/specs/<slug>/spec.md"`, with the file present and readable.
2. Open that spec's card in the board's `SpecDetailsDrawer`.
3. Open its `spec.md` in the `FilePreviewModal` (i.e. `GET /api/file?spec=<id>&artifact=spec`).
4. Observe HTTP 500 with `{"code":500,"error":"could not read artifact"}`; server-side
   `verifyArtifactPath` rejects the path as "outside allowed locations".

## Expected behavior

The preview returns **HTTP 200** with the `spec.md` bytes (`text/markdown`), rendered as Markdown in
the modal, for convention-store specs just as for native-store specs.

## Actual behavior

**HTTP 500** `{"code":500,"error":"could not read artifact"}`. `ReadSpecArtifact` returns the
non-`fs.ErrNotExist` allowlist error from `verifyArtifactPath`, which `handleFile` maps to 500.

## Acceptance criteria

- [ ] `verifyArtifactPath` permits the spec's `SpecDoc` path when non-empty; a convention-store
      `spec.md` outside `.vector/` is read successfully.
- [ ] `ReadSpecArtifact(id, "spec")` for a convention-store spec returns the file body (→ `/api/file`
      returns 200 `text/markdown`).
- [ ] `TestReadSpecArtifactRejectsTraversal` remains green (root-escape traversal still rejected as a
      non-`fs.ErrNotExist`, 500-class error).
- [ ] New test covers a convention `SpecDoc` outside `.vector/`.
- [ ] `go test ./cli/internal/state/... ./cli/internal/board/...` is green.

## Test plan

1. Add `TestReadSpecArtifactConventionStore` (or extend the SpecDoc test) in `artifact_test.go`:
   - `Open(t.TempDir())`.
   - `CreateSpec` with `SpecDocRel: "code/main/docs/specs/alpha/spec.md"` (and matching
     `SpecDocAbsPath` / `Body`) so the doc is written outside `.vector/`.
   - `ReadSpecArtifact("alpha", "spec")` returns the authored body.
2. Re-run `TestReadSpecArtifactRejectsTraversal` and the rest of the state suite to confirm no
   regression in the traversal defense.
3. `go test ./cli/internal/state/... ./cli/internal/board/...` green.
4. Rebuild + reinstall the `vector` binary to `~/.local/bin/vector` (dogfooding) and confirm the
   preview opens for a convention-store spec.

## Risks

- **Low.** The added path comes from already-persisted, committed state and is gated behind the
  unchanged `repoRoot`-escape check (`isUnder(repoRoot, abs)`), so no new traversal surface is
  introduced. Only the exact `SpecDoc` file is allowed (`isUnder` returns true when `prefix == abs`),
  not its directory.

## Open questions

- Is `specStore: convention` already released/documented, or is this a pre-release bug? (Affects
  whether release notes are needed.) — non-blocking.
- Should the inline comment in `verifyArtifactPath` explicitly state that `SpecDoc` is safe to allow
  because it is persisted state? (Planned: yes, update the comment.) — non-blocking.

## Impact

- `cli/internal/state` only: `verifyArtifactPath` allowlist + comments (`artifact.go`) and a new test
  case (`artifact_test.go`). No change to `web/`, the HTTP layer, the state schema, or the board
  projection. No new dependency. The board stays read-only.
