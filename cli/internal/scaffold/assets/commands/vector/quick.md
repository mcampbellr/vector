---
name: quick
description: Apply a small, low-risk change (refactor, rename, extracted helper, copy tweak, missing index, promoted file) in a single run — register it as a quick-win card, implement it directly, validate with the repo's gate, log the work, optionally commit, and land it in review. The Vector-native equivalent of /quick-win. No OpenSpec change, no Sonnet validator.
argument-hint: "\"<change description>\" [{ticket|spec-id}]"
user-invocable: true
category: Workflow
tags: [vector, quick-win, refactor, lifecycle]
allowed-tools: Read, Write, Edit, Grep, Glob, Bash(git *), Bash(vector *), Bash(go *), Bash(npm *), Bash(npx *), Bash(cargo *), Bash(ruff *), Bash(mypy *), Bash(pnpm *), Bash(yarn *), Agent, AskUserQuestion
---

Apply a **small, low-risk change in the same run**. Unlike `/vector:raw` → `/vector:propose` →
`/vector:apply` (full ceremony + an OpenSpec change), `/vector:quick` is for mechanical work — a
refactor, a symbol rename, an extracted helper, a copy tweak, a missing index, a promoted file.
It registers a board card **born `in-progress` and marked quick-win**, implements the change,
validates with the repo's lint/typecheck gate, logs the work, optionally commits (asking), and
moves the card to `review`. Closing stays explicit (`/vector:close`). It is the Vector-native
equivalent of the global `/quick-win` skill — **distributable** and **agnostic to the user's
repo**, leaving the board + `activity.jsonl` as the single record (no parallel `quick-wins.md`).

**You never write Vector's state yourself**: the card, the `quickWin` marker, the ticket/related
link, the worklog, and the status transition all go through the binary (CLI-owns-writes).

**Input**: `$ARGUMENTS` — a change description (quoted) followed by an optional `{ticket|spec-id}`
token (e.g. `/vector:quick "extract magic timeouts in attendance.service.ts" ACME-1421`).

> Token routing: the sanity-check, link resolution, and the implementation run in the **main
> loop**; refinement is delegated to a cheap **Haiku** agent (`vector-quick-refiner`). There is
> **no** Sonnet validator — the "validation" is the repo's lint/typecheck gate
> (`product/token-routing.md`).

## 0. Get repo context

Fetch the setup context from the binary before refining or implementing:

```bash
CONTEXT=$(vector context --json --repo-root "$REPO_ROOT" 2>/dev/null)
```

Extract `BUILD_CMD` ← `CONTEXT.buildCmd`, `TEST_CMD` ← `CONTEXT.testCmd`, `LINT_CMD` ←
`CONTEXT.lintCmd`, and the detected stack (`CONTEXT.intel.stack`). **Fallback when it fails**:
emit a one-line warning and discover the gate from the repo's manifests when you reach step 10.

## 1. Parse the input

Split `$ARGUMENTS` into the change description (`RAW_QW`) and the optional trailing
`{ticket|spec-id}` token. If `RAW_QW` is empty, ask for the change description with
`AskUserQuestion` and stop.

## 2. Confirm Vector is initialized

Read `.vector/config.json` for `specPath`. If it's missing, tell the user to run `vector init`
in the repo root and stop — without it there is no place to register the card.

## 3. Sanity-check: is this really a quick-win?

Before spending the refiner, screen `RAW_QW` for red flags and **escalate instead of
expanding**:

- New screen / page / modal / endpoint / feature, or any net-new user-visible behavior →
  recommend `/vector:raw` and stop.
- "Broken" / "regression" / "doesn't work" / a defect needing investigation →
  recommend `/vector:bug` and stop.
- Multiple unrelated changes bundled together → ask the user to split; suggest one `/vector:quick`
  per change, and stop.
- Schema / migration / new endpoint → recommend `/vector:raw`, **unless** it is a literal
  one-line change (e.g. adding a missing index).

When you escalate, **do not** invoke the refiner and **do not** create a card. Name the better
command and why.

## 4. Refine (Haiku) + scope-guard

Spawn the embedded **`vector-quick-refiner`** agent (Haiku, read-only) with:

```
RAW_QW: <the change description>
```

It returns the light brief (Optimized Title / Kebab-case Slug / Change Type / What Changes / Why
/ Files to Touch / Acceptance / Risks / Blocking Clarifying Questions / Non-Blocking Notes).

**Scope-guard** — if any of the following holds, escalate to `/vector:raw` and stop (do not
create a card):
- More than ~6 files to touch.
- A visible behavior change disguised as a refactor.
- More than 3 blocking questions (the change isn't actually small/clear).

## 5. Clarify (≤3 blocking questions)

If the brief lists blocking questions (cap 3), ask them with `AskUserQuestion` and fold the
answers into the brief. Zero blocking questions → skip.

## 6. Resolve the optional link (never blocks creation)

From the trailing arg (step 1) or by running `detectTicket` semantics over `RAW_QW`:
- **Ticket** → build the `--ticket` JSON (`{provider,key,url,auto}`); seed it on create.
- **Existing Vector spec id** → confirm via `vector spec list --json`; if it resolves, seed a
  relation `--related '[{"kind":"spec","ref":"<id>","source":"manual"}]'`.

Only when it resolves with confidence. Ambiguous → ask once or omit. **Never guess; never block
card creation on the link** — if the binary rejects the link, re-run create without it.

## 7. Register the card (`in-progress` + quick-win) via the binary

Write the brief as the card's doc and create it directly in `in-progress`, marked quick-win:

```bash
printf '%s' "$BRIEF" | vector spec create \
  --title "<Optimized Title>" \
  --id "<Kebab-case Slug>" \
  --status in-progress \
  --quick-win \
  [--ticket "$TICKET_JSON"] \
  [--related "$RELATED_JSON"] \
  --body-file - --json
```

Parse the JSON for `id`, `status`, and `specDoc`. Include `--ticket`/`--related` only when step 6
resolved them.

## 8. Implement the change (main loop)

Implement strictly to the brief's **Files to Touch** and **Acceptance**, with Read/Edit/Write,
respecting the repo's own conventions (Vector imposes no architecture). If the change **grows out
of scope** mid-implementation:

```bash
git restore <touched files>
vector spec status <id> needs-attention --reason "out of scope for a quick-win: use /vector:raw"
```

…and stop with the recommendation surfaced.

## 9. Validate with the repo's gate (scoped)

Run the **minimal** gate for the detected stack, scoped to the change — do **not** run the whole
suite:
- TypeScript → typecheck (`LINT_CMD`/`tsc --noEmit` on the affected package).
- Go → `go vet ./<changed-pkg>`.
- Python → `ruff` / `mypy` on the changed files.
- Rust → `cargo check`.

Use `BUILD_CMD`/`LINT_CMD` from step 0 when set; otherwise discover from the manifest. If the
gate fails, fix and re-run; if you can't get it green, set
`vector spec status <id> needs-attention --reason "<what's failing>"` and stop.

## 10. Log the work

Record what this run did so the standup digest reflects it (appends `work.logged`; not a gate):

```bash
vector spec worklog <id> --files <comma,sep,files> --tasks "<comma,sep,acceptance items>" \
  --note "<one short line on the substance>"
```

## 11. Commit (asking, every run)

Ask with `AskUserQuestion`: "Commit this quick-win?".
- **Yes** → atomic **Conventional Commit in English**, staging **only** the touched files + the
  card doc. Never `--no-verify`, never `--amend`, never `git add -A`. Use the brief's Change Type
  as the commit type (`refactor`/`chore`/`style`/`perf`/`docs`/`test`). Report the SHA.
- **No** → leave the working tree and report the uncommitted changes.

## 12. Transition to review

```bash
vector spec status <id> review
```

`in-progress → review`. Closing stays an explicit user step (`/vector:close`). Never jump to
`closed` here.

## 13. Record token routing

Only the refiner was routed to a cheaper tier; capture the saving (the binary derives cost/saved
and appends `agent.routed` — you never write the JSON):

```bash
vector spec route <id> --model haiku --baseline opus --task "refine quick-win" \
  --tokens-in <refiner-in> --tokens-out <refiner-out>
```

## 14. Post-action summary

Reuse the same pipeline as `/vector:apply` §7: run `vector spec summarize <id> --json`, pass the
**exact JSON** to the `vector-summary-writer` subagent (model: Haiku), shape-gate its
`{ "summary": … }`, and pipe it to
`vector spec summarize <id> commit --action apply --summary-file -`. The binary projects and
persists; the Haiku agent writes the prose. **You never write the summary yourself.** On a double
malformed response, skip and note it in the report.

## 15. Report

Report: the id, `quickWin: true`, the transition (`in-progress → review`), the ticket/related
link (or none), the commit SHA **or** "uncommitted changes left in the working tree", the gate
result, and the next step: `/vector:close <id>`.

## Notes — state discipline & token routing

- **CLI-owns-writes.** The card, the `quickWin` marker, the ticket/related link, the worklog, and
  every status transition are written **only** by the binary. Never edit `.vector/` by hand
  (`workflows/state-sync-discipline.md`). Illegal transitions error out — surface the error, don't
  work around it.
- **Token routing.** Refinement = Haiku (`vector-quick-refiner`); sanity-check, link resolution,
  and implementation = main loop; **no** Sonnet validator (`product/token-routing.md`). Only the
  refiner is recorded via `vector spec route`.
- **Escalate, don't expand.** A change that grows beyond a quick-win routes to `/vector:raw` (or
  `/vector:bug` for a defect) — it never grows silently.
- If `vector` is not found, it isn't installed — tell the user; never edit state manually.
