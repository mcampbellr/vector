# Spec: Standup digest con traza de actividad enriquecida

## 1. Objetivo

Construir `/vector:standup`: un comando que produce un **resumen de actividad para la ceremonia
de scrum** del periodo desde el último standup, generado en lenguaje natural por un agente
barato, y persistido de forma que el **board web lo muestre** (digest global + mini-resumen por
spec) junto a una **timeline de actividad** expandible en cada card.

Para que ese resumen sea fiel a "lo que se hizo" y no solo a "cómo cambió el estado", se
**enriquece la traza**: cada `/vector:apply` registra un evento `work.logged`
(archivos tocados + tasks completadas + nota) en `activity.jsonl`, además del `status.changed`
que ya existe.

Esta feature permite que un dev cierre su ceremonia de scrum **sin redactar el reporte a mano**:
el bot lee la traza y le entrega el resumen; el board lo deja a la vista del equipo.

## 2. Alcance

### Incluido en esta fase

- **Enriquecimiento de la traza en cada apply**: nuevo evento `work.logged`
  (`WorkLoggedData{change, filesTouched, tasksCompleted, note}`) en `activity.jsonl`, escrito
  por un nuevo subcomando `vector spec worklog` que `/vector:apply` invoca tras implementar.
- **Marcador "último standup"** personal en `.vector/local/standup.json` (gitignored). El
  resumen por defecto cubre **desde ese marcador**; el marcador **avanza al correr** el comando
  (al persistir el digest).
- **Proyección de actividad** (`cli/internal/standup`): leer eventos desde el marcador (o una
  ventana `--since`), agrupar por spec, y exponer una estructura `Projection` (por-spec +
  totales) — proyección read-only, sin generación de prosa.
- **Subcomando binario `vector standup`**: `--json` proyecta la actividad del periodo; el
  subcomando `vector standup commit --digest-file -` persiste el digest generado y avanza el
  marcador. CLI sigue siendo el único escritor.
- **Project command `/vector:standup`** (`kit/commands/vector/standup.md`): orquesta —
  proyecta vía binario, pasa el JSON al agente **Haiku** (`vector-standup-writer`) para generar
  el digest **global + por spec**, y persiste vía `vector standup commit`.
- **Resumen persistido para la UI**: el digest (global + por-spec) se guarda en
  `.vector/local/standup.json`; el board lo sirve en `GET /api/standup`.
- **UI del board**: vista **StandupView** dedicada (digest del periodo + tarjetas por spec) y
  **SpecTimeline** expandible en el detalle de cada card (historial de eventos). Nuevo endpoint
  `GET /api/activity?spec=<id>` para la timeline. Hook `useStandup` + tipos TS.

### Fuera de scope

- **`/vector:daily`** (roll-up del día) — comando aparte, no se implementa ni se fusiona aquí.
- **Exportación a sistemas externos** (Slack, Jira, calendarios). El digest queda en CLI + UI.
- **Plantillas de digest personalizables por repo**. La plantilla del agente es fija en V1.
- **Métricas de productividad / burndown** y el Token Savings Meter (ya existe, es aparte).
- **Editar o borrar eventos** desde la UI (la timeline es read-only).
- **Capturar detalle por cada cambio de estado o por commit de git**. La fuente de "trabajo
  hecho" es el evento `work.logged` de apply; `status.changed` aporta las transiciones.
- **Persistir el digest committed/compartido**: es personal y gitignored, como `activity.jsonl`.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Binario: **Go** (módulo único, stdlib; `cli/`). Sin dependencias externas.
- Project command: **Markdown + frontmatter** orquestado por Claude (patrón `kit/commands/vector/apply.md`, `raw.md`).
- Frontend: **React + TypeScript** (Vite), embebido en el binario (`cli/internal/webui/dist`).
- Agente de generación: archivo markdown del kit (`kit/agents/vector-standup-writer.md`), tier **Haiku** (token-routing).

### Versiones relevantes

- Go: `1.26` (citado de `cli/go.mod`). Confirmar exacto: `TBD — ver Open questions` si difiere.
- React/Vite: la versión exacta vive en `web/package.json` (no se altera).

### Patrones existentes a respetar

- **CLI-owns-writes**: el binario es el único escritor de `state.json` y `activity.jsonl`; el
  command nunca edita `.vector/` a mano (`workflows/state-sync-discipline.md`).
- **Eventos tipados** en `activity.jsonl`: payload por `Type`, decodificado por type switch; sin
  `any` (`cli/internal/state/event.go`). El nuevo `work.logged` sigue ese patrón.
- **Append-only** del activity log: lectura O(n), sin mutación; `Store.ReadEvents()` ya existe
  (`cli/internal/state/store.go:332`); `Store.AppendEvent` serializa con mutex
  (`store.go:358`).
- **Board = proyección read-only** de `state` (`cli/internal/board/board.go`); el server SSE
  re-renderiza (`server.go`). El frontend no lee el filesystem; consume la API HTTP.
- **Token routing**: la generación de prosa es trabajo barato (input = JSON estructurado,
  output = 1–3 párrafos) → agente Haiku, documentado en el command (`product/token-routing.md`).
- Git artifacts en inglés kebab-case; `id` del spec == nombre del change OpenSpec.
- One-component-per-file en `web/` (`standards/typescript-react.md`): `StandupView/index.tsx`
  + hermanos por subcomponente.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `cli/internal/state` con `Store.ReadEvents()` (`store.go:332`) y `Store.AppendEvent`
      (`store.go:358`) — verificado.
- [x] Eventos `status.changed` poblándose desde `apply`/`status`/`sync` (`transition.go:93`).
- [x] `vector serve` con server SSE y endpoints `/api/board` + `/api/events`
      (`cli/internal/board/server.go:30`) — verificado.
- [x] `kit/commands/vector/apply.md` (punto de integración del enriquecimiento) — verificado.
- [x] Agentes del kit como patrón de invocación (`kit/agents/vector-spec-refiner.md`,
      `vector-spec-validator.md`) — verificado.
- [x] `.vector/config.json` presente (`vector init` corrido) — verificado en este repo.

Si alguna dependencia no existe, el binario se detiene con mensaje accionable. No inventa
contratos ni rutas.

---

## 5. Arquitectura

### Patrón a usar

**Append-only log → proyección read-only en memoria → generación NL por agente → persistencia
del digest vía CLI → exposición HTTP + UI.** Misma separación que el board: el binario proyecta
y persiste; la prosa la genera el command con un agente barato; la UI consume la API.

### Capas afectadas

- **State** (`cli/internal/state`): sí — nuevo evento `work.logged` + payload; método
  `WorkLog(...)`; lectura/escritura del marcador y digest (`standup.json`).
- **Standup (nuevo paquete)** (`cli/internal/standup`): sí — proyección de eventos por spec y
  por periodo (sin LLM).
- **Binario CLI** (`cli/cmd/vector`): sí — `runStandup` (`standup` / `standup commit`) y
  `case "worklog"` en `runSpec`.
- **Board/API** (`cli/internal/board`): sí — handlers `GET /api/standup` (digest persistido) y
  `GET /api/activity?spec=<id>` (timeline proyectada).
- **Web** (`web/`): sí — `StandupView`, `SpecTimeline`, hook `useStandup`, tipos.
- **Kit** (`kit/`): sí — command `standup.md`, agente `vector-standup-writer.md`, y MODIFICAR
  `apply.md` para invocar `worklog`.

### Flujo esperado

**A. Enriquecimiento (en cada apply):**

1. `/vector:apply` implementa el change (flujo existente; `vector spec apply` ya flipa
   `open → in-progress` y emite `spec.applied` + `status.changed`).
2. Tras implementar, el command reúne **archivos tocados** y **tasks completadas** (de su propio
   trabajo) y una nota corta, y llama `vector spec worklog --id <id> --files … --tasks … --note …`.
3. El binario appendea un evento `work.logged` (sin tocar `status.json`; solo activity log).

**B. Standup (en la ceremonia):**

1. Dev ejecuta `/vector:standup` (opcional `--since 24h|today|7d`; default = desde el marcador).
2. Command llama `vector standup --json` → el binario lee eventos desde el marcador/ventana,
   los agrupa por spec (`standup.Project`) y retorna `{period, since, perSpec[], totals}`.
3. Command pasa el JSON al agente **Haiku** (`vector-standup-writer`): genera **digest global**
   (1–3 párrafos para la ceremonia) + **mini-resumen por spec activo**.
4. Command persiste vía `vector standup commit --digest-file -` (stdin): el binario escribe el
   digest a `.vector/local/standup.json` y **avanza el marcador** a "ahora".
5. Command imprime el digest en la terminal y sugiere abrir la StandupView del board.
6. En el board web, `GET /api/standup` sirve el digest persistido (StandupView); `GET
   /api/activity?spec=<id>` sirve la timeline de cada card (SpecTimeline).

### Ubicación de archivos nuevos

```txt
cli/internal/standup/standup.go        # proyección
cli/cmd/vector/main.go                 # runStandup + case "worklog" (MODIFICAR)
cli/internal/board/server.go           # nuevos handlers (MODIFICAR)
kit/commands/vector/standup.md         # project command
kit/agents/vector-standup-writer.md    # agente Haiku
web/src/components/StandupView/        # vista dedicada
web/src/components/SpecTimeline/       # timeline por card
web/src/api/useStandup.ts              # hook
```

No crear carpetas nuevas si ya existe convención equivalente.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `cli/internal/state/event.go` | MODIFICAR | `EvtWorkLogged = "work.logged"` + `WorkLoggedData{Change, FilesTouched, TasksCompleted, Note}` | `cli/internal/state/event.go` (`AppliedData`, `StatusChangedData`) |
| `cli/internal/state/store.go` | MODIFICAR | `WorkLog(id, data, actor, now)` (append `work.logged`); `ReadStandup()/WriteStandup(digest, marker)` sobre `.vector/local/standup.json` | `cli/internal/state/store.go` (`AppendEvent`, `activityPath`) |
| `cli/internal/standup/standup.go` | NUEVO | `Project(events, since) Projection`: agrupa por spec, totales por status; sin LLM | `cli/internal/board/board.go` (`Build`, `rollupSavings`) |
| `cli/internal/standup/standup_test.go` | NUEVO | Table-driven: filtro por `since`, agrupación por spec, periodo vacío | `cli/internal/board/server_test.go` |
| `cli/cmd/vector/main.go` | MODIFICAR | `case "standup"` → `runStandup` (`--since`, `--json`, `commit --digest-file`); `case "worklog"` en `runSpec` (`--id`, `--files`, `--tasks`, `--note`) | `cli/cmd/vector/main.go` (`runSpec`, `runSpecCreate`) |
| `cli/internal/board/server.go` | MODIFICAR | `GET /api/standup` (digest persistido); `GET /api/activity?spec=<id>` (timeline) | `cli/internal/board/server.go` (`handleBoard`) |
| `cli/internal/board/server_test.go` | MODIFICAR | Tests de los nuevos handlers (200 + JSON, 400 `since` inválido, 404 spec) | `cli/internal/board/server_test.go` |
| `kit/commands/vector/standup.md` | NUEVO | Project command: proyecta, rutea a Haiku, persiste digest, reporta | `kit/commands/vector/apply.md`, `raw.md` |
| `kit/commands/vector/apply.md` | MODIFICAR | Tras implementar, invocar `vector spec worklog …` con archivos/tasks/nota | `kit/commands/vector/apply.md` (paso de finish) |
| `kit/agents/vector-standup-writer.md` | NUEVO | Agente Haiku: JSON de actividad → digest global + por spec | `kit/agents/vector-spec-validator.md` |
| `cli/internal/scaffold/assets/commands/vector/standup.md` | NUEVO (generado) | Copia embebida del command vía la directiva `//go:generate` en `cli/internal/scaffold/scaffold.go:13` (la misma que vendoriza `apply.md`/`raw.md`) | sibling `apply.md`, `raw.md` |
| `web/src/types/standup.ts` | NUEVO | Tipos `StandupDigest`, `SpecActivity`, `ActivityEvent` (espejo de la API) | `web/src/types/board.ts` |
| `web/src/api/useStandup.ts` | NUEVO | Fetch `GET /api/standup` (+ `GET /api/activity?spec=`) con loading/error | `web/src/api/useBoard.ts` |
| `web/src/components/StandupView/index.tsx` | NUEVO | Vista dedicada: digest global + tarjetas por spec | `web/src/components/KanbanBoard` |
| `web/src/components/SpecTimeline/index.tsx` | NUEVO | Timeline expandible en el detalle de card | `web/src/components/SpecCard` |

### Detalle por archivo

#### `cli/internal/state/event.go` — MODIFICAR

- Agregar `EvtWorkLogged EventType = "work.logged"` al bloque de constantes.
- Agregar `type WorkLoggedData struct { Change string; FilesTouched []string; TasksCompleted []string; Note string }` con tags JSON (`change`,`filesTouched`,`tasksCompleted`,`note`), siguiendo el estilo de `AppliedData`.
- No alterar eventos existentes ni `EventVersion` (campo nuevo es aditivo; un consumidor viejo ignora `work.logged`).

#### `cli/internal/state/store.go` — MODIFICAR

- `WorkLog(id string, data WorkLoggedData, actor string, now time.Time) error`: valida que el spec exista; appendea `Event{Type: EvtWorkLogged, SpecID: id, …}` vía `appendEvent` (no toca `state.json`).
- Marcador + digest: `standupPath()` = `.vector/local/standup.json`; `ReadStandup() (*StandupFile, error)` (devuelve cero-value si no existe); `WriteStandup(digest StandupDigest, markerAt time.Time) error` (escritura serializada por mutex, atómica como el resto del store).
- `.vector/local/` ya es gitignored (schema). No commitear.

#### `cli/internal/standup/standup.go` — NUEVO

- `Project(events []state.Event, since time.Time) Projection`: filtra `e.TS >= since`, agrupa por `SpecID`, produce `SpecActivity{ID, Title, LastStatus, LastChanged, ChangeCount, Work []WorkLoggedData, Transitions []StatusChangedData}` y `Totals{Specs, Changes, ByStatus map[string]int}`.
- Usa el `ts` de cada evento (nunca `time.Now()` para filtrar — determinista).
- Sin generación de prosa, sin escritura, sin red.

#### `cli/cmd/vector/main.go` — MODIFICAR

- `runSpec`: agregar `case "worklog": return runSpecWorklog(args[1:])` (flags `--id`, `--files` csv, `--tasks` csv, `--note`).
- `case "standup"` en el switch raíz → `runStandup(os.Args[2:])`:
  - sin subcomando: resuelve `since` (default = marcador de `ReadStandup`; o `--since 24h|today|7d`), corre `standup.Project`, emite JSON (`--json`) o texto.
  - `commit --digest-file <path|->`: lee el digest (stdin si `-`), `WriteStandup` + avanza marcador a `now`.
- Validar `--since`; error accionable si formato inválido.

#### `cli/internal/board/server.go` — MODIFICAR

- `mux.HandleFunc("/api/standup", s.handleStandup)`: lee el digest persistido (`ReadStandup`), responde JSON (o `{}` si no hay digest aún).
- `mux.HandleFunc("/api/activity", s.handleActivity)`: query `spec` (requerido) + `since` (opcional, default `24h`); proyecta vía `standup.Project` filtrado por spec; 400 si `since` inválido, 404 si el spec no existe.
- Read-only; no mutación. No tocar el SSE de `/api/events`.

#### `kit/commands/vector/standup.md` — NUEVO

- Leer `$ARGUMENTS` (ventana opcional: `24h`/`today`/`7d`; vacío → desde marcador).
- `vector standup --since <win> --json` → proyección.
- **Token routing:** pasar el JSON al agente **Haiku** `vector-standup-writer` (no rehacer lectura en el agente); recibir `{global, perSpec}`.
- `vector standup commit --digest-file -` (stdin con el digest) → persiste + avanza marcador.
- Reportar: digest global, conteos, y "abre la StandupView del board". No persistir nada a mano.

#### `kit/commands/vector/apply.md` — MODIFICAR

- En el paso de finish (tras implementar y antes de transicionar a review), agregar: reunir archivos tocados (del diff del trabajo) + tasks completadas (de `tasks.md`/OpenSpec) + nota corta, e invocar `vector spec worklog --id <id> --files … --tasks … --note …`.
- Restricción: no cambiar la lógica de selección ni las transiciones existentes; el worklog es **aditivo** (un evento más), no un gate.

#### `kit/agents/vector-standup-writer.md` — NUEVO

- Tier **Haiku**. Input: JSON de `standup.Project`. Output: `{ "global": "<1–3 párrafos para standup>", "perSpec": [{ "id": "...", "summary": "<1–2 frases>" }] }`.
- Prompt fijo, orientado a ceremonia (qué se avanzó, qué pasó a review, qué quedó en needs-attention). Sin inventar trabajo no presente en los eventos.

#### `web/src/api/useStandup.ts` — NUEVO

- `useStandup()` → fetch `GET /api/standup`; `useSpecActivity(specId)` → `GET /api/activity?spec=<id>`. Retornan `{data, loading, error}`. Tipos de `web/src/types/standup.ts`. Manejo de error como `useBoard`.

#### `web/src/components/StandupView/index.tsx` y `SpecTimeline/index.tsx` — NUEVO

- `StandupView`: render del digest global + lista de tarjetas por spec (id, título, mini-resumen, último status). Estados loading/empty/error.
- `SpecTimeline`: lista vertical de eventos (`ts` + transición/`work.logged`), expandible en el detalle de la card. Read-only.

Restricciones: one-component-per-file; tokens visuales de `docs/kanban-ui-reference.md`; sin librerías nuevas.

---

## 7. API Contract

> Vector no usa `docs/api-contract.md`; el contrato `cli/ ↔ web/` vive en
> `cli/internal/board/*.go` y se **espeja a mano** en `web/src/types/` hasta que exista typegen
> (`standards/typescript-react.md`). Esta sección define los endpoints nuevos; son la fuente de
> verdad para esta fase.

### Endpoints involucrados

- `GET /api/standup` → digest persistido del último standup.
- `GET /api/activity?spec=<id>&since=<dur>` → eventos proyectados de un spec (timeline).

`GET /api/standup` (200):

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-06-25T15:00:00Z",
  "since": "2026-06-24T09:00:00Z",
  "markerAt": "2026-06-25T15:00:00Z",
  "global": "<digest en prosa para la ceremonia>",
  "perSpec": [
    { "id": "new-patient-expediente", "title": "New patient expediente",
      "status": "review", "summary": "<1–2 frases>", "changeCount": 3 }
  ],
  "totals": { "specs": 5, "changes": 12, "byStatus": { "review": 1, "in-progress": 2 } }
}
```

`GET /api/activity?spec=<id>&since=24h` (200):

```json
{
  "spec": "new-patient-expediente",
  "since": "24h",
  "events": [
    { "ts": "2026-06-24T14:40:00Z", "type": "status.changed", "from": "open", "to": "in-progress", "trigger": "apply" },
    { "ts": "2026-06-24T15:30:00Z", "type": "work.logged", "filesTouched": ["a.go","b.go"], "tasksCompleted": ["DTO mapper"], "note": "money assembler wired" }
  ]
}
```

### Forma del error (ambos endpoints)

En error, el handler responde el status code + un body JSON `{ "error": "<message>" }`
(`Content-Type: application/json`). Los hooks (`useStandup`/`useSpecActivity`) parsean ese
campo `error`. El `500` también devuelve body (`{ "error": "could not read activity log" }`),
no solo status.

- `400` → `{ "error": "invalid since: use 24h, today or 7d" }` (`?since=` inválido).
- `404` → `{ "error": "spec '<id>' not found" }` (solo `/api/activity`).
- `500` → `{ "error": "could not read activity log" }`.
- `markerAt`: timestamp del marcador tras el último standup (para "last standup ran at …" en la
  StandupView). Igual a `generatedAt` cuando el digest se acaba de persistir.
- No inferir campos extra ni renombrar propiedades; los tipos TS espejan exactamente estas formas.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `vector spec worklog --id <id> --files … --tasks … --note …` appendea un `work.logged` y no muta `state.json`.
- [ ] `/vector:apply` registra un `work.logged` tras implementar (verificable en `activity.jsonl`).
- [ ] `vector standup --json` proyecta la actividad desde el marcador (o `--since`) agrupada por spec.
- [ ] `/vector:standup` genera digest global + por-spec vía Haiku y lo persiste; el marcador avanza.
- [ ] `GET /api/standup` devuelve el digest persistido; `GET /api/activity?spec=<id>` la timeline.
- [ ] StandupView muestra el digest; SpecTimeline muestra el historial expandible por card.
- [ ] `--since` / `?since=` acepta `24h`,`today`,`7d`; rechaza inválidos con mensaje claro.
- [ ] Sin regresiones: `apply`, `status`, `close`, `sync`, `/api/board`, `/api/events` intactos.
- [ ] Sin errores de `go vet`, linter, ni typecheck de `web/`.

### Tests requeridos

- [ ] `standup.Project`: agrupación por spec + filtro `since` (table-driven).
- [ ] `WorkLog`: appendea evento, no toca `state.json`; spec inexistente → error.
- [ ] `ReadStandup/WriteStandup`: round-trip + avance de marcador.
- [ ] Handlers: `/api/standup` (200/{}), `/api/activity` (200, 400 `since` inválido, 404 spec).
- [ ] Periodo vacío: proyección vacía sin panic; digest `{}`.

### Comandos de verificación

```bash
go -C cli vet ./...
go -C cli test ./...
npm --prefix web run typecheck
npm --prefix web run build
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

> Feature CLI + web; las subsecciones de formularios/passwords no aplican (no hay formularios).

### CLI (`/vector:standup`)

- Imprime el **digest global** legible + conteos (`5 specs, 12 cambios desde el último standup`).
- Sugiere abrir la StandupView del board y/o copiar (`| pbcopy` en macOS).
- Errores accionables: `invalid --since: use 24h, today or 7d`.

### Board web

- **StandupView**: encabezado con el periodo (`since … → ahora`), el digest global, y tarjetas
  por spec (título, status pill, mini-resumen). Estado **empty**: "no activity since last
  standup".
- **SpecTimeline**: lista vertical compacta (hora + transición/trabajo + nota), expandible desde
  el detalle de la card. No abruma: trunca a N eventos con "ver más" si excede.

### Loading / Errores / Navegación

- Loading: spinner + "loading standup…". Error: banner "error loading standup: <razón>" con
  reintento. Navegación: la StandupView es una vista/panel del board; no rompe el flujo del
  kanban.

### Accesibilidad

- Timeline con HTML semántico (lista ordenada); status no transmitido solo por color (el pill ya
  lleva label). Controles con labels accesibles.

---

## 10. Decisiones tomadas

El agente no debe cuestionar ni cambiar estas decisiones (tomadas con el usuario):

- **Enriquecer la traza con detalle por apply** (no solo `status.changed`): nuevo `work.logged`.
  *Por qué:* `status.changed` dice "cómo cambió el estado", no "qué se hizo"; el digest de
  ceremonia necesita el trabajo (archivos/tasks), que solo el flujo de apply conoce.
- **Comando nuevo `/vector:standup`** (no extender `/vector:daily`). *Por qué:* son intenciones
  distintas (ceremonia recurrente vs roll-up del día); mezclarlas en flags acopla dos ciclos de
  vida y complica el default "desde el último standup".
- **Ventana por defecto = desde el último standup**; el marcador **avanza al correr** el comando.
  *Por qué:* la ceremonia cubre "lo nuevo desde la última vez"; avanzar al correr es el flujo de
  un solo paso que el dev espera (no un commit separado que se olvida), y `activity.jsonl`
  retiene todo, así que un avance no destruye historial.
- **Resumen también en la UI**: el digest se persiste y el board lo sirve. *Por qué:* el equipo
  ve el resumen sin que el dev pegue texto; y el board (Go, read-only) no puede regenerar prosa,
  así que debe leer un digest ya escrito.
- **Granularidad: global + por spec**. *Por qué:* la ceremonia necesita el titular global, pero
  el dev también reporta ticket por ticket ("resumen de lo que se hizo en esos tickets").
- **Generación NL por agente Haiku** (`vector-standup-writer`); el binario Go no llama LLM.
  *Por qué:* mantener el binario sin dependencias de LLM/red (distribución de un solo binario);
  la prosa es trabajo barato de input estructurado → tier barato (`product/token-routing.md`).
- **CLI-owns-writes**: el binario escribe `work.logged`, el marcador y el digest; el command no
  edita `.vector/` a mano. *Por qué:* única escritura serializada = invariante del estado
  (`workflows/state-sync-discipline.md`); evita corrupción/race del log y el digest.
- **Digest + marcador personales/gitignored** (`.vector/local/standup.json`), como `activity.jsonl`.
  *Por qué:* la actividad y el marcador son por-dev y cambian a cada corrida; committearlos
  generaría conflictos y ruido en git sin valor para el equipo.

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación, no la
implementa.

---

## 11. Edge cases

### Datos inválidos

- `--since` / `?since=` con formato no soportado → error claro + uso; no proyectar.
- `vector spec worklog` sin `--id` o con spec inexistente → error accionable.
- `vector standup commit --digest-file -` con **JSON inválido en stdin** → error
  `invalid digest json`; **no escribir el digest y NO avanzar el marcador** (la corrida no
  cuenta como standup válido). Igual para `--digest-file <path>` inexistente/ilegible.

Comportamiento: no escribir nada si la validación falla.

### Códigos HTTP (API local)

La API es **local, sin auth y solo GET**. Solo aplican `400` (`since` inválido), `404`
(spec inexistente en `/api/activity`) y `500` (lectura del log). **No aplican**
`401/403/409/422/429` — no hay autenticación, mutación, conflicto ni rate-limit en un
binario local.

### Activity log vacío o roto

- `activity.jsonl` ausente/vacío → proyección vacía, digest `{}`, StandupView muestra **empty**;
  nunca panic.
- Línea JSONL corrupta → saltarla y continuar (log a stderr), no abortar todo el resumen.

### Periodo sin actividad

- Sin eventos desde el marcador → digest "no activity since last standup"; el marcador **igual
  avanza** (corrida válida).

### Eventos sin `reason` / sin `work.logged`

- `status.changed` sin `reason`: mostrar la transición sin la frase; no colapsar la línea.
- Spec con solo transiciones (sin `work.logged`): el digest se basa en las transiciones.

### Concurrencia

- Lectura del activity log es read-only (append-only ⇒ segura). La escritura del digest/marcador
  pasa por el mutex del `Store` (serializada), como el resto.

### Fetch lento / timeout en UI

- Si `GET /api/activity` (log grande) tarda, `useSpecActivity` mantiene el estado **loading**
  (spinner en la timeline expandida); ante fallo/timeout del fetch, pasa a **error** con
  reintento. No bloquea el resto del board (la timeline es lazy por card).

### API

- `/api/activity` con `spec` inexistente → 404. `/api/standup` sin digest aún → `{}` (no 500).
- Timestamps: filtrar por el `ts` del evento, nunca por el reloj al renderizar.

---

## 12. Estados de UI requeridos

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | StandupView con último digest persistido (o CTA si nunca se corrió) | abrir, expandir timelines |
| loading | spinner + "loading standup…" | esperar |
| success | digest global + tarjetas por spec + timelines expandibles | leer, expandir, copiar |
| error | banner "error loading standup: <razón>" | reintentar |
| empty | "no activity since last standup" | correr `/vector:standup`, cambiar ventana |

`disabled`/`offline`: No aplica — el board web es local y efímero (no hay modo offline propio).

---

## 13. Validaciones

### Validaciones de cliente (CLI + web)

| Campo | Regla | Mensaje |
|---|---|---|
| `--since` / `?since=` | `24h` \| `today` \| `7d` (o vacío → marcador) | `invalid --since: use 24h, today or 7d` |
| `worklog --id` | kebab-case, spec existente | `spec '<id>' not found` |
| `worklog --files`/`--tasks` | CSV; vacío permitido | — |
| `worklog --note` | opcional; texto corto, máx. 280 chars (se trunca) | — |
| `standup commit --digest-file` | `-` (stdin) o path legible; contenido = JSON válido | `invalid digest json` / `cannot read digest file` |

### Validaciones de servidor

No aplica — no hay backend remoto; la API local valida los query params arriba. Las
validaciones de dominio (estado/transición) viven en `cli/internal/state` y no cambian aquí.

---

## 14. Seguridad y permisos

- `activity.jsonl` y `standup.json` son **personales y gitignored**; no se comparten ni
  sincronizan cross-repo.
- El agente Haiku recibe **solo el JSON estructurado** de la proyección (no output libre del
  binario); prompt fijo. No se le pasan secrets (la traza es metadata de specs).
- No registrar payloads sensibles en el digest; `work.logged.note` es texto corto del dev, no
  vuelca diffs completos.
- La API local es **read-only** (no muta estado); 401/403 no aplican (binario local sin auth).

---

## 15. Observabilidad y logging

- Reusar el activity log existente; esta feature **añade** el evento `work.logged` (no reescribe
  los existentes).
- A stderr (mecanismo de logging del binario): líneas JSONL corruptas saltadas, errores de
  lectura del log, tiempo de proyección si es notable.
- No registrar: secrets, tokens, PII, diffs completos.

---

## 16. i18n / textos visibles

Vector **no tiene sistema i18n**; los textos visibles de `web/` están en inglés hardcodeado
(convención del repo). La conversación del command es en el idioma del usuario; el digest lo
genera el agente en el idioma del usuario (ceremonia), pero los labels de UI quedan en inglés.

| Key (label UI) | Texto (EN) |
|---|---|
| standup.title | `Standup` |
| standup.loading | `loading standup…` |
| standup.empty | `no activity since last standup` |
| standup.period | `since {date}` |
| standup.error | `error loading standup` |
| timeline.header | `Activity` |
| timeline.more | `show more` |
| standup.retry | `retry` |
| timeline.retry | `retry` |

---

## 17. Performance

- Lectura de `activity.jsonl` en una pasada O(n); filtrar por `since` en la misma pasada.
- Proyección por spec con mapa en memoria (lookup O(1)). Truncar la timeline en UI a N eventos.
- `/api/standup` sirve el digest **ya persistido** (no regenera prosa por request).
- `/api/activity` proyecta on-demand; si el log crece mucho, leer streaming línea a línea
  (no cargar todo en memoria de golpe).
- La generación NL (Haiku) corre **solo** al ejecutar `/vector:standup`, no por cada request del
  board.

---

## 18. Restricciones

El agente no debe:

- Modificar el schema de eventos existentes ni `SpecState` (el `work.logged` es **aditivo**).
- Persistir el digest committed/compartido (es personal, gitignored).
- Hacer que el binario Go llame a un LLM (la prosa es del agente del command).
- Agregar dependencias externas (Go stdlib; React + libs ya presentes en `web/`).
- Cambiar subcomandos existentes ni el SSE `/api/events` (solo agregar `standup`, `worklog`,
  `/api/standup`, `/api/activity`).
- Implementar `/vector:daily`, exportación externa, o plantillas personalizables.
- Refactorizar código no relacionado ni cambiar estilos/navegación globales.
- Ignorar errores de lint/typecheck/tests.

---

## 19. Entregables

Al finalizar deben quedar:

- [ ] `work.logged` (evento + payload) y `WorkLog`/`ReadStandup`/`WriteStandup` en `state`.
- [ ] Paquete `cli/internal/standup` con `Project` + tests.
- [ ] `vector standup` (+ `commit`) y `vector spec worklog` en el binario + tests.
- [ ] Handlers `/api/standup` y `/api/activity` + tests.
- [ ] `kit/commands/vector/standup.md` (+ vendored en assets) y `apply.md` modificado.
- [ ] Agente `kit/agents/vector-standup-writer.md` (Haiku).
- [ ] `web/`: `StandupView`, `SpecTimeline`, `useStandup`, tipos `standup.ts`.
- [ ] Gate verde (`gofmt`, `go vet`, `go test`, `npm typecheck`, `npm build`).
- [ ] Docs: actualizar `docs/domain-contract.md` §4 (endpoints) y §5 (comando→escritura) y el
      schema `docs/schemas/state-and-activity.md` (evento `work.logged`).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `docs/schemas/state-and-activity.md` (eventos) y `docs/domain-contract.md` (mapa comando→state, endpoints).
- [ ] Confirmé que `activity.jsonl` y `standup.json` son append/local y gitignored.
- [ ] Mantuve CLI-owns-writes (el command no edita `.vector/` a mano).
- [ ] El `work.logged` es aditivo; no cambié eventos existentes ni `SpecState`.
- [ ] Solo modifiqué archivos listados o justifiqué la excepción.
- [ ] Seguí ejemplos reales (`board.go`, `apply.md`, `vector-spec-validator.md`, `useBoard.ts`).
- [ ] Implementé los estados de UI (loading/success/error/empty).
- [ ] Implementé los edge cases (log vacío/roto, periodo sin actividad, since inválido).
- [ ] No agregué dependencias no autorizadas.
- [ ] Ejecuté `go vet`, `go test`, `npm typecheck`, `npm build`.
- [ ] Actualicé los docs afectados.
- [ ] No dejé TODOs sin justificar.

---

## Open questions

- Versión exacta de Go (`cli/go.mod`) si difiere de `1.26`.
- Valor de `N` para truncar la timeline en UI (sugerido: 20 eventos; confirmar al implementar `web/`).
- ¿La StandupView es una ruta/página propia o un panel/drawer sobre el board? (look & feel a
  fijar contra `docs/kanban-ui-reference.md` al implementar `web/`).
- Formato de "ventanas absolutas" (`--since <RFC3339>`): fuera de V1; reevaluar si se pide.
