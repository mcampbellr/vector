# Vector — Contrato de dominio (LOCKED)

> Decisiones cerradas que fijan el modelo del que derivan `cli/`, `web/` y `kit/`. Es la
> fuente de verdad del vocabulario. Sustituye los puntos "pendiente" relacionados en las rules
> de `.claude/`. Esquema concreto: [[state-and-activity]].

## 1. Estados del spec (vocabulario canónico)

`draft` · `open` · `in-progress` · `needs-attention` · `review` · `closed` · `archived`

- kebab-case en datos; el frontend mapea a display ("Needs attention", uppercase en pills).
- `draft` es el estado de **entrada** (output de `/vector:raw`): el **spec está escrito pero
  todavía no existe el change de OpenSpec**. El change se crea en `/vector:propose`, que mueve
  el spec a `open`. Un spec puede quedarse en `draft` (idea que no se formaliza) o cerrarse desde ahí.
  Distinción spec≠change: la card de Vector existe sin change; el `specDoc` apunta al doc autorado.
- **Reemplaza** el set antiguo `todo/progress/review/done`. Ese set queda obsoleto.
- `needs-attention` es de primera clase (feature central): se entra desde `in-progress` o
  `review` cuando surgen preguntas; lo dispara un **hook**, no el modelo.
- `archived` no aparece en el board activo (vista separada).

### Máquina de estados (transiciones permitidas)

```
  /vector:raw      /vector:propose     /vector:apply       /vector:status
      │                  │                   │                   │
      ▼                  ▼                   ▼                   ▼
    draft ───────────▶ open ──────────▶ in-progress ─────────▶ review
                                            │  ▲                 │
                                   hook ────┘  └─ /vector:status ┘
                                            ▼
                                     needs-attention ◀── hook (en review)
                                            │  (resuelto) /vector:status → in-progress | review

    in-progress | review ──/vector:close──▶ closed ──/vector:archive──▶ archived
```

- `draft` no tiene change de OpenSpec; `/vector:propose` lo crea y pasa a `open`.
  Un `draft` también puede ir directo a `closed` (idea descartada) sin formalizarse.

- `needs-attention` es un overlay sobre el trabajo activo: al resolverse vuelve a
  `in-progress` o `review`. Se prioriza/resalta en board y en `/vector:daily`.

## 2. Board: columnas = ESTADO (single-axis, V1)

- Columnas del kanban = los estados del lifecycle, en orden:
  `draft | open | in-progress | needs-attention | review | closed`.
- `archived` → vista separada (no columna del board activo).
- **`stage`** (etapa de workflow, ej. Concept/Design) queda como **campo opcional** del spec,
  **no** como columna en V1. La referencia visual ([[kanban-ui-reference]]) usaba etapas como
  columnas, pero no generalizan entre repos; se reevalúa post-V1.
- Orden dentro de columna = computado (`priority` desc, luego `updatedAt`), no manual.

## 3. Estimación vs token meter (son cosas distintas)

- **`estimateMinutes`**: estimación de **tiempo** de planning, opcional/manual → ícono de
  reloj en la card.
- **Token meter**: **derivado** de los eventos `agent.routed` de `activity.jsonl` (ahorro por
  ruteo a agentes baratos) → se muestra **por separado**, no en el campo de estimación. No
  vive en el state committed.

## 4. Contrato `web/` ↔ `cli/`

- `vector serve` expone una **API HTTP versionada**; `web/` la consume. **Nunca** lee el
  filesystem ni el JSON directamente (refuerza `architecture/system-boundaries.md`).
- **SSE** para la frescura/live updates ("updated N sec ago").
- `board.json` pasa a ser **derivación/cache interna** del CLI (o en memoria); **no** es el
  contrato del frontend ni se commitea.
- Sketch de endpoints (a detallar al especificar el panel):
  - `GET /api/board` → columnas + specs proyectados
  - `GET /api/specs/:id` → detalle de un spec
  - `GET /api/daily` → resumen del día (lee `activity.jsonl` + git log)
  - `GET /api/stream` (SSE) → eventos de cambio para refrescar el board

## 5. Comando → escritura en el state (mapa)

El CLI Go es el único escritor. Cada comando escribe `updatedAt`.

| Comando | Escribe en `state.json` | Evento en `activity.jsonl` | Efecto OpenSpec |
|---------|--------------------------|-----------------------------|------------------|
| `/vector:raw [text]` | crea `<id>/state.json` (`status:draft`, `createdAt`, `specDoc` puntero) + escribe el spec doc (20 secciones) en `specPath` | `spec.created` | — (change se crea en propose) |
| `/vector:propose [id]` | `status:open`, `openspec{change,artifacts}` | `spec.proposed` + `status.changed` | crea el change `openspec/changes/<id>/` (proposal/design/tasks) |
| `/vector:link [id] [ticket]` | `ticket{provider,key,url,auto}` | `spec.linked` | — |
| `/vector:status [id] [status]` | `status` + timestamp del estado (`reviewAt`/etc) | `status.changed` (`trigger:command`) | — |
| `/vector:apply [id]` | `status:in-progress`, `startedAt` | `spec.applied` + `status.changed` (`trigger:apply`) | `openspec apply <change>` (implementa) |
| `/vector:close [id]` | `status:closed`, `closedAt` | `spec.closed` + `status.changed` | — |
| `/vector:archive [id]` | `status:archived`, `archivedAt` | `spec.archived` | mover change a `archive/` |
| `/vector:sync` | crea cards desde `openspec/changes/*` (`open`/`in-progress`/`review`/`archived` por tasks) + specs sueltos del `spec-path` sin change → `draft`; `openspec{change,artifacts}` provenance; `--reconcile` actualiza | `spec.created` (`source:sync`) / `status.changed` (`trigger:sync`) | lee (read-only); no modifica OpenSpec |
| `/vector:daily` | — (read-only) | — (lee hoy + git log) | — |
| **hook** (surgen preguntas) | `status:needs-attention`, `needsAttention{reason,since,source:hook}` | `status.changed` (`trigger:hook`) | — |

- `auto`: si `/vector:raw` menciona un ticket, `link` se aplica automáticamente (`auto:true`);
  si no, el usuario lo asocia con `/vector:link`.
- Notas/reminders custom (prompt en el flujo) → `note.added` / `reminder.set` en activity.

## IDs

- `id` de spec = **slug kebab-case**, legible en CLI y == nombre del change de OpenSpec al
  aplicar (ver [[state-and-activity]]).
