# Workspace `web/` — Manifest

> Carpeta scaffoldeada. **Sin código aún.** Declara el rol del workspace y enlaza las rules
> relevantes; no las duplica.

## Rol

Frontend del **board kanban** de Vector (panel web local). Es una **proyección** del JSON de
estado: consume la API HTTP de `cli/` y no posee estado canónico ni accede al filesystem del
usuario.

## Stack

- React/Next (TypeScript). Output buildado **embebido** en el binario Go de `cli/`.

## Depende de / es dependido por

- **Consume** la API HTTP de `cli/` (única fuente de datos). Nunca lee el JSON directamente.
- Su build es **previo** al build de `cli/` (los assets deben existir antes del embed).

## Rules aplicables (`.claude/rules/`)

- `standards/typescript-react.md` — one-component-per-file, tipado desde el contrato de la API.
- `standards/naming.md` — naming de componentes/archivos.
- `product/domain-model.md` — qué representa cada elemento del board (spec, pill, etapa…).
- `architecture/state-model.md` — el frontend es proyección; no posee el estado.
- `architecture/system-boundaries.md` — `web/` depende de `cli/`, no al revés.
- `architecture/distribution-packaging.md` — bundle ligero, build antes del embed.
- `quality/testing-and-review.md` — tests de proyección/componentes; gate de calidad.
- `documentation/docs-standards.md` — `docs/kanban-ui-reference.md` es dirección visual, no spec final.
- `workflows/git-convention.md` — convención git del repo.
