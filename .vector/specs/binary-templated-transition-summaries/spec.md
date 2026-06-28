# Spec: Binary-Templated Transition Summaries

## 1. Objetivo

Construir el mecanismo de **summary templado** para transiciones estructurales: cuando la ventana de
actividad de un spec NO contiene eventos `work.logged`, el binario genera el resumen
determinísticamente (plantilla) y el command lo persiste directamente, sin spawnar el agente
`vector-summary-writer` (Haiku).

Esta feature permite que los commands `/vector:archive`, `/vector:close`, `/vector:status` y
`/vector:propose` eviten un spawn Haiku + 2 round-trips en toda transición que sea puramente
estructural (cambio de estado sin trabajo sustantivo), reduciendo latencia y costo de tokens
sin degradar la calidad: la prosa de una transición sin trabajo es siempre una plantilla
trivial, y cuando hay trabajo real sigue corriendo el agente LLM.

## 2. Alcance

### Incluido en esta fase

- Dos campos nuevos en `summarizeProjection` (la estructura que `vector spec summarize <id> --json`
  devuelve): `hasWork bool` y `templateSummary string`.
- Función pura `buildTemplateSummary(id, title string, events []standup.TimelineEvent) string`
  en `cli/cmd/vector/summarize.go`, que genera la oración determinista desde los eventos del
  timeline.
- Actualización de los cuatro project commands con el 3-step summarize que cambia su
  comportamiento: **archive** (§3 del command), **close** (§3), **status** (§3), **propose**
  (§7). Cada uno pasa de tres pasos siempre a dos caminos: si `hasWork == false` → pipar el
  template directamente a `summarize commit` (sin Haiku); si `hasWork == true` → flujo actual
  inalterado.
- Actualización de las cuatro copias vendorizadas en
  `cli/internal/scaffold/assets/commands/vector/` (sincronizadas vía `go generate`).
- Tests nuevos en `cli/cmd/vector/summarize_test.go`: `hasWork` verdadero/falso según
  presencia/ausencia de eventos `work.logged`, y calidad del template para los tipos de evento
  relevantes.

### Fuera de scope

- `/vector:apply` — siempre llama `vector spec worklog` antes de summarizar; `hasWork` siempre
  será `true`. No se toca.
- `/vector:link` — no tiene el 3-step summarize; la operación de link es metadata pura sin
  summary post-acción. No se toca.
- El agente `vector-summary-writer` — su prompt no cambia; simplemente se le llama con menos
  frecuencia.
- El subcomando `vector spec summarize commit` — su código no cambia; la preservación
  close/archive existente (`hasWorkLoggedAfter`) sigue siendo el guard en el nivel del binario
  para el caso en que se llame con un summary LLM.
- Cambios al schema de `SpecState`, `Event`, o `activity.jsonl`.
- El pipeline del standup (`/vector:standup`, `vector-standup-writer`).
- Rutas de escritura de estado distintas al summary (`state.json`, `activity.jsonl`).
- Nuevos subcomandos o flags del binario distintos a los dos campos de la projection.
- Internacionalización del texto del template: el binario ya emite strings hardcodeados en
  inglés (consistente con el resto del CLI).

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca
relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Binario: **Go** (módulo único, stdlib; `cli/`). Sin dependencias externas.
- Project commands: **Markdown + frontmatter** ejecutados por Claude Code
  (`kit/commands/vector/*.md`), copiados por `init`/`update` al repo del usuario.

### Versiones relevantes

- Go: `1.26` (de `cli/go.mod`). Sin deps externas.
- Kit commands: artefactos Markdown; no tienen versión propia, se sincronizan con el release del
  binario.

No usar librerías, APIs, flags o patrones no documentados oficialmente ni ausentes del proyecto,
salvo los autorizados explícitamente en este spec.

### Patrones existentes a respetar

- **`summarizeProjection`** (`cli/cmd/vector/summarize.go`): el struct de salida de `vector spec
  summarize <id> --json`. Se extiende con dos campos; no se rompe el shape actual (nuevos campos
  son omitempty o siempre presentes).
- **Función pura sin side effects**: `buildTemplateSummary` opera solo sobre los datos de los
  eventos ya proyectados en el timeline; no abre el store, no lee disco.
- **Errores explícitos**: los helpers del binario usan `fmt.Errorf("…: %w", err)`. `buildTemplateSummary`
  nunca retorna error (siempre produce un string, tiene fallback).
- **Go estilo idiomático**: `gofmt`, `go vet`, tabla-driven en tests, nombres descriptivos (sin
  variables de una letra). `standards/go-conventions.md`.
- **CLI-owns-writes**: el binario es el único que escribe `.vector/local/summaries.json`; el
  command orquesta, nunca escribe estado a mano.
- **Token routing**: el command decide en base a `hasWork`; el binario no spawna agentes
  (`product/token-routing.md`).
- **Copias vendorizadas**: `kit/commands/vector/*.md` se copian manualmente a
  `cli/internal/scaffold/assets/commands/vector/*.md`; el `go generate` en `internal/scaffold`
  lee de `assets/`. Cuando se modifica un command en `kit/`, se actualiza su copia de assets.
- **Commit de las 3 copias**: cada command existe en `kit/commands/vector/`, en
  `.claude/commands/vector/` (repo Vector local), y en `cli/internal/scaffold/assets/commands/
  vector/`. Los tres deben estar en sync al finalizar.

---

## 4. Dependencias previas

Antes de iniciar esta fase deben existir:

- [x] `cli/cmd/vector/summarize.go` con `summarizeProjection`, `runSpecSummarize`,
  `runSpecSummarizeCommit` y `hasWorkLoggedAfter` — ya implementados.
- [x] `cli/cmd/vector/summarize_test.go` con `TestSummarizeCommitClosePreservesPriorWhenNoNewWork`
  — ya implementado; es el precedente directo de esta feature.
- [x] `cli/internal/standup/standup.go` con `Timeline` y `TimelineEvent` — ya implementados;
  `buildTemplateSummary` recibe `[]TimelineEvent` como input.
- [x] `cli/internal/state/event.go` con `EvtSpecProposed`, `EvtSpecClosed`, `EvtSpecArchived`,
  `EvtStatusChanged`, `EvtWorkLogged` — ya definidos.
- [x] `kit/commands/vector/archive.md`, `close.md`, `status.md`, `propose.md` con el 3-step
  summarize — ya implementados; son los que se modifican.
- [x] `cli/internal/scaffold/assets/commands/vector/{archive,close,status,propose}.md` — copias
  vendorizadas existentes.

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta.

---

## 5. Arquitectura

### Patrón

Extensión del patrón existente de **projection + agent prose** con un **camino corto
determinista**: el binario expone la señal (`hasWork`) y el texto pre-construido
(`templateSummary`); el command decide qué camino tomar. El binario nunca spawna agentes
(`CLI-owns-writes`); el command nunca escribe estado a mano.

### Capas afectadas

- **Binario CLI** (`cli/cmd/vector/summarize.go`): sí — dos campos nuevos en
  `summarizeProjection` + función pura `buildTemplateSummary`.
- **Project commands** (`kit/commands/vector/{archive,close,status,propose}.md`): sí — lógica
  condicional en el paso de summarize (cada uno tiene un paso de summarize post-acción).
- **Scaffold assets** (`cli/internal/scaffold/assets/commands/vector/`): sí — copias
  actualizadas de los cuatro commands.
- **State** (`cli/internal/state/`): no — ningún cambio en el store ni en los tipos del dominio.
- **Standup** (`cli/internal/standup/`): no — se reutiliza `TimelineEvent` sin modificar.
- **`web/`**: no — el board ya consume los summaries desde `summaries.json` vía la API; el
  contenido del texto no afecta la interfaz.
- **Agent `vector-summary-writer`**: no — su prompt no cambia.

### Flujo esperado — camino corto (hasWork == false)

1. El command ejecuta `vector spec summarize <id> --json` (ya existe, paso 1 del 3-step).
2. El binario proyecta el timeline; detecta que ningún evento es `work.logged` → `hasWork = false`;
   calcula `buildTemplateSummary(id, title, events)` → `templateSummary = "..."`.
3. Devuelve el JSON con los dos campos nuevos al command.
4. El command parsea `hasWork = false` → **no spawna** `vector-summary-writer`.
5. El command formatea `{"summary": "<templateSummary>"}` inline y lo pipa directamente a
   `vector spec summarize <id> commit --action <action> --summary-file -`.
6. El binario persiste (o preserva, si `action == close/archive` y hay prior summary con
   `hasWorkLoggedAfter == false` — safeguard existente).

**Ahorro**: se elimina el spawn del agente Haiku y el round-trip de la respuesta del agente.
El paso 1 (`--json`) y el paso 3 (`commit`) permanecen; el paso 2 (agente) desaparece.

### Flujo esperado — camino largo (hasWork == true)

El flujo actual sin cambios: `--json` → spawn Haiku → `commit`. Cuando hay `work.logged` en
el timeline, el binario sigue devolviendo `hasWork = true` + `templateSummary = ""` (o string
vacío), y el command usa el camino largo existente.

### Algoritmo de `buildTemplateSummary`

Función pura, sin I/O, recibe `(id, title string, events []standup.TimelineEvent)`:

1. Calcular el label: `title` si no vacío, si no `id`.
2. Escanear `events` en orden cronológico buscando:
   - `"spec.proposed"` → retornar `fmt.Sprintf("%s proposed (draft → open)", label)`.
   - `"spec.closed"` → retornar `fmt.Sprintf("%s closed", label)`.
   - `"spec.archived"` → retornar `fmt.Sprintf("%s archived", label)`.
3. Si no se encontró ninguno de los anteriores, buscar el **último** `"status.changed"` con
   `From != ""` y `To != ""` → retornar `fmt.Sprintf("%s: moved from %s to %s", label, e.From, e.To)`.
4. Fallback (ventana vacía o eventos irrelevantes): `fmt.Sprintf("spec %q: no recent activity", id)`.

No decodifica el campo `Data` de los eventos (ya fue aplanado por `standup.Timeline`). Opera
solo sobre los campos de `standup.TimelineEvent` (`Type`, `From`, `To`).

### Cómo `hasWork` se computa en el binario

En `runSpecSummarize`, antes de construir la `summarizeProjection`:

```go
hasWork := false
for _, te := range timelineEvents {
    if te.Type == string(state.EvtWorkLogged) {
        hasWork = true
        break
    }
}
proj.HasWork = hasWork
if !hasWork {
    proj.TemplateSummary = buildTemplateSummary(spec.ID, spec.Title, timelineEvents)
}
```

`TemplateSummary` se omite del JSON cuando `hasWork == true` (campo vacío con `omitempty`).

### Ubicación de archivos modificados

```
cli/cmd/vector/
  summarize.go          ← dos campos en summarizeProjection + buildTemplateSummary
  summarize_test.go     ← tests nuevos

kit/commands/vector/
  archive.md            ← §3: lógica condicional hasWork
  close.md              ← §3: lógica condicional hasWork
  status.md             ← §3: lógica condicional hasWork
  propose.md            ← §7: lógica condicional hasWork

cli/internal/scaffold/assets/commands/vector/
  archive.md            ← copia vendorizada de kit/commands/vector/archive.md
  close.md              ← ídem
  status.md             ← ídem
  propose.md            ← ídem
```

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `cli/cmd/vector/summarize.go` | MODIFICAR | Agregar `HasWork`/`TemplateSummary` a `summarizeProjection` + `buildTemplateSummary` | mismo archivo (`runSpecSummarize`, `hasWorkLoggedAfter`) |
| `cli/cmd/vector/summarize_test.go` | MODIFICAR | Tests de `hasWork` y `buildTemplateSummary` | mismo archivo (`TestSummarizeProjectionShape`, `TestSummarizeCommitClosePreservesPriorWhenNoNewWork`) |
| `kit/commands/vector/archive.md` | MODIFICAR | §3: camino corto si `hasWork == false` | `kit/commands/vector/close.md` (mismo patrón) |
| `kit/commands/vector/close.md` | MODIFICAR | §3: camino corto si `hasWork == false` | `kit/commands/vector/archive.md` (mismo patrón) |
| `kit/commands/vector/status.md` | MODIFICAR | §3: camino corto si `hasWork == false` | `kit/commands/vector/close.md` |
| `kit/commands/vector/propose.md` | MODIFICAR | §7: camino corto si `hasWork == false` | `kit/commands/vector/close.md` |
| `cli/internal/scaffold/assets/commands/vector/archive.md` | MODIFICAR | Copia vendorizada de `kit/commands/vector/archive.md` | `kit/commands/vector/archive.md` |
| `cli/internal/scaffold/assets/commands/vector/close.md` | MODIFICAR | Copia vendorizada | `kit/commands/vector/close.md` |
| `cli/internal/scaffold/assets/commands/vector/status.md` | MODIFICAR | Copia vendorizada | `kit/commands/vector/status.md` |
| `cli/internal/scaffold/assets/commands/vector/propose.md` | MODIFICAR | Copia vendorizada | `kit/commands/vector/propose.md` |

### Detalle por archivo

#### `cli/cmd/vector/summarize.go` — MODIFICAR

Cambios requeridos:

1. En `summarizeProjection` agregar:
   ```go
   HasWork        bool   `json:"hasWork"`
   TemplateSummary string `json:"templateSummary,omitempty"`
   ```
   `HasWork` siempre presente en el JSON (sin `omitempty`); `TemplateSummary` solo cuando
   `HasWork == false`.

2. En `runSpecSummarize`, justo antes de construir `proj`, calcular `hasWork` iterando
   `proj.Events` (los `TimelineEvent` ya calculados) buscando `Type == "work.logged"`:
   ```go
   hasWork := false
   for _, te := range timelineEvents {
       if te.Type == string(state.EvtWorkLogged) {
           hasWork = true
           break
       }
   }
   ```
   Poblar `proj.HasWork = hasWork`; si `!hasWork`, poblar
   `proj.TemplateSummary = buildTemplateSummary(spec.ID, spec.Title, timelineEvents)`.
   El orden de construcción de `proj` ya existe — insertar estos dos campos.

3. Agregar la función pura `buildTemplateSummary` (ver algoritmo en §5). Sin I/O, sin error
   de retorno, sin acceso al store. Ubicarla junto a `hasWorkLoggedAfter` al final del archivo.

Restricciones:
- No cambiar la signature de `runSpecSummarize` ni de `runSpecSummarizeCommit`.
- No cambiar el comportamiento del `commit` step ni el safeguard `hasWorkLoggedAfter`.
- No agregar dependencias externas.
- No cambiar `hasWorkLoggedAfter` (opera sobre `[]state.Event` en nivel de store; es diferente
  de `hasWork` que opera sobre `[]standup.TimelineEvent` ya proyectado).
- Mantener `--json` como único modo que devuelve el JSON; la salida humana (no-`--json`) no
  cambia.

#### `cli/cmd/vector/summarize_test.go` — MODIFICAR

Agregar estos tests (tabla-driven donde aplique):

1. `TestSummarizeProjectionHasWorkFalseWhenNoWorkLogged`: crear spec, appendar solo un
   `status.changed` (sin `work.logged`), correr `runSpecSummarize` con `--json`, parsear
   `summarizeProjection` → `hasWork == false`, `templateSummary != ""`.

2. `TestSummarizeProjectionHasWorkTrueWhenWorkLogged`: crear spec, llamar `store.WorkLog`,
   correr `--json` → `hasWork == true`, `templateSummary == ""`.

3. `TestBuildTemplateSummary`: tabla-driven con casos:
   - Evento `spec.proposed` → `"… proposed (draft → open)"`.
   - Evento `spec.closed` → `"… closed"`.
   - Evento `spec.archived` → `"… archived"`.
   - Evento `status.changed` (from=`in-progress`, to=`review`) → `"…: moved from in-progress to review"`.
   - Sin eventos → fallback `"spec \"…\": no recent activity"`.
   - Con `title` vacío → usa `id` como label.

Restricciones:
- Reusar `captureStdout` ya definido en el mismo archivo de test.
- No cambiar los tests existentes.
- Usar `t.TempDir()` para aislamiento.

#### `kit/commands/vector/archive.md` — MODIFICAR

Cambio en **§3 "Summarize what was done (post-action)"**:

Reemplazar el bloque de 3 pasos actuales por lógica condicional basada en `hasWork`:

```
## 3. Summarize what was done (post-action)

Generate the per-spec "what was done" summary the board's details drawer shows. The binary
projects and persists; a cheap **Haiku** agent writes the prose only when there is substantive
work to describe. **You never write the summary yourself.**

1. `vector spec summarize <id> --json` → `{ id, title, status, ticket?, priorSummary?,
   hasWork, templateSummary?, events[] }`.

2. **If `hasWork == false`** (no `work.logged` events in the window — a structural
   transition): skip the Haiku agent. The binary already produced `templateSummary`. Pipe it
   directly:
   ```bash
   echo '{"summary":"<templateSummary>"}' | vector spec summarize <id> commit \
     --action archive --summary-file -
   ```
   The binary's existing safeguard preserves the richer prior summary when present
   (no new work → prior is kept regardless of what you pipe).

3. **If `hasWork == true`** (substantive work in the window): pass the **exact JSON** from
   step 1 to the `vector-summary-writer` subagent (Haiku); it returns
   `{ "summary": "<2–3 sentences>" }`. Pipe its JSON to:
   ```bash
   vector spec summarize <id> commit --action archive --summary-file -
   ```
   Empty/invalid prose → nothing is written (not a gate); note it and move on.
```

Restricciones:
- No cambiar las secciones §1, §2 ni §4 del command.
- No cambiar el comportamiento del binario descrito (el safeguard existe y se documenta).

#### `kit/commands/vector/close.md` — MODIFICAR

Mismo cambio que archive.md en su §3, con `--action close` en lugar de `--action archive`.
La descripción del safeguard es idéntica.

#### `kit/commands/vector/status.md` — MODIFICAR

Mismo cambio que archive.md en su §3, con `--action status`. Diferencia notable: para
`status`, el safeguard de close/archive NO aplica — si `hasWork == false`, el template
`templateSummary` ES lo que se persiste (no hay preservación automática). El template es
siempre apropiado para una transición sin trabajo (p. ej. `"<title>: moved from in-progress to review"`).

Restricciones: no cambiar §1, §2 ni §4.

#### `kit/commands/vector/propose.md` — MODIFICAR

Mismo cambio en su **§7** (paso de summarize post-acción), con `--action propose`. `propose`
nunca registra `work.logged` (los artefactos del change se generan pero no se logean como
trabajo en `activity.jsonl` — el evento registrado es `spec.proposed` + `status.changed`).
Por tanto `hasWork` siempre será `false` para propose. El template será del tipo
`"<title> proposed (draft → open)"`.

Restricciones: no cambiar §1–§6 ni §8 de propose.md.

#### `cli/internal/scaffold/assets/commands/vector/{archive,close,status,propose}.md` — MODIFICAR

Copias byte-a-byte de sus correspondientes en `kit/commands/vector/`. Sin modificaciones
adicionales. Seguir el patrón de los archivos sibling ya existentes en `assets/`.

---

## 7. API Contract

Sin superficie HTTP nueva. La única interfaz que cambia es la **CLI del binario**:

**Output extendido de `vector spec summarize <id> --json` (éxito, sin work.logged):**
```json
{
  "id": "add-foo",
  "title": "Add foo",
  "status": "closed",
  "ticket": null,
  "priorSummary": "Wired the foo subsystem end to end; tests green.",
  "hasWork": false,
  "templateSummary": "Add foo closed",
  "events": [
    { "ts": "...", "type": "status.changed", "from": "review", "to": "closed", "trigger": "command" }
  ]
}
```

**Output extendido cuando hay work.logged (sin cambio de comportamiento):**
```json
{
  "id": "add-foo",
  ...
  "hasWork": true,
  "events": [
    { "ts": "...", "type": "work.logged", "filesTouched": ["a.go"], "note": "did work" },
    { "ts": "...", "type": "status.changed", "from": "in-progress", "to": "review", "trigger": "command" }
  ]
}
```
`templateSummary` ausente del JSON cuando `hasWork == true` (campo vacío con `omitempty`).

El subcomando `vector spec summarize <id> commit` no cambia: su signature y comportamiento
son los mismos. El command continúa siendo el que decide qué prose pipear.

---

## 8. Criterios de éxito

- [x] `vector spec summarize <id> --json` incluye `hasWork: false` cuando el timeline no tiene
  eventos `work.logged`; `hasWork: true` cuando los tiene.
- [x] `templateSummary` presente en el JSON cuando `hasWork == false`; ausente cuando `true`.
- [x] `buildTemplateSummary` genera la oración correcta para cada tipo de evento
  (`spec.proposed`, `spec.closed`, `spec.archived`, `status.changed`, vacío).
- [x] Los commands archive, close, status, propose NO spawnan `vector-summary-writer` cuando
  `hasWork == false`; pipean el `templateSummary` directamente a `commit`.
- [x] Cuando `hasWork == true`, archive/close/status/propose siguen el flujo existente sin
  cambios.
- [x] `/vector:apply` sigue su flujo actual sin cambios (nunca toca esta ruta).
- [x] El safeguard `hasWorkLoggedAfter` en `commit --action close/archive` sigue funcionando
  cuando se le pasa el template (preserva el prior cuando corresponde).
- [x] Sin regresiones: los tests existentes en `summarize_test.go` siguen verdes.
- [x] Nuevos tests de `hasWork` y `buildTemplateSummary` pasan.

### Tests requeridos

- [x] `TestSummarizeProjectionHasWorkFalseWhenNoWorkLogged` — `hasWork == false` sin work.logged.
- [x] `TestSummarizeProjectionHasWorkTrueWhenWorkLogged` — `hasWork == true` con work.logged.
- [x] `TestBuildTemplateSummary` — tabla-driven con los cinco casos del algoritmo.
- [x] Tests existentes: siguen pasando sin modificación.

### Comandos de verificación

```bash
cd cli && gofmt -l . && go vet ./... && go test ./cmd/vector/... ./internal/state/... ./internal/standup/...
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

Aplica al **command** y al **output del binario** (no hay UI web involucrada).

### Transparencia del camino tomado

- El command debe loggear o reportar cuál camino tomó al resumir (template vs Haiku), de modo
  que el dev entienda que no hubo spawn. Forma sugerida (en el paso de reporte §4/§8 del
  command): `"summary: template (no work logged)"` vs `"summary: generated (Haiku)"`. No es
  una gate; es informativo.
- No interrumpir el flujo del command si el commit del template falla — tratarlo igual que el
  camino largo (not a gate, anotar y continuar).

### Consistencia con el camino largo

- La calidad de la oración template debe ser comparable a la del agente para el mismo input:
  precisa, basada en eventos, en tiempo presente o pasado simple. No genérica hasta el punto de
  ser confusa (p. ej. "moved from review to closed" es perfectamente descriptiva).
- El board no distingue visualmente si el summary vino del agente o del template — mismo campo,
  misma presentación.

### Errores

- Si `templateSummary` llega vacío al command (bug en el binario), el command **no pipa** una
  cadena vacía a `commit` (commit rechazaría un summary vacío de todas formas). Comportamiento:
  anotar "no templateSummary received, skipping summary" y continuar sin summary.
- Si el `commit` falla (I/O, spec no encontrado), no es gate; el command reporta y continúa.

### Accesibilidad

- Salida legible en texto plano; `--json` no cambia el contrato del binario.

---

## 10. Decisiones tomadas

- **`hasWork` en la projection, no en un subcomando separado.** Evita un round-trip extra.
  El command ya ejecuta `--json` como paso 1; agregar los campos al output existente es gratis.
- **`templateSummary` generado en el binario, no en el command.** El binario tiene los eventos
  ya proyectados y el contexto del spec (id, title); el command no tiene acceso a esos datos
  sin serialización extra. El binario es la fuente de lógica determinista
  (`product/token-routing.md`).
- **`buildTemplateSummary` es una función pura sin error de retorno.** Siempre produce un
  string válido (tiene fallback). El command no maneja error aquí.
- **`HasWork bool` (siempre presente) + `TemplateSummary string` (omitempty).** El command
  puede distinguir `hasWork: false` sin necesidad de null-checks adicionales; `templateSummary`
  ausente cuando no aplica para no inflar el JSON del camino largo.
- **El camino largo (hasWork == true) no se toca.** Paridad total con el comportamiento
  actual: si hay trabajo, el agente corre igual. La optimización es solo para el camino sin
  trabajo.
- **`/vector:apply` no se toca.** Siempre llama `worklog` antes de summarizar; su
  `hasWork` será siempre `true`. Excluirlo del alcance reduce complejidad y riesgo.
- **El safeguard close/archive en `commit` no se modifica.** Sigue siendo el guard de
  segundo nivel. Para close/archive sin prior summary, el template se persiste directamente
  (comportamiento correcto). Para close/archive con prior summary y sin work.logged, el
  safeguard descarta el template y preserva el prior (igual que con un LLM-generated summary
  degradado). Doble protección sin duplicar lógica.
- **La función scan `hasWork` opera sobre `[]standup.TimelineEvent`, no sobre `[]state.Event`.**
  El timeline ya fue filtrado por ventana temporal en `runSpecSummarize`; operar sobre él evita
  re-leer el log de events. `hasWorkLoggedAfter` en `commit` opera sobre `[]state.Event` sin
  filtro de ventana — son funciones distintas con propósitos distintos; no fusionarlas.

Si el agente detecta una alternativa mejor, la reporta como observación, pero no la implementa.

---

## 11. Edge cases

### `templateSummary` vacío en el binario (bug defensivo)

Si por algún motivo `buildTemplateSummary` retorna string vacío (no debería dado el fallback),
el command detecta `templateSummary == ""` y **no pipa** a commit. Comportamiento: anotar
que no hay template, no crashear.

### `hasWork == false` pero la ventana de 24h no captura todos los eventos relevantes

La ventana de summarize es `24h` (`summarizeWindow`). Si un evento `work.logged` ocurrió hace
más de 24h (p. ej. en una sesión anterior), la ventana no lo incluye y `hasWork == false`.
Esto es correcto por diseño: el summary post-acción describe lo que pasó en la sesión
reciente, no el historial completo (ese rol lo cumple `priorSummary`). No es un edge case a
corregir; es el comportamiento esperado. El agente no debe extender la ventana.

### Spec sin título (`title == ""`)

`buildTemplateSummary` usa `id` como label si `title` está vacío. No hay error.

### Ventana sin eventos (spec sin actividad reciente)

`buildTemplateSummary` llega con slice vacío → retorna el fallback
`"spec "<id>": no recent activity"`. El commit lo persiste. No es ideal, pero es raro
(¿cómo llegó el command al paso de summarize sin haber ejecutado la transición?). En la
práctica no ocurre, pero el fallback es seguro.

### Eventos con `Type` desconocido en el timeline

`buildTemplateSummary` los ignora (no matchean ninguno de los casos conocidos). No pánico, no
error. El fallback al `status.changed` o al string de no-actividad es suficiente.

### `hasWork == false` para `propose` con modo `delegate` (OpenSpec tooling)

Cuando el mode es `delegate`, el OpenSpec tooling corre y genera artefactos. Eso no emite
`work.logged` — el evento es `spec.proposed` + `status.changed`. Por tanto `hasWork == false`
siempre para propose. El template `"<title> proposed (draft → open)"` es correcto; describe
la transición que ocurrió.

### Concurrencia en summarize

El `WriteSummary` en `store.go` está serializado por el mutex. El commit del template sigue
el mismo path que el commit del LLM-summary — sin cambios en la serialización.

### Binario ausente

Si `vector` no se encuentra, el command ya reporta el error antes de llegar al paso de
summarize. No es un edge case nuevo.

### Sin HTTP surface

No aplica.

---

## 12. Estados de UI requeridos

No hay componentes UI web nuevos. Los estados de salida relevantes son los del **command**:

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | command esperando ejecutar | invocar el command con id |
| transition done | binario retorna la nueva card + events | continúa al paso de summarize |
| summarize-check | `--json` ejecutado; se inspecciona `hasWork` | esperar |
| template-path | template pipeado a commit; "summary: template (no work logged)" | ninguna acción requerida |
| llm-path | Haiku spawneado; "summary: generated (Haiku)" | esperar al agente |
| summary-committed | commit exitoso | continúa al reporte |
| summary-skipped | `templateSummary` vacío o commit falló; se anota y continúa | ninguna acción requerida |
| report | transición reportada al usuario | siguiente paso (p. ej. `/vector:archive`) |
| error | spec not found / illegal transition / I/O | corregir y reintentar |
| disabled | No aplica — no hay componentes UI interactivos | — |
| offline | No aplica — CLI local-only | — |

---

## 13. Validaciones

### Validaciones en el binario (projection)

| Campo | Regla | Comportamiento |
|---|---|---|
| `<id>` | kebab-case, spec existe | error accionable si no |
| `events` | slice puede estar vacío | `hasWork = false`, fallback template |
| `work.logged` en events | presencia determina `hasWork` | determinista, sin threshold |

### Validaciones en el command

| Campo | Regla | Comportamiento |
|---|---|---|
| JSON de `--json` | parseable, contiene `hasWork` | si no parseable → anotar error, intentar camino largo |
| `templateSummary` | no vacío antes de pipear | si vacío → skip sin error |
| Prose del agente (camino largo) | non-empty string válido (`agentSummary`) | if empty/invalid → not a gate; anotar y continuar |

### Validaciones del binario en `commit`

Sin cambios respecto al comportamiento actual: `--action` requerido, `--summary-file` requerido,
summary no vacío, spec existente. El safeguard close/archive ya está implementado.

---

## 14. Seguridad y permisos

- No se introducen nuevas superficies de escritura. `buildTemplateSummary` es read-only.
- `templateSummary` es una cadena generada por el binario desde los propios eventos del spec
  del usuario — no contiene datos sensibles.
- No se exponen paths internos ni contenido del spec doc en el template.
- El commit del template sigue el mismo path que el commit del LLM-summary (misma
  `writeFileAtomic` vía `WriteSummary`).
- No se introducen dependencias externas.

---

## 15. Observabilidad y logging

Los eventos existentes en `activity.jsonl` no cambian — la optimización no reduce visibilidad
del dominio. Los únicos eventos de la transición son los que ya logea el binario
(`spec.closed`, `status.changed`, etc.).

No hay nuevo evento para "summary templado" — el costo de tokens ahorrado no se loggea como
`agent.routed` (no hubo ruteo a agente; fue un no-spawn). Si se quiere trazar el ahorro,
podría añadirse un `agent.routed` con `model: "binary-template"` y `savedUSD` estimado, pero
esto queda fuera de scope de esta fase (TBD — ver Open questions §20).

El command reporta qué camino tomó en el texto de reporte (template vs Haiku) — suficiente
para auditoría manual.

---

## 16. i18n / textos visibles

**El proyecto no tiene sistema de i18n.** El binario emite strings hardcodeados en inglés
(consistente con el resto del CLI). Las plantillas de `buildTemplateSummary` son strings en
inglés. El command conversa en el idioma del usuario; el texto del summary es un artefacto
técnico del board (no parte de la conversación). Tabla de identificadores de documentación:

| Identificador (doc) | Texto hardcodeado (EN) |
|---|---|
| `template.proposed` | `"<title> proposed (draft → open)"` |
| `template.closed` | `"<title> closed"` |
| `template.archived` | `"<title> archived"` |
| `template.status_changed` | `"<title>: moved from <from> to <to>"` |
| `template.no_activity` | `"spec \"<id>\": no recent activity"` |

Donde `<title>` es `spec.Title` si no vacío, si no `spec.ID`.

---

## 17. Performance

- `buildTemplateSummary` es O(n) sobre `[]standup.TimelineEvent` (típicamente 1–5 eventos en
  una ventana de 24h). Sin I/O, sin allocaciones significativas.
- La ganancia neta es la eliminación del spawn del agente Haiku + 2 round-trips en ~4 comandos
  (`archive`, `close`, `status`, `propose`) cuando la ventana no contiene `work.logged`.
  Estimado: ~500–800ms por transición estructural evitada.
- El paso `--json` (proyección) sigue corriendo — es un I/O local mínimo (<50ms).
- Sin cambios en el path caliente de `apply` (el más frecuente con trabajo real).

---

## 18. Restricciones

El agente no debe:

- Cambiar `runSpecSummarizeCommit` ni el safeguard `hasWorkLoggedAfter`.
- Modificar `/vector:apply` (no tiene el patrón de camino corto).
- Modificar `/vector:link` (no tiene paso de summarize).
- Agregar imports externos al módulo Go.
- Cambiar el schema de `SpecState`, `Event`, `ArtifactSet`, `SpecSummary` o cualquier tipo del
  paquete `state`.
- Cambiar la signature de `runSpecSummarize` ni de `standup.Timeline`.
- Modificar el agente `vector-summary-writer.md` (su prompt no cambia).
- Renombrar ni eliminar los tests existentes en `summarize_test.go`.
- Extender la ventana de `summarizeWindow` (se mantiene en `"24h"`).
- Crear carpetas nuevas ni nuevos archivos Go fuera de los listados en §6.
- Hardcodear la decisión hasWork/LLM en el binario: la decisión sigue siendo del command
  (el binario provee la señal, el command decide).

---

## 19. Entregables

Al finalizar, deben quedar:

- [x] `summarizeProjection` extendido con `HasWork bool` + `TemplateSummary string` (omitempty).
- [x] `buildTemplateSummary` implementado y cubierto por tests.
- [x] `runSpecSummarize` poblando los dos campos nuevos.
- [x] Tests nuevos: `TestSummarizeProjectionHasWorkFalseWhenNoWorkLogged`,
  `TestSummarizeProjectionHasWorkTrueWhenWorkLogged`, `TestBuildTemplateSummary` (tabla-driven).
- [x] `kit/commands/vector/archive.md`, `close.md`, `status.md`, `propose.md` actualizados con
  la lógica condicional.
- [x] Las cuatro copias vendorizadas en `cli/internal/scaffold/assets/commands/vector/`
  actualizadas e idénticas a sus sources en `kit/`.
- [x] Gate verde: `gofmt -l . && go vet ./... && go test ./cmd/vector/... ./internal/state/...
  ./internal/standup/...` en `cli/`.
- [x] Sin regresiones en tests existentes.

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `cli/cmd/vector/summarize.go` — entiendo `summarizeProjection`, `runSpecSummarize`,
  `runSpecSummarizeCommit`, `hasWorkLoggedAfter`.
- [ ] Revisé `cli/internal/standup/standup.go` — entiendo `TimelineEvent` y `Timeline`.
- [ ] Revisé `cli/internal/state/event.go` — entiendo los `EventType` disponibles
  (`EvtWorkLogged`, `EvtSpecProposed`, `EvtSpecClosed`, `EvtSpecArchived`, `EvtStatusChanged`).
- [ ] Revisé `cli/cmd/vector/summarize_test.go` — entiendo los patterns de tests existentes
  (`captureStdout`, `writeTempSummary`, `t.TempDir()`).
- [ ] Revisé `kit/commands/vector/{archive,close,status,propose}.md` — entiendo el 3-step
  summarize actual en cada uno.
- [ ] Solo modifiqué los archivos listados en §6.
- [ ] `buildTemplateSummary` es función pura: sin I/O, sin error de retorno, con fallback.
- [ ] `HasWork` siempre presente en el JSON (sin `omitempty`); `TemplateSummary` con
  `omitempty`.
- [ ] El camino largo (`hasWork == true`) no fue modificado.
- [ ] `/vector:apply` no fue tocado.
- [ ] Los commands actualizados conservan sus §1/§2/§4 (archive, close, status) y §1–§6/§8
  (propose) sin cambios.
- [ ] Las cuatro copias vendorizadas son byte-idénticas a sus fuentes en `kit/`.
- [ ] Ejecuté `gofmt -l .` → sin output.
- [ ] Ejecuté `go vet ./...` → sin warnings.
- [ ] Ejecuté `go test ./...` → todos los tests verdes.
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar.

## Open questions

- **Telemetría del ahorro**: ¿Debería el command emitir un `agent.routed` con
  `model: "binary-template"` + `savedUSD` estimado para el Token Savings Meter cada vez que
  toma el camino corto? Esto haría visible el ahorro en el board. TBD — ver §15 y la
  arquitectura del meter en `internal/state/pricing.go`. No bloqueante para esta fase.
- **`propose` con `delegate` mode**: en modo delegado, ¿deberían los artefactos generados por
  OpenSpec tooling contarse como `work.logged`? Actualmente no se logean. Si en el futuro
  `/vector:propose` registra un `work.logged` para el trabajo del tooling, `hasWork` cambiará
  a `true` y el agente correrá. Esta spec no define ese comportamiento futuro. TBD al diseñar
  el logging del modo delegate.
