# Isolate each spec in its branch-per-spec worktree at creation (`/vector:raw`, `/vector:bug`)

## Why

On repos that declare a bare+worktree layout (their resolved `spec-path`/`changes-path` contains
the `[branch]` placeholder), `/vector:raw` and `/vector:bug` write the spec doc to the resolved
path **without creating the git worktree**. The result is a loose `code/<slug>/` directory that
is not tracked in `git worktree list`, has no `feat/<slug>` branch, breaks per-branch isolation,
and blocks the downstream flow (`/vector:propose`, `/vector:apply`) — a later
`git worktree add code/<slug>` fails because the path already exists.

Root cause (deduced from git): the defect entered in commit `d91f8a5` (*"feat: powerful
/vector:raw, per-repo config, and vector update"*), where `vector spec create` began writing the
doc to the config-resolved location — including `[branch]`-resolved paths — but never created the
worktree (`store.CreateSpec` uses `os.MkdirAll`). Worktree-layout awareness was added later and
only on the **read** side (`vector sync`: `820f283`, `e25efde`, `5150d00`, `ed0988e`); the
**write/create** side was never made worktree-aware. No tracked spec card maps to those commits,
so this card carries **no `relatedTo[]`** (a hallucinated relation would be worse than none).

## What changes

- In `kit/commands/vector/raw.md` and `kit/commands/vector/bug.md`: a **worktree-resolve/create**
  step that runs **before** writing the spec doc / invoking `vector spec create --body-file`,
  gated on the resolved config declaring a worktree layout (the `[branch]` placeholder is present
  in `spec-path` or `changes-path`).
- Command run: `git worktree add <worktree-root>/<slug> -b <prefix>/<slug> <base-branch>`, where
  `<worktree-root>` is the literal template prefix before `[branch]` (e.g. `code`), `<base-branch>`
  comes from config (default `main`), and `<prefix>` is the configurable branch prefix
  (default `feat/`).
- **Idempotency**: if the slug already has an active worktree/branch (`git worktree list` lists it),
  reuse it without error or recreation.
- The spec doc is written **inside** the created/reused worktree (the binary-resolved path already
  falls under `<worktree-root>/<slug>/…`), so it stays tracked on the feature branch.
- **Inert on non-worktree repos**: if the resolved `spec-path` has no `[branch]` (e.g. Vector's own
  repo, `.vector/specs/<slug>/`), the step is skipped entirely — behavior identical to today.
- **(Conditional, Q-A)** if the orchestration cannot derive the layout context from the current
  `vector context --json`, extend `vector context` + a `config.go` helper (`HasBranchPlaceholder`,
  root/base-branch/prefix accessors) — additive, `SchemaVersion` unchanged.
- Single-source propagation: edit `kit/commands/vector/{raw,bug}.md` → `go generate
  ./internal/scaffold` → reinstall binary → `vector update`.

## Scope

- **In**: the conditional worktree-resolve/create step in `raw.md` + `bug.md`, regenerated assets
  (`TestAssetsMatchKit` green), the conditional `vector context`/`config.go` layout-context
  exposure with tests, and tests for worktree creation, idempotency, non-worktree regression,
  base/prefix defaults+override, and the actionable loose-stub failure.
- **Out**: recovering/cleaning pre-existing loose stubs (user's responsibility — actionable error,
  no auto-delete); moving the worktree logic into the binary (it stays in command orchestration —
  the binary remains worktree-unaware); changing `/vector:propose` / `/vector:apply` beyond
  inheriting the benefit (`TBD`, Q-B); `/vector:quick` (`TBD`, Q-C); state schema, board HTTP
  endpoints, or web.

Authored spec: `.vector/specs/fix-raw-bug-worktree-creation/spec.md`.
