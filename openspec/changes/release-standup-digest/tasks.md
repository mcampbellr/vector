# Tasks — release-standup-digest

## 1. Build + re-embed

- [x] 1.1 `npm --prefix web run build` verde; `web/dist` contiene `index.html` + `assets/*`.
- [x] 1.2 Copiar `web/dist/.` a `cli/internal/webui/dist/` (sobre-escribe el build anterior).
- [x] 1.3 Verificar que el embed solo trae build de producción (sin `.env`/fuentes/secrets).
- [x] 1.4 `go -C cli generate ./internal/scaffold` sin errores (assets del kit sincronizados).

## 2. Recompilar e reinstalar

- [x] 2.1 `go -C cli build -o ~/.local/bin/vector ./cmd/vector` compila exitosamente.
- [x] 2.2 `~/.local/bin/vector version` responde (binario nuevo en PATH).
- [x] 2.3 `vector serve` levanta y loguea `ui: embedded` (no `stale`).

## 3. Gate de calidad (no-regresiones)

- [x] 3.1 `go -C cli vet ./...` + `go -C cli test ./...` verdes.
- [x] 3.2 `npm --prefix web run typecheck` + `npm --prefix web run build` verdes.
- [x] 3.3 `apply`/`status`/`close`, `/api/board`, `/api/events` (SSE) intactos; `/vector:apply` invoca `worklog` sin error.

## 4. UAT — happy path (manual)

- [ ] 4.1 Board carga con el tab **Standup** y el **SpecTimeline** expandible por card.
- [x] 4.2 `/vector:standup` end-to-end: proyecta → digest Haiku → `standup commit` persiste → marcador avanza → `GET /api/standup` devuelve el digest → board lo refleja por SSE.
- [x] 4.3 `vector standup --since 24h|today|7d` proyecta la ventana correcta.

## 5. UAT — edge cases (manual)

- [x] 5.1 `--since`/`?since=` inválido → error/`400` `invalid since: use 24h, today or 7d`.
- [x] 5.2 Periodo sin actividad → digest `{}` / "no activity since last standup"; marcador igual avanza al commitear.
- [x] 5.3 `standup commit --digest-file -` con JSON inválido → `invalid digest json`; **no escribe digest y NO avanza marcador** (verificar ambos).
- [x] 5.4 `/api/activity?spec=<inexistente>` → `404 {error}`; `activity.jsonl` ausente/corrupto → sin panic (líneas corruptas a stderr).

## 6. UAT — estados de UI (manual)

- [ ] 6.1 loading (`loading standup…` / `loading activity…`), success, empty, error con **retry** visibles.
- [ ] 6.2 Labels en inglés renderizan (standup.title/period/empty/error/retry, timeline.header/more/retry).

## 7. Docs

- [x] 7.1 Registrar la sesión de UAT (checklist + resultado) en `docs/uat.md`.
- [x] 7.2 Actualizar `docs/status.md` (binario con standup reinstalado; `add-standup-digest` listo para `/vector:close`).
- [x] 7.3 Confirmar que `.vector/local/` (activity log / standup.json) no quedó staged en git.
