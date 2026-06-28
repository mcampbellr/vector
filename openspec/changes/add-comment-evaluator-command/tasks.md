# Tasks — add-comment-evaluator-command

## 1. Agente evaluador (Sonnet)

- [x] 1.1 `kit/agents/vector-comment-evaluator.md`: frontmatter tier **Sonnet**, read-only (Read, Grep, Glob, `Bash(git *)`); patrón `vector-spec-validator.md`.
- [x] 1.2 Rubric de 6 preguntas (factualidad vs código, accionabilidad, problema real vs bikeshedding, ya manejado, contradice convenciones, valor vs costo/riesgo).
- [x] 1.3 Salida estructurada: `VERDICT` (`VÁLIDO Y VALIOSO`/`VÁLIDO PERO MARGINAL`/`INVÁLIDO O SIN VALOR` + `N/10`), `EVIDENCE` (`[SEVERITY] (confidence) file:line — …`), `AI-RED-FLAGS`, `REMEDIATION` (si válido). El agente reúne su propia evidencia desde inputs crudos.

## 2. Project command `/vector:comment`

- [x] 2.1 `kit/commands/vector/comment.md`: frontmatter (`name: comment`, `argument-hint: "[comment-text] {spec-id|branch}"`, `user-invocable: true`, `allowed-tools`).
- [x] 2.2 Parseo: separar `COMMENT` del token opcional `{spec-id|branch}`; comentario vacío → `AskUserQuestion` y detener.
- [x] 2.3 Resolver rama (`git worktree list`/`branch -a`); ambigüedad → `AskUserQuestion`. Nunca adivinar.
- [x] 2.4 Obtener diff: `AskUserQuestion` `gh` (PR) vs `git diff <base>..HEAD` local; diff vacío → reportar y detener; `gh` falla/timeout → fallback local, no silenciar.
- [x] 2.5 Resolver spec card vía `vector spec list --json` (o `{spec-id}`); varios/ninguno → `AskUserQuestion` con "ninguno" (omite `work.logged`).
- [x] 2.6 Invocar el agente Sonnet con `COMMENT`/`WORKTREE`/`BRANCH`/`BASE`/`PR_URL`; re-chequear la evidencia; salida no parseable → `INVÁLIDO O SIN VALOR`, no implementar, ofrecer reintentar.
- [x] 2.7 Reportar veredicto en el idioma configurado del proyecto (`config.language`, con fallback al idioma de la conversación) (confianza, evidencia, AI-red-flags, qué haría falta).
- [x] 2.8 Elegir acción según veredicto (`AskUserQuestion`): implementar (solo válido+bajo riesgo) / reply / nada.
- [x] 2.9 Token routing documentado (Sonnet para evaluar; orquestación en main loop). Recordatorio CLI-owns-writes.

## 3. Implementación condicional + integración de estado

- [x] 3.1 Implementar solo con `VÁLIDO Y VALIOSO` + bajo riesgo; grande/ambiguo → plan-y-confirma.
- [x] 3.2 Detectar comandos de verificación (`.vector/config.json` + manifests); preguntar si falta; correr y reportar resultado real; fallo → no marcar hecho.
- [x] 3.3 Con `SPEC_ID` y verificación OK: `vector spec worklog <id> --files … --tasks … --note "comment: …"`; ofrecer `vector spec status <id> in-progress` solo si está en `review`. No auto-commit.

## 4. Reply

- [x] 4.1 Generar reply grounded en la evidencia (inglés por defecto); pasar por skill `humanizer`; copiar a clipboard e imprimir; no postear.

## 5. Vendoring + verificación

- [x] 5.1 `go -C cli generate ./internal/scaffold` (copia a `assets/`); no editar los assets a mano.
- [x] 5.2 `cli/internal/scaffold/scaffold_test.go`: añadir `comment.md`/agente al esperado solo si enumera el set.
- [x] 5.3 `go -C cli vet ./...` y `go -C cli test ./internal/scaffold/...` en verde.
- [x] 5.4 Verificar que `vector init` en repo limpio siembra `comment.md` + `vector-comment-evaluator.md`.

## 6. Docs

- [x] 6.1 Actualizar el índice de commands del kit (`docs/plugin-and-commands.md`) si enumera los `/vector:*`.

## Fixes

- [x] Remove hardcoded Spanish from /vector:comment verdict-report language; adopt the per-project config.language policy with conversation-language auto-detect fallback.
