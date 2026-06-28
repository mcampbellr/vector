# Design — binary-templated-transition-summaries

## Decisiones clave

- **La señal `hasWork` vive en la projection, no en un subcomando separado.** El command ya
  ejecuta `vector spec summarize <id> --json` como paso 1 del 3-step; agregar `HasWork` y
  `TemplateSummary` al output existente no añade round-trips. Un subcomando aparte costaría
  exactamente lo que se quiere eliminar.

- **`buildTemplateSummary` vive en el binario, no en el command.** El binario tiene los eventos
  ya proyectados y el contexto del spec (id, title) sin serialización adicional. El command no
  tiene acceso a esos datos salvo a través del JSON que ya recibe; trasladar la lógica al
  command violaría el principio CLI-owns-writes y rompería el token-routing: la decisión de qué
  texto usar es determinista y pertenece a la capa de lógica, no de orquestación.

- **`buildTemplateSummary` es función pura sin error de retorno.** Siempre produce un string
  válido (tiene fallback `"spec \"<id>\": no recent activity"`). El command no maneja error aquí;
  solo valida que `templateSummary != ""` antes de pipear.

- **`HasWork bool` (sin omitempty) + `TemplateSummary string` (con omitempty).** El command
  puede branching con un booleano claro; `templateSummary` ausente del JSON en el camino largo
  no infla el payload ni rompe parsers existentes que no esperan el campo.

- **La decisión hasWork/LLM es del command, no del binario.** El binario provee la señal y el
  texto pre-construido; el command decide qué camino tomar. Mantiene la separación de
  responsabilidades: binario = lógica de dominio + escritura, command = orquestación + UX.

- **El safeguard `hasWorkLoggedAfter` en `commit` no se toca.** Para `close`/`archive` con
  prior summary y sin work.logged en la ventana, el safeguard preserva el prior summary aunque
  se le pase el template — doble protección sin duplicar lógica. Para `status`, el template
  siempre se persiste (el safeguard close/archive no aplica allí).

- **`hasWork` opera sobre `[]standup.TimelineEvent` (ventana 24h), no sobre `[]state.Event`.**
  El timeline ya fue filtrado por `summarizeWindow`; operar sobre él evita re-leer el log.
  `hasWorkLoggedAfter` en `commit` opera sobre `[]state.Event` sin filtro de ventana — funciones
  distintas con propósitos distintos; no fusionarlas.

- **`/vector:propose` siempre tomará el camino corto.** `propose` registra `spec.proposed` +
  `status.changed` pero nunca `work.logged`; `hasWork` será siempre `false`. El template
  `"<title> proposed (draft → open)"` es exactamente la descripción correcta de esa transición.

- **`/vector:apply` no se modifica.** Siempre llama `vector spec worklog` antes de summarizar;
  `hasWork` será siempre `true`. Excluirlo reduce complejidad y riesgo sin pérdida de valor.

## Algoritmo de `buildTemplateSummary`

Función pura `(id, title string, events []standup.TimelineEvent) string`:

1. Label = `title` si no vacío; si no, `id`.
2. Scan en orden cronológico:
   - `"spec.proposed"` → `"<label> proposed (draft → open)"`.
   - `"spec.closed"` → `"<label> closed"`.
   - `"spec.archived"` → `"<label> archived"`.
3. Último `"status.changed"` con `From != ""` y `To != ""` → `"<label>: moved from <from> to <to>"`.
4. Fallback → `"spec \"<id>\": no recent activity"`.

No decodifica el campo `Data`; opera solo sobre `Type`, `From`, `To` de `standup.TimelineEvent`.

## Superficie afectada

| Capa | Archivo | Cambio |
|---|---|---|
| Binario | `cli/cmd/vector/summarize.go` | `HasWork`/`TemplateSummary` en `summarizeProjection`; `buildTemplateSummary`; poblar campos en `runSpecSummarize` |
| Tests | `cli/cmd/vector/summarize_test.go` | `TestSummarizeProjectionHasWorkFalseWhenNoWorkLogged`, `TestSummarizeProjectionHasWorkTrueWhenWorkLogged`, `TestBuildTemplateSummary` |
| Kit commands | `kit/commands/vector/archive.md` | §3: bifurcación hasWork |
| Kit commands | `kit/commands/vector/close.md` | §3: bifurcación hasWork |
| Kit commands | `kit/commands/vector/status.md` | §3: bifurcación hasWork (sin safeguard close/archive) |
| Kit commands | `kit/commands/vector/propose.md` | §7: bifurcación hasWork |
| Scaffold assets | `cli/internal/scaffold/assets/commands/vector/{archive,close,status,propose}.md` | Copias byte-idénticas a sus fuentes en `kit/` |

## Sin cambios en

- `runSpecSummarizeCommit` y el safeguard `hasWorkLoggedAfter`.
- `cli/internal/state/` (ningún tipo ni evento nuevo).
- `cli/internal/standup/standup.go` (`TimelineEvent` se reutiliza tal cual).
- `web/` (el board consume `summaries.json` vía la API; el contenido del texto no afecta la UI).
- `vector-summary-writer.md` (su prompt no cambia; simplemente se lo invoca con menos frecuencia).
- La ventana `summarizeWindow` (permanece en `"24h"`).
