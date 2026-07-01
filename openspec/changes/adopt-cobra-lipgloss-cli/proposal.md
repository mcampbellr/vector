# Adopt cobra + lipgloss in the Vector CLI

## Why

The `vector` binary routes its subcommands by hand: `cli/cmd/vector/main.go` (~1127 lines)
switches on `os.Args[1]` (line 38), each subcommand builds its own `flag.NewFlagSet`, and the
help text is plain hardcoded output in `func usage()` (lines 1098-1126). This has three costs:

- **No shell completions** and **no styled output** — the terminal surface is bare text.
- **Hand-rolled parsing edge cases**: the stdlib `flag.FlagSet` stops parsing at the first
  non-flag argument, so the code carries manual workarounds (`leadingID` in
  `spec_transitions.go:17-22`, the two-order handling of `spec summarize <id> commit` /
  `summarize commit <id>`, a second-positional peel in `link`/`status`).
- **Help drifts** from the actual command tree because it is maintained separately in `usage()`.

This change replaces the manual dispatch with a **cobra** (`github.com/spf13/cobra`) command
tree and adds a new **lipgloss** (`github.com/charmbracelet/lipgloss`) package
`cli/internal/ui` that styles the **human** output branch only. It gives the terminal user
auto-generated `--help`, `vector completion <shell>`, and clear colored/structured output —
**without touching the `--json` contract** consumed by the `/vector:*` project commands. That
stdout must stay **byte-identical** before and after the change.

These are the **first external dependencies** of the Go module (`cli/go.mod` has no `require`
block today — pure stdlib): cobra (Apache-2.0) and lipgloss (MIT), both license-compatible with
the repo (Apache-2.0) and with commercial distribution.

## What changes

- **`cli/cmd/vector/` (MODIFY, the rewritten layer)** — each `runXxx` that owns a
  `flag.FlagSet` gains a `newXxxCmd() *cobra.Command` factory that registers the **same flags**
  (name, type, default, help — 1:1) via `cmd.Flags().*Var(...)` and moves the business body
  into `RunE`. The body between the old `fs.Parse` and `return` is unchanged except `*x` → `x`.
  Covers every subcommand documented in `usage()`: `init`, `update`, `context` (incl. `--for`),
  `sync`, `serve`, `standup` (+ `commit`), `spec create|list|propose|apply|fix|link|relate|
  status|close|archive|next|worklog|summarize (+ commit)|route|attach-sketch`, `detect-ticket`,
  `version`/`--version`/`-v`, `help`/`-h`/`--help`.
- **`cli/cmd/vector/root.go` (NEW)** — `newRootCmd()` builds a fresh tree per call (testable in
  isolation), maps exit codes 0/1/2 to preserve today's behavior (§11 of the spec), wires
  `-v`/`--version`/`version` to print `vector <version>` (cobra's auto `--version` is disabled),
  and applies `ui.ApplyCustomHelp`.
- **`cli/cmd/vector/completion.go` (NEW)** — `vector completion bash|zsh|fish|powershell`,
  generated on-the-fly via cobra's `Gen*Completion*` (nothing embedded).
- **`cli/cmd/vector/main.go` (MODIFY)** — reduced to building `rootCmd` + `Execute()` + exit-code
  mapping; `switch os.Args[1]` and `func usage()` deleted.
- **`cli/internal/ui/` (NEW)** — `ui.go` (`Bold/Green/Red/Dim/Cyan`, `Success/Info/Warning/Error`,
  `Table`, `KeyValue`) and `help.go` (`ApplyCustomHelp` + styled help render), modeled 1:1 on
  the external reference `~/Developer/Personal/flagify/cli/internal/ui/*`. Applied **only** in
  the human output branch, never inside `if jsonOut { ... }`.
- **Golden `--json` snapshot suite (NEW)** — `testdata/golden/*.json` + `golden_test.go` capture
  the byte-exact stdout of each `--json` command **before** touching dispatch and compare
  **after**. This is the hard gate.
- **Test harness migration** — the ~11 `*_test.go` files that call `runXxx(args)` directly move
  to a shared `execXxxCmd` helper (`testutil_test.go`) that drives the cobra factory.
- **`cli/go.mod` / `cli/go.sum` (MODIFY)** — add cobra v1.10.2 + lipgloss v1.1.0 and transitives
  via `go mod tidy`.
- **Docs (MODIFY)** — `README.md`, `docs/plugin-and-commands.md`,
  `.claude/rules/architecture/distribution-packaging.md`, `cli/CLAUDE.md`.
- **Binary weight measurement** — release-equivalent build (`CGO_ENABLED=0 -ldflags "-s -w"`)
  measured before vs after and reported as an acceptance criterion; **no hard threshold** (the
  user decides with the number in hand).

## Scope

**Out of scope:** `huh`/`bubbletea` (the CLI is non-interactive — no prompts in `cli/`); any
coexistence phase (atomic change, no hybrid state on `main`); any change to the `--json` shape,
field order, type, or indentation; new short-form flag aliases; touching domain packages
(`internal/state`, `internal/config`, `internal/board`, `internal/webui`, etc.); the board HTTP
endpoints; `.goreleaser.yml`; Windows as a supported platform (the powershell completion string
is generated but not CI-tested end-to-end); re-styling `vector context` / `vector detect-ticket`
(Open question #3).

The agent must not implement anything outside this scope, even if it seems related. Full detail
lives in the source spec: `.vector/specs/adopt-cobra-lipgloss-cli/spec.md`.
