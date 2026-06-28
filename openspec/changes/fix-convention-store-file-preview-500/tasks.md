# Tasks — fix-convention-store-file-preview-500

## 1. State — extend the allowlist

- [x] 1.1 `cli/internal/state/artifact.go`: in `verifyArtifactPath`, after the `.vector/specs/<id>/`
      entry and when `spec.SpecDoc != ""`, append `filepath.Join(repoRoot, filepath.FromSlash(spec.SpecDoc))`
      to `allowed` (keep the `OpenSpec` entry). Leave the `isUnder(repoRoot, abs)` root-escape check first.
- [x] 1.2 `cli/internal/state/artifact.go`: update the `verifyArtifactPath` doc comment and the
      file-level comment to record that the spec's own `SpecDoc` location is an allowed source
      (already-trusted committed state), so the convention store is covered.

## 2. State — close the test gap

- [x] 2.1 `cli/internal/state/artifact_test.go`: add `TestReadSpecArtifactConventionStore` — create a
      spec via `CreateSpec` with a convention `SpecDoc` outside `.vector/`
      (e.g. `code/main/docs/specs/alpha/spec.md`, set `SpecDocRel`/`SpecDocAbsPath` + `Body`); assert
      `ReadSpecArtifact("alpha", "spec")` returns the authored body.
- [x] 2.2 Confirm `TestReadSpecArtifactRejectsTraversal` stays green (root-escape traversal still a
      non-`fs.ErrNotExist`, 500-class error).

## 3. Verification

- [x] 3.1 `go -C cli vet ./...` green.
- [x] 3.2 `go test ./cli/internal/state/... ./cli/internal/board/...` green.
- [x] 3.3 Rebuild + reinstall the `vector` binary to `~/.local/bin/vector` (dogfooding); confirm the
      preview opens for a convention-store spec.
