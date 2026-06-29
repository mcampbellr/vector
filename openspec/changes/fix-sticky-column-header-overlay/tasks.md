# Tasks — fix-sticky-column-header-overlay

## 1. CSS fix (the defect)

- [x] 1.1 `web/src/components/BoardColumn/BoardColumn.module.css`: in the `.header` rule, add
      `background-color: var(--color-app-bg)` (an existing opaque token from
      `web/src/styles/tokens.css`; candidates `--color-app-bg` `#f7f8fa` / `--color-surface`
      `#ffffff` / `--color-surface-muted` `#f3f4f6`, `--color-app-bg` preferred so the header masks
      the column track) and `z-index: 1`. Keep all existing properties (`display`, `align-items`,
      `gap`, `padding`, `position: sticky`, `top: 0`). Never hardcode a color; never add a new token
      to `tokens.css`. Bump `z-index` only if `1` proves insufficient to clear the `.cards` stacking
      context.
- [x] 1.2 Do not modify any other rule in the file (`.column`, `.cards`, `.title`, `.count`,
      `.empty`) and do not add `border`/`box-shadow`/separator. Do not change `BoardColumn.tsx`
      unless strictly required for the test setup.

## 2. DOM test environment (conditional — only if not already present)

- [x] 2.1 `web/package.json`: confirm whether a DOM test env is configured. If not, add to
      `devDependencies` a DOM environment (`happy-dom`, lighter and Vitest-native — preferred; or
      `jsdom`) and optionally `@testing-library/react` at a React-19-compatible version. Do not touch
      `dependencies`, `scripts`, or any other field; do not duplicate an already-present dep.
- [x] 2.2 `web/vite.config.ts`: if no `test` block exists in `defineConfig`, add
      `test: { environment: 'happy-dom' }` (or `'jsdom'` matching 2.1) at the same level as
      `plugins`/`server`/`build`. Do not change existing keys or the `API_TARGET`/`/api` proxy. If a
      `test` block already exists, only ensure `environment` is set.

## 3. Component test

- [x] 3.1 `web/src/components/BoardColumn/BoardColumn.test.tsx`: new Vitest component test. Import
      `BoardColumn` and types `Column`/`Card` from `../../types/board`. Define a `makeColumn(overrides:
      Partial<Column>): Column` fixture (sensible defaults: label, count, empty cards) — own fixture,
      not `makeCard`. Reference style: `web/src/components/SpecCard/relationChips.test.ts` (describe/
      it/expect, Vitest imports) and `web/src/components/SpecDetailsDrawer/entries.test.ts` (defaults
      + partial-override fixture pattern).
- [x] 3.2 Case "column with cards": render `makeColumn({ cards: [...3 minimal cards] })`; assert the
      `<header>` is in the DOM, the `<h2>` contains the column label, and the `<span>` contains the
      count.
- [x] 3.3 Case "empty column": render `makeColumn({ cards: [] })`; assert the "No specs" text is
      visible. No assertions on computed CSS (`background-color`/`position`/`z-index`), no API mocks,
      no empty snapshots.

## 4. Verification

- [x] 4.1 `npm --prefix web run typecheck` clean (no `any`, no TS errors).
- [x] 4.2 `npm --prefix web run test` green — new `BoardColumn.test.tsx` plus existing
      `entries.test.ts` / `relationChips.test.ts` (no regression).
- [x] 4.3 `npm --prefix web run build` succeeds (required for the binary embed).
- [x] 4.4 Source review: `.header` declares `background-color: var(--token)` (not `transparent`, not
      hardcoded) and `z-index > 0`; `.column`/`.cards`/`.title`/`.count`/`.empty` intact.
- [ ] 4.5 Manual QA via `vector serve`: scroll a column with enough cards to trigger overflow —
      header stays an opaque, legible band with no card overlap; layout of unscrolled/empty columns
      is unchanged. Spot-check Firefox and Safari for scrollbar artifacts from `z-index`.
- [x] 4.6 Re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild + reinstall the `vector`
      binary to `~/.local/bin/vector` (dogfooding uses the PATH binary).
