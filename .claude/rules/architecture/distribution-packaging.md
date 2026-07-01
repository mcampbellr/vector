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

Flujo canónico cada vez que se toca `web/` (antes de reinstalar el binario global):

1. `npm --prefix web run build` → regenera `web/dist`.
2. Re-embeber en el snapshot que compila el binario:
   ```bash
   rm -rf cli/internal/webui/dist/assets cli/internal/webui/dist/index.html
   cp -R web/dist/. cli/internal/webui/dist/
   ```
3. `go -C cli build -o ~/.local/bin/vector ./cmd/vector` (o el reinstall de la Memory).
4. **Reiniciar cualquier `vector serve` en marcha** — un server ya corriendo tiene el binario
   viejo en memoria y sigue sirviendo el frontend anterior hasta reiniciarse.

`cli/internal/webui/dist/assets/` está **gitignored** (se regenera en cada build); solo
`index.html` se rastrea. Verificar que no haya drift: `ls cli/internal/webui/dist/assets` debe
igualar `ls web/dist/assets`. Ver también la Memory `reinstall-vector-binary-after-changes`.

> Estado: el mecanismo de embed (`//go:generate` + `embed.FS` + `SeedCommands`) ya está activo.
> Pendiente: layout del pipeline de release y script de instalación de un paso. Ver nota de
> distribución en `docs/vision.md` (§Techstack).
