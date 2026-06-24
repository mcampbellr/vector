# Workspace `kit/` — Manifest

> Carpeta scaffoldeada. **Sin código aún.** Declara el rol del workspace y enlaza las rules
> relevantes; no las duplica.

## Rol

El **ecosistema distribuible** de Vector: project commands, rules, memorias y `devup` que Vector
instala en el repo **del usuario**. Son artefactos (markdown + assets), no lógica de runtime de
Go/TS. Es lo que materializa la propuesta "ecosistema que estandariza la organización del repo".

## Contenido

- **Project commands** distribuibles en `commands/vector/*.md` (`/vector:raw`, etc.). El
  subdirectorio `vector/` da el namespace con colon. **No es un plugin** — ver
  `docs/plugin-and-commands.md`.
- **Rules/memorias** plantilla que Vector siembra en el repo del usuario.
- **`devup`** — herramienta existente del usuario, unificada aquí (lanzar dev local vía bloque
  `run:` en `.project-structure`).

## Depende de / es dependido por

- **Independiente en runtime** de `cli/` y `web/`. `cli/` (vía `vector init`) copia
  `kit/commands/vector/` a `<repo>/.claude/commands/vector/` del usuario; `kit/` no importa
  código de los otros workspaces.
- Instalación **per-proyecto** (modelo OpenSpec), bajo las salvaguardas de seguridad.

## Rules aplicables (`.claude/rules/`)

- `product/principles.md` — developer-focused, agnóstico, comercial día 0, unifica devup.
- `product/token-routing.md` — cada skill elige el tier de agente apropiado por paso.
- `product/domain-model.md` — vocabulario del dominio que las skills manipulan.
- `standards/naming.md` — skills/comandos/flags en kebab-case.
- `security/destructive-ops-consent.md` — instalar/sembrar en el repo del usuario con permiso + backup.
- `workflows/state-sync-discipline.md` — toda skill que toca el dominio mantiene el JSON up-to-date.
- `documentation/docs-standards.md` — cómo se documentan los artefactos del kit.
- `workflows/git-convention.md` — convención git del repo.
