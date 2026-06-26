---
name: "Vector: Apply"
description: Select the next work-item by tracked status + priority (the plus over OpenSpec), start it (open → in-progress), and implement the change — delegating to OpenSpec tooling when present, native otherwise. Autonomy is configurable via applyMode.
category: Workflow
tags: [vector, openspec, apply, lifecycle, implement]
---

Pick up work on the board and implement it. Unlike OpenSpec (where you name one change),
`/vector:apply` can **select** the next work-item itself, because Vector tracks **status +
priority on every card** — the differentiator. **You never write Vector's state yourself**:
you call the binary (`vector spec apply|status`), which flips board state (CLI-owns-writes).

**Input**: `$ARGUMENTS` (optional spec id). With an id → skip selection and apply that spec.
Without → select per `applyMode`.

> Token routing: selection + detection are light (cheap). The implementation step is the
> expensive work — that's where the model earns its cost.

## 1. Select the work-item (skip if an id was given)

Run `vector spec next --json`. It returns the recommended `id`, its `status`/`priority`, and
the repo's `applyMode`. Selection ranks **in-progress > needs-attention > review > open**, then
by priority — continue what's started before opening new work.

Behave per `applyMode`:

- **`auto`** → take the recommended pick and proceed without asking.
- **`ask`** (default) → propose the pick **with its reason** ("`<id>` is in-progress, highest
  priority") and confirm with `AskUserQuestion` before proceeding.
- **`always-ask`** → show the candidate list (`vector spec list`) and let the user choose.

If `next` reports nothing actionable (only draft/closed/archived remain), say so and stop —
there's nothing to apply.

## 2. Start the spec (transition by current status)

Read `.vector/specs/<id>/state.json`. Then:

- **`open`** → `vector spec apply <id> --json`. Transitions `open → in-progress`, stamps
  `startedAt`, logs `spec.applied` + `status.changed (trigger:apply)`. Now implement (step 3+).
- **`in-progress`** → a **continuation**. Do **not** call apply again; resume implementing.
- **`needs-attention`** → surface `needsAttention.reason` first and resolve the blocker. Once
  unblocked, `vector spec status <id> in-progress` and continue.
- **`review`** → implementation is already done; nothing to apply. Point the user at
  `/vector:close <id>`. Stop.

## 3. Detect the mode (delegate vs native)

OpenSpec mode is **"is this an OpenSpec project?"** — does `openspec/` with change structure
exist — **not** "is the `openspec` CLI on PATH". (Lesson from propose's bootstrap.)

- **OpenSpec project** → **delegate**: implement via the repo's OpenSpec tooling
  (`openspec apply <change>` / `opsx:apply`), following the change's `tasks.md`.
- **Not an OpenSpec project** → **native**: implement directly from the spec doc / proposal,
  working a `tasks.md` checklist if one exists.

The change name == the spec id (domain contract). Read it from the card's `openspec.change`.

## 4. Implement

Follow the change's `proposal.md`/`design.md`/`tasks.md` (or the spec doc in native mode).
Check off `tasks.md` items as you complete them so progress is visible. Respect the repo's own
conventions — Vector is **agnostic to the user's code** and imposes no architecture.

- Run the repo's test/build gate as you go; keep it green.
- **Do not auto-commit by default** — leave the working tree for the user to review (apply
  implements; it doesn't ship). Mention this in the report.
- If a question blocks you that you can't resolve, set
  `vector spec status <id> needs-attention --reason "<what's ambiguous>"` and stop with the
  question surfaced.

## 5. Log the work (enriches the standup trace)

After implementing and **before** transitioning, record what this run actually did so the
standup digest reflects "what was done", not just "how the status changed":

```
vector spec worklog <id> --files <comma,sep,files> --tasks "<comma,sep,tasks>" --note "<short note>"
```

- `--files`: the files you touched this run (from the working-tree diff).
- `--tasks`: the `tasks.md` / OpenSpec items you completed this run.
- `--note`: one short line on the substance (truncated to 280 chars).

This appends an **additive** `work.logged` event — it never mutates `state.json` and is **not a
gate**. Skip it only if the run touched nothing (e.g. a pure continuation that just transitions).

## 6. Finish — transition to review (or closed)

When implementation tasks are done, reuse the `sync` rule:

- All tasks done **or** only manual-QA tasks remain → `vector spec status <id> review`.
- If there is genuinely nothing left to verify → leave it for the user to `/vector:close`.

Never jump straight to `closed` from here — closing is an explicit user step (`/vector:close`).

## 7. Report

Report: the id and the transition made (e.g. `open → in-progress → review`), the mode
(delegate/native), tasks completed vs total, the gate result, whether the working tree has
uncommitted changes, and the next step (`/vector:close <id>` when in review).

## Notes

- `applyMode` lives in `.vector/config.json` (`auto` | `ask` | `always-ask`); default `ask`.
- An explicit `/vector:apply <id>` overrides selection but still respects the state machine.
- The binary enforces the state machine: illegal transitions error out — do not work around them
  by editing `.vector/` by hand.
- If `vector` is not found, it isn't installed — tell the user; never edit state manually.
