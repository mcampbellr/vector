# Design — add-dark-mode

## Context

The board (`web/`) is a React 19 + Vite 6 + TypeScript CSR app, styled with CSS Modules over a
single set of CSS custom properties defined on `:root` in `web/src/styles/tokens.css` (imported
once in `web/src/main.tsx`). Every component already consumes `var(--token)` rather than literal
colors — except a handful of hardcoded hex values inlined in component `*.module.css` files. State
management is the React Context API; there is no theming library and no canonical client state (the
board is a read-only SSE projection of CLI state). Vitest tests **pure logic only** — no DOM
environment (`jsdom`/`happy-dom`) and no React Testing Library are installed, and this phase adds
no dependencies.

## Goals / Non-Goals

**Goals:**
- A tri-state Light / Dark / System theme for the **whole** board web UI, driven by the existing
  token set (one shared palette, not board-only).
- System tracks the OS live (`matchMedia`); explicit Light/Dark overrides the OS; the choice
  persists per browser (`localStorage`) and applies **before first paint** (no flash).
- A Dracula-based dark palette meeting **WCAG AA**, with a ~150ms color fade honoring
  `prefers-reduced-motion`.

**Non-Goals:**
- Any non-visual behavior, new board feature, flow, or domain state; the board stays read-only.
- CLI/API theming or writing the theme to the state JSON (theme is browser-only).
- A settings/preferences panel, scheduled auto-switching beyond OS-follow, server/cross-device
  persistence, or any redesign of the light palette.
- Adding a theming dependency (`next-themes`, MUI, Tailwind, CSS-in-JS) or any new dependency.

## Decisions

- **React Context + CSS custom properties.** `ThemeProvider` owns `mode` and writes a resolved
  `data-theme` attribute onto `document.documentElement`; all visual change happens in CSS by
  overriding token **values** under `:root[data-theme="dark"]` / scoped `@media (prefers-color-scheme: dark)`.
  Existing variable names are preserved; components stay theme-agnostic. Chosen because the token
  system already exists, so a built-in Context keeps the embedded bundle light (no new dependency).
- **`data-theme` on `<html>`** as the activation hook — standard, CSS-Modules-safe, and it lets the
  OS `@media` query be scoped to `:root:not([data-theme])` so the OS only wins when no explicit
  choice is set; `:root[data-theme="light"]` pins light against a dark OS.
- **Pure helpers extracted** to `web/src/context/themeResolve.ts`: `resolveTheme(mode, prefersDark)`
  and `parseStoredMode(raw)` (anything outside the enum → `'system'`). This is the only logic with
  branches, and the Vitest setup is DOM-free — so the testable decisions live outside React/DOM
  (mirrors the `SpecDetailsDrawer/entries.ts` pure-module pattern). The provider's DOM side effects
  are verified by manual QA, not automated, because no DOM test env is installed and none is added.
- **No-flash inline boot script** in `web/index.html` `<head>`, before the module script: reads
  `localStorage['vector-theme']`, resolves (`system`/absent → `matchMedia` match), sets `data-theme`
  before first paint. Required — provider-only application mounts after first paint in this CSR app,
  so it would not prevent the flash. The script is minimal, dependency-free, `try/catch`-wrapped, and
  mirrors (does not replace) the provider's resolution.
- **`localStorage` (`vector-theme`)** for persistence, never the state JSON or the server — a theme
  is a per-browser personal preference, not shared board state. Reads/writes are guarded
  (`try/catch`); on failure the theme works in-memory for the session and falls back to `system`.
- **Dracula** as the dark palette source, mapped token-by-token (anchor palette in the spec).
  Low-emphasis text anchors (`comment #6272a4`) fail AA on dark surfaces and are **lightened** until
  they pass; status/savings pill backgrounds are tuned likewise. Dark values are starting anchors,
  verified with a contrast checker before merge.
- **Theme control = tri-state in `BoardHeader`**, right side near the connection status:
  `Sun`/`Moon`/`Monitor` reflecting the current mode, an `aria-label`/`title` like
  "Theme: System (click to change)", and a visible focus ring. Either a Light→Dark→System cycle or a
  small menu is acceptable as long as the active mode is visible and the control is keyboard-operable.
  Promoted to `BoardHeader/ThemeControl.tsx` the moment it needs more than a single button with one
  click handler (a menu, multiple buttons, or local open/close state).
- **Gated transition**: a ~150ms color-only transition on themable surfaces, disabled under
  `@media (prefers-reduced-motion: reduce)`; no layout/size animation.

## Risks / Trade-offs

- **Dracula contrast for low-emphasis text** — raw Dracula foregrounds can fail AA on dark
  surfaces. Mitigated by tuning secondary/tertiary text and pill backgrounds and verifying the
  enumerated pairs with a contrast checker before merge.
- **Flash of wrong theme** — unavoidable with provider-only application in CSR; mitigated by the
  inline boot script that sets `data-theme` before first paint.
- **No automated coverage of DOM side effects** — `ThemeProvider`'s attribute/persistence/`matchMedia`
  behavior is verified by manual QA, since adding a DOM test env is explicitly out of scope. The pure
  resolution logic that carries all the branching is unit-tested.
- **`localStorage` unavailable (private mode/quota)** — guarded; theme degrades to in-memory +
  `system` fallback with no visible error.

## Migration / Rollout

Pure presentation change; no state, schema, API, or data migration. Verified by
`npm --prefix web run typecheck`, `npm --prefix web run test`, and `npm --prefix web run build`,
then manual visual + WCAG-AA contrast QA of both themes across every surface (board, header,
columns, cards, status pills, priority flags, details drawer, standup view, savings meter) via
`vector serve`. On completion, re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild +
reinstall the `vector` binary for dogfooding.
