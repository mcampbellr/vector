# Vector — Estado del proyecto (handoff / contexto de continuación)

> Punto de retome rápido. Última actualización: 2026-06-25 (sesión board web + apply + needsUat +
> standup-digest + release/UAT). Leé esto + `docs/Home.md` al empezar una sesión. El detalle de
> cada decisión está en los docs enlazados.
>
> **Rama abierta:** `feat/board-panel-and-apply` (**no** mergeada a `main` ni pusheada).
> Cubre: board panel (serve+SPA), máquina de estados + transiciones, `/vector:apply`, la feature
> `needsUat`, y la feature **standup-digest** (`work.logged`, `/vector:standup`, StandupView +
> SpecTimeline). Gate verde. Para retomar: `git checkout feat/board-panel-and-apply`.
>
> **Standup-digest:** implementado y verificado. El binario de `~/.local/bin/vector` quedó
> recompilado con la SPA embebida (StandupView) + subcomandos `standup`/`worklog`, y el UAT
> exhaustivo pasó (ver `docs/uat.md`). `add-standup-digest` está en `review`, **listo para
> `/vector:close`**; el cierre (`release-standup-digest`) en `review` tras este apply. El merge de
> la rama a `main` sigue pendiente (paso aparte). Tras tocar comandos/agents: `/reload-plugins`.

## Qué es Vector (en una línea)

CLI Go (`cli/`) + kit de comandos Claude (`kit/`) que organiza specs sobre OpenSpec en un board
kanban. Binario global + comandos/state **per-proyecto**. Agnóstico al repo del usuario, token-eficiente,
comercial día-0. Visión: `docs/vision.md`.

## Cómo correrlo (dogfooding)

- **Binario** en `~/.local/bin/vector` (en PATH). Recompilar tras cambios:
  ```bash
  go -C cli generate ./internal/scaffold/   # sync kit/ → assets embebidos
  go -C cli build -o ~/.local/bin/vector ./cmd/vector
  ```
- **Re-sembrar** los comandos en un repo tras actualizar el binario: `vector update` (preserva
  config/state). En este repo ya están sembrados (`.claude/commands/vector/`).
- **Gate**: `gofmt -l cli && go -C cli vet ./... && go -C cli test -race ./...` + `npm --prefix web run typecheck` (todo verde hoy).
- Tras cambiar comandos/agents: `vector update` + `/reload-plugins` en Claude Code.
- **`vector serve` standalone embebe el board en build-time**: si tocás `web/`, para verlo en el
  binario hay que rebuildear web → copiar al embed → rebuildear el binario (ver `web/README.md`).
  En dev es más rápido `vector serve` + `npm --prefix web run dev` (Vite proxy, hot reload).

## Qué está construido (binario `vector`)

- `vector init` — siembra el motor (`/vector:*` + agents + template) en `.claude/`, crea
  `.vector/config.json` (migra de `.project-structure`), esqueleto de state. Aditivo.
- `vector update` — re-siembra el kit preservando config/state; `kitVersion` stamp.
- `vector sync` — proyecta OpenSpec changes + spec docs al board (idempotente, multi-worktree,
  `supersededBy`, branch=preferencia). Ver `docs/sync-and-dedup.md`.
- `vector spec create|list|propose` — `propose` flipea `draft → open` con provenance OpenSpec.
- **`vector spec apply|status|close|archive|next`** — transiciones sobre la **máquina de estados
  LOCKED** (`internal/state/transition.go`): `apply` (open→in-progress, `startedAt`), `status`
  (genérico validado, resuelve needs-attention), `close`/`archive`, `next` (pick por
  status+prioridad + `applyMode`). Habilita `/vector:apply`.
- **`vector serve`** — panel local: API HTTP read-only (`/api/board`), stream **SSE**
  (`/api/events`), UI embebida; puerto auto (o `--port`), watcher por polling del `.vector/`,
  `--web-dir` para servir el build desde disco en dev. Ver `web/README.md`.
- `vector version`.

**Paquetes Go** (stdlib only): `internal/state` (Store, único escritor, eventos +
`ReadEvents`; máquina de estados + `SpecState.NeedsUAT`), `internal/config` (`.vector/config.json`,
migración, colapso multi-worktree, `applyMode`), `internal/openspec` (lectura de changes/tasks,
`isVerificationTask`), `internal/scaffold` (embed + seed del kit), **`internal/board`** (proyección
read-only + **Token Savings Meter** desde `agent.routed` + `Card.NeedsUAT`; `Server` API+SSE),
**`internal/webui`** (embed de la SPA de `web/`, handler SPA).

**Feature `needsUat`** (refina `review`, change `review-uat-flag`): marcador **derivado** que
distingue el `review` que solo espera UAT manual del review limpio. Lo computa `sync`
(`syncNeedsUAT`, reusa `isVerificationTask`) — **no es estado nuevo** ni cambia la máquina; el
board lo muestra como badge "UAT" (review-gated). Los subcomandos de transición no lo tocan
(sync-derivado). Ver `docs/domain-contract.md` §1.

## Qué está construido (kit)

- Commands: `/vector:raw` (idea → spec 20-secciones validado → card `draft`), `/vector:sync`,
  `/vector:propose`, **`/vector:apply`** (selección por `applyMode` → start → delegate/native →
  implementar → `review`; no auto-commitea). Ver `docs/apply-design.md`.
- Agents: `vector-spec-refiner` (Haiku), `vector-spec-validator` (Sonnet). Template:
  `kit/vector/spec-template.md`.

## Qué está construido (web/ — board panel, slice inicial)

- **React 19 + Vite + TS**, CSS Modules + tokens (`src/styles/tokens.css` de
  `docs/kanban-ui-reference.md`), iconos `lucide-react`. Sin librería de componentes.
- Componentes: `KanbanBoard` (columnas=estado) · `BoardColumn` · `SpecCard` (badge de ahorro
  por-spec + **badge "UAT"** review-gated) · `StatusPill` · `PriorityFlag` · `BoardHeader`
  (frescura + estado SSE) · **`TokenSavingsMeter`** (héroe: total ahorrado, % más barato, barra
  spent/baseline, desglose por modelo). Data vía `useBoard` (SSE live).
- Verificado end-to-end contra `vector serve`: `/api/board` + SSE + UI buildada servida (el
  binario instalado ya embebe el board real, no el placeholder).
- **Meter con datos de muestra**: sembré `agent.routed` representativos en
  `.vector/local/activity.jsonl` (gitignored/local) — no son datos de producción. Falta que el
  kit emita `agent.routed` por ruteo real.

## Decisiones cerradas (LOCKED — leer antes de tocar)

- `docs/domain-contract.md` — estados (`draft·open·in-progress·needs-attention·review·closed·archived`),
  columnas=estado, mapa comando→state (§5), máquina de estados.
- `docs/plugin-and-commands.md` — `/vector:*` son **project commands** (namespace por subdirectorio,
  estilo opsx), NO un plugin. Instalación per-proyecto.
- `docs/sync-and-dedup.md` — identidad=slug, colapso multi-worktree, `supersededBy`, branch=preferencia.
- `docs/state-architecture.md`, `architecture/*` rules — CLI-owns-writes, JSON = fuente de verdad.

## Board actual (de Vector sobre sí mismo)

- `add-propose-command` → **`review`** (UAT) — impl done, solo QA manual `5.3` pendiente; el flag
  `needsUat` se activó vía `sync --reconcile`. Change en `openspec/changes/add-propose-command/`.
- `review-uat-flag` → **`closed`** — la feature `needsUat`, recorrida de punta a punta esta sesión
  (`raw → propose → apply → review → close`). Change en `openspec/changes/review-uat-flag/`.
  El `tasks.md` tiene `6.3` (UAT manual) sin marcar; se cerró aceptándolo.

## Próximo (sugerencias)

- **Mergear/abrir PR** de `feat/board-panel-and-apply` a `main` (8 commits, no pusheado).
- Comandos restantes del contrato como `.md`: `/vector:status`, **`/vector:close`** (lo usé como
  subcomando `vector spec close`, falta el command), `/vector:archive`, `/vector:link`,
  `/vector:daily`. El binario ya soporta todas sus escrituras.
- API HTTP de **escritura** en `vector serve` → habilita **drag-and-drop** del board (mover card =
  `SetStatus`). Hoy las transiciones son solo por CLI.
- Automatizar el copy `web/dist` → embed en el build del binario (hoy es manual; fricción real
  para `vector serve` standalone).

## Pendiente (resto del contrato + producto)

- **Board web**: API de **escritura** (drag-and-drop), rail de iconos + sidebar de proyectos,
  typegen del contrato Go→TS (hoy espejo a mano en `board.ts`).
- `vector init`: detección/reorg del repo + backup/consent (pregunta abierta #3 del vision).
- `install.sh` (instalación día-0) + copy automático del embed — hoy build manual.
- Emisión real de `agent.routed` (hoy el meter consume eventos sembrados a mano; falta que el
  kit los registre por ruteo real).

## Lecciones / gotchas de esta sesión

- **Detección OpenSpec** = "¿el repo es proyecto OpenSpec?" (existe `openspec/` con estructura), **no**
  "¿el CLI `openspec` está en PATH?". (Visto en el bootstrap de propose.)
- En bare+worktrees: changes **activos** viven en worktrees, **archivados** en el árbol root → sync
  lee ambos y colapsa.
- `open` **no** estampa `startedAt` (eso es `in-progress`/apply) — lo atrapó el validator.
- Dedup cross-slug (spec `/idea` ↔ change de otro nombre) NO se infiere por nombre: el command
  `/vector:sync` propone matches por contenido y persiste `supersededBy` en el frontmatter.
- Patrón de trabajo: feature en `cli/`+`kit/` → `go generate` → gate → `vector update` (reseed) →
  commits separados (tool vs dogfood `.claude`/`.vector`).
- **Board web**: contrato = `internal/board` (Go) ↔ `web/src/types/board.ts` (espejo a mano);
  mantenerlos en sync hasta que haya typegen. `web/` se sirve en dev por Vite (proxy `/api`) o
  buildado vía `vector serve --web-dir web/dist`. Embed: placeholder `index.html` committeado +
  `assets/` gitignored; el release copia `web/dist` → `cli/internal/webui/dist`.
- SSE sin deps: el watcher hace **polling** del fingerprint de `.vector/` (count+size+mtime) y
  hace broadcast on-change. Evita fsnotify (regla stdlib-only).
- **`needsUat` es sync-derivado**, NO lo tocan los subcomandos de transición (decisión del spec).
  Una card `closed` puede conservar `needsUat:true` como registro histórico; el badge es
  review-gated en la UI, así que no se muestra. El clear activo ocurre en `ReconcileStatus`/
  `CreateSpec` cuando el status resultante no es `review`.
- **`isVerificationTask`** (`internal/openspec`) clasifica QA/UAT manual: `smoke test`/`e2e` o
  `manual` + (`check`|`qa`|`test`|`verif`). Si un repo escribe "UAT" sin esos tokens, falso
  negativo → no marca `needsUat` (ampliar el wording es open question del change).
- **Validación OpenSpec**: `openspec validate` estricto pide deltas en `specs/`, pero los changes
  de este repo usan la **forma liviana** (proposal/design/tasks, sin deltas) — non-goal explícito
  del adapter de propose. Los artefactos matchean la convención del repo, no la validación estricta.
- **Embed en build-time**: reconstruir el binario desde el source con el placeholder restaurado
  vuelve a embeber el placeholder. Para el board real: build web → copy a `cli/internal/webui/dist/`
  → build binario, y restaurar el placeholder en el working tree (el binario ya quedó embebido).
- Artefactos git en inglés; conversación/docs en español.
