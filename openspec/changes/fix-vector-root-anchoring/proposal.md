# Anchor `.vector` root resolution to the nearest ancestor canonical store

## Why

`resolveRepoRoot()` (`cli/cmd/vector/main.go:1018`) resolves the Vector root through exactly three
steps — explicit `--repo-root` → `git rev-parse --show-toplevel` → `os.Getwd()` — and **never walks
up** the directory tree to check whether a canonical store already exists above. `config.Resolve()`
(`cli/internal/config/config.go:604`) compounds it with a silent fallback (`VectorFallbackSpecPath`,
`SpecStore: vector`) when no config is found at the resolved root.

The combination lets an agent or a developer running `vector init` or `vector spec create` from a
subdirectory (e.g. `website/`) create a **nested, orphan `.vector/`** right there, fragmenting the
board across several stores and breaking the single-source-of-truth invariant documented in
`.claude/rules/architecture/state-model.md`.

Root cause (deduced): this is **not a regression** but a design gap present since the binary's
origin — the foundational commit `1f24d74` ("first vertical slice"), which predates Vector's own
spec model and maps to no board spec id. Per explicit user decision during clarification, the card
carries **no `relatedTo[]`**; a fabricated link would be worse than none.

## What changes

- **`config.FindAncestorConfig(startDir) (root string, strayDirs []string, found bool)`** (new, in
  `cli/internal/config/config.go`): walks up level by level checking `<dir>/.vector/config.json`.
  Valid config → that dir is the canonical root, stop. A `.vector/` **without** a parseable
  `config.json` is a *stray*: recorded for warning, **never adopted**, and the walk continues
  upward. Stops at the filesystem root; when nothing is found, current behavior is preserved.
- **`resolveRepoRoot()` gains the walk-up** when no explicit `--repo-root` is passed, tried before
  the current git-toplevel/cwd fallback. `--repo-root` stays **final** — precedence unchanged.
- **`vector init` gains an ancestor guard**: it computes its *raw target* (today's resolution, no
  walk-up) and looks for a valid ancestor **strictly above** it. If one exists and `!force`, `init`
  fails naming the ancestor path and suggesting `--force`; nothing is written. With `--force` it
  proceeds exactly as today — an explicitly consented override. Never silent-adopt.
- **Stray detection surfaces a `ui.Warning`** naming both the stray path and the canonical store —
  human branch only, never inside a `--json` branch (byte-identical `--json` guarantee).
- **`vector doctor`** (new command, `cli/cmd/vector/doctor.go`): bare `doctor` scans read-only for
  `.vector/` dirs without `config.json` below the canonical root; `doctor adopt <stray-path>`
  migrates the stray's specs, local state and `activity.jsonl` into the canonical store and deletes
  the stray — **only with explicit `--force`** (without it, prints the plan and mutates nothing),
  per `.claude/rules/security/destructive-ops-consent.md`.
- **Documented guardrail for agents/AI**: new shared snippet
  `kit/agents/_shared/root-anchoring-guardrail.md` plus notes in `kit/commands/vector/{raw,bug,
  quick}.md` and `kit/CLAUDE.md` — an ancestor `.vector/` is the single base; creating another one
  in a subdirectory is forbidden; `vector doctor` is the way to consolidate pre-existing strays.
- **Correction — nearest-wins alone is insufficient in a bare+worktree layout with a non-git
  root**: when the workspace root is *not* a git repo and each worktree carries its **own**
  tracked `.vector/config.json` (`specStore: vector`), a command run inside a worktree anchors to
  the worktree's own store, *shadowing* the canonical workspace-root store above it — the
  split-board bug Vector's own dogfooding hit. Nearest-wins is unchanged as the default, but two
  explicit **pins** now take precedence over it:
  1. **`VECTOR_REPO_ROOT` env var** — an ephemeral pin; when set and non-empty, its absolute value
     is the repo root and the walk-up is skipped entirely.
  2. **`stateRoot` config field** (new, top-level in `Config`, `omitempty`) — a persistent pin
     baked into a tracked `.vector/config.json`. When the config the walk-up anchors to carries a
     non-empty `stateRoot`, `resolveRepoRootStrays` re-anchors to it (resolved relative to that
     config's own directory when not absolute, then validated via `config.Load`). An invalid or
     self-referencing `stateRoot` is ignored, falling through to the nearest-wins result rather
     than erroring.
  - **Precedence (highest→lowest)**: explicit `--repo-root` > `VECTOR_REPO_ROOT` env >
    `stateRoot` (from the walk-up config) > walk-up nearest-wins > `git rev-parse
    --show-toplevel` > `os.Getwd()`.
  - **Shadowing warning**: when the walk-up anchors to a worktree store but a *further* ancestor
    store exists above it and no pin redirected there, `resolveRepoRootStrays` surfaces a
    `ShadowNotice` and human-branch callers (`update`, `spec create`, `serve`, `doctor`) print a
    `ui.Warning` naming both paths and suggesting `stateRoot`/`VECTOR_REPO_ROOT` — never inside a
    `--json` branch (byte-identical `--json` guarantee holds).
- `config.SchemaVersion` stays **1**: `stateRoot` is additive and `omitempty`, mirroring the
    existing `language` field precedent — no schema bump.

## Scope

- **In**: `config.FindAncestorConfig` + tests; walk-up in `resolveRepoRoot`; ancestor guard in
  `newInitCmd`; `doctor.go` (scan + `adopt`) registered in `root.go`; new tests
  (`root_resolution_test.go`, `doctor_test.go`, additions to `config_test.go`); golden suite
  verified free of drift on existing `--json` shapes; the `kit/` guardrail docs.
- **Out**: designing/enforcing subproject scoping over the already-existing `repo` field
  (`SpecState.Repo`, `cli/internal/state/types.go:129`) — this phase only *documents* the ban on
  nested stores; intentional multi-store monorepos; changing `vector update`'s behavior (no
  special-casing, it uses the general walk-up); the web UI (`web/`); new HTTP endpoints (`doctor`
  is terminal-only); advanced `activity.jsonl` merge/dedup beyond chronological append; and
  resolving walk-up vs git/worktree boundaries — all `TBD`, see Open questions.
- `config.SchemaVersion` stays at **1**: this is a path-resolution fix, not a schema change.

Authored spec: `.vector/specs/fix-vector-root-anchoring/spec.md`.
## Rebaseline — workspace root is authoritative

This section supersedes earlier nearest-wins and pin-precedence language. When more than one valid
ancestor store exists, Vector selects the outermost workspace store. A `--repo-root`,
`VECTOR_REPO_ROOT`, or `stateRoot` target inside that workspace is rejected; `init --force` cannot
create an inner store. `doctor adopt` locks the canonical store, completes collision preflight
before mutation, writes activity atomically, and rolls moved artifacts back after a later failure.
