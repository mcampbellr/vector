# Design — adopt-cobra-lipgloss-cli

## Key decisions

- **Cobra as a thin routing/adapter layer; business logic does not move.** Each subcommand
  keeps a `runXxx`-equivalent body (calls to `state.Open`, `store.*`, `config.*`,
  `printJSON(Value)`); only *how it receives its flag values* changes: from
  `fs := flag.NewFlagSet(...)` + `*x` to a `newXxxCmd()` factory registering
  `cmd.Flags().StringVar(&x, ...)` + `RunE`. The body between parse and `return` changes only
  `*x` → `x`. (Spec §5.)

- **Domain packages are untouched.** `internal/state`, `internal/config`, `internal/board`,
  `internal/openspec`, `internal/scaffold`, `internal/standup`, `internal/webui`,
  `internal/intel` — zero changes; none imports cobra or lipgloss. Cobra/lipgloss are a
  routing/presentation layer over `cmd/vector/`.

- **`RunE` writes with `fmt.Print*` to `os.Stdout`/`os.Stderr` directly, not
  `cmd.OutOrStdout()`.** This keeps the `--json` stdout independent of cobra wiring, so the
  byte-identical gate is easy to reason about. Only the **help** (`cmd.Help()`) and cobra's
  **error messages** (unknown flag/command) use `cmd.OutOrStderr()`. (Spec §5.)

- **`newRootCmd()` builds a fresh tree per call** (not a package-level singleton), so tests can
  execute commands in isolation without shared state — a deliberate divergence from the flagify
  reference (`var rootCmd`), which never tests the whole tree.

- **`--json` contract is byte-identical, no exception.** No JSON output gains or loses a field.
  Two-space indent (`json.MarshalIndent(v, "", "  ")`) and trailing newline (`fmt.Println`) are
  preserved because `printJSON`/`printJSONValue` (`main.go:989-1002`) are not modified. A golden
  snapshot suite captures each `--json` command's stdout **before** the dispatch change and
  compares **after** — captured pre-change, not reconstructed post-hoc (that would void the gate).

- **`internal/ui` applies only in the human branch.** lipgloss auto-degrades to plain text under
  `NO_COLOR`, `TERM=dumb`, or non-TTY stdout — defense in depth *on top of* the hard rule that
  `ui.*` is never called inside an `if jsonOut` branch, not a substitute for it.

- **Version handling is hand-rolled.** Cobra's automatic `--version` prints `<name> version
  <Version>` — a different format from today's `vector <version>`. It is disabled (no
  `rootCmd.Version`); a persistent bool `-v`/`--version` in the root `PersistentPreRunE` and an
  explicit `version` subcommand both print `fmt.Println("vector", version)` (stdout, exit 0) in
  any invocation position. `var version = "dev"` and the `-X main.version=...` ldflag
  (`.goreleaser.yml:29`) are preserved literally.

- **`--repo-root`/`--json` stay local per-subcommand flags, not persistent on the root** — so
  the `--help` of commands that don't accept `--repo-root` today (e.g. `version`) is unchanged.

- **Help goes to stderr** (parity with today's `usage()`, which uses `fmt.Fprint(os.Stderr,...)`),
  not stdout — scripts that redirect `2>/dev/null` and expect clean stdout see no change.

- **Completions generated on-the-fly, not embedded** — `architecture/distribution-packaging.md`
  imposes no embed requirement for completions.

- **Only long-form flags in this phase** — no new short aliases, to avoid widening the flag
  surface beyond the 1:1 migration.

## Exit-code mapping (spec §11)

`main()`/`Execute()` inspects the error from `rootCmd.Execute()` with `SilenceErrors: true`,
`SilenceUsage: true` (cobra's auto print/exit is bypassed):

| Invocation | Today | Required after |
|---|---|---|
| valid, `nil` error | exit 0 | exit 0 |
| `vector` (no args) | `usage()` + exit 2 | exit 2 (override cobra's default help+exit 0) |
| `vector foo` (unknown command) | stderr + exit 2 | **exit 2 forced** (cobra defaults to exit 1); message may adopt cobra's format but stays on stderr and names the bad command |
| `vector spec` (no subverb) | `error: usage: vector spec <…>` + exit 1 | exit 1 (parent `specCmd` keeps an explicit `RunE` returning the same error; not help+exit 0) |
| `vector spec bogus` (unknown subverb) | exit 1 | exit 1 (verify cobra's default matches; text may differ) |
| unknown flag on a leaf (`spec create --bogus`) | parse error, exit 1 | exit 1 (verify pflag matches; do not assume) |
| business error (spec missing) | `error: <msg>` + exit 1 | exit 1 (styled with `ui.Error`) |

## Parsing behavior change (superset, not regression)

`pflag` (cobra's engine) **interleaves** flags and positionals natively, unlike stdlib `flag`.
Invocations that require a strict order today (e.g. `vector spec apply --json my-spec`, flag
before id) **start working** — a superset of the accepted language. Every valid invocation today
stays valid and produces the same output; no case is retired. `leadingID` and the second-
positional peel are reimplemented against the `args []string` cobra hands `RunE` (the leftover
positionals after flag parsing), with the same "first/second non-flag token = id/ref/target"
semantics.

## `spec summarize commit` — deliberate exception

Two orders are supported today: `summarize commit <id>` and `summarize <id> commit`
(`summarize.go:54-62`; the kit commands use the latter). A pure cobra `AddCommand("commit")`
only understands the first. **Both must be preserved**: the `summarize` `RunE` keeps manual
two-order detection over its `args`, documented in-code as a deliberate exception to the
"cobra child command" pattern used everywhere else in the tree.

## File layout (spec §5)

```txt
cli/
  cmd/vector/
    root.go            # NEW — rootCmd, exit-code mapping, -v/--version/help wiring
    completion.go      # NEW — vector completion <shell>
    main.go            # MODIFY — reduced to build rootCmd + map exit code
    context.go serve.go standup.go route.go sketch.go summarize.go spec_transitions.go  # MODIFY — runXxx → newXxxCmd
    testutil_test.go   # NEW — shared execCmd(t, factory, args...) harness
    golden_test.go     # NEW — TestJSONGoldenUnchanged
    testdata/golden/   # NEW — byte-exact --json snapshots
  internal/ui/
    ui.go help.go ui_test.go   # NEW — presentation package (terminal analog of internal/webui)
```

No new domain folders. `internal/ui` is a presentation package (role analogous to
`internal/webui` but for the terminal).

## References

- External pattern (not imported, replicated): `~/Developer/Personal/flagify/cli/cmd/root.go`,
  `cmd/completion.go`, `internal/ui/ui.go`, `internal/ui/help.go`.
- Dependency versions: cobra **v1.10.2**, lipgloss **v1.1.0** (confirm the exact transitive tree
  against generated `cli/go.sum`, not against a hand-listed set).

## Open questions (carried, not decided here)

1. **lipgloss color palette** — reuse flagify's hex tokens (`#00D4FF`/`#00CC88`/`#FF6B6B`/
   `#FFCC00`/`#666666`) as a documented placeholder, or define a Vector-own palette. No brand
   decision recorded in the repo.
2. **Short-form flag aliases** — default is long-form only; reopen in a future phase.
3. **`vector context` / `vector detect-ticket` styling** — machine-first consumers; out of scope
   for re-styling this phase.
