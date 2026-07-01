---
name: "Vector: Research"
description: Investigate whether a raw idea is worth building before specifying it — auto-detect the applicable feasibility lenses (technical always; security/marketing/design on signals), review each with a skeptical Sonnet reviewer, consolidate a go/no-go verdict, gate with the user, and only then author a full 20-section spec with the feasibility report embedded and register it as a draft. The exhaustive sibling of /vector:raw. You never write Vector's state yourself; the binary owns the writes.
argument-hint: "[idea-text]"
user-invocable: true
category: Workflow
tags: [vector, spec, research, feasibility, idea]
allowed-tools:
  - Read
  - Grep
  - Glob
  - Bash(vector *)
  - Agent
  - AskUserQuestion
---

Investigate a raw idea **across disciplines** to decide whether it is worth building, then —
**only if the user says go** — author a complete 20-section Vector spec with the feasibility
report embedded and register it as a `draft` card. This is the exhaustive sibling of
`/vector:raw`: where `raw` refines-and-emits, `research` **investigates → evaluates → decides →
emits**. It stops at a `draft` card; it does **not** create the OpenSpec change (`/vector:propose`)
or implement the feature (`/vector:apply`).

**Input**: `$ARGUMENTS` (the raw idea). If empty, use the user's latest message; if there is none,
ask for it and stop.

**You never write Vector's state files yourself** — the `vector` binary is the sole writer. You
author the **spec doc** (a repo artifact) and call the binary to register the card; the binary
writes the doc to the repo's configured location and creates the draft card.

> Token routing: lens **detection**, **orchestration**, **re-checking** and **consolidation** are
> light → stay in the main loop. **Refinement** runs on **Haiku** (`vector-spec-refiner`).
> **Feasibility reviews** and final **validation** are real reasoning → **Sonnet**
> (`vector-feasibility-reviewer` per lens, `vector-spec-validator` once). The expensive tier is
> spent only on the lenses that actually apply, in parallel (`product/token-routing.md`).

## Hard rules

- **Never implement code.** Stop after the spec is authored, validated, and registered as `draft`.
  `research` evaluates and authors; it never writes the feature or the OpenSpec change.
- **Gate before emitting.** Never author or register a card without an explicit go from the user
  at the go/no-go gate. On abort, no card is created and no spec doc is written.
- **No inference of product intent.** When behavior is unclear, ask — do not invent.
- **Don't trust the reviewers blindly.** Re-check the `file:line` evidence each lens cites; if it
  doesn't hold, downgrade that verdict and note it. The skepticism applied to the idea applies to
  the subagents too.
- **Agnostic to the repo.** Detect; never hardcode a package manager, layout, or framework. When
  lens detection is ambiguous, **ask** via `AskUserQuestion` — never run all four "just in case".
- **Cite, don't guess.** Paths, versions, endpoints must be verified against the repo or marked
  `TBD — ver Open questions`.
- **Spec language follows the project** (detect from existing specs; default English). The report
  and verdict use `config.language`, else the conversation language. Slugs / paths / git artifacts
  are English kebab-case.

## Steps

0. **Get repo context** — fetch the setup context from the binary in one call before any other
   work:

   ```bash
   CONTEXT=$(vector context --json --repo-root "$REPO_ROOT" 2>/dev/null)
   ```

   > Token routing: one zero-token binary call returns examplePath + language so later steps need
   > not re-derive them from globs.

   Extract from `CONTEXT`:
   - `SPEC_EXAMPLE_PATH` ← `CONTEXT.examplePath`
   - `SPEC_LANGUAGE` ← `CONTEXT.language`

   **Fallback when `vector context` fails** (binary not in PATH or exits 1): emit a one-line
   warning, then glob the configured `specPath` directory and common locations
   (`docs/specs/**`, `openspec/changes/*/spec.md`, `specs/**`) for an example spec; detect
   `SPEC_LANGUAGE` from it (default English).

1. **Read the raw idea** (`$ARGUMENTS`, or the latest message). Hold as `RAW_IDEA`. If empty,
   ask for it via `AskUserQuestion` and stop until provided.

2. **Confirm the repo is initialized.** The spec doc location and `config.language` come from
   `.vector/config.json` (written by `vector init`, migrated from `.project-structure`). If it is
   missing, run `vector init` first (or tell the user to), so the spec lands in the repo's
   convention instead of the `.vector/` fallback. Note the resolved `specPath` for the report.

3. **Detect the lenses** (main loop, cheap). Decide which feasibility lenses to run from the text
   of `RAW_IDEA`. **`technical` always runs** (the minimum core). Activate the others on signals:

   | Lens | Activate when the idea mentions / implies |
   |---|---|
   | `technical` | always (core) |
   | `security` | auth, data/PII, secrets, permissions, external input, writing/moving/deleting files in the user's repo, destructive operations |
   | `marketing` | a user-facing feature, pricing, growth, onboarding, positioning, commercial value |
   | `design` | UI, the board, the web panel, visual/interaction concerns, accessibility |

   Show the selected set (`research.lenses.detected`). If the signals are ambiguous or
   contradictory, run only `technical` and offer to adjust the set via `AskUserQuestion`
   (the candidate lenses plus an open "Other") — **never force** lenses, never run all four "just
   in case". Hold the result as `LENSES`.

4. **Refine** — invoke the `vector-spec-refiner` subagent (**model: haiku**, read-only) with:
   `RAW_IDEA`, `SPEC_EXAMPLE_PATH`, and the template path `.claude/vector/spec-template.md`.
   It returns a structured brief surfacing ambiguity per the 20-section checklist. Call it `BRIEF`.

5. **Clarify** — walk the open dimensions of `BRIEF`. For each unresolved one, batch ≤5 questions
   via `AskUserQuestion`. Keep iterating until every dimension has concrete content, or the user
   says stop (then mark `TBD — ver Open questions`). Hold the resolved idea (`BRIEF` +
   clarifications) as `REFINED_IDEA` — this is what the reviewers evaluate.

6. **Review feasibility** (Sonnet, **one invocation per lens in `LENSES`, in parallel**). For each
   lens, spawn the `vector-feasibility-reviewer` subagent (`subagent_type:
   "vector-feasibility-reviewer"`), passing:
   - `LENS` — the single lens.
   - `IDEA` — `REFINED_IDEA`.
   - `REPO_CONTEXT` — repo root + the stack/specPath/language resolved in steps 0/2.

   Spawn the lenses in one batch so they run concurrently. Each gathers its own evidence and
   returns `LENS` / `VERDICT` (`go`/`go-with-risks`/`no-go` + `N/10`) / `FINDINGS` / `RISKS` /
   `RECOMMENDATION`. Do **not** pre-summarize the repo for them — hand them the refined idea and
   let them ground their verdict in the real code.

7. **Re-check** (main loop). Do not trust the reviewers blindly. For each lens, spot-check the key
   `file:line` evidence it cites — read it yourself and confirm the code/docs say what the verdict
   claims. If the evidence doesn't hold, **downgrade** that verdict and note the discrepancy. If a
   lens returns output that is **not parseable** into the five sections (missing `VERDICT`/
   `FINDINGS`), treat it as `go-with-risks` "lente no concluyente", offer to retry that one lens,
   and never invent a verdict for it.

8. **Consolidate the verdict** (main loop). Combine the lenses into one global verdict:
   - `no-go` if **any** critical lens is `no-go` (`technical` and `security` are critical; a
     `marketing`/`design` `no-go` consolidates to at most `go-with-risks` unless the user treats it
     as a blocker).
   - `go-with-risks` if no `no-go` but any lens is `go-with-risks` or carries unresolved risks.
   - `go` if every run lens is `go`.

   Present it before the gate, in `config.language`: a per-lens line (lens · verdict · `N/10` ·
   one-line reason) and the consolidated verdict with a 1–2 line summary.

9. **Go/no-go gate** (`AskUserQuestion`). Present the consolidated verdict and ask whether to emit
   the spec. The recommendation follows the verdict, but **the human decides**. Options:
   - **Emitir el spec** (recommended when consolidated is `go` / `go-with-risks`)
   - **Refinar más** → return to step 5 (clarify) or step 6 (re-review) as the user directs
   - **Abortar** → terminate **without** creating a card or writing the spec doc; confirm with
     `research.aborted` and stop.

   A `no-go` consolidated verdict recommends **not** emitting; the user may still force emit (the
   risk is recorded in the embedded report).

10. **Resolve metadata and compose the spec** (only past the gate):

    a. **Derive the title and id** from `BRIEF` (the refiner proposes them in
       `## Optimized Change Title` / `## Kebab-case Change Name`). Confirm with the user if step 5
       left ambiguity in the name. Hold as `SPEC_TITLE` and `SPEC_ID`.

    b. **Detect the ticket** in `RAW_IDEA` with the same logic as `raw`/`bug`:

       ```bash
       DETECT_JSON=$(echo "$RAW_IDEA" | vector detect-ticket --repo-root "$REPO_ROOT" --json)
       ```

       `DETECT_JSON.ticket` non-null → `TICKET_JSON = DETECT_JSON.ticket`; null → leave it unset
       (falls through to the `/vector:link` hint). Detection is deterministic and zero-token.

    c. **Priority** only if the idea clearly implies one; else omit (defaults to `normal`).

    d. **Invoke `vector-spec-composer`** (**model: sonnet**, may write a file) with:
       - `BRIEF` (full refiner output from step 4)
       - `CLARIFICATIONS` (all Q&A pairs from step 5, in order)
       - `TEMPLATE_PATH`: absolute path to `.claude/vector/spec-template.md`
       - `SPEC_EXAMPLE_PATH` (from step 0, or `no example yet`)
       - `SPEC_TITLE`, `SPEC_ID`, `SPEC_LANGUAGE`
       - `OUTPUT_PATH`: `.vector/tmp/<SPEC_ID>/spec.md`

       The composer writes the 20-section spec to `OUTPUT_PATH` and returns the path + the count of
       `TBD` markers. Hold the path as `SPEC_PATH`. **The main loop does not retain the spec text —
       only the path.**

    e. **Append the feasibility report** as an annex after §20 (alongside `## Open questions`, the
       repo's annex convention). Do not edit `.vector/`; append to `SPEC_PATH` (the temp artifact)
       before registering. The annex is a `## Reporte de viabilidad` section with this exact shape,
       each `<…>` slot filled from steps 6–8 (lenses not in `LENSES` are written
       `No corrida — no aplica`):

       ```markdown
       ## Reporte de viabilidad

       | Lente | Veredicto | Confianza | Hallazgos clave | Riesgos |
       |---|---|---|---|---|
       | technical | <go|go-with-risks|no-go> | <N/10> | <hallazgos> | <riesgos> |
       | security | <verdict · o No corrida — no aplica> | <N/10> | <hallazgos> | <riesgos> |
       | marketing | <verdict · o No corrida — no aplica> | <N/10> | <hallazgos> | <riesgos> |
       | design | <verdict · o No corrida — no aplica> | <N/10> | <hallazgos> | <riesgos> |

       **Veredicto consolidado:** <go|go-with-risks|no-go> — <resumen de 1–2 líneas>.
       ```

       Write the annex prose in `SPEC_LANGUAGE`.

11. **Validate** — invoke the `vector-spec-validator` subagent (**model: sonnet**, read-only),
    passing the composed spec text, `SPEC_EXAMPLE_PATH`, the template path, and the 20-section
    checklist. React to the verdict:
    - `PASS` → continue.
    - `PASS_WITH_WARNINGS` → show warnings; ask address-now / keep. Fix and re-validate if
      addressing.
    - `BLOCK` → fix the required items (ask the user if a fix needs info). Re-validate. **Cap 3
      cycles**; if still blocked, surface the report verbatim and stop **without** registering.

12. **Register the draft card** — pass the validated spec to the binary via file path. The binary
    reads the doc from `SPEC_PATH` and creates the card in `draft`:

    ```bash
    vector spec create \
      --title "<SPEC_TITLE>" \
      --id "<SPEC_ID>" \
      [--priority "<priority>"] \
      [--ticket "$TICKET_JSON"] \
      --status draft \
      --body-file "$SPEC_PATH" --json
    ```

    Include `--ticket` only when step 10b set `TICKET_JSON`. Parse the JSON for `id`, `status`, and
    `specDoc` (where the doc landed). **Never block creation on linking**: if the binary rejects the
    `--ticket` (malformed JSON / uninferable provider), re-run `vector spec create` **without**
    `--ticket` and fall through to the `/vector:link` hint.

13. **Record the token routing** (feeds the board's Token Savings Meter). For **each** cheap/medium
    agent step you ran, call the binary once so the saving is captured — you never write the JSON
    yourself; the binary derives cost/saved from the model and token counts and appends the
    `agent.routed` event:

    ```bash
    # The refiner ran on Haiku instead of the Opus baseline:
    vector spec route "<SPEC_ID>" --model haiku  --baseline opus --task "refine idea" \
      --tokens-in <refiner-in> --tokens-out <refiner-out>
    # Each feasibility lens ran on Sonnet instead of the Opus baseline (one per lens in LENSES):
    vector spec route "<SPEC_ID>" --model sonnet --baseline opus --task "review feasibility (<lens>)" \
      --tokens-in <lens-in> --tokens-out <lens-out>
    # The compositor ran on Sonnet instead of the Opus baseline:
    vector spec route "<SPEC_ID>" --model sonnet --baseline opus --task "compose spec" \
      --tokens-in <composer-in> --tokens-out <composer-out>
    # The validator ran on Sonnet instead of the Opus baseline:
    vector spec route "<SPEC_ID>" --model sonnet --baseline opus --task "validate spec" \
      --tokens-in <validator-in> --tokens-out <validator-out>
    ```

    **Precision**: omit `--precision` (defaults to `estimated`) unless the harness exposed exact
    token counts. If it did, pass `--precision actual`. Never pass `--precision actual` for numbers
    you derived yourself; an honest estimate is more trustworthy than a false claim of precision.
    Round estimates to the nearest thousand. Skip a route you did not run. `--baseline` defaults to
    `opus`; keep it explicit.

14. **Report** (in `config.language`, else the conversation language): the card id, `status: draft`,
    the `specDoc` path, the **consolidated feasibility verdict** (with the per-lens summary), and
    the validator verdict. For the ticket: if one was seeded, say `linked <KEY> (<provider>)`; if a
    reference was detected but ambiguous, say it can be linked with `/vector:link`; if none, don't
    mention a ticket. Tell the user the next step: **`/vector:propose`** generates the OpenSpec
    change (proposal/design/tasks) and moves the card `draft → open`; **`/vector:apply`** implements
    it. Note the token routing (refiner = Haiku, lenses + composer + validator = Sonnet,
    orchestration = main loop) and that **the binary owns every state write** (CLI-owns-writes).

15. **Sketch Excalidraw (opt-in)** — after the report, offer a design wireframe when the spec is
    UI-facing. Same tail step as `/vector:raw` step 12: optional, Sonnet-costly, fires only on a
    strong UI signal with the user's confirmation; it never blocks the draft (registered in step 12).

    a. **Opt-out check.** Skip **silently** (no prompt, no mention) if the user passed `--no-sketch`
       **or** `CONTEXT.sketchEnabled === false` (from step 0). Otherwise continue.

    b. **UI heuristic** over the composed spec (the `specDoc` from step 10). **Strong signal** iff
       **either** the spec's §12 **"Estados de UI"** section is non-empty (real UI states, not
       `No aplica` / `N/A`) **or** **≥ 2** distinct layer keywords appear in title + body: `board`,
       `drawer`, `modal`, `web/`, `component`, `componente`, `UI`, `pantalla`, `formulario`, `card`.
       A single loose keyword is weak → skip silently. No strong signal → skip silently.

    c. **Confirm** via `AskUserQuestion` whether to generate the wireframe. **Decline → end cleanly**
       (spec stays a draft, no sketch). **Confirm → continue.**

    d. **Spawn the designer (async) + register routing.** Spawn the **`vector-ui-ux-designer`**
       subagent (**model: sonnet**) as a **fresh async agent** and return immediately (the sketch
       attaches later via SSE). Pass it:

       ```
       SPEC_PATH:   <abs path to specDoc from step 10>
       SPEC_ID:     <id>
       OUTPUT_PATH: <REPO_ROOT>/.vector/tmp/<id>/sketch.excalidraw
       REPO_ROOT:   <REPO_ROOT>
       ```

       The agent writes the `.excalidraw` JSON and calls `vector spec attach-sketch <id> --file
       <OUTPUT_PATH>` itself (the binary validates + persists; a malformed sketch is silently
       rejected). Then register the **estimated** routing for the meter:

       ```bash
       vector spec route <id> --model sonnet --baseline opus --task "generate ui sketch" \
         --tokens-in <est> --tokens-out <est> --precision estimated
       ```

## Notes

- `draft` = spec authored, **no OpenSpec change yet**. The change is created at `/vector:propose`;
  implementation starts at `/vector:apply`. The id is reused as the OpenSpec change name.
- The feasibility report travels **with the spec** (embedded annex), so the verdict is queryable
  later — there is no separate `spec.researched` event or board panel (out of scope).
- Always `draft` in V1 (even on `go-with-risks`): the risk lives in the embedded report, not in a
  `needs-attention` status.
- Keep the spec honest: mark unknowns as `TBD — ver Open questions`, never invent detail or a
  verdict.
- If `vector` is not found, the binary isn't installed — tell the user to install it; do not write
  `.vector/` or the spec doc by hand as a substitute for the binary's registration.
