# Tasks — add-board-command-palette

## 1. Pure matching logic

- [x] 1.1 `web/src/components/CommandPalette/matchCards.ts`: new pure module.
      `export function matchCards(cards: Card[], query: string, priorities: Priority[]): Card[]`.
      Normalize the query with `.trim().toLowerCase()`; an empty query filters nothing by text. Per
      card build `` `${card.title} ${card.id} ${card.priority} ${card.status}`.toLowerCase() `` and
      test with `haystack.includes(normalized)` — **no `RegExp`**. When `priorities.length > 0`,
      additionally require `priorities.includes(card.priority)`. Preserve input order; no relevance
      sorting. No React import. Reference: `web/src/components/SpecCard/relationChips.ts`.
- [x] 1.2 `web/src/components/CommandPalette/matchCards.test.ts`: Vitest unit test. Cover
      case-insensitive substring by title, id, priority-as-text, and status-as-text; regex
      metacharacters (`.`, `*`, `(`, `)`, `[`, `\`) treated as literal text without throwing; empty
      / whitespace-only query returns every card; multi-select priority filter with 0, 1, and 2+
      active priorities; input order preserved. Reference:
      `web/src/components/SpecCard/relationChips.test.ts`.

## 2. Trigger hook

- [x] 2.1 `web/src/lib/useCommandPaletteTrigger.ts`: new standalone hook returning
      `{ isOpen: boolean; open: () => void; close: () => void }`. `open()` stores
      `document.activeElement` in a `useRef<HTMLElement | null>` and sets `isOpen = true`;
      `close()` sets `isOpen = false` and returns focus to the stored element if it is still
      connected to the DOM. One `useEffect` registers a `window` `keydown` listener (with cleanup)
      that ignores everything except `/`, ignores it when the event target / `document.activeElement`
      is an `INPUT`, `TEXTAREA`, or has `isContentEditable === true`, and otherwise calls
      `event.preventDefault()` + `open()`. No filtering or rendering logic here. Reference:
      `web/src/lib/useNow.ts`.

## 3. Command palette component tree

- [x] 3.1 `web/src/components/CommandPalette/index.tsx`: new component.
      Props `{ cards: Card[]; onSelectCard: (card: Card) => void; onClose: () => void }`. Local
      state `query: string` (`''`), `priorities: Priority[]` (`[]`), `highlightedIndex: number`
      (`0`) — reset comes from conditional mount, add no explicit reset effect. Compute
      `const results = useMemo(() => matchCards(cards, query, priorities), [cards, query, priorities])`
      and clamp `highlightedIndex` to `Math.min(highlightedIndex, Math.max(results.length - 1, 0))`.
      Outer `<div className={styles.overlay} onClick={onClose}>` wrapping an inner panel with
      `onClick={(event) => event.stopPropagation()}`, `role="dialog"`, `aria-modal="true"`,
      `aria-label="Command palette"`. Reference:
      `web/src/components/SpecDetailsDrawer/index.tsx:39-58`. No `fetch`, no config reads, no
      `view` changes, no `dangerouslySetInnerHTML`, no `<form>`.
- [x] 3.2 Search input inside `index.tsx`: `<input ref={inputRef}>` controlled by `query`, focused
      on mount via `useEffect` + `inputRef.current?.focus()`, with `role="combobox"`,
      `aria-expanded={results.length > 0}`, `aria-controls="command-palette-listbox"`,
      ``aria-activedescendant={results[highlightedIndex] ? `palette-option-${results[highlightedIndex].id}` : undefined}``,
      and `placeholder="Search specs by title, id, priority or status…"`.
- [x] 3.3 Result list inside `index.tsx`: `<ul role="listbox" id="command-palette-listbox">` with one
      `<PaletteResultRow>` per result (``id={`palette-option-${card.id}`}``,
      `highlighted={index === highlightedIndex}`, `onSelect={handleSelect}`). Add an always-visible
      counter node with `aria-live="polite"` and `aria-atomic="true"` showing `<n> results` (`1 result`
      singular), collapsing to the exact copy `No results` when `results.length === 0`. Use that same
      node for the empty state so screen readers are not announced twice.
- [x] 3.4 Keyboard handling in `index.tsx` via `onKeyDown` on the dialog panel: `ArrowDown`/`ArrowUp`
      move `highlightedIndex` clamped without wraparound (`preventDefault`); `Enter` calls
      `handleSelect(results[highlightedIndex])` when `results.length > 0` (`preventDefault`);
      `Escape` calls `onClose()` **and** `event.stopPropagation()` so the drawer's `window` listener
      never sees it; `Tab`/`Shift+Tab` `preventDefault()` (trivial trap — the input is the only
      focusable control). `handleSelect(card)` calls `onSelectCard(card)` then `onClose()`.
- [x] 3.5 `web/src/components/CommandPalette/PaletteResultRow.tsx`: new component. Props
      `{ id: string; card: Card; highlighted: boolean; onSelect: (card: Card) => void }`. Renders
      `<li id={id} role="option" aria-selected={highlighted} className={highlighted ? styles.rowHighlighted : styles.row} onClick={() => onSelect(card)}>`
      containing the title, the id as plain text (**not** `CopyableSlug`),
      `<StatusPill status={card.status} />`, and `<PriorityFlag priority={card.priority} />`.
      Purely presentational. Reference: `web/src/components/SpecCard/SpecCard.tsx:77,99`.
- [x] 3.6 `web/src/components/CommandPalette/PalettePriorityFilter.tsx`: new component. Props
      `{ selected: Priority[]; onToggle: (priority: Priority) => void }`. Four
      `<button type="button">` chips (`urgent`/`high`/`normal`/`low`) with
      `aria-pressed={selected.includes(priority)}`, active/inactive styling, and
      `onClick={() => onToggle(priority)}`. Multi-select: 0–4 active; 0 active means no priority
      filter. No status or other filter axis. Reference:
      `web/src/components/PriorityFlag/PriorityFlag.tsx`.
- [x] 3.7 `web/src/components/CommandPalette/CommandPalette.module.css`: new stylesheet.
      `.overlay` (full-screen semi-transparent backdrop), `.palette` (centered panel — not a side
      drawer — using `var(--color-surface)`, `var(--radius-card)`, `var(--shadow-raised)`),
      `.input`, `.priorityFilter`, `.results`, `.row`, `.rowHighlighted`, `.empty`. Use only
      existing variables from `web/src/styles/tokens.css`; define no new colors and no new media
      queries. Reference: `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css`.

## 4. Header trigger

- [x] 4.1 `web/src/components/BoardHeader/PaletteTrigger.tsx`: new component. Props
      `{ onOpen: () => void }`. A `<button type="button">` with the `Search` icon from
      `lucide-react` (`size={16} strokeWidth={2}`, matching `ThemeControl`), `onClick={onOpen}`,
      `aria-label="Open command palette"`, `title="Search specs (/)"`. Imports the shared
      `BoardHeader.module.css`. No open/close logic. Reference:
      `web/src/components/BoardHeader/ThemeControl.tsx`.
- [x] 4.2 `web/src/components/BoardHeader/BoardHeader.module.css`: add a `.paletteTrigger` class
      with the same visual chrome as `.themeControl` (no background, centered icon,
      `var(--radius-button)`, consistent `:hover` / `:focus-visible`). **Do not** modify
      `.themeControl` or any other existing rule.
- [x] 4.3 `web/src/components/BoardHeader/BoardHeader.tsx`: add `onOpenPalette: () => void` to
      `BoardHeaderProps`; render `<PaletteTrigger onOpen={onOpenPalette} />` inside
      `<div className={styles.actions}>` **before** `<ThemeControl />` (order: connection status →
      palette trigger → theme control). Do not change the existing status markup / `CONNECTION_LABEL`
      or rename `styles.actions`.

## 5. State elevation and wiring

- [x] 5.1 `web/src/components/KanbanBoard/KanbanBoard.tsx`: remove the local
      `useState<Card | null>` and the `<SpecDetailsDrawer>` render, plus the now-unused
      `SpecDetailsDrawer` import. Add `onSelectCard: (card: Card) => void` to `KanbanBoardProps` and
      pass it straight through to `<BoardColumn>`. Update the architecture comment so it stays
      honest: selection and the drawer now live in `App`; `KanbanBoard` remains a read-only
      projection of `columns`. Do not change `BoardColumn`'s signature or add any other local state.
- [x] 5.2 `web/src/App.tsx`: add `const [selectedCard, setSelectedCard] = useState<Card | null>(null)`
      (import `Card` from `./types/board`) and
      `const { isOpen: paletteOpen, open: openPalette, close: closePalette } = useCommandPaletteTrigger()`
      — both declared **before** the existing loading/error early return, per the rules of hooks.
      After that return, derive `const cards = board.columns.flatMap((column) => column.cards)`.
      Pass `onOpenPalette={openPalette}` to `<BoardHeader>` and `onSelectCard={setSelectedCard}` to
      `<KanbanBoard>`. Render, **outside** the `view` conditional and exactly once each:
      `{paletteOpen && <CommandPalette cards={cards} onSelectCard={setSelectedCard} onClose={closePalette} />}`
      and
      `{selectedCard && <SpecDetailsDrawer card={selectedCard} onClose={() => setSelectedCard(null)} />}`.
      Do not touch the `view` state or its tab logic.

## 6. Component and integration tests

- [x] 6.1 `web/src/components/CommandPalette/index.test.tsx`: input focused on mount; filtering by
      title, id, priority-as-text, and status-as-text (case-insensitive); priority chips refine the
      list; ArrowDown/ArrowUp move the highlight and Enter selects it, calling `onSelectCard` **and**
      `onClose`; Escape calls `onClose`; overlay click closes while a click inside the panel does
      not; the empty state renders `No results` without crashing. Reference:
      `web/src/components/SpecDetailsDrawer/index.test.tsx` (`render`/`screen`/`afterEach(cleanup)`,
      `makeCard` helper).
- [x] 6.2 `web/src/components/KanbanBoard/KanbanBoard.test.tsx`: `KanbanBoard` no longer renders
      `SpecDetailsDrawer` and owns no selection state; clicking a card invokes the `onSelectCard`
      prop with the matching `Card`. Reference:
      `web/src/components/BoardColumn/BoardColumn.test.tsx`.
- [x] 6.3 `web/src/App.test.tsx`: pressing `/` with focus outside any input opens the palette while
      `view` is `'standup'` and while it is `'tokens'`; selecting a result opens
      `SpecDetailsDrawer` **without** changing the active tab; pressing `/` with focus inside an
      input (including the palette's own input) neither reopens nor interferes. `SpecDetailsDrawer`
      calls `useSpecSummary` unconditionally on mount (`SpecDetailsDrawer/index.tsx:35`), so it
      **must** be mocked as in `SpecDetailsDrawer/index.test.tsx:9`.
- [x] 6.4 Escape precedence regression test: with the drawer already open behind the palette, one
      Escape closes only the palette and a second closes the drawer — never both on one press.

## 7. Verification

- [x] 7.1 `npm --prefix web run typecheck` clean (no `any`, no TS errors).
- [x] 7.2 `npm --prefix web test` green (new and existing tests).
- [x] 7.3 `npm --prefix web run build` succeeds (required for the binary embed). Note:
      `web/package.json` has **no** `lint` script — do not invent one.
- [x] 7.4 Manual QA via `vector serve`: both triggers open the palette; `/` is ignored while focus
      is in an input; search and priority chips behave per spec; arrows + Enter and click both jump
      to the drawer; jump-to from `standup`/`tokens` leaves the tab unchanged; the board behind is
      never filtered; focus returns to the opener on close; reopening starts empty.
- [x] 7.5 Re-embed the frontend and reinstall the binary (mandatory for any `web/` change, per
      `architecture/distribution-packaging.md`): `npm --prefix web run build`;
      `rm -rf cli/internal/webui/dist/assets cli/internal/webui/dist/index.html`;
      `cp -R web/dist/. cli/internal/webui/dist/`;
      `go -C cli build -o ~/.local/bin/vector ./cmd/vector`; then restart any running `vector serve`.
