---
name: "Vector: Archive"
description: Archive a closed spec — move its card to `archived` (out of the active board into the archived view). Only a closed spec can be archived. You never write Vector's state yourself; the binary owns the transition.
category: Workflow
tags: [vector, lifecycle, archive]
---

Archive a finished, closed spec so it leaves the active board for the archived view. **Only a
`closed` spec can be archived** (the state machine allows `closed → archived` and nothing else
into `archived`). **You never write Vector's state yourself** — you call `vector spec archive`,
which flips board state (CLI-owns-writes).

**Input**: `$ARGUMENTS` (the spec id). If empty, run `vector spec list` and ask which to archive.

## 1. Confirm it is closed

Read `.vector/specs/<id>/state.json`. If the card is **not** `closed`, stop and tell the user to
`/vector:close <id>` first — archiving a non-closed card is illegal and the binary will reject it.
If it is already `archived`, say so and stop.

## 2. Archive it

```bash
vector spec archive <id> --json
```

The binary transitions the card to `archived`, stamps `archivedAt`, and logs `spec.archived` +
`status.changed`. Archived cards live in a separate view, not the active columns.

## 3. Summarize what was done (post-action)

Generate the per-spec "what was done" summary the board's details drawer shows. The binary
projects and persists; **you never write the summary yourself.** The path taken depends on
whether the activity window contains real work:

1. `vector spec summarize <id> --json` → `{ id, title, status, hasWork, templateSummary?, ... }`.
2. **If `hasWork == false`** (no `work.logged` events — typical for archive):
   - If `templateSummary` is non-empty: pipe `{"summary":"<templateSummary>"}` directly to
     `vector spec summarize <id> commit --action archive --summary-file -`.
     Log: `"summary: template (no work logged)"`. Skip spawning the agent.
   - If `templateSummary` is empty (defensive edge case): log
     `"no templateSummary received, skipping summary"` and continue without writing.
3. **If `hasWork == true`**: pass the full JSON to the `vector-summary-writer` subagent
   (Haiku); it returns `{ "summary": "<2–3 sentences>" }`. Pipe its JSON to
   `vector spec summarize <id> commit --action archive --summary-file -`. Empty/invalid prose
   → nothing is written (not a gate); note it and move on. Log: `"summary: generated (Haiku)"`.

## 4. Report

Report the id and the transition (`closed → archived`). If the spec maps to an OpenSpec change,
note that archiving the **Vector card** is separate from archiving the **OpenSpec change**
(`openspec archive <change>`), which the user runs with the repo's OpenSpec tooling if desired.

## Notes

- `archived` is terminal in the Vector state machine — there is no transition out of it.
- If `vector` is not found, it isn't installed — tell the user; never edit `.vector/` by hand.
