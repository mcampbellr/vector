# Move relation chips from SpecCard board face to SpecDetailsDrawer

**Change type:** refactor

## What changes
Remove the `RelatedChips` component (the "relates to" chip with the git icon, derived
from `card.relatedTo`) from the `SpecCard` board face. Render those same relation chips
only inside the `SpecDetailsDrawer`, in a new "Related" section. The data model, the Go
board contract, and the `relationChips` derivation logic remain unchanged — pure
presentation move (card → drawer).

## Why
Tightens the board card face to show only essential metadata; relation context belongs
in the full drawer where the user examines spec connections without board clutter.

## Files to touch
- `web/src/components/SpecCard/SpecCard.tsx` — remove `RelatedChips` import + conditional render
- `web/src/components/SpecDetailsDrawer/index.tsx` — import `RelatedChips`; add a guarded "Related" section

## Acceptance
- `RelatedChips` no longer renders on board cards.
- The drawer renders a "Related" section only when `card.relatedTo` is non-empty.
- Chips render identically in the drawer (same icon, labels, aria-labels).
- Typecheck/build gate green.

## Out of scope (separate /vector:raw)
- Showing the real git branch/worktree per spec — needs new backend data + API + contract.