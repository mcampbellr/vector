# Vector — Referencia de UI del Kanban Board

> Análisis de la imagen de referencia (estilo "unprocrast.com") que el usuario dio como
> dirección visual para el board del panel web de Vector. Esto describe el **look & feel y la
> estructura**, no es spec final. Mapeo al dominio de Vector al final.
>
> Imagen de referencia: [`assets/kanban-reference.png`](assets/kanban-reference.png)

## Layout general

Tres zonas de izquierda a derecha:

1. **Rail de iconos** (estrecho, ~56px, izquierda del todo) — navegación global por iconos.
2. **Panel de navegación** (sidebar ancho, blanco) — proyectos + sub-navegación.
3. **Área principal** — header + columnas del kanban con scroll horizontal.

Tema **claro**, muy limpio, mucho whitespace. Fondo gris muy claro; tarjetas blancas con
borde/sombra sutil y esquinas redondeadas (~12px). Tipografía sans-serif: título oscuro,
texto secundario gris. Iconos de línea finos.

## 1. Rail de iconos (global)

Iconos verticales apilados: logo/capas (top), grid (activo), personas/equipo, reloj/historial,
carpeta, check-circle, y abajo del todo un inbox/descarga. Solo iconos, sin texto.

## 2. Sidebar de navegación

- **Header**: avatar + nombre del workspace ("Brook Greens") + menú `...`.
- Lista de **proyectos** como secciones colapsables, cada una con header en mayúsculas:
  - `CREATE NEW WEBSITE`, `DESIGN NEW APP`, `BUILD A SYSTEM`.
  - Cada proyecto tiene sub-items con **badge de conteo** a la derecha:
    - **Today** (item activo resaltado, badge `15`)
    - **Calendar** (badge `87`)
    - **View** (expandible) → sub-vistas con punto de color:
      - `Timeline` (punto rojo), `Gantt` (punto verde), `Table` (punto azul)
  - Algunos `View` muestran un `+` para añadir vista.

> Patrón: workspace → proyectos → vistas (Today / Calendar / View{Timeline,Gantt,Table}).

## 3. Header del área principal

- Título grande **"TODAY"** con icono circular de refresh/spinner (verde).
- Subtítulo: `"17 tasks, updated 20 sec ago"` (conteo + frescura del estado).
- Acciones a la derecha: **campana** (notificaciones), icono de **sliders/filtros**,
  botón **"New task"** (oscuro, con icono `+`).

## 4. Columnas del kanban

4 columnas visibles (scroll horizontal): **Concept · Wireframe · Design · Development**.
Header de cada columna:

- Título de la columna.
- Subtítulo: `"{N} tasks, {H} hours"` (resumen agregado de la columna).
- Menú `...` a la derecha.

Las columnas representan **etapas del workflow**, no el estado de la tarjeta (el estado vive
en el pill de cada tarjeta).

## 5. Anatomía de la tarjeta (card)

De arriba a abajo:

1. (Opcional) **Cover/gradiente**: algunas tarjetas destacadas tienen una cabecera con
   imagen degradada colorida (ej. "Create Mood Board", "Conduct Market Research").
2. **Título** (bold).
3. **Descripción** (gris, ~2 líneas, truncada con ellipsis).
4. **Fila meta** (inferior), en orden:
   - **Status pill** — etiqueta uppercase pequeña, redondeada, con color de fondo:
     - `PROGRESS` → naranja/ámbar
     - `REVIEW` → púrpura/violeta
     - `TODO` → gris-azulado (slate)
     - `DONE` → verde
   - **Prioridad** — icono de bandera + label: `Urgent` / `High` / `Normal` / `Low`.
   - **Estimación** — icono de reloj + tiempo (`30 min`, `90 min`, `120 min`…).
   - **Comentarios** — icono de bocadillo + conteo (`2`, `4`, `6`…).

Estados visuales de tarjeta:
- **Activa/seleccionada**: ring/sombra resaltada.
- **Fade inferior**: las últimas tarjetas de cada columna se desvanecen (indicador de scroll).

### Inventario observado (para entender densidad/datos)

**Concept** (3 tasks, 2 h): Define Goals and Objectives (PROGRESS·Urgent·30m·2) ·
Create User Personas (PROGRESS·Urgent·30m·1) · Conduct Market Research [cover] (DONE·Normal·60m·3).

**Wireframe** (5 tasks, 4 h): Sketch Initial Wireframes (PROGRESS·High·90m·2) ·
Review Accessibility Guidelines (REVIEW·High·90m·4) · Gather Feedback (REVIEW·Normal·50m·1) ·
Create Mobile Versions (DONE·High·90m·3) · Finalize Wireframes (DONE·High·90m).

**Design** (5 tasks, 6 h): Design Style Guide (PROGRESS·High·90m·2) ·
Create Mood Board [cover] (REVIEW·High·90m·1) · Iterative Design Reviews (REVIEW·High·90m·5) ·
Optimize for Accessibility (DONE·High·90m).

**Development** (4 tasks, 3 h): Implement Interactivity (PROGRESS·Urgent·120m·2) ·
HTML/CSS Markup (TODO·High·20m·6) · Implement SEO Best Practices (TODO·High·20m·1) ·
Set Up Development Environment (TODO·Low·30m).

## Paleta / tokens implícitos

- Fondo app: gris casi blanco. Tarjeta: blanco, borde 1px gris claro, sombra suave.
- Acento de marca: gradiente (rosa→violeta→azul) en covers y avatar.
- Botón primario: negro/gris muy oscuro, texto blanco, esquinas redondeadas.
- Colores de status: ámbar (progress), violeta (review), slate (todo), verde (done).
- Radio de esquinas: tarjetas ~12px, pills ~6px, botones ~8–10px.

## Mapeo al dominio de Vector (propuesta, no final)

| Elemento de la imagen          | Equivalente en Vector                                   |
|--------------------------------|----------------------------------------------------------|
| Workspace ("Brook Greens")     | Repo / proyecto raíz administrado por Vector             |
| Proyectos del sidebar          | Repos o sub-workspaces (mono/micro)                      |
| Columnas (Concept…Development) | Etapas del workflow de specs (configurable)              |
| Tarjeta (task)                 | **Spec** (creado con `/vector:raw [text]`)               |
| Status pill                    | Estado del spec (todo/progress/review/done)              |
| Prioridad (bandera)            | Prioridad del spec                                       |
| Estimación (reloj)             | Estimación de tiempo **o budget de tokens** del spec     |
| Comentarios (bocadillo)        | Notas / historial / link al ticket asociado              |
| "updated 20 sec ago"           | Frescura del JSON de estado (sync con el board)          |

> Decisiones abiertas para la spec del board: qué representan exactamente las columnas
> (¿estado vs. fase del workflow?), si la estimación es tiempo o tokens, y cómo se mapea el
> link del ticket en la tarjeta. Ver `docs/vision.md`.
