# Workspace `cli/` — Manifest

> Manifest del workspace: rol + enlaces a las rules (no las duplica).

## Rol

Módulo Go único que produce **el binario de Vector**: los comandos del CLI, la **API HTTP del
board** y el servidor que sirve el panel web embebido. Es el único que lee/escribe el **JSON
de estado** (CLI-owns-writes). Los commands `/vector:*` (`kit/commands/vector/`) invocan este binario.

## Estado actual

- `internal/state` — paquete dueño del estado: `SpecState`/`Event` (incluye estado `draft`,
  puntero `specDoc`, provenance `openspec`), `Store` (CreateSpec, `ReconcileStatus` para sync,
  `ProposeSpec`, ReadSpec, ListSpecs, AppendEvent, `ReadEvents`), slug, escritura atómica.
  **Máquina de estados LOCKED** (`transition.go`): `CanTransition` + `ApplySpec`/`CloseSpec`/
  `ArchiveSpec`/`SetStatus` (primitiva validada con flag de needs-attention) + `SelectNext`
  (ranking status+prioridad para apply). Con tests.
- `internal/config` — `.vector/config.json` (specPath/store/source/kitVersion/**applyMode**);
  `Resolve` migra de `.project-structure`, auto-detecta o cae al fallback `.vector/`. Con tests.
- `internal/openspec` — lectura read-only de `openspec/changes/*` (artefactos + progreso de
  `tasks.md`) para `vector sync`. Con tests.
- `internal/scaffold` — embebe `kit/{commands,agents,vector}` (`embed.FS`, sync por
  `go generate`) y siembra el motor en `<repo>/.claude/` de forma aditiva. Con tests.
- `internal/board` — proyección read-only del board (columnas=estado, cards) + roll-up del
  **Token Savings Meter** desde `activity.jsonl` (`agent.routed`). `Server` expone la API HTTP
  (`/api/board`) y el stream SSE (`/api/events`). Con tests.
- `internal/webui` — embebe la SPA buildada de `web/` (`embed.FS` de `dist/`) y la sirve como
  SPA (fallback a `index.html`); `--web-dir` sirve desde disco en dev.
- `internal/ui` — capa de **presentación de terminal** (análogo de `internal/webui` para el CLI):
  wrappers lipgloss (`Bold/Green/Red/Dim/Cyan`, `Success/Info/Warning/Error`, `Table`, `KeyValue`)
  + `ApplyCustomHelp` (help estilizado de cobra). Se aplica **solo en la rama humana**, nunca dentro
  de un branch `if jsonOut` (garantía byte-idéntico del `--json`). Con tests.
- `cmd/vector` — entrypoint sobre **cobra** (`newRootCmd` arma un árbol fresco por llamada;
  `newXxxCmd()` factories con los flags 1:1 y el cuerpo en `RunE`): `vector init` (siembra motor +
  config + esqueleto de estado), `vector update` (re-siembra el kit preservando config/state,
  version stamp), `vector sync` (proyecta changes de OpenSpec al board, idempotente/aditivo),
  `vector serve` (panel local: API+SSE+UI embebida, puerto auto, watcher por polling),
  `vector spec create|list|propose|apply|fix|link|relate|status|close|archive|next|worklog|summarize|route|attach-sketch`,
  `vector standup (+commit)`, `vector detect-ticket`, `vector completion <shell>`, `vector version`
  (`-v`/`--version` en cualquier posición). Suite golden (`golden_test.go` + `testdata/golden/`)
  fija el `--json` byte-idéntico como gate.
- Pendiente: detección/reorg de repo en `init` (pregunta abierta), endpoints HTTP de escritura
  (para drag-and-drop del board; hoy las transiciones son por CLI), `vector:link/status/close/
  archive/daily` (commands del resto del contrato; el binario ya soporta las transiciones).

## Stack

- Go (módulo único). Layout `cmd/` + `internal/`. Frontend de `web/` se embebe vía `embed.FS`.
  Deps externas (las **primeras** del módulo): **cobra** (`spf13/cobra`, Apache-2.0) para el árbol
  de comandos + completions, y **lipgloss** (`charmbracelet/lipgloss`, MIT) para el output humano
  estilizado; ambas compatibles con la distribución comercial. Sin `huh`/`bubbletea` (el CLI es no
  interactivo). El resto del código sigue siendo stdlib.

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
