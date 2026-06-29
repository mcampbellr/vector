# Tolerate .md extension and casing in --artifacts parsing (propose/fix)

## Why

The two `--artifacts` parsers in the CLI — `parseArtifacts` (`cli/cmd/vector/main.go`) and
`parseFixArtifacts` (`cli/cmd/vector/spec_transitions.go`) — are case-sensitive and reject any
`.md` extension. A user who writes `--artifacts proposal.md,Design.MD` gets an error even though
the intent is unambiguous. The artifact names are `proposal` / `design` / `tasks` and the input
should be tolerant of casing and the optional `.md` suffix.

## What changes

- **`parseArtifacts`** normalizes each CSV segment before comparing: `TrimSpace` → strip a single
  `.md` suffix (case-insensitive) → `ToLower` → switch against `proposal | design | tasks`.
  `parseArtifacts("")` still returns `ArtifactSet{}` with no error.
- **`parseFixArtifacts`** applies the same normalization and now returns the **canonical** names
  (lowercase, no `.md`) instead of the raw input values, so the persisted state is always
  canonical. `splitCSV` is reused for trimming — not reimplemented.
- The error message is unchanged: `invalid --artifacts %q: allowed proposal,design,tasks`, with
  the original (un-normalized) segment for user context.
- **Table-driven tests** for both parsers: `cli/cmd/vector/main_test.go` (new, `TestParseArtifacts`)
  and `cli/cmd/vector/spec_transitions_test.go` (new, `TestParseFixArtifacts`), `package main`.
- **Project command docs**: `kit/commands/vector/propose.md` (step 6) and `kit/commands/vector/fix.md`
  (§6) get a short note that `--artifacts` takes the canonical names and tolerates `.md` and any casing.
- **Re-vending**: regenerate the embedded copies under `cli/internal/scaffold/assets/commands/vector/`
  via `go generate ./internal/scaffold` from `cli/`; `TestAssetsMatchKit` guards against drift.

## Scope

### In scope

- Tolerant normalization in both parsers (stdlib only: `strings.ToLower` / `strings.EqualFold`).
- `parseFixArtifacts` returns canonical names.
- New table-driven tests for both parsers.
- Doc notes in `propose.md` / `fix.md` and regeneration of their embedded copies.

### Out of scope

- Formalizing the lightweight change convention (proposal/design/tasks without deltas) in `docs/`.
- Re-wording the "delegate to OpenSpec" framing in `propose.md`.
- Changing the error message to mention `.md`/casing tolerance.
- Any `docs/` edit, the state machine, the `draft → open` transition, or any `state.ArtifactSet`
  type change.
- Migrating existing changes to the OpenSpec delta model.
- Accepting more than one level of `.md` strip, or accepting an empty post-normalization segment.
