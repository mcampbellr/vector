# Design — fix-convention-store-file-preview-500

## Context

`ReadSpecArtifact` (`cli/internal/state/artifact.go`) is the only read seam the board's
`GET /api/file` handler uses to serve a spec's source documents. For the `spec` key it resolves to
the spec's `SpecDoc`; for `proposal`/`design`/`tasks` it resolves to
`openspec/changes/<change>/<key>.md` gated by the `Artifacts` flags. After resolving a repo-relative
path it calls `verifyArtifactPath(repoRoot, abs, spec)` as defense in depth before `os.ReadFile`.

The client never sends a path — callers pass a spec id and an artifact enum — so traversal is
removed by design; the prefix check is belt-and-suspenders over already-trusted committed state.

`CreateSpec` (`cli/internal/state/store.go:113-129`) already supports a caller-supplied `SpecDoc`
location: `SpecDocAbsPath`/`SpecDocRel` (repo convention or an OpenSpec change), falling back to
`.vector/specs/<id>/spec.md` only when neither is given. So a spec's `SpecDoc` can legitimately point
outside `.vector/` — but `verifyArtifactPath`'s allowlist was never updated to match.

## Root cause

`verifyArtifactPath` builds `allowed` from `.vector/specs/<id>/` and (when `OpenSpec != nil`)
`openspec/changes/<change>/`. A convention `SpecDoc` (e.g. `code/main/docs/specs/<slug>/spec.md`) is
under neither, so the function returns `"artifact path … outside allowed locations"`. That error is
**not** wrapped `fs.ErrNotExist`, so `handleFile` maps it to **500** rather than 404 — a hard failure
for a file that exists and is readable.

## Decision

Add the spec's own `SpecDoc` path to the allowlist when present:

```go
allowed := []string{filepath.Join(repoRoot, ".vector", "specs", spec.ID)}
if spec.SpecDoc != "" {
    allowed = append(allowed, filepath.Join(repoRoot, filepath.FromSlash(spec.SpecDoc)))
}
if spec.OpenSpec != nil {
    allowed = append(allowed, filepath.Join(repoRoot, "openspec", "changes", spec.OpenSpec.Change))
}
```

`isUnder(prefix, abs)` returns true when `prefix == abs` (its `rel == "."` branch), so adding the
exact `SpecDoc` file path permits that one file and nothing else under its directory.

### Why this is safe

- The `repoRoot`-escape guard (`isUnder(repoRoot, abs)`) runs **before** the allowlist loop and is
  unchanged, so a crafted path that escapes the repo is still rejected as a non-`fs.ErrNotExist`
  error → 500. `TestReadSpecArtifactRejectsTraversal` continues to pass.
- `SpecDoc` is persisted, committed state written by `CreateSpec`, not a client-supplied value. The
  allowlist is defense in depth, and the source it now trusts is the same one that produced the
  resolved `rel` path — closing a contradiction, not opening a hole.
- The exact-file match (not the directory) means no sibling-file access is granted in the convention
  directory.

## Goals / Non-Goals

**Goals:**
- Convention-store `spec.md` (SpecDoc outside `.vector/`) reads succeed via `/api/file`.
- Preserve the traversal/root-escape defense and the 404-vs-500 error semantics.
- Add regression coverage for the convention store.

**Non-Goals:**
- Any change to `web/`, `handleFile`'s error mapping, the state schema, or `CreateSpec`.
- Granting directory-wide access in the convention path; rendering raw HTML; new dependencies.

## Test strategy

`cli/internal/state/artifact_test.go`:

- **New:** `TestReadSpecArtifactConventionStore` — `CreateSpec` with `SpecDocRel`/`SpecDocAbsPath`
  pointing to `code/main/docs/specs/alpha/spec.md` (outside `.vector/`) and a `Body`; assert
  `ReadSpecArtifact("alpha", "spec")` returns the body.
- **Unchanged/green:** `TestReadSpecArtifactRejectsTraversal` (root-escape still rejected),
  `TestReadSpecArtifactSpecDoc`, `TestReadSpecArtifactOpenSpecGatedByFlags`,
  `TestReadSpecArtifactUnknownSpecAndKey`.

Verify: `go test ./cli/internal/state/... ./cli/internal/board/...`.
