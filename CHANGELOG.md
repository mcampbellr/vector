# Changelog

All notable changes to Vector are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and Vector adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Releases are cut by
pushing a `vX.Y.Z` tag, which triggers the GoReleaser pipeline
(`.github/workflows/release.yml`).

## [0.4.0] - 2026-07-21

### Added

- **`/vector:ship` command** — land a reviewed spec as a pull request
  (commit → rebase onto the base branch → generate PR text → push → open a draft
  PR → record it on the card). Backed by new binary surface: a `ShipConfig` block
  in `.vector/config.json` (base branch, `ask`/`auto` mode, draft, exclude globs,
  auth bootstrap) with nil-safe resolvers; `vector config set-ship` (incremental
  per-field writer); `vector spec pr <id> <url>` (idempotent PR recorder, no
  status transition); a `PR` field and `pr.opened` event on the spec state; and a
  `ship` block in `vector context --json`, present only when configured.
- **Structured `needs-attention` reason** — the free-form blocker note is now a
  structured shape (category + summary + markdown detail) across the state
  machine, CLI transitions, board projection, and the web `SpecCard` /
  `SpecDetailsDrawer` (new `AttentionCategoryChip`).

### Changed

- **Wireframe opt-in is now an explicit two-option question** — the Excalidraw
  sketch confirmation in `/vector:raw` and `/vector:research` is a bounded
  `AskUserQuestion` select (*Generate wireframe* / *Skip*) instead of a free-text
  prompt the model rendered inconsistently.

### Fixed

- **`vector config set-ship` is now reachable** — the `config` command family was
  implemented but never registered in the command tree, so the documented way to
  configure the ship block (`vector config set-ship …`) returned `unknown command`
  and exit 2. It is now a proper cobra command hung off the root, with a
  dispatch-level regression test guarding the wiring.
- **Blank-board guard (`vector serve`)** — a binary built from a worktree with no
  web build embedded an `index.html` referencing absent (gitignored) `/assets/*`,
  rendering the board blank with a silent `200`. `vector serve` now prints a loud
  startup warning with the exact rebuild+re-embed command, `spaHandler` returns a
  real `404` for missing assets (no SPA fallback), and `webui.ValidateAssets` /
  `EmbeddedAssetsMissing` report the broken references.
- **Spec card ticket badge** no longer clips under long titles.

## [0.3.0] - 2026-07-07

### Added

- Adopted **cobra + lipgloss** for the command tree (`vector completion <shell>`,
  styled `--help`), keeping the `--json` contract byte-identical.
- **Deterministic standup digest** with a one-paragraph-per-item engineering
  format, and a redesigned web Standup tab for scannable per-item reading.
- `sketch.attached` event emitted by `Store.AttachSketch`; sketch badge shown on
  cards; canonical sketch filenames.

## [0.2.0] - 2026-07-01

### Added

- **Windows support** in distribution (GoReleaser `.zip` archives + PowerShell
  installer).
- **Branch-per-spec worktree** isolation on spec creation.
- **Excalidraw design sketch generation** for UI-facing specs.

## [0.1.0] - 2026-06-29

- Initial public release: spec-driven kanban board for Claude Code — a single Go
  binary bundling the CLI, the board HTTP API + SSE, and the embedded web panel;
  OpenSpec-backed specs projected onto a state-as-columns board; token-routed
  `/vector:*` project commands; one-step install script.

[0.4.0]: https://github.com/mcampbellr/vector/releases/tag/v0.4.0
[0.3.0]: https://github.com/mcampbellr/vector/releases/tag/v0.3.0
[0.2.0]: https://github.com/mcampbellr/vector/releases/tag/v0.2.0
[0.1.0]: https://github.com/mcampbellr/vector/releases/tag/v0.1.0
