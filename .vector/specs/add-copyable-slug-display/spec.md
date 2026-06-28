# Spec: Copyable slug on card face and drawer

## 1. Goal

Add an **always-visible, copyable slug** (`card.id`) to the board **card face** and to the
**details drawer header**, so a developer can copy a spec's slug in one click without extracting
it from the next-command string or from the `.vector/specs/<slug>/` path. A new shared
presentational component `CopyableSlug` renders the slug as a compact monospace chip plus a
copy-to-clipboard button (Copy → Check feedback ~1.5s), reusing the existing copy pattern.

Today the slug only appears **embedded inside** the next command on the card
(`/vector:apply <id>` via `CardNextCommand`) and as a **non-copyable** `<code>` in the drawer
header (`SpecDetailsDrawer/index.tsx:55`). This feature surfaces the bare slug as a first-class,
copyable element on both surfaces, so the developer can grab the exact id (for git branches,
`.vector/specs/<slug>/` paths, ticket comments, standup notes) directly.

## 2. Scope

### Included in this phase

- A new **shared** presentational component `CopyableSlug` (`web/src/components/CopyableSlug/`)
  that renders an inline row: the slug (`<code>`, monospace, single line, ellipsis on overflow)
  + a copy `<button>` with check-icon feedback (~1.5s), reusing the existing copy pattern.
- Wiring `CopyableSlug` into **`SpecCard.tsx`**, rendered directly under the header (title/ticket
  row), so the slug sits below the title — above the attention/related/artifacts blocks.
- Wiring `CopyableSlug` into **`SpecDetailsDrawer/index.tsx`**, replacing the current
  non-copyable `<code className={styles.id}>{card.id}</code>` (line 55) in `.headerMain`, so the
  drawer header slug becomes copyable with the same affordance.
- The copy button calls `event.stopPropagation()` so copying on the card face does **not** also
  open the drawer (the card itself stays clickable → opens the drawer). Harmless in the drawer.
- **Deletion of the now-orphaned** `.id` rule in
  `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css` once line 55 stops using it.

### Out of scope

- The `CardNextCommand` row — it stays exactly as is (the slug-in-command is a separate
  affordance; this feature is complementary, not a replacement). The next command keeps embedding
  the id; no change.
- The drawer's "Useful commands" list and `CopyableCommand` — unchanged.
- Any board write/mutation: the board stays a read-only projection of CLI state.
- Card redesign beyond adding this one slug row (no change to title, ticket, artifacts, pills,
  priority, estimate, savings, next command).
- Domain model, `Card` type, CLI, API, or new endpoints (`card.id` is already in the contract).
- New dependencies.

The agent must not implement anything outside the scope above, even if it seems related.

---

## 3. Project technologies and conventions

### Stack

- Framework: React 19 (SPA, Vite).
- Language: TypeScript (no `any`).
- Package manager: npm (`web/` workspace).
- UI library: none — CSS Modules + CSS variables (tokens in `web/src/styles/tokens.css`).
- Icons: `lucide-react`.
- State management: local component state (`useState`); board data via SSE projection. No
  canonical client state.
- API client: read-only (`fetch`/SSE) — not used by this feature.
- Forms: none.
- Validation: none (no user input).
- Testing: none configured for `web/` (gate = typecheck + build). See §8.

### Versiones relevantes

- React: 19 (see `web/package.json`).
- Vite: per `web/package.json`.
- `lucide-react`: per `web/package.json`.

No usar librerías, APIs, flags o patrones que no estén ya presentes en el proyecto, salvo que este
spec lo autorice explícitamente.

### Patrones existentes a respetar

- **One component per file** (global rule + `standards/typescript-react.md`). `CopyableSlug` is
  used by **two** parents (card + drawer), so it is **not** card- or drawer-scoped: it lives in
  its own folder `web/src/components/CopyableSlug/` (sibling of `SpecCard/`, `SpecDetailsDrawer/`).
- Copy-to-clipboard pattern with `navigator.clipboard` guard + check-icon feedback for ~1.5s
  (see `web/src/components/SpecCard/CardNextCommand.tsx` and
  `web/src/components/SpecDetailsDrawer/CopyableCommand.tsx`).
- CSS Modules with design tokens (`var(--color-*)`, `var(--space-*)`, `var(--radius-*)`); copy
  button visual states mirror `CardNextCommand.module.css` `.copyBtn` / `.copyBtn.copied`.
- Semantic, verbose component naming (`CopyableSlug`, not `Slug`/`Chip`/`Row`).

---

## 4. Dependencias previas

- [x] `Card.id` is the slug (`string`) in the board contract (`web/src/types/board.ts`).
- [x] `SpecCard.tsx` is clickable and owns the `onSelect(card)` callback; it renders a `<header
      className={styles.head}>` row with title + optional ticket
      (`web/src/components/SpecCard/SpecCard.tsx`).
- [x] `SpecDetailsDrawer/index.tsx` renders the id at line 55 inside `.headerMain`
      (`<code className={styles.id}>{card.id}</code>`) — the element this spec makes copyable.
- [x] Copy pattern exists: `CardNextCommand.tsx` (with `event.stopPropagation()`) and
      `CopyableCommand.tsx` (without). `lucide-react` `Copy`/`Check` icons available.
- [x] Copy-button CSS pattern exists (`CardNextCommand.module.css` `.copyBtn` / `.copyBtn.copied`).
- [x] Design tokens available (`web/src/styles/tokens.css`): `--color-surface-muted`,
      `--color-border`, `--color-text-secondary`, `--color-text-tertiary`, `--color-accent`,
      `--radius-pill`, `--space-1`, `--space-2`.

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta.

---

## 5. Arquitectura

### Patrón a usar

Feature-first, presentation-only, component composition. A single shared presentational component
`CopyableSlug` owns the slug chip + copy affordance; `SpecCard` and `SpecDetailsDrawer` both
compose it. No state beyond the local `copied` flag. No data fetching. One component for both
surfaces (reuse before create), since the visual treatment and behavior are identical (slug below
the title, monospace chip + copy button).

### Capas afectadas

- presentation (web): **sí** — new `CopyableSlug` component + `SpecCard` composition + drawer
  composition + CSS (new module + one orphaned rule removed).
- application/use-cases: **no**.
- domain: **no**.
- data/infrastructure: **no**.
- shared/common: **no** (the component is presentational, not a shared util).

### Flujo esperado

1. The board renders a `SpecCard` for each spec (projection from CLI state); clicking a card opens
   the `SpecDetailsDrawer`.
2. Both `SpecCard` and the drawer render `<CopyableSlug slug={card.id} />` just below the title.
3. `CopyableSlug` renders an inline row: the slug (`<code>`, monospace, ellipsis) + a copy button.
4. User clicks the copy button → `event.stopPropagation()` prevents the card's `onSelect` (no
   drawer open on the card; harmless in the drawer, whose panel already stops propagation); the
   **bare slug** (`card.id`, e.g. `add-copyable-slug-display`) is written to the clipboard; the
   button shows a check icon for ~1.5s, then reverts.
5. Clicking anywhere else on the card still calls `onSelect(card)` → opens the drawer, where the
   same copyable slug is available in the header.

### Ubicación de archivos nuevos

```txt
web/src/components/CopyableSlug/
  CopyableSlug.tsx          (NEW)
  CopyableSlug.module.css   (NEW)

web/src/components/SpecCard/
  SpecCard.tsx              (MODIFY — compose CopyableSlug under the header)

web/src/components/SpecDetailsDrawer/
  index.tsx                 (MODIFY — replace the non-copyable <code> with CopyableSlug)
  SpecDetailsDrawer.module.css (MODIFY — remove the orphaned .id rule)
```

`CopyableSlug` is its own folder because it is shared by two parents; it is **not** placed inside
`SpecCard/` or `SpecDetailsDrawer/`.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `web/src/components/CopyableSlug/CopyableSlug.tsx` | NUEVO | Shared inline copyable slug chip + copy button | `web/src/components/SpecCard/CardNextCommand.tsx` |
| `web/src/components/CopyableSlug/CopyableSlug.module.css` | NUEVO | Styles for the slug chip + copy button (tokens) | `web/src/components/SpecCard/CardNextCommand.module.css` (`.command`/`.copyBtn`) |
| `web/src/components/SpecCard/SpecCard.tsx` | MODIFICAR | Render `<CopyableSlug slug={card.id} />` directly under the `<header>` | `SpecCard.tsx` (existing header/footer composition) |
| `web/src/components/SpecDetailsDrawer/index.tsx` | MODIFICAR | Replace `<code className={styles.id}>{card.id}</code>` (line 55) with `<CopyableSlug slug={card.id} />` | `SpecDetailsDrawer/index.tsx` (`.headerMain` block) |
| `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css` | MODIFICAR | Remove the now-orphaned `.id` rule | — |

### Detalle por archivo

#### web/src/components/CopyableSlug/CopyableSlug.tsx

Acción: NUEVO

Debe implementar:

- Props `{ slug: string }`.
- Local `copied` state (`useState(false)`).
- Render an inline row: `<code>` with the slug (monospace, single line, ellipsis on overflow) and
  a copy `<button>` (`lucide-react` `Copy` → `Check` on copied).
- `handleCopy(event: MouseEvent<HTMLButtonElement>)`: call `event.stopPropagation()` first; guard
  `if (!navigator.clipboard) return`; `navigator.clipboard.writeText(slug)` → on success set
  `copied = true` and reset after 1500ms (mirror `CardNextCommand`).
- Copy button `aria-label="Copy spec id"` — static (the slug text is visible inline, so a static
  label avoids redundancy; same rationale as `CardNextCommand`'s static label).
- Button class toggles `copied`: `` `${styles.copyBtn}${copied ? ` ${styles.copied}` : ''}` ``.

Debe seguir como referencia:

- `web/src/components/SpecCard/CardNextCommand.tsx` (copy logic + `stopPropagation` + feedback).

No debe incluir:

- A collapse/toggle (always visible).
- Any data fetching, board mutation, or label/command list (single slug only).
- A dynamic `aria-label` interpolating the slug (keep it static).

#### web/src/components/CopyableSlug/CopyableSlug.module.css

Acción: NUEVO

Debe implementar:

- A compact row (`display: flex; align-items: center; gap: var(--space-2)`) — **no** top border
  and **no** `margin-top`/`padding-top` separator (it sits under the title, not after metadata;
  that is the difference vs `CardNextCommand.module.css` `.body`).
- `.slug`: `flex: 1; min-width: 0;` monospace font stack (same as `CardNextCommand.module.css`
  `.command`), `font-size: 11px`, `color: var(--color-text-secondary)`,
  `background: var(--color-surface-muted)`, `border: 1px solid var(--color-border)`,
  `border-radius: var(--radius-pill)`, `padding: 3px var(--space-2)`,
  `white-space: nowrap; overflow: hidden; text-overflow: ellipsis`.
- `.copyBtn` and `.copyBtn.copied`: mirror `CardNextCommand.module.css` `.copyBtn` exactly (24×24,
  pill border, `--color-text-tertiary` idle, `--color-accent` + `#eef2ff` hover, `#047857` +
  `#d1fae5` copied).

#### web/src/components/SpecCard/SpecCard.tsx

Acción: MODIFICAR

Cambios requeridos:

- Import `CopyableSlug` from `'../CopyableSlug/CopyableSlug'`.
- Render `<CopyableSlug slug={card.id} />` as the first child of `<article>` **after** the
  `</header>` (title/ticket) block and **before** the `attentionReason` paragraph, so the slug sits
  directly under the title.
- Update the component-doc comment to mention the slug: e.g. "metadata (title, slug, ticket,
  artifacts, status, priority, estimate, savings) plus a quick-copy next command."

Restricciones:

- No cambiar la metadata existente ni el `CardNextCommand` (sigue mostrando el id en el comando).
- No cambiar el comportamiento de click/teclado de la card (`onSelect`, Enter/Space) salvo el
  `stopPropagation` del botón de copy (que vive dentro de `CopyableSlug`).
- No refactorizar partes no relacionadas. No tocar `.head` salvo lo necesario para insertar la fila.

#### web/src/components/SpecDetailsDrawer/index.tsx

Acción: MODIFICAR

Cambios requeridos:

- Import `CopyableSlug` from `'../CopyableSlug/CopyableSlug'`.
- Replace `<code className={styles.id}>{card.id}</code>` (line 55) with
  `<CopyableSlug slug={card.id} />`, keeping it inside `<div className={styles.headerMain}>` below
  the `<h2>` title.

Restricciones:

- No cambiar el resto del header (title, close button) ni el `metaRow`.
- No tocar `CopyableCommand`, `UsefulCommands` ni el timeline.

#### web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css

Acción: MODIFICAR

Cambios requeridos:

- Remove the `.id` rule (now orphaned once line 55 no longer references it). Do not remove
  `.headerMain` or `.title`.

---

## 7. API Contract

No aplica — el board es una proyección read-only del estado del CLI. Esta feature es puramente de
presentación: usa `card.id`, ya presente en el contrato (`web/src/types/board.ts`). No añade
endpoints, no llama a la API y no muta estado.

---

## 8. Criterios de éxito

- [ ] Every board card shows, directly under its title, an inline row with the slug (`card.id`,
      monospace, ellipsis) and a copy button.
- [ ] The drawer header shows the same copyable slug below the title (replacing the old
      non-copyable `<code>`).
- [ ] Clicking the copy button copies the **bare slug** (e.g. `add-copyable-slug-display`), not the
      next command, and shows the check icon ~1.5s, then reverts.
- [ ] Clicking the copy button on the card does **not** open the drawer (propagation stopped).
- [ ] Clicking elsewhere on the card still opens the drawer; Enter/Space still open it.
- [ ] `CardNextCommand` is unchanged and still present below the metadata.
- [ ] The orphaned `.id` CSS rule is removed; no `styles.id` reference remains in the drawer.
- [ ] No TypeScript errors; build succeeds.

### Tests requeridos

- No automated test framework is configured for `web/` (see §3). No unit tests are added in this
  phase; verification is via typecheck + build + manual check of the success criteria above. If a
  test setup is later introduced, cover: slug rendered, copy writes the bare id, and copy-button
  propagation stop on the card.

### Comandos de verificación

```bash
npm --prefix web run typecheck
npm --prefix web run build
```

La fase no está completa si alguno de estos comandos falla. Tras el build, re-embeber `web/dist` en
`cli/internal/webui/dist/` y reinstalar el binario (ver §19) para dogfooding.

---

## 9. Criterios de UX

### Loading

- No aplica — sin requests ni estados de carga en esta feature.

### Formularios

- No aplica — sin formularios ni inputs.

### Passwords

- No aplica.

### Errores

- Si `navigator.clipboard` no está disponible, el handler no hace nada (guard); no se muestra error
  ni se rompe la UI (mismo patrón que `CardNextCommand`/`CopyableCommand`).

### Navegación

- El botón de copy no navega ni abre el drawer (propagación detenida).
- El resto de la card abre el drawer (comportamiento existente, sin cambios).

### Accesibilidad

- El botón de copy tiene `aria-label="Copy spec id"`.
- El botón es enfocable por teclado; el orden de tabulación dentro de la card/drawer es coherente.
- El slug se muestra en texto (`<code>`), legible por tecnologías asistivas.
- Contraste de los estados (idle/hover/copied) según tokens existentes.

---

## 10. Decisiones tomadas

El agente no debe cuestionar ni cambiar estas decisiones (confirmadas por el usuario):

- **Ambos sitios**: slug copiable en la **cara de la card** y en el **header del drawer** (hoy el
  drawer lo muestra pero no es copiable; pasa a serlo).
- **Posición en la card = header, bajo el título** (no una fila al pie junto al next command).
- **Botón de copy explícito** (icono Copy → Check), no click-to-copy sobre el chip.
- **Componente único compartido** `CopyableSlug` para card y drawer (reuso antes que duplicar),
  por ser visualmente idéntico en ambas superficies.
- El `CardNextCommand` **no** se toca: el slug standalone es complementario, no reemplaza el id
  embebido en el comando.
- El board sigue read-only; el slug es copiable, nunca ejecutado/mutado desde la web.
- Sin nuevas dependencias: `lucide-react` (ya en uso) aporta los iconos `Copy`/`Check` y
  `navigator.clipboard` es nativo — no hace falta ninguna librería adicional.

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación pero no la
implementa.

---

## 11. Edge cases

### Datos inválidos

- No aplica — sin entrada de usuario. `card.id` siempre existe (es la clave del spec en el board).

### Slug largo

- Slug largo: el `<code>` trunca con ellipsis en una sola línea (`white-space: nowrap; overflow:
  hidden; text-overflow: ellipsis`); no expande la altura de la card ni del header del drawer. El
  slug completo sigue siendo copiable (se copia `card.id` íntegro, no el texto truncado visible).

### Clipboard no disponible

- `navigator.clipboard` ausente: el handler retorna sin efecto; no rompe la UI.

### Interacción card vs copy

- Click en el botón de copy: `event.stopPropagation()` evita disparar `onSelect` (no abre el drawer).
- Click en cualquier otra parte de la card: abre el drawer (sin cambios).

### Doble click de copy

- Reentrante e idempotente: cada click reinicia el temporizador de feedback; no hay request ni
  mutación.

### Redundancia con el next command

- El slug aparece tanto standalone como embebido en el comando del `CardNextCommand`. Es esperado y
  decidido (§10): cada uno sirve a un flujo distinto (copiar el id solo vs copiar el comando).

### API errors / Sin conexión / Timeout / Respuesta vacía

- No aplican — la feature no llama a la API.

---

## 12. Estados de UI requeridos

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | Fila inline `slug + [copy]` bajo el título (card y drawer) | copiar el slug, o abrir/cerrar el drawer |
| copied | El botón muestra el ícono de check ~1.5s | esperar; el estado revierte solo |
| loading | No aplica | — |
| error | No aplica (guard de clipboard) | — |
| offline | No aplica | — |

---

## 13. Validaciones

No aplica — la feature no tiene entrada de usuario ni validaciones de cliente/servidor. La única
"validación" es el guard de `navigator.clipboard` antes de copiar.

---

## 14. Seguridad y permisos

- Sin nueva superficie de seguridad. El slug copiado es el id público del spec en el board — sin
  secrets ni tokens.
- No se loggea nada; la copia es una operación de cliente.
- El slug no se ejecuta; copiarlo es una acción de cliente sin efectos en el servidor.

---

## 15. Observabilidad y logging

No aplica — no se añade logging. La acción de copy es client-side y no genera eventos de servidor
(coherente con `architecture/state-model.md`: el board no muta estado).

---

## 16. i18n / textos visibles

El proyecto no tiene sistema de i18n; los textos visibles de la UI están hardcodeados en inglés
(patrón existente, p. ej. `CardNextCommand`/`CopyableCommand`). Textos de esta feature:

| Elemento | Texto |
|---|---|
| Copy button aria-label | `Copy spec id` |
| Slug text | el valor de `card.id` (p. ej. `add-copyable-slug-display`) |

No se introducen claves de traducción nuevas.

---

## 17. Performance

- Sin llamadas a API ni fetches; solo estado local (`copied`).
- `CopyableSlug` es puro respecto de su prop (`slug`); se renderiza una vez por card y una vez en el
  drawer abierto.
- No bloquea el hilo principal; el `setTimeout` solo resetea el feedback.

---

## 18. Restricciones

El agente no debe:

- Tocar el `CardNextCommand` (sigue mostrando el id embebido en el comando) ni la lista de useful
  commands del drawer.
- Cambiar el `Card` type ni el contrato de la API (`card.id` ya existe).
- Introducir mutaciones/escrituras desde la web.
- Cambiar la metadata o el comportamiento de click/teclado de la card (salvo `stopPropagation` del
  botón de copy dentro de `CopyableSlug`).
- Instalar dependencias nuevas.
- Refactorizar código no relacionado ni cambiar estilos globales/tokens.
- Dejar reglas CSS huérfanas (`.id`) tras el cambio.
- Ignorar errores de typecheck/build.

---

## 19. Entregables

- [ ] `CopyableSlug.tsx` + `CopyableSlug.module.css` creados en `web/src/components/CopyableSlug/`.
- [ ] `SpecCard.tsx` compone `CopyableSlug` bajo el header y su doc-comment actualizado.
- [ ] `SpecDetailsDrawer/index.tsx` reemplaza el `<code>` no copiable por `CopyableSlug`.
- [ ] Regla CSS `.id` huérfana eliminada de `SpecDetailsDrawer.module.css`.
- [ ] Estados de UX implementados (idle/copied).
- [ ] Edge cases cubiertos (slug largo/ellipsis, clipboard guard, stopPropagation).
- [ ] `npm --prefix web run typecheck` verde.
- [ ] `npm --prefix web run build` exitoso.
- [ ] `web/dist` re-embebido en `cli/internal/webui/dist/` y binario reconstruido + reinstalado en
      `~/.local/bin/vector` (dogfooding).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] No aplica `docs/api-contract.md` (feature read-only sin API).
- [ ] Confirmé que todas las dependencias previas existen.
- [ ] Solo modifiqué/creé/eliminé los archivos listados en §6.
- [ ] Seguí el patrón de copy de `CardNextCommand`/`CopyableCommand`.
- [ ] El botón de copy copia el slug bare (no el comando) y detiene la propagación.
- [ ] El click en el resto de la card sigue abriendo el drawer (Enter/Space también).
- [ ] El slug se muestra bajo el título en la card y en el drawer header.
- [ ] Eliminé la regla CSS `.id` huérfana y verifiqué que nada la referencia.
- [ ] No agregué dependencias ni cambié decisiones tomadas.
- [ ] Ejecuté typecheck y build (verdes).
- [ ] Re-embebí `web/dist` y reinstalé el binario.
- [ ] No dejé logs temporales ni TODOs sin justificar.
