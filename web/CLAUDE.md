# Workspace `web/` — Manifest

> SPA del board en marcha (slice inicial). Declara el rol del workspace y enlaza las rules
> relevantes; no las duplica.

## Rol

Frontend del **board kanban** de Vector (panel web local). Es una **proyección** del JSON de
estado: consume la API HTTP de `cli/` y no posee estado canónico ni accede al filesystem del
usuario.

## Stack

- **React 19 + Vite + TypeScript**. Estilos: **CSS Modules + CSS variables** (tokens en
  `src/styles/tokens.css`, derivados de `docs/kanban-ui-reference.md`). Iconos: `lucide-react`.
  Sin librería de componentes (regla de bundle ligero). Output buildado **embebido** en el
  binario Go de `cli/`.

## Estructura

- `src/types/board.ts` — contrato espejo de `cli/internal/board` (única forma que renderiza).
- `src/api/useBoard.ts` — suscripción SSE a `/api/events` (push del board; auto-reconnect).
- `src/components/*` — un componente por carpeta (`KanbanBoard`, `BoardColumn`, `SpecCard`,
  `StatusPill`, `PriorityFlag`, `BoardHeader`, **`TokenSavingsMeter`** = el diferenciador).
- `src/lib/` — `format.ts` (USD/tiempo/relativo), `useNow.ts` (tick de frescura).

## Dev loop

- `vector serve --port 8787` (API+SSE) **+** `npm run dev` (Vite 5173, proxy `/api` → 8787).
- Alternativa: `npm run build` y `vector serve --web-dir web/dist`.
- Build embebido: `npm run build` → copiar `web/dist` a `cli/internal/webui/dist/` **antes** de
  compilar/reinstalar el binario. **Obligatorio en TODO cambio de web** (no solo en release):
  `go build` no rebuildea el frontend, así que un binario recompilado sin re-embeber sirve el UI
  viejo **silenciosamente**. Flujo completo y modo de fallo en
  `architecture/distribution-packaging.md` (§Flujo de edición del frontend).

## Depende de / es dependido por

- **Consume** la API HTTP de `cli/` (única fuente de datos). Nunca lee el JSON directamente.
- Su build es **previo** al build de `cli/` (los assets deben existir antes del embed).

## Pendiente

- Drag-and-drop (mover cards = API de escritura, otro slice). Rail de iconos + sidebar de
  proyectos. Typegen del contrato desde Go (hoy el espejo en `board.ts` es a mano).

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
