# Design — detect-external-blocker-in-apply

## Decisiones clave

- **Detección markdown-only**: vive en `apply.md` como juicio del agente guiado por las tres
  señales; **no** se añade un helper binario (`vector spec detect-blockers`) ni se toca Go.
- **CLI-owns-writes**: el command construye la razón y delega la transición a
  `vector spec status <id> needs-attention --reason "…"`; el binario valida y persiste. El command
  nunca edita `state.json`.
- **Guard de falso-positivo = exclusión test-only/cosmético** (determinista, el único filtro
  mecánico). La lógica de "cubierto por otra card" consultando `.vector/specs/` queda **fuera**;
  el deferral apuntado a otro ticket es juicio del agente.
- **Routing automático, independiente de `applyMode`**: es salvaguarda de integridad del board,
  no elección de workflow; no se pide confirmación.
- **`Attention.Source = "command"`** para la `needs-attention` disparada por apply (igual que el
  hard-stop de §4).
- **Una sola transición y una sola razón por run** (la razón puede enumerar varios bloqueos,
  liderando con el que gobierna runtime) — evita componer transiciones potencialmente ilegales.
- **Surfacing reutiliza plumbing existente**: el board lee `needsAttention.reason`, el standup lee
  `TimelineEvent.Reason`; sin plumbing nueva.

## Superficie

- `kit/commands/vector/apply.md`: §6 (sub-paso de detección + transición condicional) y §7
  (surfacing del bloqueo). Resto de §1–§5 y §7 sin cambios.
- `cli/internal/scaffold/assets/commands/vector/apply.md`: copia embebida regenerada por
  `go generate` (no edición manual).
- Go/CLI y web/board/standup: **sin cambios** — se ejercen `runSpecStatus`
  (`cli/cmd/vector/spec_transitions.go:142`), `SetStatus`/`Attention`
  (`cli/internal/state/types.go:134`) y la proyección `TimelineEvent.Reason`
  (`cli/internal/standup/standup.go:138`).

## Flujo

§4 implementa → §5 worklog → **§6 (nuevo sub-paso)** evalúa las 3 señales con el guard
test-only/cosmético → si hay bloqueo: construye razón (qué falta + unblock + PR) y
`vector spec status <id> needs-attention --reason "…"` (automático) → §7 surfacea el bloqueo. Si
limpio: `vector spec status <id> review` (o se deja para `/vector:close`) como hoy → §7 "ready for
review".

## Edge cases

- Card ya en `needs-attention`: refresca el `reason` vigente; el binario valida la transición.
- `tasks.md` ausente: se omite esa señal sin error; se evalúan las otras dos.
- Working tree sin cambios: no hay artefactos para señales 1–2; se evalúa tasks/aceptación si existe.
- El binario rechaza la transición: el command surfacea el error y no enmascara; no edita estado.
- Secreto literal en una señal: la razón describe el faltante **sin** incluir el valor (§14 spec).
