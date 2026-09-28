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

**Input**: `$ARGUMENTS` (the spec id, optionally followed by why it is being closed, e.g.
`login-v1 duplicate of login-v2`). If no id is given, run `vector spec list` and ask which to
close.

## 1. Confirm the target

Read `.vector/specs/<id>/state.json` (or `vector spec list --json`). `closed` is reachable from
`open`, `in-progress`, or `review` — the normal path is `review → closed` after apply
and any manual UAT passed. If the card is already `closed`/`archived`, say so and stop.

If the card is `needs-attention`, it cannot be closed directly: resolve it first with
`vector spec status <id> in-progress` (or `review`) — the binary's error lists the legal next
statuses with their commands. Ask the user which one applies; never guess past a blocker.

## 2. Decide the resolution

Every close records **why** the spec is closed (`--resolution`):

| Resolution | When | Counts toward epic progress |
|------------|------|-----------------------------|
| `done` (default) | The work shipped. | yes — counted as done |
| `obsolete` | No longer needed ("ya no aplica", "no hace falta"). | no — dropped |
| `duplicate` | Same work as another spec ("es duplicado de X"). | no — dropped |
| `superseded` | Replaced by another spec/approach ("lo reemplazó X"). | no — dropped |

Infer it from `$ARGUMENTS` and the conversation: finished/merged/UAT passed → `done`; the user
says obsolete/duplicate/replaced (in any language) → that resolution. Put the reason or the
other spec's id in `--note` (e.g. `--note "duplicate of login-v2"`). **Ask only when it is
genuinely unclear** — e.g. the user says "close it" on a card that never reached `review` and gave
no reason. Then use `AskUserQuestion` with the four options (`done` first as the recommended
default); do not ask when the work is clearly finished.

## 3. Close it

```bash
vector spec close <id> [--resolution done|obsolete|duplicate|superseded] [--note "..."] --json
```

`--resolution` defaults to `done`; the JSON echoes `resolution` (and `resolutionNote` when set).
The binary transitions the card to `closed`, stamps `closedAt`, persists the resolution on the
spec, and logs `spec.closed` (with the resolution) + `status.changed`. It enforces the state
machine — an illegal move errors out with the legal targets; do not work around it by editing
`.vector/` by hand.

## 4. Summarize what was done (post-action)

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

## 5. Report

Report the id, the transition (e.g. `review → closed`) and the resolution (e.g. `duplicate —
duplicate of login-v2`). If the spec maps to an OpenSpec
change (`openspec.change`), note that closing the **Vector card** is separate from archiving the
**OpenSpec change** — archive that with the repo's OpenSpec tooling if/when desired. The next
lifecycle step, if any, is `/vector:archive <id>` (only from `closed`).

## Notes

- Closing is deliberate and one-directional in spirit; from `closed` the only move is
  `archived` (via `/vector:archive`). There is no reopen: pick the resolution carefully (it
  can still be overridden once, when archiving).
- If `vector` is not found, it isn't installed — tell the user; never edit `.vector/` by hand.
