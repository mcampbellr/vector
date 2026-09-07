# Fix CommandPalette not finding cards by a linked ticket-key fragment

## Why

The board's `CommandPalette` (search-and-jump overlay, `web/src/components/CommandPalette/`) no
longer surfaces a card whose only match point is a fragment of its linked external ticket key
(e.g. typing `1839` fails to find a card whose badge reads `MH-1839`), even though this is
specified behaviour already covered by test (`matchCards.test.ts:103-112`).

By code reading the whole chain looks correct:

- The haystack already includes the ticket key —
  `` `${card.title} ${card.id} ${card.ticket?.key ?? ''} ${card.priority} ${card.status}` ``
  (`matchCards.ts:21-22`) — with shared diacritic/case folding on query and haystack
  (`matchCards.ts:3-9,17,21`).
- The Go projection `toCard` populates `card.Ticket` from `spec.Ticket` without transformation
  (`cli/internal/board/board.go:241-243`), and `useBoard.ts:26` consumes the SSE payload via a
  direct `JSON.parse` with no intermediate mapping that could drop the field.

Because the static chain reads correct, the most probable cause is Risk #1 from the brief — a
**deployed/embedded web bundle predating the ticket-key search commit `e94d6fb`** — but this is
not confirmed. The regression must be **diagnosed deterministically before touching any file**,
not guessed.

Fixing it lets a dev on the Vector board jump to a card by typing any fragment of its linked
ticket key (numeric, domain prefix, or full key case-insensitive) to reach the spec without
recalling its title or slug.

## What changes

- **Deterministic diagnosis first** (Test Plan of the brief): run the existing suite
  (`cd web && npm test -- matchCards.test.ts`) to decide whether the regression lives in
  `matchCards.ts` logic or is a data/deploy problem at runtime.
- **Fix the single confirmed point of failure**, scoped to one of the §6 candidates the
  diagnosis points at — not all at once:
  - `matchCards.ts` haystack/normalization (only if the suite goes red);
  - `card.ticket` population across `cli/internal/board` → SSE → `useBoard.ts` (only if the key
    arrives empty at runtime despite existing in persisted state);
  - or the **deployed bundle** (Risk #1) — a re-embed + reinstall operation, **not** a source
    edit — if `GET /api/board` serves the key correctly but the rendered UI does not.
- **Regression covered by test**: if the diagnosis reveals a case not covered by
  `matchCards.test.ts:103-112` (e.g. a purely numeric fragment `'1839'`), add exactly that case.
- **No regression** on the other search modes already covered (title, id/slug, priority-as-text,
  status-as-text, literal metacharacters, priority filter, diacritics, preserved order).
- Persist the deduced root-cause relation
  `relatedTo: [{"kind":"spec","ref":"add-board-command-palette","source":"blame"}]` on this
  spec's state (via the CLI — never editing `.vector/` by hand).

## Scope

- **In** (conditional, only the layer the diagnosis confirms):
  `web/src/components/CommandPalette/matchCards.ts`, its test
  `web/src/components/CommandPalette/matchCards.test.ts`, `web/src/api/useBoard.ts`,
  `cli/internal/board/board.go`, or the re-embed of `cli/internal/webui/dist/`; plus the
  `relatedTo` persistence.
- **Out**: any new search feature (ranking/relevance, fuzzy match, match highlighting, new
  haystack fields); UI/UX changes to `CommandPalette` beyond the strict fix; changing the
  `Board`/`Card`/`Ticket` contract shape or field names (Go or TS) — only data-population logic
  may be corrected within the existing shape; work on any other board filter or view (Kanban,
  Standup, Tokens); introducing `RegExp` into `matchCards` or any new dependency.

Authored spec: `.vector/specs/fix-palette-ticket-key-search/spec.md`.
