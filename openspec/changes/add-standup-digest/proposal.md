# Add standup digest with enriched activity trace

## Why

The activity log only records `status.changed` — *how* a spec's state moved, not *what* was
done. A scrum standup needs the work (files touched, tasks completed), which only the
`/vector:apply` flow knows. Today a dev writes the ceremony report by hand and the team has no
shared view of what progressed since the last standup.

## What changes

- **Enriched trace**: new `work.logged` event (`WorkLoggedData{Change, FilesTouched,
  TasksCompleted, Note}`) appended on each apply via a new `vector spec worklog` subcommand,
  alongside the existing `status.changed`. Additive — no existing event or `SpecState` changes.
- **Personal "last standup" marker** in `.vector/local/standup.json` (gitignored). The default
  window covers activity since the marker; the marker advances when the command runs.
- **Read-only projection** (`cli/internal/standup`): reads events from the marker (or a
  `--since` window), groups by spec, exposes a `Projection` (per-spec + totals). No prose, no LLM.
- **Binary `vector standup`**: `--json` projects the period; `vector standup commit
  --digest-file -` persists the generated digest and advances the marker. CLI-owns-writes.
- **Project command `/vector:standup`**: projects via the binary, routes the JSON to a Haiku
  agent (`vector-standup-writer`) for the global + per-spec digest, persists via the binary.
- **UI**: dedicated `StandupView` (period digest + per-spec cards) and expandable `SpecTimeline`
  per card, fed by `GET /api/standup` and `GET /api/activity?spec=<id>`.

## Capabilities

### New Capabilities
- `standup-digest`: project activity since the last standup, generate a natural-language digest
  (global + per-spec) via a cheap agent, persist it, and expose it on the board.
- `activity-worklog`: enrich the append-only activity log with `work.logged` events emitted by
  `/vector:apply`.

### Modified Capabilities
<!-- None: work.logged is additive; existing events and SpecState are unchanged. -->

## Impact

- `cli/internal/state` (event + store), new `cli/internal/standup` package, `cli/cmd/vector`
  (`standup`, `worklog`), `cli/internal/board` (two read-only GET handlers).
- `web/`: `StandupView`, `SpecTimeline`, `useStandup`, `types/standup.ts`.
- `kit/`: `commands/vector/standup.md`, `agents/vector-standup-writer.md`, modify `apply.md`.
- No new dependencies (Go stdlib; existing React libs). The binary never calls an LLM.

Authored spec: `.vector/specs/add-standup-digest/spec.md`.
