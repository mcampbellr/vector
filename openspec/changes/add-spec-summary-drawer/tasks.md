# Tasks — add-spec-summary-drawer

## 1. State — local summary store

- [x] 1.1 `cli/internal/state/summary.go`: `SpecSummary{SchemaVersion, ID, Summary, Action, GeneratedAt}` (JSON-tagged), `summariesPath()` → `.vector/local/summaries.json`.
- [x] 1.2 `Store.ReadSummaries()` (missing file → empty map), `ReadSummary(id)` (nil when absent), `WriteSummary(id, summary, action, now)` (atomic + mutex; stamps version + UTC `GeneratedAt`), mirroring `standup.go`.
- [x] 1.3 Tests: `WriteSummary` → `ReadSummary`/`ReadSummaries` round-trip; missing file → empty map; last-writer-wins per id; never a partial file.

## 2. CLI — `vector spec summarize`

- [x] 2.1 `cli/cmd/vector/summarize.go`: `runSpecSummarize(args)` — `commit` subpath vs projection path (mirror `runStandup`).
- [x] 2.2 Projection: emit `{ id, title, status, ticket?, priorSummary?, events[] }` where `events = standup.Timeline(events, id, from)` over a recent window; `priorSummary` from `ReadSummary(id)`.
- [x] 2.3 Commit: `--action <name>` (required) + `--summary-file -|path` (required); empty/unreadable → write nothing + clear error; else `WriteSummary`.
- [x] 2.4 `case "summarize"` in `runSpec` (`main.go:539`) + usage line.
- [x] 2.5 Tests: projection shape; commit persists valid prose; empty/invalid → writes nothing.

## 3. Standup reuse

- [x] 3.1 Add `PriorSummary string \`json:"priorSummary,omitempty"\`` to `standup.SpecActivity` (keep `Project`/`Timeline` store-free).
- [x] 3.2 `enrichProjection` (`standup.go:90`): set `sa.PriorSummary` from `store.ReadSummary(sa.ID)`.
- [x] 3.3 Test: a spec with a stored summary gets `PriorSummary`; one without stays empty.

## 4. API — `GET /api/summary`

- [x] 4.1 `Source` interface (`board.go:130`): add `ReadSummary(id) (*state.SpecSummary, error)` (`*state.Store` satisfies it).
- [x] 4.2 `server.go`: route `/api/summary` + `handleSummary` — 400 missing `spec`, `{}` (200) when nil, marshaled summary otherwise, 500 on read error (mirror `handleActivity`).
- [x] 4.3 Handler tests: present → 200 body; absent → `{}`; missing param → 400.

## 5. Kit — agent + commands

- [x] 5.1 `kit/agents/vector-summary-writer.md` (Haiku, read-only): summarize projection → short "what was done" prose; transform-only; valid output on empty `events`.
- [x] 5.2 Modify `kit/commands/vector/{apply,propose,status,close,archive}.md`: final post-action step (summarize `--json` → `vector-summary-writer` → `summarize commit --action <command> --summary-file -`).
- [x] 5.3 Modify `kit/agents/vector-standup-writer.md`: add `priorSummary` to the input example + a rule to use it as context; output shape unchanged.
- [x] 5.4 Regenerate scaffold assets: `go generate ./internal/scaffold/...` (do not hand-edit `cli/internal/scaffold/assets/`).

## 6. Web — details drawer

- [x] 6.1 `web/src/types/board.ts`: add `SpecSummary { id, summary, action, generatedAt }`.
- [x] 6.2 `web/src/api/useSpecSummary.ts`: `useSpecSummary(specId | null)` → `GET /api/summary?spec=<id>`, lazy; reuse the `useFetchJSON` helper.
- [x] 6.3 `SpecDetailsDrawer/index.tsx`: right-side panel (header + summary + activity + next command + useful commands); `role="dialog"`, `aria-modal`, close on button/Esc/overlay; lazy fetch on open.
- [x] 6.4 `SpecDetailsDrawer/UsefulCommands.tsx`: copyable, context-aware commands (`/vector:link` only when `!card.ticket`; status/close/archive per legality), `NextCommand` copy pattern.
- [x] 6.5 `SpecCard.tsx`: remove inline `NextCommand` + `SpecTimeline`; clickable `<article>` (`role="button"`, key handling) calling `onSelect(card)`; header/footer unchanged.
- [x] 6.6 `BoardColumn.tsx` thread `onSelect`; `KanbanBoard.tsx` own `selectedCard` state and render one `SpecDetailsDrawer`.
- [x] 6.7 Reuse `SpecTimeline`/`TimelineEntry` and `nextCommandFor` — no duplication.

## 7. Verification + docs

- [x] 7.1 `go -C cli vet ./...` and `go -C cli test ./...` green.
- [x] 7.2 `npm --prefix web run typecheck` and `npm --prefix web run build` green.
- [x] 7.3 Rebuild + reinstall the `vector` binary to `~/.local/bin/vector` (dogfooding).
- [x] 7.4 Resolve or carry forward the spec's open questions (window default, agent wire shape, per-status command set, `link` refresh).
