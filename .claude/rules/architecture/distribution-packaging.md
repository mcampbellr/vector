# Architecture — Distribución y empaquetado

> Aplica a: build, release, instalación, y cualquier decisión que afecte cómo el usuario final
> obtiene y ejecuta Vector.

La comercialización/distribución es un requisito **desde el día 0**, no una fase posterior.
Cada decisión de arquitectura se evalúa contra el costo de instalación.

## Principios

- **Un solo binario Go**: `cli/` produce un binario que incluye el CLI **y** el servidor del
  panel web. El frontend de `web/` se **embebe** como assets buildados (p. ej. `embed.FS`)
  dentro del binario. El usuario no instala Node ni levanta procesos separados.
- **Instalación de un paso**: objetivo `curl … | install.sh` (o equivalente) desde GitHub,
  sin pasos manuales. Cualquier dependencia de runtime adicional rompe este objetivo y debe
  justificarse.
- **Deps del binario (ya no 100% stdlib)**: el CLI adopta **cobra** (`spf13/cobra`, Apache-2.0)
  para el árbol de comandos + `vector completion <shell>` (scripts generados on-the-fly, nada
  embebido) y **lipgloss** (`charmbracelet/lipgloss`, MIT) para el output humano estilizado y el
  `--help` auto-generado; son las **primeras** deps externas del módulo, ambas compile-time (no
  runtime), estáticas y con licencias compatibles con la distribución comercial. Sin
  `huh`/`bubbletea` (el CLI es no interactivo). Costo de peso medido (build release-equivalent
  `CGO_ENABLED=0 -ldflags "-s -w"`): ~6.6 → ~8.0 MiB (+~1.4 MiB, +~22%); sin umbral duro. El
  `--json` consumido por los `/vector:*` permanece **byte-idéntico** (gate: suite golden en
  `cli/cmd/vector/`).
- **Panel web local efímero**: se levanta en un puerto disponible y poco usado solo cuando el
  dev administra Vector; no es un servicio permanente.
- **El kit son project commands, no un plugin**: los `/vector:*` son archivos markdown en
  `kit/commands/vector/*.md` (el subdirectorio da el namespace con colon). El binario **embebe**
  esos commands (`embed.FS`, junto con los assets de `web/`), de modo que el binario global basta
  para sembrarlos sin necesidad de `kit/` en la máquina del usuario.
- **Todos los agentes del kit se embeben, salvo los de OpenSpec**: cualquier agente que Vector
  distribuya (`kit/agents/*.md` — refiners, validators, writers, evaluators, etc.) **debe**
  vendorizarse (`//go:generate` → `assets/`) y embeberse (`//go:embed all:assets`) en el binario,
  de modo que `vector init`/`update` lo siembren sin depender de skills globales del usuario. La
  **única excepción** son los agentes propios de **OpenSpec** (tooling externo `opsx:*`): esos
  pertenecen a OpenSpec, no a Vector, y no se embeben. Regla: un agente del kit que un command
  `/vector:*` invoque nunca debe asumir que existe en `~/.claude/` del usuario.
- **Instalación per-proyecto (modelo OpenSpec)**: el binario `vector` es global en el `PATH`; el
  subcomando de terminal **`vector init`** escribe los commands embebidos en
  `<repo>/.claude/commands/vector/` del repo del usuario (bootstrap + detección + consentimiento),
  de forma reproducible y sin plugin ni marketplace. `init` es subcomando del binario, **no** un
  slash command. Ver `docs/plugin-and-commands.md`.

## Implicaciones para el desarrollo

- El build de `web/` es una **etapa previa** al build de `cli/`: los assets deben existir
  antes de compilar el binario. Documentar el orden en el pipeline de release.
- Versionar juntos binario + assets embebidos para evitar drift entre API y frontend.

### Flujo de edición single-source (kit → binario → .claude/)

`kit/` es la **única fuente editable** de agentes y commands. `cli/internal/scaffold/assets/`
es una copia generada (nunca editar a mano); `.claude/agents/` y `.claude/commands/vector/` en
cualquier repo son copias sembradas por el binario (no rastreadas en git, no editar a mano).

Flujo canónico para propagar un cambio:

1. Editar el archivo en `kit/agents/<agente>.md` o `kit/commands/vector/<cmd>.md`.
2. Correr `go generate ./internal/scaffold` desde `cli/` → actualiza `assets/`.
3. Reinstalar el binario (`go install ./cmd/vector` o el script de la Memory).
4. Correr `vector update` en la raíz del repo → `SeedCommands` siembra `.claude/` desde el
   binario.

`assets/` permanece rastreado en git como snapshot del último `go generate` corrido. El test
`TestAssetsMatchKit` (en `cli/internal/scaffold/scaffold_test.go`) detecta drift entre `kit/`
y `assets/` antes del merge. Ver comentario de paquete en `cli/internal/scaffold/scaffold.go`.

### Flujo de edición del frontend (web → embed → binario) — OBLIGATORIO

> **Regla dura: todo cambio en `web/` exige re-embeber ANTES de reconstruir el binario.**
> El binario embebe `cli/internal/webui/dist/` vía `embed.FS`; ese dist es un **snapshot** del
> último `npm run build`. Recompilar el binario **no** rebuildea `web/` — si editas `web/` y solo
> corres `go build`, el binario sirve el **frontend viejo** de forma **silenciosa** (sin error, sin
> warning). Fue exactamente el bug de `add-ui-sketch-generation`: el sketch se generaba y persistía
> bien, pero el board no mostraba la entrada de descarga porque el dist embebido era anterior al
> cambio de web.

Flujo canónico cada vez que se toca `web/`: **usar `make install`** (o `scripts/dev-install.sh`),
el único path sancionado de reinstalación desde fuente. Encadena, con abort-on-fail:

1. `web-build` → `npm --prefix web run build` regenera `web/dist`.
2. `embed` → re-embebe en el snapshot que compila el binario (lo que el target corre por dentro):
   ```bash
   rm -rf cli/internal/webui/dist/assets cli/internal/webui/dist/index.html
   cp -R web/dist/. cli/internal/webui/dist/
   ```
3. `guard` → `go -C cli test ./internal/webui/... -run TestEmbeddedBoardIntegrity`; **aborta** si el
   `index.html` recién embebido referencia `/assets/*` ausentes, **antes** de tocar el binario instalado.
4. `build` → `go -C cli build -o "$VECTOR_INSTALL_DIR" ./cmd/vector` (default `~/.local/bin/vector`).
5. **Reiniciar cualquier `vector serve` en marcha** — un server ya corriendo tiene el binario viejo
   en memoria y sigue sirviendo el frontend anterior hasta reiniciarse.

`cli/internal/webui/dist/assets/` **y** el `index.html` real (hasheado, producto de `web build`)
están **gitignored** (se regeneran en cada build); lo único trackeado en `dist/` es
`index.placeholder.html` (sin refs a `/assets/*`). Verificar que no haya drift: `ls
cli/internal/webui/dist/assets` debe igualar `ls web/dist/assets`. Ver también la Memory
`reinstall-vector-binary-after-changes`.

**Guard de integridad del board embebido** (`internal/webui`): construir el binario desde un worktree
sin `web build` embebía un `index.html` que referenciaba `/assets/*` inexistentes → board en blanco
**silencioso** (200 con HTML servido donde el browser pide JS). La defensa es en **dos capas**:

*Build-time (previene)* — el estado "no buildeado" trackeado es ahora `index.placeholder.html` (sin
refs a `/assets/*`); el `index.html` real, hasheado, está gitignored. Esto hace **decidible** la
distinción "no buildeado" vs "embed roto": en checkout limpio no hay `index.html` en el embed, así que
`ValidateAssets` corta-circuita retornando `nil` (nada que validar) y el guard PASA — el job `go` de CI
sigue verde **sin** `web build`; sólo tras un `web build`+re-embed real aparecen refs hasheadas que
deben existir o el guard falla. El test `TestEmbeddedBoardIntegrity` corre ese `ValidateAssets` sobre
el `embed.FS` real y es el guard que `make install`/`scripts/dev-install.sh` ejecutan **antes** de
compilar: un embed parcial/roto **aborta la instalación** sin sobrescribir el binario. `make install` es
el **único** path sancionado de reinstalación desde fuente (un `go build` suelto deja de ser flujo
recomendado; si igual se hace desde un worktree sin `web build`, el binario embebe el placeholder honesto
y `spaHandler` sirve una página "board not built" en `/`, no un blanco).

*Runtime (red secundaria, sin cambios)* — al arrancar `vector serve`:
- `webui.ValidateAssets(fsys)` / `webui.EmbeddedAssetsMissing()` reportan los `/assets/*` que el
  `index.html` referencia pero no están en el embed.
- `vector serve` imprime un **WARNING ruidoso** a stderr cuando el board embebido está roto, con el
  comando exacto para rebuildear+re-embeber (en vez de servir el board en blanco).
- `spaHandler` devuelve un **404 real** para `/assets/*` faltantes (sin fallback a `index.html`), de
  modo que la ruptura aparece en la consola del browser, no como HTML disfrazado de JS con 200.

> Nota histórica: la doctrina previa declaraba el guard "de runtime a propósito" porque, con el
> `index.html` hasheado trackeado, un checkout limpio era byte-indistinguible de un embed roto. El
> placeholder trackeado elimina esa ambigüedad y habilita el guard de build-time; esa imposibilidad ya
> no aplica.

> Estado: el mecanismo de embed (`//go:generate` + `embed.FS` + `SeedCommands`) ya está activo.
> Pendiente: layout del pipeline de release y script de instalación de un paso. Ver nota de
> distribución en `docs/vision.md` (§Techstack).
