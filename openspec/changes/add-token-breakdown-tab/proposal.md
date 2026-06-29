# Token breakdown tab: per-spec token usage

## Why

Vector routes trivial work to cheaper-than-baseline models, but the board only surfaces this as a
global dollar meter on the rail plus a per-card `$X saved` badge. A developer can't see, in one
consolidated view, **how many tokens each spec moved off the expensive baseline** or **which cheap
models handled it** — they'd have to sum per-card numbers and think in dollars. Token counts are
also the honest denominator here: they're model-invariant, so a per-spec token breakdown reads the
same regardless of pricing, while the dollar figure depends on a pricing table that may be missing a
model. This change makes the savings story token-native and per-spec, and removes dollars from the
board UI entirely.

## What changes

- **Per-spec token projection (cli)** — extend the board projection so each `Card` carries its own
  `TokensIn`/`TokensOut` and a per-spec `ByModel []ModelRollup` breakdown, derived from that spec's
  `agent.routed` events. `ModelRollup` gains `TokensIn`/`TokensOut`. `rollupSavings` accumulates the
  per-spec token totals + a per-spec `model→baseline` token rollup in the same single pass it already
  makes; `toCard` writes them onto the card. The **global** `byModel` loop is fixed to also sum
  `TokensIn`/`TokensOut` (today it only bumps `Routes`/`SavedUSD`, so the aggregate per-model
  breakdown would render zeros once the token fields exist). `Board.SchemaVersion` bumps (additive
  wire growth). The global `TokenSavings` USD fields and the event/pricing schema are untouched.
- **Tokens tab (web)** — a new top-level "Tokens" tab next to Board and Standup, following the
  existing `view` state + tab-button pattern. New `web/src/components/TokenBreakdownView/` renders a
  **pure projection** of the already-loaded `board` (same `useBoard()` SSE — no new endpoint, no
  second fetch): an aggregate **tokens saved** headline (`tokensIn+tokensOut`), route count, and the
  global per-model breakdown; one row per spec with `routes > 0` (id, title, in, out, routes, and an
  expandable per-spec model breakdown), sorted by `tokensIn+tokensOut` desc; tab-local **idle** and
  **empty** (`routes === 0`) states. Loading/error stay App-level — the tab never mounts without a
  board. **No `$` anywhere.**
- **Remove dollars from the board (web)** — delete the aggregate `TokenSavingsMeter` from the board
  rail and the component itself; reframe the per-card badge from `formatUsd(card.savedUsd)` + "saved"
  to `formatCompact(card.tokensIn + card.tokensOut)` + "tok" (keeping the `Sparkles` icon and the
  `routes`-gated visibility); remove the now-unused `formatUsd` from `web/src/lib/format.ts`.
- **Tests** — extend `TestBuildRollsUpTokenSavings` to assert the new per-spec `TokensIn`/`TokensOut`/
  `ByModel`; web typecheck + build are the web gate.

## Capabilities

### Added Capabilities
- `board-tokens`: the board web UI gains a top-level Tokens tab projecting per-spec routed-token
  volume — an aggregate "tokens saved" headline (`tokensIn+tokensOut`), route count, a global
  per-model breakdown, and one row per routed spec with its own per-model breakdown, sorted
  tokens-desc — denominated only in tokens, fed by the existing `useBoard()` SSE.

### Modified Capabilities
- `board-projection`: each `Card` (and `ModelRollup`) gains per-spec token totals; the global
  per-model rollup now carries token counts; `SchemaVersion` bumps.
- `board-card`: the per-card savings badge is reframed from dollars to tokens.
- `board-rail`: the aggregate dollar `TokenSavingsMeter` is removed.

## Impact

- New: `web/src/components/TokenBreakdownView/` (`index.tsx`, `TokenBreakdownView.module.css`, and
  `TokenBreakdownRow.tsx` / `SpecModelBreakdown.tsx` if the row markup grows).
- Modified: `cli/internal/board/board.go` (`Card`/`ModelRollup` token fields, `rollupSavings`/`toCard`,
  global `byModel` token fix, `SchemaVersion`), `cli/internal/board/board_test.go`,
  `web/src/types/board.ts` (mirror), `web/src/App.tsx` (tab + meter removal),
  `web/src/components/SpecCard/SpecCard.tsx` (badge), `web/src/lib/format.ts` (drop `formatUsd`).
- Deleted: `web/src/components/TokenSavingsMeter/`.
- **No** new dependencies, **no** new HTTP endpoint or second SSE, **no** write/mutation surface, **no**
  change to the CLI dollar economics (`agent.routed`, `vector spec route`, pricing, global
  `TokenSavings` USD fields stay — only the web stops rendering dollars), **no** change to the activity
  schema or state machine.
- After the `web/` change, re-embed `web/dist` and rebuild + reinstall the `vector` binary (dogfooding
  uses the PATH binary).

Authored spec: `.vector/specs/add-token-breakdown-tab/spec.md`.
