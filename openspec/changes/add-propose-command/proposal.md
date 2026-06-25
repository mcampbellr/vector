# Add /vector:propose command

## Why

Specs created by `/vector:raw` stay in `draft` with no OpenSpec change. Formalizing the work
required leaving Vector's flow and running OpenSpec by hand, leaving the board out of sync.
We need one operation that closes the gap `draft → open`.

## What changes

- New `/vector:propose <id>` project command + `vector spec propose <id>` binary subcommand.
- **Adapter**: when the repo is an OpenSpec project, delegate to its tooling (`openspec` CLI or
  `opsx:propose`/`openspec-propose` skills); otherwise a light native fallback writes the
  `proposal/design/tasks` artifacts from the authored spec doc. **Not** a full OpenSpec clone
  (no spec-delta model, no catalog) — that is an explicit non-goal.
- The card transitions `draft → open`, records `openspec{change,artifacts}` provenance, and logs
  `spec.proposed` + `status.changed`. No `startedAt` (work starts at `/vector:apply`).

## Scope

- In: the command, the binary state-flip, the adapter (delegate/native), idempotency, the
  worktree-location resolution.
- Out: implementing the work (`/vector:apply`), git worktree/branch management, full OpenSpec
  parity.

Authored spec: `.vector/specs/add-propose-command/spec.md`.
