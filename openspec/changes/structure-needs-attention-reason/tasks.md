# Tasks — structure-needs-attention-reason

## 1. State (cli/internal/state)

- [ ] 1.1 `types.go`: add `Category`/`Summary`/`Detail` (`omitempty`) to `Attention` + a comment
      that they are optional/additive (legacy specs deserialize to `""`). Add the category enum
      validator (`validAttentionCategories`). No changes to `Reason`/`Since`/`Source`, no
      `SchemaVersion` bump.
- [ ] 1.2 `transition.go`: keep `SetStatus` at 5 args; add `SetStatusAttention(id, to, att, actor,
      now)`; extend `transitionOpts` + `applyTransition` for the structured vs legacy construction
      (structured sets `Reason=Summary`; legacy migrates `Category="other"`, `Summary` truncated
      ~80, `Detail=reason`). Preserve the "requires a reason" error when neither arrives, and the
      `Flag` cleanup on exit.
- [ ] 1.3 `attention_test.go` (**new**, do not edit `transition_test.go`): `SetStatusAttention`
      persists the 3 fields + `Reason=Summary`, invalid enum → error; `SetStatus` legacy migrates
      correctly; missing reason/summary → existing error preserved.

## 2. Binary (cli/cmd/vector)

- [ ] 2.1 `spec_transitions.go`: add `--summary`/`--category`/`--detail`/`--detail-file` to
      `runSpecStatus`; validate (only for `needs-attention`) mutual exclusion `--reason` vs new
      flags, enum membership, `--detail` vs `--detail-file` exclusion, `--detail-file` read; route
      to `SetStatusAttention` (structured) or `SetStatus` (legacy). Keep `--json` and other targets
      unchanged.
- [ ] 2.2 `spec_transitions_test.go`: mutual exclusion → error; invalid `--category` → error;
      `--detail` vs `--detail-file` exclusive; `--detail-file` reads the file; structured happy
      path (`--category`+`--summary`+`--detail`).

## 3. Board projection (cli/internal/board)

- [ ] 3.1 `board.go`: add `AttentionCategory`/`AttentionSummary`/`AttentionDetail` (`omitempty`) to
      `Card`; `toCard` maps all four with no branching (`AttentionReason = spec.Flag.Reason`
      unconditional). No `board.SchemaVersion` bump.
- [ ] 3.2 `board_test.go`: `toCard` projects the 4 fields from a full `Attention`; a legacy
      `Attention` (only `Reason`/`Since`/`Source`) projects only `AttentionReason`.

## 4. Web (web/src)

- [ ] 4.1 `types/board.ts`: add `attentionCategory?` (literal union), `attentionSummary?`,
      `attentionDetail?` to `Card`; keep `attentionReason?`.
- [ ] 4.2 `components/SpecCard/AttentionCategoryChip.tsx` (**new**): pure chip, capitalized label +
      per-category CSS Module class consuming `--status-cat-*`; returns `null` for absent/unknown
      category. Mirror `ArtifactDot.tsx`.
- [ ] 4.3 `SpecCard.tsx` + `SpecCard.module.css`: replace the red `<p>` with chip + summary
      truncated ~60 chars (CSS ellipsis + full `title`); fallback to truncated `attentionReason`;
      nothing when unblocked. Add `.attentionRow`/`.attentionSummary`/per-category classes; keep
      `.attention` for the legacy fallback.
- [ ] 4.4 `SpecDetailsDrawer/index.tsx` + `SpecDetailsDrawer.module.css`: lazy `MarkdownView`;
      replace the `<p>` with chip + full summary header + `attentionDetail` via `MarkdownView`
      inside `Suspense`; fallback to plain-text `attentionReason`. `MarkdownView` stays the single
      `react-markdown` import point. Add `.attentionHeader` + reuse/extend `.markdown`.
- [ ] 4.5 `styles/tokens.css`: add 5 `--status-cat-<cat>-fg`/`-bg` pairs across the 4 theme blocks
      (`:root`, dark, light, `@media prefers-color-scheme: dark`), following the status palette.

## 5. Kit docs + assets

- [ ] 5.1 `kit/commands/vector/apply.md` (§4 + §6b), `fix.md` (§5), `status.md` (§2): replace the
      `--reason` example with the structured `--category`/`--summary`/`--detail` contract; note
      `--reason` remains as the legacy path.
- [ ] 5.2 `go generate ./internal/scaffold` from `cli/` to regenerate
      `cli/internal/scaffold/assets/commands/vector/{apply,fix,status}.md` (never edit by hand).

## 6. Web tests

- [ ] 6.1 `SpecCard.test.tsx`: correct chip label/color per category; summary truncated / `title`
      present; fallback to `attentionReason` when the new fields are absent.
- [ ] 6.2 Drawer test (new or extending a `SpecDetailsDrawer/` file, render/DOM like
      `SpecCard.test.tsx`): `attentionDetail` with emphasis/lists/links renders via `MarkdownView`;
      fallback to plain-text `attentionReason` when there is no `attentionDetail`.

## 7. Verification gate

- [ ] 7.1 `go -C cli generate ./internal/scaffold` && `go -C cli test ./internal/scaffold`
      (`TestAssetsMatchKit` — no kit ↔ assets drift).
- [ ] 7.2 `gofmt -l cli` (empty), `go -C cli vet ./...`, `go -C cli test ./...`,
      `go -C cli build ./...`.
- [ ] 7.3 `npm --prefix web run typecheck`, `npm --prefix web run test`, `npm --prefix web run build`.
