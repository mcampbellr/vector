# Mark review cards that require manual UAT

## Why

`review` today conflates two cases: "implementation done, tests pass, nothing to verify" and
"implementation done, only manual UAT/QA remains". A dev or QA can't tell from the board which
review cards are waiting on a human to verify them, so they can't prioritise or coordinate that
work. We want that distinction visible without inflating the lifecycle.

## What changes

- Add a derived boolean `needsUat` to a spec, persisted in `state.json`. **No new status, no new
  board column, no change to the state machine** — `review` is refined, not replaced.
- Compute the flag in `sync` only (the single path that reads `tasks.md`): a `syncNeedsUAT`
  helper sets it when a change reaches `review` because only verification tasks remain
  (`TasksDone > 0 && TasksDone < TasksTotal && PendingReal == 0`), reusing the existing
  `isVerificationTask` classifier. The flag is cleared when the card leaves `review`.
- Project `needsUat` onto the board `Card` and render a small **"UAT" badge** on review cards in
  the web panel. The column stays `review` (single-axis intact).
- `/vector:apply`'s finish refreshes the flag via `vector sync --reconcile`; the transition
  subcommands gain no UAT-specific logic.

## Scope

- In: the `NeedsUAT` field + serialization, the `syncNeedsUAT` computation threaded through
  `CreateSpec`/`ReconcileStatus`, the board projection, the web badge, and a domain-contract note.
- Out: a new status/column, manual override of the flag (depends on the future board write API),
  UAT ownership/assignment, external QA-tracker integration, drag-and-drop.

Authored spec: `.vector/specs/review-uat-flag/spec.md`.
