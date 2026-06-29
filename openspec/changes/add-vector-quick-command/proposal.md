# Add /vector:quick command for small one-run changes

## Why

Today, dispatching a small low-risk change (a refactor, a symbol rename, an extracted helper, a copy
tweak, a missing index, a promoted file) through Vector means paying the full ceremony of
`/vector:raw` → `/vector:propose` → `/vector:apply` and creating an OpenSpec change. That is
disproportionate for a mechanical one-line edit, and it discourages leaving a board trail for trivial
work. The user relies on the global `/quick-win` skill, but it is non-distributable and writes a
parallel `quick-wins.md`/CHANGELOG log outside Vector's single source of truth.

Vector needs the same capability, **agnostic to the user's code** and **distributable** inside the
binary (seeded by `vector init`), so any Vector-managed repo gets it. Being inside Vector, the small
change becomes a board card that is **applied in the same run** and leaves a record: the card is born
`in-progress` **marked as a quick-win**, the change is applied and validated with the repo's gate, the
work is logged (`work.logged`, visible in the standup), and the card lands in `review` for the user to
close with `/vector:close`. The commit is optional and asked on every run.

`/vector:quick` is the Vector-native equivalent of `/quick-win`. It deliberately does **not** create an
OpenSpec change and does **not** invoke the Sonnet validator — the "validation" is the repo's
lint/typecheck gate.

## What changes

- **Project command `/vector:quick`** (`kit/commands/vector/quick.md`): parse the change description +
  optional `{ticket|spec-id}` token, sanity-check that this really is a quick-win (vs `/vector:raw`,
  `/vector:bug`), delegate refinement to a **Haiku** agent, scope-guard, register the card
  `in-progress` marked quick-win via the binary, **apply the change directly in the same run**,
  validate with the repo's gate, log the work (`worklog`), commit optionally (asking), and transition
  to `review`. Escalate (recommend `/vector:raw`/`/vector:bug`) instead of expanding silently.
- **Refiner agent `vector-quick-refiner`** (`kit/agents/vector-quick-refiner.md`), tier **Haiku**,
  read-only: turns the raw description into a **light brief** (title / slug / change-type /
  what-changes / why / files-to-touch / acceptance / risks / blocking-questions / notes),
  agnosticized from the global `quick-win-refiner`. Surfaces ambiguity only when it would change the
  diff.
- **New state field `quickWin` (bool)** on `SpecState` (`cli/internal/state/types.go`): persists that
  the card is a quick-win. Seeded at create (`vector spec create … --quick-win`), exposed on the board
  projection/API, and shown as a read-only badge on the web card. Backward-compatible (`omitempty`).
- **Apply-in-run lifecycle**: the card is created directly in `in-progress` (status seed), the change
  is applied, `worklog` is recorded (record for the daily/standup), and the card transitions
  `in-progress → review`. Closing stays an explicit user step (`/vector:close`).
- **Optional ticket/spec link** (`/vector:quick [text] {ticket|spec-id}`): reuses `/vector:raw`'s
  ticket detection (`detectTicket`) to seed `--ticket`, or resolves the arg as an existing Vector spec
  id and records a `--related` relation. Only when it resolves with confidence; ambiguous → ask or
  omit. **Never blocks card creation on the link.**
- **Optional asked commit**: after implementation + validation, **ask** with `AskUserQuestion` whether
  to commit. Yes → atomic Conventional Commit in English, staging only the touched files. No → leave
  the working tree and report it. Never `--no-verify`/`--amend`/`git add -A`.
- **Vendoring**: command + agent embedded into the binary via `go generate` (`cli/internal/scaffold`),
  so `vector init`/`update` seed them.

## Capabilities

### New Capabilities

- `quick-win-authoring`: turn a small low-risk change description into a light brief (Haiku refiner,
  no Sonnet validator) and register it as an `in-progress` card marked quick-win, agnostic to the
  user's repo.
- `apply-in-run`: apply the change in the same run strictly to the brief, validate with the repo's
  gate, log the work (`work.logged`), optionally commit (asking), and transition `in-progress →
  review` — escalating to `/vector:raw` instead of expanding when the change grows.

### Modified Capabilities

- `state-model`: `SpecState` gains the optional `quickWin` bool; the board `Card` projection and
  `/api/board` gain `quickWin` as a read-only field.
- `spec-cli`: `vector spec create` gains `--quick-win`.

## Out of scope

- Creating an OpenSpec change (quick applies directly; grows → escalate to `/vector:raw`).
- The Sonnet `vector-spec-validator` (the gate is lint/typecheck, not the 20-section validator).
- A parallel docs log (`quick-wins.md`/CHANGELOG) in the user's repo — the board + `activity.jsonl`
  is the record.
- Editing `quickWin` from the web (display-only badge).
- Auto-closing the card (ends in `review`; closing is `/vector:close`).
- A unified card-type enum (`feature|bug|quick-win`) — V1 uses a bool; the enum is an open question.
