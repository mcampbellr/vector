# Spec: Dark mode for the board UI

## 1. Goal

Add a **dark theme** to Vector's local board web UI (`web/`) so a developer can use the
board comfortably in low-light environments. The feature is **purely visual**: it introduces
a second color palette and a way to select it, without changing any board behavior, data, or
the CLI/API.

This feature lets a developer **choose between Light, Dark, and System** themes from a control
in the board header. The choice is persisted locally; `System` follows the OS
`prefers-color-scheme` live. Every web surface (board, header, columns, cards, status pills,
priority flags, the details drawer, the standup view, the token-savings meter) adapts through
the **existing CSS custom-property token system** in `web/src/styles/tokens.css` — no per-color
rewrite of individual components beyond extracting a few hardcoded hex values into tokens.

Decisions already made by the user (Section 10): **tri-state Light / Dark / System** control
(System tracks the OS in real time); **whole board web UI** in scope (single shared token set);
**Dracula** as the named dark palette source; **WCAG AA** contrast target; **subtle ~150ms color
fade** on theme change, honoring `prefers-reduced-motion`.

## 2. Scope

### Included in this phase

- **Dark palette tokens** added to `web/src/styles/tokens.css`: a Dracula-based dark value for
  every existing `:root` token (surfaces, borders, text, status fg/bg pairs, priority, savings,
  accent, brand gradient).
- **Two activation paths in CSS**, both driving the same dark values:
  - `@media (prefers-color-scheme: dark)` scoped so it applies **only when the user has not
    chosen** an explicit theme (i.e. when `System` is active), and
  - `:root[data-theme="dark"] { … }` for an explicit Dark selection (and `:root[data-theme="light"]`
    pinning the light values so an explicit Light wins over a dark OS).
- **Theme context** `web/src/context/ThemeContext.tsx`: a `ThemeProvider` + `useTheme()` hook
  holding `mode: 'light' | 'dark' | 'system'`, persisting `mode` to `localStorage`, reflecting
  the **resolved** theme onto `document.documentElement` via `data-theme`, and — while `mode`
  is `system` — subscribing to `matchMedia('(prefers-color-scheme: dark)')` to re-resolve live.
- **Theme control** in `web/src/components/BoardHeader/BoardHeader.tsx`: an accessible control
  that cycles/selects Light → Dark → System, using `lucide-react` icons (`Sun` / `Moon` /
  `Monitor`) and an `aria-label`/`title` reflecting the current mode.
- **Provider mount** in `web/src/main.tsx`: wrap `<App />` with `<ThemeProvider>`.
- **Extract hardcoded hex values into tokens** so dark mode actually reaches them: `#eef2ff`
  (accent background), the `BoardHeader` connection dots `#10b981` / `#f59e0b` / `#ef4444`, and
  the success greens `#047857` / `#d1fae5` currently inlined in component `*.module.css` files
  (see Section 6). Each becomes a new `:root` token with a Dracula dark counterpart.
- **Color-transition** rule (~150ms) on the themable surfaces, gated by
  `@media (prefers-reduced-motion: reduce)` to disable it.
- **Tests**: a unit test for `ThemeContext` (resolve/persist/system-tracking) and manual visual
  QA of both themes; typecheck + build green.

### Out of scope

- **Any non-visual behavior.** No new board features, flows, data, or domain state. The board
  stays read-only (`architecture/state-model.md`).
- **CLI / API theming.** The Go binary and the HTTP API are untouched; `data-theme` and the
  preference live entirely in the browser (`localStorage`), never in the state JSON.
- **A dedicated settings/preferences panel.** The header control is the only affordance.
- **Time-of-day / scheduled auto-switching** beyond following the OS via `System`.
- **Server-side or cross-device persistence** of the theme.
- **Redesign of the palette identity** beyond adopting Dracula for dark; the light theme values
  are unchanged.

The agent must not implement anything outside the scope above, even if it seems related.

---

## 3. Project technologies and conventions

### Stack

- Framework: React 19 (`web/`)
- Language: TypeScript
- Build tool: Vite 6
- Package manager: npm (`web/package-lock.json`)
- UI library: none (hand-rolled components) + `lucide-react` for icons
- State management: React Context API (no new dependency for theme)
- Styling: **CSS Modules + CSS custom properties** (design tokens in
  `web/src/styles/tokens.css`, imported once in `web/src/main.tsx`)
- Testing: Vitest

### Relevant versions

- react / react-dom: `^19.1.0` (`web/package.json`)
- vite: `^6.0.0` (`web/package.json`)
- typescript: `^5.7.2` (`web/package.json`)
- lucide-react: `^0.469.0` (`web/package.json`)
- **No new dependencies are added by this phase.**

Do not use libraries, APIs, flags, or patterns not already present in the project. No theming
library (`next-themes`, MUI, Tailwind, CSS-in-JS) — Context + CSS variables only.

### Existing patterns to respect

- Global tokens as CSS custom properties on `:root` in `web/src/styles/tokens.css`.
- One `*.module.css` co-located per component; components reference `var(--token)`, not literals.
- Custom hooks under `web/src/api/` (e.g. `useBoard.ts`) as the structural reference for the
  `useTheme` hook shape; types live under `web/src/types/`.
- Strong typing, no `any` (project + user global rule).
- One component per file (user global rule); the theme control, if it grows past a trivial
  button, becomes its own component file under `BoardHeader/`.

---

## 4. Prior dependencies

Before starting, the following must exist (all confirmed in-repo):

- [x] Token system in `web/src/styles/tokens.css` (`:root` custom properties).
- [x] Global stylesheet import in `web/src/main.tsx` (`import './styles/tokens.css'`).
- [x] `BoardHeader` component (`web/src/components/BoardHeader/BoardHeader.tsx`).
- [x] App root mounting `<App />` in `web/src/main.tsx`; `web/index.html` with `<html lang="en">`.
- [x] `lucide-react` available for icons (`web/package.json`).

If a dependency is missing, stop and report exactly what is missing. Do not invent contracts,
paths, or structures.

---

## 5. Architecture

### Pattern to use

**React Context + CSS custom properties.** A `ThemeProvider` owns the theme `mode` and writes a
resolved `data-theme` attribute onto `document.documentElement`. All visual change happens in
CSS by overriding token values under `:root[data-theme="dark"]` / `@media (prefers-color-scheme: dark)`.
Components remain theme-agnostic — they already consume `var(--token)`.

### Layers affected

- presentation (web): **yes** — `BoardHeader` gains the theme control; the new provider wraps
  the app; no other component logic changes (they inherit via tokens).
- application/use-cases: **no**.
- domain (cli state): **no** — theme is never written to the state JSON.
- data/infrastructure: **no** — persistence is `localStorage`, browser-only.
- shared/common (web): **yes** — new `web/src/context/ThemeContext.tsx`; extended `tokens.css`.

### Expected flow

1. App mounts; `ThemeProvider` initializes: read `localStorage['vector-theme']`
   (`'light' | 'dark' | 'system'`); default to `'system'` when absent or invalid.
2. Resolve the effective theme: `system` → `matchMedia('(prefers-color-scheme: dark)').matches`
   ? `dark` : `light`; otherwise the chosen mode.
3. Provider sets `document.documentElement.dataset.theme` to the resolved value
   (`'light'` | `'dark'`).
4. CSS applies the matching token values; the whole UI renders in that theme.
5. User selects a mode via the header control → `setMode(next)`; provider persists `mode` to
   `localStorage` and re-resolves/re-applies `data-theme`.
6. While `mode === 'system'`, a `matchMedia` change listener re-resolves on OS theme changes;
   the listener is removed when `mode` is not `system` (cleanup in `useEffect`).

### Location of new files

```txt
web/src/
  context/
    ThemeContext.tsx        # ThemeProvider + useTheme hook (new)
  styles/
    tokens.css              # extend with dark token values (modify)
  components/BoardHeader/
    ThemeControl.tsx        # theme cycle/select control, if non-trivial (new, optional)
```

Do not create new folders where an equivalent convention already exists.

---

## 6. Files to create or modify

The list must be exact. Do not modify files outside this section without justifying it.

| Path | Action | Purpose | Project example to follow |
|---|---|---|---|
| `web/src/context/ThemeContext.tsx` | NEW | `ThemeProvider` + `useTheme` (mode state, localStorage, `data-theme`, system tracking) | `web/src/api/useBoard.ts` (custom hook structure) |
| `web/src/components/BoardHeader/ThemeControl.tsx` | NEW (if non-trivial — see below) | The Light/Dark/System control rendered inside the header | existing `BoardHeader/*` component files |
| `web/src/styles/tokens.css` | MODIFY | Add Dracula dark values for every token via `:root[data-theme="dark"]` + `@media (prefers-color-scheme: dark)`; add `:root[data-theme="light"]`; add new tokens for the extracted hardcoded hex; add the gated color-transition | current `:root` block in the same file |
| `web/index.html` | MODIFY | Add a tiny inline boot `<script>` in `<head>` that reads `localStorage['vector-theme']` and sets `data-theme` **before** the module script — the only way to prevent a theme flash in this CSR app | current `<head>` in same file |
| `web/src/main.tsx` | MODIFY | Wrap `<App />` with `<ThemeProvider>` | current root render in same file |
| `web/src/components/BoardHeader/BoardHeader.tsx` | MODIFY | Render the theme control (consume `useTheme`) | same file |
| `web/src/components/SpecCard/SpecCard.module.css` | MODIFY | Replace inlined `#eef2ff` with a new accent-bg token | uses of `var(--token)` in same file |
| `web/src/components/SpecCard/CardNextCommand.module.css` | MODIFY | Replace inlined `#eef2ff`, `#047857`, `#d1fae5` with tokens | same |
| `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css` | MODIFY | Replace inlined `#eef2ff` + success greens with tokens | same |
| `web/src/components/BoardHeader/BoardHeader.module.css` | MODIFY | Replace inlined connection-dot hex (`#10b981`, `#f59e0b`, `#ef4444`) with tokens | same |
| `web/src/context/themeResolve.ts` | NEW | Pure helpers extracted for testing: `resolveTheme(mode, prefersDark)` and `parseStoredMode(raw)` | `web/src/components/SpecDetailsDrawer/entries.ts` (pure logic module) |
| `web/src/context/themeResolve.test.ts` | NEW | Vitest unit test of the pure helpers (no DOM, no React) | `web/src/components/SpecDetailsDrawer/entries.test.ts` |

> Note: the exact set of `*.module.css` files carrying hardcoded hex must be confirmed at
> implementation time with `grep -rE '#[0-9a-fA-F]{6}' web/src/components` — the rows above are
> the values surfaced during spec authoring. Any additional inlined color discovered there is
> in scope to tokenize.

### Detail per file

#### `web/src/context/ThemeContext.tsx`

Action: NEW.

Must implement:

- `export type ThemeMode = 'light' | 'dark' | 'system'`.
- `useTheme(): { mode: ThemeMode; resolved: 'light' | 'dark'; setMode(m: ThemeMode): void }`.
- `ThemeProvider`: reads/validates `localStorage['vector-theme']` (via `parseStoredMode`), resolves
  the effective theme (via `resolveTheme`), writes `document.documentElement.dataset.theme`,
  persists on change, and subscribes to `matchMedia` only while `mode === 'system'` (with
  `useEffect` cleanup).
- Guard `window`/`localStorage`/`matchMedia` access (defensive; Vite is CSR but storage can throw
  in private modes).
- The two side-effect-free decisions live in `web/src/context/themeResolve.ts` so they are unit
  testable without a DOM: `resolveTheme(mode: ThemeMode, prefersDark: boolean): 'light' | 'dark'`
  and `parseStoredMode(raw: string | null): ThemeMode` (anything outside the enum → `'system'`).

Must follow as reference: `web/src/api/useBoard.ts`, `web/src/main.tsx`,
`web/src/components/SpecDetailsDrawer/entries.ts` (pure-helper extraction pattern).

Must not include: inline styles or component-specific logic; it only sets the attribute and
exposes state.

> **"Non-trivial" definition for `ThemeControl.tsx`**: extract it to its own file the moment the
> control needs more than a single `<button>` with one inline click handler — i.e. as soon as it
> renders a menu, multiple buttons, or holds local open/close state. A bare cycle button can stay
> inline in `BoardHeader.tsx`.

#### `web/src/styles/tokens.css`

Action: MODIFY.

Required changes:

- Keep the existing `:root { … }` as the light default.
- Add `:root[data-theme="dark"] { … }` and an equivalent `@media (prefers-color-scheme: dark) { :root:not([data-theme]) { … } }`
  block (so the OS preference only drives the UI when no explicit theme is set), both assigning
  the Dracula dark values from the **Dracula → token mapping** table below.
- Add `:root[data-theme="light"] { … }` pinning the light values (explicit Light beats a dark OS).
- Add the new tokens that replace the extracted hardcoded hex (e.g. `--color-accent-bg`,
  `--conn-live`, `--conn-reconnecting`, `--conn-error`, `--color-success-fg`, `--color-success-bg`),
  with both light and dark values.
- Add a color-only transition on themable surfaces, gated by
  `@media (prefers-reduced-motion: reduce) { * { transition: none } }` (~150ms).

Constraints:

- Do not rename existing variables — only add dark values and the new extracted tokens.
- Do not duplicate light values into the dark block where they are identical without reason.
- Dark values must meet WCAG AA (Section 8); tune secondary/tertiary text and pill backgrounds
  as needed (pure Dracula `comment #6272a4` may be too low-contrast for secondary text and must
  be lightened to pass).

##### Dracula → token mapping

Anchor palette (Dracula official): `bg #282a36`, `bg-darker #21222c`, `current-line #44475a`,
`foreground #f8f8f2`, `comment #6272a4`, `cyan #8be9fd`, `green #50fa7b`, `orange #ffb86c`,
`pink #ff79c6`, `purple #bd93f9`, `red #ff5555`, `yellow #f1fa8c`.

Every existing `:root` token keeps its **name**; only its value changes per theme. The dark
values below are the **proposed anchors** — those marked *(tune for AA)* must be verified/adjusted
with a contrast checker before merge (Section 8); they are starting points, not final-by-decree.

| Token | Light (current, unchanged) | Dark (Dracula) |
|---|---|---|
| `--color-app-bg` | `#f7f8fa` | `#21222c` |
| `--color-surface` | `#ffffff` | `#282a36` |
| `--color-surface-muted` | `#f3f4f6` | `#343746` |
| `--color-border` | `#e5e7eb` | `#44475a` |
| `--color-border-strong` | `#d1d5db` | `#6272a4` |
| `--color-text` | `#111827` | `#f8f8f2` |
| `--color-text-secondary` | `#6b7280` | `#aeb8d4` *(tune for AA — raw `#6272a4` fails)* |
| `--color-text-tertiary` | `#9ca3af` | `#8893b8` *(tune for AA)* |
| `--color-accent` | `#6366f1` | `#bd93f9` |
| `--gradient-brand` | `linear-gradient(120deg,#f472b6,#a855f7,#6366f1)` | `linear-gradient(120deg,#ff79c6,#bd93f9,#8be9fd)` |
| `--status-draft-fg` / `--status-draft-bg` | `#475569` / `#f1f5f9` | `#c8cee0` / `#343746` |
| `--status-open-fg` / `--status-open-bg` | `#475569` / `#eef2f7` | `#c8cee0` / `#343746` |
| `--status-progress-fg` / `--status-progress-bg` | `#b45309` / `#fef3c7` | `#ffb86c` / `#3b3324` *(tune for AA)* |
| `--status-attention-fg` / `--status-attention-bg` | `#b91c1c` / `#fee2e2` | `#ff6e6e` / `#3b2630` *(tune for AA)* |
| `--status-review-fg` / `--status-review-bg` | `#7c3aed` / `#ede9fe` | `#bd93f9` / `#312a46` *(tune for AA)* |
| `--status-closed-fg` / `--status-closed-bg` | `#047857` / `#d1fae5` | `#50fa7b` / `#1f3a2c` *(tune for AA)* |
| `--priority-urgent` | `#dc2626` | `#ff5555` |
| `--priority-high` | `#ea580c` | `#ffb86c` |
| `--priority-normal` | `#6b7280` | `#8893b8` |
| `--priority-low` | `#9ca3af` | `#6272a4` |
| `--savings-fg` / `--savings-bg` | `#047857` / `#ecfdf5` | `#50fa7b` / `#1f3a2c` *(tune for AA)* |

New tokens (extracted from the hardcoded hex in Section 6), each needing both values:

| New token | Replaces (current hardcoded) | Light | Dark (Dracula) |
|---|---|---|---|
| `--color-accent-bg` | `#eef2ff` | `#eef2ff` | `#312a46` |
| `--conn-live` | `#10b981` | `#10b981` | `#50fa7b` |
| `--conn-reconnecting` | `#f59e0b` | `#f59e0b` | `#ffb86c` |
| `--conn-error` | `#ef4444` | `#ef4444` | `#ff5555` |
| `--color-success-fg` | `#047857` | `#047857` | `#50fa7b` |
| `--color-success-bg` | `#d1fae5` | `#d1fae5` | `#1f3a2c` |

#### `web/index.html`

Action: MODIFY.

Required changes:

- Add a small inline `<script>` in `<head>`, **before** `<script type="module" src="/src/main.tsx">`,
  that reads `localStorage['vector-theme']`, resolves it (`system`/absent →
  `matchMedia('(prefers-color-scheme: dark)').matches`), and sets
  `document.documentElement.setAttribute('data-theme', resolved)`. This runs before first paint and
  is the only reliable way to prevent a flash of the wrong theme in this CSR app.

Constraints:

- Keep the logic minimal and dependency-free (plain JS, wrapped in `try/catch`); it mirrors —
  does not replace — the provider's resolution logic.
- Note the two scripts live in **different elements**: the new boot script goes in `<head>`; the
  existing `<script type="module" src="/src/main.tsx">` stays in `<body>`. The boot script only
  needs to run before the module loads.

#### `web/src/main.tsx`

Action: MODIFY. Wrap `<App />` with `<ThemeProvider>` (inside or around `StrictMode`); keep the
existing root-element guard and imports.

#### `web/src/components/BoardHeader/BoardHeader.tsx`

Action: MODIFY. Render the theme control (own `ThemeControl.tsx` if non-trivial) consuming
`useTheme`; place it on the header's right side near the connection status without changing the
existing layout/behavior.

#### `*.module.css` (SpecCard, CardNextCommand, SpecDetailsDrawer, BoardHeader)

Action: MODIFY. Replace each hardcoded hex listed above with the corresponding new `var(--token)`.
Do not change layout or any non-color rule.

---

## 7. API Contract

No aplica — this phase adds **no** endpoints and changes **no** existing ones. Theme state lives
in the browser (`localStorage` + a DOM attribute) and is never sent to the CLI/API. The board's
existing data contract (`/api/board`, SSE) is untouched.

---

## 8. Success criteria

The implementation is correct when:

- [ ] On first load with no stored preference, the UI follows the OS (`System` mode):
      `prefers-color-scheme: dark` → dark, otherwise light.
- [ ] The header control switches between Light, Dark, and System; the entire web UI updates.
- [ ] `System` mode tracks live OS theme changes (toggle the OS while the page is open → UI follows).
- [ ] The selected mode persists across reloads (`localStorage['vector-theme']`).
- [ ] An explicit Light choice stays light even on a dark OS, and vice-versa.
- [ ] Every surface adapts: board, header, columns, cards, status pills, priority flags, details
      drawer, standup view, token-savings meter — no element stays in the wrong theme.
- [ ] Text and interactive elements meet **WCAG AA** in both themes (≥ 4.5:1 text, ≥ 3:1 UI).
- [ ] Theme change shows a ~150ms color fade, disabled under `prefers-reduced-motion: reduce`.
- [ ] No regression: all board behavior is identical in both themes (visual-only change).
- [ ] TypeScript has no errors; the web build succeeds (required for the binary embed).

### Required tests

The project's Vitest setup tests **pure logic only** — no DOM environment (`jsdom`/`happy-dom`)
and no React Testing Library are installed, and this phase adds **no** dependencies. Therefore:

- [ ] **Automated (Vitest, pure):** `themeResolve.test.ts` covers `resolveTheme` (every `mode` ×
      `prefersDark` combination → correct `'light'|'dark'`; `system` follows `prefersDark`; explicit
      modes ignore it) and `parseStoredMode` (valid values pass through; `null`/garbage → `'system'`).
- [ ] **Manual QA (documented, not automated):** the `ThemeProvider` DOM side effects
      (`data-theme` on `document.documentElement`, `localStorage` persistence, live `matchMedia`
      tracking while `system`) are verified by hand in the browser — they require a DOM environment
      this phase deliberately does not add.
- [ ] **Manual contrast check** in the dark theme with a contrast checker (WebAIM / axe) for, at
      minimum: `--color-text` on `--color-app-bg`; `--color-text` on `--color-surface`;
      `--color-text-secondary` on `--color-surface`; `--color-text-tertiary` on `--color-surface`;
      each status pill (`--status-*-fg` on its `--status-*-bg`, all six); each `--priority-*` on
      `--color-surface`; and `--savings-fg` on `--savings-bg`.
- [ ] **Manual visual QA** of both themes across all listed surfaces.

### Verification commands

```bash
npm --prefix web run typecheck
npm --prefix web run test
npm --prefix web run build
# then visually verify both themes + the control via `vector serve`
```

The phase is not complete if any of these fail.

---

## 9. UX criteria

### Initial load

- With no stored preference the UI silently applies the OS theme (no prompt/modal).
- The theme is applied **before first paint** by the inline boot `<script>` in `web/index.html`
  (Section 6), which sets `data-theme` before the module bundle loads. The `ThemeProvider` then
  takes over on mount as the source of truth. Setting `data-theme` only inside the provider would
  **not** prevent the flash in this CSR app (React mounts after first paint), so the inline script
  is required — not optional.

### Theme control

- Location: right side of `BoardHeader`, near the connection status badge.
- Represents the **current mode** with a `lucide-react` icon: `Sun` (Light), `Moon` (Dark),
  `Monitor` (System).
- Interaction: activating it advances Light → Dark → System → Light (a clear cycle), or opens a
  small menu of the three — either is acceptable as long as the active mode is visible and the
  control is keyboard-operable.
- Accessible name: `aria-label`/`title` reflects current mode, e.g. "Theme: System (click to change)".
- Visible `:focus` ring.

### Transition

- A subtle ~150ms color transition on themable surfaces; **no** layout/size animation.
- Respect `prefers-reduced-motion: reduce` → no transition.

### Scope / consistency

- All surfaces switch together; no element is left in the previous theme.

### Accessibility

- Icons inherit text color and remain visible in both themes.
- Contrast respects WCAG AA (≥ 4.5:1 body text; ≥ 3:1 components).

---

## 10. Decisions made

These are decided; the agent must not question or change them:

- **Tri-state Light / Dark / System** control; `System` tracks the OS live (`matchMedia`) — gives
  the developer an explicit override while still honoring the OS by default.
- **Whole board web UI** is in scope via the single shared token set (not board-only) — partial
  theming would leave inconsistent light/dark surfaces; one token set is also the cheaper path.
- **Dracula** is the named dark palette source — a well-documented, license-friendly palette
  familiar from developer/terminal tooling, fitting Vector's developer-focused audience.
- **WCAG AA** is the contrast target — the standard minimum bar; AAA would rule out several
  Dracula foreground colors and is not required for this tool.
- **~150ms color fade** on theme change, honoring `prefers-reduced-motion` — smooths the switch
  without animating layout, and respects users who opt out of motion.
- **React Context + CSS custom properties**; no theming library, no new dependency — the token
  system already exists, so a built-in Context is sufficient and keeps the embedded bundle light.
- **`localStorage` (`vector-theme`)** for persistence; never the state JSON, never the server — a
  theme is a per-browser personal preference, not shared board state.
- **`data-theme` attribute on `<html>`** as the activation mechanism — standard, CSS-Modules-safe
  override hook; existing variable names are preserved, only their values change per theme.

If the agent finds a seemingly better alternative, report it as an observation; do not implement it.

---

## 11. Edge cases

### No stored preference

- Resolve via `prefers-color-scheme`; if the OS reports no preference, default to light.

### OS theme changes while the page is open

- If `mode === 'system'`, the UI follows live (via the `matchMedia` listener). If an explicit
  Light/Dark is set, the OS change is ignored (the explicit choice wins).

### `localStorage` unavailable or throws (private mode, quota)

- Theme still works in memory for the session; reads/writes are wrapped in `try/catch`; on read
  failure fall back to `system`. No visible error.

### Invalid stored value

- Any value not in `{light,dark,system}` is treated as `system`.

### Dracula contrast for low-emphasis text

- Pure Dracula `comment #6272a4` as secondary/tertiary text on `#282a36` may fail AA; lighten the
  secondary/tertiary text tokens until they pass (≥ 4.5:1). Tune, then verify.

### Brand gradient / accent backgrounds in dark

- `--gradient-brand` and the extracted accent background get a Dracula-tinted dark value
  (pink → purple → cyan family) so text over them keeps AA contrast.

### Reduced motion

- With `prefers-reduced-motion: reduce`, the ~150ms transition is disabled.

### Flash of wrong theme on load

- Prevented by the inline boot `<script>` in `web/index.html` (Section 6 / Section 9) that sets
  `data-theme` before first paint. Provider-only application would not prevent it in CSR.

### Browsers

- `prefers-color-scheme`, CSS custom properties, and `matchMedia` are supported by the project's
  modern target (Chrome/Firefox/Safari/Edge current). No legacy (IE11) support is required.

---

## 12. Required UI states

| State | What is shown | What the user can do |
|---|---|---|
| initial load | resolved theme (OS or stored) applied | use the board normally |
| control — Light active | `Sun` icon, label "Theme: Light" | activate to go Dark |
| control — Dark active | `Moon` icon, label "Theme: Dark" | activate to go System |
| control — System active | `Monitor` icon, label "Theme: System" | activate to go Light |
| light theme active | light surfaces / dark text / light pills | use the board (no behavior change) |
| dark theme active | Dracula surfaces / light text / dark-tinted pills | use the board (no behavior change) |
| transitioning | ~150ms color fade (unless reduced-motion) | non-blocking |

No `empty` / `offline` / `disabled` states apply to the theme control itself.

---

## 13. Validations

No aplica — there are no forms or user-entered values. The control selects among three fixed,
always-valid modes; the stored value is validated (Section 11) but there is nothing to validate
on input beyond constraining it to the enum.

---

## 14. Security and permissions

- `localStorage` holds only `vector-theme` (`light|dark|system`) — no secrets, tokens, or PII.
- No new network calls; nothing sensitive logged.
- No role/permission gating (theme is a personal UI preference).

---

## 15. Observability and logging

No aplica — the theme change is a local UI preference with no domain event. Use the project's
existing logging only; do not add logging for theme switches in this phase. No tokens/PII logged.

---

## 16. i18n / visible text

Vector's web UI ships English text and has no i18n system. The only added text is the control's
accessible name.

| Key (conceptual) | Text |
|---|---|
| theme.control.aria | "Theme: <Light\|Dark\|System> (click to change)" |
| theme.icon | `Sun` / `Moon` / `Monitor` (lucide-react) |

Do not introduce an i18n framework; keep the label inline as the rest of the header does.

---

## 17. Performance

- The provider holds a tiny state (`mode`); theme changes are rare and cheap (set an attribute,
  write `localStorage`). A full re-render only on toggle is acceptable.
- CSS cost is additional token blocks only — negligible bundle/runtime impact; no new dependency.
- The `matchMedia` listener exists only while `mode === 'system'` and is cleaned up otherwise.
- Keep `useTheme` consumers minimal so context updates don't re-render unrelated trees
  unnecessarily; the control and the provider are the only consumers needed.

---

## 18. Restrictions

The agent must not:

- Add any theming dependency (`next-themes`, MUI, Tailwind, CSS-in-JS) or any new dependency.
- Make `prefers-color-scheme` the only mechanism — the manual Light/Dark/System control is required.
- Rename existing CSS variables or refactor `tokens.css` to another format (SCSS/PostCSS/Tailwind).
- Touch CLI/API logic or write the theme to the state JSON.
- Change any board behavior, layout, or non-color styling.
- Invent dark colors without verifying WCAG AA contrast.
- Ignore lint/typecheck/test/build failures.

---

## 19. Deliverables

On completion:

- [ ] `ThemeContext.tsx` (provider + `useTheme`, with system tracking + persistence).
- [ ] `tokens.css` extended with Dracula dark values, `data-theme` overrides, extracted tokens,
      and the reduced-motion-gated transition.
- [ ] `main.tsx` wrapping `<App />` with `<ThemeProvider>`.
- [ ] `BoardHeader` rendering the Light/Dark/System control (own component file if non-trivial).
- [ ] Hardcoded hex values tokenized across the listed `*.module.css` files.
- [ ] `themeResolve.test.ts` (pure) green; both themes verified visually and for AA contrast.
- [ ] Typecheck clean, tests green, web build successful.
- [ ] Go binary rebuilt and reinstalled to `~/.local/bin/vector` after the `web/` change
      (dogfooding uses the PATH binary, which embeds the built `web/dist`).

---

## 20. Final checklist for the agent

Before delivering, verify:

- [ ] I read this whole spec.
- [ ] No API contract changed (Section 7).
- [ ] All prior dependencies confirmed to exist.
- [ ] Only listed files modified, or any exception justified.
- [ ] Followed real project examples (`useBoard.ts`, existing `*.module.css`, token usage).
- [ ] Implemented all required UI states and the tri-state control.
- [ ] Handled all edge cases (no preference, OS change, storage failure, invalid value,
      reduced motion, low-contrast Dracula text).
- [ ] Added no unauthorized dependencies.
- [ ] Did not change the decisions in Section 10.
- [ ] Ran typecheck / tests / build (`npm --prefix web run typecheck|test|build` — no `lint` script exists).
- [ ] Verified WCAG AA contrast for the enumerated pairs (Section 8) in both themes.
- [ ] Rebuilt and reinstalled the `vector` binary (web change → re-embed `web/dist`).
- [ ] No leftover debug logs or unjustified TODOs.
