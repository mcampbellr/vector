# Spec: Modal de lectura de markdown redimensionable desde los bordes derecho e inferior

## 1. Objetivo

Construir tres manejadores de redimensionado (borde derecho, borde inferior, esquina
inferior-derecha) sobre el modal de previsualización de markdown existente
(`FilePreviewModal.tsx`), de modo que el usuario pueda ampliar la ventana de lectura
arrastrando desde esos bordes, con el tamaño persistido en `localStorage` entre reaperturas.

Esta feature permite que un **developer** pueda **ampliar el modal de previsualización** de
archivos markdown para leer contenido extenso con más espacio vertical u horizontal, y que el
modal **recuerde el tamaño elegido** entre sesiones sin necesidad de ninguna dependencia
externa.

## 2. Alcance

### Incluido en esta fase

- Tres handles de arrastre: borde derecho (ajusta solo ancho), borde inferior (ajusta solo
  alto), esquina inferior-derecha (ajusta ambos simultáneamente).
- Implementación mediante eventos nativos del DOM: `mousedown` en el handle →
  `mousemove`/`mouseup` registrados en `document` → `mouseup` elimina los listeners.
  Throttle vía `requestAnimationFrame` para evitar re-renders en exceso.
- Restricción de bounds: floor = tamaño por defecto (720 px de ancho; `window.innerHeight *
  0.86` px de alto, calculado al montar). El modal solo crece; nunca se puede encoger por
  debajo del floor. Ceiling ~95 vw / ~95 vh, recalculado en cada evento de drag.
- Persistencia del tamaño en `localStorage` con clave `vector:file-preview-modal:size` (un
  tamaño global para el modal, no por archivo). Restauración al reabrir con re-clamp si el
  valor almacenado es inválido o excede el viewport actual.
- `localStorage` protegido con `try/catch`; entornos sin storage caen a tamaño en memoria
  sin error.
- Handles ocultos en viewports ≤ ~640 px o `pointer: coarse` vía media query CSS; el modal
  mantiene su sizing responsivo original en esos entornos.
- Atributos de accesibilidad en los handles: `role="separator"`, `tabIndex={0}`,
  `aria-label` en español por eje; cursores `col-resize`, `row-resize`, `se-resize`.
- Promoción de `FilePreviewModal.tsx` a carpeta `FilePreviewModal/` si se extrae
  `ResizeHandle.tsx` (según complejidad de implementación).

### Fuera de scope

- Redimensionado desde bordes izquierdo o superior.
- Propiedad CSS `resize: both` o cualquier variante de CSS native resize.
- Soporte táctil o `pointer: coarse` (esta fase es desktop-only).
- Redimensionado con teclas de flecha (deferred enhancement, documentado en Open questions).
- Tamaños por archivo (la persistencia es un único tamaño global para el modal).
- Cambios al comportamiento de cierre (Escape, clic en overlay, botón X), focus management,
  code-splitting de `MarkdownView` y scroll de `.modalBody`.
- Cambios al API HTTP, al binario CLI, al JSON de estado ni a ningún componente fuera de
  `FilePreviewModal` y `SpecDetailsDrawer.module.css`.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca
relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Framework: React 19.1.0 (sin librerías de drag externas; eventos nativos del DOM)
- Lenguaje: TypeScript 5.7.2 (strong typing; sin `any`)
- Bundler: Vite 6.0.0
- Package manager: npm
- Estilos: CSS Modules (`SpecDetailsDrawer.module.css`), design tokens CSS del proyecto
- Testing: Vitest `^4.1.9` (confirmado en `web/package.json`, script `test`: `vitest run`);
  añadir cobertura para esta feature es TBD — ver Open questions #2.
- State management: estado local con `useState` + `useRef` para drag; sin Zustand ni
  React Query involucrados en esta feature

### Versiones relevantes

- React: 19.1.0 (declarado en `web/package.json`)
- TypeScript: 5.7.2 (declarado en `web/package.json` o `web/tsconfig.json`)
- Vite: 6.0.0 (declarado en `web/package.json`)

No instalar dependencias nuevas. No usar librerías, APIs ni patrones no presentes en el
proyecto. Todo el drag usa exclusivamente la Web API nativa.

### Patrones existentes a respetar

- **One-component-per-file** (regla global del usuario): si se extrae `ResizeHandle`,
  promover `FilePreviewModal.tsx` a carpeta `FilePreviewModal/index.tsx` con
  `ResizeHandle.tsx` como archivo hermano.
- **CSS Modules**: los estilos del componente viven en `SpecDetailsDrawer.module.css`; no
  añadir estilos globales ni crear archivos CSS nuevos fuera de esa convención.
- **Design tokens**: usar `--color-border`, `--space-*`, `--shadow-*` del proyecto para los
  handles; no hardcodear colores ni espaciados literales.
- **Cleanup en `useEffect`**: registrar listeners de `document` dentro de un `useEffect` cuyo
  cleanup los elimina si quedan activos (aunque el flujo normal los limpia en `mouseup`).
- **Sin `any` en TypeScript**: tipar el estado de tamaño con una interface explícita
  (`interface ModalSize { width: number; height: number }`).
- **Focus management existente**: el componente restaura el foco al elemento previo al cerrar;
  no debe alterarse.
- **Clave de `localStorage` en kebab-case con namespace**: `vector:file-preview-modal:size`.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx` — componente existente con
      clases `.modalPanel` (fixed width `min(720px, 94vw)`, `max-height: 86vh`), `.modalOverlay`
      (z-index 60, fixed), `.modalBody`, cierre por Escape/overlay/X, focus management.
- [x] `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css` — archivo de
      estilos con las clases `.modalPanel`, `.modalOverlay`, `.modalBody` ya definidas.
- [x] Scripts `npm run typecheck` y `npm run build` funcionales desde `web/`.
- [x] Design tokens CSS (`--color-border`, `--space-*`, `--shadow-*`) disponibles en el
      proyecto y accesibles desde `SpecDetailsDrawer.module.css`.

Si alguna dependencia no existe, el agente debe detenerse y reportar qué falta. No inventar
rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

**Estado local en el componente + eventos DOM nativos**: el tamaño del modal vive en
`useState<ModalSize>` dentro de `FilePreviewModal`. El arrastre se gestiona con `mousedown`
en cada handle → listeners `mousemove`/`mouseup` registrados en `document` (para capturar
eventos fuera del elemento) → `mouseup` limpia los listeners. `requestAnimationFrame` throttlea
las actualizaciones de estado al ritmo de la pantalla. El tamaño se persiste en `localStorage`
al soltar el ratón (`mouseup`), no en cada frame.

### Capas afectadas

- presentation (`web/src/components/SpecDetailsDrawer/`): **sí** — `FilePreviewModal.tsx` (o
  carpeta `FilePreviewModal/`) y `SpecDetailsDrawer.module.css`.
- application/use-cases: **no** — no hay lógica de negocio nueva.
- domain: **no** — `ModalSize` es un tipo UI interno, no un tipo de dominio.
- data/infraestructura: **no** — `localStorage` es persistencia de preferencia de cliente
  sin API ni JSON de estado involucrados.
- shared/common: **no** — `ResizeHandle` (si se extrae) vive dentro de la carpeta del
  componente, no en shared.

### Flujo esperado

1. El usuario abre el modal de previsualización de un archivo markdown.
2. `FilePreviewModal` monta; llama a `loadPersistedSize()`: lee `localStorage`, valida el
   JSON y los valores, re-clampa al viewport actual. Si falla o no existe, retorna `null` y
   arranca con el default (`{ width: 720, height: window.innerHeight * 0.86 }`).
3. El modal renderiza con `style={{ width: `${size.width}px`, height: `${size.height}px` }}`
   sobre `.modalPanel`.
4. El usuario presiona `mousedown` sobre uno de los tres handles.
5. El handler registra `startX`, `startY`, `startWidth`, `startHeight` y el eje del handle
   activo. Registra `onMouseMove` y `onMouseUp` en `document`. Cancela el evento para
   evitar selección de texto.
6. En cada `mousemove`, dentro de un `requestAnimationFrame`, calcula el nuevo ancho/alto
   según el delta (solo el eje correspondiente al handle activo), aplica
   `Math.max(floor, Math.min(delta, ceiling))` y actualiza el estado.
7. En `mouseup`, elimina los listeners de `document` y llama a `savePersistedSize(size)`
   (con `try/catch`).
8. Al cerrar el modal (Escape / overlay / X), el comportamiento de cierre y focus management
   permanecen sin cambios. Si hay un drag activo al cerrar, el `useEffect` cleanup elimina
   los listeners huérfanos.

### Ubicación de archivos nuevos

Si se extrae `ResizeHandle`:

```txt
web/src/components/SpecDetailsDrawer/
  FilePreviewModal/
    index.tsx           ← componente principal (contenido de FilePreviewModal.tsx)
    ResizeHandle.tsx    ← handle reutilizable parametrizado por eje
  SpecDetailsDrawer.module.css  ← permanece en el directorio padre (sin mover)
```

Si los handles caben limpiamente como JSX inline sin aumentar significativamente la
complejidad de `FilePreviewModal.tsx`, se pueden dejar inline y no crear la carpeta.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx` | MODIFICAR | Añadir interface `ModalSize`, estado, lógica de drag para tres handles, helpers de localStorage, inline style en `.modalPanel`, render de los handles. | `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx` (patrón existente de `useEffect` con cleanup + focus management) |
| `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css` | MODIFICAR | `position: relative` en `.modalPanel`; reemplazar `width` fija por `max-width` de seguridad; clases de handles con posición absoluta, cursores y color; media query para ocultar handles en viewport pequeño. | `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css` (clases `.modalPanel`, `.modalOverlay` existentes como referencia de estilo) |
| `web/src/components/SpecDetailsDrawer/FilePreviewModal/ResizeHandle.tsx` | NUEVO (condicional) | Componente `ResizeHandle` con `role`, `tabIndex`, `aria-label`, `onMouseDown`, clase CSS por eje. Solo se crea si la lógica justifica la extracción. | `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx` (patrón de componente funcional TS sin estado propio) |

### Detalle por archivo

#### web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx

Acción: MODIFICAR

Debe implementar:

- `interface ModalSize { width: number; height: number }`.
- Constantes de bounds: `DEFAULT_WIDTH = 720`, `DEFAULT_HEIGHT = window.innerHeight * 0.86`
  (calculado una vez al inicializar el estado, no en cada render), `MAX_WIDTH_RATIO = 0.95`,
  `MAX_HEIGHT_RATIO = 0.95`.
- `function loadPersistedSize(): ModalSize | null` — lee `localStorage['vector:file-preview-modal:size']`
  con `try/catch`, hace `JSON.parse`, verifica que `width` y `height` son `number` y
  `isFinite`, re-clampa al viewport actual (`Math.max(DEFAULT_WIDTH, Math.min(v, window.innerWidth * 0.95))`
  para el ancho, análogo para el alto). Retorna `null` en cualquier caso de error.
- `function savePersistedSize(size: ModalSize): void` — `JSON.stringify` + `localStorage.setItem`
  con `try/catch` silencioso.
- `useState<ModalSize>` inicializado con `() => loadPersistedSize() ?? { width: DEFAULT_WIDTH, height: DEFAULT_HEIGHT }`.
- Tres handlers de `mousedown` (uno por eje: `'right'`, `'bottom'`, `'corner'`) que registran
  `onMouseMove` y `onMouseUp` en `document`, cancela `e.preventDefault()` para evitar
  selección de texto durante el drag.
- En `onMouseMove`: leer el delta respecto a la posición inicial, calcular el nuevo tamaño
  clampado, actualizar el estado dentro de `requestAnimationFrame`. Solo actualizar los ejes
  que correspondan al handle activo.
- En `onMouseUp`: eliminar los listeners de `document` y llamar a `savePersistedSize`.
- `useRef` para el `frameId` de `requestAnimationFrame` (para cancelar si `mouseup` llega
  antes del siguiente frame).
- `useEffect` con cleanup que elimina `onMouseMove` y `onMouseUp` de `document` si el
  componente se desmonta durante un drag.
- `style={{ width: `${size.width}px`, height: `${size.height}px` }}` sobre `.modalPanel`.
- Render de los tres handles (como JSX inline o como `<ResizeHandle>`).

No debe incluir:

- Cambios al handler de Escape, al handler de clic en overlay, al botón X, al focus
  management, al code-splitting de `MarkdownView` ni al scroll de `.modalBody`.
- Handlers de teclado para redimensionar (fuera de scope esta fase).
- Lógica de touch o `pointer: coarse` (se delega a media query CSS).

Restricciones:

- No refactorizar partes del componente no relacionadas con esta feature.
- Mantener compatibilidad total con el flujo existente de apertura/cierre.
- `loadPersistedSize` y `savePersistedSize` pueden ser funciones puras de módulo (fuera del
  componente) para facilitar su testeo unitario.

#### web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css

Acción: MODIFICAR

Cambios requeridos:

- Añadir `position: relative` a `.modalPanel` (necesario para que los handles absolutamente
  posicionados funcionen correctamente dentro del panel).
- Reemplazar la declaración `width: min(720px, 94vw)` de `.modalPanel` por ausencia de `width`
  fija (el ancho lo controla el inline style desde el estado). Mantener `max-width: 95vw` como
  fallback CSS de seguridad para el caso en que el inline style no esté presente.
- Eliminar o reemplazar `max-height: 86vh` de `.modalPanel` por `max-height: none` (el control
  de bounds pasa a ser por lógica JS + inline style).
- Añadir clases para los tres handles:

  ```css
  .resizeHandleRight   { position: absolute; top: 0; right: 0; width: TBD — ver Open questions; height: 100%; cursor: col-resize; … }
  .resizeHandleBottom  { position: absolute; bottom: 0; left: 0; width: 100%; height: TBD — ver Open questions; cursor: row-resize; … }
  .resizeHandleCorner  { position: absolute; bottom: 0; right: 0; width: TBD — ver Open questions; height: TBD — ver Open questions; cursor: se-resize; … }
  ```

  Usar `--color-border` (con `opacity` o como `background` semitransparente). Como los handles
  son hijos `position: absolute` de `.modalPanel` (`position: relative`), basta `z-index: 1`
  para situarlos sobre el contenido de `.modalBody` dentro del panel; no hay comparación con el
  z-index 60 del overlay (es un ancestro en otro contexto de apilamiento).

- Media query para ocultar los handles en viewport pequeño o puntero grueso:

  ```css
  @media (max-width: 640px), (pointer: coarse) {
    .resizeHandleRight,
    .resizeHandleBottom,
    .resizeHandleCorner { display: none; }
  }
  ```

Restricciones:

- No modificar `.modalOverlay`, `.modalBody`, `.modalHeader` ni ninguna otra clase existente
  no listada explícitamente.
- No introducir variables CSS nuevas; usar únicamente las ya definidas en el proyecto.
- No cambiar el z-index del overlay (60) ni del panel.

#### web/src/components/SpecDetailsDrawer/FilePreviewModal/ResizeHandle.tsx

Acción: NUEVO (condicional — solo si la lógica se extrae)

Debe implementar:

- `interface ResizeHandleProps { axis: 'right' | 'bottom' | 'corner'; onMouseDown: (e: React.MouseEvent<HTMLDivElement>) => void; }`.
- Componente funcional `ResizeHandle({ axis, onMouseDown }: ResizeHandleProps)`.
- `role="separator"` con `aria-orientation` según eje (`"vertical"` para `right`,
  `"horizontal"` para `bottom`; sin `aria-orientation` para `corner`, que también usa
  `role="separator"` — decisión cerrada en §10, no reabrir).
- `tabIndex={0}`.
- `aria-label` según eje: `"Redimensionar ancho"` (right), `"Redimensionar alto"` (bottom),
  `"Redimensionar modal"` (corner).
- Clase CSS derivada del `axis` prop (importada del módulo CSS del padre o de un módulo propio
  si la carpeta se crea).

Debe seguir como referencia:

- El patrón de componente funcional sin estado propio de `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx`.

No debe incluir:

- Lógica de cálculo de tamaño (vive en el padre).
- Handlers de teclado.
- Estado propio.

---

## 7. API Contract

No aplica — este cambio es puro UI. No se introduce ni modifica ningún endpoint HTTP, contrato
de API, JSON de estado ni estructura de `.vector/specs/`. El único almacenamiento externo es
`localStorage` en el cliente, que persiste `{ width: number, height: number }` bajo la clave
`vector:file-preview-modal:size`. Ningún backend consume ese valor.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] Arrastrar el handle derecho aumenta el ancho del modal en tiempo real; el alto no cambia.
- [ ] Arrastrar el handle inferior aumenta el alto del modal en tiempo real; el ancho no cambia.
- [ ] Arrastrar el handle de esquina aumenta ancho y alto simultáneamente en tiempo real.
- [ ] El modal no puede encogerse por debajo del tamaño por defecto (720 px de ancho;
      `window.innerHeight * 0.86` px de alto al montar) mediante arrastre.
- [ ] El modal no excede ~95 vw de ancho ni ~95 vh de alto; el clamp funciona en el límite
      superior.
- [ ] Al cerrar el modal y reabrirlo, el tamaño se restaura desde `localStorage`; no se
      resetea al default si hay un tamaño válido persistido.
- [ ] Si el valor en `localStorage` es JSON inválido, el modal arranca con el default y no
      lanza error en consola.
- [ ] Si el valor en `localStorage` tiene números fuera de bounds del viewport actual, se
      re-clampa y el modal arranca normalmente.
- [ ] Si `localStorage` no está disponible, el drag funciona en memoria sin crash.
- [ ] Los handles no aparecen (ni reciben eventos) en viewport ≤ 640 px o con `pointer: coarse`.
- [ ] Cerrar con Escape, clic en overlay y botón X funciona exactamente igual que antes.
- [ ] El foco se restaura al elemento previamente enfocado tras cerrar (sin cambios).
- [ ] El scroll de `.modalBody` y el code-splitting de `MarkdownView` funcionan sin cambios.
- [ ] `npm run typecheck` pasa sin errores desde `web/`.
- [ ] `npm run build` pasa sin errores desde `web/`.

### Tests requeridos

TBD — ver Open questions (depende de si Vitest está configurado en `web/`).

Cuando el framework esté disponible, agregar tests para:

- [ ] `loadPersistedSize` con JSON válido y valores dentro de bounds → devuelve el tamaño.
- [ ] `loadPersistedSize` con JSON malformado → retorna `null` sin throw.
- [ ] `loadPersistedSize` con valores fuera del viewport actual → retorna el tamaño re-clamped.
- [ ] `loadPersistedSize` sin entrada en `localStorage` → retorna `null`.
- [ ] Función de clamp: valor por debajo del floor → floor; valor por encima del ceiling → ceiling.

### Comandos de verificación

Ejecutar desde `web/`:

```bash
npm run typecheck
npm run build
```

Si Vitest está configurado en `web/`:

```bash
npm run test
```

La fase no está completa si `typecheck` o `build` fallan.

---

## 9. Criterios de UX

### Drag experience

- El cursor cambia a `col-resize` al hacer hover sobre el handle derecho.
- El cursor cambia a `row-resize` al hacer hover sobre el handle inferior.
- El cursor cambia a `se-resize` al hacer hover sobre la esquina inferior-derecha.
- Durante el arrastre, el cursor del eje activo se fija en `document.body` para evitar que
  cambie al pasar sobre texto u otros elementos; se restaura en `mouseup`.
- El modal se redimensiona en tiempo real siguiendo el puntero, sin lag perceptible
  (`requestAnimationFrame`).
- Al soltar, el tamaño queda fijado y se persiste en `localStorage`.

### Comportamiento visual de los handles

- Los handles son visualmente discretos: una franja delgada usando `--color-border` con
  opacidad reducida. El grosor visual exacto es TBD — ver Open questions (referencia de
  partida: 6 px visibles con un área de hit de al menos 8–12 px vía `padding` o `::after`
  para facilitar la interacción).
- En viewport ≤ 640 px o `pointer: coarse`: handles ocultos (`display: none`); el modal
  vuelve a su sizing responsivo original sin `width`/`height` de inline style (o con el
  default, que es equivalente al original en esos breakpoints).

### Accesibilidad

- Handle derecho: `role="separator"` `aria-orientation="vertical"` `tabIndex={0}`
  `aria-label="Redimensionar ancho"`.
- Handle inferior: `role="separator"` `aria-orientation="horizontal"` `tabIndex={0}`
  `aria-label="Redimensionar alto"`.
- Handle esquina: `role="separator"` (sin `aria-orientation`) `tabIndex={0}`
  `aria-label="Redimensionar modal"`.
- El foco de teclado puede alcanzar los handles (para futura mejora con teclas de flecha),
  pero el redimensionado por teclado no se implementa en esta fase.

### Sin regresiones

- Escape, clic en overlay y botón X cierran el modal sin cambios en comportamiento ni en
  animación.
- El scroll de `.modalBody` sigue funcionando durante y después del drag.
- El foco se restaura al elemento previo tras cerrar.
- La selección de texto está bloqueada durante el drag (`e.preventDefault()` en `mousedown`).

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas; el agente no debe cuestionarlas ni cambiarlas:

- **Eventos nativos del DOM** (mousedown / mousemove / mouseup en `document`): no usar
  librerías de drag externas ni la propiedad CSS `resize`. Razón: sin dependencias nuevas,
  control total de bounds y accesibilidad.
- **Solo tres handles** (derecho, inferior, esquina): bordes izquierdo y superior fuera de
  scope. Razón: el caso de uso es ampliar el modal para leer más contenido; reducirlo tiene
  menos valor y añade complejidad de posicionamiento.
- **Floor = tamaño por defecto**: 720 px de ancho y `window.innerHeight * 0.86` px de alto,
  calculado al montar (constante por sesión de apertura). El usuario no puede encoger por
  debajo de ese tamaño. Razón: preservar la usabilidad mínima del modal.
- **Ceiling ~95 vw / ~95 vh**: recalculado en cada evento de drag para respetar
  redimensionamientos de la ventana del navegador entre frames. Razón: el modal nunca debe
  escapar del viewport.
- **Persistencia en `localStorage`** con clave `vector:file-preview-modal:size`. Un tamaño
  global (no por archivo). Razón: la preferencia de tamaño del usuario es de UX, no de
  contenido; un tamaño global es suficiente.
- **Re-clamp al cargar desde `localStorage`**: si el valor almacenado excede el viewport
  actual (p.ej. entre sesiones con ventana más pequeña), se re-clampa silenciosamente. Razón:
  robustez sin error visible al usuario.
- **`try/catch` en lectura y escritura de `localStorage`**: entornos sin storage caen a
  comportamiento en memoria. Razón: no crashear en contextos restrictivos (modo privado, etc.).
- **Desktop-only**: handles ocultos en `@media (max-width: 640px), (pointer: coarse)`. Razón:
  el drag con ratón no tiene análogo táctil directo; no implementar a medias.
- **Sin redimensionado por teclado** en esta fase: documentado en Open questions como deferred
  enhancement. Razón: añade complejidad de interacción y manejo de focus que merece su propio
  spec.
- **`role="separator"` con `aria-orientation`** en handles horizontales/verticales; sin
  `aria-orientation` en la esquina. Razón: `separator` describe semánticamente un divisor de
  área ajustable; es más preciso que `button` para este patrón.
- **`requestAnimationFrame`** para throttle de `mousemove`: actualiza el estado una vez por
  frame. Razón: evita re-renders en exceso sin introducir utilidades de throttle externas.
- **`position: relative` en `.modalPanel`** y tamaño por inline style: el CSS deja de
  controlar `width`/`height` fijos; los controla el estado de React. Razón: permite que el
  estado sea la única fuente de verdad del tamaño.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, no
implementarla.

---

## 11. Edge cases

### Bounds y viewport

- Arrastre que intentaría reducir el ancho por debajo de 720 px → clamp al floor; el modal no
  se encoge. El cursor sigue siendo `col-resize` pero el tamaño no cambia.
- Arrastre que intentaría superar 95 vw / 95 vh → clamp al ceiling en cada frame.
- `window.innerHeight` cambia durante un drag (el usuario redimensiona la ventana del
  navegador) → el ceiling se recalcula en cada `mousemove`; el floor sigue siendo el calculado
  al montar (constante por sesión de apertura).
- El viewport es más estrecho que `DEFAULT_WIDTH` (720 px) al montar → `loadPersistedSize`
  re-clampa; el tamaño inicial es `Math.min(720, window.innerWidth * 0.95)`.

### localStorage

- Valor ausente (primera apertura): `loadPersistedSize` retorna `null`; arranca con el
  default.
- JSON malformado en la clave: `try/catch` en `JSON.parse` captura; `loadPersistedSize`
  retorna `null`; arranca con el default.
- Valores numéricos fuera de bounds del viewport actual (p.ej. guardado en pantalla grande y
  reabierto en pantalla pequeña): `loadPersistedSize` re-clampa y retorna el tamaño ajustado.
- `localStorage` no disponible (`SecurityError`): `try/catch` captura; el modal funciona en
  memoria sin persistencia. El usuario no ve error.
- Valor con campos faltantes o de tipo incorrecto (p.ej. `{ width: "720" }`): validación
  `typeof === 'number' && isFinite` falla; `loadPersistedSize` retorna `null`.

### Drag fuera del área del modal

- El usuario arrastra el puntero fuera del modal o de la ventana del navegador → el listener
  está en `document`, por lo que el drag continúa siguiendo el puntero en tanto el botón esté
  presionado.
- `mouseup` fuera de la ventana puede no dispararse en algunos browsers (p.ej. al soltar fuera
  del Chrome tab): el drag queda "colgado". Mitigación: registrar también `mouseleave` en
  `window` (o `blur` en `window`) como fallback de cancelación que limpie los listeners.

### Cierre durante un drag activo

- El usuario presiona Escape mientras arrastra → el handler de Escape existente desmonta el
  componente. El `useEffect` cleanup elimina los listeners de `document`; el tamaño no se
  persiste (el drag se cancela).

### Viewport pequeño

- En viewport ≤ 640 px o `pointer: coarse`: los handles tienen `display: none`, por lo que no
  reciben `mousedown`. El modal usa el default (o el persistido) pero visualmente el CSS puede
  sobreescribir con `width: min(720px, 94vw)` cuando los handles están ocultos (el inline style
  sigue presente; la media query solo oculta los handles, no revierte el inline style). Si se
  quiere restaurar el sizing responsivo original en esos viewports, añadir en la media query
  `max-width: min(720px, 94vw)` al `.modalPanel` para que el inline style no exceda ese valor.

---

## 12. Estados de UI requeridos

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle (sin drag, desktop) | Modal con tamaño actual (default o persistido); tres handles visibles con cursor de resize en hover | Arrastrar handles, hacer scroll del contenido, cerrar con Escape/overlay/X |
| dragging | Modal redimensionándose en tiempo real siguiendo el puntero; cursor fijado al tipo del handle activo en todo el documento | Soltar para confirmar el tamaño; mover el puntero |
| drag-clamped | Modal en el floor o ceiling; no crece ni encoge más allá del límite; cursor igual | Soltar, cambiar dirección del drag |
| idle (viewport pequeño o coarse) | Handles ocultos; modal en sizing responsivo original | Leer el contenido, cerrar; sin capacidad de resize |
| storage-unavailable | Igual a idle; el drag funciona pero el tamaño no se persiste entre sesiones; el usuario no percibe diferencia | Arrastrar (tamaño en memoria únicamente durante la sesión) |

---

## 13. Validaciones

No aplica en el sentido clásico (sin formulario ni datos de usuario). La única sanitización
relevante es la del valor de `localStorage` y el clamp de bounds durante el drag:

| Entrada | Regla | Comportamiento si falla |
|---|---|---|
| `localStorage['vector:file-preview-modal:size']` | JSON válido con campos `width: number` y `height: number` finitos | Retornar `null`; usar default sin error visible |
| `size.width` al cargar | `>= DEFAULT_WIDTH` y `<= window.innerWidth * 0.95` | Clamp al bound correspondiente |
| `size.height` al cargar | `>= DEFAULT_HEIGHT` y `<= window.innerHeight * 0.95` | Clamp al bound correspondiente |
| Delta de drag (nuevo ancho/alto) | `>= floor` y `<= ceiling` en cada frame | Clamp silencioso; el modal no supera ni baja de los límites |

---

## 14. Seguridad y permisos

No aplica — no hay autenticación, no hay datos sensibles y no hay llamadas a API. `localStorage`
persiste únicamente `{ width: number, height: number }` (dos números en px); no contiene
información personal, tokens ni secretos. No se imprime nada en consola en flujo normal. El
`try/catch` de `localStorage` falla silenciosamente.

---

## 15. Observabilidad y logging

No aplica — no se añade logging. Los errores de `localStorage` (parse o acceso) se capturan
silenciosamente y se cae al default; no se registran en ningún sistema de tracking. Si el
proyecto ya tuviera un mecanismo de error tracking centralizado, un error de `localStorage` de
preferencia UI sería de muy bajo interés y puede ignorarse.

---

## 16. i18n / textos visibles

El componente `FilePreviewModal` no usa un sistema de traducciones centralizado; los strings de
accesibilidad existentes en el proyecto se hardcodean en el JSX. Esta feature sigue el mismo
patrón. Los únicos textos visibles (por tecnologías asistivas) introducidos son los `aria-label`
de los tres handles:

| Elemento | `aria-label` |
|---|---|
| Handle borde derecho | `"Redimensionar ancho"` |
| Handle borde inferior | `"Redimensionar alto"` |
| Handle esquina inferior-derecha | `"Redimensionar modal"` |

Si el proyecto adopta un sistema de i18n en el futuro, estos tres strings serán los candidatos
a externalizar. Por ahora se incluyen directamente en el JSX, consistente con el patrón del
componente existente.

---

## 17. Performance

- **`requestAnimationFrame`** en el handler de `mousemove`: garantiza que el estado de React se
  actualiza como máximo una vez por frame (~60 fps), evitando re-renders en exceso durante el
  drag. El `frameId` devuelto por `requestAnimationFrame` se guarda en un `useRef` para poder
  cancelarlo si `mouseup` llega antes del siguiente frame (`cancelAnimationFrame`).
- **Persistencia solo en `mouseup`**: `savePersistedSize` se llama una sola vez al soltar, no
  en cada frame. Evita escrituras excesivas en `localStorage`.
- **Estado local sin lifting**: `ModalSize` vive en `FilePreviewModal`; no provoca re-renders
  en componentes padre (`SpecDetailsDrawer` ni arriba).
- **Listeners en `document` solo mientras hay drag activo**: se registran en `mousedown` y se
  eliminan en `mouseup` (o en el cleanup del `useEffect`). No quedan listeners huérfanos en
  estado normal.
- **Sin dependencias nuevas**: cero impacto en el tamaño del bundle de Vite.

---

## 18. Restricciones

El agente no debe:

- Instalar librerías npm nuevas (ni de drag, ni de resize, ni de throttle/debounce).
- Usar la propiedad CSS `resize: both` o cualquier variante de CSS native resize.
- Implementar redimensionado desde bordes izquierdo o superior.
- Implementar redimensionado con teclas de flecha (deferred enhancement).
- Añadir soporte táctil o `pointer: coarse` (desktop-only esta fase).
- Cambiar el comportamiento de cierre (Escape, overlay, X), focus management, code-splitting
  de `MarkdownView` ni scroll de `.modalBody`.
- Modificar archivos fuera de `FilePreviewModal.tsx` (o `FilePreviewModal/`) y
  `SpecDetailsDrawer.module.css`.
- Cambiar estilos globales ni introducir variables CSS nuevas (usar solo las existentes).
- Persistir el tamaño por archivo (la persistencia es un único tamaño global para el modal).
- Cambiar la clave de `localStorage` (debe ser exactamente `vector:file-preview-modal:size`).
- Refactorizar partes de `FilePreviewModal` no relacionadas con esta feature.
- Ignorar errores de `typecheck` o `build`.
- Usar `any` en TypeScript.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `FilePreviewModal.tsx` (o `FilePreviewModal/index.tsx`) con `interface ModalSize`,
      estado, `loadPersistedSize`, `savePersistedSize`, lógica de drag para tres handles vía
      eventos nativos del DOM, inline style en `.modalPanel`.
- [ ] `SpecDetailsDrawer.module.css` con `position: relative` en `.modalPanel`, clases
      `.resizeHandleRight`, `.resizeHandleBottom`, `.resizeHandleCorner` (posición, cursor,
      color con token CSS) y media query `@media (max-width: 640px), (pointer: coarse)`.
- [ ] `ResizeHandle.tsx` en `FilePreviewModal/` (solo si la lógica justifica la extracción).
- [ ] `aria-label` en español en los tres handles; `role="separator"` y `tabIndex={0}`.
- [ ] Edge cases de bounds y `localStorage` cubiertos en el código.
- [ ] `npm run typecheck` verde desde `web/`.
- [ ] `npm run build` verde desde `web/`.
- [ ] Tests de Vitest para `loadPersistedSize` y función de clamp (si Vitest está configurado
      en `web/`; si no, documentar como deuda).

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] El modal crece desde borde derecho, inferior y esquina; no se puede encoger por debajo
      del default (720 px ancho, `window.innerHeight * 0.86` alto al montar).
- [ ] El modal no excede ~95 vw de ancho ni ~95 vh de alto en ninguna condición.
- [ ] `loadPersistedSize` y `savePersistedSize` usan `try/catch` en toda operación de
      `localStorage`.
- [ ] El tamaño se restaura al reabrir; si el valor almacenado está fuera de bounds del
      viewport actual, se re-clampa sin error.
- [ ] Los handles no aparecen en viewport ≤ 640 px ni con `pointer: coarse`.
- [ ] Los handles tienen `role="separator"`, `tabIndex={0}` y `aria-label` en español.
- [ ] `requestAnimationFrame` throttlea el `mousemove`; el `frameId` se cancela si es necesario.
- [ ] `savePersistedSize` se llama solo en `mouseup`, no en cada frame.
- [ ] Los listeners de `document` se eliminan en `mouseup` y en el cleanup del `useEffect`.
- [ ] El comportamiento de cierre (Escape, overlay, X) no cambió.
- [ ] El focus management no cambió.
- [ ] El scroll de `.modalBody` no cambió.
- [ ] No instalé dependencias npm nuevas.
- [ ] No usé CSS `resize` nativo ni librerías de drag.
- [ ] Solo modifiqué `FilePreviewModal.tsx` (o `FilePreviewModal/`) y `SpecDetailsDrawer.module.css`.
- [ ] No usé `any` en TypeScript.
- [ ] Ejecuté `npm run typecheck` desde `web/` — verde.
- [ ] Ejecuté `npm run build` desde `web/` — verde.
- [ ] No dejé logs temporales ni TODOs sin justificar.

---

## Open questions

1. **Grosor exacto de los handles**: el brief no especifica el valor de píxeles exacto del
   área visible ni del área de hit de los handles. Punto de partida sugerido: 6 px de franja
   visible con un `::after` o padding adicional que amplíe el área de hit a ~12 px. Confirmar
   con el diseñador o establecer como convención del proyecto.
2. **Cobertura de Vitest en `web/`**: si Vitest no está configurado en el workspace `web/`,
   los tests de `loadPersistedSize` y clamp quedan como deuda técnica. Confirmar si el
   framework de test está presente antes de implementar.
3. **Redimensionado con teclas de flecha** (deferred enhancement): implementar en una fase
   posterior como mejora de accesibilidad (p.ej. `ArrowRight` en el handle derecho añade
   10 px al ancho con clamp). Documentado aquí para no olvidarlo.
4. **Comportamiento del inline style en viewport pequeño**: en `@media (max-width: 640px)` el
   inline style `style={{ width, height }}` sigue presente en el DOM aunque los handles estén
   ocultos. Si se quiere restaurar el sizing responsivo original en esos breakpoints, añadir
   en la media query `max-width: min(720px, 94vw); height: auto` a `.modalPanel`. Confirmar
   si esto es deseable.
5. **Fallback de `mouseup` fuera de ventana**: en algunos browsers, soltar el botón del ratón
   fuera del tab no dispara `mouseup` en `document`. Confirmar si se implementa `mouseleave`
   en `window` como fallback de cancelación o si se acepta ese edge case como tolerable.
