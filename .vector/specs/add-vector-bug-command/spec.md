# Spec: Comando /vector:bug para investigar bugs con trazabilidad de causa (`relatedTo[]`)

## 1. Objetivo

Construir `/vector:bug`: un **project command** del kit que toma un reporte crudo de bug, lo
**refina** mediante un subagente Haiku (`vector-bug-refiner`, espejo agnosticizado del
`bug-spec-refiner` global), **deduce la causa raíz** del bug (qué trabajo previo lo originó —
spec de Vector o ticket externo) usando `git blame`/`git log` como señal, y registra el bug
como **spec card en `draft`** con un campo nuevo `relatedTo[]` persistido en el estado. Es el
equivalente bug-framed de `/vector:raw`, integrado al board y distribuible embebido en el binario.

Esta feature permite que un developer **reporte un bug y obtenga automáticamente la trazabilidad
de su causa** (el/los spec(s)/ticket(s) que lo provocaron), de modo que el board conserve un
**record consultable** de por qué apareció el bug — visible en la card, la API y el standup —
sin armar esa relación a mano. Tras `/vector:bug`, el dev continúa con `/vector:propose` (que crea
el OpenSpec change `fix-…` y mueve `draft → open`) y `/vector:apply` (que implementa el fix).

## 2. Alcance

### Incluido en esta fase

- **Project command `/vector:bug`** (`kit/commands/vector/bug.md`): parseo del reporte, resolución
  de rama/archivo/spec opcional, deducción de causa vía git (main loop, barato), delegación al
  refiner Haiku, composición del spec bug-framed con la plantilla canónica de 20 secciones,
  validación (Sonnet, reusando `vector-spec-validator`), y registro del card `draft` con
  `relatedTo[]` vía el binario. **Termina en `draft`** (no crea el OpenSpec change; eso es
  `/vector:propose`).
- **Agente refiner `vector-bug-refiner`** (`kit/agents/vector-bug-refiner.md`), tier **Haiku**,
  read-only: refina el reporte crudo en un brief estructurado (problem / expected / actual /
  reproduction / acceptance / test plan / risks / open questions), agnosticizado del global.
- **Campo de estado `relatedTo[]`** en `SpecState` (`cli/internal/state/types.go`): lista
  opcional de relaciones causa→bug. Cada item: `{kind, ref, source}` con `kind ∈ {spec, ticket}`,
  `ref` (id de spec de Vector o `provider:key` del ticket) y `source ∈ {blame, manual}`. Persistido
  en `state.json`, expuesto en la proyección del board (API) y mostrado en la card del panel web.
- **Escritura de relaciones por el binario** (CLI-owns-writes): seed al crear
  (`vector spec create … --related '<json>'`) y, fuera del create, un subcomando
  `vector spec relate <id> --kind <k> --ref <r> [--source blame|manual]` para añadir/gestionar
  relaciones. Cada escritura appendea un evento `spec.related` a `activity.jsonl`.
- **Flag `--json` en `vector spec list`**: hoy el subcomando solo imprime columnas de texto
  (`cli/cmd/vector/main.go:776`); se añade salida JSON para que la deducción de causa resuelva
  commits → spec ids de forma robusta, sin parsear texto frágil.
- **Deducción de causa (inferir, luego preguntar)**: el command corre `git blame`/`git log` sobre
  los archivos/símbolos del reporte para hallar commits sospechosos, y mapea esos commits a un
  **spec de Vector** (vía el change de OpenSpec / id) o a un **ticket** (vía trailer del commit).
  Si la confianza es alta y el candidato es único → lo siembra. Si es ambiguo, hay varios, o la
  confianza es baja → **pregunta** con `AskUserQuestion`. Nunca adivina.
- **Vendoring**: command + agente embebidos en `cli/internal/scaffold/assets/` vía `go generate`
  (`//go:generate` en `scaffold.go:13`, `//go:embed all:assets` en `scaffold.go:26`).

### Fuera de scope

- **Implementar el fix**: eso es `/vector:apply`. `/vector:bug` solo autora y registra el draft.
- **Crear el OpenSpec change**: `/vector:bug` termina en `draft`; el change `fix-…` lo crea
  `/vector:propose` (no se delega ni duplica su orquestación dentro de `/vector:bug`).
- **Almacenar commits/PRs crudos como tipo de relación**: el commit de `git blame` es **señal de
  inferencia**, no un `kind` persistido. Los `kind` de `relatedTo[]` en V1 son `spec` y `ticket`
  (ver Open questions para un posible `kind: commit` futuro).
- **Validar el ticket/spec relacionado contra el tracker externo** (Jira/Linear/GitHub) ni llamadas
  de red: la relación se registra localmente; la verificación del ticket sigue siendo de
  `/vector:link`.
- **Drag-and-drop / edición de `relatedTo[]` desde el panel web**: el web solo **muestra** las
  relaciones (read-only, como el resto del board hoy).
- **Postear el bug en un tracker externo** ni abrir issues automáticamente.
- **Rediseñar la plantilla de spec**: se reusa `.claude/vector/spec-template.md` (bug-framed).

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- **Project command**: Markdown + frontmatter orquestado por Claude (patrón
  `kit/commands/vector/raw.md`, `comment.md`, `apply.md`).
- **Agente refiner**: Markdown del kit (`kit/agents/vector-bug-refiner.md`), tier **Haiku**
  (token-routing: refinar un reporte en secciones es trabajo estructurado, no razonamiento caro).
  Patrón de agente: `kit/agents/vector-spec-refiner.md` (read-only, brief estructurado).
- **Validación del spec**: se reusa el agente existente `vector-spec-validator` (Sonnet), igual que
  `/vector:raw`. No se crea un validador nuevo.
- **Estado (Go)**: `cli/internal/state` (struct `SpecState`, `Store`, eventos) — se añade el campo
  `RelatedTo` y un evento `spec.related`. `cli/cmd/vector` — se añade el subcomando
  `vector spec relate` y el flag `--related` a `vector spec create`.
- **Board/web**: `cli/internal/board` (proyección `Card`) expone `relatedTo`; `web/` (tipos +
  `SpecCard`) lo muestra.
- **Deducción de causa**: `git blame`, `git log -S`, `git log --grep`, `git show` (stdlib del
  sistema vía `Bash(git *)`); sin dependencias externas.

### Versiones relevantes

- Go: `1.26` (de `cli/go.mod`).
- No se introducen dependencias externas nuevas (stdlib Go; `git` del sistema).

### Patrones existentes a respetar

- **CLI-owns-writes**: el command **nunca** edita `.vector/` a mano; toda mutación de estado
  (`relatedTo[]`, creación del card) pasa por el binario (`workflows/state-sync-discipline.md`,
  `architecture/state-model.md`).
- **El binario es el único escritor del spec doc**: el command autora el markdown y lo pasa por
  stdin a `vector spec create --body-file -`; el binario lo escribe en `specPath` (igual que
  `/vector:raw`).
- **Token routing** (`product/token-routing.md`): investigación barata (parseo, `git blame`,
  resolución de rama/spec) en el main loop; refinación → **Haiku**; validación → **Sonnet**;
  composición en el main loop. Documentar el tier por paso en el command.
- **Agnosticism** (`product/principles.md`): no asumir git provider, GitHub, Jira ni layout del
  repo. Si la deducción no resuelve, preguntar; nunca hardcodear.
- **Idioma**: el reporte/brief siguen `config.language` (`.vector/config.json`), con fallback al
  idioma de la conversación; el id del spec, el `kind`/`ref` y los artefactos de git en inglés
  kebab-case (`workflows/git-convention.md`, `standards/naming.md`).
- **Agentes embebidos** (`architecture/distribution-packaging.md`): todo agente del kit
  (`vector-bug-refiner` incluido) se vendoriza y embebe en el binario; solo los agentes propios de
  OpenSpec quedan fuera. Por eso el refiner vive en `kit/agents/` y se copia a `assets/`.
- **Vendoring**: command + agente copiados a `cli/internal/scaffold/assets/` vía `//go:generate`
  (`scaffold.go:13`) y embebidos con `//go:embed all:assets` (`scaffold.go:26`).

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Subcomando `vector spec create --title … [--id] [--repo] [--priority] [--status] [--body-file]
      [--ticket] [--json]` — verificado (`cli/cmd/vector/main.go:702,869`).
- [x] Subcomando `vector spec list` (resolución de spec por id/nombre) — verificado (dispatch
      `cli/cmd/vector/main.go:564`, `runSpecList` en `:776`). Hoy imprime texto; el flag `--json`
      se **añade en esta fase** (§2, §6), no es dependencia previa.
- [x] Subcomando `vector spec worklog <id>` (evento aditivo en `activity.jsonl`) — verificado
      (`cli/cmd/vector/standup.go:223`).
- [x] Struct `SpecState` (destino del campo `RelatedTo`) — verificado
      (`cli/internal/state/types.go:79`).
- [x] Proyección `Card` del board (destino del campo `relatedTo`) — verificado
      (`cli/internal/board/board.go:40`).
- [x] Tipos del board en web (`web/src/types/board.ts`) y componente `SpecCard`
      (`web/src/components/SpecCard/SpecCard.tsx`) — verificados.
- [x] Mecanismo de scaffold/vendoring (`//go:generate` + `//go:embed` en
      `cli/internal/scaffold/scaffold.go:13,26`) — verificado.
- [x] Patrón de project command (`kit/commands/vector/raw.md`, `comment.md`) y de agente del kit
      (`kit/agents/vector-spec-refiner.md`, `vector-spec-validator.md`) — verificados.
- [x] Agente `vector-spec-validator` (Sonnet) reutilizable para validar el spec — verificado.
- [x] Skill global `/bug` (`~/.claude/skills/bug/SKILL.md`) como contenido base a agnosticizar —
      verificado.
- [x] `.vector/config.json` presente (`vector init` corrido) — verificado en este repo.

Si alguna dependencia no existe, el command/CLI se detiene con un mensaje accionable. No inventa
contratos, rutas ni subcomandos.

---

## 5. Arquitectura

### Patrón a usar

**Orquestación por project command (espejo bug-framed de `/vector:raw`) + deducción de causa local
+ delegación a refiner Haiku + validación Sonnet + escritura de estado vía el binario.** El command
coordina e infiere; el refiner refina; el validador valida; el binario es el único escritor del
estado (`relatedTo[]`, card, evento). Misma separación que `/vector:raw`.

### Capas afectadas

- presentation (web): **sí** — `web/` muestra `relatedTo` en `SpecCard` (read-only).
- application/orquestación (kit command): **sí** — NUEVO `kit/commands/vector/bug.md`.
- domain/agente (kit): **sí** — NUEVO `kit/agents/vector-bug-refiner.md` (Haiku).
- data/estado (Go): **sí** — `SpecState.RelatedTo`, evento `spec.related`, `Store` (escritura
  serializada), subcomando `vector spec relate` + flag `--related` en `create`, proyección `Card`.
- shared/scaffold: **sí** — assets embebidos regenerados por `go generate`.

### Flujo esperado

1. Usuario ejecuta `/vector:bug "<reporte>" [{spec-id|rama|archivo}]`.
2. **Parseo**: separar el reporte (`RAW_BUG`) del token opcional. Si `RAW_BUG` vacío →
   `AskUserQuestion` y detenerse.
3. **Confirmar repo inicializado**: leer `.vector/config.json` para `specPath`. Si falta, indicar
   correr `vector init` (igual que `/vector:raw`).
4. **Deducir causa (main loop, barato)**:
   - Identificar archivos/símbolos citados en `RAW_BUG`; correr `git blame`/`git log -S`/`--grep`
     para hallar commits sospechosos.
   - Mapear cada commit a **un spec de Vector** (por el nombre del change de OpenSpec / id) o a un
     **ticket** (trailer del commit, p. ej. `ACME-12`).
   - Candidato único + alta confianza → preparar `relatedTo[]` (`source: blame`). Ambiguo / varios
     / baja confianza / sin match → `AskUserQuestion` ofreciendo candidatos + "ninguno" +
     entrada manual (`source: manual`). Nunca adivinar.
5. **Refinar** (Haiku): invocar `vector-bug-refiner` con `RAW_BUG` (+ causas deducidas como
   contexto). Retorna el `BRIEF` estructurado.
6. **Clarificar** ambigüedad bloqueante del brief con `AskUserQuestion` (≤5 por lote); lo no
   resuelto va a "Open questions".
7. **Componer** el spec bug-framed con la plantilla canónica de 20 secciones
   (`.claude/vector/spec-template.md`): el comportamiento esperado/actual y la reproducción del bug
   viven en §8 (criterios de éxito = aceptación del fix) y §11 (edge cases); las causas en §4 y en
   `relatedTo[]`. Derivar `title` (≤8 palabras) e `id` kebab-case con prefijo `fix-`.
8. **Validar** (Sonnet): invocar `vector-spec-validator` con el spec compuesto. PASS → seguir;
   warnings → mostrar y decidir; BLOCK → corregir y revalidar (máx 3 ciclos).
9. **Registrar el card `draft`** vía el binario, pasando el doc por stdin y las relaciones:
   `vector spec create --title … --id fix-<slug> --status draft --related '<json>' --body-file -`.
   El binario escribe el doc en `specPath`, crea el card en `draft`, persiste `relatedTo[]` y
   appendea `spec.related` por cada relación.
10. **Registrar token routing** (todos los flags de `runSpecRoute`, `cli/cmd/vector/route.go:33`):
    `vector spec route <id> --model haiku --baseline opus --task "refine bug" --tokens-in N
    --tokens-out M`, y otra llamada `--model sonnet … --task "validate spec" --tokens-in …
    --tokens-out …`.
11. **Reportar**: id del card, `status: draft`, `specDoc`, relaciones registradas, veredicto del
    validador, y el siguiente paso (`/vector:propose` para crear el OpenSpec `fix-…`).

### Ubicación de archivos nuevos

```txt
kit/commands/vector/bug.md                                  # project command
kit/agents/vector-bug-refiner.md                            # agente refiner (Haiku)
cli/internal/scaffold/assets/commands/vector/bug.md         # copia embebida (generada)
cli/internal/scaffold/assets/agents/vector-bug-refiner.md   # copia embebida (generada)
```

Cambios de código Go en archivos existentes (ver §6). No crear carpetas nuevas: ya existen
`kit/commands/vector/`, `kit/agents/`, `cli/internal/state`, `cli/cmd/vector`.

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/commands/vector/bug.md` | NUEVO | Project command: parseo, deducción de causa, refiner, composición, validación, registro `draft` + `relatedTo[]` | `kit/commands/vector/raw.md`, `comment.md` |
| `kit/agents/vector-bug-refiner.md` | NUEVO | Agente Haiku read-only: refina el reporte en brief estructurado | `kit/agents/vector-spec-refiner.md` |
| `cli/internal/state/types.go` | MODIFICAR | Añadir `RelatedTo []RelatedItem` a `SpecState` + tipo `RelatedItem{Kind,Ref,Source}` + constantes de `kind`/`source` | `Ticket` struct en el mismo archivo (`types.go:113`) |
| `cli/internal/state/store.go` | MODIFICAR | Persistir `RelatedTo` en create; método `RelateSpec(id, item)` serializado; appendear evento `spec.related` | bloque `Ticket` en `store.go:176,214` |
| `cli/internal/state/event.go` (o donde vivan los tipos de evento) | MODIFICAR | Añadir el tipo de evento `spec.related` | tipos de evento existentes (`work.logged`, `agent.routed`) |
| `cli/cmd/vector/main.go` | MODIFICAR | Flag `--related '<json>'` en `spec create`; flag `--json` en `spec list`; subcomando `spec relate <id> --kind --ref [--source]`; usage | dispatch (`main.go:564`), `spec create` (`main.go:702`), `runSpecList` (`main.go:776`) |
| `cli/cmd/vector/ticket.go` | MODIFICAR | Añadir `parseRelatedFlag` (JSON de `--related`) y `parseRelateFlags` (kind/ref/source), análogos a `parseTicketFlag` | `parseTicketFlag` (`ticket.go:292`) |
| `cli/internal/board/board.go` | MODIFICAR | Exponer `RelatedTo` (subset display) en la proyección `Card` | `Ticket` en `Card` (`board.go:40,62`) |
| `web/src/types/board.ts` | MODIFICAR | Tipo `RelatedItem` + campo `relatedTo?: RelatedItem[]` en el spec del board | `ticket?: Ticket` (`board.ts:38`) |
| `web/src/components/SpecCard/SpecCard.tsx` (+ `.module.css`) | MODIFICAR | Mostrar las relaciones (chips read-only) en la card | render de `ticket` en `SpecCard` |
| `cli/internal/scaffold/assets/commands/vector/bug.md` | NUEVO (generado) | Copia embebida del command (`go generate`) | siblings `raw.md`, `comment.md` |
| `cli/internal/scaffold/assets/agents/vector-bug-refiner.md` | NUEVO (generado) | Copia embebida del agente (`go generate`) | siblings en `assets/agents/` |
| `cli/internal/scaffold/scaffold_test.go` | MODIFICAR (si enumera) | Incluir `bug.md` + agente en el set esperado tras vendoring | `cli/internal/scaffold/scaffold_test.go` |
| `cli/internal/state/store_test.go` | MODIFICAR | Tests de `RelatedTo` (create + `RelateSpec` + evento) | tests de `Ticket` (`store_test.go:254`) |
| `docs/plugin-and-commands.md`, `docs/schemas/state-and-activity.md`, `docs/domain-contract.md` | MODIFICAR | Documentar `/vector:bug`, `relatedTo[]`, el evento `spec.related` y `kind/ref/source` | secciones existentes |

### Detalle por archivo

#### `kit/commands/vector/bug.md`

Acción: NUEVO

Debe implementar (frontmatter + cuerpo en pasos, espejo bug-framed de `/vector:raw` + deducción):

- **Frontmatter**: `name: bug`, `description`, `argument-hint: "[bug-report] {spec-id|branch|file}"`,
  `user-invocable: true`, `allowed-tools` (Read, Grep, Glob, `Bash(git *)`, `Bash(vector *)`, Agent,
  AskUserQuestion, Skill).
- **Pasos** según §5: parseo → confirmar init → deducir causa (`git blame`/`log`, mapear a
  spec/ticket, preguntar si ambiguo) → refinar (Haiku) → clarificar → componer 20 secciones →
  validar (Sonnet) → `vector spec create … --status draft --related … --body-file -` →
  `vector spec route` → reporte.
- **Token routing**: documentar por qué refiner=Haiku, validador=Sonnet, orquestación=main loop.
- **Disciplina de estado**: recordar explícitamente que `relatedTo[]` y el card se escriben **solo**
  vía el binario; nunca editar `.vector/` a mano.

No debe incluir: lógica del repo del usuario (pnpm, layout fijo), implementación del fix, creación
del OpenSpec change, posteo en trackers.

#### `kit/agents/vector-bug-refiner.md`

Acción: NUEVO

Debe implementar:

- Tier **Haiku**, **read-only** (Read, Grep, Glob, `Bash(git *)` opcional). No edita ni decide
  implementación.
- Input: `RAW_BUG` + causas deducidas (contexto).
- Output: brief estructurado — problem summary / expected / actual / reproduction steps /
  acceptance criteria / test plan / risks / open questions. Surface ambigüedad; no inventa intent.

No debe: editar archivos, crear el change, ni asumir stack del repo.

#### `cli/internal/state/types.go`

Acción: MODIFICAR

- Añadir a `SpecState`: `RelatedTo []RelatedItem \`json:"relatedTo,omitempty"\``.
- Nuevo tipo: `RelatedItem struct { Kind RelatedKind; Ref string; Source RelatedSource }` con
  `json` tags `kind`/`ref`/`source`.
- Constantes: `RelatedKind ∈ {spec, ticket}`, `RelatedSource ∈ {blame, manual}` (kebab/lowercase).

Restricciones: no romper el JSON existente (`omitempty`, retrocompatible); tipar con structs (sin
`map[string]any`).

#### `cli/internal/state/store.go`

Acción: MODIFICAR

- Persistir `RelatedTo` en `CreateSpec` cuando viene en el request.
- Método `RelateSpec(id string, item RelatedItem) error` serializado por el mutex del `Store`,
  idempotente (no duplicar `{kind,ref}`), que appendea `spec.related`.

Restricciones: escritura atómica; no tocar la máquina de estados (relacionar no cambia `status`).

#### `cli/cmd/vector/main.go`

Acción: MODIFICAR

- `spec create`: flag `--related '<json-array>'` (cada item `{kind,ref,source}`; `source` default
  `manual`). Validar con el parser; relación inválida → error claro, **no** abortar la creación del
  card si el doc es válido (degradar: crear sin relaciones y reportar, análogo al fallback de
  `--ticket` en `/vector:raw`).
- Nuevo subcomando `spec relate <id> --kind spec|ticket --ref <ref> [--source blame|manual]`.
- Actualizar el usage (`main.go:559,869`).

#### `cli/internal/board/board.go` y web

Acción: MODIFICAR

- `Card`: añadir `RelatedTo []RelatedItem` (subset display) en la proyección.
- `web/src/types/board.ts`: `RelatedItem` + `relatedTo?`.
- `SpecCard.tsx`: render read-only de las relaciones (chips), sin edición.

Restricciones: el web no muta `relatedTo[]`; solo lo muestra.

---

## 7. API Contract

> **No aplica como API HTTP de escritura nueva.** `/vector:bug` es un project command; su "contrato"
> es la **CLI** del binario. La API HTTP del board (`/api/board`) gana un campo de **lectura**
> (`relatedTo` en cada card); el stream SSE (`/api/events`) sigue igual.

### Contrato CLI (fuente de verdad de escritura)

- `vector spec create --title … --id fix-<slug> --status draft [--related '<json-array>']
  --body-file - [--json]` → crea el card `draft`, escribe el doc, persiste `relatedTo[]`.
- `vector spec relate <id> --kind spec|ticket --ref <ref> [--source blame|manual] [--json]` →
  añade/gestiona una relación (idempotente), appendea `spec.related`.
- `vector spec list --json` → resolver candidatos de spec para el mapeo de causa (el flag `--json`
  se añade en esta fase; ver §2/§6).
- `vector spec route <id> --model … --baseline … --task … --tokens-in … --tokens-out …` → token
  meter.

Formato de `relatedTo` (item): `{"kind":"spec|ticket","ref":"<spec-id|provider:key>","source":"blame|manual"}`.

### Endpoints involucrados

- `GET /api/board` → ahora incluye `relatedTo` por card (lectura). No se añaden endpoints de
  escritura (`cli/internal/board/server.go` solo gana el campo en la proyección).

No inferir campos adicionales ni cambiar nombres de propiedades existentes.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `/vector:bug "<reporte>"` parsea el reporte; vacío → `AskUserQuestion`, no continúa.
- [ ] La deducción de causa corre `git blame`/`git log`, mapea a spec/ticket, y **pregunta** ante
      ambigüedad/varios/baja confianza; nunca adivina.
- [ ] `vector spec list --json` devuelve los specs en JSON y la deducción resuelve el spec id del
      candidato a partir de esa salida (no parseo frágil de texto).
- [ ] El refiner **Haiku** retorna un brief de las 8 secciones (problem/expected/actual/repro/
      acceptance/test plan/risks/open questions).
- [ ] El validador **Sonnet** (`vector-spec-validator`) emite veredicto; BLOCK se resuelve en ≤3
      ciclos o se reporta sin registrar.
- [ ] `vector spec create … --status draft --related …` crea el card en `draft`, escribe el doc en
      `specPath` y persiste `relatedTo[]` (`kind ∈ {spec,ticket}`, `source ∈ {blame,manual}`).
- [ ] `vector spec relate <id> …` añade una relación idempotente y appendea `spec.related`
      (verificable en `activity.jsonl`).
- [ ] `--related`/`relate` con JSON o `kind`/`ref` inválido → error accionable; en `create` degrada
      (crea sin relaciones) en vez de perder el card.
- [ ] `GET /api/board` devuelve `relatedTo` por card y el `SpecCard` del web lo muestra (read-only).
- [ ] El command termina en `draft` y reporta el siguiente paso (`/vector:propose`).
- [ ] Command + agente quedan embebidos tras `go generate`; `vector init` los siembra en repo limpio.
- [ ] Sin regresiones: specs sin `relatedTo` siguen serializando/leyéndose igual (retrocompatible).

### Tests requeridos

- [ ] `cli/internal/state/store_test.go`: create con `--related`, `RelateSpec` idempotente, evento
      `spec.related`, retrocompatibilidad (spec sin `relatedTo`).
- [ ] `cli/internal/board/board_test.go`: la proyección `Card` incluye `relatedTo`.
- [ ] Parser de `--related`/`relate`: kind/ref/source válidos e inválidos.
- [ ] `cli/internal/scaffold/...`: `bug.md` + `vector-bug-refiner.md` embebidos; `vector init` los
      escribe.
- [ ] Web: render de `relatedTo` en `SpecCard` (test de componente con comportamiento, no snapshot
      vacío).

### Comandos de verificación

```bash
go -C cli generate ./internal/scaffold
go -C cli vet ./...
go -C cli test ./...
# web:
npm --prefix web run typecheck && npm --prefix web run lint && npm --prefix web run build
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

> La feature combina un project command (CLI) y un cambio menor de lectura en el panel web. Las
> subsecciones de formularios/passwords no aplican (no hay formularios).

### CLI (`/vector:bug`)

- Reporte claro en el **idioma configurado** (`config.language`, fallback al idioma de la
  conversación): causa(s) deducida(s), brief refinado, veredicto del validador, id/`draft`/`specDoc`.
- Ambigüedad de causa/rama/spec → `AskUserQuestion` con candidatos (+ "ninguno" + entrada manual).
- Progreso legible por paso (deduciendo causa / refinando / validando / registrando).
- Sin `git` o repo no detectado → mensaje accionable; continuar sin causas si el usuario lo elige.

### Web (`SpecCard`)

- Las relaciones se muestran como **chips read-only** (p. ej. `↳ fix originado por: add-foo`,
  `ACME-12`), sin acciones de edición. Consistentes con el chip de `ticket`.

### Loading / Errores / Navegación

- Loading: líneas de progreso por paso. Errores: accionables (git no disponible, validación BLOCK,
  binario ausente). No dejar la terminal en estado ambiguo.

### Accesibilidad

- CLI: salida estructurada (no solo color) para causas y veredicto.
- Web: los chips de relación con contraste suficiente y `aria-label` describiendo la relación.

---

## 10. Decisiones tomadas

El agente no debe cuestionar ni cambiar estas decisiones (tomadas con el usuario):

- **Lifecycle = draft nativo (como `/vector:raw`)**: `/vector:bug` autora y registra el card en
  `draft`; **no** crea el OpenSpec change. *Por qué:* mantiene el bug en el board y reusa el flujo
  `raw → propose → apply` sin duplicar `/vector:propose`.
- **`relatedTo[]` = campo de estado nuevo en Go** (no solo prosa, no solo evento): persistido en
  `SpecState`, queryable, y surface en board/API. El evento `spec.related` se añade además para la
  timeline. *Por qué:* el usuario quiere un **record consultable** de la causa del bug.
- **`kind` de relación en V1 = `spec` y `ticket`** (Vector spec id o ticket externo). El commit de
  `git blame` es **señal de inferencia**, no un `kind` almacenado. *Por qué:* el usuario eligió
  explícitamente specs y tickets como ítems relacionados.
- **Inferir, luego preguntar**: `git blame`/`git log` deducen la causa; si hay ambigüedad/varios/
  baja confianza/sin match → `AskUserQuestion`. Nunca adivinar. *Por qué:* agnosticism + evitar
  vínculos alucinados.
- **Refiner propio embebido `vector-bug-refiner` (Haiku)**, no reuso del global. *Por qué:* el kit
  debe ser autocontenido/distribuible; **todo agente del kit se embebe salvo los de OpenSpec**
  (`architecture/distribution-packaging.md`).
- **Validación reusa `vector-spec-validator` (Sonnet)**: no se crea un validador nuevo. *Por qué:*
  el spec autorado es un spec de Vector estándar; el validador existente aplica.
- **Token routing**: refiner=Haiku, validador=Sonnet, orquestación/deducción=main loop. *Por qué:*
  `product/token-routing.md` (ruteo al tier más barato capaz).
- **Web read-only para `relatedTo[]`**: el panel solo muestra; toda escritura es por el binario.
  *Por qué:* `architecture/state-model.md` (CLI-owns-writes; el front no posee estado canónico).

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación, no la
implementa.

---

## 11. Edge cases

### Datos inválidos

- Reporte vacío/solo-espacios → `AskUserQuestion` pidiéndolo; no continuar.
- `{spec|rama|archivo}` dado pero ambiguo/inexistente → listar candidatos y preguntar; no adivinar.
- `--related` con JSON malformado o `kind`/`source` fuera del enum → error claro; en `create`
  degradar (crear el card sin relaciones) y reportar; en `relate` rechazar la operación.

### Deducción de causa

- Archivos del reporte inexistentes en el repo → `git blame` falla → reportar; ofrecer continuar
  sin causas.
- Varios commits/specs candidatos → preguntar cuál es la causa primaria (permitir varios o ninguno).
- Commit no mapea a ningún spec/ticket → no inventar una relación; ofrecer registrarla manualmente
  o continuar sin causa.
- `git` ausente o repo no-git → saltar la deducción con aviso; el bug se autora sin `relatedTo[]`.

### Estado / persistencia

- `RelateSpec` con `{kind,ref}` ya presente → no-op idempotente (no duplicar, no doble evento).
- Spec inexistente en `relate` → error accionable; no crear el card implícitamente.
- Escritura concurrente → serializada por el mutex del `Store`; el command no escribe `.vector/`.
- **Re-invocación del command** con el mismo reporte → por diseño registra un **segundo card draft**
  distinto (el command no deduplica reportes; cada corrida es un bug nuevo). El usuario decide
  archivar/cerrar el duplicado. No es invocación concurrente: el CLI corre secuencialmente.

### Timeout

- `git blame`/`git log` que excede un umbral razonable (repo enorme, historial profundo) → cortar,
  reportar "la deducción de causa agotó el tiempo", y ofrecer continuar **sin** `relatedTo[]`; no
  bloquear la autoría del bug.
- `vector spec create`/`relate` que se cuelga → mostrar el error/timeout del binario; no dejar un
  doc huérfano ni el command colgado.

### Superficie HTTP

- Los códigos HTTP (400/401/403/404/409/422/429/500) **no aplican al flujo del command**: la única
  superficie HTTP tocada es `GET /api/board`, que solo gana el campo de **lectura** `relatedTo`
  (cambio server-side); el command no hace requests HTTP.

### Validación / registro

- Validador BLOCK irresoluble en 3 ciclos → surfacear el reporte y **no** registrar.
- `vector spec create` falla → reportar el error; no dejar un doc huérfano sin card.

### Retrocompatibilidad

- Specs existentes sin `relatedTo` → leen/serializan igual (`omitempty`); board/web no rompen.

---

## 12. Estados de UI requeridos

> CLI: secuencias de salida en terminal. Web: estado de presentación de la card.

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | invitación a `/vector:bug "<reporte>"` | ejecutar el command |
| loading | progreso por paso (deduciendo / refinando / validando / registrando) | esperar |
| pending-decision | `AskUserQuestion` (causa candidata, rama/spec, clarificaciones) | elegir / entrada manual |
| success | brief + causas + id/`draft`/`specDoc` + veredicto | seguir con `/vector:propose` |
| error | mensaje accionable (git ausente, BLOCK, binario ausente, create falla) | corregir / reintentar / abortar |
| card (web) | card del bug con chips de `relatedTo` (read-only) | ver la relación; abrir el spec |

`empty`/`disabled`/`offline`: No aplica — herramienta CLI local + panel de lectura; sin modo
offline propio.

---

## 13. Validaciones

### Validaciones de cliente (command + CLI)

| Campo | Regla | Mensaje |
|---|---|---|
| `$ARGUMENTS` (reporte) | no vacío | `el reporte no puede estar vacío; usa /vector:bug "<texto>"` |
| `{spec|rama|archivo}` (opcional) | si se da, único/unambiguo | `"<arg>" es ambiguo; candidatos: …` |
| `--related` item `kind` | ∈ `{spec, ticket}` | `kind inválido %q: permitidos spec,ticket` |
| `--related` item `ref` | no vacío; si `kind=spec`, existe en `vector spec list` | `relación spec "<ref>" no existe; candidatos: …` |
| `--related` item `source` | ∈ `{blame, manual}` (default `manual`) | `source inválido %q: permitidos blame,manual` |

### Validaciones de servidor

No aplica — no hay backend remoto. Las invariantes de dominio (estado/transición, escritura
serializada, retrocompatibilidad del JSON) viven en `cli/internal/state` y se cubren con tests.

---

## 14. Seguridad y permisos

- El refiner Haiku recibe el reporte (puede ser sensible) y accede a git/código read-only; no se le
  pasan secrets ni tokens.
- `relatedTo[]` puede vincular a specs/commits con autoría; es metadato local del board, no se
  postea ni expone externamente.
- No imprimir ni registrar secrets/tokens/PII; `spec.related` guarda `{kind,ref,source}` (refs
  cortos), no vuelca el reporte ni diffs.
- La mutación de estado es local y serializada por el binario; sin auth (binario local) → 401/403
  no aplican.

---

## 15. Observabilidad y logging

- Reusar el mecanismo existente: cada relación deja rastro como evento `spec.related` en
  `activity.jsonl` (vía el binario), de modo que aparece en la timeline/standup.
- El command reporta al usuario la causa deducida, el brief, el veredicto y el registro; sin logging
  propio fuera de eso.
- No registrar: secrets, tokens, PII, ni el reporte/diff completo en los eventos.

---

## 16. i18n / textos visibles

Vector no tiene sistema i18n; los textos del command están en el Markdown. **Idioma del reporte/
brief:** `config.language` (`.vector/config.json`), fallback al idioma de la conversación (misma
política que `vector-standup-writer`/`vector-spec-refiner`). El **id del spec** (`fix-…`), los
`kind`/`ref`/`source` y los artefactos de git en **inglés** kebab-case.

| Key (texto del command) | Texto |
|---|---|
| bug.report.empty | `el reporte no puede estar vacío; usa /vector:bug "<texto>"` |
| bug.cause.deducing | `deduciendo causa (git blame)…` |
| bug.cause.ambiguous | `causa ambigua; elige el trabajo que la originó:` |
| bug.cause.none | `sin causa deducida automáticamente` |
| bug.refining | `refinando el reporte…` |
| bug.validating | `validando el spec…` |
| bug.registered | `bug registrado en draft:` |

Textos visibles del web (`SpecCard`): la relación se renderiza como un chip read-only que **espeja
el patrón del chip de ticket** existente (`web/src/components/SpecCard/SpecCard.tsx:38` —
`<span className={styles.ticket} …>`). La etiqueta vive como key del componente:

| Key (texto del web) | Texto |
|---|---|
| relatedTo.chip.originatedBy | `originado por` |

El `ref` (spec id / `provider:key`) no se traduce.

---

## 17. Performance

- Deducción de causa: `git blame`/`log` son locales; en repos grandes pueden tardar — acotar a los
  archivos del reporte y reportar progreso; no bloquear sin feedback.
- Refiner Haiku: una invocación por corrida; barato.
- Validador Sonnet: una invocación (más reintentos si BLOCK, máx 3).
- Estado: `relatedTo[]` es una lista corta (refs); sin impacto en lectura/escritura del board.
- Web: render de unos pocos chips por card; sin coste perceptible.

---

## 18. Restricciones

El agente no debe:

- Asumir git provider/GitHub/Jira/layout del repo del usuario; si no deduce, preguntar.
- Editar `.vector/` a mano (CLI-owns-writes); toda mutación vía `vector spec …`.
- Crear el OpenSpec change ni implementar el fix (eso es `/vector:propose` y `/vector:apply`).
- Almacenar commits/PRs crudos como `kind` de relación en V1 (señal de inferencia, no estado).
- Permitir que el web escriba `relatedTo[]` (solo lectura).
- Inventar relaciones cuando `git blame` no resuelve; preguntar o continuar sin causa.
- Romper la retrocompatibilidad del JSON de estado (usar `omitempty`).
- Postear el bug en trackers externos.
- Ignorar fallos de `vet`/tests/build (Go y web).

---

## 19. Entregables

Al finalizar deben quedar:

- [ ] `kit/commands/vector/bug.md` (project command bug-framed).
- [ ] `kit/agents/vector-bug-refiner.md` (agente Haiku, read-only).
- [ ] `SpecState.RelatedTo` + `RelatedItem` + evento `spec.related` en `cli/internal/state`.
- [ ] `vector spec create --related` + `vector spec relate` en `cli/cmd/vector` (con parser/validación).
- [ ] `relatedTo` en la proyección `Card` (`cli/internal/board`) y en la API `/api/board`.
- [ ] `relatedTo` mostrado (read-only) en `SpecCard` del web (+ tipos).
- [ ] Copias embebidas en `cli/internal/scaffold/assets/…` vía `go generate`; `vector init` siembra.
- [ ] Tests: estado (`store_test.go`), board (`board_test.go`), parser, scaffold, componente web.
- [ ] `go -C cli vet ./...` + `go -C cli test ./...` + typecheck/lint/build de web en verde.
- [ ] Docs actualizados: `docs/plugin-and-commands.md`, `docs/schemas/state-and-activity.md`,
      `docs/domain-contract.md`.

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `kit/commands/vector/raw.md` y `comment.md` (patrón de command) y
      `~/.claude/skills/bug/SKILL.md` (contenido a agnosticizar).
- [ ] Revisé `kit/agents/vector-spec-refiner.md` (patrón de refiner) y reuso `vector-spec-validator`.
- [ ] Añadí `RelatedTo`/`RelatedItem`/`spec.related` con retrocompatibilidad (`omitempty`).
- [ ] Implementé `--related` (con degradación) y `vector spec relate` (idempotente) en el binario.
- [ ] La deducción de causa infiere y **pregunta** ante ambigüedad; nunca adivina.
- [ ] Mantuve CLI-owns-writes; el web solo muestra `relatedTo[]`.
- [ ] El command termina en `draft` y no crea el OpenSpec change.
- [ ] Refiner=Haiku, validador=Sonnet, orquestación=main loop (token routing documentado).
- [ ] Corrí `go generate` + `go vet` + `go test` (Go) y typecheck/lint/build (web).
- [ ] Verifiqué que `vector init` siembra `bug.md` + `vector-bug-refiner.md`.
- [ ] Actualicé los docs (`plugin-and-commands`, schema, domain-contract).
- [ ] No agregué dependencias externas ni endpoints de escritura.
- [ ] No dejé TODOs sin justificar.

---

## Open questions

- ¿Añadir un `kind: commit` (y/o `pr`) a `relatedTo[]` en una fase futura, para conservar el SHA
  crudo cuando un commit culpable no mapea a ningún spec/ticket? V1 lo deja fuera por decisión
  explícita (señal de inferencia, no estado).
- ¿La deducción de causa debe cachearse (no re-correr `git blame` si ya hay `relatedTo[]`), o
  re-deducir siempre que se invoque? (Sugerido: registrar una vez; `relate` para ajustes.)
- ¿Límite máximo de relaciones por bug (p. ej. top-N commits/specs) para evitar ruido?
- ¿El `SpecCard` debe linkear el chip de relación a la card del spec causante (navegación dentro
  del board) o solo mostrarlo como texto en V1?
- ¿`vector init` debe re-sembrar `vector-bug-refiner` en repos ya inicializados vía `vector update`,
  o solo en `init` nuevos? (Probable: `update` lo cubre por el patrón de re-seed del kit.)
