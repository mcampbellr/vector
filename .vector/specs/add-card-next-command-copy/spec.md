# Spec: Copyable next command on the card face

## 1. Goal

Add a compact, always-visible **copyable next-command affordance to the board card face**
(`SpecCard`), so a developer can copy the slash command to run next on a spec **without opening
the details drawer**. The command is derived from the card status via the existing
`nextCommandFor(status, id)` and shown as a single inline row (monospace command + copy button)
under the card metadata. The same command stays available in the drawer (already copyable), so the
card becomes a quick-copy shortcut and the drawer remains the full detail surface.

This feature lets a developer **copy the next command in one click from the card** for the common
case (paste into Claude Code), while keeping the drawer as the place for the AI summary, activity,
useful commands and files.

## 2. Scope

### Included in this phase

- A new card-scoped component `CardNextCommand` (web) that renders an **inline, always-visible**
  row: the next slash command (monospace, truncated with ellipsis) + a copy-to-clipboard button
  with check-icon feedback (~1.5s), reusing the existing copy pattern.
- Wiring `CardNextCommand` into `SpecCard.tsx`, rendered below the metadata footer, only when a
  next command exists for the card status.
- Ensuring the copy button **stops click propagation** so copying does not also open the drawer
  (the card itself stays clickable → opens the drawer).
- Reuse of the existing `nextCommandFor.ts` mapper (shared by card and drawer; unchanged).
- **Deletion of the orphaned** `web/src/components/SpecCard/NextCommand.tsx` and its
  `NextCommand.module.css` (the old collapsible variant, no longer imported anywhere).

### Out of scope

- The drawer's next-command section — it is **already copyable** (`CopyableCommand`); no change.
- The drawer's "Useful commands" list — unchanged.
- Any board write/mutation: the board stays a read-only projection of CLI state.
- Card redesign beyond adding this one row (no change to title, ticket, artifacts, pills, savings).
- State machine, domain model, CLI, API, or new endpoints.
- New dependencies.

The agent must not implement anything outside the scope above, even if it seems related.

---

## 3. Project technologies and conventions

### Stack

- Framework: React 19 (SPA, Vite).
- Language: TypeScript (no `any`).
- Package manager: npm (`web/` workspace).
- UI library: none — CSS Modules + CSS variables (tokens in `src/styles/tokens.css`).
- Icons: `lucide-react`.
- State management: local component state (`useState`); board data via SSE projection. No canonical
  client state.
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

- **One component per file** (global rule + `standards/typescript-react.md`); card-scoped
  subcomponents live in `web/src/components/SpecCard/` next to `SpecCard.tsx`.
- Copy-to-clipboard pattern with `navigator.clipboard` guard + check-icon feedback for ~1.5s
  (see `web/src/components/SpecDetailsDrawer/CopyableCommand.tsx` and the component being deleted,
  `NextCommand.tsx`).
- CSS Modules with design tokens (`var(--color-*)`, `var(--space-*)`, `var(--radius-*)`); copy
  button visual states mirror `NextCommand.module.css` `.copyBtn` / `.copyBtn.copied`.
- Semantic, verbose component naming (`CardNextCommand`, not `Row`/`Item`).

---

## 4. Dependencias previas

- [x] `SpecCard.tsx` is clickable and owns the `onSelect(card)` callback
      (`web/src/components/SpecCard/SpecCard.tsx`).
- [x] `nextCommandFor(status, id)` exists and returns the slash command or `null`
      (`web/src/components/SpecCard/nextCommandFor.ts`).
- [x] Drawer renders its own copyable next command via `CopyableCommand`
      (`web/src/components/SpecDetailsDrawer/index.tsx`) — the half this spec does not touch.
- [x] Copy-affordance CSS pattern exists (`web/src/components/SpecCard/NextCommand.module.css`,
      `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css`).
- [x] Design tokens available (`web/src/styles/tokens.css`).

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta.

---

## 5. Arquitectura

### Patrón a usar

Feature-first, presentation-only, component composition. A new presentational component
`CardNextCommand` owns the inline copy affordance; `SpecCard` composes it. No state beyond the
local `copied` flag. No data fetching.

### Capas afectadas

- presentation (web): **sí** — new `CardNextCommand` component + `SpecCard` composition + CSS.
- application/use-cases: **no**.
- domain: **no**.
- data/infrastructure: **no**.
- shared/common: **no**.

### Flujo esperado

1. The board renders a `SpecCard` for each spec (projection from CLI state).
2. `SpecCard` computes the next command via `nextCommandFor(card.status, card.id)`.
3. If the command is `null` (status `closed`), `CardNextCommand` renders nothing.
4. Otherwise `CardNextCommand` renders an inline row: the command (monospace, ellipsis) + copy
   button.
5. User clicks the copy button → `event.stopPropagation()` prevents the card's `onSelect`; the
   command is written to the clipboard; the button shows a check icon for ~1.5s, then reverts.
6. Clicking anywhere else on the card still calls `onSelect(card)` → opens the details drawer
   (unchanged), where the same command and the full detail remain available.

### Ubicación de archivos nuevos

```txt
web/src/components/SpecCard/
  SpecCard.tsx              (MODIFY)
  CardNextCommand.tsx       (NEW)
  CardNextCommand.module.css(NEW)
  nextCommandFor.ts         (unchanged, reused)
  NextCommand.tsx           (DELETE — orphaned)
  NextCommand.module.css    (DELETE — orphaned)
```

No crear carpetas nuevas: el componente es card-scoped y vive junto a `SpecCard.tsx`.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `web/src/components/SpecCard/CardNextCommand.tsx` | NUEVO | Inline always-visible copyable next-command row for the card face | `web/src/components/SpecDetailsDrawer/CopyableCommand.tsx` |
| `web/src/components/SpecCard/CardNextCommand.module.css` | NUEVO | Styles for the inline command + copy button (tokens) | `web/src/components/SpecCard/NextCommand.module.css` (`.body`/`.command`/`.copyBtn`) |
| `web/src/components/SpecCard/SpecCard.tsx` | MODIFICAR | Render `<CardNextCommand status id>` below the footer when a command exists | `web/src/components/SpecCard/SpecCard.tsx` (existing footer block) |

> **Orden de implementación**: crear `CardNextCommand.tsx` + `CardNextCommand.module.css` (copiando
> el look de `NextCommand.module.css`) **antes** de eliminar `NextCommand.tsx`/`.module.css`, ya que
> esos archivos son la referencia de estilo y se borran en esta misma spec.
| `web/src/components/SpecCard/NextCommand.tsx` | ELIMINAR | Orphaned collapsible variant, no longer imported | — |
| `web/src/components/SpecCard/NextCommand.module.css` | ELIMINAR | CSS of the deleted orphaned component | — |

### Detalle por archivo

#### web/src/components/SpecCard/CardNextCommand.tsx

Acción: NUEVO

Debe implementar:

- Props `{ status: Status; id: string }`.
- Compute `command = nextCommandFor(status, id)`; if `null`, `return null`.
- Render an inline row: `<code>` with the command (monospace, single line, ellipsis on overflow)
  and a copy `<button>` (`lucide-react` `Copy` → `Check` on copied).
- `handleCopy(event)`: call `event.stopPropagation()` first; guard `navigator.clipboard`; on
  success set `copied = true` and reset after 1500ms (mirror `CopyableCommand`/`NextCommand`).
- Copy button `aria-label="Copy next command"` — **static on purpose** (diverges from
  `CopyableCommand`'s dynamic `` `Copy command: ${command}` ``): on the card face the command text is
  already visible inline, so a static label avoids redundancy. Do not "correct" it to the dynamic form.

Debe seguir como referencia:

- `web/src/components/SpecDetailsDrawer/CopyableCommand.tsx` (copy logic + feedback).

No debe incluir:

- A collapse/toggle (it is always visible — that is the difference vs the deleted `NextCommand`).
- Any data fetching, board mutation, or label list (single command only).

#### web/src/components/SpecCard/CardNextCommand.module.css

Acción: NUEVO

Debe implementar:

- A compact row (`display:flex; align-items:center; gap`) with a top border separating it from the
  metadata, reusing the token-based look of `NextCommand.module.css` `.body`, `.command`, `.copyBtn`
  and `.copyBtn.copied`.

#### web/src/components/SpecCard/SpecCard.tsx

Acción: MODIFICAR

Cambios requeridos:

- Import `CardNextCommand`.
- Render `<CardNextCommand status={card.status} id={card.id} />` after the `<footer>` metadata block,
  inside the `<article>`.
- Update the component-doc comment: the current text claims the card is "metadata only" and that the
  next command lives in the drawer. Broaden it to reflect the new reality — e.g. "metadata + a
  quick-copy next command; the activity timeline, AI summary and useful commands remain in the
  drawer." Do not leave the stale "metadata only" / "next command ... in the drawer" wording.

Restricciones:

- No cambiar la metadata existente (title, ticket, artifacts, pills, priority, estimate, savings).
- No cambiar el comportamiento de click/teclado de la card (`onSelect`, Enter/Space) salvo que el
  botón de copy detenga la propagación.
- No refactorizar partes no relacionadas.

---

## 7. API Contract

No aplica — el board es una proyección read-only del estado del CLI. Esta feature es puramente de
presentación: no añade endpoints, no llama a la API y no muta estado.

---

## 8. Criterios de éxito

- [ ] On a card whose status maps to a command (`draft`/`open`/`in-progress`/`needs-attention`/
      `review`), the card shows an inline next-command row with a copy button.
- [ ] On a `closed` card the row is **absent** (`nextCommandFor` returns `null`).
- [ ] Clicking the copy button copies the exact command and shows the check icon ~1.5s, then reverts.
- [ ] Clicking the copy button does **not** open the drawer (propagation stopped).
- [ ] Clicking elsewhere on the card still opens the drawer; Enter/Space still open it.
- [ ] The drawer's next command remains present and copyable (unchanged).
- [ ] `NextCommand.tsx` and `NextCommand.module.css` are deleted and no import references them.
- [ ] No TypeScript errors; build succeeds.

### Tests requeridos

- No automated test framework is configured for `web/` (see §3). No unit tests are added in this
  phase; verification is via typecheck + build + manual check of the success criteria above. If a
  test setup is later introduced, cover: command present vs `null`, and copy-button propagation stop.

### Comandos de verificación

```bash
npm --prefix web run typecheck
npm --prefix web run build
```

La fase no está completa si alguno de estos comandos falla. Tras el build, re-embeber y reinstalar
el binario (ver §19) para dogfooding.

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
  ni se rompe la UI (mismo patrón que `CopyableCommand`).

### Navegación

- El botón de copy no navega ni abre el drawer (propagación detenida).
- El resto de la card abre el drawer (comportamiento existente, sin cambios).

### Accesibilidad

- El botón de copy tiene `aria-label="Copy next command"`.
- El botón es enfocable por teclado; el orden de tabulación dentro de la card es coherente.
- El comando se muestra en texto (no solo ícono), legible por tecnologías asistivas.
- Contraste de los estados (idle/hover/copied) según tokens existentes.

---

## 10. Decisiones tomadas

El agente no debe cuestionar ni cambiar estas decisiones (confirmadas por el usuario):

- **Ambos sitios**: quick-copy del next command en la cara de la card **y** detalle en el drawer
  (el drawer ya lo tiene copiable; no se toca).
- **Estilo en la card = botón de copy inline**: una línea siempre visible (comando + copy), **sin**
  acordeón/colapsable.
- **Eliminar siempre** el componente huérfano `NextCommand.tsx` (+ su CSS) en esta spec.
- Reusar `nextCommandFor.ts` como única fuente del mapeo status → comando (card y drawer).
- El board sigue read-only; el comando es copiable, nunca ejecutado/mutado desde la web.
- Sin nuevas dependencias.

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación pero no la
implementa.

---

## 11. Edge cases

### Datos inválidos

- No aplica — sin entrada de usuario.

### Estados del comando

- `nextCommandFor` devuelve `null` (status `closed`): `CardNextCommand` renderiza `null` (no row,
  sin borde extra).
- Comando largo: el `<code>` trunca con ellipsis en una sola línea; no expande la altura de la card.

### Clipboard no disponible

- `navigator.clipboard` ausente: el handler retorna sin efecto; no rompe la UI.

### Interacción card vs copy

- Click en el botón de copy: `event.stopPropagation()` evita disparar `onSelect` (no abre el drawer).
- Click en cualquier otra parte de la card: abre el drawer (sin cambios).

### Doble click de copy

- Reentrante e idempotente: cada click reinicia el temporizador de feedback; no hay request ni
  mutación.

### API errors / Sin conexión / Timeout / Respuesta vacía

- No aplican — la feature no llama a la API.

---

## 12. Estados de UI requeridos

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle (command present) | Card metadata + fila inline `comando + [copy]` | copiar el comando, o abrir el drawer |
| copied | El botón muestra el ícono de check ~1.5s | esperar; el estado revierte solo |
| empty (no command, `closed`) | Card metadata sin fila de next command | abrir el drawer |
| loading | No aplica | — |
| error | No aplica (guard de clipboard) | — |
| offline | No aplica | — |

---

## 13. Validaciones

No aplica — la feature no tiene entrada de usuario ni validaciones de cliente/servidor. La única
"validación" es el guard de `navigator.clipboard` antes de copiar.

---

## 14. Seguridad y permisos

- Sin nueva superficie de seguridad. El comando copiado contiene solo el id del spec (y, en su caso,
  el nombre del comando) — sin secrets ni tokens.
- No se loggea nada; la copia es una operación de cliente.
- La ejecución del comando ocurre fuera de Vector (en Claude Code), no desde la web.

---

## 15. Observabilidad y logging

No aplica — no se añade logging. La acción de copy es client-side y no genera eventos de servidor
(coherente con `architecture/state-model.md`: el board no muta estado).

---

## 16. i18n / textos visibles

El proyecto no tiene sistema de i18n; los textos visibles de la UI están hardcodeados en inglés
(patrón existente, p. ej. `CopyableCommand`/`SpecCard`). Textos de esta feature:

| Elemento | Texto |
|---|---|
| Copy button aria-label | `Copy next command` |
| Command text | el valor de `nextCommandFor(status, id)` (p. ej. `/vector:apply <id>`) |

No se introducen claves de traducción nuevas.

---

## 17. Performance

- Sin llamadas a API ni fetches; solo estado local (`copied`).
- `nextCommandFor` es O(1) (switch); se evalúa por card en render (igual que hoy en el drawer).
- Evitar renders innecesarios: el componente es puro respecto de sus props (`status`, `id`).
- No bloquea el hilo principal; el `setTimeout` solo resetea el feedback.

---

## 18. Restricciones

El agente no debe:

- Tocar el drawer (su next command ya es copiable) ni la lista de useful commands.
- Cambiar `nextCommandFor.ts` (mapeo compartido).
- Introducir mutaciones/escrituras desde la web.
- Cambiar la metadata o el comportamiento de click/teclado de la card (salvo `stopPropagation` del
  botón de copy).
- Instalar dependencias nuevas.
- Refactorizar código no relacionado ni cambiar estilos globales/tokens.
- Dejar el componente huérfano `NextCommand.tsx`/`.module.css` en el repo.
- Ignorar errores de typecheck/build.

---

## 19. Entregables

- [ ] `CardNextCommand.tsx` + `CardNextCommand.module.css` creados.
- [ ] `SpecCard.tsx` compone `CardNextCommand` y su doc-comment actualizado.
- [ ] `NextCommand.tsx` y `NextCommand.module.css` eliminados; sin imports colgando.
- [ ] Estados de UX implementados (idle/copied/empty).
- [ ] Edge cases cubiertos (null command, clipboard guard, stopPropagation).
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
- [ ] Seguí el patrón de copy de `CopyableCommand`/`NextCommand`.
- [ ] El botón de copy detiene la propagación y no abre el drawer.
- [ ] El click en el resto de la card sigue abriendo el drawer (Enter/Space también).
- [ ] La card oculta la fila cuando `nextCommandFor` devuelve `null`.
- [ ] Eliminé el componente huérfano y verifiqué que nada lo importa.
- [ ] No agregué dependencias ni cambié decisiones tomadas.
- [ ] Ejecuté typecheck y build (verdes).
- [ ] Re-embebí `web/dist` y reinstalé el binario.
- [ ] No dejé logs temporales ni TODOs sin justificar.
