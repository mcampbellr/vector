# Copyable slug on card face and drawer

## Why

A spec's slug (`card.id`) is the stable handle a developer reaches for — to name a git branch,
point at `.vector/specs/<slug>/`, reference the spec in a ticket comment or a standup note. Today
the slug is never exposed as a first-class, copyable element: on the board card it only appears
**embedded inside** the next command (`/vector:apply <id>` via `CardNextCommand`), and in the
details drawer header it is shown as a **non-copyable** `<code>` (`SpecDetailsDrawer/index.tsx:55`).
Grabbing the bare id means copying the whole command and trimming it, or hand-retyping. The card and
the drawer should both let the developer copy the exact slug in one click.

## What changes

- **Shared `CopyableSlug` component (web)** — a new presentational component
  (`web/src/components/CopyableSlug/`) renders an inline row: the slug (`<code>`, monospace, single
  line, ellipsis on overflow) + a copy-to-clipboard button with check-icon feedback (~1.5s), reusing
  the existing copy pattern (`CardNextCommand`/`CopyableCommand`). `handleCopy` calls
  `event.stopPropagation()` first (so copying on the card does not open the drawer), guards
  `navigator.clipboard`, writes the **bare** `card.id`, and shows `Check` for ~1.5s. Static
  `aria-label="Copy spec id"`.
- **Wire into `SpecCard` (web)** — `SpecCard` composes `<CopyableSlug slug={card.id} />` directly
  under the `<header>` (title/ticket) block and above the attention/related/artifacts blocks, so the
  slug sits just below the title. The card's click/keyboard behavior is unchanged except the copy
  button stops propagation. `CardNextCommand` is untouched (the slug-in-command stays as a separate
  affordance).
- **Wire into the drawer (web)** — `SpecDetailsDrawer/index.tsx` replaces the non-copyable
  `<code className={styles.id}>{card.id}</code>` (line 55) inside `.headerMain` with
  `<CopyableSlug slug={card.id} />`, and the now-orphaned `.id` rule is removed from
  `SpecDetailsDrawer.module.css`.

## Capabilities

### Modified Capabilities
- `spec-card`: the card face gains an always-visible, copyable slug row directly under the title
  (above the metadata). Click/keyboard behavior is unchanged except the copy button stops
  propagation. `CardNextCommand` is unchanged.
- `spec-details-drawer`: the drawer header's slug becomes copyable (same `CopyableSlug` affordance),
  replacing the previous read-only `<code>`.

## Impact

- New `web/src/components/CopyableSlug/CopyableSlug.tsx` + `CopyableSlug.module.css` (shared by two
  parents, hence its own folder).
- `web/src/components/SpecCard/SpecCard.tsx`: composes `CopyableSlug` under the header; doc-comment
  broadened to mention the slug.
- `web/src/components/SpecDetailsDrawer/index.tsx`: composes `CopyableSlug` in the header;
  `SpecDetailsDrawer.module.css`: orphaned `.id` rule removed.
- No change to `CardNextCommand`, `nextCommandFor.ts`, the `Card` type, the CLI, the API, the domain
  model, or dependencies. The board stays a read-only projection of CLI state; the slug is copyable,
  never executed from the web.

Authored spec: `.vector/specs/add-copyable-slug-display/spec.md`.
