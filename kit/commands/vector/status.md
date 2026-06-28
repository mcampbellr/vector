---
name: "Vector: Status"
description: Move a spec to a target status if the move is legal (the generic transition). Use it to flag/unflag needs-attention or to step a card back/forward. You never write Vector's state yourself; the binary enforces the state machine.
category: Workflow
tags: [vector, lifecycle, status, transition]
---

Move a spec to a target status. This is the **generic transition** for moves the dedicated
commands don't cover — e.g. flagging `needs-attention`, returning `review → in-progress`, or
resuming `needs-attention → in-progress`. **You never write Vector's state yourself** — you call
`vector spec status`, which validates and flips the board state (CLI-owns-writes).

**Input**: `$ARGUMENTS` = `<id> <status> [reason]`. Statuses: `open`, `in-progress`,
`needs-attention`, `review`, `closed`, `archived`. If id or status is missing, ask (show
`vector spec list`).

## 1. Resolve the move

Read `.vector/specs/<id>/state.json` for the current status. The binary enforces the LOCKED
state machine; legal moves include: `in-progress ↔ review`, `* → needs-attention`,
`needs-attention → in-progress|review`, and the closing moves. `draft → open` is **not** here —
that's `/vector:propose`. Entering `needs-attention` **requires a reason**.

## 2. Apply the transition

```bash
vector spec status <id> <status> [--reason "<why>"] --json
```

`--reason` is required when `<status>` is `needs-attention` (it populates `needsAttention.reason`,
surfaced on the card). The binary logs `status.changed` (`trigger:command`). An illegal move
errors out — surface the error; do not edit `.vector/` by hand.

## 3. Summarize what was done (post-action)

Generate the per-spec "what was done" summary the board's details drawer shows. The binary
projects and persists; **you never write the summary yourself.** The path taken depends on
whether the activity window contains real work. Note: the close/archive safeguard that
preserves a prior summary when there is no new work does **not** apply here — for a plain
status transition the template is always committed.

1. `vector spec summarize <id> --json` → `{ id, title, status, hasWork, templateSummary?, ... }`.
2. **If `hasWork == false`** (no `work.logged` events — typical for a status change):
   - If `templateSummary` is non-empty: pipe `{"summary":"<templateSummary>"}` directly to
     `vector spec summarize <id> commit --action status --summary-file -`.
     Log: `"summary: template (no work logged)"`. Skip spawning the agent.
   - If `templateSummary` is empty (defensive edge case): log
     `"no templateSummary received, skipping summary"` and continue without writing.
3. **If `hasWork == true`**: pass the full JSON to the `vector-summary-writer` subagent
   (Haiku); it returns `{ "summary": "<2–3 sentences>" }`. Pipe its JSON to
   `vector spec summarize <id> commit --action status --summary-file -`. Empty/invalid prose
   → nothing is written (not a gate); note it and move on. Log: `"summary: generated (Haiku)"`.

## 4. Report

Report the id and the transition (e.g. `in-progress → needs-attention`, with the reason). If the
target is a dedicated step, point at it instead: `closed` → prefer `/vector:close`; `open` from a
draft → `/vector:propose`; `archived` → `/vector:archive`.

## Notes

- Dedicated transitions have their own commands: `/vector:apply` (open → in-progress),
  `/vector:close`, `/vector:archive`. Use `/vector:status` for everything else legal.
- If `vector` is not found, it isn't installed — tell the user; never edit `.vector/` by hand.
