# Design — fix-spec-card-ticket-badge-clipping

## Decisiones clave

- **Criterio de aceptación normativo, método abierto**: el badge de ticket **nunca** se recorta y
  siempre es legible (texto completo o, como fallback, expuesto vía `title`), en cualquier longitud
  de título. El *cómo* (Opción A in-place vs. Opción B reubicación) se decide en `/vector:apply` con
  `/ui-ux-pro-max` sobre la card; este change fija el criterio, no el método.
- **Opción A — fix in-place**: `.title` gana `min-width: 0` + overflow controlado
  (ellipsis o `line-clamp`); `.ticket` gana `max-width`, `white-space: nowrap` y
  `overflow: hidden; text-overflow: ellipsis`. No toca `SpecCard.tsx` ni el drawer. Se alinea al
  patrón ya presente en el mismo módulo (`.attentionSummary`/`.attentionRow`, `.relation`) — no se
  inventa un patrón nuevo.
- **Opción B — reubicación**: retirar el badge del `<header>` y moverlo a una fila propia, o dejar
  que el drawer (que ya lo renderiza, `SpecDetailsDrawer/index.tsx:72-83`) sea la única superficie.
  Precedente directo: `RelatedChips` movido al drawer por `move-relations-to-drawer`.
- **Sin cambios de contrato**: el fix es 100% de presentación en `web/`. No se toca `Card.ticket`
  (`web/src/types/board.ts:16-20`) ni `board.Ticket` (`cli/internal/board/board.go:70-75`), ni el
  endpoint `GET /api/board`. El frontend sigue siendo proyección read-only (sin mutación de estado
  ni llamadas de red nuevas).
- **`title` attribute como red de seguridad**: el badge ya expone `title={card.ticket.url}`
  (`SpecCard.tsx:44`); se mantiene como fallback de legibilidad ante cualquier truncamiento textual.
- **Causa raíz registrada como relación**: `spec.fixed`/`relatedTo` apunta a `add-ticket-linking`
  ("la feature que hizo visible el bug"), no al commit `5441b80` que introdujo el CSS defectuoso —
  un commit no es un `kind` almacenable (`spec`|`ticket`), se documenta como contexto.

## Superficie

- `web/src/components/SpecCard/SpecCard.module.css` (MODIFICAR, siempre): Opción A → `min-width: 0`
  + overflow en `.title`; `max-width` + `white-space: nowrap` + overflow en `.ticket`. Referencia:
  `.attentionSummary`/`.attentionRow` (`:70-86`), `.relation` (`:130-142`). Sin hardcodear valores
  fuera de los tokens ya usados.
- `web/src/components/SpecCard/SpecCard.tsx` (MODIFICAR, **solo Opción B**): retirar el bloque
  `{card.ticket && (…)}` del `<header>` (`:41-49`) y renderizarlo en una región que no compita por
  espacio horizontal con el título, o retirarlo dejando el drawer como única superficie.
- `web/src/components/SpecDetailsDrawer/index.tsx` (REVISAR, **solo Opción B**): el drawer ya muestra
  el badge en `.metaRow` (`:72-83`); si se retira de la card, confirmar que no se duplica la
  superficie visible. Sin cambio funcional esperado.
- `web/src/components/SpecCard/SpecCard.test.tsx` (MODIFICAR, siempre): nuevo `describe`/`it` con
  `makeCard({ title: '<80-120 chars>', ticket: { provider, key: 'MH-1814', url } })`; assertar que
  la key completa existe en el DOM (`getByText('MH-1814')`/`getByTitle`), no `'MH-'` ni vacío.

## Flujo esperado (post-fix)

1. El board carga cards vía `useBoard` (SSE) desde `GET /api/board`; `card.ticket` llega ya poblado
   (sin cambios).
2. `SpecCard` renderiza `<header className={styles.head}>` con `.title` y, si `card.ticket` existe,
   el badge `.ticket`.
3. Bajo **cualquier** longitud de título, `.title` encoge/ellipsis (o el layout se reorganiza bajo
   Opción B) y el badge de ticket permanece completo e intacto, nunca desplazado fuera del borde.
4. El usuario ve la key completa (o su valor íntegro vía hover/`title` si hay truncamiento textual).

## Open questions (a resolver en `/vector:apply`)

1. **Criterio de wrap del título**: una línea con ellipsis vs. `line-clamp` multilínea — sin
   preferencia fijada; se decide con `/ui-ux-pro-max`.
2. **Lint de `web/`**: no hay script ni config de ESLint en el workspace; no se introduce como parte
   de este fix (fuera de scope), registrado como brecha de tooling.
3. **Barrido sistémico** de otros headers `space-between` sin `min-width: 0` de `5441b80`: riesgo de
   clipping latente, no bloqueante; candidato a spec separado.
