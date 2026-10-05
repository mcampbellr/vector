# Add `vector spec set --priority` (reprioritize an existing spec)

## Why

Today a spec's `Priority` can only be set at creation (`vector spec create --priority`,
`cli/cmd/vector/main.go:1171`). There is **no CLI path to change it afterward**. This surfaced
during `/vector:propose` when trying to mark a card as Urgent — and `needs-attention` is **not**
a proxy for urgency: it means "blocked / requires attention", an axis orthogonal to priority.
Reprioritizing is normal backlog grooming (priority feeds `priorityRank` → `SelectNext`, which
decides what `vector spec next` / `/vector:apply` picks), yet it is currently unrepresentable
without hand-editing `.vector/` — which violates CLI-owns-writes.

## What changes

- New `Store.SetPriority(id, priority, actor, now) (*SpecState, error)` primitive in
  `cli/internal/state/store.go` — validates the priority before locking, rejects archived specs,
  treats a same-value call as an idempotent no-op (no event), otherwise mutates `Priority` +
  `UpdatedAt`, writes `state.json` atomically, and appends `priority.changed`.
- New `EvtPriorityChanged = "priority.changed"` event + `PriorityChangedData{From, To Priority}`
  payload in `cli/internal/state/event.go`. It is a **lateral metadata mutation**, not a state
  machine transition — no `Trigger`/`Reason`, no `Status` change.
- New `vector spec set <id> --priority <urgent|high|normal|low>` subcommand (`newSpecSetCmd` in
  `cli/cmd/vector/spec_transitions.go`), following the existing `link`/`status`/`fix` shape:
  leading-id positional (or `--id`), required `--priority`, plus `--dry-run`, `--json`,
  `--repo-root`. Registered in `newSpecCmd()` and documented in `usage()`.
- Unit tests (`Store.SetPriority`: success+event, no-op, invalid, unknown, archived), CLI tests
  (`vector spec set`: parsing, `--dry-run`, `--json`, errors) with a `runSpecSet` shim, and a
  `spec-set` golden case fixing the `--json` contract byte-identically.

## Scope

- **In**: the `SetPriority` primitive, the `priority.changed` event + payload, the `vector spec
  set` subcommand + registration + help, and the unit/CLI/golden tests.
- **Out**: changing `needs-attention` semantics; wiring `--priority` into
  `propose`/`apply`/`status`; board drag-and-drop reprioritization (would need HTTP write
  endpoints — `internal/board` is read-only); any `SchemaVersion`/`EventVersion` bump (`Priority`
  already exists as a field); refactoring `applyTransition`/`SetStatus`/the LOCKED state machine;
  new external dependencies.

Authored spec: `.vector/specs/add-spec-priority-mutation/spec.md`.
