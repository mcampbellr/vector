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

## 0. Get repo context

Fetch the setup context from the binary before selecting or implementing:

```bash
CONTEXT=$(vector context --json --repo-root "$REPO_ROOT" 2>/dev/null)
```

> Token routing: one zero-token binary call returns buildCmd/testCmd/lintCmd cached from
> `vector init`, so step 4 (build/test gate) need not re-discover manifests on each run.

Extract from `CONTEXT`:
- `BUILD_CMD` ← `CONTEXT.buildCmd`
- `TEST_CMD`  ← `CONTEXT.testCmd`
- `LINT_CMD`  ← `CONTEXT.lintCmd`

**Fallback when `vector context` fails**: emit a one-line warning; discover build/lint/test
from the repo's manifests in step 4 as before.

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

## 3a. Evalúa el tier del modelo

Read `applyModel` from the JSON returned by `vector spec next --json` in §1 (already in
memory). In a direct continuation (§2 `in-progress` path, where §1 was skipped), read
`.vector/config.json` and call `vector context --json` to obtain the value; if unavailable,
default to `"opus"`.

**Dispatch by `applyModel` value:**

| Value | Action |
|---|---|
| `""` or `"opus"` | Tier = **Opus**. Continue to §4 and implement inline (current behavior, no change). |
| `"sonnet"` | Tier = **Sonnet**. Skip §4 — delegate immediately to `vector-apply-impl`. |
| `"conditional"` | Evaluate the five mechanical signals below. Route to Sonnet if **all five** are mechanical; route to Opus if any is architectural or ambiguous. |

**Five mechanical signals (for `conditional` only):**

| Dimension | Mechanical | Architectural / ambiguous |
|---|---|---|
| **File scope** | ≤ 5 distinct paths/file names mentioned in `proposal.md`/`tasks.md`/`design.md` | > 5 files, or scope not quantifiable from artefacts |
| **API/HTTP contracts** | No changes to endpoints, response bodies, or routes | Adds, modifies, or removes endpoints |
| **Domain types** | Does not touch `SpecState`, `Config`, event types, or state machine | Modifies domain structures or state machine |
| **Dependencies** | No new imports or external libraries | Adds a lib or external dependency |
| **Open decisions** | `design.md` has no pending alternatives or trade-offs | `design.md` contains open decisions or listed alternatives |

**Fallback rule:** any signal that is ambiguous (artefacts absent, `tasks.md` empty or not
found, scope unreadable) counts as **architectural** — default to Opus. Conservative routing
is always safer than a false downgrade.

**When tier = Sonnet**, dispatch to `vector-apply-impl` as a **fresh agent** (not a fork) with
the structured brief below. Do not implement inline; do not load the artefact contents into
your own context. The change paths are already available from §3.

```
spec_id: <id>
proposal: <abs_path>/openspec/changes/<id>/proposal.md
design:   <abs_path>/openspec/changes/<id>/design.md
tasks:    <abs_path>/openspec/changes/<id>/tasks.md
repo_root: <abs_path>
build_cmd: <BUILD_CMD from §0, or "">
test_cmd:  <TEST_CMD from §0, or "">
mode: delegate | native
openspec_change: <id>   # only in delegate mode
```

In native mode without `tasks.md`, omit the `tasks` field and include instead:
`spec_doc: <abs_path>/.vector/specs/<id>/spec.md`

Await the agent's JSON result (shape: `files_changed`, `tasks_completed`, `tasks_pending`,
`build_passed`, `test_passed`, `blocked`, `note`). Consume this JSON in §5 and §6a in place
of your own run's artifacts. If `"blocked": true`, treat the `note` as the blocker reason for
§6b. If both `build_passed` and `test_passed` are false and `files_changed` is empty, the
agent encountered a non-recoverable error — surface `note` to the user and stop.

## 4. Implement

> **If the tier was assigned to Sonnet in §3a, skip this section: the implementation has
> already been delegated to the `vector-apply-impl` subagent.** Proceed directly to §5,
> consuming the agent's JSON result.

Follow the change's `proposal.md`/`design.md`/`tasks.md` (or the spec doc in native mode).
Check off `tasks.md` items as you complete them so progress is visible. Respect the repo's own
conventions — Vector is **agnostic to the user's code** and imposes no architecture.

- **Build/test gate**: use `BUILD_CMD`/`TEST_CMD`/`LINT_CMD` from step 0 when non-empty.
  If those fields are empty (not configured or step 0 failed), discover the commands from the
  repo's manifests (`package.json`, `go.mod`, `Makefile`, `pyproject.toml`, etc.) and run them.
  If you still cannot determine the commands, **ask** via `AskUserQuestion`. Keep the gate green.
- **Do not auto-commit by default** — leave the working tree for the user to review (apply
  implements; it doesn't ship). Mention this in the report.
- If a question blocks you that you can't resolve, flag it with the structured needs-attention
  contract and stop with the question surfaced:
  ```
  vector spec status <id> needs-attention --category decision --summary "<the open question>" [--detail "<md context>"]
  ```

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

## 6. Finish — detect external blockers, then transition

Before transitioning, run one detection sub-step. Then transition: to `needs-attention` if an
external-dependency blocker is present, otherwise to `review` (or leave it for `/vector:close`)
exactly as before.

### 6a. Detect an external-dependency blocker

Your implementation may compile and pass the (mocked) suite yet still be **gated on something you
cannot resolve yourself** — a third-party credential, an external identifier/`api_name` not yet
confirmed, data another team owns. Inspect **your own run's artifacts** (the working-tree diff,
the `tasks.md`/acceptance items, and any outbound artifact the run produced) and judge whether any
of these three signals is present (any one is enough):

1. **Runtime-governing TODO/placeholder.** The run left a `TODO(<ticket>)`/placeholder in
   **production** code that governs runtime behavior (not cosmetic, not test-only) and depends on
   an external datum/credential/identifier not yet provided (e.g. the `api_names` of Zoho left as
   `TODO(MH-1582)` pending settings-read credentials).
2. **Outbound request artifact.** The run produced an artifact whose purpose is to **ask a
   human/another team for something** — a draft ticket comment "please send the credentials",
   "ask X for the api_names", a request-for-input note.
3. **Mock-only acceptance item.** A `tasks.md`/acceptance item is satisfiable **only against
   mocks** and is explicitly marked pending a real datum/credential.

**Deterministic false-positive guard — these never trigger `needs-attention`:**

- `TODO`/`FIXME` in test-only files (`*_test.go`, `*.test.*`, dirs `test`/`tests`/`__tests__`).
- Cosmetic comments (refactor, rename, typo, tidy-up) with no external dependency.
- A `TODO` that deliberately defers to another already-tracked card/ticket (intentional deferral)
  — your judgment, **not** a lookup against `.vector/specs/`.

This is **agent judgment guided by the signals above**, not hardcoded regex; the only mechanical
filter is the test-only/cosmetic guard. Inspect and judge — don't grep-and-flag.

> **Auditable heuristic.** This differs from the §4 hard-stop: there the implementation **stopped**
> because something was ambiguous; here the implementation **finished** but is gated on an external
> dependency. Both route to `needs-attention` (structured `--category`/`--summary`/`--detail`); this
> one fires at the close of a completed run, automatically.

### 6b. Transition

- **Blocker present** (any signal, not filtered by the guard) → route to `needs-attention`,
  **automatically and independent of `applyMode`** — it's a board-integrity safeguard, not a
  workflow choice, so do **not** ask for confirmation even under `ask`/`always-ask`:

  ```
  vector spec status <id> needs-attention \
    --category <dependency|env|decision|external|other> \
    --summary "<one-liner: what's pending>" \
    --detail "<markdown: how/who unblocks it + open PR ref>"
  ```

  Pick the **`--category`** that fits the blocker (`dependency` for a third-party
  credential/service, `external` for data another team owns, `decision` for an open product/design
  call, `env` for missing local/CI setup, else `other`; defaults to `other`). The **`--summary`**
  is the one-liner the card shows — concrete and short. The optional **`--detail`** (markdown, or
  `--detail-file <path>`) carries the actionable body: **what's pending** + **how/who unblocks it** +
  **the open PR ref**. Lead with the runtime-governing blocker; emit **one** transition per run
  (enumerate multiple blockers within the detail). Never leak a secret value — describe the missing
  thing without its value (summary + detail are committed in `state.json` and shown on the
  board/standup, so treat them as public). The legacy `--reason "<text>"` still works (mutually
  exclusive with the structured flags, auto-migrated to `category=other`), but prefer the structured
  contract. Example: `--category dependency --summary "Zoho CRM api_names pending settings-read
  credentials" --detail "Unblock by providing creds to fill \`TODO(MH-1582)\`; PR #367 open"`.

  Edge cases: if the card is **already** in `needs-attention` (e.g. flagged in §4), refresh the
  reason with the live blocker — the binary validates the transition. If `tasks.md` is absent or
  the working tree is unchanged, skip the signals you can't evaluate and judge the rest. If the
  binary rejects the transition, surface its error — do not mask it, do not edit `.vector/` by hand.

- **No blocker** → behave exactly as before, reusing the `sync` rule:
  - All tasks done **or** only manual-QA tasks remain → `vector spec status <id> review`.
  - If there is genuinely nothing left to verify → leave it for the user to `/vector:close`.

Never jump straight to `closed` from here — closing is an explicit user step (`/vector:close`).

## 7. Summarize what was done (post-action)

After the transition, generate the per-spec "what was done" summary the board's details drawer
shows. This mirrors the standup pipeline: the binary projects and persists, a cheap **Haiku**
agent writes the prose. **You never write the summary yourself.**

1. Run `vector spec summarize <id> --json` — it returns `{ id, title, status, ticket?,
   priorSummary?, events[] }` for the recent window.
2. Pass that **exact JSON** to the `vector-summary-writer` subagent (model: Haiku). It returns
   `{ "summary": "<2–3 sentences>" }`. Do not summarize yourself — the agent's whole job is the prose.
3. Validate the summary (shape-gate): check that the response is parseable JSON and `summary` is a
   non-empty string.
   - **Valid (attempt 1):** proceed to step 4.
   - **Invalid (attempt 1):** re-spawn `vector-summary-writer` (same Haiku tier) with the same
     projection JSON plus a correction directive prepended to the prompt (above the JSON):
     ```
     The previous attempt returned malformed or invalid JSON.
     Return ONLY a valid JSON object matching exactly:
     {"summary": "<2–3 sentences>"}
     No preface, no code fences, no trailing text.
     ```
     Check again:
     - **Valid (attempt 2):** proceed to step 4.
     - **Invalid (attempt 2):** skip — do **not** pipe to the binary; note in §8:
       `summary skipped: subagent returned invalid JSON twice`. Move on.
4. Pipe its JSON to `vector spec summarize <id> commit --action apply --summary-file -`. The binary
   validates and writes `.vector/local/summaries.json` (gitignored). Empty/invalid prose → nothing
   is written (not a gate); note it and move on.

> Token routing: the projection is free (binary) and the prose is cheap bounded work → the Haiku
> `vector-summary-writer` agent (`product/token-routing.md`). The binary never calls an LLM.

## 8. Report

Report: the id and the transition made (e.g. `open → in-progress → review`), the mode
(delegate/native), tasks completed vs total, the gate result, whether the working tree has
uncommitted changes, and the next step.

- **Routed to `review`** → next step is `/vector:close <id>` (ready for review).
- **Routed to `needs-attention`** (external blocker, §6) → surface the blocker and its `reason`
  (what's pending + unblock path + PR ref) **instead of** "ready for review"; the next step is to
  provide the missing dependency, then `/vector:apply <id>` to resume. Form:
  `external blocker → needs-attention: <reason>`.
- **Summary skipped** (§7 step 3 double failure) → append a brief note at the end of the report:
  `summary skipped: subagent returned invalid JSON twice`. Apply still completes normally — this
  is not a gate.

## Notes

- `applyMode` lives in `.vector/config.json` (`auto` | `ask` | `always-ask`); default `ask`.
- `applyModel` lives in `.vector/config.json` (`opus` | `sonnet` | `conditional`); default `opus`
  (opt-in — `vector init`/`update` never writes this field). Set it manually to opt into token
  routing. The resolved value is always exposed by `vector spec next --json` as `"applyModel"`.
- An explicit `/vector:apply <id>` overrides selection but still respects the state machine.
- The binary enforces the state machine: illegal transitions error out — do not work around them
  by editing `.vector/` by hand.
- If `vector` is not found, it isn't installed — tell the user; never edit state manually.
