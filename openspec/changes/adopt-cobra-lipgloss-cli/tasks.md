# Tasks — adopt-cobra-lipgloss-cli

## 0. Golden capture (BEFORE touching dispatch)

- [x] 0.1 Add `cli/cmd/vector/testdata/golden/*.json` — one file per `--json` command (spec §7):
      `init`, `update`, `sync`, `context` (+ `--for`), `standup`, `spec create|list|propose|apply|
      link|relate|status|close|archive|next|worklog|summarize|summarize commit|route|attach-sketch`,
      `detect-ticket`.
- [x] 0.2 Capture each snapshot's byte-exact stdout against the **current** dispatch (not
      reconstructed post-hoc — that voids the gate).
- [x] 0.3 Add `cli/cmd/vector/golden_test.go` (`TestJSONGoldenUnchanged`): run each command, compare
      byte-for-byte against its golden file.

## 1. Dependencies

- [x] 1.1 `go -C cli get github.com/spf13/cobra@v1.10.2` and
      `go -C cli get github.com/charmbracelet/lipgloss@v1.1.0`; run `go -C cli mod tidy`.
- [x] 1.2 Confirm the resolved transitive tree against generated `cli/go.sum` (do not hand-edit).
      Verify **no** `huh`/`bubbletea`/`bubbles` pulled in.

## 2. internal/ui package

- [x] 2.1 `cli/internal/ui/ui.go`: `Bold/Green/Red/Dim/Cyan` (lipgloss style wrappers);
      `Success` (`✓`), `Info` (`●`), `Warning` (`⚠`), `Error` (`✗`); `Table(headers, rows)` via
      `charmbracelet/lipgloss/table`; `KeyValue(label, value)`. Palette = flagify hex tokens as a
      documented placeholder (Open question #1). No `AddFormatFlag`/`IsJSON`/`PrintJSON`; no
      huh/bubbletea import.
- [x] 2.2 `cli/internal/ui/help.go`: `ApplyCustomHelp(cmd)` → `cmd.SetHelpFunc(customHelp)`; styled
      USAGE/COMMANDS/FLAGS sections named `vector`, written to `cmd.OutOrStderr()` (help → stderr).
- [x] 2.3 `cli/internal/ui/ui_test.go`: each helper returns non-empty text containing its input;
      `Table` includes the headers; plain-text degradation off-TTY.

## 3. Root command + exit codes

- [x] 3.1 `cli/cmd/vector/root.go`: `newRootCmd()` builds a fresh tree per call;
      `Use: "vector"`, `Short/Long` from `usage()` line 1099; `SilenceErrors`/`SilenceUsage: true`;
      `AddCommand` every `newXxxCmd()`; `ui.ApplyCustomHelp(root)`.
- [x] 3.2 Hand-rolled version: disable cobra's auto `--version`; persistent bool `-v`/`--version`
      in root `PersistentPreRunE` + explicit `version` subcommand, both print
      `fmt.Println("vector", version)` (stdout, exit 0) in any position. Preserve `var version`
      and the `-X main.version=...` ldflag.
- [x] 3.3 Exit-code mapping in `main()`/`Execute()` per the table in design.md: `nil`→0;
      `vector` no-args / unknown command → 2; `spec` no-subverb / unknown subverb / unknown flag /
      business error → 1.
- [x] 3.4 `cli/cmd/vector/completion.go`: `vector completion bash|zsh|fish|powershell`, `Args`
      validator (exactly one of the four), `RunE` → `rootCmd.Gen{Bash,Zsh,Fish}Completion` /
      `GenPowerShellCompletionWithDesc`. Nothing embedded.

## 4. Migrate command files (runXxx → newXxxCmd)

- [x] 4.1 `main.go`: reduce `main()` to build `rootCmd` + `Execute()` + exit code; delete
      `switch os.Args[1]` (line 38) and `func usage()` (1098-1126);
      `runInit/Update/Sync/SpecCreate/SpecList/DetectTicket/Spec` → their `newXxxCmd`.
- [x] 4.2 `context.go`: `runContext` → `newContextCmd`; preserve `--json` default **true** and `--for`.
- [x] 4.3 `serve.go`: `runServe` → `newServeCmd`; replace `fs.Visit` port-detection (41-46) with
      `cmd.Flags().Changed("port")`, same free-port fallback behavior.
- [x] 4.4 `standup.go`: `runStandup`/`runStandupCommit` → `newStandupCmd` + child `commit`.
- [x] 4.5 `route.go`, `sketch.go`: `runSpecRoute` → `newSpecRouteCmd`; `runSpecAttachSketch` →
      `newSpecAttachSketchCmd`.
- [x] 4.6 `summarize.go`: `runSpecSummarize(+Commit)` → `newSpecSummarizeCmd` with **manual
      two-order detection** for `summarize commit <id>` and `summarize <id> commit` (documented
      exception; do not delegate to a pure cobra child).
- [x] 4.7 `spec_transitions.go`: `apply/link/relate/status/close/archive/next/fix` → their
      `newSpecXxxCmd`; reimplement `leadingID` + second-positional peel against cobra's post-parse
      `args []string`, same semantics.
- [x] 4.8 Preserve every flag 1:1 (name, type, default, help). No global/persistent flags. No new
      short aliases. `--repo-root`/`--json` stay local per subcommand.

## 5. Test harness migration

- [x] 5.1 `cli/cmd/vector/testutil_test.go`: shared `execCmd(t, factory func() *cobra.Command,
      args ...string) (stdout, stderr string, err error)`.
- [x] 5.2 Migrate the ~11 `*_test.go` calling `runXxx(args)` to `execXxxCmd`/harness: `main_test`,
      `context_test`, `standup_test`, `sync_test`, `init_language_test`, `related_test`,
      `ticket_test`, `summarize_test`, `spec_fix_test`, `spec_transitions_test`, `sketch_test`
      (pure-function tests like `parseFixArtifacts`/`inferProvider` unchanged). Same coverage.
- [x] 5.3 New dispatch tests (`main_test.go`): `vector` no-args → exit 2; unknown command → exit 2
      (stderr); `spec` no-subverb → exit 1; `spec bogus` → exit 1; `spec create --bogus` → exit 1.
- [x] 5.4 `context --json` default true; `serve --port` explicit-vs-fallback via `Changed("port")`;
      `summarize commit` both orders; version/ldflag build test (`-X main.version=v9.9.9-test`).

## 6. Docs

- [x] 6.1 `README.md`: mention `vector completion <shell>` and `vector --help` as the terminal
      surface; supported-platforms/commands notes.
- [x] 6.2 `docs/plugin-and-commands.md`: the "Binario Go" row now exposes cobra-generated
      `--help`/`completion`.
- [x] 6.3 `.claude/rules/architecture/distribution-packaging.md`: binary is no longer 100% stdlib
      (first external deps: cobra + lipgloss); completions on-the-fly; measured-weight note.
- [x] 6.4 `cli/CLAUDE.md`: update the "stdlib, sin deps externas" line; document `internal/ui`.

## 7. Gate + weight

- [x] 7.1 `gofmt -l cli` empty; `go -C cli vet ./...` clean; `go -C cli test ./...` green;
      `go -C cli build ./...` succeeds; `go -C cli mod tidy` leaves `go.sum` with no diff.
- [x] 7.2 All golden `--json` tests pass (byte-identical) after migration.
- [x] 7.3 Measure release-equivalent binary (`CGO_ENABLED=0 go -C cli build -ldflags "-s -w"`)
      before vs after; report bytes + % delta. **No threshold** — report only.
- [x] 7.4 Verify (script or inspection) no `ui.*` call sits inside any `if jsonOut`/`if *jsonOut`
      branch; repo not left in a hybrid flag/cobra state.
