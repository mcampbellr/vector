# Integrate a graphify MCP bridge as graph context for Vector kit agents

## Why

Today `vector-spec-refiner.md:29` already tries to passively read
`graphify-out/GRAPH_REPORT.md` when it exists, but no kit agent can **query** the graph
interactively (a node's neighbors, communities, shortest path, "god" nodes), and there is no
installed path to generate or serve that graph from the Vector binary. graphify already indexes a
repo as a knowledge graph of code↔code and code↔spec relations; exposing it over MCP lets any kit
agent enrich its work with real repository structure — **opt-in, never mandatory**.

## What changes

- **`vector graph mcp`** — new binary subcommand acting as a **stdio bridge**: detects graphify +
  `graphify-out/graph.json`; when both are present, shells out to
  `python3 -m graphify.serve graphify-out/graph.json` and proxies its stdio transparently; when
  either is missing, **fails gracefully** (actionable stderr message + non-zero exit), breaking
  nothing else in Vector.
- **`vector graph build`** — new subcommand, manual entry point to (re)generate the graph. The
  exact graphify invocation stays `TBD` (see Open questions); the subcommand skeleton
  (detection + shell-out + report) is specified, guarded by an explicit error until wired.
- **`--with-graph-mcp` flag** on `vector init` and `vector update` — opt-in seed that writes/merges
  a `vector-graph` entry into `.mcp.json` at the user repo root, whose `command` points at the
  `vector` binary (`graph mcp`), never at `python3` directly. **Never automatic**: without the flag,
  `init`/`update` never touch `.mcp.json`.
- **New package `cli/internal/graph`** — detection (graphify installed / `graph.json` present) +
  construction of the `serve`/`build` `exec.Cmd`s; no dependency on `internal/config`/`internal/state`.
- **New shared doctrine `kit/agents/_shared/graph-context.md`** — what the 7 graph tools are, how to
  use them opt-in, how to degrade cleanly when the server is not connected.
- **4 kit agents adopt the graph tools (opt-in)** — `vector-spec-refiner`, `vector-bug-refiner`,
  `vector-apply-impl`, `vector-feasibility-reviewer` each gain the 7 `mcp__vector-graph__*` tools in
  their frontmatter plus a short pointer to the shared doctrine. No step is made to require them.
- **Embedded scaffold assets regenerated** via `go generate ./internal/scaffold`
  (`TestAssetsMatchKit` stays green).
- **README** documents the new subcommands, the flag, and that graphify is an **optional** dependency.

## Scope

- **In**: `vector graph mcp` (graceful-fail + transparent proxy), `vector graph build` skeleton,
  `--with-graph-mcp` + `scaffold.SeedMCPConfig` (additive/idempotent merge of `.mcp.json`),
  `cli/internal/graph`, `_shared/graph-context.md`, the 4 agents' frontmatter + guidance, asset
  regeneration, README, and the associated tests.
- **Out**: bundling/installing graphify; pre-spec research via graph (`/vector:research` wiring); a
  board UI for the graph (`web/`); child-process supervision in Go (ownership is Claude Code's); the
  exact auto-regeneration trigger/cadence; the `graphify-out/` versioning decision; wiring agents
  beyond the 4 named.

## Impact

- **CLI** (`cli/cmd/vector`): new `graph.go` (`newGraphCmd`/`newGraphMCPCmd`/`newGraphBuildCmd`),
  registered in `root.go`; `--with-graph-mcp` in `newInitCmd`/`newUpdateCmd`; `usage()` update.
- **New domain** (`cli/internal/graph`): detection + command construction, spec-domain-agnostic.
- **Scaffold** (`cli/internal/scaffold`): new `SeedMCPConfig` writing **outside** `.claude/`, first
  Vector write to the user repo root — gated strictly behind the flag.
- **Kit** (`kit/agents`): 1 new shared doc + 4 modified agents + regenerated embedded copies.
- **Dependencies**: no new Go modules; graphify (Python) stays an **optional external** dependency,
  confirmed absent on this machine — the graceful-fail path is the real, testable case.
- **No changes** to `web/`, `kit/commands`, or Vector state (`.vector/specs`, `state.json`,
  `activity.jsonl`).
