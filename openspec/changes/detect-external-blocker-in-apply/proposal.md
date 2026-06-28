# Detect external-dependency blocker in /vector:apply

## Why

When `/vector:apply` finishes, it transitions the card to `review` unconditionally. But an
implementation can compile and pass its (mocked) suite while still being gated by something the
agent cannot resolve itself — third-party credentials, unconfirmed external `api_names`, data
owed by another team. Today that card lands in `review` pretending to be ready, hiding a
human-action blocker.

Motivating case (user repo, not Vector): `MH-1582` "prospect applications endpoint" — code
complete, PR #367 open, build/lint/test green, but the exact Zoho CRM `api_names` were left as
`TODO(MH-1582)` pending settings-read credentials; the run produced a comment asking for them.
The card went to `review` and had to be moved to `needs-attention` by hand. This change
automates that correction at apply's close.

## What changes

- **New detection sub-step in `apply.md` §6 (Finish), before the transition**: the agent
  inspects its own run artifacts (working-tree diff, `tasks.md`/acceptance items, any outbound
  artifacts the run produced) and judges whether an external-dependency blocker exists. Any of
  three signals suffices:
  1. A `TODO(<ticket>)`/placeholder in production code that governs runtime behavior and depends
     on an external datum/credential/identifier not yet provided.
  2. An outbound artifact whose purpose is to ask a human/another team for something.
  3. A `tasks.md`/acceptance item satisfiable only against mocks and explicitly marked pending a
     real datum/credential.
- **Deterministic false-positive guard**: `TODO`/`FIXME` in test-only files
  (`*_test.go`, `*.test.*`, `test`/`tests`/`__tests__` dirs) and cosmetic comments
  (refactor/naming/typo) never trigger. A `TODO` deliberately pointing at another tracked
  ticket is an intentional deferral (agent judgment, not a `.vector/specs/` lookup).
- **Automatic routing, independent of `applyMode`**: on a blocker the agent runs
  `vector spec status <id> needs-attention --reason "<reason>"` instead of `... review`. No
  confirmation even when `applyMode` is `ask`/`always-ask` — it is a board-integrity safeguard,
  not a workflow choice.
- **Concrete, actionable reason**: names *what is pending* + *how/who unblocks it* + *open PR
  ref* when present.
- **§7 report updated**: when routed to `needs-attention`, the report surfaces the blocker + reason
  instead of "ready for review".
- **Heuristic documented inside `apply.md`** (auditable), and the embedded copy
  `cli/internal/scaffold/assets/commands/vector/apply.md` regenerated via `go generate`.

## Scope

- In: detection sub-step + conditional transition in `apply.md` §6, blocker surfacing in §7,
  the documented heuristic, and the regenerated embedded asset.
- Out: auto-resolving the dependency (no fetching credentials, no third-party API calls), any Go
  change (`runSpecStatus`/`SetStatus`/`Attention` already exist), state-machine changes,
  retroactive rescans, multiple transitions per run, and the §4 ambiguity hard-stop (untouched).

Authored spec: `.vector/specs/detect-external-blocker-in-apply/spec.md`.
