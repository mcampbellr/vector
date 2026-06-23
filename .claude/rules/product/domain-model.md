# Product — Modelo de dominio (OpenSpec + kanban)

> Aplica a: `cli/`, `web/` y `kit/` — el vocabulario compartido del producto.

Vector se apoya en **OpenSpec** como modelo de specs y lo proyecta en un **board kanban**. La
persistencia del dominio vive en el JSON de estado (`architecture/state-model.md`).

## Entidades y mapeo (de `docs/kanban-ui-reference.md`)

| Concepto kanban | En Vector |
|-----------------|-----------|
| Workspace | Repo / proyecto raíz administrado por Vector |
| Proyecto del sidebar | Repos o sub-workspaces (mono/micro) |
| Columna | **Estado del lifecycle** del spec (single-axis, V1) — ver `docs/domain-contract.md` |
| Card / task | **Spec** (creado con `/vector:raw [text]`, equivalente al `/idea` del usuario) |
| Status pill | Estado: `open` · `in-progress` · `needs-attention` · `review` · `closed` (+ `archived`) |
| Prioridad (bandera) | Prioridad del spec: urgent / high / normal / low |
| Estimación (reloj) | Tiempo de planning (`estimateMinutes`, opcional). El ahorro de tokens es un meter derivado **aparte** |
| Comentarios | Notas / historial / link al ticket asociado |
| "updated N sec ago" | Frescura del JSON de estado (sync con el board vía SSE) |

## Distinciones clave

- **Columna = estado** (LOCKED, `docs/domain-contract.md`). `stage` (etapa de workflow) es un
  campo **opcional** del spec, no una columna en V1.
- Un spec puede enriquecerse con metadatos (link de ticket, etc.) administrados sobre el JSON.

## Operaciones del dominio

- `/vector:raw [text]` → crea un spec.
- Administración sobre el JSON → añadir ticket, mover de estado, ajustar prioridad.
- Cada operación mantiene el JSON up-to-date (`workflows/state-sync-discipline.md`).
- Mapa completo comando→escritura: `docs/domain-contract.md` §5.
