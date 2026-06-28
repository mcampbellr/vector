# Post-action spec summaries and details drawer

## Why

Board cards carry their "next command" and activity timeline inline, which clutters the face and
still answers only *how* a spec's status moved — never a readable *what was done*. The only
AI prose Vector produces today is the standup digest, scoped to specs active since the last
standup. A developer who wants to know what happened on a spec must run a standup or read the raw
event timeline. There is no reusable "summarize what just happened" step, and the board has no
place to surface one on demand.

## What changes

- **Post-action summary pipeline (cli + kit)** — a new *orchestrate-then-summarize* primitive
  mirroring the standup pipeline. After every domain transition (`/vector:propose`,
  `/vector:apply`, `/vector:status`, `/vector:close`, `/vector:archive`) the command runs
  `vector spec summarize <id> --json`, spawns a cheap **Haiku** agent (`vector-summary-writer`) on
  the projection, and persists the prose via `vector spec summarize <id> commit`. The summary lives
  in `.vector/local/summaries.json` (gitignored), keyed by spec id. CLI-owns-writes; the binary
  never calls an LLM.
- **Standup reuse** — `standup.SpecActivity` gains a `PriorSummary` field that `enrichProjection`
  fills from the stored summary; `vector-standup-writer` consumes it as context, so `/vector:standup`
  digests build on what is already known.
- **Read endpoint** — `GET /api/summary?spec=<id>` serves the persisted summary (`{}` when none);
  the board `Source` interface gains `ReadSummary`.
- **Spec details drawer (web)** — board cards become clickable and keep metadata only on the face;
  the inline `NextCommand` and `SpecTimeline` move into a right-side `SpecDetailsDrawer` that
  surfaces the AI summary, the activity timeline, the next command, and a set of **copyable** useful
  commands (e.g. `/vector:link` only when the spec has no ticket). The board stays read-only:
  "assign a ticket from here" is a copyable command, not a web mutation.

## Capabilities

### New Capabilities
- `spec-summary`: after each domain transition, generate and persist a per-spec "what was done"
  summary via a cheap Haiku agent, stored locally and served read-only at `GET /api/summary`.
- `spec-details-drawer`: a clickable-card drawer surfacing the summary, activity, next command, and
  copyable useful commands; the card face is reduced to metadata.

### Modified Capabilities
- `standup-digest`: the digest now consumes each spec's `priorSummary` as context for richer prose
  (additive; a missing summary leaves today's behavior unchanged).

## Impact

- `cli/internal/state` (new `summary.go`: `SpecSummary` + `summaries.json` store), `cli/cmd/vector`
  (`summarize` subcommand + `enrichProjection` fill), `cli/internal/standup` (`SpecActivity`
  field), `cli/internal/board` (`Source.ReadSummary` + `GET /api/summary`).
- `kit/`: new `agents/vector-summary-writer.md`; modify `commands/vector/{apply,propose,status,close,
  archive}.md` and `agents/vector-standup-writer.md`. Scaffold assets regenerated via `go generate`.
- `web/`: new `SpecDetailsDrawer/` + `UsefulCommands`, `useSpecSummary` hook, `SpecSummary` type;
  modify `SpecCard`, `BoardColumn`, `KanbanBoard`.
- No new dependencies (Go stdlib; existing React libs). The board stays read-only.

Authored spec: `.vector/specs/add-spec-summary-drawer/spec.md`.
