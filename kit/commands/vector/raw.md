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

3. **Find an example spec** (for tone/depth). Glob the configured `specPath` directory and
   common locations (`docs/specs/**`, `openspec/changes/*/spec.md`, `specs/**`). If one
   exists, hold its path as `SPEC_EXAMPLE_PATH`; else `no example yet`.

4. **Detect the spec language** from the example / existing specs. Default English. State it
   in one line.

5. **Refine** — invoke the `vector-spec-refiner` subagent (**model: haiku**, read-only) with:
   `RAW_IDEA`, `SPEC_EXAMPLE_PATH`, and the template path `.claude/vector/spec-template.md`.
   It returns a structured brief surfacing ambiguity per the 20-section checklist. Call it `BRIEF`.

6. **Clarify** — walk every dimension of the 20-section checklist. For each unresolved one,
   batch ≤5 questions via `AskUserQuestion`. **No total cap** — keep iterating until every
   dimension has concrete content, or the user says stop (then mark `TBD — ver Open questions`).

7. **Compose the spec** using the canonical template at `.claude/vector/spec-template.md` —
   all **20 sections, in order**, replacing every `[...]` placeholder with verified content.
   Also:
   - Derive a concise **title** (≤ ~8 words) and a **kebab-case id** (slug of the title).
   - **Detect a ticket** reference (e.g. `VEC-42`, a Jira/Linear/GitHub URL) — note it for `/vector:link`.
   - **Priority** only if the idea clearly implies one; else omit (defaults to `normal`).

8. **Validate** — invoke the `vector-spec-validator` subagent (**model: sonnet**, read-only),
   passing the composed spec text, `SPEC_EXAMPLE_PATH`, the template path, and the 20-section
   checklist. React to the verdict:
   - `PASS` → continue.
   - `PASS_WITH_WARNINGS` → show warnings; ask address-now / keep. Fix and re-validate if addressing.
   - `BLOCK` → fix the required items (ask the user if a fix needs info). Re-validate. **Cap 3 cycles**;
     if still blocked, surface the report verbatim and stop without registering.

9. **Register the draft card** — pipe the validated spec to the binary via stdin. The binary
   writes the doc to the configured location and creates the card in `draft`:

   ```bash
   vector spec create \
     --title "<title>" \
     --id "<slug>" \
     [--repo "<repo-name>"] \
     [--priority "<priority>"] \
     --status draft \
     --body-file - --json <<'SPEC'
   <the full 20-section spec markdown>
   SPEC
   ```

   Parse the JSON for `id`, `status`, and `specDoc` (where the doc landed).

10. **Report**: the card id, `status: draft`, the `specDoc` path, and the validator verdict.
    If a ticket was detected, say it can be linked with `/vector:link`. Tell the user the next
    step: **`/vector:propose`** generates the OpenSpec change (proposal/design/tasks) and moves
    the card from `draft` to `open`.

## Notes

- `draft` = spec authored, **no OpenSpec change yet**. The change is created later at
  `/vector:propose`; implementation starts at `/vector:apply`.
- The id is reused as the OpenSpec change name when proposed/applied.
- Keep the spec honest: mark unknowns as `TBD — ver Open questions`, never invent detail.
- If `vector` is not found, the binary isn't installed — tell the user to install it; do not
  write `.vector/` or the spec doc by hand as a substitute for the binary's registration.
