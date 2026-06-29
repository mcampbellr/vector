# Design — add-research-command

## Context

`/vector:raw` ya autora specs de forma self-contained (refiner Haiku + validator Sonnet, registro
`draft`), pero no evalúa si la idea **vale la pena**. El usuario quiere un comando que investigue la
viabilidad de un feature antes de comprometerse a especificarlo: veredictos por disciplina,
decisión humana go/no-go y, si procede, un spec ya enriquecido con esa investigación. El binario ya
expone `vector spec create` (`cli/cmd/vector/main.go:568,709`) y `vector spec route` (`:592`), el
kit ya tiene el patrón de project command (`raw.md`, `comment.md`) y de agente
(`vector-comment-evaluator.md`, `vector-spec-validator.md`), y el vendoring vía `go generate`
(`cli/internal/scaffold/scaffold.go:13,26`). `research` se apoya en todo eso sin código Go nuevo.

## Goals / Non-Goals

**Goals:**
- Investigación multidisciplinar de la idea: lentes `technical` (núcleo) + `security`/`marketing`/
  `design` por señales, cada una con veredicto estructurado de un agente Sonnet read-only.
- Detección barata de lentes en el main loop; ajuste por el usuario si es ambiguo.
- Reuso del pipeline de autoría de `raw` (refiner Haiku + validator Sonnet) sin duplicarlo.
- Gate explícito go/no-go: el humano decide; sin "go" no se crea card.
- Spec de 20 secciones + anexo `## Reporte de viabilidad` embebido; card `draft` vía el binario.

**Non-Goals:**
- Código Go nuevo, eventos (`spec.researched`), endpoints o UI del board nuevos.
- Investigación web / fuentes externas (competidores, prior art): V1 es sobre la idea + el repo.
- Crear el OpenSpec change o implementar el feature investigado (eso es `/vector:propose`/`apply`).
- Lentes más allá de las cuatro; correr las cuatro "por si acaso"; auto-commit/push.

## Decisions

- **Self-contained, reusando el patrón de `raw`**: `research` compone refiner+validator en su propio
  flujo añadiendo la capa de viabilidad; **no** invoca `/vector:raw` como comando externo. Un solo
  comando, la investigación alimenta el spec directo.
- **Un agente revisor parametrizado por lente** (no un agente por disciplina): mantiene el kit
  pequeño y el vendoring simple; cada invocación trae su rubric de lente. (Alternativa —agentes
  especializados— en Open questions.)
- **Auto-detección con núcleo mínimo**: `technical` siempre; el resto por señales del texto (§13 del
  spec); ambigüedad → `AskUserQuestion`. Token-routing: no gastar Sonnet en lentes irrelevantes.
- **Tiering**: detección/orquestación/consolidación en el main loop; refinamiento en **Haiku**;
  revisiones de viabilidad y validación en **Sonnet** (el juicio de viabilidad es razonamiento real;
  token-routing autoriza el tier caro cuando aporta valor). Las lentes pueden correr en paralelo.
- **Re-chequeo del main loop**: no confiar ciegamente en cada lente; validar la evidencia `file:line`
  citada; si no se sostiene, degradar el veredicto y notarlo. Salida no parseable → tratarla como
  `go-with-risks` "no concluyente", ofrecer reintentar; nunca inventar un veredicto.
- **Gate explícito go/no-go**: tras consolidar (no-go si una lente crítica es no-go), el usuario
  decide emitir / refinar más / abortar. Abortar → terminar **sin** crear card ni escribir spec doc.
- **Card `draft` + reporte embebido como anexo** (después de §20): un solo artefacto, consistente con
  que `raw` crea `draft`; el reporte viaja con el spec. Siempre `draft` en V1 (el riesgo vive en el
  reporte), no `needs-attention`.
- **CLI-owns-writes**: card/spec doc/route solo vía `vector spec create`/`route`; nunca editar
  `.vector/` a mano. Sin código Go, eventos ni endpoints nuevos.

## Superficie

- `kit/commands/vector/research.md`: project command (intake, detección de lentes, refinamiento,
  revisiones por lente, consolidación, gate, composición + reporte embebido, registro `draft`).
- `kit/agents/vector-feasibility-reviewer.md`: agente Sonnet read-only, rubric por lente, salida
  estructurada (`LENS`/`VERDICT`+`N/10`/`FINDINGS`/`RISKS`/`RECOMMENDATION`).
- `cli/internal/scaffold/assets/{commands/vector/research.md,agents/vector-feasibility-reviewer.md}`:
  copias embebidas regeneradas por `go generate`.
- `cli/internal/scaffold/scaffold_test.go`: añadir el par `research.md`+agente si enumera el set
  (patrón `TestSeedCommandsSeedsBugCommandAndRefiner`).
- Reuso (sin cambios): `vector spec create` (`cli/cmd/vector/main.go:568,709`), `vector spec route`
  (`:592`), agentes `vector-spec-refiner`/`vector-spec-validator`.

## Risks / Trade-offs

- **Agnosticism imperfecto en la detección de lentes**: repos/ideas variados → señales ambiguas.
  Mitigación: `AskUserQuestion` para ajustar el set en vez de adivinar (default elegido).
- **Lente que cita código/símbolos inexistentes o salida no parseable**: el main loop re-chequea,
  baja la confianza y, si es ilegible, la marca no concluyente sin inventar veredicto.
- **Costo de Sonnet por lente**: aceptado; solo corren las lentes detectadas (no las cuatro) y en
  paralelo; el refinamiento/orquestación no usan tier caro.
- **Vendoring incorrecto rompería el scaffold**: mitigación: `go generate` + test del scaffold +
  verificar `vector init` en un repo limpio.

## Open questions

- ¿Agente parametrizado por lente (V1) o agentes especializados por disciplina si la calidad lo
  justifica?
- ¿Lentes con investigación web (competidores, prior art) en una extensión futura? V1: idea + repo.
- ¿Persistir el mapa señal→lente o el set por tipo de idea en `.vector/config.json`? Fuera de V1.
- ¿`go-with-risks` debería mover la card a `needs-attention` en vez de `draft`? V1: siempre `draft`.
- ¿`scaffold_test.go` enumera el set (requiere editar) o solo valida presencia?
