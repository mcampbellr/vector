# Product — Modelo de dominio (OpenSpec + kanban)

> Aplica a: `cli/`, `web/` y `kit/` — el vocabulario compartido del producto.

Vector se apoya en **OpenSpec** como modelo de specs y lo proyecta en un **board kanban**. La
persistencia del dominio vive en el JSON de estado (`architecture/state-model.md`).

## Entidades y mapeo (de `docs/kanban-ui-reference.md`)

| Concepto kanban | En Vector |
|-----------------|-----------|
| Workspace | Repo / proyecto raíz administrado por Vector |
| Proyecto del sidebar | Repos o sub-workspaces (mono/micro) |
| Columna | **Etapa del workflow** de specs (configurable) — *no* el estado de la card |
| Card / task | **Spec** (creado con `/vector:raw [text]`, equivalente al `/idea` del usuario) |
| Status pill | Estado del spec: `todo` · `progress` · `review` · `done` |
| Prioridad (bandera) | Prioridad del spec: urgent / high / normal / low |
| Estimación (reloj) | Tiempo **o** budget de tokens del spec (abierto) |
| Comentarios | Notas / historial / link al ticket asociado |
| "updated N sec ago" | Frescura del JSON de estado (sync con el board) |

## Distinciones clave

- **Etapa ≠ estado**: las columnas representan la etapa del workflow; el estado vive en el
  pill de la card. (Confirmación final pendiente — pregunta abierta #3 del vision.)
- Un spec puede enriquecerse con metadatos (link de ticket, etc.) administrados sobre el JSON.

## Operaciones del dominio

- `/vector:raw [text]` → crea un spec.
- Administración sobre el JSON → añadir ticket, mover de etapa/estado, ajustar prioridad.
- Cada operación mantiene el JSON up-to-date (`workflows/state-sync-discipline.md`).

> Estado: pendiente — qué representan exactamente las columnas, si la estimación es tiempo o
> tokens, y cómo se mapea el link de ticket en la card.
