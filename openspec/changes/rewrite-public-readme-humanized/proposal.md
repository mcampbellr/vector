# Rewrite the public README in English, humanized and well explained

## Why

The root `README.md` (~37 lines, Spanish) still carries the notice "captura inicial de la idea /
**Nada implementado todavía**" on its second line. That is no longer true: the Go CLI has working
subcommands (`vector init`, `vector serve`, `vector standup`), the kit ships 11 executable
`/vector:*` project commands, `web/` is under active development, and there are compiled specs in
`.vector/`. A developer landing on the repo today is misinformed about the real state and gets no
usage guidance.

## What changes

- **Near-complete rewrite of `README.md`** (root only): new structure, English prose, run through
  the `/humanizer` skill to strip AI-writing signals. No other file in the repo is touched.
- **Nine sections** in order: `What is Vector`, `Why Vector`, `Installation`, `Quickstart`,
  `Key Concepts`, `Commands Reference`, `Walkthrough — End-to-End Flow`, `Contributing / License`,
  `Further Reading`.
- **Real install steps of today**: `git clone <url>` + `go build -o ~/.local/bin/vector ./cmd/vector`
  from `cli/`, requiring Go 1.26+ (from `cli/go.mod`), plus `vector init` per repo. The
  `curl | install.sh` one-liner is shown only as "coming soon / planned", never as a working step.
- **Commands table** with descriptions verified against `kit/commands/vector/*.md` (11 commands
  confirmed at write time; the implementer re-verifies the exact count).
- Board image `docs/assets/kanban-reference.png` with descriptive alt text; verified links to
  `docs/vision.md`, `docs/domain-contract.md`, `docs/plugin-and-commands.md`,
  `docs/commercialization.md`. License section marked explicitly **TBD** (no `LICENSE` invented).

## Scope

- In: editorial rewrite of the single root `README.md`, verified content, mandatory `/humanizer`
  pass, the two chosen optional sections (Walkthrough, Contributing/License).
- Out: the `curl | install.sh` installer (separate `/vector:raw` follow-up), status roadmap,
  tool comparisons, any file other than `README.md`, duplicating `docs/vision.md`, a changelog.

Authored spec: `.vector/specs/rewrite-public-readme-humanized/spec.md`.
