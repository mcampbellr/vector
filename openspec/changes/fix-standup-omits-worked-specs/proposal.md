# Fix standup omitting worked specs

## Why

`vector standup` silently drops specs that had real work in the window, because of two
independent defects in the projection layer:

1. **Projection gap** — `cli/internal/standup/standup.go` (`Project` and `Timeline`) only decodes
   `spec.created`, `status.changed` and `work.logged`. The `spec.fixed` event emitted by
   `/vector:fix` (`Store.FixSpec`, `cli/internal/state/store.go:555-586`) carries `Classification`
   + `Files` (`FixedData`, `cli/internal/state/event.go:89-94`) but is ignored: it only bumps
   `ChangeCount`, with no `Work` entry and no timeline entry. A spec whose only event in the window
   is `spec.fixed` disappears from the digest entirely.
2. **Malformed marker window** — `Project`/`Timeline` filter only `TS >= since`, with no upper
   bound (`standup.go:94,167`); the marker advances to a wall-clock `now` captured in `runStandup`
   (`cli/cmd/vector/standup.go:36`, `time.Now()`), decoupled from the real `to` of the projected,
   committed window. An event written between the captured `now` and the commit lands in a race
   zone: it can be missing from this period's digest and, worse, reappear and be double-counted in
   the next.

Real evidence in the somnio dataset: spec `listing-detail-costs-and-fees` (MH-1723), closed
2026-07-13, never appeared in that day's standup — the marker had advanced 25s **before** the real
close (direct proof of leak #2). The fix lets any dev running `/vector:standup` see all observable
work in the window — `/vector:fix` corrections, state transitions and logged work — each with its
concrete detail (files, classification).

## What changes

- **Project `spec.fixed`** in `Project` and `Timeline` (`standup.go`): decode `FixedData` and
  produce a work entry carrying `Classification` + `Files` (reusing `state.WorkLoggedData` with a
  documented semantic mapping, or a dedicated `Fixed []state.FixedData` field — implementer's
  choice), plus a `Timeline` entry with `Type: "spec.fixed"`. `ChangeCount`/`LastChanged` stay as
  they are today.
- **Half-open window `[from, to)`** in `Project`/`Timeline`: add an upper-bound `to`/`until`
  parameter (filter becomes `TS < since || TS >= until`); an event exactly at `to` is excluded
  now, included next period. `runStandup` captures `to` **once** and exposes it in the `--json`
  output (`until`); `runStandupCommit` reuses that exact `to` for `WriteStandup` so the marker
  advances to the projected boundary, not a recomputed `now`.
- **Projection-side only** — `Store.FixSpec` is unchanged and **no** `work.logged` is emitted from
  it, preserving compatibility with historical logs that already carry `spec.fixed` without an
  accompanying `work.logged`. New projection/timeline fields are additive (`omitempty`).
- **Regression tests** in `cli/internal/standup/standup_test.go` (spec.fixed projection in Project
  + Timeline; half-open boundary before/at/after `to`) and, recommended, a command-level scenario
  in `cli/cmd/vector/standup_test.go` (`FixSpec` without `worklog` → `vector standup --json` →
  assert the derived work entry).

## Scope

- In: `spec.fixed` projection in `Project`/`Timeline`, the `[from, to)` half-open window + single
  `to` capture reused for the marker, the additive `until`/`--until` mechanism on
  `standup`/`standup commit`, regression + boundary tests, optional command-level test.
- Out: changing the `spec.fixed` contract (`FixedData`) / `/vector:fix` / `Store.FixSpec`; emitting
  `work.logged` from `FixSpec` (decided, §10); bumping `EventVersion`/`StandupSchemaVersion`;
  rewriting/migrating historical `activity.jsonl`; other event types
  (`sketch.attached`/`pr.opened`/…); mandatory rewrite of the `/vector:standup` kit command wiring
  (documented as a design note, not a blocker); web board component changes beyond verifying
  `SpecTimeline` renders the new `Type` (open question).

Authored spec: `.vector/specs/fix-standup-omits-worked-specs/spec.md`.
