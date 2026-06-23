# Standards — TypeScript / React (panel web)

> Aplica a: `web/` (frontend del board kanban). Hereda el global del usuario, incluida la
> regla **one-component-per-file** y el strong typing (sin `any`).

## Componentes

- **Un componente por archivo** (regla global). Cuando un componente crece y necesita
  subcomponentes/helpers, se promueve a carpeta `Parent/index.tsx` + hermanos por
  subcomponente (`KanbanCard.tsx`, `StatusPill.tsx`, `ColumnHeader.tsx`), con
  `helpers.ts`/`types.ts`/`constants.ts` al lado.
- Naming semántico y verboso: `KanbanBoard`, `SpecCard`, `StatusPill`, `PriorityFlag` —
  nunca `Item`/`List`/`Row`. Sin cadenas monstruo; si el nombre es larguísimo, la estructura
  de carpetas está mal.

## Tipado y datos

- Sin `any`. Los tipos del board **derivan del contrato de la API** de `cli/` (estado, spec,
  etapa, prioridad). No duplicar el esquema a mano; generarlo o importarlo desde una fuente
  única cuando exista.
- El frontend **no posee estado canónico**: el JSON de estado vive en el backend
  (`architecture/state-model.md`). El estado del cliente es proyección/caché de la API.

## UI

- Referencia visual y tokens en `docs/kanban-ui-reference.md` (tema claro, cards blancas
  ~12px, status pills por color: progress→ámbar, review→violeta, todo→slate, done→verde).
  Tratarlo como dirección de look & feel, **no** como spec final.
- Accesibilidad y contraste como criterio, no afterthought.

## Build

- El output buildado se **embebe** en el binario Go (`architecture/distribution-packaging.md`).
  El build de `web/` corre antes del de `cli/`. Mantener el bundle ligero.

> Estado: pendiente — elección exacta (Next vs React + Vite), gestor de paquetes, librería de
> drag-and-drop del kanban y estrategia de generación de tipos desde la API.
