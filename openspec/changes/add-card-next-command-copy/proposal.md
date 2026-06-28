# Copyable next command on the card face

## Why

The board card (`SpecCard`) shows a spec's metadata but not its next slash command — to copy the
command to run next a developer must open the details drawer. The drawer already exposes a copyable
next command, but for the common case (copy the command, paste into Claude Code) opening the drawer
is friction. The card should be a one-click quick-copy shortcut while the drawer stays the full
detail surface.

## What changes

- **Inline copyable next-command row on the card (web)** — a new presentational component
  `CardNextCommand` renders an always-visible row under the card metadata: the next slash command
  (monospace, single line, ellipsis on overflow) + a copy-to-clipboard button with check-icon
  feedback (~1.5s), reusing the existing copy pattern (`CopyableCommand`/`NextCommand`). The command
  is derived from the card status via the existing `nextCommandFor(status, id)` mapper (unchanged,
  shared with the drawer). When `nextCommandFor` returns `null` (status `closed`), the row renders
  nothing.
- **Wire into `SpecCard` (web)** — `SpecCard` composes `<CardNextCommand status id>` after the
  `<footer>` metadata block, inside the `<article>`. The copy button calls `event.stopPropagation()`
  so copying does not also open the drawer; clicking elsewhere on the card (and Enter/Space) still
  opens the drawer, unchanged.
- **Remove the orphaned collapsible variant** — delete `SpecCard/NextCommand.tsx` and
  `NextCommand.module.css` (the old collapsible component, no longer imported anywhere). Its CSS is
  the style reference for `CardNextCommand`, so the new files are authored before the deletion.

## Capabilities

### Modified Capabilities
- `spec-card`: the card face gains an always-visible, copyable next-command row (derived from
  status), below the existing metadata. The card's click/keyboard behavior is unchanged except the
  copy button stops propagation. The orphaned collapsible `NextCommand` variant is removed.

## Impact

- `web/src/components/SpecCard/`: new `CardNextCommand.tsx` + `CardNextCommand.module.css`;
  `SpecCard.tsx` composes the new component and its doc-comment is broadened (no longer "metadata
  only"); `NextCommand.tsx` + `NextCommand.module.css` deleted.
- No change to `nextCommandFor.ts`, the drawer, the CLI, the API, the domain model, or dependencies.
  The board stays a read-only projection of CLI state; the command is copyable, never executed from
  the web.

Authored spec: `.vector/specs/add-card-next-command-copy/spec.md`.
