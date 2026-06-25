# Spec: Comando `/vector:propose` (draft → open vía OpenSpec)

## 1. Objetivo

Construir `/vector:propose <id>`: la operación que **formaliza** un spec Vector en estado `draft`
(creado por `/vector:raw`) generando el **change de OpenSpec** (proposal/design/tasks) y
transicionando la card del board a `open` con la provenance (`openspec{change,artifacts}`).

Permite que un dev cierre el gap `draft → open` **sin salir del flujo de Vector**: en repos con
OpenSpec, Vector **orquesta el tooling que el repo ya tiene** (cero tooling nuevo); en repos sin
OpenSpec, Vector usa un **proposer nativo liviano**.

## 2. Alcance

### Incluido en esta fase

- Nuevo **project command** `/vector:propose <id>` (`kit/commands/vector/propose.md`, sembrado por `init`/`update`).
- Nuevo **subcomando de binario** `vector spec propose <id>` que transiciona el state `draft → open` y registra `openspec{change,artifacts}` + eventos.
- **Adapter de generación** con dos modos, decidido por detección:
  - **Delegado (OpenSpec presente):** el command corre la herramienta OpenSpec del repo (`opsx:propose`/`openspec-propose` skill, o el CLI `openspec`) para crear `openspec/changes/<id>/{proposal,design,tasks}.md`.
  - **Nativo liviano (sin OpenSpec):** el command escribe los 3 artefactos desde el spec doc (proposal ← reformat del spec; design/tasks como stubs accionables). Sin modelo de deltas ni catálogo.
- **Ubicación en bare+worktrees:** el command resuelve en qué worktree crear el change; ante ambigüedad **pregunta y persiste** la elección (mismo patrón que sync `--branch`).
- **Idempotencia:** si el change ya existe, **pregunta** (overwrite / keep) en vez de pisar; si la card ya está `open`, reporta sin fallar.

### Fuera de scope

- **Paridad total con OpenSpec** (modelo de deltas `specs/<cap>/`, validación, aplicación al catálogo, archive). El fallback nativo es mínimo; non-goal explícito.
- Implementación del trabajo del change (eso es `/vector:apply`).
- Creación/gestión de git worktrees o branches (el dev maneja su branch-per-spec; Vector solo escribe artefactos donde se le indique).
- Modificar specs que no estén en `draft`.
- Panel web / proyección visual (es read-only del state).

El agente no implementa nada fuera de este alcance aunque parezca relacionado.

## 3. Tecnologías y convenciones del proyecto

### Stack

- Binario: **Go** (módulo único, stdlib; `cli/`). Sin dependencias externas.
- Project command: **Markdown + frontmatter** orquestado por Claude (patrón `kit/commands/vector/raw.md`).

### Versiones relevantes

- Go: `1.26` (citado de `cli/go.mod`).
- OpenSpec: detectado en runtime (CLI `openspec` en PATH y/o skills `opsx:*`/`openspec-*` en `.claude/`); no se pinnea versión — el modo delegado usa lo que el repo tenga.

### Patrones existentes a respetar

- `cli/cmd/vector/main.go` → `runSpec()`: switch de subcomandos `spec *`. Se agrega `case "propose"`.
- `cli/internal/state/Store`: único escritor, serializado por mutex; métodos `CreateSpec`, `ReconcileStatus`, `ReadSpec`, `setStatusTimestamp`. Se agrega `ProposeSpec`.
- `cli/internal/state/types.go`: `SpecState.OpenSpec` (`{Change, Artifacts{Proposal,Design,Tasks}}`) y los estados ya existen — sin migración de schema.
- `cli/internal/config`: `Config` resuelve `specPath`, `changesPath`, `Branch` (bare+worktree). Se reutiliza para ubicar el change.
- Command pattern: el command orquesta y llama al binario; **CLI-owns-writes** (el command nunca edita `.vector/` a mano).
- Git artifacts en inglés kebab-case; el id del spec == nombre del change de OpenSpec.

## 4. Dependencias previas

- [ ] Spec existente en `draft` (`/vector:raw` corrido) — es el input de propose.
- [ ] `.vector/config.json` presente (`specPath`; en bare+worktrees `changesPath`/`branch`).
- [ ] Binario con `internal/state`, `internal/config`, `internal/openspec` (ya existen).
- [ ] Para el modo **delegado**: tooling OpenSpec del repo disponible (CLI `openspec` o skill `opsx:propose`/`openspec-propose`). Si falta, se cae a modo nativo (no es bloqueante).

Si una dependencia no existe, el binario se detiene con mensaje accionable (p. ej. "run `vector init` first"). No inventa contratos.

## 5. Arquitectura

### Patrón

CLI-owns-writes con **adapter** de generación. El command orquesta (detecta modo, genera/deletga artefactos, maneja prompts de ambigüedad e idempotencia) y el binario persiste el state.

### Capas afectadas

- **Project command** (`kit/commands/vector/propose.md`): sí — detección de modo, generación delegada/nativa, prompts, invoca binario.
- **Binario CLI** (`cli/cmd/vector`): sí — `runSpecPropose` (flags, validación, reporte).
- **State** (`cli/internal/state`): sí — `ProposeSpec` (transición + provenance + eventos).
- **Config** (`cli/internal/config`): sí — resolver ubicación del change + nuevo campo `ProposeBranch` para **persistir** la elección de worktree (reusa `Branch` si vacío).
- **web/**: no.

### Flujo esperado

1. Dev ejecuta `/vector:propose <id>`.
2. Command valida que el spec exista y esté en `draft` (vía binario; si no, reporta/pregunta).
3. Command **detecta modo**: ¿hay tooling OpenSpec? (CLI `openspec` o skill `opsx:propose`/`openspec-propose`).
4. Command **resuelve la ubicación** del change (`<worktree>/openspec/changes/<id>/`); si es ambiguo, pregunta y persiste.
5. Si el change ya existe → pregunta (overwrite/keep).
6. **Genera artefactos**: delegado (corre el tooling OpenSpec) o nativo (escribe los 3 desde el spec).
7. Command invoca `vector spec propose <id> --change <id> --artifacts proposal,design,tasks ...`.
8. Binario lee el spec, valida `draft`, escribe `status:open` + `openspec{change,artifacts}` + `updatedAt`, appendea eventos, retorna JSON. **No** estampa `startedAt` (eso es `in-progress`/`/vector:apply`).
9. Command reporta: id, `draft → open`, ubicación del change, artefactos, próximo paso (`/vector:apply`).

### Ubicación de archivos nuevos

- `kit/commands/vector/propose.md` (+ copia sembrada en `.claude/commands/vector/propose.md` y vendored en `cli/internal/scaffold/assets/`).
- Cambios en `cli/cmd/vector/main.go` y `cli/internal/state/store.go`.

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `kit/commands/vector/propose.md` | NUEVO | Project command: detección de modo, generación delegada/nativa, prompts (worktree, overwrite), invoca el binario | `kit/commands/vector/raw.md` |
| `cli/cmd/vector/main.go` | MODIFICAR | `runSpec()` + `case "propose"` → `runSpecPropose()` (flags, validación, reporte JSON/humano) | `cli/cmd/vector/main.go` (`runSpecCreate`) |
| `cli/internal/state/store.go` | MODIFICAR | `ProposeSpec(id, openspec, actor, now)`: transición `draft → open`, provenance, eventos | `cli/internal/state/store.go` (`ReconcileStatus`) |
| `cli/internal/state/event.go` | MODIFICAR | Agregar `EvtSpecProposed = "spec.proposed"` + `ProposedData{Change string, Artifacts ArtifactSet}` | `cli/internal/state/event.go` (`StatusChangedData`) |
| `cli/internal/config/config.go` | MODIFICAR | Persistir la elección de worktree para propose: campo `ProposeBranch string` en `Config` (reusa `Branch` si vacío) | `cli/internal/config/config.go` (`Branch`) |
| `cli/internal/scaffold/assets/commands/vector/propose.md` | NUEVO (generado) | Copia embebida del command (vía `go generate`) | sibling `raw.md`/`sync.md` |

### Detalle por archivo

#### `kit/commands/vector/propose.md` — NUEVO

Debe implementar:
- Leer `$ARGUMENTS` (el `<id>`); validar kebab-case y que la card exista en `draft`.
- Detectar tooling OpenSpec (CLI `openspec` en PATH; skills `opsx:propose`/`openspec-propose`).
- Resolver la ubicación del change (worktree); si ambiguo, `AskUserQuestion` y persistir.
- Modo delegado: invocar el skill/CLI OpenSpec. Modo nativo: escribir `proposal.md` (reformat del spec doc), `design.md` y `tasks.md` (stubs accionables).
- Si el change ya existe: `AskUserQuestion` (overwrite/keep).
- Invocar `vector spec propose <id> …` y reportar.

Seguir como referencia: `raw.md` (estructura, token-routing, steps, reporte), `sync.md` (idempotencia, multi-worktree, prompt+persist).

No incluir: implementación del trabajo, gestión de worktrees/branches, edición directa de `.vector/`.

#### `cli/cmd/vector/main.go` — MODIFICAR

- En `runSpec()` agregar `case "propose": return runSpecPropose(args[1:])`.
- `runSpecPropose`: flags `--repo-root`, `--change <name>` (default = id), `--artifacts proposal,design,tasks`, `--change-dir <abs>` (override de ubicación), `--dry-run`, `--json`.
- Validar id kebab-case; cargar spec; verificar `status==draft`.
- Llamar `store.ProposeSpec(...)`; reportar id, status nuevo, change dir, artefactos.
- Errores claros: not found / already open / invalid id / not draft.

Restricciones: no cambiar `runSpecCreate`/`runSpecList`; escritura serializada vía `Store`; no cambiar el schema JSON existente.

#### `cli/internal/state/store.go` — MODIFICAR

- `ProposeSpec(id string, openspec *OpenSpec, actor string, now time.Time) (*SpecState, error)`:
  - Lock mutex; `ReadSpec(id)`; validar `Status == StatusDraft` (error accionable si no).
  - Set `Status=StatusOpen`, `OpenSpec=openspec`, `UpdatedAt=now`. **No** llamar `setStatusTimestamp`: ese helper no tiene `case StatusOpen` y no debe extenderse — `open` = change creado pero **no iniciado**; `StartedAt` se estampa recién en `in-progress` (`/vector:apply`).
  - `writeSpecFile`; appendear DOS eventos (dentro del mutex, vía `appendEvent`), siguiendo el shape de `ReconcileStatus`:
    - `Event{V:EventVersion, TS:now, Type:EvtSpecProposed, SpecID:id, Repo:spec.Repo, Actor:actor, Data: json(ProposedData{Change, Artifacts})}` — requiere agregar `EvtSpecProposed` y `ProposedData` en `event.go`.
    - `Event{… Type:EvtStatusChanged, Data: json(StatusChangedData{From:StatusDraft, To:StatusOpen, Trigger:"command"})}` (tipo ya existe).
  - Retornar el spec actualizado.

Restricciones: no tocar `CreateSpec`/`ReconcileStatus`/`ReadSpec`; mantener invariante `UpdatedAt`; eventos serializados con la escritura (dentro del mutex).

## 7. API Contract

Sin API surface HTTP — `no aplica`. La interfaz relevante es la **CLI del binario** consumida por el command:

```bash
vector spec propose <id> [--change <name>] [--artifacts proposal,design,tasks] \
  [--change-dir <abs>] [--repo-root <path>] [--dry-run] [--json]
```

Salida `--json` (éxito):
```json
{ "id": "add-foo", "status": "open", "change": "add-foo",
  "artifacts": {"proposal": true, "design": true, "tasks": true},
  "changeDir": "openspec/changes/add-foo", "updatedAt": "2026-..." }
```
Salida humana:
```
proposed spec "add-foo" (status: draft → open)
  change: openspec/changes/add-foo  [proposal, design, tasks]
  next: /vector:apply add-foo
```
Exit: `0` éxito; `1` error (mensaje a stderr).

## 8. Criterios de éxito

- [ ] `vector spec propose <id>` existe, parsea flags y transiciona `draft → open`.
- [ ] `OpenSpec{Change,Artifacts}` poblado correctamente en `state.json`.
- [ ] Dos eventos: `spec.proposed` + `status.changed` (`trigger:command`).
- [ ] El command genera el change vía **delegado** cuando hay OpenSpec, y vía **nativo** cuando no.
- [ ] Idempotencia: proponer una card ya `open` no falla (reporta); change existente → prompt overwrite/keep.
- [ ] Ubicación en bare+worktrees: pregunta+persiste cuando es ambiguo.
- [ ] `--dry-run` no escribe; `--json` parseable.
- [ ] Sin regresiones: `spec create|list`, `sync`, `init`, `update` siguen funcionando.

### Tests requeridos

- [ ] `ProposeSpec` transiciona `draft → open` + provenance (`OpenSpec{Change,Artifacts}`) + `UpdatedAt`. **No** setea `StartedAt`.
- [ ] `ProposeSpec` falla si el spec no está en `draft`.
- [ ] `ProposeSpec` appendea los 2 eventos con los datos correctos.
- [ ] `runSpecPropose` valida flags/id (table-driven).

### Comandos de verificación

```bash
gofmt -l cli && go -C cli vet ./... && go -C cli test ./...
```

## 9. Criterios de UX

Aplica al **command** (no a UI web):

- **Reporte claro:** qué cambió (status), dónde quedó el change, qué artefactos, y el próximo paso (`/vector:apply`).
- **Idempotencia amable:** card ya `open` → "ya está open, nada que hacer" + próximo paso; no error.
- **Errores accionables:** `"spec 'foo' not found; corré /vector:raw primero"`, `"spec 'foo' está en 'closed', no 'draft'"`.
- **Atomicidad:** o la card queda 100% `open` con provenance, o 100% `draft` (sin escritura parcial del state).
- **Prompts solo ante ambigüedad real:** worktree (si hay varios candidatos) y overwrite (si el change existe). La decisión de worktree se **persiste** para no re-preguntar.
- **Navegación:** en éxito, sugerir `/vector:apply <id>`; nunca disparar apply implícitamente.
- **Accesibilidad:** salida legible en texto plano y JSON.

## 10. Decisiones tomadas

- **Adapter, no reimplementación:** delegar a OpenSpec cuando está; fallback nativo liviano cuando no. **Paridad total con OpenSpec = non-goal** (evita divergencia/fricción). *Por qué:* los repos objetivo ya usan OpenSpec; reimplementar el modelo de deltas es caro y diverge.
- **CLI-owns-writes:** el binario es el único escritor del state; el command orquesta.
- **Transición fija `draft → open`:** la formalización ocurre en propose. *Por qué:* lockeado en `domain-contract.md` §5.
- **id == nombre del change.** *Por qué:* contrato de dominio.
- **Eventos `spec.proposed` + `status.changed` (trigger:command).** *Por qué:* contrato §5.
- **Worktree: preguntar+persistir; change existente: preguntar overwrite/keep.** *Por qué:* decisión del usuario; nada de adivinar ni pisar en silencio.

Si el agente ve una alternativa mejor, la reporta como observación; no la implementa.

## 11. Edge cases

- **Datos inválidos / id no kebab:** `"invalid spec id: must be kebab-case"`; no escribe.
- **Spec no encontrado:** `"spec 'foo' not found"`.
- **Status incorrecto** (`open`/`in-progress`/`review`/`closed`/`archived`): reportar el status actual; si ya `open`, idempotente (no error); otros → error accionable.
- **Change ya existe** (`openspec/changes/<id>/`): `AskUserQuestion` overwrite/keep; nunca pisa en silencio.
- **Sin tooling OpenSpec:** cae a modo nativo (no es error).
- **Tooling OpenSpec falla** (exit≠0 al delegar): surfacear stderr; **no** tocar el state (no transicionar si la generación falló).
- **`specDoc` roto** (el puntero del state no existe en disco): en nativo, error `"spec doc not found"`; no generar proposal vacío silenciosamente.
- **Permisos / disco:** error de I/O al crear el change dir → mensaje con contexto; state intacto.
- **Repo no inicializado** (`.vector/config.json` falta): `"run vector init first"`.
- **Bare+worktree sin candidato claro:** preguntar; en no-TTY, error accionable (set en config o `--change-dir`).
- **Concurrencia:** dos propose del mismo id → el mutex serializa; el segundo ve `open` → idempotente.
- **Sin HTTP surface:** este comando es CLI/filesystem; los códigos HTTP (400/401/403/404/409/422/429/500) **no aplican**.

## 12. Estados de UI requeridos

Estados de salida del command/binario (no UI web):

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | `/vector:propose <id>` esperando input | invocar con un id |
| validating | valida spec/draft | esperar |
| generating | genera/deletga artefactos | esperar |
| success | `proposed … draft → open` + change + artefactos + next | ir a `/vector:apply` |
| idempotent | `spec ya open (sin cambios)` | continuar a apply |
| needs-input | prompt de worktree u overwrite | elegir (se persiste) |
| error | not found / wrong status / OpenSpec falló / I/O | reintentar con id o ubicación correctos |
| empty | — (sin specs en `draft` que proponer si se invoca sin id válido) | correr `/vector:raw` primero |
| disabled | No aplica — sin componentes UI interactivos | — |
| offline | No aplica — CLI local-only, sin dependencia de red | — |

## 13. Validaciones

### Validaciones de cliente (command)

| Campo | Regla | Mensaje |
|---|---|---|
| `<id>` | requerido, kebab-case `[a-z0-9-]+` | "id requerido y kebab-case" |
| `<id>` | existe en `.vector/specs/<id>/state.json` | "spec '<id>' not found; corré /vector:raw" |
| `<id>` | status `draft` | "spec '<id>' ya está '<status>', no 'draft'" |

### Validaciones de flags (binario)

| Flag | Regla | Error |
|---|---|---|
| `--change <name>` | kebab-case `[a-z0-9-]+`; default = `<id>` | "invalid --change: must be kebab-case" |
| `--artifacts a,b,c` | subconjunto de `{proposal,design,tasks}`; default los 3 | "invalid --artifacts: allowed proposal,design,tasks" |
| `--change-dir <abs>` | absoluto y **dentro** del repo (`repoRoot`) | "invalid --change-dir: must be an absolute path inside the repo" |

### Validaciones de servidor (binario)

Mismas reglas de id/status que el cliente, enforced en Go: kebab vía `Slug()`/regex; status vía `Status.Valid()` + chequeo `==draft`; errores de I/O atrapados y reportados. El binario es la autoridad final (CLI-owns-writes).

## 14. Seguridad y permisos

- No exponer secrets ni paths internos sensibles en stdout.
- Los artefactos del change son del repo del usuario (no sensibles); respetar `.gitignore`.
- No imprimir el spec doc entero; reportar ids y paths.
- Sin asumir permisos: error con contexto si no se puede crear el change dir.
- El modo delegado ejecuta tooling del repo (`openspec`/skill) — no introduce binarios externos nuevos; si el tooling no está, no se ejecuta nada (fallback nativo).

## 15. Observabilidad y logging

Usar el `activity.jsonl` existente:

- `spec.proposed` — datos `{change, artifacts}`.
- `status.changed` — datos `{from:"draft", to:"open", trigger:"command"}`.

No logear: el spec doc completo, secrets, paths internos irrelevantes. El token-meter (`agent.routed`) es derivado aparte; el modo delegado/nativo puede registrar su ruteo de tier si corresponde.

## 16. i18n / textos visibles

**El proyecto no tiene sistema de i18n.** El binario emite strings en **inglés hardcodeado** (consistente con `spec create`/`sync`/`init`). La tabla siguiente son **identificadores de documentación** de esos strings, no keys de ningún archivo de traducción. El command conversa en el idioma del usuario; el spec body sigue el idioma del proyecto.

| Identificador (doc) | Texto (hardcoded EN) |
|---|---|
| propose.success | `proposed spec "{id}" (status: draft → open)` |
| propose.already_open | `spec "{id}" is already open (no change)` |
| propose.not_found | `spec "{id}" not found` |
| propose.invalid_status | `spec "{id}" is "{status}", not "draft"` |
| propose.next | `next: /vector:apply {id}` |

## 17. Performance

- Creación del change dir = I/O local, típicamente <100ms.
- Mutación del state (lock + read + write + 2 eventos) serializada, <50ms.
- Sin llamadas de red (local-only). El modo delegado depende del tiempo del tooling OpenSpec (fuera del control de Vector).
- Sin I/O redundante: leer spec una vez, escribir artefactos una vez.

## 18. Restricciones

**Permitido y necesario:** actualizar el string de uso de `runSpec()` (`"usage: vector spec <create|list> ..."`) para incluir `propose`, y el `usage()` global. Eso **no** es "cambiar subcomandos existentes".

El agente no debe:
- Cambiar el schema de `SpecState` (los campos ya existen; solo se agrega `EvtSpecProposed`/`ProposedData` en `event.go` y `ProposeBranch` en `config.go`).
- Transicionar specs que no estén en `draft`.
- Crear el change fuera de `openspec/changes/`.
- Reimplementar el modelo de deltas/catálogo de OpenSpec (non-goal).
- Instalar dependencias nuevas (Go stdlib).
- Cambiar el entrypoint o subcomandos existentes.
- Pisar un change existente sin confirmación.
- Crear/mover git worktrees o branches.

## 19. Entregables

- [ ] `vector spec propose <id>` implementado.
- [ ] `Store.ProposeSpec` + tests.
- [ ] `runSpecPropose` + tests de flags.
- [ ] `kit/commands/vector/propose.md` (+ vendored en `assets/` vía `go generate`, sembrado por init/update).
- [ ] Eventos `spec.proposed` + `status.changed`.
- [ ] Adapter: delegado (OpenSpec presente) + nativo (ausente).
- [ ] Idempotencia y prompts (worktree, overwrite) persistidos.
- [ ] `--dry-run`/`--json`.
- [ ] Sin regresiones; `gofmt`/`vet`/tests verdes.
- [ ] `domain-contract.md` §5 ya documenta `/vector:propose` (verificar consistencia).

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `docs/domain-contract.md` §5 (fila `/vector:propose`) y `docs/sync-and-dedup.md`.
- [ ] Confirmé que `SpecState.OpenSpec`/`ArtifactSet` ya existen en `types.go`.
- [ ] Seguí los patrones reales (`runSpecCreate`, `ReconcileStatus`, `setStatusTimestamp`, mutex).
- [ ] Solo modifiqué los archivos listados o lo justifiqué.
- [ ] Implementé el adapter (delegado/nativo) y la detección de OpenSpec.
- [ ] Implementé idempotencia + prompts persistidos (worktree, overwrite).
- [ ] Implementé el manejo de cada edge case.
- [ ] No reimplementé el modelo de deltas (non-goal).
- [ ] No cambié decisiones tomadas ni el schema del state.
- [ ] Ejecuté `gofmt`, `go vet`, `go test`.
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar.

## Open questions

- Modo nativo: ¿qué tan "ricos" deben ser `design.md`/`tasks.md` (stubs mínimos vs derivar tasks del spec)? — TBD al implementar; arrancar con stubs accionables.
- Detección de tooling OpenSpec: criterio exacto y orden de preferencia (CLI `openspec` en PATH vs skills `opsx:propose`/`openspec-propose` sembrados) — definir al implementar.
