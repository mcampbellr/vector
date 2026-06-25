# Vector — `/vector:apply` (notas de diseño / contexto capturado)

> **No LOCKED.** Captura del proceso de "apply" recorrido sobre `add-propose-command`, más los
> requisitos para construir `/vector:apply`. Sirve de input (vía `/vector:raw` o `/fix`) cuando lo
> implementemos. La diferencia clave con OpenSpec — **autonomía configurable** — está en §3.

## 1. Qué es "apply" (y el plus de Vector sobre OpenSpec)

- **OpenSpec apply**: tomás **un** change nombrado (`openspec apply <change>` / `opsx:apply`) e
  implementás sus tasks; el progreso vive en `tasks.md` (checkboxes).
- **El plus de Vector**: tenemos **status traqueado + prioridad en TODAS las cards** del board
  (`docs/domain-contract.md` §1). Entonces `/vector:apply` no necesita que le nombres el change:
  puede **seleccionar** el próximo work-item por status/prioridad. OpenSpec no tiene esa señal.

## 2. Recorrido real: aplicando `add-propose-command`

Lo que hice (manualmente, porque `/vector:apply` aún no existe) y lo que un `/vector:apply`
automatizaría:

| Paso | Qué pasó | Qué haría `/vector:apply` |
|---|---|---|
| 0. Selección | la card estaba `open` (change formalizado por propose) | elegir el work-item (ver §3: auto/ask/always-ask) |
| 1. Arranque | — | `open → in-progress`, `startedAt`, evento `status.changed (trigger:apply)` + `spec.applied` |
| 2. Modo | repo NO es proyecto OpenSpec → **nativo** | delegate: `openspec apply <change>` si es proyecto OpenSpec; native: trabajar `tasks.md` directo |
| 3. Implementación | edité Go + kit (event.go, store.go, main.go, config.go, propose.md) + tests | el agente implementa siguiendo el spec/proposal; commits atómicos |
| 4. Progreso de tasks | marqué `tasks.md` a medida (11/12 ✅) | marcar checkboxes a medida que avanza; reflejar en el board |
| 5. Estado final | 11/12 done; la única pendiente (`5.3`) es **QA manual** | tasks de implementación hechas, solo QA → **`review`** (misma regla que `sync`) |
| 6. Cierre | (no hecho) | `/vector:close` → `closed`; `/vector:archive` → `archived` + mover el change a `archive/` |
| hook | — | si surgen preguntas → `needs-attention` (overlay sobre el trabajo activo) |

**Resultado:** el change quedó implementado y verificado (gate verde); la card *debería* estar en
`review` (impl done, QA pendiente). Hoy sigue `open` porque no hay subcomando de transición todavía
— eso es parte de lo que `/vector:apply` aporta (ver §4).

## 3. La diferencia clave: **autonomía configurable** (lo que pediste)

`/vector:apply` debe ser configurable en **cuánto decide el LLM vs cuánto pregunta**, usando el
status traqueado como señal. Config `applyMode` en `.vector/config.json`:

| Modo | Comportamiento | Cuándo |
|---|---|---|
| `auto` | el LLM **elige** el work-item (por status/prioridad/orden) y arranca sin preguntar | flujo autónomo, máxima velocidad |
| `ask` (default sugerido) | el LLM **propone un pick** (el mejor candidato, con su razón) y pide confirmar | balance: el LLM sugiere, vos confirmás |
| `always-ask` | siempre muestra la lista de candidatos y vos elegís | control total |

- La **selección** usa el plus de Vector: `in-progress` (continuar lo empezado) > `needs-attention`
  (desbloquear) > `review` (cerrar) > `open` por prioridad. OpenSpec no puede hacer esto (no
  trackea status cross-change).
- **Granularidad a definir**: ¿`applyMode` aplica a (a) elegir QUÉ change, (b) elegir qué TASK
  dentro del change, o (c) ambos? Propuesta: ambos, con el mismo setting; el LLM en `auto` elige
  change y va task por task; en `ask` confirma el change y luego corre.
- Si `/vector:apply <id>` recibe un id explícito, salta la selección (override del modo).

## 4. Superficie esperada (binario + command)

- **Binario** (CLI-owns-writes):
  - `vector spec apply <id>`: `open → in-progress`, `startedAt`, eventos `spec.applied` +
    `status.changed (trigger:apply)`. (Análogo a `ProposeSpec`.)
  - Transiciones de cierre: `vector spec review|close|archive <id>` (o un `vector spec status <id>
    <status>` genérico que valide la máquina de estados del contrato). Reusar el patrón de
    `ReconcileStatus`/`ProposeSpec` (lock + write + evento; stamp del timestamp correcto:
    `reviewAt`/`closedAt`/`archivedAt`).
  - Selección: `vector spec next [--json]` que devuelva el candidato recomendado (por
    status/prioridad) — el command lo usa en modo `auto`/`ask`.
- **Command** `/vector:apply [id]`:
  - Sin id → seleccionar según `applyMode` (auto/ask/always-ask), usando `vector spec next`.
  - Detectar modo OpenSpec (criterio: **¿el repo es proyecto OpenSpec?** — existe `openspec/` con
    estructura — NO solo "el CLI está en PATH"; lección del bootstrap de propose).
  - Delegate (`openspec apply`/`opsx:apply`) o native (implementar siguiendo el change).
  - Marcar `tasks.md` a medida; al terminar, transicionar a `review` (o `closed` si no hay QA).
  - Reportar.

## 5. Estados que apply toca (del contrato §1)

```
open ──apply──▶ in-progress ──(tasks done / solo QA)──▶ review ──close──▶ closed ──archive──▶ archived
                     ▲ │
                hook │ ▼ (preguntas)
                  needs-attention
```

## 6. Open questions (a cerrar al implementar)

- `applyMode` default (`ask` sugerido) y dónde se persiste (config).
- Granularidad del modo (change-level vs task-level vs ambos).
- Cómo se refleja el progreso de `tasks.md` en el board (¿un meter derivado? ¿re-sync?).
- `review` automático cuando tasks 100% o solo-QA: ya tenemos la regla en `sync` (`internal/openspec`
  + `syncStatus`) — reusarla para apply.
- Delegate vs native: el criterio de detección exacto (pendiente desde propose, task `5.3`).
- ¿`/vector:apply` commitea? (como `/fix`) o deja el working tree para el usuario.
