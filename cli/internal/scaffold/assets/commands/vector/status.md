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

## 3. Report

Report the id and the transition (e.g. `in-progress → needs-attention`, with the reason). If the
target is a dedicated step, point at it instead: `closed` → prefer `/vector:close`; `open` from a
draft → `/vector:propose`; `archived` → `/vector:archive`.

## Notes

- Dedicated transitions have their own commands: `/vector:apply` (open → in-progress),
  `/vector:close`, `/vector:archive`. Use `/vector:status` for everything else legal.
- If `vector` is not found, it isn't installed — tell the user; never edit `.vector/` by hand.
