# Design — add-spec-summary-drawer

## Context

The board projects committed `state` read-only and serves `/api/board` + `/api/events` (SSE),
`/api/standup`, and `/api/activity`. The activity log is append-only and the standup pipeline
already follows a two-step shape: `vector standup --json` (projection) → Haiku
`vector-standup-writer` → `vector standup commit` (persist to `.vector/local/standup.json`). Cards
render `NextCommand` + `SpecTimeline` inline (`web/src/components/SpecCard/SpecCard.tsx:61-63`).
There is no per-spec summary primitive and no on-demand detail surface.

## Goals / Non-Goals

**Goals:**
- A reusable post-action summary primitive: orchestrate → summarize with Haiku → persist locally.
- Fire it after every domain transition (`propose`/`apply`/`status`/`close`/`archive`).
- Reuse the summary in `/vector:standup` (`priorSummary` as context).
- A clickable-card drawer surfacing summary + activity + next command + copyable commands; the card
  face becomes metadata-only.

**Non-Goals:**
- Web write endpoints / real mutations from the drawer (board stays read-only; "assign ticket" is a
  copyable `/vector:link`).
- Triggering the summary from `raw`/`sync`/`link` (no "what was done" content).
- Committing the summary to git or to `state.json` (it is local/gitignored).
- Crediting the Haiku summary call in the Token Savings Meter (no defined baseline).
- A drawer on `StandupSpecRow` or surfaces other than board kanban cards.

## Decisions

- **CLI-owns-writes**: the binary is the sole writer of `summaries.json`; commands never edit
  `.vector/` by hand. The agent only transforms its input JSON (`vector-standup-writer` pattern).
- **Mirror the standup pipeline**: `vector spec summarize <id>` = projection (`--json`) +
  `commit --action <name> --summary-file -`. Invalid/empty input → write nothing (mirrors
  `runStandupCommit`).
- **Local persistence**: `.vector/local/summaries.json`, a `map[specID]SpecSummary`, atomic +
  mutex like `standup.json`. The summary is derived and regenerable; it does not travel via git.
- **Every domain transition triggers**; `raw`/`sync`/`link` are excluded as producing no work prose.
- **`Project`/`Timeline` stay store-free**; `PriorSummary` is filled by the caller
  (`enrichProjection`), keeping the pure projection functions store-independent.
- **Read-only board, copyable commands**: the drawer mutates nothing; useful commands are
  context-aware copyable slash commands (`/vector:link` only when the spec has no ticket).
- **One drawer at board level**: `KanbanBoard` owns `selectedCard`; selection is local UI state, not
  domain state (the board remains a projection).
- **Reuse web seams**: `nextCommandFor`, the `NextCommand` copy affordance, `TimelineEntry`/
  `useSpecActivity`, the `useFetchJSON` helper — no new state library or router.

## Open questions (resolved)

- **Default activity window for the summarize projection** → `24h` (`summarizeWindow` in
  `cmd/vector/summarize.go`). A post-action summary only needs the recent slice of the timeline.
- **Agent wire shape** → `{ "summary": "..." }` JSON, mirroring the standup agent. `summarize commit`
  parses that shape; empty/invalid prose writes nothing.
- **Per-status set of useful commands** → `usefulCommandsFor(card)` in
  `web/src/components/SpecDetailsDrawer/`: `/vector:link` only when the spec has no ticket, plus the
  legal status moves for the current status (in-progress → review / needs-attention; needs-attention
  → in-progress; review → in-progress; closed → archive). The primary next command is shown
  separately, so it is not duplicated here.
- **Whether `/vector:link` should also refresh the summary** → no. `link` produces no "what was done"
  prose, so it stays excluded (same rationale as `raw`/`sync`); a later transition regenerates the
  summary with the ticket present.
- **Go version target** → unchanged (read from `cli/go.mod`).
