# Tasks — add-dark-mode

## 1. Pure theme-resolution logic (testable, no DOM)

- [ ] 1.1 `web/src/context/themeResolve.ts`: new pure module. `export type ThemeMode = 'light' |
      'dark' | 'system'`; `resolveTheme(mode: ThemeMode, prefersDark: boolean): 'light' | 'dark'`
      (`system` → `prefersDark ? 'dark' : 'light'`; explicit modes ignore `prefersDark`);
      `parseStoredMode(raw: string | null): ThemeMode` (valid enum passes through; anything else →
      `'system'`). No React, no DOM. Reference: `web/src/components/SpecDetailsDrawer/entries.ts`.
- [ ] 1.2 `web/src/context/themeResolve.test.ts`: Vitest unit test (pure). Cover `resolveTheme` for
      every `mode` × `prefersDark` combination and `parseStoredMode` for valid values, `null`, and
      garbage. Reference: `web/src/components/SpecDetailsDrawer/entries.test.ts`.

## 2. Theme context

- [ ] 2.1 `web/src/context/ThemeContext.tsx`: new `ThemeProvider` + `useTheme(): { mode: ThemeMode;
      resolved: 'light' | 'dark'; setMode(m: ThemeMode): void }`. On mount: read/validate
      `localStorage['vector-theme']` via `parseStoredMode`, resolve via `resolveTheme`, write
      `document.documentElement.dataset.theme`. Persist `mode` on change. Subscribe to
      `matchMedia('(prefers-color-scheme: dark)')` and re-resolve live **only while** `mode ===
      'system'`, with `useEffect` cleanup. Guard `window`/`localStorage`/`matchMedia` access
      (`try/catch`; read failure → `system`). Reference: `web/src/api/useBoard.ts`,
      `web/src/main.tsx`. No inline styles, no component-specific logic.

## 3. Dark token palette

- [ ] 3.1 `web/src/styles/tokens.css`: keep `:root { … }` as the light default. Add
      `:root[data-theme="dark"] { … }` and `@media (prefers-color-scheme: dark) { :root:not([data-theme]) { … } }`,
      both assigning the Dracula dark values from the spec's Dracula→token mapping (§6). Add
      `:root[data-theme="light"] { … }` pinning the light values. Do not rename existing variables.
- [ ] 3.2 Add the new extracted tokens with both light and dark values: `--color-accent-bg`
      (`#eef2ff` / `#312a46`), `--conn-live` (`#10b981` / `#50fa7b`), `--conn-reconnecting`
      (`#f59e0b` / `#ffb86c`), `--conn-error` (`#ef4444` / `#ff5555`), `--color-success-fg`
      (`#047857` / `#50fa7b`), `--color-success-bg` (`#d1fae5` / `#1f3a2c`).
- [ ] 3.3 Tune dark values flagged *(tune for AA)* in the spec until they meet WCAG AA — in
      particular lighten `--color-text-secondary` / `--color-text-tertiary` (raw Dracula
      `#6272a4` fails) and the status/savings pill pairs. Verify with a contrast checker.
- [ ] 3.4 Add a color-only transition (~150ms) on themable surfaces, gated by
      `@media (prefers-reduced-motion: reduce) { * { transition: none } }`. No layout/size animation.

## 4. Tokenize hardcoded hex in components

- [ ] 4.1 Run `grep -rE '#[0-9a-fA-F]{6}' web/src/components` to confirm the full set of inlined
      colors; tokenize any additional ones found (in scope) alongside the known list.
- [ ] 4.2 Replace inlined hex with `var(--token)` in: `SpecCard/SpecCard.module.css` (`#eef2ff`);
      `SpecCard/CardNextCommand.module.css` (`#eef2ff`, `#047857`, `#d1fae5`);
      `SpecDetailsDrawer/SpecDetailsDrawer.module.css` (`#eef2ff` + success greens);
      `BoardHeader/BoardHeader.module.css` (`#10b981`, `#f59e0b`, `#ef4444`). Color rules only — do
      not change layout or any non-color rule.

## 5. Boot script, provider mount, and control

- [ ] 5.1 `web/index.html`: add a minimal, dependency-free, `try/catch`-wrapped inline `<script>` in
      `<head>` **before** `<script type="module" src="/src/main.tsx">` that reads
      `localStorage['vector-theme']`, resolves (`system`/absent → `matchMedia` match), and sets
      `document.documentElement.setAttribute('data-theme', resolved)`. Runs before first paint.
- [ ] 5.2 `web/src/main.tsx`: wrap `<App />` with `<ThemeProvider>` (inside or around `StrictMode`);
      keep the existing root-element guard and imports.
- [ ] 5.3 `web/src/components/BoardHeader/BoardHeader.tsx`: render the tri-state Light/Dark/System
      control consuming `useTheme`, on the header's right side near the connection status, without
      changing existing layout/behavior. Icons `Sun`/`Moon`/`Monitor` (`lucide-react`) reflect the
      current mode; `aria-label`/`title` reflects it (e.g. "Theme: System (click to change)"); visible
      `:focus` ring; keyboard-operable. Promote to `BoardHeader/ThemeControl.tsx` if it grows past a
      single button with one click handler (menu / multiple buttons / local open-close state).

## 6. Verification

- [ ] 6.1 `npm --prefix web run typecheck` clean (no `any`, no TS errors).
- [ ] 6.2 `npm --prefix web run test` green (`themeResolve.test.ts`).
- [ ] 6.3 `npm --prefix web run build` succeeds (required for the binary embed).
- [ ] 6.4 Manual QA via `vector serve`: first load follows OS with no stored pref; control switches
      Light/Dark/System and the whole UI updates; System tracks live OS changes; choice persists
      across reloads; explicit Light/Dark beats the OS; every surface adapts (board, header, columns,
      cards, status pills, priority flags, drawer, standup, savings meter); ~150ms fade present and
      disabled under reduced-motion; no behavior regression.
- [ ] 6.5 Manual WCAG-AA contrast check (WebAIM/axe) for the enumerated pairs (spec §8): `--color-text`
      on `--color-app-bg` and on `--color-surface`; `--color-text-secondary`/`--color-text-tertiary`
      on `--color-surface`; all six status pill fg/bg pairs; each `--priority-*` on `--color-surface`;
      `--savings-fg` on `--savings-bg`.
- [ ] 6.6 Re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild + reinstall the `vector`
      binary to `~/.local/bin/vector` (dogfooding uses the PATH binary).
