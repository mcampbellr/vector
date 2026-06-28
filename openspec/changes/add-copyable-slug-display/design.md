# Design — add-copyable-slug-display

## Context

The board card (`web/src/components/SpecCard/SpecCard.tsx`) is a clickable, presentation-only card
that owns an `onSelect(card)` callback (opens the details drawer). Its `<header className="head">`
is a flex row with the title and an optional ticket pill; the `<article>` is a flex column. The
slug (`card.id`, a `string` in `web/src/types/board.ts`) currently appears on the card only inside
the next command rendered by `CardNextCommand` (`/vector:apply <id>`). The details drawer
(`web/src/components/SpecDetailsDrawer/index.tsx`) shows the slug at line 55 inside `.headerMain` as
`<code className={styles.id}>{card.id}</code>` — visible but **not** copyable.

The copy-to-clipboard pattern already exists twice: `SpecCard/CardNextCommand.tsx` (with
`event.stopPropagation()`, `.copyBtn`/`.copyBtn.copied` styles) and
`SpecDetailsDrawer/CopyableCommand.tsx` (without). The board is a read-only SSE projection; no
canonical client state.

## Goals / Non-Goals

**Goals:**
- Surface the bare slug as an always-visible, copyable element on both the card face (under the
  title) and the drawer header, reusing the existing copy pattern and design tokens.
- Keep the card clickable to open the drawer; the copy button must not also open it.
- One shared component for both surfaces (the treatment is identical).

**Non-Goals:**
- Touching `CardNextCommand` (the id stays embedded in the command as a separate affordance).
- Changing the `Card` type / API contract (`card.id` already exists), the domain model, or tokens.
- Any board write/mutation, data fetching, or new endpoint (board stays read-only).
- Adding dependencies.

## Decisions

- **Single shared presentational component `CopyableSlug`** (reuse before create), in its own
  folder `web/src/components/CopyableSlug/` because it is composed by **two** parents (card +
  drawer) — it is not card- or drawer-scoped. Props `{ slug: string }`. Renders an inline row: a
  `<code>` with the slug (monospace, single line, ellipsis) + a copy `<button>` (`lucide-react`
  `Copy` → `Check` on copied).
- **Copy handler mirrors `CardNextCommand`**: `handleCopy(event)` calls `event.stopPropagation()`
  first (essential on the card so it does not open the drawer; harmless in the drawer, whose panel
  already stops propagation), guards `navigator.clipboard` (absent → no-op, no error), writes the
  bare `slug`, sets `copied = true`, resets after 1500ms. Reentrant/idempotent (each click restarts
  the timer).
- **Static `aria-label="Copy spec id"`** — the slug text is already visible inline, so a static
  label avoids redundancy (same rationale as `CardNextCommand`'s static label). Do not make it
  dynamic.
- **Card placement = under the header, above metadata**: render `<CopyableSlug slug={card.id} />` as
  the first `<article>` child after `</header>` and before the `attentionReason` paragraph, so the
  slug sits directly below the title (mirrors the drawer's title-then-slug header layout). Chosen
  over a footer row next to the next command, to keep the slug visually distinct from the command.
- **Drawer placement = replace the read-only `<code>`**: swap line 55 for `<CopyableSlug
  slug={card.id} />` inside `.headerMain`, below the `<h2>` title. The orphaned `.id` rule is then
  removed from `SpecDetailsDrawer.module.css`.
- **Styling**: `CopyableSlug.module.css` is a compact flex row with **no** top-border separator
  (it sits under the title, not after metadata — the difference vs `CardNextCommand.module.css`
  `.body`). The `<code>` chip reuses the `.command` look (monospace 11px, `--color-text-secondary`,
  `--color-surface-muted` background, `--color-border`, `--radius-pill`, ellipsis); the button
  mirrors `.copyBtn`/`.copyBtn.copied` exactly (24×24, `--color-accent` hover, `#047857`/`#d1fae5`
  copied).
- **Dependencies**: none added — `lucide-react` (already in use) supplies `Copy`/`Check`, and
  `navigator.clipboard` is native.

## Risks / Trade-offs

- **Redundancy with the next command** — the id appears both standalone and embedded in
  `CardNextCommand`. Accepted and intentional: copying the bare id and copying the full command are
  distinct flows.
- **Truncation** — a long slug is ellipsis-truncated visually, but the full `card.id` is always
  copied (we write the prop, not the rendered text), so no data loss.
- **Clipboard unavailable** — handler no-ops behind the `navigator.clipboard` guard; the UI does
  not break or show an error (existing pattern).

## Migration / Rollout

Pure presentation change; no state, schema, API, or data migration. Verified by `npm --prefix web
run typecheck` + `npm --prefix web run build`, then re-embed `web/dist` into
`cli/internal/webui/dist/` and rebuild/reinstall the `vector` binary for dogfooding.
