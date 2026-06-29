# Add /vector:research command to assess feasibility before specifying

## Why

`/vector:raw` refines a raw idea and emits a spec, but it never asks whether the idea is
**worth building**. A dev who wants to validate feasibility before committing to a 20-section
spec has no first-class path: is it technically feasible with this stack? are there security
risks? does it make commercial sense? does it need design? Vector needs the "exhaustive sibling"
of `raw` — a command that **investigates, evaluates, decides, and only then emits** an enriched
spec, so the dev can say "no-go" with evidence and avoid speccing dead ideas. Being inside Vector
and self-contained, it reuses the binary's write path and the existing authoring pipeline; no new
Go code, events, or endpoints.

## What changes

- **Project command `/vector:research`** (`kit/commands/vector/research.md`): orchestrates idea
  intake, cheap lens auto-detection, user refinement, the per-lens feasibility reviews, verdict
  consolidation, an explicit **go/no-go gate**, composition of the 20-section spec with the
  embedded feasibility report, and registration of the `draft` card via the binary.
- **Feasibility reviewer agent** (`kit/agents/vector-feasibility-reviewer.md`), tier **Sonnet**,
  read-only: parameterized by **lens** (`technical` | `security` | `marketing` | `design`),
  gathers its own evidence (Read/Grep/Glob) and returns a structured per-lens verdict
  (`go` / `go-with-risks` / `no-go` + confidence `N/10` + findings + risks + recommendation).
- **Lens auto-detection** (cheap, in the main loop): `technical` always runs; `security` /
  `marketing` / `design` activate on textual signals; ambiguity → `AskUserQuestion` to adjust the
  set (no forcing, never all four "just in case").
- **Reuse of the `/vector:raw` authoring pipeline** (self-contained): refinement uses the existing
  `vector-spec-refiner` (**Haiku**) and final validation uses `vector-spec-validator` (**Sonnet**),
  composed into `research`'s own flow — `research` does **not** invoke `/vector:raw` externally.
- **Explicit go/no-go gate**: after consolidating verdicts, the command asks the user whether to
  emit the spec (recommendation derived from the consolidated verdict, but **the human decides**);
  on abort, no card is created.
- **20-section spec + embedded feasibility report**: the canonical template
  (`.claude/vector/spec-template.md`) plus a `## Reporte de viabilidad` annex (per-lens verdict
  table + findings + risks) appended after §20, alongside `Open questions`.
- **Draft card registration** reusing the existing `vector spec create --status draft
  --body-file - --json`; token accounting reusing `vector spec route` (`agent.routed` per step).
- **Vendoring**: command and agent embedded into the binary via `go generate`
  (`cli/internal/scaffold`), so `vector init` seeds them.

## Capabilities

### New Capabilities

- `feasibility-research`: investigate a raw idea across the applicable lenses (technical always;
  security/marketing/design on demand) with Sonnet reviewers, consolidate a global verdict, and
  gate go/no-go with the user before any spec is written.
- `feasibility-enriched-spec`: when the gate passes, author the full 20-section Vector spec with
  the per-lens feasibility report embedded as an annex, and register it as a `draft` card via the
  binary — reusing the `raw` refiner+validator pipeline, no external command invocation.

### Modified Capabilities

<!-- None: reuses existing `vector spec create` and `vector spec route`; no Go code, events,
endpoints, or UI change. -->

## Impact

- `kit/`: new `commands/vector/research.md` and `agents/vector-feasibility-reviewer.md`.
- `cli/internal/scaffold/assets/`: embedded copies regenerated via `go generate` (no manual edit);
  `scaffold_test.go` only if it enumerates the command/agent set.
- **No** changes to `cli/` Go code, `cli/internal/state`, `cli/internal/board`, or `web/`.
- No new dependencies. Reuses `vector spec create` (`cli/cmd/vector/main.go:568,709`) and
  `vector spec route` (`cli/cmd/vector/main.go:592`).
- `docs/plugin-and-commands.md`: add `/vector:research` if it enumerates the `/vector:*` set.

Authored spec: `.vector/specs/add-research-command/spec.md`.
