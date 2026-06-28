# Spec: Comando /vector:quick para cambios pequeños aplicados en la misma corrida

## 1. Objetivo

Construir `/vector:quick`: un **project command** del kit que toma la descripción de un cambio
pequeño de bajo riesgo (refactor, rename de símbolo, helper extraído, copy ajustado, índice
faltante, archivo promovido), lo **refina** con un subagente Haiku (`vector-quick-refiner`,
espejo agnosticizado del `quick-win-refiner` global), **lo aplica en la misma corrida**, lo
valida con el gate del repo, **registra el trabajo en el board** (record para el daily) y lo
deja listo para review — todo sin la ceremonia de `/vector:raw` → `/vector:propose` →
`/vector:apply` y sin crear un OpenSpec change. Es el equivalente Vector-nativo de `/quick-win`.

Esta feature permite que un developer **despache un cambio mecánico pequeño en una sola corrida**
dejando rastro en el board: el card nace en `in-progress` **marcado como quick-win**, se aplica
el cambio, se registra el trabajo (`work.logged`, visible en el standup), y el card pasa a
`review` para que el usuario lo cierre con `/vector:close`. El commit es opcional y se pregunta
en cada corrida.

## 2. Alcance

### Incluido en esta fase

- **Project command `/vector:quick`** (`kit/commands/vector/quick.md`): parseo de la descripción
  + arg opcional `{ticket|spec-id}`, sanity-check de "esto es un quick-win" (vs `/vector:raw`,
  `/vector:bug`), delegación al refiner Haiku, scope-guard, registro del card `in-progress`
  marcado quick-win vía el binario, **implementación directa del cambio**, validación con el gate
  del repo, registro del trabajo (`worklog`), commit opcional (preguntando), y transición a
  `review`.
- **Agente refiner `vector-quick-refiner`** (`kit/agents/vector-quick-refiner.md`), tier **Haiku**,
  read-only: refina la descripción cruda en un **brief ligero** (title / slug / change-type /
  what-changes / why / files-to-touch / acceptance / risks / blocking-questions / notes),
  agnosticizado del global. Surface ambigüedad solo cuando cambiaría el diff.
- **Marcador de estado `quickWin`** (bool) en `SpecState` (`cli/internal/state/types.go`): persiste
  que el card es un quick-win. Se siembra al crear (`vector spec create … --quick-win`), se expone
  en la proyección del board (API) y **se muestra como badge en el card del panel web**.
- **Lifecycle aplicado-en-corrida**: el card se crea directo en `in-progress` (semilla de status),
  se implementa el cambio, se registra `worklog` (record para el daily/standup), y se transiciona
  `in-progress → review`. El cierre sigue siendo paso explícito del usuario (`/vector:close`).
- **Link opcional a ticket/spec** (`/vector:quick [text] {ticket|spec-id}`): reusa la detección de
  ticket de `/vector:raw` (`detectTicket`) para sembrar `--ticket`, o resuelve el arg como un spec
  id existente de Vector y registra una relación `--related '[{"kind":"spec",…}]'`. Solo cuando
  resuelve con confianza; ambiguo → preguntar u omitir. **Nunca bloquea la creación por el link.**
- **Commit opcional preguntado**: al terminar la implementación + validación, **preguntar** con
  `AskUserQuestion` si commitear. Sí → Conventional Commit en inglés, atómico, stageando solo los
  archivos tocados. No → dejar el working tree y reportarlo.
- **Vendoring**: command + agente embebidos en `cli/internal/scaffold/assets/` vía `go generate`
  (`//go:generate` en `scaffold.go`) y `//go:embed all:assets`.

### Fuera de scope

- **Crear un OpenSpec change**: `/vector:quick` no propone ni delega a `/vector:propose`; aplica un
  cambio pequeño directo. Si el cambio resulta grande/cross-cutting → **escala** recomendando
  `/vector:raw` (→ `propose` → `apply`), no expande en silencio.
- **Validación con `vector-spec-validator` (Sonnet)**: por decisión, el quick-win usa **brief
  ligero**; la "validación" es el gate de lint/typecheck del repo, no el validador de 20 secciones.
- **Log de quick-wins en una convención de docs del repo** (como hace `/quick-win` con
  `quick-wins.md`/CHANGELOG): en Vector **el board + `activity.jsonl` ES el record** (single source
  of truth, `architecture/state-model.md`); no se escribe un log paralelo en el repo del usuario.
- **Edición de `quickWin` desde el panel web**: el web solo **muestra** el badge (read-only).
- **Auto-cerrar el card**: el card termina en `review`; cerrar es paso explícito (`/vector:close`),
  igual que `/vector:apply`.
- **Cambios de arquitectura, nuevo endpoint/tabla/migración, nueva pantalla**: red flags que ruteen
  a `/vector:raw` o `/vector:bug`.
- **Taxonomía unificada de tipo de card** (`feature|bug|quick-win` como enum): V1 usa un bool
  `quickWin`; un `kind` enum queda como Open question.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- **Project command**: Markdown + frontmatter orquestado por Claude (patrón
  `kit/commands/vector/raw.md`, `apply.md`, `comment.md`).
- **Agente refiner**: Markdown del kit (`kit/agents/vector-quick-refiner.md`), tier **Haiku**
  (token-routing: refinar un cambio pequeño en un brief es trabajo estructurado, no razonamiento
  caro). Patrón: `kit/agents/vector-spec-refiner.md` y el global `quick-win-refiner`.
- **Sin validador Sonnet**: a diferencia de `/vector:raw`, no se invoca `vector-spec-validator`.
- **Estado (Go)**: `cli/internal/state` (struct `SpecState`, `Store`, `CreateSpecParams`) — se añade
  el bool `QuickWin`. `cli/cmd/vector` — se añade el flag `--quick-win` a `vector spec create`.
- **Board/web**: `cli/internal/board` (proyección `Card`) expone `quickWin`; `web/` (tipos +
  `SpecCard`) lo muestra como badge.
- **Implementación del cambio**: Read/Edit/Write en el main loop, estricto al brief.
- **Link ticket/spec**: se reusa `detectTicket` (`cli/cmd/vector/main.go`), `--ticket` y `--related`
  (`vector spec create`) ya existentes; no se crea lógica de linking nueva.

### Versiones relevantes

- Go: `1.26` (de `cli/go.mod`).
- No se introducen dependencias externas nuevas (stdlib Go; `git` del sistema).

### Patrones existentes a respetar

- **CLI-owns-writes**: el command **nunca** edita `.vector/` a mano; toda mutación de estado
  (creación del card, `quickWin`, ticket/related, worklog, transición) pasa por el binario
  (`workflows/state-sync-discipline.md`, `architecture/state-model.md`).
- **El binario es el único escritor del spec doc**: el command autora el brief ligero y lo pasa por
  stdin/path a `vector spec create --body-file`; el binario lo escribe en `specPath` (como `raw`).
- **Token routing** (`product/token-routing.md`): sanity-check, parseo, resolución de link e
  **implementación** en el main loop; refinación → **Haiku**; sin validador. Documentar el tier por
  paso en el command.
- **Agnosticism** (`product/principles.md`): no asumir stack, package manager ni layout del repo;
  el gate de validación se detecta (typescript/go/python/rust). Si algo no resuelve, preguntar.
- **Idioma**: descripción/brief siguen `config.language` (`.vector/config.json`), fallback al idioma
  de la conversación; el id del spec, el commit y los artefactos de git en inglés kebab-case
  (`workflows/git-convention.md`, `standards/naming.md`).
- **Agentes embebidos** (`architecture/distribution-packaging.md`): todo agente del kit
  (`vector-quick-refiner` incluido) se vendoriza y embebe; solo los de OpenSpec quedan fuera.
- **Vendoring**: command + agente copiados a `cli/internal/scaffold/assets/` vía `//go:generate` y
  embebidos con `//go:embed all:assets`.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Subcomando `vector spec create --title … [--id] [--repo] [--priority] [--status] [--body-file]
      [--ticket] [--related] [--json]` — verificado (`cli/cmd/vector/main.go:711,948`).
- [x] Semilla de status arbitrario al crear (`CreateSpecParams.Status`) — verificado: los tests
      siembran `StatusOpen`/`StatusReview` directo (`cli/internal/state/store_test.go:122,144`); se
      usará `in-progress` como semilla.
- [x] Transición `in-progress → review` legal — verificada en la máquina LOCKED
      (`cli/internal/state/transition.go:18`).
- [x] Subcomando `vector spec status <id> <target>` (transición genérica) — verificado.
- [x] Subcomando `vector spec worklog <id> --files --tasks --note` (evento aditivo `work.logged`)
      — verificado (`cli/cmd/vector/standup.go`).
- [x] `detectTicket` + `--ticket` + `--related` (linking reutilizable) — verificados
      (`cli/cmd/vector/main.go:331,717,778`; `vector spec relate` en `:952`).
- [x] Struct `SpecState` + `CreateSpecParams` (destino del bool `QuickWin`) — verificado
      (`cli/internal/state/types.go`, `store.go`).
- [x] Proyección `Card` del board (destino de `quickWin`) — verificado (`cli/internal/board/board.go`).
- [x] Tipos del board en web (`web/src/types/board.ts`) y componente `SpecCard`
      (`web/src/components/SpecCard/SpecCard.tsx`) — verificados.
- [x] Mecanismo de scaffold/vendoring (`//go:generate` + `//go:embed` en
      `cli/internal/scaffold/scaffold.go`) — verificado.
- [x] Patrón de project command (`kit/commands/vector/raw.md`, `apply.md`) y de agente
      (`kit/agents/vector-spec-refiner.md`) — verificados.
- [x] Skill global `/quick-win` (`~/.claude/skills/quick-win/SKILL.md`) y su `quick-win-refiner`
      como contenido base a agnosticizar — verificados.
- [x] `.vector/config.json` presente (`vector init` corrido) — verificado en este repo.

Si alguna dependencia no existe, el command/CLI se detiene con un mensaje accionable. No inventa
contratos, rutas ni subcomandos.

---

## 5. Arquitectura

### Patrón a usar

**Orquestación por project command (espejo Vector-nativo de `/quick-win`) + delegación a refiner
Haiku + implementación directa en el main loop + escritura de estado vía el binario.** El command
coordina, refina (Haiku), implementa, valida con el gate del repo, registra el trabajo y
transiciona; el binario es el único escritor del estado (card, `quickWin`, ticket/related, worklog,
status). **No hay validador Sonnet ni OpenSpec change.**

### Capas afectadas

- presentation (web): **sí** — `web/` muestra el badge `quickWin` en `SpecCard` (read-only).
- application/orquestación (kit command): **sí** — NUEVO `kit/commands/vector/quick.md`.
- domain/agente (kit): **sí** — NUEVO `kit/agents/vector-quick-refiner.md` (Haiku).
- data/estado (Go): **sí** — `SpecState.QuickWin` + `CreateSpecParams.QuickWin`, flag `--quick-win`,
  proyección `Card`.
- shared/scaffold: **sí** — assets embebidos regenerados por `go generate`.

### Flujo esperado

1. Usuario ejecuta `/vector:quick "<descripción>" [{ticket|spec-id}]`.
2. **Parseo**: separar la descripción (`RAW_QW`) del arg opcional. `RAW_QW` vacío → `AskUserQuestion`
   y detenerse.
3. **Confirmar repo inicializado**: leer `.vector/config.json` para `specPath`. Si falta, indicar
   correr `vector init` (igual que `/vector:raw`).
4. **Sanity-check (es quick-win)**: detectar red flags en `RAW_QW` — nueva pantalla/página/modal →
   `/vector:raw`; "roto"/"no funciona"/"regresión" → `/vector:bug`; varios cambios no relacionados
   ("y también") → pedir partir/elegir uno; schema/migración/nuevo endpoint → `/vector:raw` (salvo
   que sea literalmente un índice de una línea). Si hay red flag → recomendar el comando y detener;
   no invocar el refiner.
5. **Refinar** (Haiku): invocar `vector-quick-refiner` con `RAW_QW`. Retorna el **brief ligero**.
   **Scope-guard**: si el brief lista >~6 archivos, o Risks incluye cambios de comportamiento
   visibles, o hay >3 blocking questions → el cambio es muy grande: recomendar `/vector:raw` y
   detener.
6. **Clarificar** ≤3 blocking questions con `AskUserQuestion`; plegar respuestas al brief.
7. **Resolver link opcional** (si vino arg, o si `detectTicket` resuelve sobre `RAW_QW`):
   - Ticket (URL/shorthand/cue/prefijo, misma lógica que `/vector:raw`) → preparar `--ticket`.
   - Si el arg coincide con un spec id existente (`vector spec list --json`) → preparar
     `--related '[{"kind":"spec","ref":"<id>","source":"manual"}]'`.
   - Ambiguo / no resuelve → preguntar u omitir. Nunca adivinar; el link nunca bloquea la creación.
8. **Registrar el card `in-progress` + quick-win** vía el binario, pasando el brief como doc:
   `vector spec create --title … --id <slug> --status in-progress --quick-win [--ticket …]
   [--related …] --body-file -`. El binario escribe el doc en `specPath`, crea el card en
   `in-progress`, marca `quickWin`, y siembra ticket/related si vinieron.
9. **Implementar el cambio** (main loop) con Read/Edit/Write, **estricto** a `files-to-touch` del
   brief. Sin abstracciones nuevas, sin reformateos, sin drive-by fixes. Si al editar el cambio
   resulta mayor que el brief → revertir lo tocado (`git restore` solo esos archivos), poner
   `vector spec status <id> needs-attention --reason "out of scope: usar /vector:raw"` y detener.
10. **Validar** con el gate **mínimo** del stack detectado (typescript→typecheck; go→`go vet`;
    python→`ruff`/`mypy`; rust→`cargo check`), acotado a lo tocado. Regenerar artefactos hermanos si
    el repo lo exige (leer `CLAUDE.md`/`AGENTS.md`). **No** correr toda la suite. Falla → arreglar o
    revertir y reportar.
11. **Registrar el trabajo** (record para el daily): `vector spec worklog <id> --files <…>
    --tasks <…> --note "<…>"` → evento aditivo `work.logged`.
12. **Commit (preguntando)**: `AskUserQuestion` "¿commitear este quick-win?". Sí → Conventional
    Commit en inglés (`<type>(<scope>): <summary>`, footer `Co-Authored-By`), stageando **solo** los
    archivos tocados + el doc; sin `--no-verify`, sin `--amend`. No → dejar el working tree y
    reportar que hay cambios sin commitear.
13. **Transición final**: `vector spec status <id> review` (in-progress → review). El cierre lo hace
    el usuario (`/vector:close`).
14. **Token routing**: `vector spec route <id> --model haiku --baseline opus --task "refine
    quick-win" --tokens-in N --tokens-out M` (solo el refiner se rutea; no hay validador).
15. **Summary post-acción** (reusa el pipeline existente, como `/vector:apply` §7): `vector spec
    summarize <id> --json` → `vector-summary-writer` (Haiku) → `vector spec summarize <id> commit`.
16. **Reportar**: id, `quickWin`, transición `in-progress → review`, ticket/related si hubo, SHA del
    commit o "sin commitear", resultado del gate, y el siguiente paso (`/vector:close <id>`).

### Ubicación de archivos nuevos

```txt
kit/commands/vector/quick.md                                  # project command
kit/agents/vector-quick-refiner.md                            # agente refiner (Haiku)
cli/internal/scaffold/assets/commands/vector/quick.md         # copia embebida (generada)
cli/internal/scaffold/assets/agents/vector-quick-refiner.md   # copia embebida (generada)
```

Cambios de código Go/TS en archivos existentes (ver §6). No crear carpetas nuevas: ya existen
`kit/commands/vector/`, `kit/agents/`, `cli/internal/state`, `cli/cmd/vector`, `cli/internal/board`.

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/commands/vector/quick.md` | NUEVO | Project command: parseo, sanity-check, refiner, scope-guard, registro `in-progress`+quick-win, implementación, validación, worklog, commit opcional, transición a review | `kit/commands/vector/raw.md`, `apply.md` |
| `kit/agents/vector-quick-refiner.md` | NUEVO | Agente Haiku read-only: refina en brief ligero | `kit/agents/vector-spec-refiner.md`, global `quick-win-refiner` |
| `cli/internal/state/types.go` | MODIFICAR | Añadir `QuickWin bool \`json:"quickWin,omitempty"\`` a `SpecState` (junto a `NeedsUAT`) | campo `NeedsUAT` en el mismo struct |
| `cli/internal/state/store.go` | MODIFICAR | Añadir `QuickWin` a `CreateSpecParams` y persistirlo en `CreateSpec` | manejo de `NeedsUAT`/`Ticket` en create |
| `cli/cmd/vector/main.go` | MODIFICAR | Flag `--quick-win` (bool) en `spec create`; pasarlo a `CreateSpecParams`; usage | `--ticket`/`--related` en `spec create` (`main.go:717,948`) |
| `cli/internal/board/board.go` | MODIFICAR | Exponer `QuickWin` en la proyección `Card` | `NeedsUAT`/`RelatedTo` en `Card` |
| `web/src/types/board.ts` | MODIFICAR | Añadir `quickWin?: boolean` al spec del board | `needsUat?`/`relatedTo?` en el mismo tipo |
| `web/src/components/SpecCard/SpecCard.tsx` (+ `.module.css`) | MODIFICAR | Renderizar badge "Quick Win" cuando `quickWin` | badge/pill existente (status pill / ticket chip) |
| `cli/internal/scaffold/assets/commands/vector/quick.md` | NUEVO (generado) | Copia embebida del command (`go generate`) | siblings `raw.md`, `apply.md` |
| `cli/internal/scaffold/assets/agents/vector-quick-refiner.md` | NUEVO (generado) | Copia embebida del agente (`go generate`) | siblings en `assets/agents/` |
| `cli/internal/scaffold/scaffold_test.go` | MODIFICAR (si enumera) | Incluir `quick.md` + agente en el set esperado tras vendoring | set esperado actual |
| `cli/internal/state/store_test.go` | MODIFICAR | Test de `QuickWin` en create + round-trip JSON | tests de `Ticket`/`NeedsUAT` |
| `cli/internal/board/board_test.go` | MODIFICAR | La proyección `Card` incluye `quickWin` | tests de proyección existentes |
| `docs/plugin-and-commands.md`, `docs/schemas/state-and-activity.md`, `docs/domain-contract.md` | MODIFICAR | Documentar `/vector:quick`, el campo `quickWin`, el lifecycle aplicado-en-corrida y el link opcional | secciones existentes |

### Detalle por archivo

#### `kit/commands/vector/quick.md`

Acción: NUEVO

Debe implementar (frontmatter + cuerpo en pasos, espejo Vector-nativo de `/quick-win`):

- **Frontmatter**: `name: quick`, `description`, `argument-hint: "[quick-win-description] {ticket|spec-id}"`,
  `user-invocable: true`, `allowed-tools` (Read, Write, Edit, Grep, Glob, `Bash(git *)`,
  `Bash(vector *)` + gates de stack, Agent, AskUserQuestion).
- **Pasos** según §5: parseo → confirmar init → sanity-check → refinar (Haiku) + scope-guard →
  clarificar → resolver link → crear card `in-progress`+quick-win → implementar → validar →
  worklog → commit (preguntando) → transición a review → route → summary → reporte.
- **Token routing**: documentar refiner=Haiku, sin validador, orquestación/implementación=main loop.
- **Disciplina de estado**: recordar que card/`quickWin`/ticket/worklog/status se escriben **solo**
  vía el binario; nunca editar `.vector/` a mano.

No debe incluir: creación de OpenSpec change, validador Sonnet, log de docs paralelo, asunciones de
stack/layout del repo.

#### `kit/agents/vector-quick-refiner.md`

Acción: NUEVO

Debe implementar:

- Tier **Haiku**, **read-only** (Read, Grep, Glob). No edita ni corre shell.
- Input: `RAW_QW`. Output: brief ligero — Optimized Title / Kebab-case Slug / Change Type /
  What Changes / Why / Files to Touch / Acceptance / Risks / Blocking Clarifying Questions /
  Non-Blocking Notes. Preserva el idioma; terse; surface ambigüedad solo si cambia el diff.

No debe: editar archivos, asumir stack, ni proponer cambios fuera del quick-win.

#### `cli/internal/state/types.go` y `store.go`

Acción: MODIFICAR

- `SpecState`: añadir `QuickWin bool \`json:"quickWin,omitempty"\`` (junto a `NeedsUAT`).
- `CreateSpecParams`: añadir `QuickWin bool`; `CreateSpec` lo persiste en el `SpecState` inicial.

Restricciones: retrocompatible (`omitempty`); no tocar la máquina de estados; tipar con bool, no
mapas genéricos.

#### `cli/cmd/vector/main.go`

Acción: MODIFICAR

- `spec create`: flag `--quick-win` (bool, default false) → `CreateSpecParams.QuickWin`.
- Actualizar el usage de `spec create`.

#### `cli/internal/board/board.go` y web

Acción: MODIFICAR

- `Card`: añadir `QuickWin bool` (subset display) en la proyección.
- `web/src/types/board.ts`: `quickWin?: boolean`.
- `SpecCard.tsx` (+ `.module.css`): badge "Quick Win" read-only cuando `quickWin` es true.

Restricciones: el web no muta `quickWin`; solo lo muestra.

---

## 7. API Contract

> **No aplica como API HTTP de escritura nueva.** `/vector:quick` es un project command; su contrato
> de escritura es la **CLI** del binario. La API HTTP del board (`/api/board`) gana un campo de
> **lectura** (`quickWin` por card); el stream SSE (`/api/events`) sigue igual.

### Contrato CLI (fuente de verdad de escritura)

- `vector spec create --title … --id <slug> --status in-progress --quick-win [--ticket '<json>']
  [--related '<json-array>'] --body-file -` → crea el card `in-progress`, marca `quickWin`, escribe
  el doc, siembra ticket/related.
- `vector spec worklog <id> --files … --tasks … --note …` → registra el trabajo (`work.logged`).
- `vector spec status <id> review` → transición `in-progress → review`.
- `vector spec list --json` → resolver el arg como spec id para el link.
- `vector spec route <id> --model haiku --baseline opus --task … --tokens-in … --tokens-out …`.
- `vector spec summarize <id> [--json|commit …]` → summary post-acción (reuso).

### Endpoints involucrados

- `GET /api/board` → ahora incluye `quickWin` por card (lectura). No se añaden endpoints de
  escritura.

No inferir campos adicionales ni cambiar nombres de propiedades existentes.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `/vector:quick "<desc>"` parsea; vacío → `AskUserQuestion`, no continúa.
- [ ] El sanity-check rutea a `/vector:raw`/`/vector:bug` ante red flags y no invoca el refiner.
- [ ] El refiner **Haiku** retorna el brief ligero; el scope-guard detiene y recomienda `/vector:raw`
      ante cambios grandes (>~6 archivos, behavior change, >3 blocking qs).
- [ ] `vector spec create … --status in-progress --quick-win` crea el card en `in-progress`,
      persiste `quickWin: true`, escribe el doc en `specPath`, y siembra ticket/related cuando aplica.
- [ ] El arg `{ticket|spec-id}` se resuelve como ticket (vía `detectTicket`) o como relación spec; no
      adivina y **no bloquea** la creación si no resuelve.
- [ ] El cambio se **aplica en la misma corrida**, estricto al brief; crecer fuera de scope → revertir
      lo tocado + `needs-attention` y detener.
- [ ] El gate del repo (typecheck/vet/lint del stack detectado) pasa; no se corre toda la suite.
- [ ] `vector spec worklog` registra el trabajo (`work.logged`, verificable en `activity.jsonl`) —
      el record que alimenta el daily/standup.
- [ ] El commit se **pregunta**: sí → un commit atómico convencional con solo los archivos tocados;
      no → working tree intacto y reportado.
- [ ] El card termina en `review` (`in-progress → review`); cerrar queda para `/vector:close`.
- [ ] `GET /api/board` devuelve `quickWin` por card y el `SpecCard` del web muestra el badge.
- [ ] Command + agente quedan embebidos tras `go generate`; `vector init` los siembra en repo limpio.
- [ ] Sin regresiones: specs sin `quickWin` siguen serializando/leyéndose igual (retrocompatible).

### Tests requeridos

- [ ] `cli/internal/state/store_test.go`: create con `QuickWin`, round-trip JSON, retrocompatibilidad
      (spec sin `quickWin`).
- [ ] `cli/internal/board/board_test.go`: la proyección `Card` incluye `quickWin`.
- [ ] `cli/internal/scaffold/...`: `quick.md` + `vector-quick-refiner.md` embebidos; `vector init`
      los escribe.
- [ ] Web: render del badge "Quick Win" en `SpecCard` cuando `quickWin` (test de componente con
      comportamiento, no snapshot vacío).

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

### CLI (`/vector:quick`)

- Salida clara en el **idioma configurado** (`config.language`, fallback a la conversación): paso de
  refinación, implementación, validación, worklog, decisión de commit, transición.
- Ambigüedad (cambio muy grande, link no resuelto, descripción vacía) → `AskUserQuestion` con opción
  de escalar a `/vector:raw`.
- **Pregunta de commit** explícita en cada corrida (no commitea en silencio ni nunca por defecto).
- Sin `git`/repo no detectado, o gate ausente → mensaje accionable; no dejar la terminal ambigua.

### Web (`SpecCard`)

- Badge **"Quick Win"** read-only en el card (consistente con los pills/chips existentes de status y
  ticket). Sin acciones de edición.

### Loading / Errores / Navegación

- Loading: líneas de progreso por paso. Errores accionables (cambio fuera de scope → revertido +
  needs-attention; gate falla; binario ausente). No dejar el working tree a medias sin avisar.

### Accesibilidad

- CLI: salida estructurada (no solo color) para los pasos y la pregunta de commit.
- Web: el badge con contraste suficiente y `aria-label` que indique "Quick Win".

---

## 10. Decisiones tomadas

El agente no debe cuestionar ni cambiar estas decisiones (tomadas con el usuario):

- **Lifecycle aplicado-en-corrida = `in-progress → review`**: el card nace en `in-progress`
  (marcado quick-win), se aplica el cambio, se registra `worklog` (record para el daily), y pasa a
  `review`. El cierre es paso explícito (`/vector:close`). *Por qué:* respuesta directa del usuario;
  consistente con que `/vector:apply` nunca auto-cierra.
- **Brief ligero, sin validador Sonnet**: el quick-win autora un brief corto (no spec de 20
  secciones) y su "validación" es el gate de lint/typecheck del repo. *Por qué:* preserva la
  ligereza de `/quick-win`; el validador de 20 secciones es desproporcionado.
- **Marcador `quickWin` (bool) en el estado + badge en el card**: persistido en `SpecState`,
  expuesto en board/API, mostrado en `SpecCard`. *Por qué:* el usuario pidió explícitamente que el
  card "se marque" y "muestre que es un quick win".
- **Commit opcional, preguntado en cada corrida**: ni auto-commit (como `/quick-win`) ni nunca
  (como `/vector:apply`), sino `AskUserQuestion`. *Por qué:* respuesta directa del usuario.
- **Link opcional ticket/spec reusando lo existente**: `detectTicket`/`--ticket` para tickets y
  `--related kind=spec` para otro spec id; solo si resuelve, sin bloquear la creación. *Por qué:* el
  usuario lo pidió ("se podría linkear a otro id/ticket… `/vector:quick [text] {ticket}`") y la
  maquinaria ya existe.
- **El board + `activity.jsonl` es el record; sin log de docs paralelo**: a diferencia de
  `/quick-win`, no se escribe `quick-wins.md`/CHANGELOG en el repo del usuario. *Por qué:*
  single source of truth (`architecture/state-model.md`); evita un segundo registro divergente.
- **Refiner propio embebido `vector-quick-refiner` (Haiku)**, no reuso del global. *Por qué:* el kit
  debe ser autocontenido/distribuible; **todo agente del kit se embebe salvo los de OpenSpec**.
- **Escalar, no expandir**: cambio grande/cross-cutting → recomendar `/vector:raw`; nunca crece en
  silencio. *Por qué:* `/quick-win` "Stay small"; mantiene el contrato del comando.
- **Token routing**: refiner=Haiku; sin validador; orquestación/implementación=main loop. *Por qué:*
  `product/token-routing.md`.

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación, no la
implementa.

---

## 11. Edge cases

### Datos inválidos

- Descripción vacía/solo-espacios → `AskUserQuestion` pidiéndola; no continuar.
- Arg `{ticket|spec-id}` ambiguo (no resuelve a ticket ni a spec id existente) → preguntar u omitir
  el link; nunca adivinar; la creación del card no se bloquea.
- Dos refs de ticket distintas / dos spec ids → descartar por ambiguo (misma política que `raw`).

### Scope / sanity

- Red flag (nueva pantalla, "roto", schema/endpoint, varios cambios) → recomendar `/vector:raw` o
  `/vector:bug` y detener; no invocar el refiner.
- Brief con >~6 archivos, behavior change, o >3 blocking qs → escalar a `/vector:raw`, no aplicar.
- El cambio crece **durante la implementación** → `git restore` solo los archivos tocados,
  `vector spec status <id> needs-attention --reason "out of scope: usar /vector:raw"`, detener.

### Validación / commit

- Gate (typecheck/vet/lint) falla → arreglar la causa o revertir y reportar; no saltar hooks.
- Dependencia faltante para validar → indicar el comando exacto y detener (no instalar).
- Usuario rechaza commitear → dejar el working tree y reportar "cambios sin commitear".
- Commit falla (hooks, tree sucio ajeno) → reportar; no `--no-verify`, no `--amend`; el card ya está
  en `review` con el worklog registrado.

### Estado / persistencia

- Semilla `in-progress` al crear → válida (los tests siembran open/review; in-progress es status
  válido). Si el binario rechazara la semilla, surfacear el error; no editar `.vector/` a mano.
- Re-invocación con la misma descripción → por diseño registra **otro** card quick-win (el command
  no deduplica); el usuario archiva/cierra el duplicado. El CLI corre secuencialmente (no concurrente).
- Specs existentes sin `quickWin` → leen/serializan igual (`omitempty`); board/web no rompen.

### Superficie HTTP

- Los códigos HTTP (4xx/5xx) **no aplican** al flujo del command: la única superficie HTTP tocada es
  `GET /api/board`, que solo gana el campo de **lectura** `quickWin`; el command no hace requests HTTP.

### Timeout

- `git`/gate/`vector spec …` que se cuelga → mostrar el error/timeout; no dejar un doc huérfano ni el
  card sin worklog/transición.

---

## 12. Estados de UI requeridos

> CLI: secuencias de salida en terminal. Web: estado de presentación de la card.

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | invitación a `/vector:quick "<desc>" {ticket?}` | ejecutar el command |
| loading | progreso por paso (refinando / implementando / validando / registrando) | esperar |
| pending-decision | `AskUserQuestion` (escalar, clarificar, **commitear sí/no**) | elegir |
| success | brief + id/quickWin + transición `in-progress → review` + SHA o "sin commitear" | seguir con `/vector:close` |
| error | mensaje accionable (out of scope, gate falla, binario ausente) | corregir / reintentar / abortar |
| card (web) | card con badge "Quick Win" (read-only) + status pill | ver el card; abrir el spec |

`empty`/`disabled`/`offline`: No aplica — herramienta CLI local + panel de lectura; sin modo offline.

---

## 13. Validaciones

### Validaciones de cliente (command + CLI)

| Campo | Regla | Mensaje |
|---|---|---|
| `$ARGUMENTS` (descripción) | no vacío | `la descripción no puede estar vacía; usa /vector:quick "<texto>"` |
| alcance del cambio | pequeño/bajo riesgo (heurístico: ≤~6 archivos, sin behavior change) | `cambio muy grande para /vector:quick; usa /vector:raw` |
| `{ticket}` (opcional) | provider inferible (jira/linear/github) | (misma política que `/vector:raw`) |
| `{spec-id}` (opcional) | existe en `vector spec list` | `spec "<id>" no existe; ¿quisiste un ticket?` |
| `--quick-win` | bool | (flag del binario) |

### Validaciones de servidor

No aplica — no hay backend remoto. Las invariantes de dominio (status/transición, escritura
serializada, retrocompatibilidad del JSON) viven en `cli/internal/state` y se cubren con tests.

---

## 14. Seguridad y permisos

- El refiner Haiku recibe la descripción (puede ser sensible) y accede a código read-only; no se le
  pasan secrets ni tokens.
- La implementación toca el **repo del usuario**: estricta al brief, sin drive-by fixes; el commit es
  opcional y preguntado (no muta el historial sin consentimiento).
- No imprimir ni registrar secrets/tokens/PII; el worklog guarda files/tasks/nota corta, no vuelca
  diffs ni payloads.
- La mutación de estado es local y serializada por el binario; sin auth (binario local) → 401/403 no
  aplican.

---

## 15. Observabilidad y logging

- Reusar el mecanismo existente: el trabajo deja rastro como `work.logged` en `activity.jsonl` (vía el
  binario), de modo que aparece en el daily/standup; el card y `quickWin` quedan en `state.json`.
- El routing del refiner deja `agent.routed` (Token Savings Meter).
- El command reporta al usuario el brief, la implementación, el gate, la decisión de commit y la
  transición; sin logging propio fuera de eso.
- No registrar: secrets, tokens, PII, ni el diff completo en los eventos.

---

## 16. i18n / textos visibles

Vector no tiene sistema i18n; los textos del command están en el Markdown. **Idioma de la
descripción/brief:** `config.language` (`.vector/config.json`), fallback al idioma de la conversación
(misma política que `vector-spec-refiner`/`vector-standup-writer`). El **id del spec**, el mensaje de
**commit** y los artefactos de git en **inglés** kebab-case.

| Key (texto del command) | Texto |
|---|---|
| quick.desc.empty | `la descripción no puede estar vacía; usa /vector:quick "<texto>"` |
| quick.scope.tooBig | `cambio muy grande para /vector:quick; usa /vector:raw` |
| quick.refining | `refinando el quick-win…` |
| quick.implementing | `aplicando el cambio…` |
| quick.validating | `validando (gate del repo)…` |
| quick.commit.ask | `¿commitear este quick-win?` |
| quick.done | `quick-win aplicado; card en review:` |

Texto visible del web (`SpecCard`): el badge espeja el patrón de pill/chip existente. La etiqueta vive
como key del componente:

| Key (texto del web) | Texto |
|---|---|
| specCard.badge.quickWin | `Quick Win` |

---

## 17. Performance

- Refiner Haiku: una invocación por corrida; barato.
- Sin validador Sonnet (un modelo caro menos por corrida vs `/vector:raw`).
- Implementación: edits acotados al brief; el costo real es la edición (main loop), por diseño pequeño.
- Estado: `quickWin` es un bool; sin impacto en lectura/escritura del board.
- Web: render de un badge por card; sin coste perceptible.

---

## 18. Restricciones

El agente no debe:

- Asumir stack/package-manager/layout del repo del usuario; detectar el gate o preguntar.
- Editar `.vector/` a mano (CLI-owns-writes); toda mutación vía `vector spec …`.
- Crear un OpenSpec change ni invocar el validador Sonnet.
- Expandir el cambio en silencio: si crece, escalar a `/vector:raw`.
- Commitear sin preguntar, ni usar `--no-verify`/`--amend`, ni `git add -A`/`.`.
- Escribir un log de quick-wins paralelo en el repo del usuario.
- Permitir que el web escriba `quickWin` (solo lectura).
- Inventar links de ticket/spec cuando el arg no resuelve.
- Romper la retrocompatibilidad del JSON de estado (usar `omitempty`).
- Ignorar fallos del gate (typecheck/vet/lint/build) ni de los tests del repo de Vector.

---

## 19. Entregables

Al finalizar deben quedar:

- [ ] `kit/commands/vector/quick.md` (project command).
- [ ] `kit/agents/vector-quick-refiner.md` (agente Haiku, read-only).
- [ ] `SpecState.QuickWin` + `CreateSpecParams.QuickWin` + flag `--quick-win` en `cli/`.
- [ ] `quickWin` en la proyección `Card` (`cli/internal/board`) y en la API `/api/board`.
- [ ] Badge "Quick Win" (read-only) en `SpecCard` del web (+ tipos).
- [ ] Copias embebidas en `cli/internal/scaffold/assets/…` vía `go generate`; `vector init` siembra.
- [ ] Tests: estado (`store_test.go`), board (`board_test.go`), scaffold, componente web.
- [ ] `go -C cli vet ./...` + `go -C cli test ./...` + typecheck/lint/build de web en verde.
- [ ] Docs actualizados: `docs/plugin-and-commands.md`, `docs/schemas/state-and-activity.md`,
      `docs/domain-contract.md`.

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `~/.claude/skills/quick-win/SKILL.md` (contenido a agnosticizar) y su `quick-win-refiner`.
- [ ] Revisé `kit/commands/vector/raw.md` y `apply.md` (patrón de command + lifecycle) y
      `kit/agents/vector-spec-refiner.md` (patrón de refiner).
- [ ] Añadí `QuickWin` (bool) con retrocompatibilidad (`omitempty`) y el flag `--quick-win`.
- [ ] El command crea el card en `in-progress` marcado quick-win, aplica, registra worklog y
      transiciona a `review`.
- [ ] El commit se pregunta en cada corrida; sin auto-commit ni `--no-verify`/`--amend`.
- [ ] El link ticket/spec reusa `detectTicket`/`--ticket`/`--related`; no bloquea la creación.
- [ ] El scope-guard escala a `/vector:raw`; nunca expande en silencio.
- [ ] Mantuve CLI-owns-writes; el web solo muestra el badge.
- [ ] Refiner=Haiku, sin validador; orquestación/implementación=main loop (token routing documentado).
- [ ] Corrí `go generate` + `go vet` + `go test` (Go) y typecheck/lint/build (web).
- [ ] Verifiqué que `vector init` siembra `quick.md` + `vector-quick-refiner.md`.
- [ ] Actualicé los docs (`plugin-and-commands`, schema, domain-contract).
- [ ] No agregué dependencias externas ni endpoints de escritura.
- [ ] No dejé TODOs sin justificar.

---

## Open questions

- ¿Conviene una **taxonomía unificada de tipo de card** (`kind: feature|bug|quick-win`) en vez del
  bool `quickWin`, para que `/vector:bug` y futuros tipos compartan el mismo eje? V1 usa el bool por
  minimalismo; migrar a enum es retrocompatible más adelante.
- ¿El badge del card debe **filtrarse/agruparse** en el board (vista "solo quick-wins") o solo
  mostrarse? V1 solo lo muestra.
- ¿El arg opcional debe permitir **ambos** (ticket *y* spec-id) a la vez, o solo uno? V1 asume uno;
  resolver el más confiado y, si hay dos, preguntar.
- ¿`/vector:quick` debe ofrecer un atajo para **cerrar** directo (`review → closed`) cuando el usuario
  lo pide en la misma corrida, o siempre dejar el cierre a `/vector:close`? V1 deja el cierre explícito.
- ¿El scope-guard de "≤~6 archivos" debe ser configurable en `.vector/config.json` (p. ej.
  `quickWinMaxFiles`) o quedar fijo? V1 lo deja fijo en el command.
