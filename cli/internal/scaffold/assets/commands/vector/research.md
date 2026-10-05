---
name: "Vector: Research"
description: Investigate an idea through relevant feasibility lenses, gate the verdict with the user, then author, validate, formalize, and register an open Vector spec.
argument-hint: "[idea-text]"
category: Workflow
tags: [vector, research, feasibility, spec]
---

Evaluate whether an idea is worth building before specifying it. Run the applicable
technical, security, marketing, and design lenses with skeptical reviewers, consolidate a
go/no-go verdict, gate continuation with the user, and only then produce a complete
20-section spec with its feasibility report embedded. Successful creation includes the
OpenSpec artifacts and ends as an `open` card ready for `/vector:apply <id>`.

The `vector` binary is the sole writer of `.vector` state. Never edit state files directly.

## Steps

1. Read `$ARGUMENTS` or the latest user message as `RAW_IDEA`.
2. Run `vector context --json --repo-root "$REPO_ROOT"`; resolve language, spec convention,
   worktree layout, and existing examples.
3. Select lenses from evidence: technical always; security for sensitive data/auth/trust;
   marketing for acquisition/positioning/monetization; design for user-facing interaction.
4. Spawn one `vector-feasibility-reviewer` per selected lens (Sonnet, read-only). Consolidate
   findings without averaging away a serious blocker. Cite repository evidence.
5. Present the verdict and ask a bounded `AskUserQuestion`: continue, revise, or stop. On stop,
   create nothing. On revise, incorporate the user's direction and rerun affected lenses.
6. Invoke `vector-spec-refiner`, ask blocking questions, compose the 20-section spec with
   `vector-spec-composer` at `.vector/tmp/<id>/spec.md`, append the feasibility report, and
   validate with `vector-spec-validator`. Cap repair/revalidation at three cycles.
7. In a bare+worktree repo, create or reuse the spec worktree before writing. Never delete a
   conflicting path or overwrite unrelated files.
8. Resolve `CHANGE_DIR` in that worktree (or `openspec/changes/<id>/` in a simple repo).
   Generate missing proposal/design/tasks using the repo's OpenSpec authoring tooling when
   available, otherwise `vector-proposal-generator` (Sonnet). Preserve existing artifacts.
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
10. Register the completed spec through the binary:

    ```bash
    vector spec create \
      --title "<title>" \
      --id "<id>" \
      --status open \
      --change "<id>" \
      --artifacts "<created-or-existing,list>" \
      [--ticket "$TICKET_JSON"] \
      [--epic "$EPIC_ID"] \
      --body-file ".vector/tmp/<id>/spec.md" \
      --json
    ```

11. Record actual routed-agent calls with `vector spec route`.
12. For a strong UI signal, offer an optional Excalidraw wireframe with a bounded
    `AskUserQuestion`. Skipping it leaves the open card unchanged.
13. Report the verdict, id, `status: open`, spec path, epic (when assigned), change directory,
    artifacts, and any accepted risks. Present only `/vector:apply <id>` as the implementation next step.

## Hard rules

- Research and specification only; do not implement the feature.
- New cards always end `open`; no holding lifecycle state is created.
- Artifact generation is part of authoring, never a separate user command.
- Preserve uncertainty and dissent from reviewers. Never fabricate evidence or product intent.
- If `vector` is unavailable, report the installation problem and stop; never emulate writes.
