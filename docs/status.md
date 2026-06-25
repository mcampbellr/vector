# Vector — Estado del proyecto (handoff / contexto de continuación)

> Punto de retome rápido. Última actualización: 2026-06-25 (sesión board web). Leé esto +
> `docs/Home.md` al empezar una sesión. El detalle de cada decisión está en los docs enlazados.

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
- **Gate**: `gofmt -l cli && go -C cli vet ./... && go -C cli test ./...` (todo verde hoy).
- Tras cambiar comandos/agents: `vector update` + `/reload-plugins` en Claude Code.

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
`ReadEvents`), `internal/config` (`.vector/config.json`, migración, colapso multi-worktree),
`internal/openspec` (lectura de changes/tasks), `internal/scaffold` (embed + seed del kit),
**`internal/board`** (proyección read-only + **Token Savings Meter** desde `agent.routed`;
`Server` API+SSE), **`internal/webui`** (embed de la SPA de `web/`, handler SPA).

## Qué está construido (kit)

- Commands: `/vector:raw` (idea → spec 20-secciones validado → card `draft`), `/vector:sync`,
  `/vector:propose`, **`/vector:apply`** (selección por `applyMode` → start → delegate/native →
  implementar → `review`; no auto-commitea). Ver `docs/apply-design.md`.
- Agents: `vector-spec-refiner` (Haiku), `vector-spec-validator` (Sonnet). Template:
  `kit/vector/spec-template.md`.

## Qué está construido (web/ — board panel, slice inicial)

- **React 19 + Vite + TS**, CSS Modules + tokens (`src/styles/tokens.css` de
  `docs/kanban-ui-reference.md`), iconos `lucide-react`. Sin librería de componentes.
- Componentes: `KanbanBoard` (columnas=estado) · `BoardColumn` · `SpecCard` (con badge de
  ahorro por-spec) · `StatusPill` · `PriorityFlag` · `BoardHeader` (frescura + estado SSE) ·
  **`TokenSavingsMeter`** (héroe: total ahorrado, % más barato, barra spent/baseline, desglose
  por modelo). Data vía `useBoard` (SSE live).
- Verificado end-to-end contra `vector serve`: `/api/board` + SSE + UI buildada servida.
- **Falta para "verlo" lleno**: el board real tiene 1 spec (`add-propose-command`). Para el
  meter sembré `agent.routed` de muestra en `.vector/local/activity.jsonl` (gitignored/local) —
  no son datos de producción, son representativos del ruteo de la sesión.

## Decisiones cerradas (LOCKED — leer antes de tocar)

- `docs/domain-contract.md` — estados (`draft·open·in-progress·needs-attention·review·closed·archived`),
  columnas=estado, mapa comando→state (§5), máquina de estados.
- `docs/plugin-and-commands.md` — `/vector:*` son **project commands** (namespace por subdirectorio,
  estilo opsx), NO un plugin. Instalación per-proyecto.
- `docs/sync-and-dedup.md` — identidad=slug, colapso multi-worktree, `supersededBy`, branch=preferencia.
- `docs/state-architecture.md`, `architecture/*` rules — CLI-owns-writes, JSON = fuente de verdad.

## Board actual (de Vector sobre sí mismo)

- `add-propose-command` → **`open`** (el spec de `/vector:propose`, ya implementado y self-proposed).
  El subcomando de transición ya existe (`vector spec apply/status`); falta correr el flujo para
  moverlo a `review`. `vector spec next` lo recomienda como pick. Change en
  `openspec/changes/add-propose-command/`.

## Próximo (sugerencias)

- **Dogfood `/vector:apply`** sobre `add-propose-command` para llevarlo a `review` (cierra el
  loop del board real). Hoy está implementado pero no ejecutado sobre el spec real.
- Comandos restantes del contrato: `/vector:status`, `/vector:close`, `/vector:archive`,
  `/vector:link`, `/vector:daily` (el binario ya soporta sus escrituras; faltan los `.md`).
- API HTTP de escritura en `vector serve` → habilita **drag-and-drop** del board (mover card =
  `SetStatus`). Hoy las transiciones son solo por CLI.

## Pendiente (resto del contrato + producto)

- Comandos: `/vector:apply` (siguiente), `/vector:link`, `/vector:status`, `/vector:close`,
  `/vector:archive`, `/vector:daily`. Binario: subcomandos de transición de estado.
- **Board web**: API de **escritura** (mover/cerrar/archivar → habilita drag-and-drop), rail de
  iconos + sidebar de proyectos, typegen del contrato Go→TS (hoy espejo a mano en `board.ts`),
  copia de `web/dist` al embed en el pipeline de release.
- `vector init`: detección/reorg del repo + backup/consent (pregunta abierta #3 del vision).
- `install.sh` (instalación día-0) — hoy build manual.
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
- Artefactos git en inglés; conversación/docs en español.
