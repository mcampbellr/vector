# Design — add-token-breakdown-tab

## Context

The CLI already rolls up `agent.routed` events into the board projection: `rollupSavings`
(`cli/internal/board/board.go:256-303`) aggregates a **global** `TokenSavings` and a per-spec
`savedUSD`/`routes` map keyed by spec id, grouped by `model→baseline`; `toCard` (`:203-238`) writes
the per-spec economics onto each `Card`. Today a `Card` carries only `SavedUSD`/`Routes`, and the
only token counts on the wire are the **global** `TokenSavings.TokensIn/TokensOut`. The web therefore
**cannot** derive per-spec tokens from the current `Board` — the per-spec view must be produced in Go.

The web board reads the projection over a single SSE source (`useBoard()`,
`web/src/api/useBoard.ts:16-45`) and renders top-level tabs (Board, Standup) via a `view` state in
`App.tsx`. `StandupView` is the existing pure-projection view to mirror for layout/empty-state markup.

## Goals / Non-Goals

**Goals**
- Per-spec routed-token volume + per-spec model breakdown on each `Card`, produced by the CLI.
- A token-only Tokens tab: aggregate "tokens saved" headline, global per-model breakdown, one row per
  routed spec with its own model breakdown, sorted tokens-desc.
- Remove dollars from the board UI (rail meter gone, per-card badge reframed to tokens).

**Non-Goals**
- No dollar/USD display in the tab; no keeping a dollar meter on the board.
- No removal of the CLI dollar economics (`agent.routed` `CostUSD`/`SavedUSD`, `vector spec route`,
  pricing, global `TokenSavings` USD fields stay; only the web stops rendering them).
- No new endpoint, second SSE, or client-side fetch; no write surface.
- No pagination, interactive sort, search/filter, CSV export; no cost-weighted "baseline-equivalent"
  metric (Open question). No change to standup, activity schema, state machine, or other card metadata.

## Decisions

- **"tokens saved" ≔ tokens routed to a cheaper-than-baseline model** = `tokensIn + tokensOut` of each
  `agent.routed` event. Token counts are model-invariant, so there is no literal dollar-style "saved"
  quantity; this is the honest, model-invariant reading. Aggregated = global routed-token volume; per
  spec = that spec's routed-token volume; per model pair = that pair's routed-token volume. A
  cost-weighted variant is recorded as an Open question, not built.
- **Extend the projection, don't add a parallel pass.** The per-spec token totals and the per-spec
  `model→baseline` token rollup are accumulated inside the existing `rollupSavings` loops, mirroring
  the per-spec `savedUSD`/`routes` accumulation already there. `toCard` writes
  `TokensIn`/`TokensOut`/`ByModel` next to the existing `card.SavedUSD`/`card.Routes`.
- **Fix the global `byModel` token accumulation.** The existing global loop (`board.go:284-291`) only
  does `m.Routes++` / `m.SavedUSD += d.SavedUSD`. Once `ModelRollup` carries token fields, that loop
  must also `m.TokensIn += d.TokensIn` / `m.TokensOut += d.TokensOut`, or the aggregate per-model
  breakdown renders zeros.
- **Additive wire change + `SchemaVersion` bump.** `Card` gains `tokensIn`/`tokensOut`/`byModel?`;
  `ModelRollup` gains `tokensIn`/`tokensOut`. The global `TokenSavings` struct fields are unchanged.
  The web mirror (`web/src/types/board.ts`) is updated in lockstep; missing fields are treated as
  `0`/empty (optional chaining) so an older board doesn't break a newer web.
- **Pure-projection tab.** `TokenBreakdownView` takes `{ board: Board }` (non-nullable — `App.tsx`
  guards `if (!board)` before mounting any tab). It computes the headline from `board.tokenSavings`
  and the rows from `board.columns.flatMap(c => c.cards).filter(card => card.routes > 0)` sorted by
  `tokensIn+tokensOut` desc. Only tab-local state is **idle** vs **empty** (`routes === 0`).
  Loading/error/reconnect are App-level; the tab renders no spinner or retry. Unlike `StandupView`
  (which owns its own `useStandup()` fetch), the tab reuses the `board` App already holds — it does
  not fetch.
- **Reuse `formatCompact`** (`web/src/lib/format.ts:11-13`) for all token figures; do not hand-roll
  number formatting. `formatCompact(0)` → `"0"` (confirmed), correct for empty token cells.
- **One component per file**, semantic names (`TokenBreakdownView`, `TokenBreakdownRow`,
  `SpecModelBreakdown`), strong typing from the API contract (no `any`).

## Risks / Trade-offs

- **Go ↔ web mirror drift.** Mitigated by updating both in this change and guarding optional fields
  with optional chaining / `0` defaults.
- **Zeroed aggregate breakdown** if the `:284-291` global-loop token fix is missed — called out as an
  explicit success criterion and asserted indirectly by the extended test.
- **`formatUsd` removal** could leave a dangling caller. Mitigated by `grep -r "formatUsd" web/src/`
  before deleting; if another caller surfaces, stop and report rather than leave it broken.
- **Scale.** The row build is one `flatMap` + `filter` + `sort` over in-memory cards (hundreds of
  specs) — fine for now; virtualization is an Open question if a repo accrues many routed specs.

## Migration / Rollout

- Additive only; no data migration. Specs with no routes default to `0`/empty (no nil-map panics).
- After the web change, re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild + reinstall the
  `vector` binary; dogfooding uses the PATH binary.
- Gate: `go -C cli vet ./...`, `go -C cli test ./...`, `npm --prefix web run typecheck`,
  `npm --prefix web run build`.
