# Tasks — fix-palette-ticket-key-search

## 0. Diagnóstico (previo a tocar código)

- [ ] 0.1 Correr `cd web && npm test -- matchCards.test.ts` en el entorno donde se reprodujo el
  bug y documentar el resultado (pasa/falla).
- [ ] 0.2 Si **pasa**: inspeccionar el valor real de `card.ticket?.key` en runtime (log temporal,
  nunca persistente) y compararlo contra `GET /api/board`, para decidir entre bug de datos
  (backend) o bundle desplegado (Riesgo #1).
- [ ] 0.3 Confirmar cuál de los candidatos de §6 es la causa real **antes** de editar; no tocar
  más de uno sin causa concurrente demostrada.

## 1. Fix del punto de falla confirmado (uno solo)

- [ ] 1.1 (si 0.1 falla) `web/src/components/CommandPalette/matchCards.ts`: corregir haystack o
  normalización para que fragmentos de ticket key (numérico `'1839'`, prefijo `'MH-'`, key completa
  `'mh-1839'`) matcheen. Preservar substring literal (`:11-13`); no introducir `RegExp`.
- [ ] 1.2 (si bug de datos backend) `cli/internal/board/board.go`: corregir la condición/mapeo de
  `toCard` (`:241-243`) sin cambiar el shape JSON (`:50`, `:71-75`) ni introducir escritura.
- [ ] 1.3 (si pérdida de dato en frontend) `web/src/api/useBoard.ts`: corregir el punto exacto de
  pérdida de `ticket.key` (`:26`) sin añadir mapeo/transformación innecesaria.
- [ ] 1.4 (si Riesgo #1 confirmado) `cli/internal/webui/dist/`: rebuildear `web/`, re-embeber y
  reinstalar el binario siguiendo `architecture/distribution-packaging.md`; reiniciar cualquier
  `vector serve` en marcha. Acción operativa, sin edición de fuente.
- [ ] 1.5 No tocar `index.tsx`, `PalettePriorityFilter.tsx` ni `PaletteResultRow.tsx` salvo que el
  diagnóstico los involucre (no observado por lectura de código).

## 2. Tests

- [ ] 2.1 Si el diagnóstico reveló un caso no cubierto: añadir un `it(...)` en
  `matchCards.test.ts` junto al bloque "matches by the linked ticket key" (`:103-107`) para un
  fragmento numérico puro (`matchCards(cards, '1839', [])` → card con `ticket.key: 'MH-1839'`).
- [ ] 2.2 Caso "no match": fragmento sin coincidencia sigue devolviendo `[]`/"No results".
- [ ] 2.3 No modificar ni eliminar casos existentes ni `makeCard`/fixtures compartidos.
- [ ] 2.4 Si el fix cayó en `cli/internal/board`: añadir un caso de tabla en `board_test.go`
  cubriendo `toCard` con `spec.Ticket` poblado y no poblado.

## 3. Estado / relación

- [ ] 3.1 Persistir `relatedTo: [{"kind":"spec","ref":"add-board-command-palette","source":"blame"}]`
  en el estado de esta spec vía la CLI de `cli/` (nunca editando `.vector/` a mano).

## 4. Verificación

- [ ] 4.1 `cd web && npm test -- matchCards.test.ts` en verde, incluido el caso nuevo si se añadió.
- [ ] 4.2 `cd web && npm test` (suite completa) sin regresión en los demás modos de búsqueda.
- [ ] 4.3 `cd web && npm run typecheck` (`tsc -b --noEmit`) sin errores.
- [ ] 4.4 `cd web && npm run build` sin errores (gate de embed).
- [ ] 4.5 Si se tocó `cli/internal/board`: `go -C cli vet ./...` y `go -C cli test ./...` en verde.
- [ ] 4.6 Verificar manualmente los 4 pasos de reproducción (§11 del spec) confirmando que ya no
  reproducen el bug.
- [ ] 4.7 No dejar logs temporales de diagnóstico ni TODOs sin justificar.
