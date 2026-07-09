---
name: "Vector: Fix"
description: Correct work already specified on the board — something a spec missed, a UAT finding, a small course-correction — reusing the refiner → clarity-gate → implementer discipline, wired into the state machine, token meter, and activity trace. CLI-owns-writes; embedded agents, no dependency on personal skills.
category: Workflow
tags: [vector, openspec, fix, correction, lifecycle]
---

Correct a spec **already on the board**. Unlike `/vector:apply` (which implements fresh work),
`/vector:fix` applies a **correction** to specified work — something a spec missed, a UAT
finding, a small course-correction — keeping the spec and code in sync. **You never write
Vector's state yourself**: lifecycle moves go through `vector spec status` (the LOCKED machine)
and the correction is recorded by `vector spec fix` — the binary is the sole state writer.

**Input**: `$ARGUMENTS` — a spec id (required) followed by the correction note
(e.g. `add-fix-command the design section forgot the needs-attention path`).

> Token routing: classification + clarity gating are cheap (Haiku refiner). The implementation
> is the expensive step (Sonnet implementer) — that's where the model earns its cost.

## 0. Get repo context

Fetch the setup context from the binary before refining or implementing:

```bash
CONTEXT=$(vector context --json --repo-root "$REPO_ROOT" 2>/dev/null)
```

Extract `BUILD_CMD` ← `CONTEXT.buildCmd`, `TEST_CMD` ← `CONTEXT.testCmd`, `LINT_CMD` ←
`CONTEXT.lintCmd`. **Fallback when it fails**: emit a one-line warning and discover build/lint/
test from the repo's manifests when you reach the gate.

## 1. Resolve and validate the card

Parse the id (first token) and the correction note (the rest). If no id is given, ask for one
and stop. Read `.vector/specs/<id>/state.json`:

- Status `open` / `in-progress` / `needs-attention` / `review` → **fixable**, continue.
- Status `draft` → not yet specified; route the user to `/vector:propose` then `/vector:apply`,
  and stop.
- Status `closed` / `archived` → out of scope for a fix; tell the user and stop.

If the spec id does not exist, say so and stop. Read the spec's artefacts paths
(`openspec/changes/<id>/{proposal,design,tasks}.md` and/or the native spec doc from
`state.json`'s `specDoc`).

## 2. Refine (Haiku) — classify, clarity-gate, scope-guard

Spawn the embedded **`vector-fix-refiner`** agent (Haiku, read-only) with:

```
SPEC_ID: <id>
RAW_NOTE: <the correction note>
SPEC_STATUS: <status>
ARTEFACTS: <abs paths to spec doc + proposal/design/tasks that exist>
```

It returns the 8-section brief, including a **Classification** (`spec-only|code-only|spec+code`)
and a **Clarity Verdict**:

- **`OutOfScope`** → this is a fresh feature / standalone bug, not a correction. Route the user
  to the destination the refiner named (`/vector:raw`, `/idea`, or `/vector:bug`) and **stop
  without writing anything** (no transition, no `spec fix`).
- **`NEEDS_CLARIFICATION`** → surface the refiner's Open Questions via `AskUserQuestion`. Fold
  the answers into the brief and re-evaluate. If still unresolved, stop with the questions
  surfaced — do not guess.
- **`CLEAR`** → proceed.

## 3. Entry transition (via `vector spec status`)

Move the card into work for the duration of the fix — **only when it isn't already there**:

- `open` → `vector spec status <id> in-progress`
- `review` → `vector spec status <id> in-progress`
- `in-progress` → already in work; fix in place (no transition).
- `needs-attention` → surface the existing `needsAttention.reason`, then
  `vector spec status <id> in-progress` to clear the flag before fixing.

The binary enforces the machine; an illegal transition errors out — surface it, never edit
`.vector/` by hand.

## 4. Implement (Sonnet)

Spawn the embedded **`vector-fix-implementer`** agent (Sonnet) with the brief plus context:

```
spec_id: <id>
classification: <spec-only|code-only|spec+code>
correction: <the correction, from the refiner brief>
artefacts_to_amend: <abs paths the refiner listed, or none>
files_to_touch: <candidate files the refiner listed, or none>
acceptance: <acceptance criteria from the brief>
test_plan: <test plan from the brief>
repo_root: <abs path>
build_cmd: <BUILD_CMD or "">
test_cmd: <TEST_CMD or "">
spec_doc: <abs path to native spec doc, when there are no OpenSpec artefacts>
```

Await its JSON result (`classification`, `artefacts_changed`, `files_changed`, `validation`,
`build_passed`, `test_passed`, `blocked`, `note`). **No auto-commit** — the working tree is left
for the dev to review.

## 5. Gate the result (the command is the gate)

The binary does **not** gate; the command does. Inspect the implementer's JSON:

- `"blocked": true` **or** `"validation": "fail"` → do **not** transition to `review`. Move the
  card to `needs-attention` with the structured contract — a **`--category`**
  (`dependency|env|decision|external|other`), a concrete one-liner **`--summary`** (what's pending),
  and an optional markdown **`--detail`** / `--detail-file` (how/who unblocks it + PR ref):

  ```
  vector spec status <id> needs-attention --category <cat> --summary "<what's pending>" [--detail "<md>"]
  ```

  The legacy `--reason "<text>"` still works (mutually exclusive with the structured flags,
  auto-migrated to `category=other`), but prefer the structured contract.

  Then still record the fix (step 6) with `--validation-result fail` so the trace is honest, and
  stop — surface the blocker to the user.
- `"validation": "pass"` → proceed.

## 6. Record the correction (`vector spec fix`)

Append the typed `spec.fixed` event — it records the correction and **never transitions**:

```bash
vector spec fix <id> \
  --classification <spec-only|code-only|spec+code> \
  --artifacts <comma list of proposal,design,tasks amended> \
  --files <comma,sep,code,files> \
  --validation-result <pass|fail> --json
```

Omit `--artifacts` / `--files` when the corresponding list is empty. `--artifacts` takes the
canonical names `proposal,design,tasks` and tolerates any casing and an optional `.md` suffix
(e.g. `Design.md,tasks` is accepted); the persisted state always holds the canonical names.

## 7. Exit transition + trace

On a `pass` result, return the card to where review can pick it up:

```
vector spec status <id> review
```

Then record the run for the standup/meter:

```bash
vector spec worklog <id> --files <comma,sep,files> --tasks "<what was corrected>" --note "<short note>"
vector spec route <id> --task fix-refine --model haiku --baseline opus ...   # the cheap classification step
```

(Use `route` to credit the cheap refiner step to the Token Savings Meter, per
`product/token-routing.md`.)

## 8. Summarize (Haiku) — post-action drawer

Mirror the apply/standup pipeline: the binary projects, a cheap Haiku agent writes the prose,
the binary persists it. **You never write the summary yourself.**

1. `vector spec summarize <id> --json` → projection JSON.
2. Pass that exact JSON to the **`vector-summary-writer`** agent (Haiku) → `{ "summary": "..." }`.
3. Shape-gate the response (parseable JSON, non-empty `summary`); on a second failure, skip and
   note it.
4. `vector spec summarize <id> commit --action fix --summary-file -` with the agent's JSON.

## 9. Report

Report: the id, the classification, the entry/exit transitions made (e.g.
`review → in-progress → review`), the artefacts/files touched, the gate result, and that the
working tree has uncommitted changes for review.

- **Validated** → next step is `/vector:close <id>`.
- **Blocked / validation failed** → surface the `needs-attention` reason instead of "ready for
  review"; next step is to resolve the blocker, then `/vector:fix <id>` again.

## Notes

- `vector spec fix` only records `spec.fixed`; it never transitions — lifecycle moves go through
  `vector spec status` (separation of concerns, single writer).
- The two agents (`vector-fix-refiner`, `vector-fix-implementer`) are embedded and seeded by
  `vector init`/`update` — never assume the user's personal `/fix` skill exists.
- An `OutOfScope` correction writes nothing: it is a routing hint, not a board action.
- The binary enforces the state machine: illegal transitions error out — never work around them
  by editing `.vector/` by hand.
