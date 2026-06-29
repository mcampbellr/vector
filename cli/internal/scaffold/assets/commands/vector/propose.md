---
name: "Vector: Propose"
description: Formalize a draft spec into an OpenSpec change (proposal/design/tasks) and move the card draft → open. Delegates to the repo's OpenSpec tooling when present; light native fallback otherwise.
category: Workflow
tags: [vector, openspec, propose, lifecycle]
---

Formalize a Vector card in `draft` (a spec authored by `/vector:raw`) into an OpenSpec
**change**, then move the card to `open`. **You never write Vector's state yourself** —
you create the change artifacts and then call `vector spec propose`, which flips the board
state (CLI-owns-writes).

**Input**: `$ARGUMENTS` (the spec id). If empty, ask which draft to propose (`vector spec list`).

> Token routing: orchestration only. Generating artifacts is light; don't over-think it.

## Adapter: delegate to OpenSpec, or native fallback

- **OpenSpec present** (the repo has the `openspec` CLI on PATH, or `opsx:propose` /
  `openspec-propose` skills) → **delegate**: run that tooling to create
  `openspec/changes/<id>/{proposal,design,tasks}.md`. Zero new tooling for the user.
- **OpenSpec absent** → **native fallback**: write the three artifacts yourself from the spec
  doc (proposal ← the spec; design/tasks ← actionable stubs). Minimal — **not** an OpenSpec
  clone (no spec-delta model, no catalog).

## Hard rules

- **Only a `draft` can be proposed.** If the card is `open`/other, report and stop (the binary
  enforces this too). Never re-propose silently.
- **Never overwrite an existing change without asking.**
- **Never create/move git worktrees or branches.** You only write the change artifacts where resolved.
- **Stay scoped:** proposing ≠ implementing (`/vector:apply`).

## Steps

1. **Read the id** (`$ARGUMENTS`). Confirm the card exists and is `draft`:
   `vector spec propose <id> --dry-run --json` (it validates without writing; reports `wouldBe:open`,
   or an error if not draft / not found). Read the card's `specDoc` from
   `.vector/specs/<id>/state.json`.

2. **Resolve the change location.** Read `.vector/config.json`: in bare+worktree layouts the change
   goes under the worktree of `proposeBranch` (else `branch`); for simple repos it's
   `openspec/changes/<id>/` at the repo root. If several worktrees are candidates and none is set,
   ask the user (`AskUserQuestion`) and persist the choice. Hold the target dir as `CHANGE_DIR`.

3. **Detect the mode.** Is there OpenSpec tooling? Check `openspec` on PATH and the
   `opsx:propose` / `openspec-propose` skills. Set `MODE = delegate | native`.

4. **If `CHANGE_DIR` already exists**, ask (`AskUserQuestion`) overwrite / keep. On keep, skip
   generation and go to step 6 (just flip the state).

5. **Generate the change artifacts** into `CHANGE_DIR`:
   - `delegate`: invoke the OpenSpec propose tooling (skill or CLI), passing the spec id and the
     `specDoc` path as the source. Let it author `proposal.md` / `design.md` / `tasks.md`.
   - `native`: write `proposal.md` (a clear proposal derived from the spec doc — Why / What
     changes / Scope), `design.md` (key decisions + architecture from the spec, or a `TODO` stub),
     and `tasks.md` (an actionable checklist derived from the spec's success criteria / deliverables).
   Note which of the three you actually created.

6. **Flip the board state** — call the binary:
   ```bash
   vector spec propose <id> --change <id> --artifacts <created,list> --json
   ```
   It transitions `draft → open`, records `openspec{change,artifacts}`, and logs
   `spec.proposed` + `status.changed`. Parse the JSON.
   `--artifacts` takes the canonical names `proposal,design,tasks` and tolerates any casing
   and an optional `.md` suffix (e.g. `Proposal.md,design` is accepted).

7. **Summarize what was done (post-action).** Generate the per-spec "what was done" summary the
   board's details drawer shows. The binary projects and persists; **you never write the summary
   yourself.** Note: `propose` never logs `work.logged` events, so `hasWork` will always be
   `false` and the binary will always provide a `templateSummary` — the agent spawn is skipped.
   - `vector spec summarize <id> --json` → `{ id, title, status, hasWork, templateSummary?, ... }`.
   - **`hasWork` will be `false`** for propose (no `work.logged` in the transition).
     - If `templateSummary` is non-empty: pipe `{"summary":"<templateSummary>"}` directly to
       `vector spec summarize <id> commit --action propose --summary-file -`.
       Log: `"summary: template (no work logged)"`. Skip spawning the agent.
     - If `templateSummary` is empty (defensive edge case): log
       `"no templateSummary received, skipping summary"` and continue without writing.
   - If `hasWork` is unexpectedly `true`: pass the full JSON to the `vector-summary-writer`
     subagent (Haiku); it returns `{ "summary": "<2–3 sentences>" }`. Pipe its JSON to
     `vector spec summarize <id> commit --action propose --summary-file -`. Empty/invalid prose
     → nothing is written (not a gate); note it and move on. Log: `"summary: generated (Haiku)"`.

8. **Report**: the id, `draft → open`, the change directory, the artifacts created, the mode
   (delegate/native), and the next step: **`/vector:apply <id>`** to implement.

## Notes

- `open` = the change exists; work has **not** started (so no `startedAt`). That's `/vector:apply`.
- The id is the OpenSpec change name (per the domain contract).
- If `vector` is not found, it isn't installed — tell the user; do not edit `.vector/` by hand.
