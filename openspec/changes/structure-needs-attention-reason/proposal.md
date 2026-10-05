# Structure the needs-attention reason (category, summary, markdown detail)

## Why

Today `Attention.Reason` (`cli/internal/state/types.go:212-217`) is a single free-text string.
The only producers (`/vector:apply` §6b, `/vector:fix` §5) dump everything into it — from a short
phrase to a technical paragraph with PR refs and TODOs — and `web` paints it verbatim as a red
`<p>` in both `SpecCard.tsx:52` and `SpecDetailsDrawer/index.tsx:81`. The little structure that
does exist (`Since`, `Source`) is lost in the board projection (`board.go:251-252` copies only
`Reason`). A dev scanning the board cannot tell **at a glance why** a spec is blocked, nor read a
formatted technical detail — just a monolithic red block, sometimes a long technical dump, in both
the card and the drawer.

## What changes

- Three new optional fields on the Go `Attention` struct: `Category`, `Summary`, `Detail`
  (all `omitempty`, additive — **no `SchemaVersion` bump**, specs on disk are untouched).
- New flag contract on `vector spec status <id> needs-attention`: `--summary` (required on the
  structured path), `--category` (optional, validated enum `dependency|env|decision|external|other`,
  default `other`), `--detail` / `--detail-file` (optional markdown). `--reason` stays as a
  **legacy path**, mutually exclusive with the new flags, auto-migrated on write.
- `Store.SetStatus` keeps its 5-arg signature **unchanged**; the structured path lives in a new
  `Store.SetStatusAttention(id, to, att, actor, now)`. Both delegate to the same `applyTransition`.
  In the structured path `Reason` is fixed equal to `Summary`, so any legacy reader still gets the
  one-liner and `board.go` maps `AttentionReason = spec.Flag.Reason` with no branching.
- Board projection (`board.Card` + its TS mirror `web/src/types/board.ts`) gains
  `attentionCategory`/`attentionSummary`/`attentionDetail` (all `omitempty`); `attentionReason`
  stays for legacy specs.
- `web`: a new `AttentionCategoryChip.tsx` (one component per file) + `SpecCard` shows a category
  chip + truncated summary (fallback to `attentionReason`); `SpecDetailsDrawer` shows the chip +
  full summary as a header and renders `attentionDetail` as markdown via the existing lazy
  `MarkdownView.tsx` (no new deps, no raw HTML). New `--status-cat-*` tokens in `tokens.css`.
- Kit docs (`apply.md`, `fix.md`, `status.md`) updated to the structured contract and propagated
  to `cli/internal/scaffold/assets/` via `go generate` (`TestAssetsMatchKit` stays green).

## Scope

- **In**: the three `Attention` fields, the new `SetStatusAttention` method + `applyTransition`
  extension, the `runSpecStatus` flags/validation/routing, the board projection (Go + TS), the
  `AttentionCategoryChip` component + card/drawer render + tokens, the three kit doc updates +
  regenerated assets, and tests (structured/legacy transition, enum + mutual-exclusion validation,
  board projection, card + drawer render, `TestAssetsMatchKit`).
- **Out**: `SchemaVersion` bump or bulk migration of persisted specs (migration is on-read/on-write
  of the legacy path only); board filtering/search by category; wiring producers beyond
  `/vector:apply` and `/vector:fix`; extending the category enum; raw-HTML rendering in the detail;
  any new markdown/sanitization library; a `vector config` flag for the enum (fixed in code for V1).

Authored spec: `.vector/specs/structure-needs-attention-reason/spec.md`.
