# Add /vector:check command

## Why

Today nothing reconciles the status Vector persists in `.vector/specs/<id>/state.json` against
external reality. `/vector:status` (`kit/commands/vector/status.md`) only *moves* a card once a
human already knows it should move — there is no surface that **detects** drift in the first
place. A dev has to manually cross the board, Jira, and `git branch -a` spec by spec to find
"lying" cards (closed in Jira but open on the board, in a status outside the state machine, with
a branch that no longer exists). `/vector:check` collapses that into a single read-only
invocation.

## What changes

- New `vector check --json` binary subcommand (read-only, no network): sweeps non-archived specs
  and emits a `CheckReport` JSON with three families of **local** checks — state-machine validity
  (`invalid-state`, High), ticket presence/shape (`no-ticket` Low, `malformed-ticket-ref` High),
  and git drift (`missing-branch`, `unmerged-vs-base`; dirty tree on an `in-progress` card is a
  **Note**, never a discrepancy). The binary **never contacts Jira** — it only exposes each
  spec's ticket ref for the command to audit.
- New `/vector:check` project command (`kit/commands/vector/check.md`): runs `vector check
  --json`, and for each spec with a ticket reads it via the **Jira MCP** already connected in
  Claude Code, adding `ticket-closed-while-open` (High, fix-eligible) / `broken-ticket-link`
  (High) discrepancies. `--skip-jira` runs local + git checks standalone.
- New Haiku subagent `kit/agents/vector-check-reporter.md`: turns the combined `CheckReport` into
  prose grouped by severity plus a ranked, copy-pasteable action list (the exact remediation
  command per discrepancy). Prose language reuses `config.ResolvedLanguage()` (same mechanism as
  `/vector:standup`).
- `--fix` (opt-in, never default): after the report, offers to apply each `fixEligible: true`
  discrepancy **one by one with explicit `AskUserQuestion` confirmation**, invoking the existing
  `vector spec status|link|close` subcommands — never a new write primitive, never in batch. In
  V1 the only fix-eligible type is `ticket-closed-while-open` (`vector spec close <id>`).
- Reuse the `git` subprocess helper by extracting `gitOutput` from
  `cli/internal/intel/fingerprint.go` into a shared `cli/internal/gitexec/` package
  (`Output(repoRoot, args...)`), consumed by both `fingerprint.go` (3 call sites migrated) and
  the new `check/git.go` — a null-behavior refactor.

## Scope

- **In**: `vector check --json` (+ `--repo-root`); the `cli/internal/check` package
  (`CheckReport`, `Discrepancy`, `Note`, `SpecCheckable`, `Run`, git drift via `os/exec`); the
  shared `cli/internal/gitexec` package and `fingerprint.go` migration; `newCheckCmd()` registered
  in `root.go` + `"check": tierTrust` in `context.go`; `kit/commands/vector/check.md`;
  `kit/agents/vector-check-reporter.md`; regenerated scaffold embeds; table-driven tests + gate.
- **Out**: persisted audit history (`.vector/local/checks.jsonl`); any web-board health badge or
  HTTP endpoint; auto-repair without confirmation (even with `--fix`); discrepancy types beyond
  those listed (priority/estimates/`stage`); extending/replacing `/vector:status`;
  reintroducing Jira credential/network handling in the binary; checking `archived` specs;
  new external Go dependencies; a new provenance value on `transitionOpts.trigger`.

Authored spec: `.vector/specs/add-vector-check-command/spec.md`.
