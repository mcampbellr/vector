# Tasks — add-vector-bug-command

## 1. State model (`cli/internal/state`)

- [x] 1.1 `types.go`: add `RelatedItem{Kind RelatedKind; Ref string; Source RelatedSource}` with
      json tags `kind`/`ref`/`source`; enums `RelatedKind ∈ {spec,ticket}`,
      `RelatedSource ∈ {blame,manual}`; `RelatedTo []RelatedItem \`json:"relatedTo,omitempty"\`` on
      `SpecState`. Backward-compatible (mirror the `Ticket` struct at `types.go:113`).
- [x] 1.2 Event type `spec.related` alongside existing event types (`work.logged`, `agent.routed`).
- [x] 1.3 `store.go`: persist `RelatedTo` on create when present; `RelateSpec(id, item) error`
      serialized by the `Store` mutex, idempotent on `{kind,ref}`, appends `spec.related`. Atomic
      write; does not touch the state machine.

## 2. CLI (`cli/cmd/vector`)

- [x] 2.1 `ticket.go`: `parseRelatedFlag` (JSON array of `{kind,ref,source}`, `source` default
      `manual`) and `parseRelateFlags` (kind/ref/source), analogous to `parseTicketFlag`
      (`ticket.go:292`); enum + ref validation with actionable messages.
- [x] 2.2 `main.go`: `--related '<json>'` on `spec create` — invalid relation **degrades** (create
      card without relations + report), does not abort a valid card.
- [x] 2.3 `main.go`: `--json` flag on `spec list` (`runSpecList`, `main.go:776`) for robust cause
      resolution.
- [x] 2.4 `main.go`: new `spec relate <id> --kind spec|ticket --ref <ref> [--source blame|manual]
      [--json]` subcommand; spec-not-found → actionable error (no implicit card creation).
- [x] 2.5 Update usage (`main.go:559,869`).

## 3. Board projection + web (read-only)

- [x] 3.1 `cli/internal/board/board.go`: expose `RelatedTo` (display subset) on the `Card`
      projection (mirror `Ticket` at `board.go:40,62`); `/api/board` carries it. No write endpoint.
- [x] 3.2 `web/src/types/board.ts`: `RelatedItem` type + `relatedTo?: RelatedItem[]`.
- [x] 3.3 `web/src/components/SpecCard/SpecCard.tsx` (+ `.module.css`): read-only relation chips
      mirroring the ticket chip; `aria-label` describing the relation; no editing.

## 4. Refiner agent (Haiku)

- [x] 4.1 `kit/agents/vector-bug-refiner.md`: frontmatter tier **Haiku**, read-only (Read, Grep,
      Glob, optional `Bash(git *)`); pattern `kit/agents/vector-spec-refiner.md`.
- [x] 4.2 Input `RAW_BUG` + deduced causes (context); output the 8-section brief (problem / expected
      / actual / reproduction / acceptance / test plan / risks / open questions). Surfaces ambiguity;
      invents nothing; does not edit/decide implementation.

## 5. Project command `/vector:bug`

- [x] 5.1 `kit/commands/vector/bug.md`: frontmatter (`name: bug`,
      `argument-hint: "[bug-report] {spec-id|branch|file}"`, `user-invocable: true`,
      `allowed-tools`: Read, Grep, Glob, `Bash(git *)`, `Bash(vector *)`, Agent, AskUserQuestion,
      Skill). Pattern `kit/commands/vector/raw.md`, `comment.md`.
- [x] 5.2 Parse: split `RAW_BUG` from optional `{spec-id|branch|file}`; empty report →
      `AskUserQuestion` and stop. Confirm `.vector/config.json` (`vector init`).
- [x] 5.3 Cause deduction (main loop): `git blame`/`log -S`/`--grep` → suspect commits → map to
      Vector spec (`spec list --json`) or ticket trailer; unique+high-confidence → seed
      (`source: blame`); ambiguous/multiple/low/no-match → `AskUserQuestion` (+ "none" + manual).
      Never guess. No git / non-git / files absent → skip with notice.
- [x] 5.4 Refine (Haiku `vector-bug-refiner`); clarify blocking ambiguity (`AskUserQuestion`, ≤5 per
      batch); unresolved → "Open questions".
- [x] 5.5 Compose the canonical 20-section bug-framed spec (`.claude/vector/spec-template.md`);
      derive `title` (≤8 words) + `id` kebab-case with `fix-` prefix; expected/actual + reproduction
      in §8/§11, causes in §4 + `relatedTo[]`.
- [x] 5.6 Validate (Sonnet `vector-spec-validator`): PASS → continue; warnings → show/decide; BLOCK →
      fix + revalidate (≤3 cycles, else surface and do not register).
- [x] 5.7 Register: `vector spec create --title … --id fix-<slug> --status draft --related '<json>'
      --body-file -`. Then token routing: two `vector spec route` calls (Haiku refine, Sonnet
      validate) with all flags (`route.go:33`).
- [x] 5.8 Report: id, `status: draft`, `specDoc`, registered relations, validator verdict, next step
      `/vector:propose`. Language = `config.language` (conversation fallback). Document token routing
      (refiner=Haiku, validator=Sonnet, orchestration=main loop) + CLI-owns-writes reminder.

## 6. Vendoring + verification

- [x] 6.1 `go -C cli generate ./internal/scaffold` (copies command + agent to `assets/`); do not
      edit assets by hand.
- [x] 6.2 `cli/internal/scaffold/scaffold_test.go`: add `bug.md` + `vector-bug-refiner.md` to the
      expected set only if it enumerates the set.
- [x] 6.3 Verify `vector init` on a clean repo seeds `bug.md` + `vector-bug-refiner.md`.

## 7. Tests

- [x] 7.1 `cli/internal/state/store_test.go`: create with `--related`, `RelateSpec` idempotent,
      `spec.related` event, backward compatibility (spec without `relatedTo`).
- [x] 7.2 `cli/internal/board/board_test.go`: `Card` projection includes `relatedTo`.
- [x] 7.3 Parser tests: `--related`/`relate` valid + invalid kind/ref/source.
- [x] 7.4 Scaffold: `bug.md` + `vector-bug-refiner.md` embedded; `vector init` writes them.
- [x] 7.5 Web: `SpecCard` renders `relatedTo` (behavior test, not empty snapshot).
- [x] 7.6 Gate green: `go -C cli generate ./internal/scaffold`, `go -C cli vet ./...`,
      `go -C cli test ./...`; `npm --prefix web run typecheck && lint && build`.

## 8. Docs

- [x] 8.1 `docs/plugin-and-commands.md`: document `/vector:bug` in the kit command index.
- [x] 8.2 `docs/schemas/state-and-activity.md`: document `relatedTo[]` (`kind/ref/source`) + the
      `spec.related` event.
- [x] 8.3 `docs/domain-contract.md`: document the `/vector:bug` → write mapping (§5).
