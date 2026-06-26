# Release standup-digest: embed, reinstall and UAT

## Why

The `add-standup-digest` feature is implemented and gate-green on
`feat/board-panel-and-apply` (commit `1ee8a26`), but it is **not shippable** yet: the global
binary in `~/.local/bin/vector` is stale — it lacks the embedded StandupView/SpecTimeline and
the `vector standup` / `vector spec worklog` subcommands. So `/vector:standup` cannot run
end-to-end for the user, and `vector serve` serves the panel without the standup view. Closing
the ticket needs the web build embedded into the binary, a recompile + reinstall, and an
exhaustive manual UAT proving the feature works end-to-end.

## What changes

- **Re-embed**: build the frontend (`npm --prefix web run build`) and copy `web/dist` into
  `cli/internal/webui/dist/` (the source of `//go:embed all:dist`), overwriting the previous
  build and committing it — binary + assets versioned together (distribution-packaging rule).
- **Kit asset sync**: `go -C cli generate ./internal/scaffold` (idempotent; the `webui` embed
  reads `dist/` directly and needs no generate).
- **Recompile + reinstall**: `go -C cli build -o ~/.local/bin/vector ./cmd/vector`, replacing the
  stale binary on PATH so `/vector:standup` and the worklog inside `/vector:apply` work for the
  end user.
- **Exhaustive manual UAT** as the close criterion: `vector serve` + board, the full
  `/vector:standup` flow, every defined edge case (invalid `--since`, empty period, invalid
  digest JSON that does NOT advance the marker, 404/400 from the API), UI states, and a
  no-regression check.
- **Docs**: record the UAT in `docs/uat.md` and reflect the post-release state in `docs/status.md`.

## Capabilities

<!-- No capability deltas: this is a release/closeout + verification change. It packages,
     installs, and verifies the existing standup-digest / activity-worklog capabilities
     (delivered by add-standup-digest); it adds no new behavior and changes no spec. -->

### New Capabilities
<!-- None. -->

### Modified Capabilities
<!-- None: no behavior or contract change; embed + reinstall + UAT only. -->

## Impact

- `cli/internal/webui/dist/` (embedded build, committed); `docs/uat.md`, `docs/status.md`.
- Out-of-repo: `~/.local/bin/vector` (reinstalled binary).
- No new dependencies; no code changes to the feature; no API/state-machine change.
- Out of scope: merging `feat/board-panel-and-apply` to `main` (the branch carries 8 commits
  beyond standup-digest; that merge is a separate step).

Authored spec: `.vector/specs/release-standup-digest/spec.md`.
