---
name: "Vector: Epic"
description: Manage epics in natural language — create, list, show, update, or delete epics and assign or unassign existing specs (e.g. "put these 5 specs in the App Mobile epic", "what's in epic X", "take login out of its epic"). Resolves references, confirms bulk or ambiguous changes, and writes only through the binary.
argument-hint: "\"<what to do with epics>\""
category: Workflow
tags: [vector, epic, grouping, organization]
---

Manage **epics** — named groups of specs (e.g. "App Mobile") — from a natural-language request:
create/list/show/update/delete epics and assign or unassign **existing** specs. **You never write
Vector's state yourself**: every change goes through `vector epic …` or `vector spec epic …`
(CLI-owns-writes). Never edit `.vector/` by hand.

**Input**: `$ARGUMENTS` — the request, in any language (e.g. `"put add-login, add-signup and
push-notifications in App Mobile"`, `"what's in the payments epic"`, `"take login out of its
epic"`). If empty, use the user's latest message; if still empty, ask what to do.

> Token routing: orchestration only, in the main loop — parse the request, read two JSON lists,
> call the binary. No subagent: resolving names against a short list is cheaper here than spawning.

## Hard rules

- **Epic membership is metadata, not a transition.** It never changes a spec's status or priority.
- **Never guess a reference.** Every spec and epic reference must resolve to exactly one id;
  otherwise ask (`AskUserQuestion`) or report it unresolved. Never assign a spec you didn't resolve.
- **Confirm before bulk, ambiguous, or moving changes** (see step 4). A single, unambiguous
  assign/unassign/create/update runs without a confirmation round-trip.
- **Delete only with explicit confirmation.** The binary refuses to delete an epic that specs
  still reference — surface that error verbatim; never clear the specs on your own to force it.
- **Surface binary errors verbatim** (unknown epic, invalid color/id, duplicate id); don't work
  around them.

## Steps

1. **Classify the intent** — one or more of: `list`, `show`, `create`, `update`, `delete`,
   `assign`, `unassign`. A request may combine them (e.g. create "App Mobile" and put 3 specs in
   it → `create` then `assign`).

2. **Load state once** (read-only):

   ```bash
   vector epic list --json          # [{id,title,description,color,total,done,byStatus,…}]
   vector spec list --json          # only when the intent touches specs: [{id,title,status,epic,…}]
   ```

3. **Resolve references**:
   - **Epic**: exact `id` → case-insensitive `title` → clear match on `description`. More than
     one candidate → ask with `AskUserQuestion` (one option per epic, `<title> (<id>)`). None,
     for an `assign` → offer to create it (step 5, `create`), options **Create and assign** /
     **Cancel**. None, for `show`/`update`/`delete` → report "no epic matches" plus the list.
   - **Specs**: exact `id` → `id`/`title` match → references from the conversation ("these 5
     specs" = the specs just discussed/listed). Each reference must map to exactly one spec;
     ambiguous ones are asked (options = candidates `<id> — <title> (<status>)`), unresolved ones
     are reported and skipped. If the user gave a count ("these 5") and you resolved a different
     number, ask before continuing.
   - **Unassign** ("take X out of its epic") needs no epic: it clears whatever `epic` the spec has.
     A spec whose `epic` is already empty is a no-op — report it, don't call the binary.

4. **Confirm when required** — one `AskUserQuestion` showing the exact plan
   (`add-login → app-mobile`, `add-signup: payments → app-mobile`, …) with **Apply** / **Cancel**,
   when **any** of these holds:
   - two or more specs change (bulk);
   - any reference was resolved by fuzzy/title/conversation matching rather than an exact id;
   - a spec already belongs to a **different** epic (the assign moves it);
   - the intent is `delete` (always; the option reads **Delete `<id>`**).

5. **Execute through the binary** — `--json` on every call, one call per spec:

   ```bash
   vector epic create --title "<title>" [--id "<kebab-id>"] [--description "<text>"] \
     [--color slate|blue|teal|green|amber|orange|red|pink|violet] --json
   vector epic update <epic-id> [--title "<title>"] [--description "<text>"] [--color <token>] --json
   vector spec epic <spec-id> <epic-id> --json     # assign (or move)
   vector spec epic <spec-id> --clear --json       # unassign
   vector epic show <epic-id> --json               # show: the epic + its specs
   vector epic delete <epic-id> --json             # only after explicit confirmation
   ```

   - **create**: a title is required (ask if missing). Suggest a kebab-case id and a palette color
     when the user gave none; pass only what was decided. Omit `--id` to let the binary derive it.
   - **update**: pass only the fields the user asked to change; `--description ""` /
     `--color ""` clear them. `--title ""` is refused by the binary.
   - **assign/unassign**: keep going after a per-spec failure; collect each result
     (`changed: true|false`) and error.
   - **delete refused** (specs still reference it): show the binary's error with the member ids and
     stop. The user can follow up with an unassign request (`/vector:epic take … out of <id>`)
     and then delete.

6. **Report** — what changed, concisely:
   - per spec: `assigned <spec> → <epic>`, `moved <spec>: <old> → <new>`, `cleared <spec>`, or
     `no change`; plus any unresolved/failed reference with its reason;
   - for create/update/delete: the epic id, title, color;
   - for list/show: a compact table (`id`, `done/total`, `title`; for show, each spec's `id`,
     `status`, `title`). After assignments, one `vector epic show <epic-id> --json` gives the
     updated `done/total` to report.

## Notes

- Epics are stored at `.vector/epics/<id>.json`; specs point to them through their `epic` field.
  The board shows them in the **epics** tab and on each card's details drawer.
- New specs can be grouped at creation time: `/vector:idea`, `/vector:bug`, `/vector:quick`, and
  `/vector:research` pick an epic from the request (`vector spec create --epic <id>`).
- If `vector` is not found, it isn't installed — tell the user; never edit `.vector/` by hand.
