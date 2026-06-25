# Detect the ticket from the worktree folder name during sync

## Why

The sibling changes `add-ticket-linking` and `extend-ticket-auto-detection` resolve a ticket from a
spec/change's **artifacts** (frontmatter `ticket:`, a prose URL, a cue word or a known project
prefix). But in a bare+worktree repo each branch lives in a folder named `<KEY>-<slug>`
(e.g. `code/feat/mh-1592-payments-period-checkout-camelcase`), and the change's own artifacts often
**don't mention the key at all**. Those specs link nothing today.

The worktree folder name is the highest-recall deterministic signal available: in the `somnio` test
repo it raises detection from ~4 to 24 specs, with **exact** matches and **no false positives**. This
change adds a **fourth detection source** to `vector sync` that associates a change to its worktree
folder and extracts the Jira/Linear/GitHub key from that folder's name — without an LLM in the `sync`
path (token-routing) and without touching the glob that reads changes.

## What changes

- **Multi-level worktree enumerator** (`config.WorktreeTicketKeys(repoRoot) map[string]string`): derives
  the **worktree root** from the literal prefix of `changesTemplate()` before `[branch]` (e.g.
  `code/[branch]/openspec/changes` → `code`). Read-only, depth-bounded scan that finds branch folders
  one or several levels deep, tolerating grouping folders (`feat`/`chore`/`fix`/`docs`/…) and
  single-level branches (`develop`). Depth bound is a **named constant** (`worktreeMaxDepth`), not a
  magic number. If the template has no `[branch]`, returns an empty map (feature inert).
- **slug → ticket key index**: for each leaf branch folder, if its basename matches `<KEY>-<slug>`
  register the key (universal form `[A-Za-z][A-Za-z0-9]*-\d+`, project part normalized to uppercase),
  applying the `ADR`/`RFC` **denylist**. A bare `<KEY>` folder (no slug) is **not** indexed. Match to a
  change by **exact slug** (== change name) after stripping the `<KEY>-` prefix. Duplicate slugs with
  different keys are omitted (ambiguous).
- **4th source in `detectTicket`**: as the **last fallback** (after frontmatter, prose URL and
  cue/prefix), **only when** `defaultTicketProvider` is set and the branch key is not denylisted, link
  `{provider: default, key, url:"", auto:true}`.
- **Threading**: `runSync` builds the index **once** and passes the per-slug candidate key to
  `detectTicket`.

## Scope

- In: `WorktreeTicketKeys` (multi-level bounded scan + exact-slug match + denylist + uppercase),
  `detectTicket` gaining a `branchKey` last fallback (gated on provider, artifact always wins),
  `runSync` computing the index once and threading `index[change.Name]`, Go tests (config enumerator +
  ticket fallback + `runSync` integration), and the `docs/domain-contract.md` §5 update.
- Out: changing the glob that **reads** changes/spec docs (`ChangesDirs`/`compileTemplate`/`FindSpecDocs`);
  fuzzy slug matching (only exact after stripping `<KEY>-`); associating bare `<KEY>` worktrees; building
  a canonical URL from the key; validating against the tracker; multiple tickets per spec; auto-discovering
  the provider without `defaultTicketProvider`.

Authored spec: `.vector/specs/detect-ticket-from-worktree-name/spec.md`.
