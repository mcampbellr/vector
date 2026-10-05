# Design — fix-palette-ticket-key-search

## Decisiones clave

- **Diagnóstico antes que fix (normativo)**: se corre el Test Plan del brief
  (`matchCards.test.ts:103-107`) **primero** para decidir entre "bug de lógica" en `matchCards.ts`
  vs. "bug de datos/deploy" en runtime. Razón: evita adivinar la causa y tocar el archivo
  equivocado sobre una cadena que, por lectura de código, luce mayormente correcta.
- **Fix acotado a un único punto de falla confirmado**, no a los candidatos de §6 a la vez.
  Razón: minimizar blast radius (§5 del spec).
- **Sin `RegExp` en `matchCards`**: se mantiene el substring literal aunque el fix toque esa
  función. Razón: comentario explícito (`matchCards.ts:11-13`) y test dedicado a metacaracteres
  literales (`matchCards.test.ts:65-73`).
- **Sin cambio de contrato**: no se altera el shape de `Board`/`Card`/`Ticket` (Go o TS); a lo
  sumo se corrige población de datos dentro del shape existente. `internal/board` sigue siendo
  proyección read-only (sin escritura nueva).
- **Causa raíz registrada como relación**: `relatedTo` `kind=spec`, `ref=add-board-command-palette`,
  `source=blame` (el change que introdujo la búsqueda por ticket key en `e94d6fb`; el
  `CommandPalette` se introdujo en `b1c9e2a`). Un commit no es un `kind` almacenable — se documenta
  como contexto.

## Superficie (condicional — solo la capa que confirme el diagnóstico)

- `web/src/components/CommandPalette/matchCards.ts` (MODIFICAR, condicional): si el paso 1 del
  diagnóstico falla, corregir haystack/normalización para que fragmentos de ticket key (numérico,
  prefijo, key completa case-insensitive) matcheen. Referencia: el propio `foldDiacritics` +
  `includes` (`:3-26`); no introducir `RegExp`.
- `web/src/components/CommandPalette/matchCards.test.ts` (MODIFICAR, condicional): añadir un
  `it(...)` junto al bloque "matches by the linked ticket key" (`:103-107`) cubriendo un fragmento
  **puramente numérico** (`matchCards(cards, '1839', [])` contra `ticket.key: 'MH-1839'`),
  replicando el patrón `toEqual([...])`. No tocar los casos existentes ni `makeCard`/fixtures.
- `web/src/api/useBoard.ts` (MODIFICAR, condicional, baja probabilidad): corregir el punto exacto
  donde `ticket.key` se pierda entre el evento SSE y `setBoard` (`:26`). Hoy es `JSON.parse`
  directo sin transformación; no introducir un mapeo nuevo si el parse ya preserva el campo.
- `cli/internal/board/board.go` (MODIFICAR, condicional): corregir la condición de `toCard`
  (`:241-243`, `if spec.Ticket != nil { card.Ticket = &Ticket{...} }`) si `spec.Ticket` no llega
  poblado o la condición descarta casos válidos. No cambiar el shape JSON
  (`json:"ticket,omitempty"`, `:50`) ni el de `Ticket` (`:71-75`); mantener el espejo con
  `web/src/types/board.ts:16-20`. Sin escritura nueva.
- `cli/internal/webui/dist/` (ACCIÓN operativa, sin edición de fuente): si se confirma el Riesgo #1
  (bundle desplegado anterior a `e94d6fb`), rebuildear `web/`, re-embeber y reinstalar el binario
  siguiendo `architecture/distribution-packaging.md` §"Flujo de edición del frontend"; reiniciar
  cualquier `vector serve` en marcha (sirve el binario viejo hasta reiniciar).

## Flujo esperado (diagnóstico → fix → verificación)

1. `cd web && npm test -- matchCards.test.ts` — decidir si la suite de regresión pasa o falla.
2. **Falla** (`:103-112` rojo): causa en `matchCards.ts`; corregir haystack/normalización hasta que
   pase, sin romper los demás casos.
3. **Pasa**: causa de datos/deploy. Verificar en orden:
   a. Valor real de `card.ticket?.key` en el navegador (log temporal, nunca persistente — §15)
      contra lo servido por `GET /api/board`.
   b. Si llega vacío/undefined en el JSON servido pese a `spec.Ticket` presente → bug en
      `board.go:toCard` o `state.Store`; corregir ahí.
   c. Si llega correcto en `GET /api/board` pero no en el render → bundle desplegado (Riesgo #1);
      re-embeber y reinstalar, sin cambio de código fuente.
4. Aplicar el fix acotado al punto confirmado (uno solo, salvo causa concurrente demostrada).
5. Si aparece un caso no cubierto (fragmento numérico puro `'1839'`), añadirlo a la suite.
6. `npm test -- matchCards.test.ts` (y suite completa) en verde antes de cerrar.

## Open questions (a resolver en `/vector:apply`)

1. ¿`matchCards.test.ts:103-107` pasa o falla en el entorno donde se reprodujo el bug? — determina
   lógica vs. dato/deploy. TBD.
2. ¿Valor real de `card.ticket?.key` en runtime para la card `MH-1839`? TBD (inspección
   navegador/API en vivo).
3. ¿El bundle web desplegado corresponde a `e94d6fb` o posterior? TBD (verificar `dist` embebido
   contra git).
4. ¿Todas las cards llegan con `ticket.key` poblado, o puede llegar `undefined`/`null` pese a
   existir `ticket` en el estado persistido? TBD (inspección `state.Store` / `GET /api/board`).

> Nota: la lectura del código en el estado actual del repo muestra toda la cadena correcta
> (haystack incluye `ticket.key`; `toCard` puebla `Ticket`; `useBoard.ts` no transforma el
> payload). Esto **refuerza** el Riesgo #1 (bundle desactualizado) como hipótesis más probable,
> pero no lo confirma.
