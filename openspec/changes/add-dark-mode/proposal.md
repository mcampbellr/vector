# Dark mode for the board UI

## Why

Vector's local board web UI (`web/`) ships only a light theme, which is uncomfortable in
low-light environments and out of step with the developer/terminal tooling its audience already
runs in dark. The token system needed to support a second palette already exists
(`web/src/styles/tokens.css`, `:root` CSS custom properties consumed via `var(--token)` across
every component), so a dark theme is a low-cost, purely visual addition: a second set of token
values plus a way to select it. No board behavior, data, domain state, or CLI/API surface changes.

## What changes

- **Dark palette (web)** — add Dracula-based dark values for every existing `:root` token in
  `web/src/styles/tokens.css` (surfaces, borders, text, status fg/bg pairs, priority, savings,
  accent, brand gradient), activated two ways that drive the same values:
  `@media (prefers-color-scheme: dark)` scoped to `:root:not([data-theme])` (OS only wins when no
  explicit choice), and `:root[data-theme="dark"]` for an explicit Dark; `:root[data-theme="light"]`
  pins light so an explicit Light beats a dark OS. Dark values are tuned to meet **WCAG AA**.
- **Theme context (web)** — new `web/src/context/ThemeContext.tsx`: a `ThemeProvider` + `useTheme()`
  hook holding `mode: 'light' | 'dark' | 'system'`, persisting `mode` to `localStorage`
  (`vector-theme`), reflecting the **resolved** theme onto `document.documentElement` via
  `data-theme`, and subscribing to `matchMedia('(prefers-color-scheme: dark)')` live while
  `mode === 'system'`. The two pure decisions (`resolveTheme`, `parseStoredMode`) are extracted to
  `web/src/context/themeResolve.ts` for unit testing without a DOM.
- **Theme control (web)** — a tri-state Light / Dark / System control in `BoardHeader` (`lucide-react`
  `Sun` / `Moon` / `Monitor`, accessible name reflecting the current mode, visible focus ring),
  consuming `useTheme`. Promoted to its own `BoardHeader/ThemeControl.tsx` if it grows past a bare
  cycle button.
- **No-flash boot (web)** — a tiny inline `<script>` in `web/index.html` `<head>` reads the stored
  preference and sets `data-theme` before the module bundle paints (required in this CSR app).
- **Provider mount (web)** — wrap `<App />` with `<ThemeProvider>` in `web/src/main.tsx`.
- **Tokenize hardcoded hex (web)** — extract inlined colors (`#eef2ff`, connection dots
  `#10b981` / `#f59e0b` / `#ef4444`, success greens `#047857` / `#d1fae5`) from component
  `*.module.css` into new `:root` tokens with dark counterparts, so dark mode actually reaches them.
- **Gated transition (web)** — a ~150ms color-only fade on themable surfaces, disabled under
  `@media (prefers-reduced-motion: reduce)`.

## Capabilities

### Added Capabilities
- `board-theme`: the board web UI gains a tri-state Light / Dark / System theme, persisted per
  browser, applied before first paint, tracking the OS live while in System, with a Dracula dark
  palette meeting WCAG AA — all driven by the existing CSS-custom-property token set.

### Modified Capabilities
- `board-header`: gains the theme control on its right side near the connection status; existing
  layout and behavior are otherwise unchanged.

## Impact

- New: `web/src/context/ThemeContext.tsx`, `web/src/context/themeResolve.ts`,
  `web/src/context/themeResolve.test.ts`, and (if non-trivial) `web/src/components/BoardHeader/ThemeControl.tsx`.
- Modified: `web/src/styles/tokens.css` (dark values + extracted tokens + gated transition),
  `web/index.html` (boot script), `web/src/main.tsx` (provider), `web/src/components/BoardHeader/BoardHeader.tsx`
  (control), and the `*.module.css` files carrying hardcoded hex (`SpecCard`, `CardNextCommand`,
  `SpecDetailsDrawer`, `BoardHeader`).
- **No** new dependencies, **no** CLI/API/domain changes, **no** writes to the state JSON — theme
  lives entirely in the browser (`localStorage` + a DOM attribute). The board stays a read-only
  projection of CLI state.
- After the `web/` change, re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild +
  reinstall the `vector` binary (dogfooding uses the PATH binary).

Authored spec: `.vector/specs/add-dark-mode/spec.md`.
