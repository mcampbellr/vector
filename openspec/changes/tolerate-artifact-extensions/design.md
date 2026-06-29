# Design — tolerate-artifact-extensions

## Key decisions

- **Early normalization in the parser**: each parser normalizes its input before comparing. The
  normalization is deterministic and only changes representation, not semantics; the persisted
  state always holds canonical names.
- **LOCKED normalization sequence**: per segment — (1) `TrimSpace`, (2) strip a single `.md`
  suffix if present (case-insensitive, via `strings.EqualFold` over the last 3 chars), (3) `ToLower`,
  (4) switch against `proposal | design | tasks`.
- **Single level of `.md` strip**: only one trailing `.md` is removed. `proposal.md.md` strips to
  `proposal.md`, which is not canonical → error. This is intentional.
- **Empty post-normalization segment is invalid**: `.md` alone strips to `""`, which is not a
  canonical name → error. (Distinct from the empty-segment tolerance in a CSV list, e.g.
  `proposal,,tasks`.)
- **Error message keeps the raw value**: `invalid --artifacts %q: allowed proposal,design,tasks`
  uses the original segment (`part` / `v`) for user context. The message text itself is unchanged.
- **`parseFixArtifacts` returns canonical names**: the slice is always lowercase, no `.md`, so what
  is persisted in state never depends on the input format. `parseArtifacts("")` →
  `ArtifactSet{}`; `parseFixArtifacts("")` → `(nil, nil)`, both preserved.
- **Reuse `splitCSV`**: `parseFixArtifacts` keeps delegating trimming/empty-discard to `splitCSV`
  (`cli/cmd/vector/standup.go`); the new logic does not reimplement trimming.
- **stdlib only**: no external dependencies; `strings` and `fmt` are already imported.

## Surface

- `cli/cmd/vector/main.go` — `parseArtifacts` loop body normalizes each segment before the switch.
- `cli/cmd/vector/spec_transitions.go` — `parseFixArtifacts` normalizes each value and accumulates
  canonical names into the returned slice.
- `cli/cmd/vector/main_test.go` (new) — `TestParseArtifacts`, `package main`.
- `cli/cmd/vector/spec_transitions_test.go` (new) — `TestParseFixArtifacts`, `package main`.
- `kit/commands/vector/propose.md`, `kit/commands/vector/fix.md` — tolerance note.
- `cli/internal/scaffold/assets/commands/vector/{propose,fix}.md` — regenerated copies.

## Constraints

- No signature changes to `parseArtifacts` / `parseFixArtifacts`.
- No changes to `splitCSV`, `state.ArtifactSet`, the state machine, or the `draft → open` transition.
- Assets under `cli/internal/scaffold/assets/` are never hand-edited; regenerate via `go generate`.
