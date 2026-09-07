# Design — structure-needs-attention-reason

## Pattern

**Structured schema with transparent migration on the legacy path** (LOCKED). The `Attention`
struct gains three optional fields. The writer (`applyTransition`) receives a structure (not a
loose string) and decides — from what arrived — whether to apply the structured or the legacy
path. The board projection flattens the four fields with no business logic (pure read of
`spec.Flag`).

## Key decisions (LOCKED — §10 of the spec)

- **Structured schema**, not a free-text parser nor a generic JSON blob: the producer (the CLI
  itself, via `/vector:apply`/`/vector:fix`) already knows the category and can emit a short
  summary separate from the detail — simpler and more reliable than inferring structure after the
  fact.
- **Three new optional fields** (`Category`, `Summary`, `Detail`) instead of replacing `Reason`:
  preserves binary compatibility with any external `state.json` reader that depends on `reason`,
  and avoids a `SchemaVersion` bump. Same additive `omitempty` pattern as `Source` and as
  `add-agent-prose-language`'s `Config.Language`.
- **`Store.SetStatus` keeps its 5-arg signature** (`id, to, reason, actor, now`): the structured
  path lives in a **new** `Store.SetStatusAttention(id, to, att, actor, now)`, so no existing
  caller breaks (`standup_test.go:202,288`, `transition_test.go`). Both delegate to the same
  `applyTransition`, extended internally to accept the structured fields.
- **On the structured path, `Attention.Reason = Summary`** (never empty, never `Detail`): any
  legacy reader of `reason` still gets a readable one-liner, and `board.go` keeps mapping
  `AttentionReason = spec.Flag.Reason` with no branching.
- **Transparent on-write migration of the legacy path**: a producer that only passes `--reason`
  keeps working; the binary derives `Category="other"`, `Summary` truncated to ~80 chars,
  `Detail=reason`. No bulk-migration command, no touching persisted specs.
- **`--summary` required on the structured path** (the only field the card must render legibly);
  `--category` defaults to `"other"`; `--detail` optional, falls back to `--summary`.
- **`--reason` mutually exclusive with the new flags**: mixing paths is a CLI usage error, rejected
  with a clear message — not resolved silently.
- **Category enum fixed for V1**: `dependency | env | decision | external | other`.

## Layers affected

- **domain/state** (`cli/internal/state`): `Attention` +3 fields; `applyTransition` gains
  structured-vs-legacy construction; new `SetStatusAttention`.
- **application/CLI** (`cli/cmd/vector`): `runSpecStatus` gains `--summary`/`--category`/`--detail`/
  `--detail-file`, enum + mutual-exclusion validation, and routes to the right store method.
- **domain/board** (`cli/internal/board`): `Card` +3 fields; `toCard` maps them (no branching).
- **presentation** (`web`): `SpecCard`, `SpecDetailsDrawer`, new `AttentionCategoryChip`, styles,
  tokens; `MarkdownView` reused via `React.lazy` (single import point of `react-markdown`).
- **kit** (`kit/commands/vector`): `apply.md`, `fix.md`, `status.md` document the structured
  contract; regenerated into `cli/internal/scaffold/assets/` via `go generate`.

## Flow

1. `/vector:apply` (§6b) or `/vector:fix` (§5) runs the structured
   `vector spec status <id> needs-attention --category <cat> --summary "<one-liner>" [--detail <md> | --detail-file <path>]`,
   or the legacy `--reason "<text>"`.
2. `runSpecStatus` parses, validates the enum + mutual exclusion, and routes: structured →
   `SetStatusAttention`; `--reason` only → `SetStatus` (unchanged 5-arg signature).
3. Both delegate to `applyTransition`. Structured path persists `spec.Flag = &Attention{Category
   (or "other"), Summary, Detail (or Summary), Reason=Summary, Since=now, Source}`. Legacy path
   migrates: `Category="other"`, `Summary=reason` truncated ~80, `Detail=reason`, `Reason=reason`.
4. `applyTransition` writes the spec file and appends `status.changed` as today (no new event type;
   `StatusChangedData.Reason` carries `Summary` on the structured path, `reason` on the legacy).
5. `GET /api/board` projects `spec.Flag` to the four `Card` fields; a legacy spec projects only
   `attentionReason`.
6. `web` consumes via SSE: `SpecCard` paints chip + truncated summary (fallback to truncated
   `attentionReason`); `SpecDetailsDrawer` paints chip + full summary header + `attentionDetail`
   as markdown (fallback to plain-text `attentionReason`).

## Non-goals / constraints

- No `SchemaVersion` bump (state stays 1, board stays 2 — additive `omitempty`).
- No change to `SetStatus`'s signature, to `Flag` cleanup on exit, or to `ApplySpec`/`CloseSpec`/
  `ArchiveSpec`. No bulk migration, no new markdown/sanitization dependency, no raw HTML.
