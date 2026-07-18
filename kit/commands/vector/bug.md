---
name: "Vector: Bug"
description: Turn a raw bug report into a complete, validated Vector spec and register it as a draft card — deducing the bug's root cause from git history and persisting it as a queryable relatedTo[] relation. The bug-framed counterpart of /vector:raw. You never write Vector's state yourself; the binary owns the writes.
argument-hint: "[bug-report] {spec-id|branch|file}"
user-invocable: true
category: Workflow
tags: [vector, spec, bug, traceability, cause]
allowed-tools:
  - Read
  - Grep
  - Glob
  - Bash(git *)
  - Bash(vector *)
  - Agent
  - AskUserQuestion
  - Skill
---

Turn the user's raw bug report into a **complete, validated spec** and register it as a
Vector card in `draft` status — **plus** trace the bug to the prior work that caused it
(`relatedTo[]`), so the board records *why this bug appeared*. This is the bug-framed
counterpart of `/vector:raw`: it authors and registers a `draft` card and **stops there**.
It does **not** create the OpenSpec change (that's `/vector:propose`) or implement the fix
(that's `/vector:apply`).

**Input**: `$ARGUMENTS` — the raw bug report, optionally followed by a `{spec-id|branch|file}`
token that scopes cause deduction. If empty, ask for the report and stop.

**You never write Vector's state files yourself** — the `vector` binary is the sole writer. You
author the **spec doc** (a repo artifact), deduce the cause, and then call the binary to
register the card with its relations; the binary writes the doc and creates the draft card.

> Token routing: cause deduction / parsing / resolution run in the **main loop** (cheap); the
> refiner runs on **Haiku**; the validator on **Sonnet**; composition in the main loop. Do not
> run everything on the expensive tier (`product/token-routing.md`).

## Hard rules

- **Never implement the fix.** Stop after the spec is authored, validated, and registered as `draft`.
- **No inference of product intent.** When the expected behavior is unclear, ask — do not invent.
- **Never guess the cause.** Deduce from git; on ambiguity / multiple candidates / low confidence /
  no match → ask. A hallucinated `relatedTo` link is worse than none.
- **Cite, don't guess.** Paths, versions, commits, endpoints are verified against the repo or
  marked `TBD — ver Open questions`.
- **Spec language follows the project** (detect from existing specs; default English). The
  conversation stays in the user's language. Slugs / paths / git artifacts are English kebab-case.

## Steps

0. **Get repo context** — fetch the setup context from the binary in one call before any other
   work:

   ```bash
   CONTEXT=$(vector context --json --repo-root "$REPO_ROOT" 2>/dev/null)
   ```

   > Token routing: one zero-token binary call returns examplePath + language + build commands
   > so later steps need not re-derive them from manifests or globs.

   Extract from `CONTEXT`:
   - `SPEC_EXAMPLE_PATH` ← `CONTEXT.examplePath`
   - `SPEC_LANGUAGE` ← `CONTEXT.language`
   - `WT_LAYOUT` ← `CONTEXT.worktree.layout` (true when the repo declares a bare+worktree layout —
     the `[branch]` placeholder is present in `spec-path`/`changes-path`)
   - `WT_ROOT` ← `CONTEXT.worktree.root` (literal prefix before `[branch]`, e.g. `code`)
   - `WT_BASE` ← `CONTEXT.worktree.baseBranch` (fork point for new worktrees, default `main`)
   - `WT_PREFIX` ← `CONTEXT.worktree.branchPrefix` (feature-branch prefix, default `feat/`)

   **Fallback when `vector context` fails**: emit a one-line warning; `SPEC_EXAMPLE_PATH` and
   `SPEC_LANGUAGE` will be resolved in step 4 using the original glob+detect approach. Treat
   `WT_LAYOUT` as `false` (the worktree step in step 9 stays inert) when context is unavailable.

1. **Parse the input.** Split `$ARGUMENTS` into `RAW_BUG` (the report) and an optional trailing
   `{spec-id|branch|file}` token (`SCOPE`). If `RAW_BUG` is empty, ask the user for the report via
   `AskUserQuestion` and stop until you have it.

2. **Confirm the repo is initialized.** The spec doc location and ticket defaults come from
   `.vector/config.json` (written by `vector init`, migrated from `.project-structure`). If it is
   missing, run `vector init` first (or tell the user to), so the spec lands in the repo's
   convention instead of the `.vector/` fallback. Note the resolved `specPath` for the report.
   Before running it, apply `.claude/agents/_shared/root-anchoring-guardrail.md`: if a `.vector/`
   already exists at an ancestor directory, that store is the base — never `vector init` a nested
   one, and never pass `--force` to silence the guard.

3. **Deduce the root cause** (main loop — cheap). The goal: map the bug to the prior work that
   caused it, as `relatedTo[]` candidates. Be conservative — **infer, then ask.**
   1. **Find the suspect surface.** From `RAW_BUG` (and `SCOPE` if given) identify the files /
      symbols implicated. If none are named, do a light Grep for the error text / symbol.
   2. **Run git** scoped to those files: `git blame -L <range> <file>`, `git log -S "<symbol>" --`,
      `git log --grep "<term>" --`, to surface suspect commits. Report progress. On a huge repo or
      a slow history, cap the search and offer to continue **without** `relatedTo[]`.
   3. **Map each suspect commit** to a cause:
      - a **Vector spec** — resolve the commit's change/spec against `vector spec list --json`
        (match the OpenSpec `change` name or the spec `id`). → `{kind: spec, ref: <spec-id>}`.
      - a **ticket** — a tracker key in the commit trailer/message (e.g. `ACME-12`,
        `owner/repo#7`). → `{kind: ticket, ref: <provider:key or key>}`.
   4. **Decide per candidate:**
      - **Unique + high confidence** → seed it with `source: blame`.
      - **Ambiguous / multiple / low confidence / no match** → `AskUserQuestion` listing the
        candidates **plus** a "none" option **plus** "enter manually". A user-entered or
        user-confirmed relation carries `source: manual`. Never auto-pick among rivals.
   5. **No `git` / non-git repo / files absent** → skip deduction with a one-line notice and author
      the bug without relations. Deduction failing never blocks authoring.

   Collect the resolved relations as `RELATED_JSON` — a JSON array of
   `{"kind":"spec|ticket","ref":"<ref>","source":"blame|manual"}`. Leave it unset if none.

4. **Confirm example spec and language.** Use the values from step 0 when available:
   - `SPEC_EXAMPLE_PATH` — already set from `CONTEXT.examplePath` (step 0); if empty or step 0
     failed, glob the configured `specPath` directory and common locations
     (`docs/specs/**`, `openspec/changes/*/spec.md`, `specs/**`). Hold the path as
     `SPEC_EXAMPLE_PATH`, else `no example yet`.
   - `SPEC_LANGUAGE` — already set from `CONTEXT.language` (step 0); if empty, detect from the
     example spec found above (default English). State it in one line.

5. **Refine** — invoke the `vector-bug-refiner` subagent (**model: haiku**, read-only) with:
   `RAW_BUG`, the deduced causes from step 3 as context (`DEDUCED_CAUSES`), and `SPEC_EXAMPLE_PATH`.
   It returns an 8-section brief (problem / expected / actual / reproduction / acceptance / test
   plan / risks / open questions), surfacing ambiguity and inventing nothing. Call it `BRIEF`.

6. **Clarify** — resolve blocking ambiguity from `BRIEF` (chiefly the expected behavior and
   reproduction) by batching ≤5 questions via `AskUserQuestion`. Keep iterating until the bug is
   unambiguous, or the user says stop (then mark the gaps `TBD — ver Open questions`).

7. **Resolve metadata and compose the spec**:

   a. **Derive the title and id** from `BRIEF` (the bug-refiner proposes them). The kebab-case
      id must carry a **`fix-`** prefix (e.g. `fix-login-redirect-loop`). Confirm with the user
      if step 6 left ambiguity in the name. Hold as `SPEC_TITLE` and `SPEC_ID`.

   b. **Priority** only if the report clearly implies one (a crash / data loss ⇒ higher); else
      omit (defaults to `normal`).

   c. **Invoke `vector-spec-composer`** (**model: sonnet**, may write a file) with:
      - `BRIEF` (full `vector-bug-refiner` output from step 5, which includes the 8-section
        brief: problem / expected / actual / reproduction / acceptance / test plan / risks /
        open questions)
      - `CLARIFICATIONS` (all Q&A pairs from step 6, in order; the expected-vs-actual and
        reproduction details flow here so the composer places them in §8 and §11)
      - `TEMPLATE_PATH`: absolute path to `.claude/vector/spec-template.md`
      - `SPEC_EXAMPLE_PATH` (from step 4, or `no example yet`)
      - `SPEC_TITLE`, `SPEC_ID`, `SPEC_LANGUAGE`
      - `OUTPUT_PATH`: `.vector/tmp/<SPEC_ID>/spec.md`

      The subagent writes the complete spec (20 sections, bug-framed) to `OUTPUT_PATH`,
      placing expected-vs-actual in §8, reproduction steps in §11, and the deduced cause(s)
      from `RELATED_JSON` in §4. It returns a confirmation with the path and the number of
      `TBD` markers. Hold the path as `SPEC_PATH`.
      **The main loop does not retain the spec text in its context — only the path.**

8. **Validate** — invoke the `vector-spec-validator` subagent (**model: sonnet**, read-only),
   passing the composed spec text, `SPEC_EXAMPLE_PATH`, the template path, and the 20-section
   checklist. React to the verdict:
   - `PASS` → continue.
   - `PASS_WITH_WARNINGS` → show warnings; ask address-now / keep. Fix and re-validate if addressing.
   - `BLOCK` → fix the required items (ask the user if a fix needs info). Re-validate. **Cap 3
     cycles**; if still blocked, surface the report verbatim and stop **without** registering.

9. **Register the draft card**.

   a. **Ensure the spec's worktree exists** (bare+worktree layouts only). When `WT_LAYOUT` is
      true, the binary resolves the spec doc under `<WT_ROOT>/<SPEC_ID>/…`, so that per-spec git
      worktree must exist **before** `vector spec create` writes the doc — otherwise the doc lands
      as a loose, untracked `<WT_ROOT>/<SPEC_ID>/` directory (the bug this command guards against).
      Run, from `REPO_ROOT`:

      ```bash
      if [ "$WT_LAYOUT" = "true" ]; then
        WT_PATH="$WT_ROOT/$SPEC_ID"                      # e.g. code/<slug>
        WT_BRANCH="$WT_PREFIX$SPEC_ID"                   # e.g. feat/<slug>
        if git -C "$REPO_ROOT" worktree list --porcelain | grep -Fqx "worktree $REPO_ROOT/$WT_PATH"; then
          echo "reusing existing worktree $WT_PATH"      # idempotent: never recreate
        else
          git -C "$REPO_ROOT" worktree add "$WT_PATH" -b "$WT_BRANCH" "$WT_BASE"
        fi
      fi
      ```

      - **Inert on non-worktree repos**: `WT_LAYOUT` false → skip entirely; behavior identical to
        today (e.g. Vector's own repo, `.vector/specs/<slug>/`).
      - **Idempotent**: if `git worktree list` already lists `<WT_ROOT>/<SPEC_ID>`, reuse it — no
        recreation, no error.
      - **Conflict → abort, never auto-delete**: if `git worktree add` fails (a loose
        non-worktree directory or a stale `<WT_PREFIX><SPEC_ID>` branch already occupies the path),
        surface git's error **verbatim** plus the manual fix (remove/relocate the loose
        `<WT_ROOT>/<SPEC_ID>/` stub, or `git worktree add` it by hand) and **stop without
        registering the card**. Do not delete or overwrite the user's files. Cleanup of pre-existing
        stubs is out of scope (the user's responsibility).

   b. **Create the card** — pass the validated spec to the binary via file path. The binary reads
      the doc from `SPEC_PATH`, creates the card in `draft` (writing the doc inside the worktree
      created in 9a, so it stays tracked on the `<WT_PREFIX><SPEC_ID>` branch), and persists
      `relatedTo[]`:

   ```bash
   vector spec create \
     --title "<SPEC_TITLE>" \
     --id "<SPEC_ID>" \
     [--repo "<repo-name>"] \
     [--priority "<priority>"] \
     [--related "$RELATED_JSON"] \
     --status draft \
     --body-file "$SPEC_PATH" --json
   ```

   Include `--related` only when step 3 produced `RELATED_JSON`. Parse the JSON result for `id`,
   `status`, `specDoc`, and the registered `relatedTo`. **Never block creation on relations**: an
   invalid `--related` degrades (the binary creates the card without relations and warns on
   stderr) — report it and continue; do not lose a valid card.

   For a relation discovered **after** creation (e.g. the user confirms one later), add it with the
   dedicated subcommand (idempotent, never a status change):

   ```bash
   vector spec relate "fix-<slug>" --kind spec --ref "<cause-spec-id>" --source manual --json
   ```

10. **Record the token routing** (feeds the board's Token Savings Meter). For **each** cheap-agent
    step you ran, call the binary once so the saving is captured — you never write the JSON
    yourself; the binary derives cost/saved and appends the `agent.routed` event:

    ```bash
    # The refiner ran on Haiku instead of the Opus baseline:
    vector spec route "<SPEC_ID>" --model haiku  --baseline opus --task "refine bug" \
      --tokens-in <refiner-in> --tokens-out <refiner-out>
    # The compositor ran on Sonnet instead of the Opus baseline:
    vector spec route "<SPEC_ID>" --model sonnet --baseline opus --task "compose spec" \
      --tokens-in <composer-in> --tokens-out <composer-out>
    # The validator ran on Sonnet instead of the Opus baseline:
    vector spec route "<SPEC_ID>" --model sonnet --baseline opus --task "validate spec" \
      --tokens-in <validator-in> --tokens-out <validator-out>
    ```

    **Precision**: omit `--precision` (defaults to `estimated`) unless the harness exposed the
    exact token counts (e.g. via a tool-result metadata field or environment variable). If the
    harness did provide real counts, pass `--precision actual` — this marks the meter as exact
    and removes the "Estimated" badge on the board. Never pass `--precision actual` for numbers
    you derived yourself; an honest estimate is more trustworthy than a false claim of precision.

    Use the actual subagent token usage when you have it; otherwise pass your best estimate, rounded
    to the nearest thousand (the meter is an estimate by design). Skip a route you did not run.
    `--baseline` defaults to `opus`; keep it explicit.

11. **Report** (in `config.language`, else the conversation language): the card id, `status: draft`,
    the `specDoc` path, the **registered relations** (`relatedTo[]` — what caused the bug, and
    whether each was `blame`-deduced or `manual`), and the validator verdict. If deduction found no
    cause, say so plainly. Tell the user the next step: **`/vector:propose`** creates the `fix-…`
    OpenSpec change (proposal/design/tasks) and moves the card `draft → open`; **`/vector:apply`**
    implements the fix. Note the token routing (refiner = Haiku, validator = Sonnet, orchestration =
    main loop) and that **the binary owns every state write** (CLI-owns-writes).

## Notes

- `draft` = spec authored, **no OpenSpec change yet**. The change is created at `/vector:propose`;
  the fix starts at `/vector:apply`.
- The `fix-<slug>` id is reused as the OpenSpec change name when proposed/applied.
- Re-running on the same report creates a **second distinct** `draft` card by design (no dedup —
  each run is a new bug). The user archives/closes duplicates.
- `relatedTo[]` kinds in V1 are **`spec`** and **`ticket`** only; the suspect commit is an inference
  signal, not a stored kind.
- Keep the spec honest: mark unknowns as `TBD — ver Open questions`, never invent detail or a cause.
- If `vector` is not found, the binary isn't installed — tell the user to install it; do not write
  `.vector/` or the spec doc by hand as a substitute for the binary's registration.
