# Design — add-comment-evaluator-command

## Context

El usuario usa `/pr-comment` (skill global) para evaluar comentarios de PR contra el código real y
aplicarlos solo si valen la pena. Ese skill es somnio-específico (pnpm, `graphify-out/`, layout de
worktrees fijo, máx-5-files, GitHub vía `gh`) y vive fuera de Vector. Vector necesita lo mismo pero
agnóstico al repo del usuario y embebido en el binario, y puede además vincular el trabajo
resultante a un spec card. El binario ya expone `vector spec worklog`/`status`/`list`, y el kit ya
tiene el patrón de project command (`apply.md`) + agente (`vector-spec-validator.md`) y el vendoring
vía `go generate` (`cli/internal/scaffold/scaffold.go:13,26`).

## Goals / Non-Goals

**Goals:**
- Port fiel y agnosticizado de `/pr-comment`: evaluar → veredicto → acción condicional.
- Evaluación crítica delegada a un agente **Sonnet** read-only con veredicto estructurado.
- Resolución de diff agnóstica (preguntar `gh` vs `git diff` local) y detección de verificación
  (`.vector/config.json` + manifests, preguntar si falta).
- Spec-aware: al implementar, `work.logged` vía el binario (+ oferta `review → in-progress`).

**Non-Goals:**
- Postear el reply, auto-commit/push, validar el ticket contra el tracker.
- Código Go nuevo, eventos (`comment.evaluated`), endpoints o UI del board nuevos.
- Lógica somnio-específica (pnpm, `graphify-out`, máx-5-files, layout de worktrees fijo).
- Reformular el comentario antes de evaluarlo.

## Decisions

- **Spec-aware vía subcomandos existentes**: el command invoca `vector spec worklog`/`status`/
  `list`; no se añade código Go. Mantiene CLI-owns-writes sin duplicar `apply`.
- **`work.logged` aditivo**: se appendea solo al implementar y solo si hay spec resuelto; sin spec
  se omite sin error.
- **Diff = preguntar al usuario** (`gh` con PR vs `git diff <base>..HEAD` local) en cada corrida:
  Vector no puede asumir GitHub.
- **Verificación = detectar y preguntar si falta**: agnosticism; nunca hardcodear `pnpm`.
- **Evaluador en tier Sonnet**: el juicio escéptico (verificar claims, detectar AI-slop, sopesar
  valor/riesgo) es razonamiento real; token-routing autoriza el tier caro cuando aporta valor. La
  orquestación barata (parseo, rama, diff) queda en el main loop.
- **Veredicto antes de implementar; nunca sin `VÁLIDO Y VALIOSO` + bajo riesgo**: esencia de
  `/pr-comment`. Cambios grandes/ambiguos → plan-y-confirma.
- **Reply humanizado (skill `humanizer`), no posteado; sin auto-commit**: el usuario controla lo
  que se publica/commitea.

## Superficie

- `kit/commands/vector/comment.md`: project command (parseo, rama, diff, spec, evaluación, acción).
- `kit/agents/vector-comment-evaluator.md`: agente Sonnet read-only (rubric de 6 preguntas).
- `cli/internal/scaffold/assets/{commands/vector/comment.md,agents/vector-comment-evaluator.md}`:
  copias embebidas regeneradas por `go generate`.
- `cli/internal/scaffold/scaffold_test.go`: solo si enumera el set de commands esperados.
- Reuso (sin cambios): `vector spec worklog` (`cli/cmd/vector/standup.go:199`), `vector spec status`
  (`cli/cmd/vector/spec_transitions.go:143`), `vector spec list`.

## Risks / Trade-offs

- **Agnosticism imperfecto**: la detección de verificación/rama puede fallar en repos variados →
  mitigación: preguntar al usuario en vez de adivinar (default elegido).
- **Salida del evaluador no parseable o veredicto contra-evidencia**: el main loop re-chequea
  `file:line` y, si la salida no es legible, la trata como `INVÁLIDO O SIN VALOR` sin implementar.
- **Costo de Sonnet por invocación**: aceptado; corre una sola vez por comentario, la orquestación
  no usa tier caro.
- **Vendoring incorrecto rompería el scaffold**: mitigación: `go generate` + test del scaffold +
  verificar `vector init` en un repo limpio.

## Open questions

- ¿Cachear los comandos de verificación detectados en `.vector/config.json` para no re-preguntar?
  (Sugerido fuera de V1 si añade complejidad.)
- ¿Política de transición además de `review → in-progress` (p. ej. `needs-attention`)?
- ¿`scaffold_test.go` enumera el set de commands (requiere editar) o solo valida presencia?
