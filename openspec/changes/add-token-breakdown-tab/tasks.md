# Tasks — add-token-breakdown-tab

## 1. Per-spec token projection (cli)

- [ ] 1.1 `cli/internal/board/board.go` — `Card` (`:40-60`): add `TokensIn int \`json:"tokensIn"\``,
      `TokensOut int \`json:"tokensOut"\``, `ByModel []ModelRollup \`json:"byModel,omitempty"\``.
      `ModelRollup` (`:96-102`): add `TokensIn int \`json:"tokensIn"\``, `TokensOut int
      \`json:"tokensOut"\`` (keep `SavedUSD`).
- [ ] 1.2 `rollupSavings` (`:256-303`): in the per-spec accumulation block (`:277-282`) also sum
      `tokensIn`/`tokensOut`, and build a per-spec `map[modelPair]*ModelRollup` accumulating tokens +
      routes (mirror the global `byModel` block at `:284-291`). Sort each spec's `ByModel` by
      `tokensIn+tokensOut` desc (mirror `:299-301`).
- [ ] 1.3 **Global `byModel` token fix** (`:284-291`): add `m.TokensIn += d.TokensIn` and
      `m.TokensOut += d.TokensOut` so the aggregate per-model breakdown is non-zero.
- [ ] 1.4 `toCard` (`:203-238`): set `card.TokensIn`/`card.TokensOut`/`card.ByModel` from the per-spec
      economics, next to `card.SavedUSD`/`card.Routes` (`:217-218`). Specs with no routes default to
      `0`/empty without nil-map panics.
- [ ] 1.5 `SchemaVersion` (`:16`): bump by one (additive). Do not touch the event schema, pricing, or
      the global `TokenSavings` USD fields.

## 2. Projection test (cli)

- [ ] 2.1 `cli/internal/board/board_test.go` — extend `TestBuildRollsUpTokenSavings` (`:94-136`):
      assert the routed spec's `Card.TokensIn`/`TokensOut` equal the sum of its routed events and that
      `Card.ByModel` carries the `haiku→opus` pair with the right token totals + `Routes`. Keep the
      existing global-aggregate and `SavedUSD` assertions. Reuse the `routedEvent` helper (`:45-55`);
      pass token counts through if it doesn't already.
- [ ] 2.2 `go -C cli vet ./...` and `go -C cli test ./...` green.

## 3. Web type mirror

- [ ] 3.1 `web/src/types/board.ts` — `Card` (`:39-60`): add `tokensIn: number`, `tokensOut: number`,
      `byModel?: ModelRollup[]`. `ModelRollup` (`:80-85`): add `tokensIn: number`, `tokensOut: number`.
      No `any`.

## 4. Tokens tab (web)

- [ ] 4.1 `web/src/components/TokenBreakdownView/index.tsx` (NEW) — props `{ board: Board }`
      (non-nullable). Render: a **header/aggregate** ("Token Breakdown" + headline tokens saved =
      `formatCompact(board.tokenSavings.tokensIn + board.tokenSavings.tokensOut)` + route count +
      global per-model breakdown `model→baseline  <tokens>  <routes>×`, no `$`); **rows** from
      `board.columns.flatMap(c => c.cards).filter(card => card.routes > 0)` sorted by
      `(tokensIn+tokensOut)` desc, each showing id, title, in, out, routes, and the per-spec model
      breakdown from `card.byModel`; tab-local **empty** state when `board.tokenSavings.routes === 0`
      (hint copy per spec §16) vs **idle**. No tab-local loading/error UI (App-level). Pure projection
      — no fetch, no mutation, no `$`, no new dependency, no table library. Use a real `<table>` with
      `<thead>`/`scope="col"` (or matching ARIA roles). Reference: `StandupView/index.tsx` markup.
- [ ] 4.2 `web/src/components/TokenBreakdownView/TokenBreakdownView.module.css` (NEW) — table/list +
      headline styles; per-model breakdown visually secondary (indented/muted). Reference:
      `StandupView.module.css`, `TokenSavingsMeter.module.css`.
- [ ] 4.3 `web/src/components/TokenBreakdownView/TokenBreakdownRow.tsx` (+ `SpecModelBreakdown.tsx`)
      (NEW, if the row/breakdown markup grows past a few lines) — one spec row + its model breakdown,
      one component per file, semantic names, strong typing. Reference: `StandupView/StandupSpecRow.tsx`.
- [ ] 4.4 `web/src/App.tsx` — widen `View` to include `'tokens'`; add a third tab button next to
      Board/Standup (`:33-48`) labelled "Tokens", `onClick={() => setView('tokens')}`; render
      `{view === 'tokens' && <TokenBreakdownView board={board} />}`; import `TokenBreakdownView`. Do
      not change `useBoard()` wiring or the overall layout beyond the tab + meter removal.

## 5. Remove dollars from the board (web)

- [ ] 5.1 `web/src/App.tsx` — remove the rail `<TokenSavingsMeter savings={board.tokenSavings} />`
      (`:52`) and its import.
- [ ] 5.2 `web/src/components/SpecCard/SpecCard.tsx` (`:76-81`) — keep the `card.routes > 0` gate and
      the `Sparkles` icon; replace the label body with `formatCompact(card.tokensIn + card.tokensOut)`
      + "tok"; update the `title` tooltip (e.g. `${card.routes} cheap-agent routes`); remove the unused
      `formatUsd` import.
- [ ] 5.3 `web/src/lib/format.ts` — remove `formatUsd` (`:4-8`); keep `formatCompact`. First run
      `grep -r "formatUsd" web/src/` and confirm no caller remains; if one surfaces, stop and report.
- [ ] 5.4 `web/src/components/TokenSavingsMeter/` (DELETE) — confirm no importer remains
      (`grep -r "TokenSavingsMeter" web/src/` returns nothing), then delete the component + CSS.

## 6. Verification

- [ ] 6.1 `npm --prefix web run typecheck` clean (no `any`, new fields/component compile).
- [ ] 6.2 `npm --prefix web run build` succeeds (required for the binary embed).
- [ ] 6.3 `grep -r "TokenSavingsMeter\|formatUsd" web/src/` returns nothing.
- [ ] 6.4 Manual render check via `vector serve`: Tokens tab renders next to Board/Standup; aggregate
      headline + per-spec rows (tokens only, no `$`); empty state when `routes === 0`; board rail has
      no dollar meter and reflows without a gap; per-card badge reads `… tok`; no regression to board,
      standup, or `/api/*`.
- [ ] 6.5 Re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild + reinstall the `vector`
      binary to `~/.local/bin/vector` (dogfooding uses the PATH binary).
