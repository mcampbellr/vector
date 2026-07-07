---
name: "Vector: Ship"
description: Land a reviewed spec as a pull request — commit the implementation (excluding OpenSpec artifacts and the spec doc), rebase onto the base branch, generate PR text, push, open a draft PR, and record it on the card. The natural successor to /vector:apply. You never write Vector's state yourself.
category: Workflow
tags: [vector, ship, pull-request, git, lifecycle]
---

Turn a spec that finished implementation into a pull request. `/vector:apply` leaves the card in
`review` with an uncommitted working tree — by design, apply implements, it does not ship.
`/vector:ship` is the next step: commit → rebase → PR-text → push → open a draft PR → record the
link on the card. **You never write Vector's state yourself** — you orchestrate git/`gh` and then
call `vector spec pr`, which records the PR (CLI-owns-writes). Git orchestration lives here, in the
command, **not** in the binary.

**Input**: `$ARGUMENTS` (optional spec id). With an id → ship that spec. Without → select the spec
in `review` (see §1).

> Token routing: this command is **orchestration only** — read context, run git/`gh`, write the PR
> prose, call the binary. No expensive reasoning; keep it tight.

## Hard rules (read first)

- **Only a spec in `review` can be shipped.** Any other status is refused with guidance (§2).
- **Never force-push.** No `--force`, no `--force-with-lease`. If a push is rejected, stop and
  report — never overwrite the remote.
- **Never `git add -A`.** Stage explicitly, excluding `EXCLUDE_GLOBS` and the spec's own doc (§5).
- **Recording a PR never changes the card's status.** Shipping does not close the spec —
  `/vector:close` does, and only after the PR merges (out of scope here).
- **Auth bootstrap is opt-in only.** Never source a repo `.envrc`/secret implicitly (§4). Never log
  its contents.
- **Secret scanning is the repo's job.** Ship relies on the excludes and delegates detection to the
  repo's own gate (gitleaks/CI/pre-commit). If a hook blocks the commit, report and stop.
- **If `vector` is not found**, it isn't installed — tell the user; never edit `.vector/` by hand.

## 0. Get repo context

Fetch the ship knobs and worktree layout from the binary in one call:

```bash
CONTEXT=$(vector context --json --repo-root "$REPO_ROOT" 2>/dev/null)
```

Extract from `CONTEXT.ship` (falls back to defaults when the repo configured no `ship` block —
`vector config set-ship` writes it):

- `BASE_BRANCH`    ← `CONTEXT.ship.baseBranch` (else `CONTEXT.worktree.baseBranch`, else `main`)
- `SHIP_MODE`      ← `CONTEXT.ship.mode` (`ask` | `auto`; default `ask`)
- `DRAFT`          ← `CONTEXT.ship.draft` (default `true`)
- `EXCLUDE_GLOBS`  ← `CONTEXT.ship.excludeGlobs` (always includes `openspec/`)
- `AUTH_BOOTSTRAP` ← `CONTEXT.ship.authBootstrap` (empty = never bootstrap)

Also resolve the spec's authored doc path (`SPEC_DOC`) from `.vector/specs/<id>/state.json`'s
`specDoc` field — it is excluded from the commit dynamically (§5), never a hardcoded path.

## 1. Select the spec (D11)

- **Id given** → use it.
- **No id** → list cards (`vector spec list --json`) and take the one spec in `review`.
  - **Exactly one** in `review` → that's the target.
  - **None** in `review` → stop: "nothing to ship (no spec in review)".
  - **Several** in `review` → show them and ask (`AskUserQuestion`) which to ship.

## 2. Precondition — status must be `review`

Read `.vector/specs/<id>/state.json`. If `status` is not `review`, refuse with actionable guidance:

- `open` / `in-progress` → "implement it first with `/vector:apply <id>`".
- `needs-attention` → "resolve the blocker, then `/vector:apply <id>`, then ship".
- `closed` / `archived` → "already finished; nothing to ship".
- `draft` → "formalize it with `/vector:propose <id>` first".

Do not proceed unless the status is exactly `review`.

## 3. Stale-tree warning (D12) — non-blocking

Check whether the working tree is behind the base branch:

```bash
git -C "$REPO_ROOT" fetch origin "$BASE_BRANCH" --quiet
```

If `origin/$BASE_BRANCH` has advanced far beyond the branch point, **warn** (the rebase in §6 will
replay onto it) but do **not** block — the warning is informational. If the fetch itself fails
(offline), note it and continue; the rebase step will surface a hard failure if it can't proceed.

## 4. Auth bootstrap (D4) — opt-in only

Only if `AUTH_BOOTSTRAP` is non-empty:

- If it's a path → source it in the shell used for git/`gh` (`. "$AUTH_BOOTSTRAP"`).
- If it's an SSH alias / remote hint → use it for the push remote.

**Never** source anything when `AUTH_BOOTSTRAP` is empty — no implicit `.envrc`. Never echo or log
the file's contents. If, after the opt-in bootstrap (or without one), `gh auth status` / a dry push
still can't authenticate, **stop and ask** — never guess credentials, never force.

## 5. Commit the implementation (D6) — explicit staging, never `git add -A`

Stage only the implementation, excluding the OpenSpec artifacts and the spec's own doc:

1. Compute the set to stage: the working-tree changes **minus** every glob in `EXCLUDE_GLOBS`
   (always `openspec/`) **minus** `SPEC_DOC` (the dynamic `specDoc` path from §0).
2. Stage them explicitly — enumerate paths or use pathspec excludes; **never** `git add -A`:
   ```bash
   git -C "$REPO_ROOT" add -- <explicit paths>            # or:
   git -C "$REPO_ROOT" add -- . ':(exclude)openspec/' ':(exclude)<SPEC_DOC>' <other EXCLUDE_GLOBS>
   ```
3. Verify nothing excluded slipped in: `git -C "$REPO_ROOT" diff --cached --name-only` must not list
   `openspec/…`, `SPEC_DOC`, or any `EXCLUDE_GLOBS` entry. If it does, unstage and fix.
4. Commit with a Conventional Commits subject (see §7 for the title contract):
   ```bash
   git -C "$REPO_ROOT" commit -m "<type: subject>"
   ```
   If a pre-commit hook (secret scan, lint) **blocks** the commit, report its output and stop — do
   not bypass it (no `--no-verify`).

## 6. Rebase onto the base branch (D9) — handle untracked collisions

```bash
git -C "$REPO_ROOT" rebase "origin/$BASE_BRANCH"
```

- **Clean** → continue.
- **Untracked-file collision** (rebase would overwrite an untracked file) → do **not** delete the
  user's untracked files. Surface the colliding paths and ask how to proceed (stash/move/abort);
  default to `git rebase --abort` and stop if unsure.
- **Merge conflict** → stop and report the conflicted paths; resolving them is the user's call, not
  an automatic one.

## 7. PR text (D5) — inline contract, no external skill

Generate the PR title and body here (do not depend on any personal/global skill — distribution
rule):

- **Title**: Conventional Commits, `< 70` chars, imperative, English.
  `feat: add /vector:ship command` — good. `Added ship stuff` — bad.
- **Body** (English), in this shape:
  - **Why** — one short paragraph: the problem this spec solves.
  - **What** — the key changes (bullets).
  - **Test plan** — a checklist of what was verified (build, tests, manual QA).
- Copy the body to the clipboard best-effort (`pbcopy`/`wl-copy` if present); a missing clipboard
  tool is not an error.

## 8. Push (never `--force`)

```bash
git -C "$REPO_ROOT" push -u origin HEAD
```

Never `--force` / `--force-with-lease`. If the push is **rejected** (non-fast-forward), stop and
report — the branch diverged; the user decides how to reconcile.

## 9. Idempotency (D8) — detect an existing PR before opening

Two-step check:

1. Read `pr` from `.vector/specs/<id>/state.json`. If present, that's a prior ship.
2. Confirm/repair against the remote:
   ```bash
   gh pr list --head "<branch>" --base "$BASE_BRANCH" --json url,number,state
   ```

- **A PR already exists for this branch** → **surface it** and skip opening a new one; go straight to
  §11 to re-record it (the binary is idempotent on the URL, so re-recording the same URL is a no-op).
- **Several PRs** for the branch → show them all and ask which is authoritative.
- **None** → proceed to §10.

## 10. Open the PR (draft per `DRAFT`, gated by `SHIP_MODE`)

Open as a draft when `DRAFT` is true:

```bash
gh pr create --base "$BASE_BRANCH" --head "<branch>" --title "<title>" --body "<body>" [--draft]
```

- `SHIP_MODE == ask` → confirm with `AskUserQuestion` (show title + base) **before** running
  `gh pr create`.
- `SHIP_MODE == auto` → open without prompting.

Capture the resulting PR `url` and `number` from `gh`'s output.

## 11. Record the PR on the card

```bash
vector spec pr <id> "<url>" [--number <n>] [--draft] --json
```

This persists `pr{url,number,draft,openedAt}` and logs `pr.opened` — **without** transitioning the
card's status. Parse the JSON (`changed:false` means the same URL was already recorded — the
idempotent re-record path from §9). This is the **only** write to Vector's state in this command.

## 12. Report

Report:

- The id and that the card **stayed in `review`** (shipping does not transition status).
- The commit subject, the base branch, and whether a rebase replayed onto new base commits.
- The PR URL + number, and whether it's a draft.
- Whether this was a fresh PR or a re-recorded existing one (idempotency).
- **Next step**: review/merge the PR, then `/vector:close <id>` once it lands (merging and closing
  are out of this command's scope).

## Notes

- **PR ≠ ticket.** The `ticket` slot (`/vector:link`) is the operational tracker; the `pr` slot is
  the shipped pull request. A spec can carry both; recording one never touches the other.
- **Config**: `vector config set-ship --base-branch <b> --mode ask|auto --draft true|false --exclude
  <globs> --auth-bootstrap <spec>` writes the `ship` block incrementally (each flag touches only its
  field). It's strictly opt-in — `vector init`/`update` never write it; the defaults (base = worktree
  base, mode = ask, draft = true, excludes = `openspec/` + the spec doc) apply until you set it.
- **Out of scope**: merging the PR, CI babysitting, closing the spec, non-GitHub providers, a bundled
  secret scanner, per-run `--draft`/mode overrides.
