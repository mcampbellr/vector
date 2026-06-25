# Tasks — add-standup-digest

## 1. State + eventos

- [x] 1.1 `EvtWorkLogged = "work.logged"` + `WorkLoggedData{Change, FilesTouched, TasksCompleted, Note}` en `event.go` (estilo `AppliedData`).
- [x] 1.2 `Store.WorkLog(id, data, actor, now)`: valida spec existente, appendea `work.logged`, no toca `state.json`.
- [x] 1.3 `standupPath()` + `ReadStandup()`/`WriteStandup(digest, markerAt)` sobre `.vector/local/standup.json` (serializado por mutex).
- [x] 1.4 Tests: `WorkLog` (appendea, no muta state, spec inexistente → error); `ReadStandup/WriteStandup` round-trip + avance de marcador.

## 2. Proyección (`cli/internal/standup`)

- [x] 2.1 `Project(events, since) Projection`: filtra por `e.TS >= since`, agrupa por `SpecID`, `Totals` por status; sin LLM ni red.
- [x] 2.2 Tests table-driven: filtro `since`, agrupación por spec, periodo vacío sin panic.

## 3. Binario

- [x] 3.1 `case "worklog"` en `runSpec` (`--id`, `--files` csv, `--tasks` csv, `--note` máx 280).
- [x] 3.2 `runStandup`: sin subcomando resuelve `since` (marcador o `--since 24h|today|7d`), proyecta, `--json`/texto.
- [x] 3.3 `standup commit --digest-file <path|->`: valida JSON, `WriteStandup` + avanza marcador; JSON inválido → no escribe, no avanza.
- [x] 3.4 Validación de `--since` con mensaje accionable.

## 4. API (`cli/internal/board`)

- [x] 4.1 `GET /api/standup`: digest persistido (`{}` si no hay).
- [x] 4.2 `GET /api/activity?spec=<id>&since=<dur>`: proyección filtrada; 400 `since` inválido, 404 spec, 500 lectura, body `{error}`.
- [x] 4.3 Tests de handlers (200/{}, 400, 404). No tocar el SSE de `/api/events`.

## 5. Kit

- [x] 5.1 `kit/commands/vector/standup.md`: proyecta → Haiku `vector-standup-writer` → `standup commit`; reporta.
- [x] 5.2 `kit/agents/vector-standup-writer.md` (Haiku): JSON proyección → `{global, perSpec}`.
- [x] 5.3 Modificar `apply.md`: tras implementar, invocar `vector spec worklog` (aditivo, no gate).
- [x] 5.4 Sembrado vía `go generate` (assets) + `vector update`.

## 6. Web

- [x] 6.1 `types/standup.ts` (espejo exacto de §7); `useStandup`/`useSpecActivity`.
- [x] 6.2 `StandupView/index.tsx` + `SpecTimeline/index.tsx` (one-component-per-file); estados loading/success/error/empty.

## 7. Verificación + docs

- [x] 7.1 `go -C cli vet ./...`, `go -C cli test ./...`, `npm --prefix web run typecheck`, `npm --prefix web run build` verdes.
- [x] 7.2 Sin regresiones en `apply`/`status`/`close`/`sync`/`/api/board`/`/api/events`.
- [x] 7.3 Actualizar `docs/domain-contract.md` §4/§5 y `docs/schemas/state-and-activity.md` (`work.logged`).
