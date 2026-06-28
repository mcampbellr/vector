# Spec: Token Meter Accuracy — provenance de eventos y etiqueta "estimado" en el board

## 1. Objetivo

Construir la capa de **provenance de datos** del Token Savings Meter: un campo `precision`
(`"actual"` | `"estimated"`) en cada evento `agent.routed`, propagado al rollup del board
(`TokenSavings.Precision`) y expuesto en la UI web como un badge sutil **cuando el meter
contiene al menos un evento estimado**.

Esta feature permite que el board muestre la métrica de ahorro de tokens de forma **honesta**:
cuando los conteos provienen de la señal real del harness de Claude Code, el meter se presenta
sin calificador; cuando los conteos son estimaciones auto-reportadas por el command
orquestador (el caso actual), el meter lleva una etiqueta explícita `Estimated` que mantiene
la credibilidad del wedge comercial sin sobre-prometer.

## 2. Alcance

### Incluido en esta fase

- Campo `Precision string` (`"actual"` | `"estimated"`) en `AgentRoutedData`
  (`cli/internal/state/event.go`), con default `"estimated"` al deserializar eventos
  anteriores sin el campo (retrocompatibilidad).
- Flag `--precision actual|estimated` en `vector spec route` (`cli/cmd/vector/route.go`), con
  default `"estimated"` cuando se omite (preserva el comportamiento actual de los commands
  del kit sin cambiarlos).
- Campo `Precision string` en `TokenSavings` (`cli/internal/board/board.go`): el rollup
  toma el peor caso — `"estimated"` si **cualquier** evento del conjunto tiene
  `precision != "actual"`; `"actual"` solo si todos son exactos.
- Modificación de los kit commands `/vector:raw` y `/vector:bug`
  (`kit/commands/vector/raw.md` y `kit/commands/vector/bug.md`) para documentar cuándo pasar
  `--precision actual` (señal real del harness) vs. omitirlo (estimación — comportamiento por
  defecto preservado).
- Badge `Estimated` en el Token Savings Meter del panel web
  (`web/src/components/KanbanBoard/TokenSavingsMeter.tsx` o equivalente — TBD exacto de ruta,
  ver Open questions) cuando `tokenSavings.precision === "estimated"`.
- Tests de `rollupSavings` en `cli/internal/board/board_test.go`: mezcla
  actual+estimated → resultado `"estimated"`; todos `"actual"` → `"actual"`; rollup vacío →
  `""` (sin badge).
- Actualización de `docs/domain-contract.md` §3 (formato de `agent.routed`) para registrar el
  nuevo campo y su semántica.

### Fuera de scope

- **Captura automática de tokens desde el harness de Claude Code**: la disponibilidad de la
  señal real en el contexto del command orquestador es TBD (ver Open questions §1). Esta fase
  *prepara la infraestructura* para recibirla (`--precision actual`) pero no implementa la
  captura.
- Modificar la fórmula de cálculo de `CostUSD`/`SavedUSD` ni la tabla de precios en
  `pricing.go`.
- Añadir un tooltip o pantalla de detalle que desglose evento a evento la provenance (fase
  futura).
- Cambiar el shape del endpoint `/api/board`; solo se agrega `precision` al objeto
  `tokenSavings` ya existente (aditivo, sin breaking change).
- Migración de `activity.jsonl` existente: los eventos sin campo se leen como `"estimated"`
  en runtime (no se reescriben en disco).
- Exportar o mostrar el campo `precision` por evento individual en el board (solo en el
  rollup global y por-spec badge de `SavedUSD`).

El agente no debe implementar nada fuera de este alcance aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Binario: **Go** (módulo único, stdlib, `cli/`). Sin dependencias externas.
- Panel web: **React + TypeScript** (`web/`). Stack exacto TBD — ver Open questions §2.
- Kit commands: **Markdown** con instrucciones de orquestación (`kit/commands/vector/`).

### Versiones relevantes

- Go: `1.26` (citado de `cli/go.mod`).
- EventVersion: `1` (sin cambio de schema version — el nuevo campo es aditivo y se deserializa
  como cadena vacía en eventos antiguos, que se tratan como `"estimated"`).
- SchemaVersion del board: `1` (sin cambio — `precision` es aditivo en `TokenSavings`).

### Patrones existentes a respetar

- **CLI-owns-writes**: el binario es el único escritor de `activity.jsonl`; el command pasa
  el flag, el binario persiste el evento. La lógica de negocio (default de `precision`) vive
  en `RouteAgent`, no en el command.
- **Rollup read-only**: `board.Build` proyecta; `rollupSavings` agrega. No escribir estado
  en el board package.
- **Retrocompatibilidad sin migración**: campos nuevos en structs de log son aditivos. La
  deserialización de JSON ignora campos ausentes → campo vacío (`""`) → tratado como
  `"estimated"` por el rollup. No reescribir el log.
- **Peor-caso para el rollup**: si hay un solo evento estimado, el meter completo es
  `"estimated"`. Es la política más conservadora y la más honesta.
- **Errores explícitos**: `RouteAgent` falla si `precision` no es `""`, `"actual"` o
  `"estimated"` (validación estricta; `""` se normaliza a `"estimated"` dentro de la función).
- **Naming**: campo `precision` (no `source`, no `dataQuality`); badge UI `Estimated` (no
  `Approximate`, no `~`). El vocabulario debe ser auto-explicativo en el board.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `cli/internal/state/event.go` con `AgentRoutedData` y `EvtAgentRouted` (existente).
- [x] `cli/cmd/vector/route.go` con `runSpecRoute` + `Store.RouteAgent` (existente).
- [x] `cli/internal/board/board.go` con `TokenSavings` y `rollupSavings` (existente).
- [x] `cli/internal/board/board_test.go` con tests de `rollupSavings` (existente).
- [x] Panel web con el componente que renderiza `tokenSavings` del board JSON (existente;
  ruta exacta TBD — ver Open questions §2).
- [ ] Identificar si el harness de Claude Code expone conteos reales al orquestador, y bajo
  qué variable de entorno o estructura — TBD (Open questions §1). Esta fase no requiere la
  señal real; solo prepara la recepción.

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta.

---

## 5. Arquitectura

### Patrón

**Aditivo y retrocompatible**: añadir un campo al evento, propagar por la cadena existente
(evento → rollup → API → UI), y hacer que el board sea honesto sin cambiar el contrato de
ningún consumidor actual (los campos nuevos son opcionales; `precision` ausente en el JSON
serializado de eventos anteriores se trata como `"estimated"`).

### Capas afectadas

- **`cli/internal/state/`** (datos): sí — `AgentRoutedData.Precision` + validación en
  `RouteAgent` + default en rollup.
- **`cli/cmd/vector/route.go`** (CLI): sí — `--precision` flag con default `"estimated"`.
- **`cli/internal/board/board.go`** (proyección): sí — `TokenSavings.Precision` + lógica
  de peor-caso en `rollupSavings`.
- **`kit/commands/vector/raw.md`** y **`kit/commands/vector/bug.md`** (kit): sí — añadir
  guía de cuándo pasar `--precision actual` en el paso "Record the token routing".
- **`web/`** (UI): sí — badge `Estimated` cuando `tokenSavings.precision === "estimated"`.
- **`docs/domain-contract.md`** (documentación): sí — registrar el campo.

### Flujo esperado

1. Un command del kit (`/vector:raw`, `/vector:bug`) ejecuta un subagente y obtiene (o
   estima) el conteo de tokens usados.
2. El command llama `vector spec route <id> --model haiku --baseline opus --task "..." \
   --tokens-in N --tokens-out M [--precision actual]`. Si omite `--precision`, el binario
   usa `"estimated"` por defecto.
3. `runSpecRoute` pasa `--precision` a `Store.RouteAgent`.
4. `RouteAgent` normaliza el valor (`""` → `"estimated"`); valida que sea `"actual"` o
   `"estimated"`; construye `AgentRoutedData` con el campo `Precision`; appenda el evento.
5. `board.Build` llama a `rollupSavings(events)`, que ya itera `EvtAgentRouted`. La lógica
   de peor-caso: si algún evento decodificado tiene `Precision != "actual"` (incluyendo
   `""` de eventos antiguos), el rollup marca `TokenSavings.Precision = "estimated"`.
6. El servidor expone `GET /api/board` con el campo `tokenSavings.precision` en el JSON.
7. El componente web del Token Savings Meter lee `tokenSavings.precision`; si es
   `"estimated"`, muestra el badge `Estimated` junto al valor monetario de ahorro.
8. Si todos los eventos son `"actual"`, `Precision = "actual"` y no aparece el badge.

### Ubicación de archivos nuevos

No se crean archivos nuevos. Todos los cambios son modificaciones de archivos existentes.

```txt
cli/
  internal/
    state/
      event.go        ← MODIFICAR: AgentRoutedData.Precision
      standup.go      ← MODIFICAR: RouteAgent valida + normaliza precision
    board/
      board.go        ← MODIFICAR: TokenSavings.Precision + rollup peor-caso
      board_test.go   ← MODIFICAR: tests de precision en rollup
  cmd/vector/
    route.go          ← MODIFICAR: --precision flag
kit/commands/vector/
  raw.md              ← MODIFICAR: guía en step "Record token routing"
  bug.md              ← MODIFICAR: guía en step "Record token routing"
web/src/components/
  KanbanBoard/        ← MODIFICAR: badge en TokenSavingsMeter (ruta TBD)
docs/
  domain-contract.md  ← MODIFICAR: registrar campo precision en agent.routed
```

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `cli/internal/state/event.go` | MODIFICAR | `AgentRoutedData.Precision string` (aditivo) | `WorkLoggedData`, `AppliedData` |
| `cli/internal/state/standup.go` | MODIFICAR | `RouteAgent`: validar + normalizar `precision`; pasarlo a `AgentRoutedData` | `RouteAgent` existente |
| `cli/internal/board/board.go` | MODIFICAR | `TokenSavings.Precision string`; `rollupSavings` peor-caso | `rollupSavings` existente |
| `cli/internal/board/board_test.go` | MODIFICAR | Tests de `rollupSavings` con `precision` | Tests existentes de `rollupSavings` |
| `cli/cmd/vector/route.go` | MODIFICAR | Flag `--precision actual\|estimated` en `runSpecRoute` | flags existentes de `runSpecRoute` |
| `kit/commands/vector/raw.md` | MODIFICAR | Guía `--precision actual` en step 10 | step 10 actual de `raw.md` |
| `kit/commands/vector/bug.md` | MODIFICAR | Guía `--precision actual` en step 10 | step 10 actual de `bug.md` |
| `cli/internal/scaffold/assets/commands/vector/raw.md` | MODIFICAR | Copia vendored de `kit/commands/vector/raw.md` (vía `go generate`) | sibling `bug.md` |
| `cli/internal/scaffold/assets/commands/vector/bug.md` | MODIFICAR | Copia vendored de `kit/commands/vector/bug.md` (vía `go generate`) | sibling `raw.md` |
| Web: componente Token Savings Meter | MODIFICAR | Badge `Estimated` condicional | TBD — ver Open questions §2 |
| `docs/domain-contract.md` | MODIFICAR | Registrar `precision` en el shape de `agent.routed` | Sección existente de eventos |

### Detalle por archivo

#### `cli/internal/state/event.go` — MODIFICAR

Cambios requeridos:

- Añadir `Precision string \`json:"precision,omitempty"\`` a `AgentRoutedData`. El tag
  `omitempty` asegura que los eventos con `"estimated"` (el caso típico actual) no crezcan
  en tamaño: solo `"actual"` se serializa explícitamente; ausente → tratado como `"estimated"`
  al deserializar. **Alternativa**: serializar siempre `precision` (sin `omitempty`). Ver
  Open questions §3 — elegir antes de implementar.
- Añadir comentario al campo: `// "actual" = token counts from the harness; "estimated" = self-reported by the orchestrating command (default).`

Restricciones:
- No cambiar `EventVersion` (el campo es aditivo; no es una migración de schema).
- No reordenar ni renombrar campos existentes.

#### `cli/internal/state/standup.go` — MODIFICAR

Cambios en `RouteAgent`:

- Recibir el parámetro `precision string` tras `tokensOut int`.
- Normalizar: si `precision == ""`, usar `"estimated"`.
- Validar: si `precision != "actual"` && `precision != "estimated"`, retornar error
  `"invalid precision %q: must be actual or estimated"`.
- Asignar `data.Precision = precision` (ya normalizado).

Restricciones:
- No cambiar la firma del método en cuanto al tipo de retorno (`AgentRoutedData, error`).
- Mantener la validación de modelos y token counts que ya existe.
- La lógica de pricing (`CostUSD`, `SavedUSD`) no cambia.
- La función `RouteAgent` tiene la firma:
  `RouteAgent(specID, task, model, baseline string, tokensIn, tokensOut int, precision string, actor string, now time.Time)`
  — el parámetro `precision` va entre `tokensOut` y `actor` para agrupar con los conteos.

#### `cli/internal/board/board.go` — MODIFICAR

Cambios en `TokenSavings`:

- Añadir `Precision string \`json:"precision,omitempty"\`` al struct. Semántica:
  `"actual"` si todos los eventos rollup son exactos; `"estimated"` si alguno no es exacto
  (incluyendo eventos deserializados con `Precision == ""`).

Cambios en `rollupSavings`:

- En el loop de eventos `EvtAgentRouted`, después de deserializar `AgentRoutedData`:
  - Si `s.Precision != "actual"` (i.e. campo vacío de eventos viejos o `"estimated"`),
    marcar una variable local `hasEstimated = true`.
- Al finalizar el loop: si `s.Routes > 0 && !hasEstimated`, `s.Precision = "actual"`;
  si `s.Routes > 0 && hasEstimated`, `s.Precision = "estimated"`;
  si `s.Routes == 0`, `s.Precision = ""` (sin badge — meter vacío).

Restricciones:
- No modificar `Build`, `toCard`, ni el ordenamiento de columnas.
- `perSpec` (el `specEconomics` por spec) no lleva `precision` individual — solo el rollup
  global la expone. El per-spec `SavedUSD` en `Card` no cambia.

#### `cli/internal/board/board_test.go` — MODIFICAR

Tests requeridos (añadir a los existentes):

- `TestRollupSavings_AllActual`: todos los eventos con `Precision="actual"` → `TokenSavings.Precision = "actual"`.
- `TestRollupSavings_AllEstimated`: todos con `Precision="estimated"` → `"estimated"`.
- `TestRollupSavings_Mixed`: mezcla `"actual"` + `"estimated"` → `"estimated"` (peor caso).
- `TestRollupSavings_OldEvents`: eventos sin campo (`Precision=""`) → `"estimated"` (retrocompatibilidad).
- `TestRollupSavings_Empty`: sin eventos `EvtAgentRouted` → `Precision = ""`.

Restricciones:
- No tocar los tests existentes de `rollupSavings` ni de `Build`.
- Usar el patrón table-driven del proyecto.

#### `cli/cmd/vector/route.go` — MODIFICAR

Cambios en `runSpecRoute`:

- Añadir flag: `precision := fs.String("precision", "", "data quality: actual|estimated (default: estimated)")`
- Pasar `*precision` a `store.RouteAgent(...)` en la nueva posición del parámetro.
- La salida `--json` y la salida human-readable ya incluyen `costUsd`/`savedUsd`; no necesitan
  mostrar `precision` salvo que `--json` lo lleve por completitud. Añadir `"precision"` al
  map del `--json` output.

Restricciones:
- No cambiar los flags existentes (`--model`, `--baseline`, `--task`, `--tokens-in`,
  `--tokens-out`, `--repo-root`, `--json`).
- No modificar el error path cuando `--model` falta.

#### `kit/commands/vector/raw.md` — MODIFICAR

En el paso 10 "Record the token routing", añadir:

- Explicar cuándo pasar `--precision actual`: solo cuando el harness de Claude Code expone
  el conteo real de tokens del subagente (p.ej. accesible en el contexto del command). TBD
  la forma exacta — ver Open questions §1. Mientras tanto, omitir el flag (el binario
  defaultea a `"estimated"`).
- El texto actual dice "Use the actual subagent token usage when you have it; otherwise pass
  your best estimate... (the meter is an estimate by design)". Complementar con: cuando el
  conteo es estimado, **no pasar** `--precision actual`; el default `"estimated"` es el
  comportamiento correcto y honesto.
- No cambiar la estructura ni los otros pasos del command.

#### `kit/commands/vector/bug.md` — MODIFICAR

Idéntico al cambio en `raw.md` paso 10. No cambiar otros pasos.

#### Componente web — MODIFICAR

Ruta TBD (ver Open questions §2). El componente que renderiza el Token Savings Meter:

- Leer `tokenSavings.precision` del board JSON.
- Si `precision === "estimated"`: mostrar el badge `Estimated` junto al total de ahorro (p.ej.
  `$12.34 saved · Estimated`). El badge es textual, tono neutro, no alarmante.
- Si `precision === "actual"` o ausente y no hay ahorro: no mostrar badge.
- No cambiar el layout del meter; el badge es aditivo.

Seguir como referencia: el patrón de status pill existente (colores, tipografía).

#### `docs/domain-contract.md` — MODIFICAR

En la sección que documenta el evento `agent.routed` (shape de `AgentRoutedData`), añadir el
campo `precision`:

```
precision: "actual" | "estimated"
  "actual"    — conteo real del harness de Claude Code
  "estimated" — self-reported por el command orquestador (default; ausente en eventos
                anteriores se trata como "estimated")
```

---

## 7. API Contract

El contrato HTTP relevante es `GET /api/board` (consumido por `web/`). El campo `tokenSavings`
del board ya existe; esta fase agrega `precision` de forma aditiva:

```json
{
  "tokenSavings": {
    "totalSavedUsd": 12.3456,
    "totalSpentUsd": 0.9876,
    "baselineUsd": 13.3332,
    "routes": 14,
    "tokensIn": 42000,
    "tokensOut": 8000,
    "precision": "estimated",
    "byModel": [...]
  }
}
```

Valores posibles de `precision`:
- `"estimated"` — al menos un evento del rollup tiene `precision != "actual"` (incluyendo
  eventos viejos sin el campo). **Caso típico** en V1.
- `"actual"` — todos los eventos tienen `precision == "actual"`. Disponible cuando el harness
  expone conteos reales.
- `""` / ausente — meter vacío (sin eventos `agent.routed`). La UI no muestra badge.

El campo no es breaking: consumidores existentes que no conocen `precision` simplemente lo
ignoran. El web frontend es el único consumidor actual.

La **CLI del binario** (`vector spec route`) se actualiza:

```bash
vector spec route [id] --model haiku --baseline opus \
  --tokens-in N --tokens-out M [--task "..."] \
  [--precision actual|estimated] [--repo-root path] [--json]
```

Salida `--json` (éxito):
```json
{
  "model": "haiku",
  "baseline": "opus",
  "tokensIn": "3000",
  "tokensOut": "800",
  "costUsd": "0.007000",
  "savedUsd": "0.033000",
  "precision": "estimated"
}
```

---

## 8. Criterios de éxito

- [ ] `vector spec route` acepta `--precision actual|estimated`; sin flag → `"estimated"` por
  defecto; flag inválido → error accionable.
- [ ] `AgentRoutedData.Precision` se persiste en `activity.jsonl`; con `omitempty`, los eventos
  estimados son ligeramente más compactos (o siempre serializan el campo — ver Open questions §3).
- [ ] Eventos viejos sin `Precision` (campo vacío al deserializar) producen `rollup.Precision =
  "estimated"` — retrocompatibilidad verificada con test.
- [ ] `TokenSavings.Precision = "actual"` solo cuando todos los eventos son `"actual"`.
- [ ] `TokenSavings.Precision = "estimated"` cuando hay cualquier evento no-`"actual"`.
- [ ] `TokenSavings.Precision = ""` cuando no hay eventos `agent.routed`.
- [ ] El board JSON expone `tokenSavings.precision` correctamente (verificable con `curl /api/board`).
- [ ] El panel web muestra el badge `Estimated` cuando `precision == "estimated"` y lo omite cuando `"actual"`.
- [ ] Los kit commands (`raw.md`, `bug.md`) no rompen su comportamiento actual (sin `--precision`,
  el default `"estimated"` es el resultado correcto; los commands no requieren cambio de
  sus invocaciones existentes de `vector spec route`).
- [ ] Sin regresiones: `rollupSavings` existente, `Build`, tests actuales del board, `runSpecRoute`
  sin el flag siguen funcionando.

### Tests requeridos

- [ ] `TestRollupSavings_AllActual`: todos `"actual"` → `Precision = "actual"`.
- [ ] `TestRollupSavings_AllEstimated`: todos `"estimated"` → `Precision = "estimated"`.
- [ ] `TestRollupSavings_Mixed`: mezcla → `Precision = "estimated"`.
- [ ] `TestRollupSavings_OldEvents`: `Precision = ""` en el evento → `"estimated"` en rollup.
- [ ] `TestRollupSavings_Empty`: sin eventos → `Precision = ""`.
- [ ] `TestRouteAgent_PrecisionDefault`: sin pasar `precision` → `data.Precision = "estimated"`.
- [ ] `TestRouteAgent_PrecisionActual`: `precision = "actual"` → persiste.
- [ ] `TestRouteAgent_PrecisionInvalid`: `precision = "bogus"` → error.

### Comandos de verificación

```bash
gofmt -l cli && go -C cli vet ./... && go -C cli test ./...
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

Aplica al panel web (badge) y al reporte CLI de `vector spec route`.

### Badge web (Token Savings Meter)

- **Diseño**: texto `Estimated` junto al total de ahorro, en tipografía más pequeña, tono
  neutro (slate o gris claro — no rojo ni naranja). No es una advertencia; es una aclaración
  honesta de calidad de dato.
- **Posición**: inline con el valor monetario (`$12.34 saved · Estimated`) o como sub-label
  del meter. No en overlay ni modal.
- **Ausencia de badge**: cuando `precision == "actual"`, el meter muestra solo el valor sin
  calificador. No añadir `Exact` ni `Actual` como label positivo — el estado "sin badge" ya
  implica exactitud.
- **Rollup vacío** (`Precision == ""`): el meter puede mostrar `$0.00 saved` o estar oculto;
  ningún badge en este caso.
- **Accesibilidad**: el badge debe tener `aria-label` o texto accesible; el contraste del
  texto gris sobre blanco debe cumplir WCAG AA.

### CLI (`vector spec route --json`)

- El campo `precision` aparece en la salida `--json` para que el command pueda verificar lo
  que quedó registrado.
- La salida human-readable no necesita mostrar `precision` explícitamente (el `saved $X.XXXX`
  es suficiente para el uso típico del command).

### Errores accionables

- `--precision bogus` → `"invalid precision "bogus": must be actual or estimated"`.
- No hay UX de confirmación para esta feature (no es destructiva).

---

## 10. Decisiones tomadas

- **Default `"estimated"`**, no `"actual"**: el comportamiento actual de los commands es
  auto-reportar estimaciones; invertir el default rompería la honestidad del meter con cero
  cambio de comportamiento en los commands existentes. Los commands no necesitan tocarse para
  mantener su comportamiento actual; `--precision actual` es un opt-in explícito para cuando
  la señal real esté disponible.
- **Peor-caso en el rollup**: si hay un evento estimado, el metro completo es `"estimated"`.
  La razón: un meter mixto (algunos exactos, algunos estimados) no puede presentarse como
  exacto sin engañar al usuario. El wedge comercial requiere integridad, no falsa precisión.
- **`omitempty` en `AgentRoutedData.Precision`**: TBD (ver Open questions §3). La elección
  está bloqueada en esta fase; se documenta como open question para que el implementador elija
  antes de escribir código.
- **Sin migración del log**: los eventos viejos sin campo se leen como `"estimated"` en
  runtime. No hay reescritura del `activity.jsonl`. Razón: el log es append-only (invariante
  del sistema); la retrocompatibilidad se maneja en el rollup.
- **Sin `precision` en `Card.SavedUSD` del board**: el ahorro por-spec en la card del kanban
  no lleva badge. Solo el rollup global lo lleva. Razón: granularidad excesiva sin beneficio
  en V1; simplifica la implementación.
- **Vocabulary**: `precision` (no `source`, `quality`, `dataQuality`). `"actual"` (no `"real"`,
  `"exact"`). `"estimated"` (no `"approximate"`, `"guess"`). Consistente con el dominio de
  métricas de telemetría.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación,
pero no implementarla.

---

## 11. Edge cases

### Datos inválidos

- **`--precision` con valor desconocido**: `RouteAgent` retorna error; `runSpecRoute`
  lo propaga a stderr + exit 1. El evento no se appendea.
- **`--precision ""` (string vacío)**: `RouteAgent` normaliza a `"estimated"` (no error).
- **`--tokens-in 0 --tokens-out 0` con `--precision actual`**: válido — un subagente puede
  usar cero tokens (edge caso; no bloquearlo). El ahorro será `$0.00`.

### Retrocompatibilidad

- **Evento antiguo sin `Precision`**: `json.Unmarshal` deja el campo como `""`. El rollup
  lo trata como `"estimated"` — el meter muestra `Estimated`. Correcto y conservador.
- **Board JSON sin `precision`** (antes de este deploy): el frontend que no conoce el campo
  lo ignora → sin badge (comportamiento seguro). Una vez desplegado, el campo siempre está.
- **`vector spec route` sin `--precision`**: `precision = ""` en `runSpecRoute` → `RouteAgent`
  normaliza a `"estimated"`. Idéntico al comportamiento pre-cambio (el evento se registra sin
  diferencia visible externamente), salvo que ahora `precision = "estimated"` queda en el log.

### Rollup edge cases

- **Todos los eventos son `"actual"`**: `Precision = "actual"` — sin badge. Este es el estado
  objetivo a largo plazo cuando el harness exponga la señal real.
- **Mix de `"actual"` y `"estimated"`**: peor-caso → `"estimated"`. Ver §10.
- **Un solo evento `"estimated"` entre muchos `"actual"`**: aún `"estimated"`. Correcto.
- **Rollup vacío** (sin eventos `EvtAgentRouted`): `Precision = ""`. La UI no muestra badge.
  Ver §12 (estado `empty`).
- **Evento malformado** (JSON inválido): `rollupSavings` ya tiene `continue` en el error de
  `json.Unmarshal`. Comportamiento inalterado — el evento malformado se ignora.

### Sin HTTP surface relevante nueva

Los códigos HTTP (400/401/404/500) aplican a `/api/board`, que es read-only y no cambia su
comportamiento de error. Los cambios son aditivos en la respuesta.

---

## 12. Estados de UI requeridos

El Token Savings Meter del board web tiene estos estados tras esta fase:

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| `actual` | Total de ahorro (`$X.XX saved`), sin badge | Ninguna acción requerida — dato exacto |
| `estimated` | Total de ahorro + badge `Estimated` | Ninguna — informativo |
| `empty` | `$0.00 saved` o meter oculto, sin badge | — |
| `loading` | Skeleton o spinner (comportamiento existente) | Esperar |
| `error` | Mensaje de error de API (comportamiento existente) | Reintentar |

El badge no tiene estado `loading` ni `error` propio — depende del meter que ya los maneja.

---

## 13. Validaciones

### Validaciones de CLI (`runSpecRoute`)

| Parámetro | Regla | Error |
|---|---|---|
| `--precision` | `"actual"`, `"estimated"`, o vacío (`""`) | `"invalid precision %q: must be actual or estimated"` |
| `--precision ""` | normalizar a `"estimated"` (sin error) | — |
| `--model` | ya validado por `LookupModelPrice` | ya existente |
| `--tokens-in / --tokens-out` | `>= 0`, ya validado | ya existente |

### Validaciones en `RouteAgent` (binario)

El binario es la autoridad final. Las validaciones de `--precision` se aplican también en
`RouteAgent` (no solo en `runSpecRoute`), ya que `RouteAgent` es una función pública que
otros consumidores podrían llamar directamente.

### Validaciones de frontend

El frontend no valida `precision`; lo trata como una cadena opaca y compara con `=== "estimated"`.
Un valor desconocido (p.ej. `"actual-ish"` por un error futuro) no muestra badge — comportamiento
seguro y extensible.

---

## 14. Seguridad y permisos

- No se introducen nuevas superficies de API ni nuevos permisos.
- El campo `precision` no contiene información sensible (solo `"actual"` o `"estimated"`).
- No se exponen detalles de pricing internos ni token counts sensibles más allá de lo que ya
  se expone en `AgentRoutedData` y el board JSON.
- El `activity.jsonl` es local (gitignored), no se envía a ningún servidor externo.
- El badge en el board es solo informativo; no hay acción del usuario que lo modifique
  directamente (el flag en el command lo controla).

---

## 15. Observabilidad y logging

El mecanismo de logging existente es `activity.jsonl` (append-only, local). Esta fase:

- Añade `precision` al evento `agent.routed` ya existente. La observabilidad no cambia —
  los consumidores del log (rollup, standup, future analytics) pueden leer el campo.
- No se añaden nuevos tipos de evento.
- No se logea información adicional (no hay nada más a registrar para esta feature).

El rollup del board es la única agregación de esta señal en V1. En el futuro, un analytics
pass podría desglosar eventos `"actual"` vs. `"estimated"` para reportar confianza de la
métrica a lo largo del tiempo — la infraestructura lo permite.

---

## 16. i18n / textos visibles

**El proyecto no tiene sistema de i18n.** El binario emite strings en inglés hardcodeado
(consistente con el resto del CLI). El frontend tampoco tiene sistema de traducción. Los
strings son fijos y en inglés. La siguiente tabla es documentación de los strings, no keys
de ningún archivo de traducción.

| Identificador (doc) | Texto hardcoded |
|---|---|
| `route.precision.invalid` | `invalid precision %q: must be actual or estimated` |
| `board.badge.estimated` | `Estimated` |
| `route.json.precision` | `"precision": "estimated"` (clave en JSON output) |

La conversación con el usuario del command ocurre en el idioma que elija; el badge en el
board es en inglés (consistente con los status labels: `Draft`, `Open`, `In progress`, etc.).

---

## 17. Performance

- El campo `Precision` en `AgentRoutedData` es una cadena de ≤9 bytes (`"estimated"`). El
  costo en serialización/deserialización es negligible.
- `rollupSavings` ya itera todos los eventos `EvtAgentRouted`; añadir la lógica de peor-caso
  es O(1) por evento (una comparación de string). Sin impacto medible.
- La API `/api/board` añade `precision` al objeto `tokenSavings` ya existente — ≤20 bytes
  adicionales en el JSON. Sin impacto en latencia perceptible.
- El badge en el frontend es condicional y sin lógica pesada: `{precision === 'estimated' && <Badge>}`.
  Sin renders innecesarios.

---

## 18. Restricciones

El agente no debe:

- Cambiar `EventVersion` (el campo es aditivo; no es una migración de schema que requiera
  versión nueva).
- Reescribir eventos existentes en `activity.jsonl` (el log es append-only, invariante del
  sistema).
- Cambiar la fórmula de `CostUSD`/`SavedUSD` ni la tabla de precios en `pricing.go`.
- Añadir `precision` al `Card.SavedUSD` per-spec del board (fuera de scope).
- Cambiar el contrato de `GET /api/board` de forma breaking (solo additive).
- Instalar dependencias nuevas (Go stdlib; React existente en `web/`).
- Refactorizar `rollupSavings` más allá de los cambios especificados.
- Cambiar el comportamiento de `runSpecRoute` cuando `--precision` no se pasa (el default
  `"estimated"` debe producir exactamente el mismo resultado que hoy para los commands del
  kit que no pasan el flag).
- Mostrar el badge `Estimated` con tono de alerta (rojo, naranja, icono de advertencia).
  Debe ser neutral — es una aclaración, no un error.
- Modificar las copias embebidas en `cli/internal/scaffold/assets/` manualmente: esas se
  regeneran vía `go generate` a partir de `kit/`.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `AgentRoutedData.Precision` añadido en `event.go`.
- [ ] `RouteAgent` actualizado (parámetro `precision`, normalización, validación) en `standup.go`.
- [ ] `TokenSavings.Precision` + lógica de peor-caso en `board.go`.
- [ ] Tests de precision en `board_test.go` (5 casos nuevos) y tests de `RouteAgent` (3 casos nuevos).
- [ ] Flag `--precision` en `runSpecRoute`; `precision` en la salida `--json`.
- [ ] Kit commands `raw.md` y `bug.md` actualizados con guía de `--precision actual`.
- [ ] Copias vendored en `cli/internal/scaffold/assets/commands/vector/` regeneradas.
- [ ] Componente web del Token Savings Meter con badge `Estimated` condicional.
- [ ] `docs/domain-contract.md` actualizado con el campo `precision`.
- [ ] `gofmt`, `go vet`, `go test ./...` verdes.
- [ ] Build web (`cd web && npm run build`) exitoso (necesario para el embed).

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Revisé `docs/domain-contract.md` (sección de `agent.routed`) y `cli/internal/state/event.go`.
- [ ] Confirmé que `AgentRoutedData` y `EvtAgentRouted` ya existen — este spec solo los extiende.
- [ ] Elegí `omitempty` vs. serialización explícita para `Precision` (Open questions §3) y lo documenté.
- [ ] Implementé el default `"estimated"` en `RouteAgent` (no en el CLI flag, sino en la función).
- [ ] Implementé la retrocompatibilidad: campo `""` → `"estimated"` en el rollup.
- [ ] Implementé el peor-caso: un solo `"estimated"` produce `TokenSavings.Precision = "estimated"`.
- [ ] Solo modifiqué los archivos listados o lo justifiqué.
- [ ] No cambié `EventVersion` ni la fórmula de pricing.
- [ ] No reescribí `activity.jsonl` existente.
- [ ] El badge web es neutral (no alarmante); accesible.
- [ ] Ejecuté `gofmt`, `go vet`, `go test ./...`.
- [ ] Ejecuté el build de `web/` (o confirmé que el componente que modifiqué compila sin error de tipos).
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar.
- [ ] Verifiqué que los kit commands sin `--precision` siguen funcionando con el mismo resultado que antes.

---

## Open questions

1. **Disponibilidad de la señal real del harness**: ¿expone Claude Code el conteo real de
   tokens de un subagente al command orquestador (p.ej. como variable de entorno, en el output
   JSON de `spawn`, o en la metadata del tool result)? Si la señal existe, documentar la forma
   exacta aquí y actualizar los kit commands para capturarla y pasar `--precision actual`. Si
   no existe aún, esta fase sigue siendo valiosa: la infraestructura queda lista para recibirla
   cuando el harness la exponga. **TBD — verificar con docs del harness antes de implementar
   los kit commands.**

2. **Ruta exacta del componente web del Token Savings Meter**: el git status lista
   `web/src/components/KanbanBoa...` (truncado). La ruta exacta del componente que renderiza
   `tokenSavings` es TBD. **El agente debe localizarla antes de modificarla** (buscar usos de
   `tokenSavings` / `totalSavedUsd` en `web/src/`).

3. **`omitempty` vs. serialización explícita en `AgentRoutedData.Precision`**:
   - Con `omitempty`: eventos `"estimated"` no serializan el campo → retrocompatibilidad
     perfecta (log más compacto), pero el rollup debe asumir `"" → estimated` igualmente.
   - Sin `omitempty`: todos los eventos nuevos serializan `"precision":"estimated"` → log
     más verboso pero semánticamente explícito.
   - Recomendación inicial: `omitempty` (consistente con `WorkLoggedData.Change` y otros campos
     opcionales del proyecto). **Confirmar antes de implementar.**
