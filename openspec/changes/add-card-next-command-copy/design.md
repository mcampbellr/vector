# Design — add-card-next-command-copy

## Context

`SpecCard` (`web/src/components/SpecCard/SpecCard.tsx`) is a clickable, presentation-only card that
owns an `onSelect(card)` callback (opens the details drawer) and renders metadata only. The drawer
already exposes a copyable next command via `CopyableCommand`
(`web/src/components/SpecDetailsDrawer/`). The status → command mapping lives in
`SpecCard/nextCommandFor.ts` (`nextCommandFor(status, id)` returns the slash command or `null`),
shared by card and drawer. A previous collapsible card variant, `SpecCard/NextCommand.tsx` (+
`NextCommand.module.css`), is orphaned — no longer imported anywhere — but still in the repo as the
copy-affordance style reference. The board is a read-only SSE projection; no canonical client state.

## Goals / Non-Goals

**Goals:**
- A compact, always-visible copyable next-command row on the card face, derived from status,
  reusing the existing copy pattern and design tokens.
- Keep the card clickable to open the drawer; the copy button must not also open it.
- Remove the orphaned collapsible `NextCommand` component in the same change.

**Non-Goals:**
- Touching the drawer's next command (already copyable) or its useful-commands list.
- Any board write/mutation, data fetching, or new endpoint (board stays read-only).
- Changing `nextCommandFor.ts`, the card metadata, the click/keyboard behavior (beyond
  `stopPropagation`), tokens, or adding dependencies.

## Decisions

- **New presentational component `CardNextCommand`** (one component per file, card-scoped, next to
  `SpecCard.tsx`). Props `{ status: Status; id: string }`. Computes
  `command = nextCommandFor(status, id)`; `command === null` → `return null` (no row, no extra
  border). Otherwise renders an inline row: a `<code>` with the command (monospace, single line,
  ellipsis) + a copy `<button>` (`lucide-react` `Copy` → `Check` on copied).
- **Copy handler mirrors `CopyableCommand`**: `handleCopy(event)` calls `event.stopPropagation()`
  first, guards `navigator.clipboard` (absent → no-op, no error), writes the command, sets
  `copied = true`, resets after 1500ms. Reentrant/idempotent (each click restarts the timer).
- **Static `aria-label="Copy next command"`** — intentionally diverges from `CopyableCommand`'s
  dynamic `` `Copy command: ${command}` ``: on the card the command text is already visible inline,
  so a static label avoids redundancy. Do not "correct" it to the dynamic form.
- **Composition in `SpecCard`**: render `<CardNextCommand status={card.status} id={card.id} />`
  after the `<footer>` metadata, inside the `<article>`. Broaden the component doc-comment to reflect
  the new reality ("metadata + a quick-copy next command; activity / AI summary / useful commands
  remain in the drawer") — no stale "metadata only" wording.
- **CSS**: `CardNextCommand.module.css` reuses the token-based look of `NextCommand.module.css`
  (`.body` / `.command` / `.copyBtn` / `.copyBtn.copied`): compact flex row, top border separating
  it from the metadata, `var(--color-*)` / `var(--space-*)` / `var(--radius-*)`.
- **Authoring order**: create `CardNextCommand.tsx` + `.module.css` first (copying the look from
  `NextCommand.module.css`), then delete `NextCommand.tsx` + `NextCommand.module.css`, since the
  deleted files are the style reference.

## Open questions

- None substantive. No test framework is configured for `web/`; verification is typecheck + build +
  manual check (and re-embed + reinstall the binary for dogfooding). If a test setup is later added,
  cover: command present vs `null`, and copy-button propagation stop.
