---
name: "Vector: Epic"
description: Manage epics in natural language — create, list, show, update, reorder, focus or delete epics and assign or unassign existing specs (e.g. "put these 5 specs in the App Mobile epic", "put App Mobile first", "pin the payments epic", "take login out of its epic"). Resolves references, confirms bulk or ambiguous changes, and writes only through the binary.
argument-hint: "\"<what to do with epics>\""
category: Workflow
tags: [vector, epic, grouping, organization]
---

Manage **epics** — named groups of specs (e.g. "App Mobile") — from a natural-language request:
create/list/show/update/reorder/focus/delete epics and assign or unassign **existing** specs. **You never write
Vector's state yourself**: every change goes through `vector epic …` or `vector spec epic …`
(CLI-owns-writes). Never edit `.vector/` by hand.

**Input**: `$ARGUMENTS` — the request, in any language (e.g. `"put add-login, add-signup and
push-notifications in App Mobile"`, `"what's in the payments epic"`, `"take login out of its
epic"`, `"pon App Mobile primero"`, `"reordená las épicas: web, mobile, backend"`, `"pineá la
épica payments"`). If empty, use the user's latest message; if still empty, ask what to do.

> Token routing: orchestration only, in the main loop — parse the request, read two JSON lists,
> call the binary. No subagent: resolving names against a short list is cheaper here than spawning.

## Hard rules

- **Epic membership, order and focus are metadata, not transitions.** They never change a spec's
  status or priority. Epic focus is **never copied onto specs**: every non-closed spec of a focused
  epic inherits it at read time, so specs assigned later inherit it too, and unfocusing the epic
  leaves individually focused specs focused.
- **Never guess a reference.** Every spec and epic reference must resolve to exactly one id;
  otherwise ask (`AskUserQuestion`) or report it unresolved. Never assign a spec you didn't resolve.
- **Confirm before bulk, ambiguous, or moving changes** (see step 4). A single, unambiguous
  assign/unassign/create/update runs without a confirmation round-trip.
- **Delete only with explicit confirmation.** The binary refuses to delete an epic that specs
  still reference — surface that error verbatim; never clear the specs on your own to force it.
- **Surface binary errors verbatim** (unknown epic, invalid color/id, duplicate id); don't work
  around them.

## Steps

1. **Classify the intent** — one or more of: `list`, `show`, `create`, `update`, `reorder`,
   `focus`, `unfocus`, `delete`, `assign`, `unassign`. A request may combine them (e.g. create
   "App Mobile" and put 3 specs in it → `create` then `assign`).
   - `reorder`: "put X first", "pon X primero", "X before Y", "reordená: A, B, C". Order is an
     integer per epic (1 = first; unordered epics sort after the ordered ones, by title).
   - `focus` / `unfocus`: "pin/focus/pineá/focuseá the epic X", "work on X first" (when X is an
     epic, not a spec), "unpin X".

2. **Load state once** (read-only):

   ```bash
   vector epic list --json          # display order: [{id,title,order?,focus?,total,done,dropped?,byStatus,…}]
   vector spec list --json          # only when the intent touches specs: [{id,title,status,epic,…}]
   # narrower reads when useful: vector spec list --epic <id> | --no-epic | --status open,review | --focus
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
   - the intent is `delete` (always; the option reads **Delete `<id>`**);
   - a `reorder` renumbers two or more epics (show the resulting order `1. App Mobile, 2. Web, …`).

5. **Execute through the binary** — `--json` on every call; **one bulk call per target epic**:

   ```bash
   vector epic create --title "<title>" [--id "<kebab-id>"] [--description "<text>"] \
     [--color slate|blue|teal|green|amber|orange|red|pink|violet] [--order N] --json
   vector epic update <epic-id> [--title "<title>"] [--description "<text>"] [--color <token>] \
     [--order N] --json                                    # --order 0 unsets the order
   vector epic focus <epic-id> --json                     # its open specs inherit focus
   vector epic unfocus <epic-id> --json
   vector spec epic --epic <epic-id> <spec-id> <spec-id>… --json   # assign (or move) several
   vector spec epic --clear <spec-id> <spec-id>… --json            # unassign several
   vector epic show <epic-id> --json               # show: the epic + its specs
   vector epic delete <epic-id> --json             # only after explicit confirmation
   ```

   For long lists, pipe newline-separated ids instead of positionals:
   `printf '%s\n' add-login add-signup | vector spec epic --epic app-mobile --stdin --json`.
   The single-spec form `vector spec epic <spec-id> <epic-id>` still works.

   - **create**: a title is required (ask if missing). Suggest a kebab-case id and a palette color
     when the user gave none; pass only what was decided. Omit `--id` to let the binary derive it.
   - **update**: pass only the fields the user asked to change; `--description ""` /
     `--color ""` clear them. `--title ""` is refused by the binary.
   - **assign/unassign**: the bulk call validates every id first — if any spec id is unknown the
     binary fails **before writing anything** and names the unknown ids; fix/drop them (ask if
     needed) and rerun. On success it returns `{epic, specs:[{id, changed, previous?}], changed,
     unchanged}` — report from that.
   - **reorder**: compute the full target order, then one `vector epic update <id> --order N` per
     epic whose order changes (1-based, contiguous). "Put X first" → X gets 1 and every other
     **ordered** epic shifts down by one, keeping their relative order; unordered epics stay
     unordered unless the user listed them. "Remove X from the order" → `--order 0`.
   - **focus/unfocus**: `vector epic focus|unfocus <epic-id>`; `changed: false` means it already
     was — say so. Mention that it affects every open spec of the epic (board column order and
     `/vector:apply` selection).
   - **delete refused** (specs still reference it): show the binary's error with the member ids and
     stop. The user can follow up with an unassign request (`/vector:epic take … out of <id>`)
     and then delete.

6. **Report** — what changed, concisely:
   - per spec: `assigned <spec> → <epic>`, `moved <spec>: <old> → <new>`, `cleared <spec>`, or
     `no change`; plus any unresolved/failed reference with its reason;
   - for create/update/delete: the epic id, title, color, order;
   - for reorder: the resulting order (`1. …, 2. …`); for focus: which epic is now focused;
   - for list/show: a compact table (`#order`, `id`, `done/total` (+ `dropped` when non-zero),
     `title`, focus marker; for show, each spec's `id`, `status`, `title`, and a resolution when
     it is not `done`). After assignments, one `vector epic show <epic-id> --json` gives the
     updated `done/total` to report.

## Notes

- Epics are stored at `.vector/epics/<id>.json`; specs point to them through their `epic` field.
  The board shows them in the **epics** tab (with order, focus pin and progress) and on each
  card's details drawer.
- Epic progress: `done` counts closed/archived specs resolved as `done`; specs closed as
  `obsolete`/`duplicate`/`superseded` are `dropped` (excluded from done and total).
- New specs can be grouped at creation time: `/vector:idea`, `/vector:bug`, `/vector:quick`, and
  `/vector:research` pick an epic from the request (`vector spec create --epic <id>`).
- If `vector` is not found, it isn't installed — tell the user; never edit `.vector/` by hand.
