---
name: "Vector: Close"
description: Close a finished spec — flip its card to `closed`. The explicit user step after `/vector:apply` (and any manual UAT) has the work in `review`. You never write Vector's state yourself; the binary owns the transition.
category: Workflow
tags: [vector, lifecycle, close]
---

Close a spec whose work is done. This is the **explicit user step** that `/vector:apply` stops
short of: apply implements and moves a card to `review`; `/vector:close` is the deliberate
"this is finished" flip to `closed`. **You never write Vector's state yourself** — you call
`vector spec close`, which flips board state (CLI-owns-writes).

**Input**: `$ARGUMENTS` (the spec id). If empty, run `vector spec list` and ask which to close.

## 1. Confirm the target

Read `.vector/specs/<id>/state.json` (or `vector spec list`). `closed` is reachable from
`draft`, `open`, `in-progress`, or `review` — the normal path is `review → closed` after apply
and any manual UAT passed. If the card is already `closed`/`archived`, say so and stop.

## 2. Close it

```bash
vector spec close <id> --json
```

The binary transitions the card to `closed`, stamps `closedAt`, and logs `spec.closed` +
`status.changed`. It enforces the state machine — an illegal move errors out; do not work around
it by editing `.vector/` by hand.

## 3. Summarize what was done (post-action)

Generate the per-spec "what was done" summary the board's details drawer shows. The binary
projects and persists; **you never write the summary yourself.** The path taken depends on
whether the activity window contains real work:

1. `vector spec summarize <id> --json` → `{ id, title, status, hasWork, templateSummary?, ... }`.
2. **If `hasWork == false`** (no `work.logged` events — typical for close without new apply):
   - If `templateSummary` is non-empty: pipe `{"summary":"<templateSummary>"}` directly to
     `vector spec summarize <id> commit --action close --summary-file -`.
     Log: `"summary: template (no work logged)"`. Skip spawning the agent.
   - If `templateSummary` is empty (defensive edge case): log
     `"no templateSummary received, skipping summary"` and continue without writing.
3. **If `hasWork == true`**: pass the full JSON to the `vector-summary-writer` subagent
   (Haiku); it returns `{ "summary": "<2–3 sentences>" }`. Pipe its JSON to
   `vector spec summarize <id> commit --action close --summary-file -`. Empty/invalid prose
   → nothing is written (not a gate); note it and move on. Log: `"summary: generated (Haiku)"`.

## 4. Report

Report the id and the transition (e.g. `review → closed`). If the spec maps to an OpenSpec
change (`openspec.change`), note that closing the **Vector card** is separate from archiving the
**OpenSpec change** — archive that with the repo's OpenSpec tooling if/when desired. The next
lifecycle step, if any, is `/vector:archive <id>` (only from `closed`).

## Notes

- Closing is deliberate and one-directional in spirit; from `closed` the only move is
  `archived` (via `/vector:archive`).
- If `vector` is not found, it isn't installed — tell the user; never edit `.vector/` by hand.
