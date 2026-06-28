# Spec: Token breakdown tab — per-spec token usage, no USD

## 1. Goal

Build a dedicated **"Tokens" tab** in the Vector web board that projects, **per spec**, how many
tokens were routed to cheaper-than-baseline models — with a **per-model breakdown** per spec — and
shows a single aggregate **"tokens saved"** headline. The view is denominated **only in tokens; no
dollar amounts appear anywhere in it**.

This feature lets a **developer** **see, in one consolidated view, the token volume each spec moved
off the expensive baseline (and which cheap models handled it)** so they can **understand where the
cheap-routing happened without summing per-card numbers or thinking in dollars**.

As part of the same card, the **dollar-denominated aggregate meter is removed from the board** and
the **per-card "$X saved" badge is reframed in tokens** — the savings story becomes token-native.

### What "tokens saved" means here (read before implementing)

Token counts are essentially **model-invariant**: the same task spends roughly the same tokens on
Haiku or on Opus. There is therefore no literal "tokens saved" quantity the way there is a *dollar*
saving (the dollar saving exists because a cheaper model bills less per token). To keep the metric
**honest**, this spec defines:

> **tokens saved** ≔ the total tokens (`tokensIn + tokensOut`) that ran on a **cheaper-than-baseline
> model** — i.e. every token a routed cheap agent processed that would otherwise have hit the
> expensive baseline.

Per `agent.routed` event this equals `tokensIn + tokensOut`. Aggregated it is the global routed-token
volume; per spec it is that spec's routed-token volume; per model pair it is that pair's routed-token
volume. This is **the same number** as "tokens spent on cheap routing" — the framing changes, the
quantity does not. The alternative (a cost-weighted "baseline-equivalent" figure) is recorded as an
Open question, not built.

## 2. Scope

### Included in this phase

**A. Per-spec token projection (cli)**

- Extend the board projection so each `Card` carries its **per-spec token totals** and a **per-spec
  model breakdown**, derived from that spec's `agent.routed` events. Today a `Card` only carries
  `SavedUSD` and `Routes` (`cli/internal/board/board.go:40-60`); the web cannot derive per-spec
  tokens from the current `Board` (the only token counts are the **global** `TokenSavings.TokensIn/
  TokensOut`). So the projection **must** be extended in Go — it cannot be a pure client derivation.
- `Card` gains `TokensIn int`, `TokensOut int`, and `ByModel []ModelRollup` (the spec's own model
  breakdown). `ModelRollup` gains `TokensIn int` and `TokensOut int` (it currently carries
  `Model`/`Baseline`/`Routes`/`SavedUSD`, `board.go:96-102`).
- `rollupSavings` (`cli/internal/board/board.go:256-303`) accumulates per-spec `tokensIn/tokensOut`
  and a per-spec `model→baseline` token rollup alongside the existing per-spec `savedUSD`/`routes`;
  `toCard` (`board.go:203-238`) writes them onto the card. The **global** `TokenSavings` **struct
  fields** are unchanged (no new top-level fields — it already carries `TokensIn`/`TokensOut`/
  `ByModel`), **but** the per-model token counts inside each global `ByModel` entry must now be
  populated: the existing global `byModel` loop (`:284-291`) sets only `m.Routes++`/`m.SavedUSD +=
  d.SavedUSD`, so once `ModelRollup` gains `TokensIn`/`TokensOut` those entries would carry `0` and
  the tab's aggregate per-model breakdown would silently render zeros. The loop must also
  `m.TokensIn += d.TokensIn` / `m.TokensOut += d.TokensOut`.
- Bump `Board.SchemaVersion` (`board.go:16`) since the wire shape grows (additive fields).

**B. Tokens tab (web)**

- A new **top-level tab** "Tokens" in `App.tsx` next to **Board** and **Standup**, following the
  existing `view` state + tab-button pattern (`web/src/App.tsx:33-48`).
- New component `web/src/components/TokenBreakdownView/` rendering a **pure projection** of the
  already-loaded `board` (same `useBoard()` SSE source — **no new endpoint, no second fetch**):
  - An **aggregate headline**: total **tokens saved** = `board.tokenSavings.tokensIn +
    tokensOut`, route count, and the global per-model breakdown — **tokens only, no `$`**.
  - **One row per spec** that has `routes > 0`, collected from `board.columns[].cards[]`. Each row
    shows: spec id, title, **tokens-in**, **tokens-out**, **routes**, and an expandable/secondary
    **per-model breakdown** (`haiku→opus  12.0k  1×`, `sonnet→opus  3.4k  1×`).
  - Default sort: **tokens saved (in+out) descending**.
  - States (tab-local): **idle** (rows) and **empty** (`board.tokenSavings.routes === 0`). Board-level
    loading/error are handled by `App.tsx`; the tab is never mounted without a board (`StandupView` is
    the markup pattern, not the data contract).

**C. Remove dollars from the board (web)**

- **Remove** the aggregate dollar `TokenSavingsMeter` from the board rail (`App.tsx:52`) and delete
  the now-unused component (`web/src/components/TokenSavingsMeter/`). The savings story now lives
  **only** in the Tokens tab, token-denominated.
- **Reframe** the per-card badge in `SpecCard.tsx:76-81`: instead of `formatUsd(card.savedUsd)`
  + "saved", show the spec's routed tokens, e.g. `formatCompact(card.tokensIn + card.tokensOut)` +
  "tok" (the `Sparkles` icon and `routes`-gated visibility stay).
- `formatUsd` (`web/src/lib/format.ts:4-8`) loses its last caller; remove it (and its unused import
  in `SpecCard.tsx`) so lint/build stay clean. `formatCompact` (`format.ts:11-13`) is reused.

**D. Tests**

- Extend `TestBuildRollsUpTokenSavings` (`cli/internal/board/board_test.go:94-136`) to assert the new
  per-spec `TokensIn`/`TokensOut`/`ByModel`. Web typecheck + build are the web gate.

### Out of scope

- **Any dollar/USD display in the new tab.** Explicit user exclusion — tokens only.
- **Keeping a dollar meter on the board.** The user decided the aggregate meter is **removed** from
  the board and the savings live only in the Tokens tab (in tokens). Not a "move", a removal + new tab.
- **Removing the dollar economics from the CLI/state.** `agent.routed` keeps `CostUSD`/`SavedUSD`
  (`event.go:97-107`); `vector spec route` keeps reporting them (`cmd/vector/route.go`); the global
  `TokenSavings.TotalSavedUSD`/`TotalSpentUSD`/`BaselineUSD` Go fields stay (only the **web** stops
  rendering them). Pricing (`cli/internal/state/pricing.go`) is untouched.
- **A new HTTP endpoint or a second SSE stream.** The tab consumes the existing `GET /api/board`
  via `useBoard()`.
- **Write/mutation from the tab.** Read-only projection, like every other web view.
- **Pagination, interactive column sorting, search/filter, CSV export, copy-to-clipboard of ids.**
  MVP is a single token-desc-sorted list. Listed as Open questions / future.
- **A cost-weighted "baseline-equivalent tokens" metric.** Out — see §1 and Open questions.
- **Changing the standup view, the activity log schema, the state machine, or any card metadata**
  other than the one badge reframed in C.

The agent must not implement anything outside the scope above, even if it seems related.

---

## 3. Project technologies and conventions

### Stack

- **cli/**: Go (single module, stdlib only). Projection lives in `cli/internal/board`.
- **web/**: TypeScript + React 19 + Vite. CSS Modules + CSS variables (`web/src/styles/tokens.css`);
  icons via `lucide-react`; no component library (light-bundle rule).
- State (web): SSE/`fetch` projection of `cli/`'s HTTP API via `useBoard()`; the frontend owns no
  canonical state.
- API client (web): hand-mirrored types in `web/src/types/board.ts` (no typegen yet).
- Testing: Go `testing` (table-driven); `web/` typecheck + build as the gate.

### Relevant versions

- Go: `TBD — ver Open questions` (confirm in `cli/go.mod`; not changed here).
- React: 19 (`web/package.json`; not changed here).

No new libraries, APIs, flags, or patterns beyond those already in the project. In particular: **no
table/grid library** — build the list with the existing CSS-Modules approach.

### Existing patterns to respect

- **Top-level tab pattern**: `App.tsx` holds `const [view, setView] = useState<View>(...)` and renders
  tab buttons + a conditional per view (`web/src/App.tsx:33-48`). The "Tokens" tab is a third value.
- **Pure-projection view**: `StandupView` takes its data prop from a `useXxx()` hook and renders
  loading/empty/error/idle states without owning canonical state. `TokenBreakdownView` mirrors it.
- **Single SSE source**: `useBoard()` (`web/src/api/useBoard.ts:16-45`) holds the live `board` over
  `/api/events`. The tab reuses the same `board` already in `App` — no extra connection.
- **One component per file/folder**, semantic names (`TokenBreakdownView`, `TokenBreakdownRow`,
  `SpecModelBreakdown`), strong typing from the API contract (no `any`). See
  `standards/typescript-react.md`.
- **Go projection**: `rollupSavings` already keys per-spec economics by spec id and groups by
  `model→baseline`; extend those same loops rather than adding a parallel pass (`board.go:256-303`).
- **Token formatting**: reuse `formatCompact` (`web/src/lib/format.ts:11-13`, "1.2k" style). Do not
  hand-roll number formatting.

---

## 4. Prerequisites

Before starting, the following must already exist (all verified present):

- [x] `agent.routed` event + payload with token counts and an optional spec id
      (`cli/internal/state/event.go:26,97-107` — `AgentRoutedData{Task,Model,Baseline,TokensIn,
      TokensOut,CostUSD,SavedUSD}`).
- [x] `rollupSavings` per-spec + per-model aggregation and `toCard` projection
      (`cli/internal/board/board.go:256-303`, `:203-238`).
- [x] `Board`/`Card`/`TokenSavings`/`ModelRollup` structs (`board.go:20-28,40-60,86-94,96-102`) and
      their web mirrors (`web/src/types/board.ts:39-60,80-85,87-95,101-109`).
- [x] `GET /api/board` + SSE `/api/events` and the `Source` interface (`cli/internal/board/server.go:
      31-44,141`); `useBoard()` (`web/src/api/useBoard.ts:16-45`).
- [x] Tab navigation + a pure-projection view to copy: `App.tsx` tabs (`:33-48`) and `StandupView`
      (`web/src/components/StandupView/index.tsx`, `StandupSpecRow.tsx`, `StandupView.module.css` —
      all confirmed present; `StandupView` takes **no props** and owns its own `useStandup()` fetch).
- [x] `formatCompact` for tokens (`web/src/lib/format.ts:11-13`); `TokenSavingsMeter` to remove
      (`web/src/components/TokenSavingsMeter/`); `SpecCard` badge to reframe
      (`web/src/components/SpecCard/SpecCard.tsx:76-81`).
- [x] Board projection tests as the mirror to extend (`cli/internal/board/board_test.go:45-55,94-136`;
      `server_test.go:16-42`).

If a prerequisite is missing, stop and report exactly what is absent. Do not invent contracts.

---

## 5. Architecture

### Pattern to use

**Extend-the-projection + pure-projection-view.** The CLI already rolls up `agent.routed` events into
the board; this change extends that rollup to carry **per-spec token totals and a per-spec model
breakdown** on each `Card`. The web Tokens tab is a **pure projection** of the already-loaded `board`
(no new endpoint, no second SSE, no client-side canonical state) — it groups the cards it already has
and renders them as a token-denominated table. The binary stays the sole producer of the numbers; the
web only renders.

### Affected layers

- **presentation (web)**: yes — new `TokenBreakdownView` (+ row/breakdown subcomponents); `App.tsx`
  gains the tab and **drops** the rail `TokenSavingsMeter`; `SpecCard` badge reframed; `TokenSavingsMeter`
  component + `formatUsd` deleted.
- **application/use-cases (cli)**: no new command; the projection builder is the only logic touched.
- **domain (cli state)**: no — `agent.routed`, the event schema, pricing, and the state machine are
  untouched.
- **data/infrastructure (cli projection)**: yes — `Card` and `ModelRollup` grow token fields;
  `rollupSavings`/`toCard` populate them; `SchemaVersion` bumps.
- **kit**: no.
- **shared/common**: no.

### Expected flow (data)

1. Cheap agents are routed during normal commands; each appends an `agent.routed` event with
   `tokensIn/tokensOut/model/baseline` (existing — `vector spec route`).
2. `Build` → `rollupSavings` reads all events, accumulates **global** `TokenSavings` (unchanged) **and**
   per-spec `tokensIn/tokensOut` + a per-spec `model→baseline` token rollup (new).
3. `toCard` writes the per-spec `TokensIn/TokensOut/ByModel` onto each `Card`.
4. `GET /api/board` (and the SSE board) carry the larger shape; `useBoard()` receives it.

### Expected flow (view)

1. The user clicks the **Tokens** tab; `App` sets `view='tokens'` and renders
   `<TokenBreakdownView board={board} />` with the same `board` it already holds.
2. The view computes the aggregate headline from `board.tokenSavings` and the rows from
   `board.columns[].cards[]` filtered to `routes > 0`, sorted by `tokensIn+tokensOut` desc.
3. Each row renders spec id, title, in/out, routes, and a per-model breakdown from `card.byModel`.
4. On any board change the SSE pushes a new `board`; the table re-renders live. No dollars anywhere.

### Location of new files

```txt
web/src/components/TokenBreakdownView/
  index.tsx                       # container: aggregate headline + rows + states
  TokenBreakdownView.module.css
  TokenBreakdownRow.tsx           # one spec row (+ its model breakdown), if it grows past a few lines
  SpecModelBreakdown.tsx          # the per-spec model rollup list, if extracted
```

No new Go packages; extend `internal/board`.

---

## 6. Files to create or modify

| Path | Action | Purpose | Project example to follow |
|---|---|---|---|
| `cli/internal/board/board.go` | MODIFY | Add `TokensIn`/`TokensOut`/`ByModel` to `Card`; add `TokensIn`/`TokensOut` to `ModelRollup`; accumulate per-spec tokens + per-spec model rollup in `rollupSavings`; write them in `toCard`; bump `SchemaVersion` | `board.go:256-303` (`rollupSavings`), `:203-238` (`toCard`) |
| `cli/internal/board/board_test.go` | MODIFY | Assert per-spec `TokensIn`/`TokensOut`/`ByModel` in `TestBuildRollsUpTokenSavings` | `board_test.go:94-136` |
| `web/src/types/board.ts` | MODIFY | Mirror the new fields: `Card.tokensIn`/`tokensOut`/`byModel`; `ModelRollup.tokensIn`/`tokensOut` | `web/src/types/board.ts:39-60,80-85` |
| `web/src/App.tsx` | MODIFY | `View` gains `'tokens'`; add a "Tokens" tab button; render `<TokenBreakdownView board={board} />`; **remove** the rail `<TokenSavingsMeter .../>` and its import | `web/src/App.tsx:33-48,52` |
| `web/src/components/TokenBreakdownView/index.tsx` | NUEVO | Aggregate headline (tokens saved) + per-spec rows + idle/empty (`routes===0`) states only (loading/error are App-level); sort tokens desc; no `$` | `web/src/components/StandupView/index.tsx`, `TokenSavingsMeter.tsx` (byModel list markup) |
| `web/src/components/TokenBreakdownView/TokenBreakdownView.module.css` | NUEVO | Table/list + headline styles | `StandupView.module.css`, `TokenSavingsMeter.module.css` |
| `web/src/components/TokenBreakdownView/TokenBreakdownRow.tsx` | NUEVO | One spec row + its model breakdown (extract if the row markup grows) | `web/src/components/StandupView/StandupSpecRow.tsx` |
| `web/src/components/SpecCard/SpecCard.tsx` | MODIFY | Badge: `formatUsd(card.savedUsd)` + "saved" → `formatCompact(card.tokensIn+card.tokensOut)` + "tok"; drop the `formatUsd` import | `web/src/components/SpecCard/SpecCard.tsx:76-81` |
| `web/src/lib/format.ts` | MODIFY | Remove `formatUsd` (last caller gone); keep `formatCompact` | `web/src/lib/format.ts:4-8,11-13` |
| `web/src/components/TokenSavingsMeter/` | DELETE | Component + CSS no longer used once removed from `App.tsx` | — |

### Detail per file

#### cli/internal/board/board.go

Action: MODIFY.

- `Card` (`:40-60`): add `TokensIn int \`json:"tokensIn"\``, `TokensOut int \`json:"tokensOut"\``,
  `ByModel []ModelRollup \`json:"byModel,omitempty"\`` (the spec's own model breakdown).
- `ModelRollup` (`:96-102`): add `TokensIn int \`json:"tokensIn"\``, `TokensOut int \`json:"tokensOut"\``.
  Keep `SavedUSD` (still produced for the global aggregate / `vector spec route` JSON; the web simply
  ignores it now).
- `rollupSavings` (`:256-303`): in the per-spec accumulation block (`:277-282`) also sum
  `tokensIn`/`tokensOut`, and build a per-spec `map[modelPair]*ModelRollup` accumulating tokens +
  routes (mirror the global `byModel` block at `:284-291`). Sort each spec's `ByModel` by
  `tokensIn+tokensOut` desc (mirror `:299-301`).
- **Global `byModel` token fix** (`:284-291`): the existing global loop only does `m.Routes++` and
  `m.SavedUSD += d.SavedUSD`. Since `ModelRollup` now has token fields, add `m.TokensIn += d.TokensIn`
  and `m.TokensOut += d.TokensOut` here too, or the tab's aggregate per-model breakdown
  (`board.tokenSavings.byModel`) renders zeros. This is the same accumulation as the per-spec rollup,
  on the global map.
- `toCard` (`:203-238`): set `card.TokensIn`/`card.TokensOut`/`card.ByModel` from the per-spec
  economics (next to the existing `card.SavedUSD = econ.savedUSD` / `card.Routes = econ.routes` at
  `:217-218`).
- `SchemaVersion` (`:16`): bump by one (additive change; web mirror updated in lockstep).

Restrictions: do not touch the event schema, pricing, or the global `TokenSavings` fields. Per-spec
fields default to `0`/empty for specs with no routes (no nil-map panics).

#### cli/internal/board/board_test.go

Action: MODIFY. In `TestBuildRollsUpTokenSavings` (`:94-136`), the two haiku→opus routes already target
one spec; assert that spec's `Card.TokensIn`/`TokensOut` equal the sum of the routed events and that
`Card.ByModel` has the `haiku→opus` pair with the right token totals + `Routes`. Keep the existing
global-aggregate and `SavedUSD` assertions intact. Reuse the `routedEvent` helper (`:45-55`); if it
doesn't already set token counts, pass them through.

#### web/src/types/board.ts

Action: MODIFY. Add to the `Card` interface (`:39-60`): `tokensIn: number`, `tokensOut: number`,
`byModel?: ModelRollup[]`. Add to `ModelRollup` (`:80-85`): `tokensIn: number`, `tokensOut: number`.
Strong typing only; no `any`.

#### web/src/App.tsx

Action: MODIFY.

- Widen `View` to include `'tokens'`.
- Add a third tab button next to Board/Standup (`:33-48`), `onClick={() => setView('tokens')}`,
  labelled "Tokens".
- Render `{view === 'tokens' && <TokenBreakdownView board={board} />}` in the content area.
- **Remove** the rail `<TokenSavingsMeter savings={board.tokenSavings} />` (`:52`) and its import.
- Import `TokenBreakdownView`.

Restrictions: do not change `useBoard()` wiring or the overall layout beyond the tab + meter removal.

#### web/src/components/TokenBreakdownView/index.tsx

Action: NUEVO. Props `{ board: Board }` — **non-nullable**. `App.tsx` guards `if (!board) { return
<loading placeholder> }` (`:15-23`) **before** rendering any tab view, so `board` is always present
when `TokenBreakdownView` mounts; do **not** re-handle board-loading or board-error here (that is
App-level). Note: `StandupView` takes **no props** and owns its own `useStandup()` fetch — it is the
pattern for layout/empty-state *markup*, **not** for the data contract (the tokens tab reuses the
board App already holds; it does not fetch). The only tab-local state is **idle** (rows) vs **empty**
(`board.tokenSavings.routes === 0`).

Renders:
- **Header / aggregate**: "Token Breakdown" title and a headline **tokens saved** =
  `formatCompact(board.tokenSavings.tokensIn + board.tokenSavings.tokensOut)`, the route count
  (`board.tokenSavings.routes`), and the global per-model breakdown (`board.tokenSavings.byModel`,
  each as `model→baseline  <tokens>  <routes>×`). **No `$`.**
- **Rows**: collect `board.columns.flatMap(c => c.cards).filter(card => card.routes > 0)`, sort by
  `(tokensIn+tokensOut)` desc; render one `TokenBreakdownRow` per spec: id, title, tokens-in,
  tokens-out, routes, and the per-model breakdown from `card.byModel`.
- **States** (tab-local only): **empty** (`board.tokenSavings.routes === 0` → hint: "No routed token
  events yet — run `/vector:raw` and follow-up commands to log cheap-agent routing here.") vs **idle**
  (the rows). Board **loading/error/reconnect are handled by `App.tsx`** (the `if (!board)` guard at
  `:15-23` and `useBoard()`'s connection/error handling) — this component is never mounted without a
  board, so it does **not** render its own loading or error/retry UI.

Restrictions: pure projection — no fetch, no mutation, no `$`, no new dependency, no table library.

#### web/src/components/SpecCard/SpecCard.tsx

Action: MODIFY (`:76-81`). Keep the `card.routes > 0` gate and the `Sparkles` icon; replace the label
body with the spec's tokens: `formatCompact(card.tokensIn + card.tokensOut)` followed by "tok"
(exact suffix `TBD — ver Open questions`). Update the `title` tooltip to read e.g.
`${card.routes} cheap-agent routes`. Remove the now-unused `formatUsd` import.

#### web/src/lib/format.ts

Action: MODIFY. Remove `formatUsd` (`:4-8`) once `SpecCard` and `TokenSavingsMeter` no longer call it.
Keep `formatCompact`. If any other caller surfaces, stop and report rather than leaving it dangling.

#### web/src/components/TokenSavingsMeter/

Action: DELETE. The `$`-denominated aggregate meter and its CSS are removed once `App.tsx` stops
rendering it. Confirm no other importer remains before deleting.

---

## 7. API Contract

No `docs/api-contract.md`; the Go structs are the source of truth and `web/src/types/board.ts` mirrors
them by hand. Changes (all **additive**, `GET /api/board` only):

- `Card` gains `tokensIn: int`, `tokensOut: int`, `byModel?: ModelRollup[]`.
- `ModelRollup` gains `tokensIn: int`, `tokensOut: int`.
- `Board.schemaVersion` increments.
- `GET /api/standup`, `/api/activity`, `/api/summary`, `/api/file`, `/api/events` are **unchanged**;
  no new endpoint is added.

Example per-spec card slice (the Go struct is canonical; the TS mirror must match):

```json
{
  "id": "add-standup-digest",
  "title": "Standup digest",
  "savedUsd": 0.41,
  "routes": 2,
  "tokensIn": 12000,
  "tokensOut": 3400,
  "byModel": [
    { "model": "haiku", "baseline": "opus", "routes": 1, "tokensIn": 12000, "tokensOut": 0, "savedUsd": 0.31 },
    { "model": "sonnet", "baseline": "opus", "routes": 1, "tokensIn": 0, "tokensOut": 3400, "savedUsd": 0.10 }
  ]
}
```

> The in/out split across the two model entries above is **illustrative**, not literal — a real
> `agent.routed` carries both `tokensIn` and `tokensOut` for one model. The example just shows the
> shape; do not infer that in/out are attributed to separate models.

### Endpoints involved

- GET /api/board (existing; carries the larger shape) — consumed via `useBoard()` SSE.

---

## 8. Success criteria

The implementation is correct when:

- [ ] `Card` carries `TokensIn`/`TokensOut`/`ByModel` and `ModelRollup` carries `TokensIn`/`TokensOut`,
      populated by `rollupSavings`/`toCard`; `SchemaVersion` bumped; specs with no routes default to
      `0`/empty without panics.
- [ ] `GET /api/board` returns the per-spec token fields; the web mirror types match (no `any`).
- [ ] A "Tokens" tab renders next to Board and Standup; selecting it shows the token breakdown.
- [ ] The tab shows an aggregate **tokens saved** headline (`tokensIn+tokensOut`), route count, and a
      global per-model breakdown — **with no `$` anywhere**.
- [ ] One row per spec with `routes > 0`: id, title, tokens-in, tokens-out, routes, and the per-spec
      model breakdown; rows sorted by `tokensIn+tokensOut` desc; the global per-model breakdown shows
      non-zero token counts (the `:284-291` fix landed).
- [ ] The empty state renders when `board.tokenSavings.routes === 0`; the table updates live via the
      shared SSE (board loading/error stay App-level — the tab renders no own loading/retry UI).
- [ ] The dollar `TokenSavingsMeter` is gone from the board; the component + `formatUsd` are deleted
      (verified by `grep -r "TokenSavingsMeter\|formatUsd" web/src/` returning nothing); the per-card
      badge shows tokens (`formatCompact`), not `$`.
- [ ] No regression: board kanban, standup view, `/api/*` endpoints, and the state machine intact.
- [ ] `go vet`/`go test` green; web typecheck + build succeed; binary rebuilt + reinstalled.

### Required tests

- [ ] `TestBuildRollsUpTokenSavings` extended: per-spec `TokensIn`/`TokensOut` sum correctly and
      `Card.ByModel` carries the right pairs + token totals; existing global + `SavedUSD` assertions
      still pass.
- [ ] (web) typecheck passes with the new fields/component; build succeeds; the empty state renders
      when `routes === 0` (manual render check — there is no JS test runner today; introducing one is
      Open question #5).

### Verification commands

```bash
go -C cli vet ./...
go -C cli test ./...
npm --prefix web run typecheck
npm --prefix web run build
```

The phase is not complete if any of these fail.

---

## 9. UX criteria

### Tokens tab

- Reachable from the top-level tab bar; selected state styled like the existing Board/Standup tabs.
- **Aggregate headline** at the top: large "tokens saved" figure (compact, e.g. "15.4k"), route count,
  and the per-model breakdown — never a `$` value.
- **Rows**: a readable list/table, one spec per row, comfortable padding, sufficient contrast. The
  per-model breakdown is visually secondary (indented/sub-rows or a muted sub-list under the row).
- Default order: highest token consumers first.
- **Empty**: when nothing was routed (`board.tokenSavings.routes === 0`), a clear hint (not a blank
  screen).
- **Loading / error**: handled by `App.tsx` (the `if (!board)` loading placeholder and `useBoard()`
  connection/error handling) — the tab itself renders no spinner or retry; it only mounts with a board.
- **Forms / Passwords**: N/A — read-only projection, no inputs.

### Board (after removal)

- The board rail no longer shows the dollar meter; layout reflows without a gap/placeholder.
- Each card's badge now reads tokens (e.g. `15.4k tok`) with the same icon/position as before; cards
  with no routes still show no badge.

### Accessibility

- Use a real `<table>` with `<thead>`/`<tbody>` and `scope="col"` headers (or `role="table"` with
  matching roles if divs are used). The tab control is keyboard-focusable like the existing tabs.

---

## 10. Decisions made

Settled by the user — do not re-litigate:

- **A dedicated top-level "Tokens" tab** (not a board-rail widget, not folded into standup).
- **Tokens only — no dollars** anywhere in the tab.
- **The board's aggregate dollar meter is removed**; the savings live **only** in the Tokens tab,
  expressed as **tokens saved**.
- **The per-card `$ saved` badge is reframed to tokens.**
- **Per-row content = tokens-in / tokens-out / routes + a per-spec model breakdown** (the user picked
  the richest option).
- **"tokens saved" ≔ tokens routed to a cheaper-than-baseline model** (`tokensIn+tokensOut` of
  `agent.routed`), per §1 — the honest, model-invariant reading. A cost-weighted variant is **not**
  built (Open question).
- **Reuse the existing `useBoard()` SSE** — the tab is a pure projection, no new endpoint or fetch.
- **The CLI keeps its dollar economics** (`agent.routed`, `vector spec route`, pricing, global
  `TokenSavings` USD fields); only the **web** stops rendering dollars.

If the agent sees a seemingly better alternative, report it as an observation; do not implement it.

---

## 11. Edge cases

### Spec with routes but no title

- Render the id; leave the title cell empty / fall back to the id. Do not break the row.

### Multiple routed events for one spec

- Sum `tokensIn`/`tokensOut` and group the model breakdown by `model→baseline`; one row per spec, no
  duplicates (the per-spec rollup already keys by spec id).

### Spec with `tokensIn` or `tokensOut` zero on a model pair

- Show `0` (or a dash) for the empty side; the pair still appears if it has routes. `formatCompact(0)`
  renders `"0"` (confirmed: `Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits:
  1 }).format(0)` → `"0"`, not `"0.0k"`).

### No routed events at all

- `board.tokenSavings.routes === 0` and no card has `routes > 0` → the empty state, not a blank table.

### Board with specs but none routed

- `flatMap(...).filter(routes>0)` is empty → empty state (same as above).

### Board fetch fails / `vector serve` down

- `useBoard()` surfaces the error/disconnected state at the `App.tsx` level; until a board arrives the
  tab is not mounted (App shows its loading/error placeholder). No new failure mode in the tab.

### A model id not in the pricing table

- `SavedUSD` may be `0` (pricing-dependent), but **token counts are unaffected** — the tokens view
  stays correct even when the dollar economics can't be computed. (One reason tokens are the honest
  denominator here.)

### Schema drift between Go and the web mirror

- The web mirror must be updated in the **same change** as the Go struct (additive fields). An older
  web bundle reading a newer board simply ignores unknown fields; a newer web reading an older board
  must tolerate missing `byModel`/token fields (treat as `0`/empty) — guard with optional chaining.

### Double submit / mutation

- N/A — every endpoint is GET; the tab is read-only. Nothing to double-submit.

---

## 12. Required UI states

| State | What is shown | What the user can do |
|---|---|---|
| tokens idle | aggregate headline + per-spec rows (tokens only) | read; switch tabs |
| tokens empty | "No routed token events yet — run `/vector:raw` and follow-up commands…" | switch tabs, run commands |
| board loading/error (App-level) | App's loading placeholder / error UI — the tab is **not** mounted yet | wait / retry at App level |
| board (meter removed) | rail without the dollar meter; cards show `… tok` badges | use the board as before |

---

## 13. Validations

Read-only; no user-facing forms.

| Field | Rule | Message |
|---|---|---|
| N/A (no inputs) | — | — |

Server-side: unchanged; `GET /api/board` validation is as today.

---

## 14. Security and permissions

- Token counts are derived from the local `activity.jsonl` (gitignored, personal); the projection
  exposes only counts and spec ids/titles — no secrets, no payloads.
- The board server stays auth-free and read-only; this change adds **no** write surface.
- No new external calls; the tab reuses the existing local SSE.

---

## 15. Observability and logging

- No new event types; the tokens view is a derived projection, not an event source.
- Reuse existing error paths: `useBoard()` handles SSE errors/reconnect; the Go projection already
  tolerates malformed/again events (`rollupSavings` filters to `EvtAgentRouted`).

---

## 16. i18n / visible text

Vector has no i18n layer; web strings are English in code. New/changed visible strings:

| Key | Text |
|---|---|
| view.tokens | "Tokens" (tab label) |
| tokens.title | "Token Breakdown" |
| tokens.saved | "tokens saved" |
| tokens.routes | "routes" |
| tokens.col.spec | "Spec" |
| tokens.col.in | "In" |
| tokens.col.out | "Out" |
| tokens.col.routes | "Routes" |
| tokens.empty | "No routed token events yet — run /vector:raw and follow-up commands to log cheap-agent routing here." |
| card.badge.tok | "<n> tok" (per-card badge suffix) |

(No `tokens.error` string — board loading/error copy stays App-level; the tab renders no error UI.)

Exact wording is `TBD — ver Open questions` if the user wants different copy (e.g. "Token Usage" vs
"Token Breakdown", or "saved" vs "routed").

---

## 17. Performance

- The tab is a **pure projection** of the already-loaded `board`; it triggers no fetch and re-renders
  only when the shared SSE pushes a new board.
- Row build is one `flatMap` + `filter` + `sort` over the cards already in memory — fine for the
  expected scale (hundreds of specs). Pagination/virtualization is an Open question if it grows.
- The Go projection extension is O(events) in the same single pass `rollupSavings` already makes; no
  extra scan.
- Removing the dollar meter and `formatUsd` slightly shrinks the bundle.

---

## 18. Restrictions

The agent must not:

- Show any `$`/USD value in the Tokens tab (or reintroduce the dollar meter on the board).
- Add a new HTTP endpoint, a second SSE stream, or any client-side fetch for this view.
- Add write/mutation surface (the tab is read-only).
- Add new dependencies — no table/grid/charting library.
- Remove or change the CLI dollar economics (`agent.routed` fields, `vector spec route`, pricing,
  global `TokenSavings` USD fields).
- Change the standup view, the activity-log schema, the state machine, or card metadata beyond the one
  reframed badge.
- Give the web canonical state or a router; selection/tab is local UI state.
- Use `any`/`interface{}` outside justified (de)serialization boundaries.
- Let the Go struct and the web mirror drift (update both in this change).
- Ignore vet/test/typecheck/build failures.

---

## 19. Deliverables

On completion:

- [ ] `Card` + `ModelRollup` token fields; `rollupSavings`/`toCard` populate them; `SchemaVersion`
      bumped; Go test extended and green.
- [ ] `web/src/types/board.ts` mirrors the new fields.
- [ ] `TokenBreakdownView` (+ row/breakdown subcomponents) + CSS; "Tokens" tab wired in `App.tsx`.
- [ ] Dollar `TokenSavingsMeter` removed from the board and the component deleted; `formatUsd` removed.
- [ ] `SpecCard` badge reframed to tokens.
- [ ] Gate green: `go vet`, `go test`, web typecheck, web build.
- [ ] Binary rebuilt + reinstalled to `~/.local/bin/vector` (dogfooding uses the PATH binary).

---

## 20. Final checklist for the agent

- [ ] Read this whole spec, including the §1 "tokens saved" definition.
- [ ] Confirmed per-spec tokens **must** come from the Go projection (not derivable in the client).
- [ ] Extended `rollupSavings`/`toCard` in the existing single pass; bumped `SchemaVersion`; updated
      the web mirror in lockstep.
- [ ] The Tokens tab is a pure projection of `useBoard()` — no new endpoint, no fetch, no mutation.
- [ ] No `$` anywhere in the tab; the board dollar meter and `formatUsd` are gone; the card badge is
      token-denominated.
- [ ] Rows sorted tokens-desc; per-spec model breakdown present; empty state (`routes===0`) renders;
      loading/error confirmed App-level (no tab-local spinner/retry).
- [ ] No new dependency; one component per file; strong typing (no `any`).
- [ ] Ran `go vet`, `go test`, web typecheck, web build.
- [ ] Rebuilt and reinstalled the `vector` binary.
- [ ] Left no temporary logs or unjustified TODOs.

---

## Open questions

1. **Cost-weighted "saved" variant** — should a future view express savings as *baseline-equivalent*
   cost (the dollar saving) rather than raw routed-token volume? This spec builds tokens-only; flag if
   a weighted figure is wanted later.
2. **Per-card badge suffix/label** — "`15.4k tok`" vs "`15.4k routed`" vs an icon-only count.
3. **Tab label / headline wording** — "Token Breakdown" vs "Token Usage"; "saved" vs "routed".
4. **Sorting/scale** — interactive column sort, search/filter, and pagination/virtualization are out of
   MVP; revisit if a repo accrues many routed specs.
5. **Web test runner** — there is no JS test runner today (gate = typecheck+build). Decide whether the
   empty/sort logic warrants introducing one, or stays a manual check.
6. **`formatCompact(0)` output** — confirm it renders "0" (not "0.0k") for empty token cells.
