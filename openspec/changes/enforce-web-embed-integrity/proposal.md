# Enforce embedded web board integrity

## Why

Reinstalling the `vector` binary from a worktree that has not rebuilt+re-embedded `web/` ships a
**silently broken board**: the embedded `index.html` references hashed `/assets/*` that are not in
the binary, so `vector serve` renders BLANK. `go build` never rebuilds `web/`, so any worktree
without a fresh `web/dist` embeds a stale/missing frontend with no error. Today the only defense is
a **runtime** WARNING in `internal/webui` (it detects the breakage at `serve` time but does not
prevent it). It keeps recurring — it just happened again in a real repo after a reinstall.

The old doctrine punted to runtime-only because, with `dist/assets/` gitignored but a **hashed
`index.html` tracked**, a clean checkout is byte-indistinguishable from a broken embed (both:
`index.html` referencing absent `/assets/*`). This change makes that distinction decidable and
moves the failure to build/install time.

## What changes

- **Canonical install path** (`make install` / `scripts/dev-install.sh`) as the ONLY sanctioned
  reinstall-from-source: always `web build` → re-embed → guard → `go build` → install; **aborts
  before overwriting** `$VECTOR_INSTALL_DIR` if the embed would be broken. Covers the global
  `~/.local/bin/vector` reinstall.
- **Build-time guard** `TestEmbeddedBoardIntegrity` (`internal/webui`) reusing `ValidateAssets`:
  every `/assets/*` referenced by the embedded `index.html` must exist in the embed.
- **Tracked placeholder** `dist/index.placeholder.html` (no `/assets/*` refs) replaces the tracked
  hashed `index.html` (now gitignored). The unbuilt state carries no hashed refs → guard PASSES on
  a clean checkout (CI `go` job stays green without a `web build`); a partial/stale embed FAILS.
- **Serve fallback**: `spaHandler` serves the placeholder when `dist/index.html` is absent, instead
  of a blank board. The existing runtime WARNING stays as a secondary net (not removed).
- **Doctrine update**: `.claude/rules/architecture/distribution-packaging.md` reverts the
  runtime-only claim; also reconcile the now-false comments in `.gitignore:15-16` and `webui.go:1-5`
  that already asserted a "committed placeholder index.html" that never existed.

## Scope

- In: the `Makefile`/`dev-install.sh` canonical path, the build-time guard test, the tracked
  placeholder + `.gitignore` change, the `webui.go` serve fallback + doc-comment fix, the doctrine
  edit.
- Out: implementing anything beyond the above; tracking `dist/assets/` in git; `go:generate` for the
  web build; removing/weakening the runtime WARNING; changing the CI `go` job to run `web build`;
  the end-user release installers (`scripts/install.sh` / `install.ps1`).

Authored spec: `.vector/specs/enforce-web-embed-integrity/spec.md`.
