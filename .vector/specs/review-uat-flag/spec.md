# Spec: Distinguir el `review` que requiere UAT manual

## 1. Objetivo

Construir un **refinamiento del estado `review`** que distinga, en el board, las cards cuya
revisión pendiente es **UAT manual** (verificación manual en prod/staging) de las que están en
review "limpio" (implementación + tests completos, nada que verificar a mano).

Esta feature permite que un dev/QA pueda **ver de un vistazo qué specs aguardan UAT manual**
para priorizarlas y coordinarlas, sin confundirlas con review técnico ya cerrado.

Decisión central (tomada con el usuario): **NO se agrega un estado nuevo** al lifecycle. Se
mantiene `review` y se le añade un **marcador `needsUat`** (derivado de las tasks de
verificación de `tasks.md`). Sin columna nueva, sin cambios en la máquina de estados ni en el
enum `Status`.

## 2. Alcance

### Incluido en esta fase

- Nuevo campo **`needsUat bool`** en `SpecState` (`cli/internal/state/types.go`), persistido en
  `.vector/specs/<id>/state.json`.
- **Set/clear del flag durante `sync`**: cuando un change entra a `review` porque solo quedan
  tasks de verificación manual (`PendingReal == 0` con trabajo hecho pero `TasksDone < TasksTotal`),
  `needsUat = true`; cuando entra a `review` con todo done, `needsUat = false`.
- Reusar el clasificador existente `isVerificationTask` (`cli/internal/openspec/openspec.go:163`):
  smoke test / e2e / "manual" + (check|qa|test|verif). **Cero metadata nueva** en el repo del usuario.
- **Proyección al board**: campo `needsUat` en la `Card` (`cli/internal/board/board.go`) y un
  **badge "UAT"** en `SpecCard` (`web/`) para las cards en `review` con el flag activo. La
  columna sigue siendo `review` (single-axis intacto).
- Tipo TS `Card` (`web/src/types/board.ts`) extendido con `needsUat?: boolean`.

### Fuera de scope

- **Un estado/columna nuevo** en el board (descartado: se refina `review`).
- **Lógica de `needsUat` propia de `/vector:apply`.** `needsUat` lo computa **solo `sync`**
  (el único path que relee `tasks.md`). El finish de `/vector:apply` ya transiciona vía
  `vector spec status … review`; para refrescar el flag, ese flujo corre
  `vector sync --reconcile` (que recomputa status + `needsUat` desde `tasks.md`). No se agrega
  lógica de UAT a los subcomandos de transición.
- Cambios en la **máquina de estados** (`allowedTransitions`) — `review` ya existe y sus
  transiciones no cambian.
- Comando para **setear/limpiar `needsUat` a mano** (override manual) — en V1 el flag es
  **derivado de `tasks.md`**, no editable por comando. Override manual = fase futura.
- **Assignment/ownership** del UAT (quién lo hace) — feature separada.
- Integración con trackers externos de QA (TestRail, etc.).
- Drag-and-drop / escritura desde el panel web (el board es read-only hoy).

El agente no debe implementar nada fuera de este alcance, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Lenguaje: Go (módulo único en `cli/`, stdlib only) + TypeScript/React (`web/`).
- State: paquete `cli/internal/state` (único escritor del JSON, CLI-owns-writes).
- Board: `cli/internal/board` (proyección read-only) servido por `vector serve` (API+SSE).
- Web: React 19 + Vite, CSS Modules + tokens.

### Versiones relevantes

- Go: `1.26` (ver `cli/go.mod`).
- React: `19` (ver `web/package.json`).

### Patrones existentes a respetar

- **`SpecState` se serializa con campos `omitempty`** cuando son opcionales (ver `types.go`).
  `needsUat` es un bool derivado → usar `json:"needsUat,omitempty"` (false = ausente).
- **El JSON lo escribe solo el binario** (`Store`, escritura atómica + lock). El flag se setea
  dentro del mismo write de `sync`/`reconcile`, no por fuera.
- **`syncStatus`** (`cli/cmd/vector/main.go:467`) decide el status de un change; la lógica de
  `needsUat` se computa junto a esa decisión (misma fuente: `openspec.Change`).
- **Tipo TS `Status`/`Card` es espejo del contrato Go** (`web/src/types/board.ts`); mantener
  sincronizado a mano hasta que haya typegen.
- One-component-per-file en `web/`.

---

## 4. Dependencias previas

Antes de iniciar esta fase ya existe (verificado):

- [x] Estado `review` en el enum (`cli/internal/state/types.go`).
- [x] `isVerificationTask` + `PendingReal`/`TasksDone`/`TasksTotal` en `cli/internal/openspec/openspec.go`.
- [x] `syncStatus` con la regla "todo done o solo QA manual → review" (`cli/cmd/vector/main.go`).
- [x] `ReconcileStatus` y `CreateSpec` como puntos de escritura de sync (`cli/internal/state/store.go`).
- [x] Proyección `Card` + `SpecCard`/`StatusPill` en board (Go + web).

No hay dependencias faltantes. Es una extensión aditiva de piezas existentes.

---

## 5. Arquitectura

### Patrón a usar

Domain-first: el flag vive en el estado (fuente de verdad), se computa en el punto de escritura
de sync, y se proyecta read-only al board. No hay lógica de UAT en el frontend.

### Capas afectadas

- presentation (web): sí — badge "UAT" en `SpecCard`; tipo `Card`.
- application/use-cases: sí — `syncStatus` (y la regla de cierre de `/vector:apply`) computan el flag.
- domain (Go state): sí — campo `needsUat` en `SpecState`; set/clear en sync/reconcile.
- data/infrastructure: sí — serialización del nuevo campo en `state.json`.
- shared/common: no.

### Flujo esperado

1. Un change avanza; en `tasks.md` quedan sin marcar solo tasks de verificación manual
   (las que `isVerificationTask` reconoce), con al menos una task ya hecha.
2. `vector sync` lee el change → `syncStatus(c)` retorna `review` (regla actual). **En el mismo
   `runSync`**, un helper sibling `syncNeedsUAT(c)` computa el bool
   `c.HasTasks && c.TasksTotal > 0 && c.TasksDone > 0 && c.TasksDone < c.TasksTotal && c.PendingReal == 0`.
3. El binario persiste `status:review` + `needsUat:true` en `state.json` (mismo write, vía
   `CreateSpec`/`ReconcileStatus`).
4. `board.Build` (`toCard`) copia `NeedsUAT` a la `Card`; `vector serve` lo emite por SSE.
5. El board muestra la card en la columna `review` con un **badge "UAT"**.
6. Cuando el dev cierra la card (`/vector:close`), pasa a `closed` (el flag deja de mostrarse).

### Decisión de diseño (threading del flag)

`syncStatus` **mantiene su firma** (`func(openspec.Change) state.Status`). El flag se computa en
un **helper aparte** `syncNeedsUAT(c openspec.Change) bool` y se pasa al store como **dato
adicional**, no rediseñando `syncStatus` para devolver una tupla. Así el cómputo de status y el
del flag quedan desacoplados y testeables por separado.

### Ubicación de archivos nuevos

No se crean carpetas nuevas. Solo se extienden archivos existentes (ver §6).

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `cli/internal/state/types.go` | MODIFICAR | Campo `NeedsUAT bool json:"needsUat,omitempty"` en `SpecState` | campos `Stage`/`Assignee` (omitempty) |
| `cli/internal/state/store.go` | MODIFICAR | Setear/limpiar `NeedsUAT` en el write de `ReconcileStatus`/`CreateSpec` de sync | campo `OpenSpec *OpenSpec` en `CreateSpecParams` + firma de `ReconcileStatus` |
| `cli/cmd/vector/main.go` | MODIFICAR | Computar `needsUat` junto a `syncStatus` y pasarlo al store | `syncStatus` (`:467`), bloque de `runSync` |
| `cli/internal/board/board.go` | MODIFICAR | Campo `NeedsUAT bool` en `Card`; copiar de `SpecState` en `toCard` | `toCard` (mapeo spec→card) |
| `web/src/types/board.ts` | MODIFICAR | `needsUat?: boolean` en `Card` | campos opcionales del `Card` |
| `web/src/components/SpecCard/SpecCard.tsx` | MODIFICAR | Badge "UAT" cuando `status==='review' && needsUat` | badge de ahorro / ticket existentes |
| `web/src/components/SpecCard/SpecCard.module.css` | MODIFICAR | Estilo del badge "UAT" (seguir el patrón de naming de `.ticket`/`.savings` del mismo archivo) | `web/src/components/SpecCard/SpecCard.module.css` |
| `docs/domain-contract.md` | MODIFICAR | Nota en §1/§5: `review` puede llevar `needsUat` (derivado), NO es estado nuevo | §1 estados, §5 mapa |

### Detalle por archivo

#### cli/internal/state/types.go

Acción: MODIFICAR. Agregar a `SpecState`:

- `NeedsUAT bool` con tag `json:"needsUat,omitempty"`, documentado como **derivado de las tasks
  de verificación del change** (no editable a mano en V1).

No debe: agregar valores al enum `Status` ni tocar `Valid()`.

#### cli/cmd/vector/main.go

Acción: MODIFICAR. En `runSync`, donde se llama `syncStatus(c)`:

- Agregar el helper `syncNeedsUAT(c openspec.Change) bool` (al lado de `syncStatus`) que retorna
  `c.HasTasks && c.TasksTotal > 0 && c.TasksDone > 0 && c.TasksDone < c.TasksTotal && c.PendingReal == 0`.
- En cada rama que crea/reconcilia un change (`store.CreateSpec`, `store.ReconcileStatus`), pasar
  el bool computado para que el store lo persista en el mismo write.

Restricción: **no** cambiar la firma de `syncStatus` (sigue devolviendo `state.Status`). El flag
viaja como dato separado, no como tupla.

#### cli/internal/state/store.go

Acción: MODIFICAR. Threading del flag por los puntos de escritura de sync:

- Agregar `NeedsUAT bool` a `CreateSpecParams`; `CreateSpec` lo asigna a `spec.NeedsUAT` antes de
  `writeSpecFile`.
- Agregar un parámetro `needsUAT bool` a `ReconcileStatus(id, status, openSpec, needsUAT, actor, now)`;
  setea `spec.NeedsUAT = needsUAT` dentro del mismo lock/write. Actualizar el call site en `runSync`.
- Limpiar el flag cuando el status resultante **no** es `review` (p. ej. al volver a `in-progress`
  por `--reconcile`): si `status != StatusReview`, forzar `needsUAT = false`.

Restricción: ningún otro método escribe `NeedsUAT`; no exponer un setter público en V1.

#### cli/internal/board/board.go

Acción: MODIFICAR.

- Agregar `NeedsUAT bool json:"needsUat,omitempty"` al struct `Card`.
- En `toCard`, copiar `card.NeedsUAT = spec.NeedsUAT`.

Restricción: no cambiar `columnOrder` ni agregar columnas.

#### web/src/types/board.ts

Acción: MODIFICAR. Agregar `needsUat?: boolean` a la interface `Card` (campo opcional, espejo del
Go). No tocar el union `Status`.

#### web/src/components/SpecCard/SpecCard.tsx

Acción: MODIFICAR. Renderizar un badge `UAT` cuando `card.status === 'review' && card.needsUat`,
dentro de la fila meta `<footer className={styles.meta}>` (al lado del `StatusPill`), **no** en el
`<header>` de la card. El badge debe tener `title`/`aria-label` accesible (p. ej. "Requires
manual UAT"). No reemplaza al `StatusPill`.

#### web/src/components/SpecCard/SpecCard.module.css

Acción: MODIFICAR. Clase `.uat` para el badge, siguiendo el patrón de `.ticket`/`.savings` del
mismo archivo (pill pequeño, color distintivo del violeta de review). No cambiar otras clases.

#### docs/domain-contract.md

Acción: MODIFICAR. Anotar en §1 y/o §5 que `review` puede llevar un marcador **derivado**
`needsUat` (UAT manual pendiente), aclarando que **NO es un estado nuevo** ni cambia la máquina de
estados. No reescribir la lista LOCKED de estados.

---

## 7. API Contract

No aplica como contrato HTTP externo nuevo. El endpoint existente `GET /api/board` y el stream
SSE `/api/events` ganan un campo **aditivo** `needsUat` en cada `Card`. Sin cambio de rutas ni
versionado: el campo es opcional y los clientes viejos lo ignoran.

---

## 8. Criterios de éxito

La implementación es correcta cuando:

- [ ] `SpecState` tiene `NeedsUAT` y serializa como `needsUat` (omitido cuando es false).
- [ ] `vector sync` sobre un change con solo tasks de verificación pendientes (y ≥1 hecha) deja
      la card en `review` con `needsUat:true`; con todas las tasks done, `review` con `needsUat` ausente.
- [ ] `vector sync --reconcile` sobre una card `review`/`needsUat:true` a la que se le agregó
      trabajo real (vuelve a `in-progress`) **limpia** el flag (`needsUat` ausente).
- [ ] `Card.needsUat` aparece en `GET /api/board` para esas cards.
- [ ] El board muestra el badge "UAT" solo en cards `review` con el flag.
- [ ] No se agregó ningún `Status` nuevo ni columna nueva.
- [ ] No hay regresiones en `sync`/`apply`/`close`/`serve`.

### Tests requeridos

- [ ] Unidad: `syncNeedsUAT` (true cuando solo quedan tasks de verificación con ≥1 hecha; false
      cuando todo done; false cuando hay trabajo real pendiente; false cuando `TasksDone==0`).
- [ ] `ReconcileStatus`/`CreateSpec`: persisten y limpian `NeedsUAT` correctamente.
- [ ] `board.Build`: la `Card` refleja `NeedsUAT` del `SpecState`.
- [ ] **Serialización HTTP/SSE** (`board` `server_test.go`): `GET /api/board` incluye
      `needsUat:true` cuando el flag está activo y **lo omite** cuando es false (verifica el
      `omitempty`, evitando una regresión silenciosa de serialización).
- [ ] Web: typecheck con el campo nuevo.

### Comandos de verificación

```bash
gofmt -l cli && go -C cli vet ./... && go -C cli test -race ./...
npm --prefix web run typecheck
```

La fase no está completa si alguno falla.

---

## 9. Criterios de UX

Aplica solo al board (panel web read-only):

- En la card de un spec en `review` con `needsUat`, mostrar un **badge "UAT"** (texto corto,
  estilo pill) junto al `StatusPill`/meta de la card.
- El badge NO reemplaza al `StatusPill` de `review` (violeta); lo acompaña.
- Cards en `review` sin el flag no muestran badge (review limpio).

Por subsección del template:

- **Loading / Formularios / Passwords:** No aplica — el board es read-only y esta feature no
  agrega formularios ni inputs.
- **Errores:** No aplica nuevo — si falla el fetch del board mientras hay cards `needsUat`, el
  comportamiento es el del board actual (placeholder de error / reintento por SSE); no cambia.
- **Navegación:** No aplica — no hay navegación nueva.
- **Accesibilidad:** el badge "UAT" debe llevar `title`/`aria-label` (p. ej. "Requires manual
  UAT") para que sea legible por tecnologías asistivas; no depender solo del color para
  diferenciarlo del `StatusPill` de review.

---

## 10. Decisiones tomadas

El agente no debe cuestionar ni cambiar:

- **Refinar `review`, NO crear estado nuevo.** Sin columna nueva, sin tocar `allowedTransitions`
  ni el enum `Status`.
- **Detección por tasks de QA/manual en `tasks.md`**, reusando `isVerificationTask`
  (`cli/internal/openspec/openspec.go:163`). Sin metadata nueva en el repo del usuario.
- **`needsUat` es derivado y se persiste en `state.json`** (set por sync, igual que el status),
  no se deriva on-the-fly en `board.json`. Razón: consistencia con cómo sync ya persiste status.
- **El flag se computa cuando entra a `review` por UAT pendiente**: `TasksDone>0 && TasksDone<TasksTotal && PendingReal==0`.
- **Presentación = badge "UAT" en la card**, no columna.

Si el agente ve una alternativa mejor, la reporta como observación, no la implementa.

---

## 11. Edge cases

- **Change con todas las tasks done** → `review`, `needsUat=false` (review limpio).
- **Change con tasks de implementación pendientes** → `in-progress`, sin flag (no es review).
- **Change con `TasksDone == 0` (todas las tasks sin marcar, aunque sean de verificación)** →
  `open`, sin flag. Importante: `syncStatus` chequea `case TasksDone == 0 → open` **antes** de la
  rama `PendingReal == 0`, así que un change sin progreso nunca cae en `review`/`needsUat`. La
  fórmula exige `TasksDone > 0` justamente para alinearse con ese guard (no es bug: es la regla).
- **Una task pendiente NO reconocida por `isVerificationTask`** mantiene `PendingReal > 0`, así
  que el change queda en `in-progress` y nunca llega a `review`/`needsUat`. El discriminador es
  exactamente `PendingReal == 0` (solo quedan tasks de verificación).
- **Change sin `tasks.md` parseable** → `open` (regla actual); sin flag.
- **Card ya en `review` con `needsUat=true` y se agrega trabajo real nuevo** → al re-sync con
  `--reconcile` puede volver a `in-progress` (regla de status existente); el flag debe limpiarse
  cuando deja de estar en `review`.
- **Spec en `review` creado a mano (no por sync)** → `needsUat` ausente (false) salvo que sync lo recompute.
- **`isVerificationTask` no reconoce el wording** → conservador: no marca `needsUat` (igual que hoy no cuenta como QA). Aceptable; el clasificador se puede ampliar aparte.

---

## 12. Estados de UI requeridos

Board (read-only). Estados relevantes:

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle/success | columna `review` con cards; badge "UAT" en las marcadas | ver; (futuro) abrir detalle |
| empty | columna `review` sin cards | — |
| loading/error | igual que el board hoy (spinner / placeholder de error) | reintentar |

Sin estados `disabled`/`offline` nuevos respecto al board actual.

---

## 13. Validaciones

- **Cliente (web):** ninguna nueva (read-only). El badge es presentación derivada de `needsUat`.
- **Servidor (Go):** `needsUat` no se acepta como input de usuario en V1 (es derivado). No hay
  endpoint de escritura que lo valide.

---

## 14. Seguridad y permisos

No aplica — feature interna del dominio, sin secrets, sin endpoints de escritura, sin datos
sensibles. El flag es un bool derivado de `tasks.md` (que ya es del repo).

---

## 15. Observabilidad y logging

- El cambio a `review` ya emite `status.changed` (trigger `sync`/`apply`). **No se agrega un
  evento nuevo** en V1: `needsUat` viaja en el `status.changed`/estado, no como evento aparte.
- Open question (no bloqueante): ¿conviene un dato `needsUat` en el payload de `status.changed`
  para reconstruir histórico? Por ahora se reconstruye del `state.json` (git log).
- No loggear nada sensible (no aplica).

---

## 16. i18n / textos visibles

Proyecto sin i18n formal; textos del board hardcodeados en inglés (convención actual).

| Identificador (doc) | Texto |
|---|---|
| badge.uat | `UAT` |

(El label corto "UAT" se eligió por brevedad y reconocibilidad; alternativa "Needs UAT" queda
como open question menor.)

---

## 17. Performance

- Computar `needsUat` reusa datos que `sync` ya parseó (`openspec.Change`): costo nulo adicional.
- Un bool más en `state.json`/`board.json`: negligible.
- Un badge condicional en la card: render trivial.

---

## 18. Restricciones

El agente no debe:

- Agregar valores al enum `Status` ni columnas al board.
- Cambiar `allowedTransitions` ni la semántica de `review`.
- Introducir metadata nueva en el repo del usuario (frontmatter `requiresUAT`, etc.) — la
  detección reusa `isVerificationTask`.
- Crear un comando para editar `needsUat` a mano (fuera de scope V1).
- Refactorizar `syncStatus` más allá de lo necesario para exponer el bool.
- Romper el espejo Go↔TS del contrato del board.

---

## 19. Entregables

- [ ] `SpecState.NeedsUAT` + serialización.
- [ ] `syncNeedsUAT` + threading por `CreateSpec`/`ReconcileStatus` (set y clear). Solo en `sync`.
- [ ] `Card.NeedsUAT` en la proyección del board.
- [ ] `needsUat?` en el tipo `Card` de TS + badge "UAT" en `SpecCard` (+ CSS).
- [ ] Tests Go (cómputo del flag + proyección) y typecheck web.
- [ ] `docs/domain-contract.md` anotado (refina `review`, no estado nuevo).
- [ ] Gate verde (gofmt/vet/test Go; typecheck web).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `docs/domain-contract.md` §1/§5 y confirmé que NO agrego estado.
- [ ] Revisé `cli/internal/openspec/openspec.go` (`isVerificationTask`, `PendingReal`).
- [ ] Revisé `cli/cmd/vector/main.go` (`syncStatus`) y `cli/internal/state/store.go` (writes de sync).
- [ ] Revisé `cli/internal/board/board.go` (`toCard`) y `web/.../SpecCard` + `board.ts`.
- [ ] Implementé el campo, el cómputo, la proyección y el badge.
- [ ] Mantuve el espejo Go↔TS.
- [ ] Agregué tests del cómputo del flag.
- [ ] No agregué estado/columna/comando fuera de scope.
- [ ] Ejecuté gofmt, go vet, go test y typecheck web.
- [ ] No dejé `[...]` ni TODOs sin justificar.

---

## Open questions

- Label del badge: `UAT` (elegido) vs `Needs UAT` (más explícito). Menor.
- ¿`needsUat` también en el payload de `status.changed` (histórico) o solo en `state.json`? (§15).
  Recomendación del review: agregarlo al payload (una línea) hace el activity log self-contained
  para replay. Por ahora se deja en `state.json` para mantener el scope mínimo.
- ¿Ampliar `isVerificationTask` para cubrir más wording de UAT (p. ej. "acceptance", "uat" suelto)?
  Hoy requiere "manual" + (check|qa|test|verif) o smoke/e2e. Posible falso negativo si el repo
  escribe "UAT manual del cliente" sin esos tokens — evaluar al implementar.
- Override manual del flag (comando/drag) → fase futura, depende de la API de escritura del board.

> Resuelto en este spec (antes era open question): el path de `/vector:apply` **no** computa
> `needsUat`; el flag lo setea solo `sync`, y el finish de apply corre `vector sync --reconcile`
> para refrescarlo (ver §2 Fuera de scope).
