# Show sketch indicator icon on board cards

**Change Type:** style (web/ frontend)

## What changes
Render a small icon in the `SpecCard` footer when the spec has one or more Excalidraw
sketches attached (`card.sketches` non-empty). Icon-only (lucide-react `Layers`), no text
label — matches the user's request ("un icono o algo así"). A `title` tooltip surfaces the
count for discoverability/a11y. The exact count stays in the details drawer.

## Why
Specs with attached wireframes become instantly visible on the board without opening the
details drawer, improving discoverability of design artifacts.

## Files to touch
- `web/src/components/SpecCard/SpecCard.tsx` — add `Layers` import + conditional icon in the meta footer.
- `web/src/components/SpecCard/SpecCard.module.css` — add a `.sketch` icon style.

## Data flow (already complete — no backend change)
- `Card.sketches` is already projected by `cli/internal/board` (`Sketches []state.SketchRef`)
  and mirrored on the web `Card` type (`sketches?: SketchRef[]`). Only the visual indicator
  was missing.

## Acceptance
- Icon appears only when `card.sketches?.length > 0`.
- Icon-only (no text), styled consistently with the footer's other indicators.
- `title`/`aria-label` conveys the sketch count.
- `tsc -b --noEmit` passes; no console warnings.

## Gate
- `npm --prefix web run typecheck`.
- Re-embed web dist into the binary per the frontend edit flow before reinstalling.
