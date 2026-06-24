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
- **Instalación per-proyecto (modelo OpenSpec)**: el binario `vector` es global en el `PATH`; el
  subcomando de terminal **`vector init`** escribe los commands embebidos en
  `<repo>/.claude/commands/vector/` del repo del usuario (bootstrap + detección + consentimiento),
  de forma reproducible y sin plugin ni marketplace. `init` es subcomando del binario, **no** un
  slash command. Ver `docs/plugin-and-commands.md`.

## Implicaciones para el desarrollo

- El build de `web/` es una **etapa previa** al build de `cli/`: los assets deben existir
  antes de compilar el binario. Documentar el orden en el pipeline de release.
- Versionar juntos binario + assets embebidos para evitar drift entre API y frontend.

> Estado: pendiente — mecanismo exacto de embed, layout del pipeline de release y script de
> instalación se definen al iniciar la implementación. Ver nota de distribución en
> `docs/vision.md` (§Techstack).
