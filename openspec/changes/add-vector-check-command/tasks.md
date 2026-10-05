# Tasks — add-vector-check-command

## 1. Shared gitexec package (reuse, not duplication)

- [ ] 1.1 `cli/internal/gitexec/gitexec.go`: `func Output(repoRoot string, args ...string) ([]byte, error)` extracted verbatim from `gitOutput` in `fingerprint.go`, keeping the "git missing / not-a-repo → wrapped error, caller degrades" pattern.
- [ ] 1.2 `cli/internal/gitexec/gitexec_test.go`: `Output` against a temp `git init` repo + the "not-a-repo" case.
- [ ] 1.3 `cli/internal/intel/fingerprint.go`: remove local `gitOutput` (lines 261-270), migrate its 3 call sites (238, 251, 254) to `gitexec.Output`; keep `cli/internal/intel` tests green (null-behavior refactor).

## 2. check package (local sweep)

- [ ] 2.1 `cli/internal/check/check.go`: `Severity` (`high|medium|low`); `DiscrepancyType` (`invalid-state`, `no-ticket`, `malformed-ticket-ref`, `missing-branch`, `unmerged-vs-base` — the five local types only; no Jira types); `Discrepancy{SpecID,Type,Severity,Description,Suggestion,FixEligible}`; `Note{SpecID,Description}`; `SpecCheckable{ID,Title,Status,Ticket *state.Ticket}` (`ticket,omitempty`); `CheckReport{GeneratedAt,Specs,Discrepancies,Notes,Language}`.
- [ ] 2.2 `func Run(store *state.Store, cfg *config.Config, root string) (CheckReport, error)`: iterate `store.ListSpecs()`, filter `state.StatusArchived`, apply the three local checks (state via `state.Status.Valid()`, ticket presence/shape, git via `git.go`), accumulate `Discrepancies`/`Notes`/`Specs`; resolve `Language = cfg.ResolvedLanguage()`.
- [ ] 2.3 `cli/internal/check/git.go`: read-only `branchExists(root,branch)` (`git rev-parse --verify --quiet refs/heads/<branch>`), `worktreeExists(root,branch)` (parse `git worktree list --porcelain`), `unmergedCount(root,base,branch)` (`git rev-list <base>..<branch> --count`), all via `gitexec.Output`, degrading to "no signal" when git is absent / not a repo (never abort the sweep).
- [ ] 2.4 `cli/internal/check/check_test.go`: table-driven `Run` — clean case; each of the five local discrepancy types; `archived` spec never appears in `Specs`/`Discrepancies`.
- [ ] 2.5 `git.go` tests against a temp real git repo (`git init` + commits), including the "not a repo → degrades" case.

## 3. check subcommand (binary)

- [ ] 3.1 `cli/cmd/vector/check.go`: `newCheckCmd()` — `Use: "check"`, `Args: cobra.NoArgs`, flags `--json` (bool) + `--repo-root` (string); `RunE` uses `openStore(repoRoot)` + `config.Load(root)` (tolerant: `cfg == &config.Config{}` on failure), calls `check.Run`, prints `json.MarshalIndent` when `--json` else a human severity summary. No `.vector/` writes, no network/MCP.
- [ ] 3.2 `cli/cmd/vector/root.go`: register `newCheckCmd()` in the `newStandupCmd()` block.
- [ ] 3.3 `cli/cmd/vector/context.go`: add `"check": tierTrust` to `commandTiers`.
- [ ] 3.4 `cli/cmd/vector/check_test.go`: `--json` produces a valid `CheckReport`; runs degraded with no `.vector/config.json` (no `Language`, no `[branch]` git check); golden JSON shape if added to the existing golden suite.

## 4. Kit (command + agent)

- [ ] 4.1 `kit/commands/vector/check.md`: orchestration steps 2-7 (§5) — run `vector check --json`, cross-check Jira via MCP (skip on `--skip-jira`, degrade gracefully when MCP unavailable), invoke `vector-check-reporter`, print grouped-by-severity report, handle `--fix` per-item via `AskUserQuestion` → `vector spec close`. English; mirror `standup.md` structure + `link.md` Hard rules. No direct `.vector/` writes, no batch fix.
- [ ] 4.2 `kit/agents/vector-check-reporter.md`: Haiku (`tools: Read`), input = combined `CheckReport` JSON, output = `{summary, bySeverity:{high,medium,low}, notes}` (pure JSON); never invent a discrepancy; `command` = input `suggestion` verbatim; group only by the input `severity`; `id`/`ticket.key` verbatim; prose in the directive language else conversation language. Mirror `vector-standup-writer.md`.
- [ ] 4.3 `go -C cli generate ./...` to regenerate `cli/internal/scaffold/assets/{commands/vector/check.md,agents/vector-check-reporter.md}`; extend the embedded-assets test to cover the new command + agent.

## 5. Verification (gate)

- [ ] 5.1 `go -C cli generate ./...`, `gofmt -l cli` (empty), `go -C cli vet ./...`, `go -C cli test ./...`, `go -C cli build ./...` — all green.
- [ ] 5.2 Success criteria (§8): clean report `[]`; each local discrepancy type with exact severity; `no-ticket` Low/`fixEligible:false`; `missing-branch` / `unmerged-vs-base` under `[branch]`; dirty `in-progress` → Note; `--skip-jira` never contacts the MCP; `--fix` confirms per item and never applies `fixEligible:false`; `vector check` never writes `.vector/` (verified via `git status` before/after); report honors `Language`.
- [ ] 5.3 No regressions in `spec create|list|apply|propose|close|status|link|standup`; `internal/intel` tests green after the `gitexec` migration.
- [ ] 5.4 Record any unresolved Open question (design.md) explicitly in the spec instead of assuming it silently.
