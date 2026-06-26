# Design — review-uat-flag

## Key decisions

1. **Refine `review`, do not add a status.** A new lifecycle state would touch the LOCKED state
   machine, board columns (Go + web), `syncStatus`, and every status consumer. A derived boolean
   on the existing `review` card delivers the same visibility with far less surface. (Decided
   with the user.)

2. **Detect via `tasks.md`, reuse `isVerificationTask`.** No new metadata in the user's repo.
   The classifier already recognises smoke test / e2e / "manual" + (check|qa|test|verif). The
   discriminator is exactly `PendingReal == 0` (only verification tasks remain).

3. **`needsUat` is computed by `sync` only and persisted in `state.json`.** `sync` is the single
   path that reads `tasks.md`; persisting the flag (like status) keeps the board a pure
   projection. The transition subcommands (`apply`/`status`/`close`) never compute it — apply's
   finish runs `vector sync --reconcile` to refresh it.

4. **Threading.** `syncStatus` keeps its signature (`func(openspec.Change) state.Status`); a
   sibling `syncNeedsUAT(c) bool` computes the flag. It is passed as a separate datum: a new
   `NeedsUAT` field on `CreateSpecParams` and a `needsUAT bool` parameter on `ReconcileStatus`,
   both setting `spec.NeedsUAT` inside the existing lock/write. The flag is forced to `false`
   whenever the resulting status is not `review`.

## State machine impact

None. `review` and its transitions are unchanged. `needsUat` is an attribute of a `review` card,
not a node in the graph.

## Edge cases

- `TasksDone == 0` (no progress, even if all tasks are verification) → `syncStatus` returns
  `open` (its `TasksDone == 0` case is evaluated before the `PendingReal == 0` case), so the flag
  never applies. The `TasksDone > 0` term in the formula matches that guard deliberately.
- A pending non-verification task keeps `PendingReal > 0` → the change stays `in-progress` and
  never reaches `review`/`needsUat`.
- Re-sync with `--reconcile` that moves a card back to `in-progress` clears the flag.

## Open questions

- Badge label: `UAT` vs `Needs UAT`.
- Whether to also carry `needsUat` in the `status.changed` payload for self-contained replay
  (kept out of scope for now; `state.json` suffices).
- Broadening `isVerificationTask` to catch more UAT wording (e.g. "acceptance", bare "uat").
