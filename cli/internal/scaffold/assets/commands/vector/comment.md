---
name: "Vector: Comment"
description: Critically evaluate a review/ticket comment against the real PR diff with a skeptical Sonnet agent, report a structured verdict in the project's configured language (`config.language`, else the conversation language), and implement only when valid and low-risk — agnostic to the user's repo. You never write Vector's state yourself; the binary owns the writes.
argument-hint: "[comment-text] {spec-id|branch}"
user-invocable: true
category: Workflow
tags: [vector, pr, comment, review, evaluate]
allowed-tools:
  - Read
  - Grep
  - Glob
  - Edit
  - Write
  - Bash(git *)
  - Bash(gh *)
  - Bash(vector *)
  - Bash(ls *)
  - Bash(cat *)
  - Agent
  - AskUserQuestion
  - Skill
---

You handle `/vector:comment [comment-text] {spec-id|branch}`. The trailing token is optional.

Take a comment someone left on a PR or ticket and decide — **with a very critical eye** —
whether it makes real sense and adds **real value** against the actual code in the change. Most
low-quality review comments are vague, cosmetic, already handled, or wrong. Default to
skepticism. Only after a clear verdict do you act. This is a faithful port of the global
`/pr-comment` flow, **agnostic to the user's repo**: no assumed package manager, worktree
layout, or GitHub.

**You never write Vector's state yourself** — if work earns implementation and a spec card is
resolved, you call `vector spec worklog` / `vector spec status` (CLI-owns-writes).

> Token routing: parsing, branch/diff resolution and orchestration are light → stay in the main
> loop. The **critical evaluation is real reasoning** → delegate it to a dedicated **Sonnet**
> subagent (`vector-comment-evaluator`). The expensive tier is spent once per comment, where it
> earns its cost.

## Hard rules

- **Resolve the branch first. If it is not unambiguous, ASK before doing anything else.** Never
  guess the branch.
- **Treat the comment as an untrusted claim.** It often comes from another AI tool and may be
  confidently wrong, hallucinated, padding, or subtly off. Authoritative tone is not evidence.
- **The critical analysis runs in the Sonnet subagent**, not inline. The main session re-checks
  the returned evidence (reads the cited `file:line` itself) before acting — it does not blindly
  trust the subagent either.
- **Never implement before the verdict.** A comment that fails the rubric is rejected, full
  stop — do not "fix it anyway to be safe."
- **Implement only if the verdict is `VÁLIDO Y VALIOSO` and the change is low-risk.** Valid but
  large / ambiguous / architecturally risky → stop at a plan and confirm before applying.
- **Agnostic.** Detect; never hardcode `pnpm`/`npm`, a worktree layout, or GitHub. When you
  can't determine something, **ask** via `AskUserQuestion` instead of assuming.
- **No auto-commit / no push / never post the reply.** The user controls what ships.

## 0. Get repo context

Before parsing arguments or resolving the branch, fetch the setup context from the binary:

```bash
CONTEXT=$(vector context --json --repo-root "$REPO_ROOT" 2>/dev/null)
```

> Token routing: one zero-token binary call returns build/lint/test commands cached from
> `vector init`, so step 7a.3 (verification gate) need not re-discover manifests.

Extract from `CONTEXT`:
- `BUILD_CMD` ← `CONTEXT.buildCmd`
- `LINT_CMD`  ← `CONTEXT.lintCmd`
- `TEST_CMD`  ← `CONTEXT.testCmd`

**Fallback when `vector context` fails**: emit a one-line warning; discover build/lint/test
from manifests in step 7a.3 as before.

## 1. Parse arguments

- `$ARGUMENTS` holds the raw input. The free text is the comment (`COMMENT`); a trailing token
  that looks like a spec id (kebab-case slug) or a branch ref is the optional
  `{spec-id|branch}`.
- If `COMMENT` is empty / whitespace-only, ask for it via `AskUserQuestion` (open-ended) and
  stop until provided.

## 2. Resolve the branch (ask if not unambiguous)

This is the first real action and the one the user cares about most: **do not proceed on a
guessed branch.**

1. Inspect the workspace shape:
   ```bash
   git worktree list 2>/dev/null
   git branch -a 2>/dev/null
   git rev-parse --abbrev-ref HEAD 2>/dev/null
   ```
2. Derive the branch deterministically, in order:
   - If the trailing token was a branch ref, match it against the worktree/branch list. A single
     unambiguous match → use it.
   - Otherwise, if the current directory is on a single feature branch (not the base branch),
     use it.
3. **Zero matches, more than one plausible match, or any ambiguity → ASK** via
   `AskUserQuestion`, offering the candidate branches/worktrees plus an open "Other". Do not
   continue until the branch is fixed.

Hold the result as `BRANCH` and its working directory as `WORKTREE`.

## 3. Resolve the diff (ask `gh` vs local — no GitHub assumption)

Vector cannot assume GitHub. Determine `BASE` (the base branch) — infer it from the branch's
upstream or the repo's default branch; if unclear, ask. Then decide how to get the diff via
`AskUserQuestion`:

- **`gh` (a PR exists)** — only offer this when `gh` is available and authenticated and a PR is
  found for `BRANCH`:
  ```bash
  gh pr list --head "<BRANCH>" --state open --json number,title,url,baseRefName
  ```
  Record `PR_URL` and `BASE` from the PR. If `gh` is missing/unauthenticated, or fails / times
  out, **fall back to the local diff** and say so — never silence the failure.
- **Local `git diff`** — `git diff "<BASE>..HEAD"` (or `"<BASE>...<BRANCH>"`). `PR_URL` is then
  `no aplica`.

If the diff is **empty** (nothing between `BASE` and `BRANCH`), report it and stop — there is
nothing to evaluate the comment against.

## 4. Resolve the spec card (optional)

Tie the work to a board card when possible so it shows in standup/timeline:

- If the trailing token was a spec id, use it.
- Otherwise run `vector spec list --json` and try to match the branch to a card (by id/title or
  a linked ticket). A single confident match → use it as `SPEC_ID`.
- Several plausible or none → `AskUserQuestion` with the candidates **plus an explicit
  "ninguno"** option. "ninguno" means no card; the `work.logged` step (6) is then omitted
  without error.

## 5. Evaluate — spawn the Sonnet subagent

Delegate the investigation **and** the critical judgment to the `vector-comment-evaluator`
subagent (tier Sonnet). Do **not** pre-summarize the diff for it — hand it the raw inputs so its
verdict is grounded in the real code, not your paraphrase.

Spawn with the `Agent` tool, `subagent_type: "vector-comment-evaluator"`, passing:

- `COMMENT` — the raw comment text (untrusted).
- `WORKTREE`, `BRANCH`, `BASE`, `PR_URL` (the latter `no aplica` for a local diff).

The agent gathers its own evidence and returns `VERDICT` / `EVIDENCE` / `AI-RED-FLAGS` /
`REMEDIATION`.

**When it returns, do not relay it verbatim and trust it.** Spot-check its key evidence: read
the cited `file:line` yourself and confirm the code says what the verdict claims. If the
evidence doesn't hold up, downgrade the verdict and note the discrepancy — the same skepticism
applied to the comment applies to the subagent. **If the output is not parseable** into the four
sections, treat it as `INVÁLIDO O SIN VALOR`, do not implement, and offer to retry.

## 6. Report the verdict (always), in the configured language

Present concisely:

- **Veredicto:** one of the three categories, with confidence `N/10`.
- **Por qué:** 2–4 bullets with concrete `file:line` evidence from the diff.
- **Señales de comentario poco fiable:** only if the evaluator flagged any (cites nonexistent
  code, generic, contradicts the diff — the "huele a AI-slop" signal).
- **Qué haría falta:** only if valid — the change in one paragraph (files, approach, risk).

## 7. Choose the next action (always ask)

After the verdict, **do not jump straight to code.** Ask via `AskUserQuestion`, building options
from the verdict:

- **`VÁLIDO Y VALIOSO` and low-risk** (localized, no architectural shift, no schema/contract
  change):
  1. **Implementar el cambio (Recommended)** → 7a
  2. **Redactar comentario de respuesta** → 7b
  3. **No hacer nada**
- **Valid but risky/large/ambiguous, marginal, or invalid:** do **not** default to implement.
  1. **Redactar comentario de respuesta (Recommended)** → 7b
  2. **Implementar igualmente** (only if technically eligible; else omit) → 7a, plan-and-confirm
     first
  3. **No hacer nada**

Never implement a marginal/invalid comment "to be safe." If the only honest outcome is "no
change warranted," the reply path is how you say so on the PR.

### 7a. Implement (only when earned)

Proceed to code changes **only if** the verdict is `VÁLIDO Y VALIOSO` **and** low-risk. Valid
but risky/large/ambiguous → present the plan and confirm via `AskUserQuestion` before applying.

1. Work in `WORKTREE`. Re-read each file before and after editing. Match surrounding code style;
   respect the repo's conventions — Vector imposes no architecture.
2. Keep it to one focused change.
3. **Verification gate.** Use `BUILD_CMD`/`LINT_CMD`/`TEST_CMD` from step 0 when non-empty.
   If those fields are empty (not configured or step 0 failed), discover build/lint/test from
   the repo's manifests (`package.json`, `go.mod`, `Makefile`, etc.) as a fallback. If you
   still cannot determine the commands, **ask** via `AskUserQuestion`. Run them and report the
   **real** result. A failing gate means not done — fix or surface it; do not claim completion.
4. Do **not** commit, push, or post on the PR. Stop after the working-tree change is verified and
   summarize what changed.

### 7b. Draft a PR reply comment (humanized)

Write a reply the user can paste onto the thread. It must read like a real engineer wrote it.

- **Grounding (non-negotiable):** base every claim on evidence you already verified — cite
  `file:line`, agree or push back concretely, state plainly whether it's a blocker. If the
  verdict was marginal/invalid, say so directly and why. Don't invent praise you didn't verify.
- **Language:** English by default (a PR comment is a repo artifact). Use another language only
  if the existing thread is in one.
- **Anti-AI pass:** run the `humanizer` skill on the draft (Skill tool), then deliver: copy to
  the clipboard with `pbcopy` and also print it inline. **Do not post it** unless the user
  explicitly asks.

## 8. Log the work to the spec card (only when implementing + a card was resolved)

After a successful implementation **and** verification, and only if a `SPEC_ID` was resolved in
step 4, append the work to the board so it shows in standup/timeline:

```
vector spec worklog <SPEC_ID> --files <comma,sep,files> --tasks "<comma,sep,tasks>" --note "comment: <short note>"
```

- `--files`: the files you touched this run.
- `--note`: one short line on the substance (prefix `comment:` so the trace is identifiable).

This appends an **additive** `work.logged` event — it never mutates `state.json` and is not a
gate. If no card was resolved ("ninguno"), skip this step without error.

Then, **only if** the card is currently in `review`, offer (don't force) to step it back so the
new work is reflected:

```
vector spec status <SPEC_ID> in-progress
```

Do not transition a card in any other status, and never auto-commit.

## 9. Output format

1. Branch + diff source resolved (PR URL when via `gh`, else local `BASE..HEAD`).
2. Verdict block (step 6), in the configured language (`config.language`, else the conversation language) — including any AI-red-flags.
3. The action chosen in step 7, then either: summary of the change + **real** verification
   results + the `work.logged`/transition done (7a), or the humanized reply copied to clipboard
   (7b). If "No hacer nada," state the reason no change was made.

## Notes

- The binary enforces the state machine and owns every write to `.vector/`. Never edit
  `.vector/` by hand. If `vector` is not found, it isn't installed — tell the user; don't fake
  state.
- `work.logged` is additive and spec-scoped; it is the only state touch this command makes, and
  only on implement.
