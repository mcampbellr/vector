# Design — add-vector-check-command

## Pattern

**Slash-command-orchestrated, binary-projects / Haiku-writes-prose** — the same pattern as
`/vector:standup`, extended with an external source (Jira). Hard doctrine: **the Go binary never
talks to Jira and never handles credentials.** Jira is read exclusively through the Jira MCP
already connected in Claude Code, from the **command** (markdown), never from the binary. This
forces two orchestration layers:

1. `vector check --json` (binary) — deterministic local, no network: state machine, ticket
   presence/shape, git drift. Emits `CheckReport`.
2. `/vector:check` (command) — runs (1), reads each `specs[]` ticket via the Jira MCP (unless
   `--skip-jira`), appends the Jira-derived discrepancies to the same `discrepancies` array, and
   passes the combined JSON to `vector-check-reporter` (Haiku). `--fix` also lives here: per-item
   confirmation then `vector spec status|link|close`.

## Layers affected

- presentation / web board: **no** (out of scope; board untouched).
- application / CLI (`cli/cmd/vector`): **yes** — new `newCheckCmd()` (`check.go`), registered in
  `root.go`; `"check": tierTrust` in `context.go`.
- domain/check (`cli/internal/check`, **new package**): `CheckReport`, `Discrepancy`, `Note`,
  `SpecCheckable`, `Run` (local sweep) + git drift (`os/exec`).
- domain/gitexec (`cli/internal/gitexec`, **new package**): `Output(repoRoot, args...)` extracted
  from `fingerprint.go`.
- domain/intel (`cli/internal/intel`): `fingerprint.go` migrated to `gitexec.Output` (3 call
  sites; `gitOutput` removed) — null-behavior refactor.
- domain/config, domain/state: **no** — existing resolvers / readers reused, no new fields, no
  writes.
- kit (`kit/commands`, `kit/agents`): new `check.md` + `vector-check-reporter.md` (+ regenerated
  scaffold embeds).

## Key decisions (locked — §10 of the spec)

- `/vector:check` is a **new, independent** command; `/vector:status` is unchanged.
- Binary never contacts Jira; Jira only via the MCP, only from the command.
- Default = report-only (read-only); `--fix` strictly opt-in, per-item confirmation, only
  `fixEligible: true`.
- V1's only fix-eligible type is `ticket-closed-while-open` → `vector spec close <id>` (always a
  legal transition from `open|in-progress|review` per `state.CanTransition`). Every other type
  stays manual (`invalid-state`, `no-ticket`, `malformed-ticket-ref`, `missing-branch`,
  `unmerged-vs-base`, `broken-ticket-link`).
- Jira is optional per spec: `no-ticket` is Low/advisory; only a linked-but-broken/closed ticket
  is High.
- Audit history is ephemeral in V1 (no new persisted artifact).
- Dirty working tree on an `in-progress` card is a `Note`, never a `Discrepancy`.
- Prose language reuses `config.ResolvedLanguage()` (same as `standup.Projection.Language`).
- `vector check` tolerates a repo with no `.vector/config.json` (runs degraded — no `Language`,
  no `[branch]` git check).
- Reuse the `git` helper via extraction to `cli/internal/gitexec/` (not duplication); the
  `fingerprint.go` refactor is null-behavior and its tests must stay green.

## Expected flow

1. Dev runs `/vector:check` (or `--fix` / `--skip-jira`).
2. Command runs `vector check --json --repo-root "$REPO_ROOT"`. Binary: list non-archived specs;
   validate `status` (`invalid-state`); classify ticket (`no-ticket` / `malformed-ticket-ref` /
   pass through to `specs[]`); when `cfg.HasBranchPlaceholder()`, derive the expected branch
   (`cfg.BranchPrefixOrDefault() + spec.ID`) and check `missing-branch` / `unmerged-vs-base` via
   read-only `git`; resolve `report.Language = cfg.ResolvedLanguage()`; emit `CheckReport`.
3. Unless `--skip-jira`, iterate `specs[]` with a ticket, read each via the Jira MCP, append
   `ticket-closed-while-open` / `broken-ticket-link` to `discrepancies`.
4. Pass the combined JSON to `vector-check-reporter` (Haiku), prepending `Write the prose in:
   <language>` when `report.language` is non-empty.
5. Agent returns `{summary, bySeverity:{high,medium,low}, notes}`.
6. Command prints the report grouped by severity with the exact remediation command per
   discrepancy (low collapses to a count line beyond 5 entries).
7. If `--fix`: per `fixEligible: true` item, `AskUserQuestion` then `vector spec close <id>
   --json`; report per-item outcome; never touch `fixEligible: false`.

## New/changed files

```txt
cli/internal/gitexec/gitexec.go            NEW   Output(repoRoot, args...) extracted from fingerprint.go
cli/internal/gitexec/gitexec_test.go       NEW
cli/internal/intel/fingerprint.go          EDIT  drop gitOutput; migrate 3 call sites to gitexec.Output
cli/internal/check/check.go                NEW   CheckReport, Discrepancy, Note, SpecCheckable, Run
cli/internal/check/git.go                  NEW   branchExists, worktreeExists, unmergedCount (uses gitexec)
cli/internal/check/check_test.go           NEW
cli/cmd/vector/check.go                     NEW   newCheckCmd()
cli/cmd/vector/check_test.go                NEW
cli/cmd/vector/root.go                       EDIT  register newCheckCmd()
cli/cmd/vector/context.go                    EDIT  "check": tierTrust
kit/commands/vector/check.md                 NEW   orchestration (local → Jira MCP → reporter → --fix)
kit/agents/vector-check-reporter.md          NEW   Haiku reporter (tools: Read)
cli/internal/scaffold/assets/commands/vector/check.md         REGEN  go generate
cli/internal/scaffold/assets/agents/vector-check-reporter.md  REGEN  go generate
```

## Open questions (carry into implementation — do not assume silently)

1. **Jira MCP response shape** — no prior `/vector:jira` or versioned `.mcp.json` to calibrate;
   confirm the exact tool name and the Jira status-category → "closed" mapping before wiring
   `check.md` §2.
2. **Git drift signal exactness** — is `missing-branch` based on `git rev-parse --verify`,
   `git worktree list --porcelain`, or both? Confirm severities (`missing-branch` High /
   `unmerged-vs-base` Medium, proposed). Dirty-tree only for the currently-checked-out worktree
   (bare+worktree: the binary runs from one dir).
3. **Audit history** — ephemeral (assumed) vs persisted `.vector/local/checks.jsonl`; if
   persisted, define schema + consumer in a future phase.
4. **Jira MCP read parallelism/timeout** across N ticketed specs — V1 sequential; confirm if a
   many-ticket repo needs parallelization or an explicit per-read timeout.
5. **Web-board health badge** — confirmed out of scope; possible future extension consuming
   `CheckReport` via a new HTTP endpoint.
