# Command palette for search and quick jump on the board

## Why

The board web UI (`web/`) has no search and no filter of any kind — confirmed by direct reading of
`web/src`, zero matches. The only way to reach a spec's details today is to spot its card visually
in a `KanbanBoard` column and click it, which stops working entirely once the user switches to the
`standup` or `tokens` view: `selectedCard` and `<SpecDetailsDrawer>` are local state inside
`KanbanBoard` (`web/src/components/KanbanBoard/KanbanBoard.tsx:16,23-25`), so those two views have
no path to a spec detail at all.

A dev administering a board with dozens of specs should be able to reach a specific one in two
keystrokes (`/` + typing) from any view. Everything needed is already in memory: `useBoard()`
delivers `board.columns` over SSE (`web/src/api/useBoard.ts`), so this is a purely client-side,
read-only addition — no new endpoint, no SSE contract change, no new dependency.

## What changes

- **Command palette overlay (web)** — new `web/src/components/CommandPalette/` tree: a modal
  dialog with a search input, multi-select priority chips, a keyboard-navigable result list, and an
  always-visible result counter in an `aria-live="polite"` node. Mounted/unmounted conditionally so
  its local state (query, priority chips, highlighted index) resets on every open with no explicit
  reset logic.
- **Literal substring matching (web)** — `CommandPalette/matchCards.ts`, a pure function
  `matchCards(cards, query, priorities)` matching case-insensitively over a
  `title + id + priority + status` haystack with `.includes()` — **no `RegExp`**, so user-typed
  regex metacharacters (`.`, `*`, `(`, `[`) are literal text. Input order is preserved; no relevance
  scoring.
- **Two triggers (web)** — an icon button in `BoardHeader` (`Search` from the already-present
  `lucide-react`, rendered in `styles.actions` immediately before `<ThemeControl />`), and the `/`
  key captured on `window` by a new standalone hook `web/src/lib/useCommandPaletteTrigger.ts` that
  guards against `INPUT` / `TEXTAREA` / `[contenteditable]` focus and captures/restores the
  previously focused element.
- **State elevation (web)** — `selectedCard` and the `<SpecDetailsDrawer>` render move from
  `KanbanBoard` up to `App`, rendered **outside** the `view` conditional so the palette and the
  drawer work identically from `board`, `standup`, and `tokens`. `KanbanBoard` becomes a pure
  read-only projection that delegates selection through an `onSelectCard` prop.
- **Escape precedence (web)** — the palette's `onKeyDown` calls `event.stopPropagation()` on
  Escape, so one press closes only the palette even when the drawer is open behind it (the drawer's
  own `window` Escape listener, `SpecDetailsDrawer/index.tsx:39-45`, is **not** modified).
- **Accessibility (web)** — `role="dialog"` + `aria-modal`, combobox/listbox pattern
  (`role="combobox"` on the input, `role="listbox"` on the results, `aria-activedescendant`,
  `role="option"` + `aria-selected` per row), autofocus on open, trivial `Tab` trap, focus returned
  to the trigger on close.

## Capabilities

### Added Capabilities
- `board-command-palette`: the board web UI gains an overlay palette that searches specs by title,
  id/slug, priority-as-text, and status-as-text (case-insensitive literal substring), refines by a
  multi-select priority filter, and jumps straight to a spec's details drawer from any of the three
  views — without ever filtering or altering the underlying board.

### Modified Capabilities
- `board-header`: gains the palette trigger button in `styles.actions`, immediately before the
  existing theme control; connection status markup and `.themeControl` are untouched.
- `kanban-board`: no longer owns selection state or renders the details drawer; it receives
  `onSelectCard` as a prop and stays a read-only projection of `columns`.

## Impact

- New: `web/src/lib/useCommandPaletteTrigger.ts`,
  `web/src/components/BoardHeader/PaletteTrigger.tsx`,
  `web/src/components/CommandPalette/{index.tsx,matchCards.ts,PaletteResultRow.tsx,PalettePriorityFilter.tsx,CommandPalette.module.css}`,
  plus tests `CommandPalette/{index.test.tsx,matchCards.test.ts}`,
  `KanbanBoard/KanbanBoard.test.tsx`, `App.test.tsx`.
- Modified: `web/src/App.tsx` (elevated state + wiring), `KanbanBoard/KanbanBoard.tsx` (drops local
  selection state and the drawer), `BoardHeader/BoardHeader.tsx` (new prop + trigger),
  `BoardHeader/BoardHeader.module.css` (adds `.paletteTrigger`; `.themeControl` unchanged).
- **No** new dependencies, **no** CLI/API/SSE/domain changes, **no** state writes — the palette is
  100% read-only over `board.columns` already in memory. `SpecDetailsDrawer/index.tsx` is **not**
  modified.
- After the `web/` change, re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild +
  reinstall the `vector` binary (dogfooding uses the PATH binary).

Authored spec: `.vector/specs/add-board-command-palette/spec.md`.
