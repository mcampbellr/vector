# Design — release-standup-digest

## Context

`add-standup-digest` is implemented and gate-green on `feat/board-panel-and-apply`. The web panel
is embedded via `embed.FS` (`cli/internal/webui/webui.go`, `//go:embed all:dist`), and the
release pipeline builds `web/` before `cli/` so the panel ships inside the binary
(`architecture/distribution-packaging.md`). The installed global binary predates the feature, so
it has neither the new UI nor the `standup`/`worklog` subcommands. This change packages, installs
and verifies — it writes no feature code.

## Goals / Non-Goals

**Goals:**
- Re-embed the real `web/dist` into `cli/internal/webui/dist/` and commit it (no drift API↔UI).
- Recompile + reinstall `~/.local/bin/vector` so the feature works end-to-end for the user.
- Exhaustive manual UAT covering the full flow, every edge case, UI states, and no-regressions.

**Non-Goals:**
- Re-implementing or refactoring the feature (already gate-green).
- Merging the branch to `main` (separate step; the branch has 8 commits beyond standup-digest).
- Automating the build→embed→install pipeline (`install.sh` / release tooling is future).
- New standup features (`/vector:daily`, export, templates, burndown).

## Decisions

- **Manual re-embed + commit** (not automated): the distribution rule requires versioning
  binary + assets together; the automated pipeline is a later phase.
- **Reinstall via `go build -o ~/.local/bin/vector`**: the channel already documented in
  `docs/status.md`; `install.sh` is future.
- **Exhaustive UAT as the close gate**: this is the last verification before `/vector:close`; a
  quick UAT would leave the feature's defining edge cases unchecked.
- **No capability delta**: a release/closeout + verification change carries no spec-delta — it
  changes no behavior or contract.
- **Embed needs no `go generate`**: `webui` embeds `dist/` directly; only the kit scaffold has a
  generate directive (run for completeness).

## Surface

- `cli/internal/webui/dist/`: replaced with the production build of `web/dist` (committed).
- `docs/uat.md`: append the standup-digest UAT checklist + results.
- `docs/status.md`: note the reinstalled binary and that the ticket is ready to close.
- `~/.local/bin/vector`: recompiled binary (out of repo).

## Risks / Trade-offs

- **API↔UI drift**: if `web/dist` is not rebuilt before the copy, the panel ships stale. Mitigate:
  `npm run build` immediately before the copy; verify `index.html` + `assets/` present.
- **Secrets in the embed**: the copy must include only the production build, never `.env`/sources.
  Mitigate: review the copied `dist/` contents.
- **Marker/digest desync**: `standup commit` must write the digest and advance the marker together
  (mutex in `Store.WriteStandup`); UAT verifies the invalid-JSON path writes nothing and does not
  advance.
- **UAT depth vs effort**: exhaustive UAT costs time, but it is the close criterion — accepted.
