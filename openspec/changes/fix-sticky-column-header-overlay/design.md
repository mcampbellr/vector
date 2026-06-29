# Design — fix-sticky-column-header-overlay

## Context

The board (`web/`) is a React 19 + Vite 6 + TypeScript CSR app styled with CSS Modules over a single
set of CSS custom properties on `:root` in `web/src/styles/tokens.css`. `BoardColumn` renders a
`<header>` (`.header`) over a scrolling `.cards` container (`overflow-y: auto`). `.header` is
`position: sticky; top: 0` but lacks `background-color` and `z-index`, so it is transparent and
shares the cards' stacking plane — on scroll, card content paints over the label/count. The defect
came in whole with commit `5441b80` and maps to no spec or ticket (`relatedTo[]` stays empty).
Vitest in `web/` tests **pure logic only**; no DOM environment (`jsdom`/`happy-dom`) or React Testing
Library is installed.

## Goals / Non-Goals

**Goals:**
- The column's sticky header stays an opaque, legible band at every scroll position; cards scroll
  under it.
- The background is an existing `tokens.css` custom property (dark-mode-ready), never a hardcoded
  color.
- A component test pins the header's DOM structure (element + label + count, and the empty state).

**Non-Goals:**
- Any change to `BoardColumn.tsx` JSX, to other `BoardColumn.module.css` rules, to `tokens.css`
  (no new token), or to any other board component.
- CLI/API/domain/state changes; the board stays a read-only SSE projection.
- Dark-mode values (that is `add-dark-mode`), borders/shadows/separators on the header, and
  automated visual-regression of real scroll (no browser-testing tool in the stack).

## Decisions

- **CSS Modules + existing design token.** The fix is two declarations on the existing `.header`
  rule: an opaque `background-color: var(--token)` and `z-index: 1`. Expressing the background as a
  token keeps the single source of truth for color (`standards/typescript-react.md`) and means a
  future `add-dark-mode` can override the value under `:root[data-theme="dark"]` without touching
  `BoardColumn.module.css`.
- **Token choice = `--color-app-bg` (preferred).** Three opaque candidates exist:
  `--color-app-bg` (`#f7f8fa`, the body/column-track background — header blends into the track and
  acts as a natural mask), `--color-surface` (`#ffffff`), `--color-surface-muted` (`#f3f4f6`).
  `--color-app-bg` is preferred for the mask effect; final pick is confirmed visually at
  implementation. No new token is introduced.
- **`z-index: 1` as the reference value.** The minimum to clear the normal stacking context of
  `.cards`; raised only if a browser proves `1` insufficient. `1` creates no extra compositing layer
  (only high z-index with `transform`/`will-change` does), so there is no render cost.
- **Test asserts DOM structure only.** Under CSS Modules in jsdom/happy-dom, computed
  `background-color`/`position`/`z-index` resolve to empty/default and are not observable, so the
  test pins element presence and textual content (label, count, "No specs"). The CSS correctness is
  verified by source review + successful `npm run build`, and the scroll behavior by manual QA.
- **Configuring the DOM test env is in-scope.** Because no DOM env is installed, component-testing
  `BoardColumn` requires adding a DOM env devDependency (`happy-dom` preferred for being lighter and
  Vitest-native; `jsdom` as the React-19-compat fallback) plus a `test: { environment }` block in
  `vite.config.ts` — both conditional on not already existing. This is a test-only devDependency; the
  shipped embedded bundle gains nothing. `@testing-library/react` is optional ergonomics.
- **Own `makeColumn` fixture.** The test builds a `Column`-typed fixture with defaults + partial
  overrides (the `entries.test.ts` pattern), not `makeCard` — `entries.test.ts`/`relationChips.test.ts`
  are style references, not reused.

## Risks / Trade-offs

- **No automated coverage of the visual fix.** Computed CSS is unobservable under CSS Modules in the
  test DOM, so `background-color`/`z-index` are checked by source review and manual scroll QA, not
  asserted. Accepted — the structural test guards the DOM contract and the build guards CSS validity.
- **Browser scrollbar artifacts.** A `z-index` on the sticky header could, in theory, interact with
  native scrollbars in Firefox/Safari; mitigated by manual spot-check in those browsers.
- **Adding a test dependency.** The DOM env is a new devDependency; mitigated by keeping it test-only
  (no runtime/bundle impact) and conditional (skipped if already configured).

## Migration / Rollout

Pure presentation change; no state, schema, API, or data migration. Verified by
`npm --prefix web run typecheck`, `npm --prefix web run test`, `npm --prefix web run build`, then
manual scroll QA across columns (with/without overflow, empty) in `vector serve`, spot-checking
Firefox and Safari. On completion, re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild +
reinstall the `vector` binary for dogfooding.
