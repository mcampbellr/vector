# Tasks — add-copyable-slug-display

## 1. Shared CopyableSlug component

- [ ] 1.1 `web/src/components/CopyableSlug/CopyableSlug.tsx`: new presentational component. Props
      `{ slug: string }`; local `copied` state. Render `<code>` (slug, monospace, single line,
      ellipsis) + copy `<button>` (`lucide-react` `Copy`/`Check`). `handleCopy(event)`:
      `event.stopPropagation()` first, guard `navigator.clipboard`, `writeText(slug)`,
      `copied = true`, reset after 1500ms. Button `aria-label="Copy spec id"` (static — do not make
      it dynamic). Reference: `web/src/components/SpecCard/CardNextCommand.tsx`. No collapse/toggle,
      no fetch, no label list.
- [ ] 1.2 `web/src/components/CopyableSlug/CopyableSlug.module.css`: compact flex row
      (`display:flex; align-items:center; gap:var(--space-2)`) with **no** top border / separator.
      `.slug`: `flex:1; min-width:0;` monospace, 11px, `var(--color-text-secondary)`,
      `var(--color-surface-muted)` background, `1px solid var(--color-border)`,
      `var(--radius-pill)`, `padding:3px var(--space-2)`, ellipsis. `.copyBtn` / `.copyBtn.copied`:
      mirror `CardNextCommand.module.css` `.copyBtn` (24×24, pill, `--color-accent` + `#eef2ff`
      hover, `#047857` + `#d1fae5` copied). Tokens only.

## 2. Wire into the card

- [ ] 2.1 `web/src/components/SpecCard/SpecCard.tsx`: import `CopyableSlug` from
      `'../CopyableSlug/CopyableSlug'`; render `<CopyableSlug slug={card.id} />` as the first
      `<article>` child after `</header>` and before the `attentionReason` paragraph. Do not change
      existing metadata, `CardNextCommand`, or the card's click/keyboard behavior (`onSelect`,
      Enter/Space) beyond the copy button's `stopPropagation`.
- [ ] 2.2 Broaden the `SpecCard` component doc-comment to mention the slug (e.g. "metadata (title,
      slug, ticket, artifacts, status, priority, estimate, savings) plus a quick-copy next command").

## 3. Wire into the drawer

- [ ] 3.1 `web/src/components/SpecDetailsDrawer/index.tsx`: import `CopyableSlug` from
      `'../CopyableSlug/CopyableSlug'`; replace `<code className={styles.id}>{card.id}</code>`
      (line 55) with `<CopyableSlug slug={card.id} />`, keeping it inside `<div
      className={styles.headerMain}>` below the `<h2>` title. Do not change the close button or
      `metaRow`.
- [ ] 3.2 `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css`: remove the now-orphaned
      `.id` rule. Do not remove `.headerMain` or `.title`. Verify no `styles.id` reference remains.

## 4. Verification

- [ ] 4.1 `npm --prefix web run typecheck` green (no `any`, no TS errors).
- [ ] 4.2 `npm --prefix web run build` succeeds.
- [ ] 4.3 Manual check of §8 success criteria: slug row present under the title on every card and in
      the drawer header; copy writes the bare id (not the command) and shows `Check` ~1.5s then
      reverts; copy on the card does not open the drawer; rest of the card and Enter/Space still open
      it; `CardNextCommand` unchanged; no orphaned `.id` reference.
- [ ] 4.4 Re-embed `web/dist` into `cli/internal/webui/dist/` and rebuild + reinstall the `vector`
      binary to `~/.local/bin/vector` (dogfooding).
