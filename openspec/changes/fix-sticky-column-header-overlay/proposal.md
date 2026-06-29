# Fix sticky column header overlapping cards on scroll

## Why

The board's `BoardColumn` sticky header (`web/src/components/BoardColumn/BoardColumn.module.css`,
`.header`) declares `position: sticky; top: 0` but has **no `background-color` and no `z-index`**,
so it renders transparent and in the same stacking plane as the scrolling `.cards` container
(`overflow-y: auto`). When a developer scrolls inside a column, card content flows over the column
label and count, leaving the header illegible. The defect was introduced whole in commit `5441b80`
(`feat(web): board kanban SPA with Token Savings Meter`), which maps to no registered spec or
ticket. The fix is purely declarative and local to one CSS file: give `.header` an opaque
background from an existing `tokens.css` custom property and a small `z-index`. No board behavior,
data, domain state, or CLI/API surface changes.

## What changes

- **Opaque sticky header (web)** — add `background-color: var(--token)` (an existing opaque token
  from `web/src/styles/tokens.css`; candidate `--color-app-bg`) and `z-index: 1` to the `.header`
  rule in `BoardColumn.module.css`, so cards scroll **under** an opaque header band instead of over
  it. Existing `.header` properties (`display`, `align-items`, `gap`, `padding`, `position: sticky`,
  `top: 0`) are unchanged; no `border`/`box-shadow`/separator is added; no other rule
  (`.column`, `.cards`, `.title`, `.count`, `.empty`) is touched; no new token is added to
  `tokens.css`.
- **DOM test environment (web)** — Vitest in `web/` currently runs **pure logic only**
  (`entries.test.ts`, `relationChips.test.ts`); no DOM environment is configured. To component-test
  `BoardColumn`, add a DOM env devDependency (`happy-dom` or `jsdom`, optionally
  `@testing-library/react` for React 19) to `web/package.json` and a `test: { environment }` block
  to `web/vite.config.ts` — **only if** not already present. No other `package.json` field or
  `vite.config.ts` key (`plugins`, `server`, `build`, `API_TARGET`) changes.
- **Component test (web)** — new `web/src/components/BoardColumn/BoardColumn.test.tsx` with a
  `makeColumn(overrides)` fixture (own `Column`-typed fixture; `entries.test.ts` is style reference
  only). Asserts DOM structure for a column with cards (header present; `<h2>` carries the label;
  `<span>` carries the count) and an empty column ("No specs" visible). **No** assertions on
  computed CSS (`background-color`/`position`/`z-index` are not observable under CSS Modules in
  jsdom/happy-dom), no API mocks, no empty snapshots.

## Capabilities

### Modified Capabilities
- `board-column`: the column's sticky header gains an opaque token-backed background and a
  `z-index`, so it stays a legible band while the column scrolls; card content no longer overlaps
  the label and count. Header semantics (`<header>` → `<h2>` + `<span>`) are unchanged.

## Impact

- New: `web/src/components/BoardColumn/BoardColumn.test.tsx`.
- Modified: `web/src/components/BoardColumn/BoardColumn.module.css` (two CSS declarations on
  `.header`); conditionally `web/package.json` (DOM test devDependency) and `web/vite.config.ts`
  (`test: { environment }`) if the DOM test env is not already configured.
- **No** dependency in the shipped bundle (test-only devDependency), **no** CLI/API/domain changes,
  **no** writes to the state JSON, **no** new design tokens. The board stays a read-only projection
  of CLI state.
- The CSS fix is verified by source review + successful build; the scroll behavior is verified by
  manual QA. After the `web/` change, re-embed `web/dist` into `cli/internal/webui/dist/` and
  rebuild + reinstall the `vector` binary (dogfooding uses the PATH binary).

Authored spec: `.vector/specs/fix-sticky-column-header-overlay/spec.md`.
