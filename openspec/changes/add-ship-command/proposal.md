# Add /vector:ship command

## Why

`/vector:apply` implements a spec and leaves the card in `review` with an uncommitted working
tree — by design, apply implements, it does not ship. Turning that into a pull request today is a
manual, error-prone sequence a human runs by hand: commit the implementation, rebase onto the base
branch, generate PR title/body, push, open a draft PR. Every step has a repo-specific gotcha (auth
in a non-interactive shell, a base branch that is `develop` not `main`, untracked-file collisions
on rebase, draft-vs-ready CI policy). This should be one command — the natural successor to
`/vector:apply`, the same way `/vector:close` follows review — staying inside Vector's
**CLI-owns-writes** and state-machine discipline.

## What changes

- New `/vector:ship [spec-id]` project command (`kit/commands/vector/ship.md`) that orchestrates the
  ship sequence entirely in the kit (commit → rebase → PR-text → push → open draft PR → record
  link). Git/`gh` orchestration lives in the command, **not** in the binary.
- New `Ship *ShipConfig` block in `.vector/config.json` (`baseBranch`, `mode` ask|auto, `draft`
  `*bool`, `excludeGlobs`, `authBootstrap` — all optional, pass-through), plus resolvers. Written
  only by a new `vector config set-ship` subcommand (mirrors `set-jira-mcp`, incremental per-field).
  `SchemaVersion` stays 1 (additive, opt-in — not written by `init`/`update`).
- PR recording as a **distinct** concept from the ticket slot: new `PR *PullRequest` field on
  `SpecState` (`pr{url,number,draft,openedAt}`), new `EvtPROpened = "pr.opened"` event +
  `PROpenedData`, `Store.RecordPR` (lock→read→idempotency-on-URL→write+event, mirrors `LinkSpec`),
  and a `vector spec pr <id> <url> [--number N] [--draft] [--json]` subcommand as its sole writer.
  Recording a PR **never** transitions the spec's status.
- `vector context --json` surfaces a `ship` block (mirror of `jira`) with the resolved
  base/mode/draft/excludeGlobs/authBootstrap, present only when the repo configured `ship`.
- Preconditions enforced by the command: only a spec in `review` can be shipped; other states are
  refused with actionable guidance. Selection defaults to the spec in `review` (error if none,
  ask if several). Idempotent: an existing PR for the branch is surfaced and re-recorded, never
  duplicated.
- Guardrails: never force-push; default excludes `openspec/` (static) and the spec's own doc
  (resolved dynamically via `cfg.SpecDocPath`, never `git add -A`); auth bootstrap is opt-in only;
  secret scanning is delegated to the repo's own gate; stale-tree warning is non-blocking.

## Scope

- In: the `/vector:ship` command; the `ShipConfig` block + `vector config set-ship`; the PR
  recording surface (`PR` field, `pr.opened` event, `Store.RecordPR`, `vector spec pr`); the `ship`
  block in `vector context --json`; commit/rebase/push/draft-PR orchestration with untracked-
  collision handling and idempotency; table-driven tests + scaffold vendoring of `ship.md`.
- Out: merging the PR, CI babysitting, closing the spec (`/vector:close`), implementing the change
  (`/vector:apply`), multi-repo/batch ship, non-GitHub PR providers, a bundled secret scanner, a
  per-run `--draft`/mode override, board/web UI for the PR, wiring `pr.opened` into the standup
  digest, `SchemaVersion`/`EventVersion` bumps.

Authored spec: `.vector/specs/add-ship-command/spec.md`.
