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
- **Sync explícito y de bajo consumo vía SSE**: el board refleja la frescura
  (`updated N sec ago`) a través del stream de la API de `cli/` (ver `docs/domain-contract.md` §4).
- **Toda acción que toque el estado actualiza el JSON en el mismo paso** — disciplina
  reforzada en `workflows/state-sync-discipline.md`.

## Entidades (mapeo al dominio, ver `product/domain-model.md`)

- **Spec** — unidad central (creada con `/vector:raw [text]`). Equivale a una card del board.
- Atributos de spec: estado (`open`/`in-progress`/`needs-attention`/`review`/`closed`/
  `archived`), `stage` opcional, prioridad, `estimateMinutes` (tiempo), link de ticket, notas.
  El ahorro de tokens es derivado (no atributo del state). Esquema: `docs/schemas/state-and-activity.md`.
- **Workspace/proyecto** — agrupa specs (mapea al repo raíz administrado por Vector).

## Decisiones cerradas (ver `docs/domain-contract.md`)

- **Esquema y ubicación**: `state.json` por-spec en `.vector/specs/<id>/` (committed, sharded);
  `activity.jsonl` local; `board.json` derivado. Detalle en `docs/schemas/state-and-activity.md`.
- **Estimación = tiempo** (`estimateMinutes`); token meter derivado de `activity.jsonl`.
- **Columnas del board = estado** (single-axis); `stage` es campo opcional.
- **Sync = SSE** vía la API de `cli/` (`vector serve`); `web/` nunca lee el filesystem.

> Estado: pendiente — solo el detalle fino de los endpoints y su versionado, al especificar
> el panel web.
