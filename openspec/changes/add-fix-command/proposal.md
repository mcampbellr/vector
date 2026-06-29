# Add /vector:fix command

## Why

Correcting work that is **already specified on the board** — something a spec missed, a UAT
finding, a small course-correction — required leaving Vector's flow and using the personal
`/fix` skill, whose domain writes and agents live outside Vector. We need a Vector-native fix
that reuses the refiner → clarity-gate → implementer discipline but is wired into the state
machine, the token meter, and the activity trace, with **CLI-owns-writes** and **its own
embedded agents** (no dependency on the user's `~/.dotfiles`).

## What changes

- New `/vector:fix <id>` project command + new `vector spec fix <id>` binary subcommand.
- `vector spec fix` **only records** a typed `spec.fixed` event (classification + files +
  validation result). It **never transitions status** — lifecycle moves go through the existing
  `vector spec status` (the LOCKED machine), keeping the binary the single writer.
- Two embedded agents: `vector-fix-refiner` (Haiku, read-only) and `vector-fix-implementer`
  (Sonnet), mirroring the `vector-spec-refiner`/`vector-spec-validator` pattern, vendored to
  `scaffold/assets/` and seeded by `init`/`update`.
- Fix lifecycle via `vector spec status`: a spec in `review` moves to `in-progress` (clear fix)
  or `needs-attention` (blocked, with `reason`) and returns to `review`; a spec in `open` enters
  `in-progress` and exits to `review`; a spec already in work is fixed in place.
- Refiner classifies the correction `spec-only` / `code-only` / `spec+code`; clarity gate
  (`CLEAR` → run; `NEEDS_CLARIFICATION` → ask via `AskUserQuestion`); scope guard routes fresh
  features/bugs to `/vector:raw`/`/idea`/`/bug` and stops without writing.
- Token routing (`vector spec route`) and work trace (`vector spec worklog`) recorded per run.
  **No auto-commit** — the working tree is left for the dev to review.

## Scope

- In: the command, the `vector spec fix` state event (`spec.fixed` + `FixedData`), the two
  embedded agents, classification + clarity gate + scope guard, lifecycle via `vector spec
  status`, token/work tracing, table-driven tests + vendoring test extension.
- Out: card-less fixes, auto-commit, fresh features/bug investigation, fixing
  `draft`/`closed`/`archived` specs, status transitions inside `vector spec fix`, `SpecState`
  schema changes, new web UI, branch/worktree management.

Authored spec: `.vector/specs/add-fix-command/spec.md`.
