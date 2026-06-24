# Workspace `cli/` — Manifest

> Manifest del workspace: rol + enlaces a las rules (no las duplica).

## Rol

Módulo Go único que produce **el binario de Vector**: los comandos del CLI, la **API HTTP del
board** y el servidor que sirve el panel web embebido. Es el único que lee/escribe el **JSON
de estado** (CLI-owns-writes). Los commands `/vector:*` (`kit/commands/vector/`) invocan este binario.

## Estado actual

- `internal/state` — paquete dueño del estado: `SpecState`/`Event` (incluye estado `draft`,
  puntero `specDoc`, provenance `openspec`), `Store` (CreateSpec con status/doc-path/openspec,
  `ReconcileStatus` para sync, ReadSpec, ListSpecs, AppendEvent), slug, escritura atómica. Con tests.
- `internal/config` — `.vector/config.json` (specPath/store/source/kitVersion); `Resolve`
  migra de `.project-structure`, auto-detecta o cae al fallback `.vector/`. Con tests.
- `internal/openspec` — lectura read-only de `openspec/changes/*` (artefactos + progreso de
  `tasks.md`) para `vector sync`. Con tests.
- `internal/scaffold` — embebe `kit/{commands,agents,vector}` (`embed.FS`, sync por
  `go generate`) y siembra el motor en `<repo>/.claude/` de forma aditiva. Con tests.
- `cmd/vector` — entrypoint: `vector init` (siembra motor + config + esqueleto de estado),
  `vector update` (re-siembra el kit preservando config/state, version stamp),
  `vector sync` (proyecta changes de OpenSpec al board, idempotente/aditivo),
  `vector spec create|list`, `vector version`.
- Pendiente: `serve` (API+SSE), la detección/reorg de repo en `init` (pregunta abierta),
  `vector:propose/apply/link/...` (resto del contrato).

## Stack

- Go (módulo único, stdlib, sin deps externas). Layout `cmd/` + `internal/`. Frontend de
  `web/` se embeberá vía `embed.FS`.

## Depende de / es dependido por

- **Embebe** los assets buildados de `web/` y los commands de `kit/commands/`
  (ver `architecture/distribution-packaging.md`).
- Expone la **API HTTP** que `web/` consume (contrato versionado).
- `vector init` embebe (no lee en runtime) `kit/commands/` vía `internal/scaffold`. No importa
  código de `kit/`.

## Rules aplicables (`.claude/rules/`)

- `standards/go-conventions.md` — estilo, layout, errores, deps.
- `standards/naming.md` — comandos/flags kebab-case, IDs de dominio.
- `architecture/system-boundaries.md` — ownership y dependencias.
- `architecture/state-model.md` — el JSON como fuente de verdad (este workspace lo posee).
- `architecture/distribution-packaging.md` — un binario, instalación día 0, embed.
- `security/destructive-ops-consent.md` — backup + permiso antes de tocar el repo del usuario.
- `workflows/state-sync-discipline.md` — mantener el JSON up-to-date en cada acción de dominio.
- `product/domain-model.md` — vocabulario (spec, estado, etapa, prioridad, ticket).
- `quality/testing-and-review.md` — tests del estado/API; gate de calidad.
- `workflows/git-convention.md` — convención git del repo.
