# Workspace `kit/` — Manifest

> Carpeta scaffoldeada. **Sin código aún.** Declara el rol del workspace y enlaza las rules
> relevantes; no las duplica.

## Rol

El **ecosistema distribuible** de Vector: skills, rules, memorias y `devup` que Vector instala
en el repo **del usuario**. Son artefactos (markdown + assets), no lógica de runtime de Go/TS.
Es lo que materializa la propuesta "ecosistema que estandariza la organización del repo".

## Contenido

- **Skills/comandos** distribuibles (`/vector:raw`, etc.).
- **Rules/memorias** plantilla que Vector siembra en el repo del usuario.
- **`devup`** — herramienta existente del usuario, unificada aquí (lanzar dev local vía bloque
  `run:` en `.project-structure`).

## Depende de / es dependido por

- **Independiente en runtime** de `cli/` y `web/`. `cli/` puede leer/copiar `kit/` durante
  `/vector init`, pero `kit/` no importa código de los otros workspaces.
- Se instala en el repo del usuario bajo las salvaguardas de seguridad.

## Rules aplicables (`.claude/rules/`)

- `product/principles.md` — developer-focused, agnóstico, comercial día 0, unifica devup.
- `product/token-routing.md` — cada skill elige el tier de agente apropiado por paso.
- `product/domain-model.md` — vocabulario del dominio que las skills manipulan.
- `standards/naming.md` — skills/comandos/flags en kebab-case.
- `security/destructive-ops-consent.md` — instalar/sembrar en el repo del usuario con permiso + backup.
- `workflows/state-sync-discipline.md` — toda skill que toca el dominio mantiene el JSON up-to-date.
- `documentation/docs-standards.md` — cómo se documentan los artefactos del kit.
- `workflows/git-convention.md` — convención git del repo.
