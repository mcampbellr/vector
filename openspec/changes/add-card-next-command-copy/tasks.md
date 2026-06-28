# Tasks — add-card-next-command-copy

## 1. New card component

- [x] 1.1 `web/src/components/SpecCard/CardNextCommand.tsx`: new presentational component. Props
      `{ status: Status; id: string }`; `command = nextCommandFor(status, id)`; `null` → `return null`.
      Render `<code>` (command, monospace, single line, ellipsis) + copy `<button>` (`lucide-react`
      `Copy`/`Check`). `handleCopy(event)`: `event.stopPropagation()` first, guard
      `navigator.clipboard`, write command, `copied = true`, reset after 1500ms. Button
      `aria-label="Copy next command"` (static — do not make it dynamic). Reference:
      `web/src/components/SpecDetailsDrawer/CopyableCommand.tsx`. No collapse/toggle, no fetch, no
      label list.
- [x] 1.2 `web/src/components/SpecCard/CardNextCommand.module.css`: compact flex row
      (`display:flex; align-items:center; gap`) with a top border separating it from the metadata,
      reusing the token-based look of `NextCommand.module.css` (`.body` / `.command` / `.copyBtn` /
      `.copyBtn.copied`); tokens only (`var(--color-*)`, `var(--space-*)`, `var(--radius-*)`).

## 2. Wire into the card

- [x] 2.1 `web/src/components/SpecCard/SpecCard.tsx`: import `CardNextCommand`; render
      `<CardNextCommand status={card.status} id={card.id} />` after the `<footer>` metadata block,
      inside the `<article>`. Do not change existing metadata or the card's click/keyboard behavior
      (`onSelect`, Enter/Space) beyond the copy button's `stopPropagation`.
- [x] 2.2 Broaden the `SpecCard` component doc-comment: replace the stale "metadata only" / "next
      command ... in the drawer" wording with "metadata + a quick-copy next command; the activity
      timeline, AI summary and useful commands remain in the drawer."

## 3. Remove the orphaned variant

- [x] 3.1 Delete `web/src/components/SpecCard/NextCommand.tsx` and
      `web/src/components/SpecCard/NextCommand.module.css` (done after §1, since they are the style
      reference). Verify no import references them.

## 4. Verification

- [x] 4.1 `npm --prefix web run typecheck` green (no `any`, no TS errors).
- [x] 4.2 `npm --prefix web run build` succeeds.
- [x] 4.3 Manual check of §8 success criteria: row present for command-bearing statuses, absent for
      `closed`; copy shows check ~1.5s then reverts; copy does not open the drawer; rest of the card
      and Enter/Space still open it; drawer next command unchanged.
- [x] 4.4 Re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild + reinstall the `vector`
      binary to `~/.local/bin/vector` (dogfooding).
