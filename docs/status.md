# Vector — Estado del proyecto (handoff / contexto de continuación)

> Punto de retome rápido. Última actualización: 2026-06-25. Leé esto + `docs/Home.md` al empezar
> una sesión. El detalle de cada decisión está en los docs enlazados.

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
- `vector version`.

**Paquetes Go** (~3k LOC, stdlib only): `internal/state` (Store, único escritor, eventos),
`internal/config` (`.vector/config.json`, migración, colapso multi-worktree), `internal/openspec`
(lectura de changes/tasks), `internal/scaffold` (embed + seed del kit).

## Qué está construido (kit)

- Commands: `/vector:raw` (idea → spec 20-secciones validado → card `draft`), `/vector:sync`,
  `/vector:propose`.
- Agents: `vector-spec-refiner` (Haiku), `vector-spec-validator` (Sonnet). Template:
  `kit/vector/spec-template.md`.

## Decisiones cerradas (LOCKED — leer antes de tocar)

- `docs/domain-contract.md` — estados (`draft·open·in-progress·needs-attention·review·closed·archived`),
  columnas=estado, mapa comando→state (§5), máquina de estados.
- `docs/plugin-and-commands.md` — `/vector:*` son **project commands** (namespace por subdirectorio,
  estilo opsx), NO un plugin. Instalación per-proyecto.
- `docs/sync-and-dedup.md` — identidad=slug, colapso multi-worktree, `supersededBy`, branch=preferencia.
- `docs/state-architecture.md`, `architecture/*` rules — CLI-owns-writes, JSON = fuente de verdad.

## Board actual (de Vector sobre sí mismo)

- `add-propose-command` → **`open`** (el spec de `/vector:propose`, ya implementado y self-proposed).
  Debería estar en `review` (impl done, solo QA-`5.3` pendiente) pero falta el subcomando de transición
  (lo aporta `/vector:apply`). Change en `openspec/changes/add-propose-command/`.

## Próximo: `/vector:apply` (contexto ya capturado)

`docs/apply-design.md` es el blueprint. Diferencia clave pedida: **autonomía configurable** —
`applyMode` (`auto` | `ask` (default) | `always-ask`) para elegir el work-item usando el **status
traqueado + prioridad** (el plus sobre OpenSpec). Superficie esperada: `vector spec apply|review|close|
archive` + `vector spec next` (selección) + command `/vector:apply [id]`.

Para construirlo: `/vector:raw` (o `/fix`) usando `docs/apply-design.md` como input.

## Pendiente (resto del contrato + producto)

- Comandos: `/vector:apply` (siguiente), `/vector:link`, `/vector:status`, `/vector:close`,
  `/vector:archive`, `/vector:daily`. Binario: subcomandos de transición de estado.
- `vector serve` + **panel web** (`web/` vacío) + API HTTP + SSE. Es ~2/3 de la arquitectura,
  sin empezar.
- `vector init`: detección/reorg del repo + backup/consent (pregunta abierta #3 del vision).
- `install.sh` (instalación día-0) — hoy build manual.
- Token meter derivado (`agent.routed` en activity).

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
- Artefactos git en inglés; conversación/docs en español.
