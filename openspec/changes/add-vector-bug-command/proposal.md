# Add /vector:bug command with cause traceability (relatedTo[])

## Why

Reporting a bug today means manually figuring out *what previous work caused it* and wiring that
relationship by hand — or not recording it at all, so the board loses the "why this bug appeared"
trail. The user relies on the global `/bug` skill, but it is somnio-specific and lives outside
Vector. Vector needs the same capability, **agnostic to the user's code** and **distributable**
inside the binary (seeded by `vector init`), so any Vector-managed repo gets it. Being inside
Vector, the resulting bug becomes a board card whose **cause is automatically traced and queryable**
(the spec(s)/ticket(s) that originated it), visible on the card, the API, and the standup.

`/vector:bug` is the bug-framed equivalent of `/vector:raw`: it authors and registers a `draft`
card. It deliberately does **not** create the OpenSpec change or implement the fix — the user
continues with `/vector:propose` (creates the `fix-…` change, `draft → open`) and `/vector:apply`.

## What changes

- **Project command `/vector:bug`** (`kit/commands/vector/bug.md`): parse the raw report, resolve an
  optional `{spec-id|branch|file}` token, **deduce the root cause** (cheap, main loop) via
  `git blame`/`git log -S`/`--grep` mapping suspect commits to a Vector spec (OpenSpec change name /
  id) or a ticket (commit trailer), delegate refinement to a **Haiku** agent, compose the canonical
  20-section spec (bug-framed), validate with the existing **Sonnet** `vector-spec-validator`, and
  register the `draft` card with `relatedTo[]` via the binary. **Ends in `draft`.** Infer, then ask:
  on ambiguity / multiple / low-confidence / no match → `AskUserQuestion`. Never guess.
- **Refiner agent `vector-bug-refiner`** (`kit/agents/vector-bug-refiner.md`), tier **Haiku**,
  read-only: turns the raw report into a structured brief (problem / expected / actual / reproduction
  / acceptance / test plan / risks / open questions), agnosticized from the global `/bug`. Surfaces
  ambiguity; never invents product intent.
- **New state field `relatedTo[]`** on `SpecState` (`cli/internal/state/types.go`): optional list of
  cause→bug relations. Each item `{kind, ref, source}` with `kind ∈ {spec, ticket}`, `ref`
  (Vector spec id or `provider:key`), `source ∈ {blame, manual}`. Persisted in `state.json`
  (`omitempty`, backward-compatible), projected on the board API, shown read-only on the web card.
- **CLI-owns-writes for relations**: seed at create (`vector spec create … --related '<json>'`) and,
  outside create, `vector spec relate <id> --kind <k> --ref <r> [--source blame|manual]` to add/
  manage relations (idempotent on `{kind,ref}`). Each write appends a `spec.related` event to
  `activity.jsonl`.
- **`--json` flag on `vector spec list`**: today it only prints text columns; JSON output lets cause
  deduction resolve commits → spec ids robustly, without fragile text parsing.
- **Board/web read-only surface**: `Card` projection exposes `relatedTo`; `GET /api/board` includes
  it per card; `SpecCard` renders relation chips (no editing — mirrors the existing ticket chip).
- **Vendoring**: command + agent embedded into the binary via `go generate`
  (`cli/internal/scaffold`), so `vector init`/`update` seed them.

## Capabilities

### New Capabilities

- `bug-authoring`: turn a raw bug report into a validated 20-section Vector spec (Haiku refiner +
  Sonnet validator) and register it as a `draft` card, agnostic to the user's repo.
- `cause-traceability`: deduce the bug's root cause via `git blame`/`git log`, map suspect commits
  to a Vector spec or external ticket, and persist it as a queryable `relatedTo[]` field with a
  `spec.related` activity event — inferring when confident, asking when ambiguous.

### Modified Capabilities

- `state-model`: `SpecState` gains the optional `relatedTo[]` field and the `spec.related` event;
  the board `Card` projection and `/api/board` gain `relatedTo` as a read-only field.
- `spec-cli`: `vector spec create` gains `--related`; `vector spec list` gains `--json`; new
  `vector spec relate` subcommand.

## Impact

- `kit/`: new `commands/vector/bug.md` and `agents/vector-bug-refiner.md`.
- `cli/internal/state/`: `types.go` (`RelatedItem`, `RelatedTo`, enums), `store.go` (`RelateSpec`,
  create persistence), event type `spec.related`; `store_test.go`.
- `cli/cmd/vector/`: `main.go` (`--related`, `--json` on list, `spec relate`, usage), `ticket.go`
  (`parseRelatedFlag`/`parseRelateFlags`, mirroring `parseTicketFlag`).
- `cli/internal/board/`: `board.go` (`relatedTo` in `Card`); `board_test.go`. `server.go` only via
  the projection — **no new write endpoints**.
- `web/`: `src/types/board.ts` (`RelatedItem` + `relatedTo?`), `SpecCard.tsx` (+ `.module.css`)
  read-only chips.
- `cli/internal/scaffold/assets/`: embedded copies regenerated via `go generate` (no manual edit);
  `scaffold_test.go` only if it enumerates the command/agent set.
- `docs/`: `plugin-and-commands.md`, `schemas/state-and-activity.md`, `domain-contract.md`.
- No new external dependencies (Go stdlib + system `git`). No HTTP write surface added.

Authored spec: `.vector/specs/add-vector-bug-command/spec.md`.
