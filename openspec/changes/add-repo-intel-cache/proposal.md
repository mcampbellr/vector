# Add repo-intel cache (class C) with per-domain fingerprint invalidation

## Why

Every `/vector:*` command re-inspects the repo (techstack, framework, runtime, structure,
entry points) on each invocation, burning tokens and re-reasoning over facts that rarely
change (finding #3 of `docs/orchestration-review.md`). The `vector-context-cached-setup` spec
cached the *setup* (build/lint/test, language) in `config.json`, but that cache only
invalidates by manually re-running `vector update` — there is no automatic staleness oracle.

This change adds the missing **class C** knowledge layer (`docs/knowledge-architecture.md`):
a local, gitignored, regenerable cache that the Go binary produces and consumes, validated by a
**per-domain sha256 content fingerprint over the working-tree**, so commands can be served
already-resolved, verified-fresh repo context — regenerating only the knowledge whose
fingerprint changed, never serving stale info that would degrade an agent's work.

## What changes

- New gitignored `.vector/cache/` (sibling of `.vector/local/` / `.vector/tmp/`) with three JSON
  artifacts: `fingerprints.json` (validity oracle), `repo-intel.json` (stack/framework/runtime +
  tsconfig paths), `structure-index.json` (workspace tree + entry points from classified
  `git ls-files`). `.vector/cache/` added to `.gitignore`.
- **Fingerprint = per-domain sha256 over the working-tree** (not HEAD commit hash, not mtime).
  Five fixed domains (`stack`, `deps`, `build`, `workspace`, `structure`), each with an
  authoritative source set; domains hashed in parallel (goroutines + `sync.WaitGroup`,
  `crypto/sha256` stdlib).
- Recompute-on-read invalidation per domain, a dependency DAG between domains
  (`stack → deps`, `structure → entry points`), and `schemaVersion`/`kitVersion` bump invalidates
  everything.
- New `cli/internal/intel` package (one concern, no `util`/`common`): per-domain fingerprint
  logic + `repo-intel.json`/`structure-index.json` generation + atomic read/validate/write of the
  cache. Consumed by `cli/cmd/vector/context.go`.
- Extend `vector context`: validate the cache by per-domain content-hash before returning derived
  values and regenerate the stale domain; `--refresh` forces full regeneration; `--for <command>`
  projects only the slice a command needs via a static command→domains map **in the binary** (not
  the `.md` files this phase). Backward-compat: `vector context --json` with no new flags still
  returns the current `ContextOutput` (new fields additive with `omitempty`).
- Materialize the "do-not-persist" anti-pattern: `dep-graph.json` is never generated, git metadata
  is always recomputed, tool availability is never persisted, detected conventions are not
  auto-written to `.claude/`.
- Table-driven tests for deterministic digest, on-read invalidation, DAG, working-tree-vs-HEAD,
  version bump, `--for` projection, `--refresh`, and edge cases.

## Scope

- In: `.vector/cache/` + three JSON artifacts, per-domain working-tree fingerprint (5 domains +
  DAG), the `cli/internal/intel` package, the `vector context` extension (`--refresh`, `--for`,
  on-read validation, static command→domains map), `.gitignore` entry, table-driven tests.
- Out: wiring the validation tiers (TRUST / LAZY-VALIDATE / FULL-VALIDATE) into the kit `.md`
  files and re-vendoring (next phase — this phase leaves the map + `--for` ready in the binary);
  per-(domain × workspace shard) granularity in monorepos; `board.json` to disk / move to cache
  (inert — `internal/board` is a pure in-memory projection over SSE, never written to disk);
  `dep-graph.json` as an artifact; filesystem watchers / real-time invalidation; knowledge daemon;
  CI command detection; touching invariants (CLI-owns-writes, state machine, per-spec sharding,
  append-only activity).

Authored spec: `.vector/specs/add-repo-intel-cache/spec.md`.
Design doc: `docs/knowledge-architecture.md`.
