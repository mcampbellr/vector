# Tasks — add-vector-quick-command

## 1. State model (`cli/internal/state`)

- [x] 1.1 `types.go`: add `QuickWin bool \`json:"quickWin,omitempty"\`` to `SpecState` (next to
      `NeedsUAT`). Backward-compatible (`omitempty`); do not touch the state machine.
- [x] 1.2 `store.go`: add `QuickWin bool` to `CreateSpecParams`; `CreateSpec` persists it on the
      initial `SpecState` (mirror `NeedsUAT`/`Ticket` handling in create).

## 2. CLI (`cli/cmd/vector`)

- [x] 2.1 `main.go`: `--quick-win` (bool, default false) flag on `spec create` → wired to
      `CreateSpecParams.QuickWin`.
- [x] 2.2 `main.go`: update the `spec create` usage to document `--quick-win`.

## 3. Board projection + web (read-only)

- [x] 3.1 `cli/internal/board/board.go`: expose `QuickWin bool` (display subset) on the `Card`
      projection (mirror `NeedsUAT`); `/api/board` carries it. No write endpoint.
- [x] 3.2 `web/src/types/board.ts`: add `quickWin?: boolean` to the board spec type.
- [x] 3.3 `web/src/components/SpecCard/SpecCard.tsx` (+ `.module.css`): render a read-only "Quick Win"
      badge when `quickWin`, with `aria-label`; no editing. Mirror the existing pill/chip pattern.

## 4. Refiner agent (Haiku)

- [x] 4.1 `kit/agents/vector-quick-refiner.md`: frontmatter tier **Haiku**, read-only (Read, Grep,
      Glob); pattern `kit/agents/vector-spec-refiner.md` + the global `quick-win-refiner`.
- [x] 4.2 Input `RAW_QW`; output the light brief (Optimized Title / Kebab-case Slug / Change Type /
      What Changes / Why / Files to Touch / Acceptance / Risks / Blocking Clarifying Questions /
      Non-Blocking Notes). Preserve language; terse; surface ambiguity only if it changes the diff.

## 5. Project command (`kit/commands/vector/quick.md`)

- [x] 5.1 Frontmatter: `name: quick`, `description`, `argument-hint`, `user-invocable: true`,
      `allowed-tools` (Read, Write, Edit, Grep, Glob, `Bash(git *)`, `Bash(vector *)` + stack gates,
      Agent, AskUserQuestion).
- [x] 5.2 Steps per design §Flow: parse → confirm init → sanity-check → refine (Haiku) + scope-guard →
      clarify → resolve link → create card `in-progress`+quick-win → implement → validate → worklog →
      commit (asking) → transition to review → route → summary → report.
- [x] 5.3 Document token routing (refiner=Haiku, no validator, orchestration/implementation=main loop)
      and state discipline (card/`quickWin`/ticket/worklog/status written only via the binary; never
      edit `.vector/` by hand).

## 6. Vendoring / scaffold (`cli/internal/scaffold`)

- [x] 6.1 `go generate ./internal/scaffold` copies `quick.md` + `vector-quick-refiner.md` into
      `assets/` (`//go:generate` + `//go:embed all:assets`).
- [x] 6.2 `scaffold_test.go`: include `quick.md` + the agent in the expected embedded set if it
      enumerates one; `TestAssetsMatchKit` stays green.

## 7. Tests

- [x] 7.1 `cli/internal/state/store_test.go`: create with `QuickWin`, JSON round-trip, backward
      compatibility (spec without `quickWin`).
- [x] 7.2 `cli/internal/board/board_test.go`: the `Card` projection includes `quickWin`.
- [x] 7.3 Web: render the "Quick Win" badge in `SpecCard` when `quickWin` (behavioral component test,
      not an empty snapshot).

## 8. Docs

- [x] 8.1 Update `docs/plugin-and-commands.md` (the `/vector:quick` command), `docs/schemas/
      state-and-activity.md` (the `quickWin` field), and `docs/domain-contract.md` (apply-in-run
      lifecycle + optional link).

## 9. Verification gate

- [x] 9.1 `go -C cli generate ./internal/scaffold` → `go -C cli vet ./...` → `go -C cli test ./...` all
      green.
- [x] 9.2 `npm --prefix web run typecheck && npm --prefix web run lint && npm --prefix web run build`
      all green.
- [x] 9.3 `vector init` in a clean repo seeds `quick.md` + `vector-quick-refiner.md`.
