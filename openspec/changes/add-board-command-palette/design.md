# Design — add-board-command-palette

## Context

The board (`web/`) is a React 19.1 + Vite 6 + TypeScript 5.7 CSR app with **no component library**
— CSS Modules over the CSS custom properties in `web/src/styles/tokens.css`. There is no external
state manager: state is React local state plus small standalone hooks in `web/src/lib/`
(`useNow.ts` is the precedent). Data arrives as a read-only SSE projection of CLI state through
`useBoard()` (`web/src/api/useBoard.ts`), which exposes `{ board, connection, error }`.

Unlike earlier phases, the test environment **is** DOM-capable: Vitest 4.1.9 with `happy-dom` and
`@testing-library/react` 16.1.0 are already installed, so component interaction can be tested
directly (pattern in `SpecDetailsDrawer/index.test.tsx`).

Today `App.tsx` holds `view: 'board' | 'standup' | 'tokens'` and renders each view conditionally,
while selection state lives one level down inside `KanbanBoard` — the structural reason the drawer
is unreachable from `standup` and `tokens`.

## Goals / Non-Goals

**Goals:**
- Reach any spec's details drawer in two keystrokes (`/` + typing) from **all three** views.
- Search over title, id/slug, priority-as-text, and status-as-text, case-insensitive, literal
  substring, refined by a multi-select priority filter local to the palette.
- Full keyboard operation and a correct combobox/listbox accessibility contract.
- Zero new dependencies, zero network calls, zero writes.

**Non-Goals:**
- Filtering or otherwise altering the board behind the palette — the palette only *opens* specs.
- A status filter inside the palette (board columns already are the status axis).
- Fuzzy search, user regex, or relevance scoring.
- A `Cmd+K` binding, persistence of palette state, or searching `/vector:*` commands (a possible
  later phase, explicitly out of this one).
- Any change to `cli/`'s HTTP API, the SSE stream, or the `Board`/`Column`/`Card` shape.

## Decisions

- **State elevation over a new context.** `selectedCard` and the palette's open state move to
  `App`, the only common ancestor of `BoardHeader`, `KanbanBoard`, `StandupView`, and
  `TokenBreakdownView`. Both the palette and the drawer render **once each, outside** the `view`
  conditional. Chosen over a React Context because the project uses no state manager for this kind
  of UI state and two props on existing components are cheaper than a new provider.
- **Conditional mount as the reset mechanism.** The palette is mounted/unmounted rather than hidden
  with CSS, so query, chips, and highlighted index reset on every open with no explicit reset
  effect. This is the simplest correct option and removes a whole class of stale-state bugs.
- **Pure `matchCards.ts` colocated with the component**, mirroring `SpecCard/relationChips.ts` — a
  plain `.ts` helper with a sibling unit test, no React import. All branching logic lives here,
  where it is trivially testable; the component keeps only wiring.
- **`.includes()`, never `RegExp`.** The haystack is
  `` `${card.title} ${card.id} ${card.priority} ${card.status}`.toLowerCase() ``. Building a
  `RegExp` from user input would both throw on unbalanced metacharacters and silently turn `.` into
  a wildcard; literal substring is what a spec-name search actually wants.
- **`/` as the trigger, with a focus guard, in a standalone hook.** `useCommandPaletteTrigger`
  registers one `window` `keydown` listener (with cleanup) and ignores the event when the target is
  an `INPUT` / `TEXTAREA` / `isContentEditable`. This single guard elegantly covers three cases at
  once: typing `/` in any app input, typing `/` inside the palette's own input, and keyboard
  auto-repeat (`/` held down — the first press moves focus into the palette input, so every repeat
  is ignored by the same rule). No `Cmd+K`: not requested.
- **Escape precedence via `stopPropagation` in the palette only.** The drawer listens for Escape on
  `window` without `stopPropagation` (`SpecDetailsDrawer/index.tsx:39-45`). Handling Escape on the
  palette's dialog panel and stopping propagation there means the bubbling event never reaches
  `window`, so one press closes the topmost element (the palette) and a second closes the drawer —
  achieved **without editing `SpecDetailsDrawer`**, keeping the change surface minimal.
- **Combobox/listbox over a bare list.** `role="combobox"` + `aria-expanded` + `aria-controls` on
  the input, `role="listbox"` on the `<ul>`, `role="option"` + `aria-selected` per row, and
  `aria-activedescendant` pointing at the highlighted row's id. Focus stays on the input the whole
  time (roving focus would break typing), which is exactly what this pattern exists for. `Tab` is
  `preventDefault`ed — a trivial trap, since the input is the only focusable control.
- **No `<form>`.** A single controlled `<input>`; Enter is handled in `onKeyDown`, avoiding native
  submit semantics and an accidental page reload.
- **Always-visible result counter in an `aria-live="polite"` / `aria-atomic="true"` node**, reusing
  the same node for the empty state so screen readers get one announcement, not two. Closed copy:
  `<n> results` / `1 result` / `No results`.
- **Index clamping instead of reset-on-change.** `highlightedIndex` is clamped to
  `[0, results.length - 1]`; because the board updates live over SSE, the highlighted row can
  disappear while the palette is open, and clamping keeps `aria-activedescendant` and Enter
  pointing at a real row.
- **No debounce, no virtualization, no `lazy()`.** Filtering is synchronous over an in-memory array
  of tens of specs, wrapped in `useMemo`. The palette tree is plain JSX + CSS with no heavy
  dependency, unlike `MarkdownView` (which is code-split for `react-markdown`).
- **Multi-select priority chips (0–4 active)**; zero active means no priority filter. Multi-select
  avoids reopening the palette to widen a search, and priority is the only filter axis — status is
  deliberately excluded as redundant with the board's columns.

## Risks / Trade-offs

- **Escape ordering depends on bubbling reaching the palette's panel first.** If a future change
  moves the palette's Escape handling to a `window` listener, the `stopPropagation` guarantee
  silently breaks and both overlays would close on one press. Covered by an explicit test.
- **State elevation touches three existing components** (`App`, `KanbanBoard`, `BoardHeader`), so
  regression risk sits in the wiring rather than the new code. Mitigated by
  `KanbanBoard.test.tsx` (delegates via prop, owns no drawer) and `App.test.tsx` (jump-to from
  `standup`/`tokens` without a `view` change).
- **`/` is a plausible keystroke outside inputs** (e.g. a future contenteditable surface). The guard
  covers the current app; any new free-text surface must be a real input/contenteditable for the
  guard to keep holding.
- **Pressing `/` while `board` is still `null`** flips `isOpen` before the palette can render; the
  palette then appears as soon as the board loads. Accepted as-is, not a bug.
- **No relevance ranking** means a short query can surface a long list in board order. Accepted:
  ranking is scope creep, and the priority chips plus a longer query are the intended refinement.

## Migration / Rollout

Pure presentation change; no state, schema, API, or data migration, and no persisted palette state.
Verified by `npm --prefix web run typecheck`, `npm --prefix web test`, and
`npm --prefix web run build` (note: `web/package.json` has **no** `lint` script), then manual QA via
`vector serve`: both triggers, the focus guard, keyboard navigation, Escape precedence with the
drawer open behind, focus restoration, and jump-to from `standup`/`tokens` without a tab change. On
completion, re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild + reinstall the
`vector` binary for dogfooding.
