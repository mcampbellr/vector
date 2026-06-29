# Tasks — tolerate-artifact-extensions

## 1. Parsers

- [x] 1.1 `parseArtifacts` (`cli/cmd/vector/main.go`): normalize each segment — trim → strip a
      single `.md` suffix (case-insensitive) → `ToLower` → switch `proposal | design | tasks`.
      Error uses the raw segment; `parseArtifacts("")` still returns `ArtifactSet{}` with no error.
- [x] 1.2 `parseFixArtifacts` (`cli/cmd/vector/spec_transitions.go`): same normalization, reusing
      `splitCSV`; return the **canonical** names (lowercase, no `.md`), not the raw input.

## 2. Tests

- [x] 2.1 `cli/cmd/vector/main_test.go` (new): `TestParseArtifacts` table-driven — bare, `.md`,
      casing, trim, mixed list, empty string, empty segment tolerated, `.md` alone, `proposal.md.md`,
      `readme` (all invalid cases → error).
- [x] 2.2 `cli/cmd/vector/spec_transitions_test.go` (new): `TestParseFixArtifacts` table-driven —
      same cases; explicitly verify the returned slice holds canonical names (e.g. `Proposal.md` →
      `["proposal"]`, not `["Proposal.md"]`).
- [x] 2.3 Existing tests unaffected: `TestRunSpecFixValidation` (`spec_fix_test.go`, case `valid`
      with `"design,tasks"`) still passes.

## 3. Command docs + re-vending

- [x] 3.1 `kit/commands/vector/propose.md` (step 6): short note that `--artifacts` takes the canonical
      names and tolerates `.md` and any casing.
- [x] 3.2 `kit/commands/vector/fix.md` (§6): analogous note.
- [x] 3.3 `go generate ./internal/scaffold` from `cli/` to regenerate
      `cli/internal/scaffold/assets/commands/vector/{propose,fix}.md` (never hand-edited).

## 4. Verification

- [x] 4.1 Gate green from repo root: `go -C cli generate ./internal/scaffold`, `gofmt -l cli` (empty),
      `go -C cli vet ./...`, `go -C cli test ./...`, `go -C cli build ./...`.
- [x] 4.2 `TestAssetsMatchKit` passes (no drift between `kit/` and `assets/`).
