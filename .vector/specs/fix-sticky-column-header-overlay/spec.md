# Spec: Títulos de columna del board se solapan con las cards al scrollear

## 1. Objetivo

Corregir el defecto de CSS en el componente `BoardColumn` por el cual el encabezado sticky
carece de fondo opaco y z-index, causando solapamiento visual del contenido de las cards sobre
el título y el contador al desplazarse dentro de una columna.

Esta corrección permite que cualquier **desarrollador que usa el board kanban** pueda
**desplazarse dentro de una columna** y ver el título de la columna (label + contador) siempre
legible, sin que las cards pasen por encima del encabezado.

## 2. Alcance

### Incluido en esta fase

- Añadir `background-color` (token opaco existente de `tokens.css`) a la clase `.header` en
  `web/src/components/BoardColumn/BoardColumn.module.css`.
- Añadir `z-index` a la misma clase `.header` para que quede por encima del contenido de
  `.cards` al hacer scroll.
- Crear `web/src/components/BoardColumn/BoardColumn.test.tsx` con test de componente que
  verifica la estructura DOM del header (elemento, clases, contenido semántico).
- Configurar el entorno de test DOM de Vitest en `web/` si aún no existe: añadir la
  devDependency necesaria (`happy-dom` o `jsdom`, y opcionalmente `@testing-library/react`) y
  el bloque `test` en `vite.config.ts`. Esto es parte del scope de este cambio (ver §4 y §6).

### Fuera de scope

- Cambios a `BoardColumn.tsx` (el JSX no se toca, salvo que sea estrictamente necesario para
  el test).
- Dark mode: el token elegido debe ser semántico, pero la paleta oscura en sí es parte del
  spec `add-dark-mode` (open).
- Modificar `tokens.css`: solo se consume un token existente, no se añaden tokens nuevos.
- Cambios a cualquier otro componente del board (`SpecCard`, `KanbanBoard`, `BoardHeader`,
  `StatusPill`, etc.).
- Lógica de API, contrato de datos, o estado del board.
- Regresión visual automatizada mediante scroll (requiere herramienta de browser testing no
  presente en el stack).

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca
relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Framework: **React 19** con **Vite 6** (SPA embebida en el binario Go de `cli/`).
- Lenguaje: **TypeScript 5.7**.
- Package manager: **npm** (inferido de `web/package.json`; no hay `pnpm-lock.yaml` ni
  `yarn.lock` en `web/`).
- Estilos: **CSS Modules** (patrón en `BoardColumn.module.css`) + **CSS custom properties**
  (tokens en `web/src/styles/tokens.css`).
- Testing: **Vitest 4.1.9** (`web/package.json`).
- Sin librería de componentes de UI — solo `lucide-react ^0.469.0` para iconos.

### Versiones relevantes (fuente: `web/package.json`)

- `react`: ^19.1.0
- `typescript`: ^5.7.2 (devDependency)
- `vite`: ^6.0.0 (devDependency)
- `vitest`: ^4.1.9 (devDependency)
- `lucide-react`: ^0.469.0 (no afectado por este fix)

### Patrones existentes a respetar

- **CSS Modules**: cada componente tiene su `.module.css` paralelo; las clases se importan
  como `styles.className` en el TSX. Ver `BoardColumn.tsx` línea 3:
  `import styles from './BoardColumn.module.css'`.
- **Tokens de diseño vía CSS variables**: todos los colores, radios y espaciados se referencian
  como `var(--token-name)`. Nunca valores hardcoded de color. Fuente única:
  `web/src/styles/tokens.css`.
- **Un componente por carpeta**: `BoardColumn/` tiene `BoardColumn.tsx` + `BoardColumn.module.css`.
  El test va en la misma carpeta: `BoardColumn/BoardColumn.test.tsx`.
- **Naming del archivo de test**: igual que el componente, con sufijo `.test.tsx`
  (igual que `relationChips.test.ts` en `SpecCard/`).
- **Tipado fuerte**: sin `any`; los props del componente ya usan `Card` y `Column` de
  `web/src/types/board.ts`.

No usar librerías, APIs ni patrones que no estén documentados oficialmente o que no estén ya
presentes en el proyecto, salvo lo autorizado explícitamente en este spec (ver §4 y §6).

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `web/src/components/BoardColumn/BoardColumn.module.css` — regla `.header` con
      `position: sticky; top: 0;` ya presente (líneas 9–16). Es el archivo a modificar.
- [x] `web/src/components/BoardColumn/BoardColumn.tsx` — renderiza el `<header>` con
      `styles.header` y contiene `<h2 className={styles.title}>` + `<span className={styles.count}>`.
- [x] `web/src/styles/tokens.css` — define los tres tokens candidatos para el fondo opaco:
      `--color-app-bg: #f7f8fa`, `--color-surface: #ffffff`, `--color-surface-muted: #f3f4f6`.
- [ ] **Entorno de test DOM para componentes React**: `web/package.json` no incluye
      `@testing-library/react` ni un entorno DOM; `vite.config.ts` no tiene bloque `test`.
      Los tests existentes (`entries.test.ts`, `relationChips.test.ts`) son lógica pura sin
      DOM. Si el entorno DOM de test no existe, **configurarlo es parte de esta fase** — el
      agente debe añadir la devDependency y el bloque `test` en `vite.config.ts` antes de
      crear `BoardColumn.test.tsx` (ver §6 y §19). No se detiene a reportar: la configuración
      es scope de este cambio.

**Causa del defecto (bug-framed):**
El bug fue introducido íntegramente en el commit `5441b80`
(`feat(web): board kanban SPA with Token Savings Meter`). Ese commit no mapea a ningún spec
registrado en `.vector/specs/` ni a ningún ticket; el campo `relatedTo[]` de este spec queda
vacío. El defecto concreto: al definir `.header` con `position: sticky` sin
`background-color`, el elemento es transparente. El elemento `.cards` que contiene las tarjetas
tiene `overflow-y: auto`, por lo que al scrollear, el contenido fluye encima del texto del
header (que queda "flotando" sin capa ni fondo).

---

## 5. Arquitectura

### Patrón a usar

**CSS Modules + token de diseño**: la corrección es declarativa y local a un solo archivo de
estilos. No se introduce ninguna abstracción nueva. El fondo se expresa como un token de
`tokens.css` para mantener la fuente única de verdad de los valores de color
(`standards/typescript-react.md`).

### Capas afectadas

- `presentation (web/)`: **sí** — modificación de `BoardColumn.module.css` (CSS puro) y
  configuración del entorno de test.
- `application/use-cases`: **no** — sin cambios de lógica.
- `domain`: **no** — sin cambios al modelo de datos ni a los tipos.
- `data/infrastructure (cli/ API)`: **no** — sin cambios de backend.
- `shared/common (tokens.css)`: **no modificado** — solo consumido.

### Flujo esperado

1. El usuario abre el board en el navegador (`vector serve --port 8787` + `npm run dev`, o
   `vector serve --web-dir web/dist`).
2. El board renderiza columnas con `<BoardColumn>` por cada estado del lifecycle.
3. Al scrollear dentro de una columna (`overflow-y: auto` en `.cards`), el `<header>` con
   `position: sticky; top: 0` permanece en la parte superior de la columna.
4. **Después del fix**: el `<header>` tiene `background-color: var(--color-app-bg)` (o el
   token elegido) y `z-index: 1`, de modo que las cards se desplazan por debajo de la banda
   opaca del header — sin solapamiento.
5. El título (`column.label` en `<h2 className={styles.title}>`) y el contador
   (`column.count` en `<span className={styles.count}>`) son legibles en cualquier posición
   de scroll.

### Ubicación de archivos

No se crean carpetas nuevas. El test va dentro de la carpeta existente
`web/src/components/BoardColumn/`.

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `web/src/components/BoardColumn/BoardColumn.module.css` | MODIFICAR | Añadir `background-color` (token opaco) y `z-index` a la clase `.header` | `web/src/styles/tokens.css` (patrón de tokens de color usados en `.count` y `.empty` del mismo CSS) |
| `web/src/components/BoardColumn/BoardColumn.test.tsx` | NUEVO | Test de componente: verifica que `<BoardColumn>` renderiza el `<header>` con estructura DOM correcta | `web/src/components/SpecCard/relationChips.test.ts` (Vitest, estructura de describe/it/expect) |
| `web/package.json` | MODIFICAR (condicional) | Añadir devDependencies para el entorno DOM de test (`happy-dom` o `jsdom`, y opcionalmente `@testing-library/react`) — solo si no están ya presentes | `web/package.json` (bloque `devDependencies` donde ya vive `vitest`) |
| `web/vite.config.ts` | MODIFICAR (condicional) | Añadir la clave `test: { environment: 'happy-dom' }` (o `'jsdom'`) dentro de `defineConfig` para Vitest — solo si el bloque `test` no existe | `web/vite.config.ts` (estructura existente de `defineConfig` con `plugins` y `server`) |

### Detalle por archivo

#### web/src/components/BoardColumn/BoardColumn.module.css

Acción: MODIFICAR

Cambios requeridos:

- En la regla `.header` (líneas 9–16), añadir las dos propiedades que faltan:
  ```css
  background-color: var(--color-app-bg); /* TBD — ver Open questions §1 */
  z-index: 1;
  ```
  El resto de propiedades existentes (`display`, `align-items`, `gap`, `padding`,
  `position: sticky`, `top: 0`) se mantienen sin cambios.

Restricciones:

- No añadir `border`, `box-shadow` ni ninguna otra propiedad no mencionada en este spec.
- No cambiar ninguna regla fuera de `.header` (`.column`, `.cards`, `.title`, `.count`, `.empty`).
- El valor de `background-color` debe ser una variable CSS de `tokens.css`; nunca un valor
  hardcoded de color.
- `z-index: 1` es el valor de referencia; ajustar al mínimo necesario para resolver el
  solapamiento si al implementar se descubre que `1` no es suficiente en algún browser.

#### web/src/components/BoardColumn/BoardColumn.test.tsx

Acción: NUEVO

Debe implementar:

- Importar `BoardColumn` y los tipos `Column` y `Card` de `../../types/board`.
- Definir una función de fixture `makeColumn(overrides: Partial<Column>): Column` con valores
  por defecto sensatos (label, count, cards vacío). El patrón de fixture (defaults + partial
  overrides) es el mismo que `makeCard` en `entries.test.ts`, pero el tipo aquí es `Column`,
  no `Card` — no reutilizar `makeCard` directamente; `entries.test.ts` es solo referencia de
  estilo.
- Caso columna con 3 cards: pasar un `makeColumn({ cards: [...] })` con cards mínimas para
  verificar que el `<header>` está en el DOM, que el `<h2>` contiene el `label` de la columna,
  y que el `<span>` contiene el `count`.
- Caso columna vacía (0 cards): pasar `makeColumn({ cards: [] })` y verificar que el elemento
  con texto "No specs" es visible.

Nota sobre aserciones de estilos computados: jsdom/happy-dom **no** aplica CSS Modules reales
al DOM; `getComputedStyle(el).backgroundColor`, `position` y `zIndex` siempre devolverán
valores vacíos o de defecto. El test no debe hacer aserciones sobre propiedades CSS computadas
(background-color, position, z-index). La corrección del CSS se verifica por:
  (a) revisión directa de `BoardColumn.module.css` (las dos propiedades presentes), y
  (b) build exitoso (`npm run build`) sin errores.
La verificación visual (scroll real) queda para revisión manual o regresión visual futura.

Debe seguir como referencia:

- `web/src/components/SpecCard/relationChips.test.ts` (estructura describe/it/expect, imports
  de Vitest).
- `web/src/components/SpecDetailsDrawer/entries.test.ts` (estilo de fixture con
  defaults + overrides parciales — referencia de patrón, no de tipo).

No debe incluir:

- Aserciones sobre propiedades CSS computadas en runtime (position, background-color, z-index):
  no son observables en jsdom/happy-dom con CSS Modules.
- Mocks de la API de `cli/` (el componente es puramente de presentación, sin fetch).
- Snapshots de componente (regla de `quality/testing-and-review.md`: no snapshots vacíos).

#### web/package.json

Acción: MODIFICAR (condicional — solo si las devDependencies del entorno DOM no existen)

Cambios requeridos:

- Añadir al bloque `devDependencies` la librería de entorno DOM elegida. Opciones:
  - `happy-dom` (más ligero, recomendado para Vitest): versión compatible con Vitest 4.x —
    TBD — ver Open questions §4.
  - `jsdom`: alternativa si happy-dom presenta incompatibilidades con React 19.
  - `@testing-library/react` (opcional pero recomendado para ergonomía de aserciones DOM):
    versión compatible con React 19 — TBD — ver Open questions §4.
- Si alguna de estas ya está presente al implementar, no duplicar.

Restricciones:

- Solo añadir devDependencies para el propósito del test de componente; no instalar nada
  más fuera de este scope.
- No cambiar `dependencies`, `scripts`, ni ningún otro campo de `package.json`.

#### web/vite.config.ts

Acción: MODIFICAR (condicional — solo si el bloque `test` no existe en `defineConfig`)

Cambios requeridos:

- Añadir la clave `test` dentro de `defineConfig(...)`:
  ```ts
  test: {
    environment: 'happy-dom', // o 'jsdom' según la devDep elegida
  },
  ```
- Ubicar la clave `test` al mismo nivel que `plugins`, `server` y `build` en el objeto de
  configuración existente.

Restricciones:

- No modificar las claves existentes (`plugins`, `server`, `build`).
- No cambiar la constante `API_TARGET` ni el proxy de `/api`.
- Si el bloque `test` ya existe, no sobreescribirlo; solo verificar que `environment` está
  configurado.

---

## 7. API Contract

No aplica — este fix es de CSS puro. No se introduce ni modifica ningún endpoint HTTP, ningún
campo del JSON de estado, ni ningún contrato entre `cli/` y `web/`. El componente
`BoardColumn` es de presentación y recibe sus datos vía props (`Column`, `Card`) que derivan
del hook `useBoard` (no modificado).

---

## 8. Criterios de éxito

**Comportamiento actual (bug confirmado):**

- `web/src/components/BoardColumn/BoardColumn.module.css`, clase `.header` (líneas 9–16):
  ```css
  position: sticky;
  top: 0;
  /* SIN background-color → transparente */
  /* SIN z-index → mismo plano visual que .cards */
  ```
- Consecuencia: al desplazar `.cards` (`overflow-y: auto`), el contenido de las cards
  renderiza encima del texto del encabezado. El título (`column.label`) y el contador
  (`column.count`) quedan ilegibles.

**Comportamiento esperado (post-fix):**

- `.header` tiene un `background-color` opaco derivado de un token de `tokens.css`.
- `.header` tiene `z-index` que lo sitúa por encima del contenido de `.cards`.
- Al scrollear dentro de la columna, el header permanece visible como banda opaca y legible
  en todo momento.

**Criterios verificables:**

- [ ] La clase `.header` en `BoardColumn.module.css` declara `background-color` con un token
      CSS de `tokens.css` (no `transparent`, no valor hardcoded).
- [ ] La clase `.header` declara `z-index` con valor > 0.
- [ ] Verificación visual manual: abrir el board con cards suficientes, scrollear en una
      columna, confirmar que el encabezado permanece legible sin solapamiento.
- [ ] El fix no altera el layout de columnas sin scroll (pocas cards o columna vacía).
- [ ] `npm run typecheck` en `web/` termina sin errores.
- [ ] `npm run test` en `web/` termina con todos los tests verdes (incluido el nuevo
      `BoardColumn.test.tsx`).
- [ ] `npm run build` en `web/` es exitoso (el asset embebido en el binario Go es válido).

### Tests requeridos

- [ ] `BoardColumn.test.tsx` — columna con cards: `<header>` presente en el DOM; `<h2>`
      contiene el label de la columna; `<span>` contiene el contador.
- [ ] `BoardColumn.test.tsx` — columna vacía (0 cards): texto "No specs" visible en el DOM.
- [ ] Tests existentes (`entries.test.ts`, `relationChips.test.ts`) siguen verdes (sin
      regresión).

**Nota**: los tests de `BoardColumn.test.tsx` verifican estructura DOM (presencia de elementos
y contenido textual), no propiedades CSS computadas. La verificación de `background-color`,
`position` y `z-index` se hace por revisión directa del CSS fuente y confirmación visual
manual — no por aserciones en el test (jsdom/happy-dom no resuelve CSS Modules reales).

### Comandos de verificación

Ejecutar desde `web/`:

```bash
npm run typecheck
npm run test
npm run build
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

El fix es visual puro — no hay formularios, navegación, ni feedback interactivo que cambiar.

### Visual

- El header debe verse como una banda opaca que "pertenece" al track de la columna: el fondo
  debe coincidir con el color del área donde se asienta la columna (ver Open questions §1 para
  la elección del token exacto).
- No introducir borde, sombra, ni separador visual adicional entre el header y las cards
  (la separación la genera el padding existente `padding: 0 var(--space-1) var(--space-3)`).
- El título ya usa `--color-text-secondary` y el contador ya usa
  `color: var(--color-text-secondary)` + `background: var(--color-surface-muted)`:
  esos valores no se tocan.

### Scroll

- Al scrollear, el header se "pega" a la parte superior de la columna (`position: sticky;
  top: 0` ya configurado) y enmascara las cards que pasan por debajo gracias al fondo opaco.
- No se altera la velocidad ni el comportamiento del scroll (CSS puro, sin JS).

### Accesibilidad

- La estructura semántica del componente no cambia: `<header>` con `<h2>` + `<span>` se
  mantiene intacta.
- El fix mejora la legibilidad (contraste) del título durante el scroll; no introduce
  regresión de accesibilidad.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas y el agente no debe cuestionarlas ni cambiarlas:

- **Fix restringido a CSS**: la causa es de presentación pura (falta de `background-color` y
  `z-index`); no se toca lógica de datos ni componentes adyacentes.
- **Uso de token existente de `tokens.css`**: no se hardcodean valores de color; el token se
  elige entre los candidatos de `tokens.css` al implementar (TBD — ver Open questions §1).
- **`z-index: 1` como valor de referencia**: es el mínimo para superar el stacking context
  normal de las cards. Si al implementar se detecta que `1` no es suficiente en algún caso,
  se ajusta al mínimo necesario; no se sobreingenieriza.
- **`relatedTo[]` vacío**: el commit origen (`5441b80`, "feat(web): board kanban SPA with
  Token Savings Meter") no mapea a ningún spec ni ticket; se registra sin relación.
- **El test se limita a aserciones de estructura DOM**: los estilos computados no están
  disponibles en jsdom/happy-dom con CSS Modules; las propiedades `position`, `background-color`
  y `z-index` del fix se verifican por revisión del CSS fuente, no en el test.
- **Configurar el entorno DOM de test es in-scope**: si `web/package.json` y `vite.config.ts`
  no tienen el entorno configurado, el agente lo configura como parte de esta fase (ver §6).
- **Un test nuevo; no se modifican tests existentes**: los tests de `entries.test.ts` y
  `relationChips.test.ts` son de lógica pura y no se ven afectados.

Si el agente detecta una alternativa aparentemente mejor (p. ej. añadir una sombra en vez de
z-index, o cambiar a CSS-in-JS), debe reportarla como observación, pero no implementarla.

---

## 11. Edge cases

### Pasos de reproducción del bug (estado previo al fix)

1. Levantar el board: `vector serve --port 8787` + `npm run dev` en `web/` (Vite en puerto
   5173, proxy `/api` → 8787). Alternativa: `npm run build` en `web/` y luego
   `vector serve --web-dir web/dist`.
2. Ir a una columna que tenga suficientes cards para que `.cards` active el overflow
   (`overflow-y: auto`; depende del viewport).
3. Scrollear verticalmente dentro de esa columna.
4. Observar: el contenido de las cards (texto de título, pills de estado, prioridad, etc.)
   se sobrepone al texto del header (título + contador). El header sticky es transparente y
   queda "tapado" visualmente.

### Edge cases del fix

- **Columna vacía (0 cards)**: no hay scroll activo; el fix no debe alterar el layout. El
  elemento `.empty` ("No specs") debe seguir visible bajo el header.
- **Columna con pocas cards (sin overflow activo)**: el fix es inerte visualmente (no hay
  scroll que genere solapamiento), pero el CSS debe seguir siendo correcto.
- **Columnas múltiples en el board**: cada `<BoardColumn>` es una sección independiente; el
  `z-index` del `.header` aplica al stacking context local de cada columna y no interfiere con
  headers de otras columnas.
- **Firefox y Safari con scrollbars personalizadas**: verificar que `z-index: 1` no produce
  artefactos visuales con las scrollbars nativas de esos navegadores sobre el área `.cards`.
- **Dark mode futuro**: el token elegido para `background-color` debe pertenecer a la paleta
  de `tokens.css` (no un valor hardcoded) para que, cuando se implemente `add-dark-mode`, la
  paleta oscura pueda sobreescribir el token en `:root[data-theme="dark"]` sin tocar
  `BoardColumn.module.css`.
- **Viewport muy pequeño**: si el viewport es tan reducido que todas las cards caben sin
  scroll, el fix no genera cambio visible pero tampoco daño.
- **Errores de API, offline, timeout, doble submit**: No aplica — cambio CSS puro, sin
  superficie de API ni formularios (ver §7).

---

## 12. Estados de UI requeridos

La pantalla/componente debe mantener estos estados (sin cambios de lógica, solo CSS):

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle (sin scroll) | Header sticky con fondo opaco visible; cards en reposo debajo del header | Scrollear la columna |
| scroll activo | Header sticky permanece como banda opaca y legible; cards pasan por debajo | Continuar o detener scroll |
| columna vacía | Header visible (label + "0") + mensaje ".empty" ("No specs") | — |
| columna con pocas cards (sin overflow) | Header y cards visibles sin solapamiento; fix inerte pero correcto | — |

No hay estados de loading, error ni offline en este componente (es de presentación pura;
el estado del board lo gestiona `useBoard`).

---

## 13. Validaciones

No aplica — este fix no introduce formularios, inputs ni flujos que requieran validación de
datos. El cambio es de CSS estático.

---

## 14. Seguridad y permisos

No aplica al dominio de seguridad — el fix modifica únicamente declaraciones CSS estáticas:

- No se añaden endpoints ni accesos a datos.
- No hay tokens, secretos ni PII involucrados.
- El CSS buildado forma parte del bundle embebido en el binario Go; no introduce superficie
  de ataque.
- No se cambian permisos de acceso al board ni al API de `cli/`.

---

## 15. Observabilidad y logging

No aplica — el fix no introduce ni requiere logging, métricas ni trazas:

- No hay errores de API ni de parsing que registrar.
- El defecto es visual; su corrección no tiene efecto en los logs del servidor (`vector serve`).
- No se modifican los mecanismos de logging existentes.

---

## 16. i18n / textos visibles

No aplica — fix de CSS puro; no se introducen ni modifican textos visibles.

Los textos del encabezado de columna (`column.label` para el título, `column.count` para el
contador) son datos dinámicos del board que vienen del JSON de estado vía la API de `cli/`.
No son strings hardcodeados en el componente ni en ningún archivo de traducciones.

El único texto hardcodeado en `BoardColumn.tsx` es `"No specs"` en el elemento `.empty`
(línea 20 del TSX actual), que ya existía antes de este spec y no se modifica.

---

## 17. Performance

- Añadir `background-color` y `z-index` a una regla CSS existente tiene costo de render
  nulo en la práctica: son propiedades baratas de pintar para el motor CSS del navegador.
- `position: sticky` ya estaba presente; no se añade un nuevo stacking context costoso.
- `z-index: 1` no crea una capa de compositing adicional (solo valores altos de z-index con
  `transform`/`will-change` lo hacen en algunos browsers); no hay impacto en memoria de GPU.
- Bundle size: sin cambio práctico (dos líneas CSS adicionales en un módulo pequeño).
- No se introducen renders adicionales ni reflows costosos.

---

## 18. Restricciones

El agente no debe:

- Cambiar el JSX de `BoardColumn.tsx` (props, estructura, clases aplicadas), salvo que sea
  estrictamente necesario para el setup del test.
- Añadir nuevos tokens a `tokens.css`; usar únicamente tokens existentes.
- Instalar dependencias en `web/package.json` que no estén autorizadas en este spec. Las
  únicas adiciones permitidas son: la devDependency del entorno DOM (`happy-dom` o `jsdom`) y,
  opcionalmente, `@testing-library/react` — únicamente para el propósito del test de
  componente (ver §6). Cualquier otra dependencia está prohibida sin autorización explícita.
- Modificar `web/vite.config.ts` más allá de añadir el bloque `test: { environment }` (ver §6);
  no cambiar `plugins`, `server`, `build`, ni la constante `API_TARGET`.
- Hardcodear valores de color (e.g., `background-color: #f7f8fa`); siempre `var(--token)`.
- Modificar las reglas CSS `.column`, `.cards`, `.title`, `.count` o `.empty` en
  `BoardColumn.module.css`.
- Cambiar la interfaz `BoardColumnProps` (`Column`, `Card`) de `board.ts`.
- Implementar dark mode ni preparación para él más allá de usar un token semántico (eso es
  scope del spec `add-dark-mode`).
- Añadir `box-shadow` o `border-bottom` al header sin autorización explícita.
- Tocar archivos fuera de los listados en §6.
- Ignorar errores de typecheck, lint o tests.
- Usar APIs CSS no soportadas en los browsers objetivo del proyecto (sin evidencia de polyfills
  en este repo).

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `web/src/components/BoardColumn/BoardColumn.module.css` modificado: `.header` tiene
      `background-color: var(<token-elegido>)` y `z-index: 1` (o el valor mínimo necesario).
- [ ] `web/src/components/BoardColumn/BoardColumn.test.tsx` creado con tests de estructura
      DOM para columna con cards y columna vacía.
- [ ] `web/package.json` actualizado con las devDependencies del entorno DOM de test, si no
      existían (ver §4 y §6). Consistente con la decisión en §10 de que esto es in-scope.
- [ ] `web/vite.config.ts` actualizado con el bloque `test: { environment }`, si no existía
      (ver §4 y §6).
- [ ] Gate verde en `web/`: `npm run typecheck`, `npm run test`, `npm run build` — todos sin
      errores.
- [ ] Sin logs temporales, sin TODOs sin justificar en los archivos modificados.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Solo modifiqué los archivos listados en §6; no toqué ningún otro sin justificarlo.
- [ ] La clase `.header` ahora tiene `background-color` con un `var()` de `tokens.css` (no
      hardcoded, no `transparent`) y `z-index > 0`.
- [ ] El token elegido es uno de los tres candidatos de `tokens.css`:
      `--color-app-bg`, `--color-surface` o `--color-surface-muted`.
- [ ] No añadí tokens nuevos a `tokens.css`.
- [ ] Las reglas `.column`, `.cards`, `.title`, `.count` y `.empty` de `BoardColumn.module.css`
      están intactas.
- [ ] `BoardColumn.tsx` no fue modificado (o si lo fue, está justificado aquí).
- [ ] El entorno DOM de test está configurado (`web/package.json` + `vite.config.ts`) si no
      existía antes de este cambio.
- [ ] El test `BoardColumn.test.tsx` usa una fixture `makeColumn` propia (no `makeCard`);
      `entries.test.ts` se usó solo como referencia de estilo.
- [ ] El test no hace aserciones sobre estilos computados CSS (fuera del alcance con CSS
      Modules en jsdom/happy-dom).
- [ ] El test `BoardColumn.test.tsx` corre con `npm run test` y pasa.
- [ ] `npm run typecheck` sin errores.
- [ ] `npm run build` exitoso.
- [ ] Verifiqué visualmente (manual) en Firefox y Safari que `z-index` no genera artefactos
      con scrollbars nativas y que el solapamiento está resuelto.
- [ ] No dejé logs temporales ni TODOs sin justificar.

---

## Open questions

1. **Token exacto para el fondo del header**: tres candidatos en `tokens.css`:
   - `--color-app-bg` (`#f7f8fa`) — es el fondo del `<body>` y del track de la columna;
     matchea el entorno visual → el header "desaparece" en el fondo y actúa como máscara
     natural. Candidato preferido del brief.
   - `--color-surface` (`#ffffff`) — fondo blanco; crearía contraste leve con el track.
   - `--color-surface-muted` (`#f3f4f6`) — gris muy claro; intermedio.
   Decidir al implementar según el efecto visual deseado — TBD — ver Open questions.

2. **Valor exacto de z-index**: `1` es el valor de referencia mínimo para superar el stacking
   context de `.cards`. Confirmar al implementar que no hay otros elementos posicionados en
   `.column` que requieran un valor mayor — TBD — ver Open questions.

3. **¿Hay max-height en `.cards` o `.column` que fuerce el overflow, o el overflow depende
   únicamente del viewport?**: `BoardColumn.module.css` no declara `max-height` explícita ni
   `height`. El overflow en `.cards` depende de que la columna tenga una altura acotada por
   el layout del board padre. Confirmar al implementar para entender cuándo se activa el
   overflow en browsers distintos — TBD — ver Open questions.

4. **Paquetes exactos del entorno DOM de test**: la decisión de configurar el entorno está
   tomada (in-scope). Queda elegir al implementar:
   - Entorno DOM: `happy-dom` (más ligero, nativo Vitest) vs `jsdom` (más compatible con
     quirks de browser APIs en React 19) — TBD — ver Open questions.
   - Aserciones: `@testing-library/react` (ergonomía de queries `getByRole`, `getByText`) vs
     queries DOM nativas de jsdom/happy-dom. Si se elige @testing-library/react, verificar
     versión compatible con React 19 al instalar — TBD — ver Open questions.
