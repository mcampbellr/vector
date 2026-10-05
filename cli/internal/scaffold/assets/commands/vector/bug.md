---
name: "Vector: Bug"
description: Turn a bug report into a validated fix spec, trace its likely cause, create its OpenSpec artifacts, and register it open for implementation.
argument-hint: "[bug-report] {spec-id|branch|file}"
category: Workflow
tags: [vector, bug, spec, openspec]
---

Turn a bug report into a complete, validated, bug-framed Vector spec. Trace the likely
cause from repository evidence, persist any confident `relatedTo[]` relation, generate the
OpenSpec proposal/design/tasks, and register an `open` card. Do not implement the fix; the
only implementation next step is `/vector:apply <id>`.

The `vector` binary is the sole writer of `.vector` state. Never edit state files directly.

## Steps

1. Parse `$ARGUMENTS` as the bug report plus an optional scope hint. If the report is empty,
   use the user's latest message.
2. Run `vector context --json --repo-root "$REPO_ROOT"` and resolve the configured spec path,
   language, worktree layout, build/test commands, and a representative existing spec.
3. Invoke `vector-bug-refiner` (Sonnet, read-only) with the report, scope hint, repository
   context, and recent `git log`/`git blame` evidence. Require a `fix-<slug>` id and distinguish
   facts from hypotheses.
4. Ask only blocking questions with `AskUserQuestion`. Never invent product intent or a root
   cause. Keep unknowns explicit.
5. Compose the 20-section spec with `vector-spec-composer` at
   `.vector/tmp/<id>/spec.md`, then validate with `vector-spec-validator`. Fix and revalidate
   blockers, capped at three cycles. Stop without registering if validation still blocks.
6. Resolve a cause relation only when evidence is strong:
   - prior Vector card → `[{"kind":"spec","ref":"<id>","source":"blame"}]`
   - external ticket → `[{"kind":"ticket","ref":"<provider>:<key>","source":"blame"}]`
   Omit ambiguous relations; a bad relation must never prevent card creation.
7. In a bare+worktree repo, create or reuse the spec worktree before writing the spec. Never
   delete or overwrite a conflicting path.
8. Resolve `CHANGE_DIR` in that worktree (or `openspec/changes/<id>/` in a simple repo).
   Generate missing proposal/design/tasks with the repo's OpenSpec authoring tooling when
   available; otherwise use `vector-proposal-generator` (Sonnet). Preserve existing artifacts.
   If formalization fails, stop without registering a non-actionable card.
9. **Epic (optional grouping).** Right before `vector spec create`, list the epics once:

   ```bash
   EPICS_JSON=$(vector epic list --json 2>/dev/null)
   ```

   Resolve `EPIC_ID` from the user's request and clarifications (one list call, no extra agent):
   - **Named or one clear match**: the request names an existing epic, or clearly matches
     exactly one by `id`/`title`/`description` → set `EPIC_ID` to its `id`; don't ask.
   - **Ambiguous**: more than one plausible epic, or only a weak match → ask once with
     `AskUserQuestion`: one option per plausible epic (`<title> (<id>)`) plus **No epic**.
   - **Fits an epic that doesn't exist** (e.g. "for the App Mobile epic" and there is none) →
     propose it with `AskUserQuestion`, showing the title, a suggested kebab-case id, and one
     color from `slate|blue|teal|green|amber|orange|red|pink|violet`: **Create and assign** /
     **No epic**. Only on **Create and assign** run
     `vector epic create --title "<title>" --id "<id>" --color <color> --json` and set
     `EPIC_ID` to the returned `id`; if it fails, show the error and continue without an epic.
   - **No epics, a failed list, or nothing plausibly fits** → leave `EPIC_ID` unset; don't ask
     and don't mention epics.

   Pass `--epic "$EPIC_ID"` only when set. The epic never blocks creation: if the binary
   rejects `--epic`, re-run `vector spec create` without it and report the error.
10. Register the card and spec atomically through the binary:

    ```bash
    vector spec create \
      --title "<title>" \
      --id "<fix-id>" \
      --status open \
      --change "<fix-id>" \
      --artifacts "<created-or-existing,list>" \
      [--ticket "$TICKET_JSON"] \
      [--related "$RELATED_JSON"] \
      [--epic "$EPIC_ID"] \
      --body-file ".vector/tmp/<fix-id>/spec.md" \
      --json
    ```

11. Record routed-agent usage with `vector spec route` for the refiner, composer, validator,
    and native artifact generator when each actually ran.
12. For a strong UI signal, offer the optional Excalidraw wireframe using a bounded
    `AskUserQuestion`; skipping it leaves the open card unchanged.
13. Report the id, `status: open`, spec path, relation evidence, epic (when assigned), validation
    result, change directory, and artifacts. Copy or present only `/vector:apply <id>` as the next command.

## Hard rules

- No implementation in this command.
- New cards always end `open`; never create a holding lifecycle state.
- **If a `.vector/` exists at an ancestor directory, that store is the base** — never
  `vector init` a nested one, and never pass `--force` to silence the guard. See
  `.claude/agents/_shared/root-anchoring-guardrail.md`.
- Artifact generation is part of authoring. It is not a user-visible follow-up step.
- Re-running the same report may create another distinct bug card when the derived id differs;
  do not silently merge incidents.
- If `vector` is unavailable, report the installation problem. Do not emulate state writes.
