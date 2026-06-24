---
name: "Vector: Sync"
description: Import the repo's existing OpenSpec changes onto the Vector board (idempotent, additive). Use in a repo that already uses OpenSpec so you don't recreate specs by hand.
category: Workflow
tags: [vector, sync, openspec, board]
---

Project the repo's OpenSpec changes onto the Vector board. This is **mechanical** —
the `vector` binary reads `openspec/changes/` and writes the cards (CLI-owns-writes).
Your job is just to run it and report; **do not** author specs or edit `.vector/` by hand.

> Token routing: pure file walk — no reasoning. Run the binary, don't think about it.

## Mapping (handled by the binary)

- `changes/<name>` active → `open` (0 tasks done) · `in-progress` (some) · `review` (all done).
- `changes/archive/<date>-<name>` → `archived` (id keeps the change name, no date prefix).
- `openspec/specs/` (applied capabilities) are **not** imported — they are the catalog, not work items.
- Synced cards carry `openspec{change,artifacts}` provenance; `/vector:raw` drafts are **never** touched.

## Steps

1. **Preview** what would change:
   ```bash
   vector sync --dry-run
   ```
   Report the counts (created / skipped / would-update) by status.
2. **Apply**:
   ```bash
   vector sync --json
   ```
   New changes become cards; existing cards are left as-is.
3. **Refresh statuses** of already-synced cards to match current task progress (optional):
   ```bash
   vector sync --reconcile --json
   ```
4. **Report** the board summary (counts by status) and remind the user that draft cards from
   `/vector:raw` and any manual edits were preserved.

## Notes

- Idempotent: re-running `vector sync` only adds missing cards. Use `--reconcile` to also move
  already-synced cards as their `tasks.md` progresses.
- If there is no `openspec/changes/`, the binary reports nothing to sync — that's fine.
- If `vector` is not found, it isn't installed — tell the user to install it; never write
  `.vector/` by hand.
