# Architecture — Modelo de estado (JSON record)

> Aplica a: cualquier feature que lea o escriba el estado del proyecto (specs, board, sync).

Vector mantiene un **JSON de estado/record** como **única fuente de verdad** del board. El
panel web es una proyección de ese JSON; nunca al revés.

## Invariantes

- **Single source of truth**: todo lo que muestra el board (specs, estado, etapa, prioridad,
  estimación, link de ticket, frescura) deriva del JSON. No hay estado canónico en memoria
  del frontend ni en una DB paralela.
- **Escritura serializada por el CLI/API de `cli/`**: el frontend nunca muta el JSON
  directamente; envía intenciones a la API y la API persiste.
- **Sync explícito y de bajo consumo**: el board refleja la frescura (`updated N sec ago`).
  El patrón de sincronización JSON ↔ board debe ser eficiente (ver investigación pendiente en
  `docs/vision.md`).
- **Toda acción que toque el estado actualiza el JSON en el mismo paso** — disciplina
  reforzada en `workflows/state-sync-discipline.md`.

## Entidades (mapeo al dominio, ver `product/domain-model.md`)

- **Spec** — unidad central (creada con `/vector:raw [text]`). Equivale a una card del board.
- Atributos de spec: estado (`todo`/`progress`/`review`/`done`), etapa del workflow,
  prioridad, estimación (tiempo **o** budget de tokens — abierto), link de ticket, notas.
- **Workspace/proyecto** — agrupa specs (mapea al repo raíz administrado por Vector).

## Decisiones abiertas

> Estado: pendiente.
> - Nombre, ubicación y **esquema** exacto del JSON (pregunta abierta #4 del vision).
> - Si la estimación representa tiempo o budget de tokens.
> - Mecanismo de sync (polling vs SSE vs file-watch) optimizado para bajo consumo.
> - Si las columnas del board son **estado** o **etapa del workflow** (pregunta abierta #3).
