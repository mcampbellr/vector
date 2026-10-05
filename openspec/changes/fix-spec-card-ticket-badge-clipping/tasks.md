# Tasks — fix-spec-card-ticket-badge-clipping

## 0. Decisión de diseño (previa a implementar)

- [ ] 0.1 Con `/ui-ux-pro-max` sobre la card, elegir Opción A (fix in-place) u Opción B
  (reubicación) y documentar cuál y por qué como observación del apply (no como cuestionamiento del
  spec).

## 1. Fix de layout

- [ ] 1.1 (Opción A) `web/src/components/SpecCard/SpecCard.module.css`: dar a `.title` (`:32-38`)
  `min-width: 0` + overflow controlado (`overflow: hidden; text-overflow: ellipsis; white-space: nowrap;`
  o `line-clamp`), siguiendo `.attentionSummary`/`.attentionRow` (`:70-86`).
- [ ] 1.2 (Opción A) Dar a `.ticket` (`:40-51`) `max-width`, `white-space: nowrap` y
  `overflow: hidden; text-overflow: ellipsis`, siguiendo `.relation` (`:130-142`). Usar tokens CSS
  ya presentes, sin hardcodear valores.
- [ ] 1.3 (Opción B, en vez de 1.1/1.2) `SpecCard.tsx`: retirar el bloque `{card.ticket && (…)}` del
  `<header>` (`:41-49`) y renderizarlo en una fila propia (análoga a `.attentionRow`) o retirarlo
  dejando el drawer como única superficie.
- [ ] 1.4 (Opción B) `SpecDetailsDrawer/index.tsx`: revisar que el badge de `.metaRow` (`:72-83`)
  sigue siendo la única superficie visible, sin duplicar lógica ni render.
- [ ] 1.5 No tocar el resto del header (`StatusPill`, `PriorityFlag`, quick-win, UAT, sketch,
  `estimate`, `savings`) ni `src/styles/tokens.css`.

## 2. Tests

- [ ] 2.1 `SpecCard.test.tsx`: nuevo `describe('SpecCard ticket badge', …)` con
  `makeCard({ title: '<título 80-120 chars>', ticket: { provider: 'linear', key: 'MH-1814', url } })`.
- [ ] 2.2 Assertar que la key completa se renderiza (`getByText('MH-1814')` o `getByTitle`), no
  `'MH-'` ni vacío; sin snapshots vacíos ni aserciones frágiles sobre `getComputedStyle`.
- [ ] 2.3 Verificar que el caso título corto + ticket sigue pasando (comportamiento existente).
- [ ] 2.4 Si Opción B retira el badge de la card, actualizar cualquier test que dependiera de verlo
  en la card.

## 3. Estado / relación

- [ ] 3.1 Persistir `relatedTo: [{"kind":"spec","ref":"add-ticket-linking","source":"manual"}]` en el
  estado de esta spec vía la CLI de `cli/` (nunca editando `.vector/` a mano).

## 4. Verificación

- [ ] 4.1 `cd web && npm run typecheck` (`tsc -b`) sin errores.
- [ ] 4.2 `cd web && npm test` (vitest run) en verde, incluido el nuevo caso.
- [ ] 4.3 `cd web && npm run build` (`tsc -b && vite build`) sin errores.
- [ ] 4.4 Verificación visual manual/`/ui-ux-pro-max`: título corto/largo, con/sin ticket → cero
  clipping, badge siempre legible.
