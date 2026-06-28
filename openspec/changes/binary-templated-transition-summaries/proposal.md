# Binary-Templated Transition Summaries

## Why

Cada transición estructural de un spec — archivar, cerrar, cambiar estado, proponer — termina
con un paso de summarize que hoy siempre spawna el agente `vector-summary-writer` (Haiku) y
espera dos round-trips. Cuando la ventana de actividad no contiene eventos `work.logged`, la
prosa resultante es invariablemente trivial: una oración que describe la transición. En esos
casos, invocar un LLM añade ~500–800ms y costo de tokens sin aportar calidad observable. El
binario ya tiene todos los datos necesarios para generar esa oración determinísticamente.

## What changes

- `summarizeProjection` (output de `vector spec summarize <id> --json`) recibe dos campos
  nuevos: `hasWork bool` (siempre presente) y `templateSummary string` (omitempty, solo cuando
  `hasWork == false`).
- Nueva función pura `buildTemplateSummary(id, title string, events []standup.TimelineEvent)
  string` en el binario: detecta el tipo de transición desde los eventos del timeline y retorna
  la oración apropiada; tiene fallback y nunca retorna error.
- Los cuatro project commands con paso de summarize post-acción (`/vector:archive`,
  `/vector:close`, `/vector:status`, `/vector:propose`) pasan a tener dos caminos: si
  `hasWork == false`, pipean `templateSummary` directamente a `vector spec summarize commit`
  sin spawnar Haiku; si `hasWork == true`, siguen el flujo actual sin cambios.
- Las cuatro copias vendorizadas en `cli/internal/scaffold/assets/commands/vector/` se
  sincronizan con sus fuentes en `kit/commands/vector/`.
- Tests nuevos en `summarize_test.go` cubren `hasWork` verdadero/falso y los cinco casos del
  algoritmo de `buildTemplateSummary`.

## Scope

**In:**
- `summarizeProjection` extendido con `HasWork` + `TemplateSummary`.
- `buildTemplateSummary` (función pura, sin I/O).
- Lógica condicional en `archive.md`, `close.md`, `status.md`, `propose.md` (solo el paso de
  summarize de cada uno).
- Copias vendorizadas de los cuatro commands en `cli/internal/scaffold/assets/`.
- Tests nuevos en `cli/cmd/vector/summarize_test.go`.

**Out:**
- `/vector:apply` — siempre tiene `work.logged`; no se toca.
- `/vector:link` — no tiene paso de summarize; no se toca.
- El agente `vector-summary-writer` — su prompt no cambia.
- `vector spec summarize <id> commit` — su código no cambia.
- Cambios al schema de `SpecState`, `Event`, o `activity.jsonl`.
- El pipeline de standup.
- Telemetría del ahorro (`agent.routed` con `model: "binary-template"`).
- Internacionalización: el binario emite strings en inglés, consistente con el CLI actual.

Authored spec: `.vector/specs/binary-templated-transition-summaries/spec.md`.
