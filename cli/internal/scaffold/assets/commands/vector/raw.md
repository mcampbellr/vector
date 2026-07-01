---
name: "Vector: Raw"
description: Turn a raw idea into a complete, validated 20-section Vector spec and register it on the board as a draft (no OpenSpec change yet). As powerful as /idea, self-contained in Vector.
category: Workflow
tags: [vector, spec, capture, idea]
---

Turn the user's raw idea into a **complete, validated spec** and register it as a
Vector card in `draft` status. This is Vector's own spec-authoring engine — as
powerful as `/idea`, but self-contained (no dependency on `/idea`).

**Input**: `$ARGUMENTS` (the raw idea). If empty, use the user's latest message.

**You never write Vector's state files yourself** — the `vector` binary is the sole
writer. You author the **spec doc** (a repo artifact) and then call the binary to
register the card; the binary writes the doc to the repo's configured location and
creates the draft card. See `.claude/CLAUDE.md` distribution notes if `vector` is missing.

> Token routing: refiner runs on **Haiku** (cheap), validator on **Sonnet**, composition
> in the main loop. Do not run everything on the expensive tier.

## Hard rules

- **Never implement code.** Stop after the spec is authored, validated, and registered.
- **No inference of product intent.** When behavior is unclear, ask — do not invent.
- **Cite, don't guess.** Paths, versions, endpoints must be verified against the repo or
  marked `TBD — ver Open questions`.
- **Spec language follows the project** (detect from existing specs; default English).
  The conversation stays in the user's language. Slugs/paths/git artifacts are English kebab-case.

## Steps

1. **Read the raw idea** (`$ARGUMENTS`, or the latest message). Hold as `RAW_IDEA`.

2. **Confirm the repo is initialized.** The spec doc location comes from `.vector/config.json`
   (written by `vector init`, migrated from `.project-structure`). If it is missing, run
   `vector init` first (or tell the user to), so the spec lands in the repo's convention
   instead of the `.vector/` fallback. Note the resolved `specPath` for the report.

3. **Get repo context** — fetch the setup context from the binary in one call:

   ```bash
   CONTEXT=$(vector context --json --repo-root "$REPO_ROOT" 2>/dev/null)
   ```

   > Token routing: one zero-token binary call replaces a manual glob of specPath and
   > ad-hoc language detection — the binary reads config.json, globs the spec store,
   > and detects manifests in parallel, returning the result as structured JSON.

   Extract from `CONTEXT`:
   - `SPEC_EXAMPLE_PATH` ← `CONTEXT.examplePath` (the first spec doc found, sorted lexicographically)
   - `SPEC_LANGUAGE` ← `CONTEXT.language` (the configured prose language, if set)
   - `WT_LAYOUT` ← `CONTEXT.worktree.layout` (true when the repo declares a bare+worktree layout —
     the `[branch]` placeholder is present in `spec-path`/`changes-path`)
   - `WT_ROOT` ← `CONTEXT.worktree.root` (literal prefix before `[branch]`, e.g. `code`)
   - `WT_BASE` ← `CONTEXT.worktree.baseBranch` (fork point for new worktrees, default `main`)
   - `WT_PREFIX` ← `CONTEXT.worktree.branchPrefix` (feature-branch prefix, default `feat/`)

   **Fallback when `vector context` fails** (binary not in PATH or exits 1): emit a one-line
   warning to stderr, then fall back to the previous behavior — glob the configured `specPath`
   directory and common locations (`docs/specs/**`, `openspec/changes/*/spec.md`, `specs/**`)
   to find an example spec; detect `SPEC_LANGUAGE` from that example (default English). Treat
   `WT_LAYOUT` as `false` (the worktree step in step 9 stays inert) when context is unavailable.

   If `SPEC_LANGUAGE` is empty after the context call (not configured), detect it from the
   example spec found above (default English).

4. **Detect ticket and language** — pass `RAW_IDEA` to the binary to resolve both in one call:

   ```bash
   DETECT_JSON=$(echo "$RAW_IDEA" | vector detect-ticket --repo-root "$REPO_ROOT" --json)
   ```

   - **Language** (`SPEC_LANGUAGE`): use `DETECT_JSON.language` if non-empty and
     `SPEC_LANGUAGE` was not already set in step 3; otherwise keep the value from step 3.

   Hold `DETECT_JSON` — step 7 reads `DETECT_JSON.ticket` without re-invoking the binary.

5. **Refine** — invoke the `vector-spec-refiner` subagent (**model: haiku**, read-only) with:
   `RAW_IDEA`, `SPEC_EXAMPLE_PATH`, and the template path `.claude/vector/spec-template.md`.
   It returns a structured brief surfacing ambiguity per the 20-section checklist. Call it `BRIEF`.

6. **Clarify** — walk every dimension of the 20-section checklist. For each unresolved one,
   batch ≤5 questions via `AskUserQuestion`. **No total cap** — keep iterating until every
   dimension has concrete content, or the user says stop (then mark `TBD — ver Open questions`).

7. **Resolve metadata and compose the spec**:

   a. **Derive the title and id** from `BRIEF` (the refiner proposes them in
      `## Optimized Change Title` and `## Kebab-case Change Name`). Confirm with the user
      if step 6 left ambiguity in the name. Hold as `SPEC_TITLE` and `SPEC_ID`.

   b. **Seed the ticket link** from `DETECT_JSON` resolved in step 4 — the binary already
      applied the 4 detection tiers; do not re-invoke:
      - `DETECT_JSON.ticket` non-null → `TICKET_JSON = DETECT_JSON.ticket`
      - `DETECT_JSON.ticket` null → leave `TICKET_JSON` unset (no match, or ambiguous / bare
        key without a configured `defaultTicketProvider`) — falls through to the `/vector:link`
        hint in step 11.

      > Token routing: detection is deterministic, zero-token, and already done in step 4.

   c. **Priority** only if the idea clearly implies one; else omit (defaults to `normal`).

   d. **Invoke `vector-spec-composer`** (**model: sonnet**, may write a file) with:
      - `BRIEF` (full refiner output from step 5)
      - `CLARIFICATIONS` (all Q&A pairs from step 6, in order)
      - `TEMPLATE_PATH`: absolute path to `.claude/vector/spec-template.md`
      - `SPEC_EXAMPLE_PATH` (from step 3, or `no example yet`)
      - `SPEC_TITLE`, `SPEC_ID`, `SPEC_LANGUAGE`
      - `OUTPUT_PATH`: `.vector/tmp/<SPEC_ID>/spec.md`

      The subagent writes the complete spec (20 sections) to `OUTPUT_PATH` and returns a
      confirmation with the path and the number of `TBD` markers. Hold the path as `SPEC_PATH`.
      **The main loop does not retain the spec text in its context — only the path.**

8. **Validate** — invoke the `vector-spec-validator` subagent (**model: sonnet**, read-only),
   passing the composed spec text, `SPEC_EXAMPLE_PATH`, the template path, and the 20-section
   checklist. React to the verdict:
   - `PASS` → continue.
   - `PASS_WITH_WARNINGS` → show warnings; ask address-now / keep. Fix and re-validate if addressing.
   - `BLOCK` → fix the required items (ask the user if a fix needs info). Re-validate. **Cap 3 cycles**;
     if still blocked, surface the report verbatim and stop without registering.

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
      the doc from `SPEC_PATH` and creates the card in `draft` (writing the doc inside the worktree
      created in 9a, so it stays tracked on the `<WT_PREFIX><SPEC_ID>` branch):

   ```bash
   vector spec create \
     --title "<SPEC_TITLE>" \
     --id "<SPEC_ID>" \
     [--repo "<repo-name>"] \
     [--priority "<priority>"] \
     [--ticket "$TICKET_JSON"] \
     --status draft \
     --body-file "$SPEC_PATH" --json
   ```

   Include `--ticket` only when step 7 set `TICKET_JSON`. Parse the JSON for `id`, `status`,
   and `specDoc` (where the doc landed). **Never block creation on linking**: if the binary
   rejects the `--ticket` (malformed JSON / uninferable provider), re-run `vector spec create`
   **without** `--ticket` and fall through to the `/vector:link` hint.

10. **Record the token routing** (feeds the board's Token Savings Meter). For **each**
    cheap-agent step you ran, call the binary once so the saving the routing produced is
    captured. You never write the JSON yourself — the binary derives cost/saved from the
    model and the token counts and appends the `agent.routed` event:

    ```bash
    # The refiner ran on Haiku instead of the Opus baseline:
    vector spec route <id> --model haiku  --baseline opus --task "refine spec" \
      --tokens-in <refiner-in> --tokens-out <refiner-out>
    # The compositor ran on Sonnet instead of the Opus baseline:
    vector spec route <id> --model sonnet --baseline opus --task "compose spec" \
      --tokens-in <composer-in> --tokens-out <composer-out>
    # The validator ran on Sonnet instead of the Opus baseline:
    vector spec route <id> --model sonnet --baseline opus --task "validate spec" \
      --tokens-in <validator-in> --tokens-out <validator-out>
    ```

    **Precision**: omit `--precision` (defaults to `estimated`) unless the harness exposed the
    exact token counts (e.g. via a tool-result metadata field or environment variable). If the
    harness did provide real counts, pass `--precision actual` — this marks the meter as exact
    and removes the "Estimated" badge on the board. Never pass `--precision actual` for numbers
    you derived yourself; an honest estimate is more trustworthy than a false claim of precision.

    Use the actual subagent token usage when you have it; otherwise pass your best estimate
    of each subagent's input/output size (the meter is an estimate by design — never invent
    precise-looking numbers, round to the nearest thousand). Skip a route you did not run
    (e.g. if validation was not needed). `--baseline` defaults to `opus`; keep it explicit.

11. **Report**: the card id, `status: draft`, the `specDoc` path, and the validator verdict.
    For the ticket: if one was seeded, say `linked <KEY> (<provider>)`; if a reference was
    detected but ambiguous (or a bare key without a configured `defaultTicketProvider`), say it
    can be linked with `/vector:link`; if none was found, don't mention a ticket. Tell the user
    the next step: **`/vector:propose`** generates the OpenSpec change (proposal/design/tasks)
    and moves the card from `draft` to `open`.

12. **Sketch Excalidraw (opt-in)** — after the report, offer a design wireframe when the spec is
    UI-facing. This step is optional and Sonnet-costly, so it only fires on a strong UI signal and
    with the user's confirmation; it never blocks the draft (already registered in step 9).

    a. **Opt-out check.** Skip this step **silently** (no prompt, no mention) if the user passed
       `--no-sketch` **or** `CONTEXT.sketchEnabled === false` (from step 3). Otherwise continue.

    b. **UI heuristic** over the composed spec (the `specDoc` written in step 9). A **strong signal**
       is present iff **either**:
       - the spec's §12 **"Estados de UI"** section is non-empty (i.e. it describes real UI states,
         not just `No aplica` / `N/A`); **or**
       - **≥ 2** distinct layer keywords appear in the title + body: `board`, `drawer`, `modal`,
         `web/`, `component`, `componente`, `UI`, `pantalla`, `formulario`, `card`.

       A single loose keyword is a **weak** signal → skip silently (a false negative is preferred
       over prompting on a non-UI spec). No strong signal → skip silently.

    c. **Confirm.** On a strong signal, ask via `AskUserQuestion` whether to generate an Excalidraw
       wireframe for the spec. **Decline → end the command cleanly** (the spec stays a draft with no
       sketch). **Confirm → continue.**

    d. **Spawn the designer (async) + register routing.** Spawn the **`vector-ui-ux-designer`**
       subagent (**model: sonnet**) as a **fresh async agent** and return immediately — do not wait
       for it (the draft is already on the board; the sketch attaches later via SSE). Pass it:

       ```
       SPEC_PATH:   <abs path to specDoc from step 9>
       SPEC_ID:     <id>
       OUTPUT_PATH: <REPO_ROOT>/.vector/tmp/<id>/sketch.excalidraw
       REPO_ROOT:   <REPO_ROOT>
       ```

       The agent writes the `.excalidraw` JSON to `OUTPUT_PATH` and calls
       `vector spec attach-sketch <id> --file <OUTPUT_PATH>` itself — the binary validates and
       persists it (CLI-owns-writes); a malformed sketch is silently rejected (the spec stays clean).

       Then register the **estimated** routing for the meter (the command does not wait for the
       agent's real token counts):

       ```bash
       vector spec route <id> --model sonnet --baseline opus --task "generate ui sketch" \
         --tokens-in <est> --tokens-out <est> --precision estimated
       ```

## Notes

- `draft` = spec authored, **no OpenSpec change yet**. The change is created later at
  `/vector:propose`; implementation starts at `/vector:apply`.
- The id is reused as the OpenSpec change name when proposed/applied.
- Keep the spec honest: mark unknowns as `TBD — ver Open questions`, never invent detail.
- If `vector` is not found, the binary isn't installed — tell the user to install it; do not
  write `.vector/` or the spec doc by hand as a substitute for the binary's registration.
