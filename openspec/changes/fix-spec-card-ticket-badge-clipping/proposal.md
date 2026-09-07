# Fix ticket badge clipping in the spec card header

## Why

The linked-ticket badge (`card.ticket`, `Tag` icon + key such as `MH-1814`) in the `SpecCard`
header (`web/src/components/SpecCard/SpecCard.tsx:43-48`) is silently clipped, truncated or pushed
off the card whenever the spec title is long. It is a pure CSS layout bug:

- `.head` (`web/src/components/SpecCard/SpecCard.module.css:25-30`) is
  `display: flex; justify-content: space-between;`, but `.title`
  (`SpecCard.module.css:32-38`) has no `min-width: 0`, so the flex item never shrinks under long
  content and instead forces `.ticket` past the card's right edge, where it is clipped without
  warning.
- `.ticket` (`SpecCard.module.css:40-51`) has no `max-width`, no `white-space: nowrap` and no
  overflow protection. The hyphen in `MH-1814` is a valid line-break point, so under compression
  the badge can split into `MH-`.

The defect is latent since the board's founding commit `5441b80` (which introduced the
`space-between` header without `min-width: 0`), but only became observable once `add-ticket-linking`
populated `card.ticket` with real data — hence the recorded root-cause relation is the feature that
made the bug visible, not the commit that wrote the faulty CSS.

Fixing it lets a dev scanning the board identify the external ticket linked to any spec at a glance,
with a badge that is always legible regardless of title length, instead of a silent clip against the
card border.

## What changes

- Guarantee the ticket badge (`.ticket`) is **always visible and legible** on every card with
  `card.ticket` defined, regardless of `card.title` length, degrading the title gracefully
  (shrink/ellipsis, or layout reorg) so the badge is never sacrificed.
- The implementation **method is left open** — decided during `/vector:apply` with `/ui-ux-pro-max`,
  not preselected in the spec:
  - **Option A — in-place fix**: align `.title`/`.ticket` to the overflow pattern already present in
    the same file (`min-width: 0` + ellipsis on `.title`, analogous to `.attentionSummary`;
    `max-width` + `overflow: hidden` + `white-space: nowrap` on `.ticket`, analogous to `.relation`).
    Touches only `SpecCard.module.css`.
  - **Option B — relocation**: move the badge out of the header (own row, or rely on the drawer,
    which already renders it at `SpecDetailsDrawer/index.tsx:72-83`), analogous to how relation chips
    were moved to the drawer.
- Add a new test in `web/src/components/SpecCard/SpecCard.test.tsx` covering a very long title with a
  linked ticket → badge rendered with the complete key, not clipped.
- Persist `relatedTo: [{"kind":"spec","ref":"add-ticket-linking","source":"manual"}]` on this spec's
  state (via the CLI, never editing `.vector/` by hand).

## Scope

- **In**: `web/src/components/SpecCard/SpecCard.module.css` (always), and conditionally
  `SpecCard.tsx` + `SpecDetailsDrawer/index.tsx` only under Option B; the new long-title test; the
  `relatedTo` persistence.
- **Out**: changing the rest of the header (`StatusPill`, `PriorityFlag`, quick-win, UAT, sketch,
  estimate, savings); the `Card.ticket` data contract (`web/src/types/board.ts:16-20`,
  `cli/internal/board/board.go:70-75`); any `cli/` or `GET /api/board` change; a systemic sweep of
  other `space-between` headers introduced by `5441b80` (latent-clipping risk noted, not blocking).

Authored spec: `.vector/specs/fix-spec-card-ticket-badge-clipping/spec.md`.
