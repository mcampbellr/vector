# Spec: Reescribir el README público en inglés, humanizado y bien explicado

## 1. Objetivo

Reescribir `README.md` (raíz del repo) para convertirlo en un artefacto público profesional
que refleja el estado real de Vector (CLI Go funcional, 11 project commands `/vector:*`
ejecutables, board web en desarrollo), guía a developers senior que descubren el proyecto por
primera vez, y transmite la propuesta de valor con prosa humanizada libre de señales de
generación por IA.

Esta feature permite que un **developer** que llega al repo de Vector pueda **entender qué es,
por qué importa y empezar a usarlo** en ~3–5 minutos de lectura, con pasos de instalación
reales y un walkthrough end-to-end del ciclo de vida de un spec.

El problema concreto: el README actual (~37 líneas, español) lleva el aviso "captura inicial
de la idea / **Nada implementado todavía**" en la segunda línea. Ese estado ya no es correcto.
El CLI tiene subcomandos funcionales, el kit tiene 11 project commands `/vector:*` ejecutables,
`web/` está en desarrollo activo y hay specs compiladas en `.vector/`. Un developer que descubre
el repo hoy se encuentra con un README que lo desinforma sobre el estado real y no ofrece ninguna
guía de uso.

## 2. Alcance

### Incluido en esta fase

- **Reescritura casi completa de `README.md`** (raíz): nueva estructura, prosa en inglés,
  humanizada vía el skill `/humanizer`.
- **Sección "What is Vector"**: propuesta de valor en 2–3 párrafos; qué hace, para quién, por
  qué. Incluye imagen del board (`docs/assets/kanban-reference.png`).
- **Sección "Why Vector"**: diferenciación concisa — token routing, board kanban, agnóstico al
  código del usuario, integración con Claude Code.
- **Sección "Installation"**: pasos reales actuales (`go build` / `go install` desde `cli/`) +
  `vector init` por repo; nota explícita de `curl | install.sh` como "coming soon / planned".
- **Sección "Quickstart"**: flujo mínimo desde cero hasta ver un spec en el board.
- **Sección "Key Concepts"**: glosario mínimo con contexto de 1–2 líneas por término (spec,
  OpenSpec, board, token routing, `/vector:*` commands, `vector init`).
- **Sección "Commands Reference"**: tabla de los 11 `/vector:*` commands verificados en
  `kit/commands/vector/` con descripción de una línea por command.
- **Sección "Walkthrough — End-to-End Flow"** (sección opcional elegida por el usuario): ejemplo
  concreto narrativo del ciclo completo: `vector init` → `/vector:raw` → `/vector:propose` →
  `/vector:apply` → card moviéndose en el board.
- **Sección "Contributing / License"** (sección opcional elegida por el usuario): nota de
  contribución breve + licencia marcada explícitamente como TBD.
- **Sección "Further Reading"**: links verificados a `docs/vision.md`,
  `docs/domain-contract.md`, `docs/plugin-and-commands.md`, `docs/commercialization.md`.
- Paso obligatorio de humanización: todos los textos del README pasados por `/humanizer` durante
  la implementación en `/vector:apply`.

### Fuera de scope

- **`curl | install.sh` (one-liner de instalación)**: no existe en el repo; se menciona en el
  README solo como "coming soon / planned". El spec del instalador se abre como un `/vector:raw`
  separado (ver Open questions).
- **Roadmap de estado o fechas de features**: fuera del scope explícito del usuario.
- **Comparativa vs Linear, GitHub Projects u otras herramientas**: fuera del scope explícito del
  usuario.
- **Modificar cualquier archivo distinto de `README.md`**: no tocar `docs/`, `kit/`, `cli/`,
  `web/`, ni ningún otro archivo del repo.
- **Duplicar `docs/vision.md`**: el README referencia `docs/` para profundidad; no reproduce la
  visión técnica larga.
- **Sección de changelog o versioning**.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- **Formato del entregable**: GitHub Flavored Markdown (GFM), archivo único `README.md`.
- **Idioma del entregable (README)**: **Inglés** — decisión explícita del usuario (el README es
  un artefacto público/comercial; ver §10). Se aparta deliberadamente de la convención del repo
  (documentos de `docs/` en español). Ver §16 para el detalle de esta distinción.
- **Idioma de este spec**: **Español** — convención del repo
  (`.claude/rules/documentation/docs-standards.md`).
- **Herramienta de humanización**: skill `/humanizer` — se aplica a cada bloque de texto del
  README durante la implementación en `/vector:apply`, para eliminar señales de prosa generada
  por IA (em-dash overuse, rule-of-three, parallelismos negativos, vocabulario AI genérico como
  "seamlessly", "robust", "comprehensive"). Este paso es obligatorio, no opcional.
- **Slugs, comandos, flags y rutas**: siempre kebab-case inglés, independientemente del idioma
  del entregable (regla del repo: `standards/naming.md`).
- **Imagen de referencia**: `docs/assets/kanban-reference.png` (path relativo desde la raíz del
  repo; confirmada presente).

### Versiones relevantes

- **Go**: **1.26** (declarado en `cli/go.mod`; relevante para los pasos de instalación del README).
- No hay dependencias de librerías para el entregable Markdown.

### Patrones existentes a respetar

- **No duplicar `docs/vision.md`**: el README remite a `docs/` para profundidad; no reproduce la
  visión técnica larga.
- **Kebab-case para todos los identificadores de cara al usuario**: commands (`/vector:raw`),
  flags (`--language`), IDs de specs.
- **Jargon siempre con contexto mínimo**: cualquier término propio (OpenSpec, spec, token routing,
  board) se acompaña de 1–2 líneas de explicación o un link a `docs/` la primera vez que aparece.
- **No prometer features inexistentes**: `curl | install.sh` → "coming soon"; cualquier feature
  marcada `pendiente` en el repo → no mencionarla como disponible.
- **Prosa humanizada**: todos los textos del README pasan por `/humanizer`; paso obligatorio antes
  de dar la implementación por terminada.
- **Un solo archivo modificado**: `README.md` (raíz).
- **Descripciones de commands verificadas**: leer `kit/commands/vector/*.md` como fuente antes de
  redactar la tabla de referencia; no inventar descripciones.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Binario Go funcional: `cli/cmd/vector/main.go`; subcomandos `vector init`,
      `vector serve`, `vector standup` implementados.
- [x] Los 11 project commands `/vector:*` en `kit/commands/vector/`: `raw`, `propose`, `apply`,
      `bug`, `comment`, `link`, `close`, `archive`, `status`, `standup`, `sync`.
      (Verificados con `Glob kit/commands/vector/*.md`. El brief mencionó "quick" como 12.º
      command pero `quick.md` no está presente en el repo al momento de escribir este spec;
      ver Open questions #3.)
- [x] `docs/vision.md` — presente en el repo; el README lo referencia como lectura adicional.
- [x] `docs/domain-contract.md` — presente; define el modelo de columnas/estados del board.
- [x] `docs/plugin-and-commands.md` — presente; referencia de comandos y plugin model.
- [x] `docs/commercialization.md` — presente; referencia de estrategia comercial.
- [x] `docs/assets/kanban-reference.png` — confirmada presente; se incluye en el README.
- [x] Skill `/humanizer` disponible en el entorno (confirmado por el usuario, clarificación #1).
- [ ] LICENSE en la raíz del repo — **no existe**. La licencia es TBD — ver Open questions.
      No crear ni inventar el archivo `LICENSE`; esta fase no lo requiere.

Si alguna dependencia no existe al momento de implementar, el agente debe detenerse y reportar
qué falta. No inventar contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

**Documento Markdown estático por secciones con humanización obligatoria.** No hay capas de
software, estado, ni API. El patrón de trabajo es editorial:

1. **Lectura y verificación**: el agente lee el README actual y los archivos de referencia del
   repo (`docs/`, lista de commands, `docs/assets/kanban-reference.png`) antes de escribir.
2. **Redacción por secciones**: cada sección del README se redacta con información verificada
   contra el repo; ningún contenido aspiracional ni prometer features pendientes.
3. **Humanización obligatoria**: cada sección se pasa por `/humanizer` para eliminar señales de
   prosa generada por IA. Este paso no es opcional.
4. **Verificación de integridad**: antes de finalizar, confirmar que los links a `docs/` existen,
   que `docs/assets/kanban-reference.png` tiene el path correcto, y que no quedan referencias al
   texto antiguo ("Nada implementado todavía").

### Capas afectadas

- presentation (web/board): **no**.
- application/CLI (`cli/`): **no** — se documentan comandos existentes; no se modifica código Go.
- domain/config (`cli/internal/`): **no**.
- kit (`kit/`): **no** — se listan los commands existentes en el README; no se modifican.
- documentación (`docs/`): **no** — el README referencia `docs/`; nada dentro de `docs/` se modifica.
- raíz del repo: **sí** — únicamente `README.md`.

### Flujo esperado

1. El agente lee `README.md` (estado actual) para entender qué hay y qué reemplazar.
2. El agente lee `kit/commands/vector/*.md` (11 archivos) para obtener descripciones verificadas
   de los commands.
3. El agente redacta cada sección del nuevo README en inglés con información verificada.
4. Pasa cada sección por `/humanizer`.
5. Ensambla el README final con las 9 secciones definidas en §6.
6. Verifica que los paths referenciados existen (comandos de §8).
7. Escribe `README.md`.

### Ubicación de archivos

```txt
README.md   ← único archivo a modificar (raíz del repo)
```

No se crean carpetas ni archivos nuevos.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `README.md` | MODIFICAR | Reescritura casi completa: inglés, prosa humanizada, instalación real, 9 secciones estructuradas (ver detalle abajo). | `docs/plugin-and-commands.md` (estructura de secciones Markdown en el repo, tono de referencia técnica) |

### Detalle por archivo

#### README.md

Acción: MODIFICAR

Debe implementar las siguientes secciones, en inglés, en este orden:

**1. Header**
- Título `# Vector` + tagline de una línea (p. ej. "Spec-driven project management for
  Claude Code developers").
- No añadir badges de CI ni shields que dependan de pipelines no configurados.

**2. `## What is Vector`**
- 2–3 párrafos: qué hace Vector, para quién (developers senior que usan Claude Code), cómo
  funciona (specs en un board kanban, agnóstico al código del usuario, eficiencia de tokens).
- Incluir la imagen `docs/assets/kanban-reference.png` con alt text descriptivo:
  `![Vector kanban board showing specs in open, in-progress, review, and closed columns](docs/assets/kanban-reference.png)`

**3. `## Why Vector`**
- Diferenciación concisa para el dev: token routing (agentes baratos para tareas triviales),
  board kanban proyectado desde el JSON de estado, integración nativa con Claude Code, agnóstico
  al stack del usuario.

**4. `## Installation`**
- Requisito previo: **Go 1.26+** (la versión declarada en `cli/go.mod`). Indicarlo antes de los
  pasos de build para que un dev con un Go más viejo no falle silenciosamente.
- Pasos reales de hoy (única vía documentada: clonar + build local, no `go install <url>`):
  ```bash
  git clone <repo-url>   # TBD — ver Open questions #2 (repo remoto privado pendiente)
  cd vector/cli
  go build -o ~/.local/bin/vector ./cmd/vector
  ```
  Luego, por repo:
  ```bash
  cd <your-project>
  vector init
  ```
- Nota explícita: `curl | install.sh` marcado como "coming soon / planned". Nunca presentarlo
  como un paso funcional.

**5. `## Quickstart`**
- Flujo mínimo de 3–4 pasos: `vector init` en un repo existente → `/vector:raw "idea"` en
  Claude Code → `vector serve` para abrir el board → ver la card aparecer.

**6. `## Key Concepts`**
- Tabla o lista breve con al menos: `spec`, `OpenSpec`, `board`, `token routing`,
  `/vector:*` commands, `vector init`. Cada uno con 1–2 líneas de contexto + link a `docs/`
  donde aplique (p. ej. `docs/domain-contract.md` para columnas del board,
  `docs/plugin-and-commands.md` para el modelo de commands).

**7. `## Commands Reference`**
- Tabla de los 11 `/vector:*` commands verificados en `kit/commands/vector/*.md`.
  El agente debe leer cada archivo fuente antes de escribir las descripciones.
  Estructura esperada de la tabla:

  | Command | What it does |
  |---|---|
  | `/vector:raw` | (verificar contra `kit/commands/vector/raw.md`) |
  | `/vector:propose` | (verificar contra `kit/commands/vector/propose.md`) |
  | `/vector:apply` | (verificar contra `kit/commands/vector/apply.md`) |
  | `/vector:bug` | (verificar contra `kit/commands/vector/bug.md`) |
  | `/vector:comment` | (verificar contra `kit/commands/vector/comment.md`) |
  | `/vector:link` | (verificar contra `kit/commands/vector/link.md`) |
  | `/vector:close` | (verificar contra `kit/commands/vector/close.md`) |
  | `/vector:archive` | (verificar contra `kit/commands/vector/archive.md`) |
  | `/vector:status` | (verificar contra `kit/commands/vector/status.md`) |
  | `/vector:standup` | (verificar contra `kit/commands/vector/standup.md`) |
  | `/vector:sync` | (verificar contra `kit/commands/vector/sync.md`) |

  (Las descripciones anteriores son marcadores; el agente debe sustituirlas con el texto real
  extraído de los archivos fuente y luego pasar la tabla por `/humanizer`.)

**8. `## Walkthrough — End-to-End Flow`**
- Ejemplo concreto narrativo del ciclo de vida de un spec:
  - `vector init` en un repo → siembra los commands `/vector:*` en `.claude/commands/vector/`.
  - `/vector:raw "add user authentication"` en Claude Code → crea un spec en
    `.vector/specs/add-user-authentication/spec.md` y la card aparece en la columna `open`.
  - `/vector:propose` → Claude propone un plan, puede pedir aclaraciones.
  - `/vector:apply` → Claude implementa el spec; la card pasa a `in-progress` y luego a
    `review`.
  - `vector serve` → el board web muestra el estado actualizado en tiempo real (SSE).
- Tono: narrativo, concreto, con comandos reales. No abstracto.

**9. `## Contributing / License`**
- Nota de contribución breve (PRs welcome, cómo reportar issues, etc.).
- Licencia: **TBD — ver Open questions**. No inventar ni asumir ninguna licencia. El texto debe
  decir explícitamente que la licencia no está definida aún.

**10. `## Further Reading`**
- Links verificados:
  - [`docs/vision.md`](docs/vision.md) — full vision and design decisions
  - [`docs/domain-contract.md`](docs/domain-contract.md) — board states, domain model
  - [`docs/plugin-and-commands.md`](docs/plugin-and-commands.md) — commands and plugin model
  - [`docs/commercialization.md`](docs/commercialization.md) — distribution and packaging

Debe seguir como referencia estructural:
- `docs/plugin-and-commands.md` (para tono y estructura de secciones técnicas en el repo).

No debe incluir:
- El texto original del README (el aviso "Nada implementado todavía" y la descripción en español).
- La sección en español **"Configuración — idioma de la prosa de los agentes"** (actualmente en
  `README.md` líneas ~15–36, añadida por el spec `add-agent-prose-language`): se reemplaza por
  completo. El flag `--language` no se documenta en el README público; vive en `docs/` y en la
  ayuda del binario. No dejar ese bloque residual tras la reescritura.
- Referencias a features no implementadas presentadas como disponibles (p. ej. `curl | install.sh`
  como paso funcional).
- Licencia inventada; la sección de licencia dice únicamente "TBD".
- Texto sin humanizar; toda la prosa pasa por `/humanizer`.
- Contenido de `docs/vision.md` duplicado.

Restricciones:
- No modificar ningún archivo fuera de `README.md` (raíz).
- Verificar con `ls` que cada path referenciado existe antes de incluirlo.
- No añadir badges o shields que dependan de CI no configurado.
- Los 11 commands de la tabla deben tener descripciones verificadas contra `kit/commands/vector/*.md`.

---

## 7. API Contract

No aplica — esta fase produce un documento Markdown estático (`README.md`). No se introducen
ni modifican endpoints HTTP, contratos JSON ni interfaces de API. El README documenta comandos
existentes pero no define ni altera contratos.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] **El README está íntegramente en inglés.** Ningún párrafo ni encabezado en español.
      Slugs, paths y commands permanecen en kebab-case inglés (invariante).
- [ ] **Refleja el estado real del proyecto**: no contiene el texto "Nada implementado todavía",
      "captura inicial" ni ninguna frase que sugiera que el repo está vacío o es solo una idea.
- [ ] **Todos los textos del README pasaron por `/humanizer`** durante la implementación. La
      prosa no tiene: em-dashes en exceso, rule-of-three, frases AI genéricas ("seamlessly",
      "comprehensive", "robust"), ni paralelismos negativos.
- [ ] **Instalación con pasos reales de hoy**: muestra `go build -o ~/.local/bin/vector ./cmd/vector`
      (desde `cli/`) + `vector init` por repo, y declara el requisito **Go 1.26+** (de `cli/go.mod`)
      antes de los pasos de build. El one-liner `curl | install.sh` aparece solo como
      "coming soon / planned", nunca como paso funcional.
- [ ] **Walkthrough end-to-end presente**: el flujo `vector init` → `/vector:raw` →
      `/vector:propose` → `/vector:apply` → card en el board aparece como ejemplo concreto y
      narrativo.
- [ ] **Sección Contributing/License presente**: nota de contribución + licencia marcada
      explícitamente como "TBD". No se inventó ninguna licencia.
- [ ] **Links a `docs/` verificados**: `docs/vision.md`, `docs/domain-contract.md`,
      `docs/plugin-and-commands.md`, `docs/commercialization.md` existen en el repo y los links
      del README apuntan a paths correctos.
- [ ] **`docs/assets/kanban-reference.png` referenciada** en la sección visual apropiada, con
      alt text descriptivo.
- [ ] **No promete features no implementadas** como disponibles.
- [ ] **Jargon con contexto**: spec, OpenSpec, token routing y board aparecen con al menos 1–2
      líneas de contexto o link a `docs/` la primera vez que se usan.
- [ ] **Tabla de commands con descripciones verificadas** contra `kit/commands/vector/*.md` (11
      commands; ver Open questions #3 sobre el recuento).

### Tests requeridos

No hay tests automáticos para un entregable Markdown. La verificación es manual:

- [ ] Leer el README completo como si fuera un developer externo: ¿entiende qué es Vector en 3
      párrafos? ¿puede instalar y completar el quickstart solo con lo que dice el README?
- [ ] Verificar que cada link del README apunta a un archivo que existe en el repo.
- [ ] Confirmar que la imagen `docs/assets/kanban-reference.png` está referenciada con el path
      correcto y tiene alt text.
- [ ] Confirmar que ningún bloque de prosa tiene señales de generación por IA (el paso
      `/humanizer` debe haberse aplicado).

### Comandos de verificación

```bash
# Verificar que los archivos referenciados en el README existen
ls docs/vision.md docs/domain-contract.md docs/commercialization.md \
   docs/plugin-and-commands.md docs/assets/kanban-reference.png

# Verificar que el texto antiguo no existe más
grep -c "captura inicial" README.md       # debe retornar 0
grep -c "Nada implementado" README.md     # debe retornar 0
grep -c "Sin código aún" README.md        # debe retornar 0

# Verificar que el README tiene contenido sustancioso
wc -l README.md                           # debe ser > 100 líneas

# Verificar que el README está en inglés (spot-check)
head -5 README.md                         # encabezado y tagline en inglés
```

La fase no está completa si alguno de estos comandos muestra resultados inesperados.

---

## 9. Criterios de UX

Para un entregable de documentación, los "criterios de UX" son criterios de legibilidad y
navegación del documento:

- **Estructura escaneable**: encabezados `##` para secciones principales; `###` para subsecciones.
  Un developer debe poder orientarse en el README solo leyendo los encabezados (GitHub genera
  tabla de contenidos automáticamente).
- **Tiempo de lectura**: la propuesta de valor y el quickstart son comprensibles en ~3–5 minutos.
  Las secciones de referencia (commands, further reading) son skippables para el lector que ya
  decidió probarlo. Métrica verificable: la prosa (excluyendo bloques de código y tablas) cabe en
  ~800–1200 palabras (`wc -w` como spot-check), suficiente para ser completo sin abrumar.
- **Primer vistazo**: el párrafo de apertura ("What is Vector") responde en ≤ 3 oraciones: qué
  hace, para quién, por qué importa. Sin disclaimers ni preamble.
- **Bloques de código**: todos los comandos de instalación y uso en bloques ` ```bash ` con
  syntax highlighting. No en texto inline para comandos multi-línea.
- **Imagen del board**: aparece en la zona de "What is Vector" o "Why Vector" para anclar
  visualmente el concepto de board antes del quickstart. Alt text descriptivo para accesibilidad.
- **Tono**: directo, técnico, sin hipérbole. La audiencia es senior; no requiere motivación ni
  lenguaje celebratorio.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas; el agente no debe cuestionarlas ni cambiarlas:

- **El README está en inglés** (clarificación #2 del usuario). Se aparta deliberadamente de la
  convención del repo (docs en español) porque el README es un artefacto público/comercial.
- **Toda la prosa del README pasa por `/humanizer`** antes de la entrega (clarificación #1).
  Este paso es obligatorio en `/vector:apply`, no opcional.
- **Los pasos de instalación son los reales de hoy**: `git clone <url>` + `go build -o
  ~/.local/bin/vector ./cmd/vector` desde `cli/`. Se documenta **una sola** vía (build local
  tras clonar), no `go install <module-url>`, porque esa última requiere el módulo publicado en
  una URL pública que aún es TBD (no hay remoto configurado; ver Open questions #2 y el follow-up
  de crear el repo remoto privado). El one-liner `curl | install.sh` se menciona solo como
  "coming soon / planned" (clarificación #3).
- **Licencia marcada TBD**: no existe archivo `LICENSE` en el repo; no se inventa una licencia
  (clarificación #5).
- **No se incluyen Roadmap de estado ni Comparativa** (vs Linear, GitHub Projects): fuera del
  scope explícito del usuario (clarificación #5).
- **Walkthrough y Contributing/License son las dos secciones opcionales elegidas** por el usuario
  (clarificación #5). No se añaden secciones opcionales adicionales.
- **Solo se modifica `README.md` (raíz)**: ningún otro archivo del repo.
- **Jargon siempre con contexto mínimo**: decisión editorial para que la audiencia (sin
  familiaridad previa con OpenSpec) entienda el README sin tener que leer la documentación técnica
  completa primero.
- **El spec del instalador `curl | install.sh` es un follow-up separado**: se abre como un
  `/vector:raw` aparte; no es entregable de este spec (clarificación #3).

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, no
implementarla.

---

## 11. Edge cases

### Jargon sin contexto

- Si el agente introduce términos como "OpenSpec", "spec", "token routing" o "board" sin contexto,
  el README es inútil para la audiencia objetivo (developers sin familiaridad previa).
- Comportamiento esperado: cada término aparece con 1–2 líneas de contexto la primera vez, o
  con un link a `docs/` si la explicación completa vive allí.

### Links rotos

- Si el agente incluye paths a `docs/*.md` o `docs/assets/*.png` que no existen, el README
  estará roto en GitHub.
- Comportamiento esperado: verificar con `ls <path>` (§8 comandos de verificación) antes de
  finalizar. Si un archivo referenciado no existe, omitir el link o marcarlo como TBD.

### Descripciones de commands incorrectas

- Si el agente inventa descripciones para los `/vector:*` commands sin verificarlas contra
  `kit/commands/vector/*.md`, la tabla de referencia será inexacta.
- Comportamiento esperado: leer los 11 archivos fuente de los commands antes de redactar la tabla.

### Texto sin humanizar

- Si el agente omite el paso `/humanizer` o lo aplica parcialmente, la prosa tiene señales de
  generación por IA que dañan la credibilidad del README como artefacto público.
- Comportamiento esperado: `/humanizer` es obligatorio para cada sección. No es una optimización
  opcional.

### `install.sh` presentado como disponible

- Si el agente presenta el one-liner `curl | install.sh` como un paso funcional de instalación,
  el README promete algo que no existe.
- Comportamiento esperado: mencionarlo solo como "coming soon / planned", nunca como paso
  funcional. No incluir la URL ficticia como si fuera real.

### Licencia inventada

- Si el agente elige una licencia (MIT, Apache, etc.) por cuenta propia, el README tiene
  información falsa con implicaciones legales.
- Comportamiento esperado: la sección Contributing/License dice explícitamente que la licencia
  es TBD. No crear el archivo `LICENSE`.

### Contenido de `docs/vision.md` duplicado

- Si el agente copia secciones enteras de `docs/vision.md`, el README se vuelve redundante y
  difícil de mantener.
- Comportamiento esperado: el README referencia `docs/vision.md` como lectura adicional;
  no duplica su contenido. Máximo un párrafo de resumen con link.

### Recuento de commands

- El brief mencionó 12 commands incluyendo `quick`; `kit/commands/vector/quick.md` no existe en
  el repo. La tabla de referencia del README debe reflejar los commands verificados en el repo,
  no el número del brief.
- Comportamiento esperado: listar los commands confirmados con `ls kit/commands/vector/` al
  momento de implementar. Ver Open questions #3.

---

## 12. Estados de UI requeridos

No aplica — esta fase produce un documento Markdown estático. No hay componentes de UI,
estados de carga ni interactividad. El README tiene el mismo "estado" en cualquier contexto de
lectura.

---

## 13. Validaciones

No aplica — no hay formularios, campos de input ni validaciones de datos en este entregable.
La verificación de calidad del contenido se realiza mediante revisión manual según los criterios
de §8.

---

## 14. Seguridad y permisos

No aplica en sentido convencional — el README es un documento público y estático. Las únicas
consideraciones relevantes para este entregable son:

- **No incluir secrets, tokens ni API keys** en los ejemplos de código del README. Los ejemplos
  de instalación/uso no deben referenciar valores sensibles reales.
- **No hardcodear tokens privados** en los ejemplos (p. ej. `sk_...`, `pk_dev_...`).
- El README es un artefacto público (visible en GitHub); cualquier dato sensible en él queda
  expuesto sin posibilidad de revocación retroactiva.

---

## 15. Observabilidad y logging

No aplica — el entregable es un documento Markdown estático. No hay runtime, logs ni mecanismos
de observabilidad para este tipo de artefacto.

---

## 16. i18n / textos visibles

**Distinción crítica — dos idiomas, dos artefactos distintos:**

- **Este spec** (el documento que estás leyendo) está en **español** — convención del repo
  (`.claude/rules/documentation/docs-standards.md`).
- **El entregable** (`README.md`) está en **inglés** — decisión explícita del usuario; el
  README es un artefacto público/comercial (clarificación #2). Esta distinción no es un error;
  es intencional y fue decidida explícitamente.

El README no usa sistema de traducciones: es un documento Markdown estático con un único idioma
(inglés). Los encabezados y textos visibles del README son:

| Sección del README | Encabezado en inglés |
|---|---|
| Qué es Vector | `## What is Vector` |
| Por qué | `## Why Vector` |
| Instalación | `## Installation` |
| Quickstart | `## Quickstart` |
| Conceptos clave | `## Key Concepts` |
| Referencia de commands | `## Commands Reference` |
| Walkthrough | `## Walkthrough — End-to-End Flow` |
| Contribución y licencia | `## Contributing / License` |
| Lectura adicional | `## Further Reading` |

Todos los slugs, flags, rutas y commands permanecen en kebab-case inglés en ambos artefactos
(este spec y el README), independientemente del idioma del entorno.

---

## 17. Performance

No aplica — el entregable es un documento Markdown estático. No hay rendering dinámico, llamadas
de API ni procesamiento en runtime. El único criterio de "tamaño" relevante es la legibilidad:
el README debe ser sustancioso pero no excesivamente largo. Objetivo orientativo: 150–300 líneas
(excluyendo bloques de código y la tabla de commands), para que sea completo sin abrumar.

---

## 18. Restricciones

El agente no debe:

- **Inventar una licencia**: no añadir MIT, Apache, GPL ni ninguna licencia al README. No crear
  un archivo `LICENSE`. La sección de licencia dice únicamente "TBD".
- **Presentar `curl | install.sh` como funcional**: el one-liner no existe en el repo. Solo se
  menciona como "coming soon / planned".
- **Duplicar el contenido de `docs/vision.md`**: el README referencia `docs/`; no reproduce la
  visión técnica larga.
- **Dejar prosa sin humanizar**: todos los textos del README deben pasar por `/humanizer`. No
  entregar un README con prosa generada sin este paso.
- **Introducir jargon sin contexto**: spec, OpenSpec, token routing y board siempre con 1–2
  líneas de contexto o link a `docs/` la primera vez que aparecen.
- **Modificar archivos fuera del scope**: solo `README.md` (raíz). No tocar `docs/`, `kit/`,
  `cli/`, `web/` ni ningún otro archivo.
- **Prometer features no implementadas** como disponibles.
- **Añadir badges de CI o shields** que dependan de pipelines no configurados (mostrarían
  "failing" en GitHub).
- **Escribir el README en español**: el idioma del entregable es inglés sin excepción.
- **Inventar descripciones para los `/vector:*` commands** sin verificar contra
  `kit/commands/vector/*.md`.
- **Incluir el command `/vector:quick`** en la tabla de referencia a menos que `quick.md`
  exista en `kit/commands/vector/` al momento de implementar (ver Open questions #3).

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `README.md` reescrito con las 9 secciones definidas en §6: What is Vector, Why Vector,
      Installation, Quickstart, Key Concepts, Commands Reference, Walkthrough, Contributing/License,
      Further Reading.
- [ ] El README está íntegramente en inglés (excepto slugs/paths que son kebab-case invariante).
- [ ] Toda la prosa del README pasó por `/humanizer` (sin señales de prosa generada por IA).
- [ ] La sección Installation muestra los pasos reales de hoy (`go build` desde `cli/` +
      `vector init` por repo); `curl | install.sh` marcado como "coming soon / planned".
- [ ] El walkthrough end-to-end (`vector init` → `/vector:raw` → `/vector:propose` →
      `/vector:apply` → board) está presente como ejemplo concreto y narrativo.
- [ ] La sección Contributing/License está presente con la licencia marcada como TBD
      (no inventada).
- [ ] Los links a `docs/vision.md`, `docs/domain-contract.md`, `docs/plugin-and-commands.md`,
      `docs/commercialization.md` están presentes y apuntan a paths verificados.
- [ ] `docs/assets/kanban-reference.png` referenciada en la sección visual apropiada con alt text
      descriptivo.
- [ ] La tabla de `/vector:*` commands tiene descripciones verificadas contra
      `kit/commands/vector/*.md` (11 commands confirmados; agente verifica el recuento exacto
      al momento de implementar).
- [ ] El README no contiene texto del estado anterior ("Nada implementado todavía", "captura
      inicial").
- [ ] Ningún otro archivo del repo fue modificado.
- [ ] Los comandos de verificación de §8 pasaron sin resultados inesperados.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Leí el README actual (`README.md`, raíz) para entender qué hay y qué reemplazar.
- [ ] Verifiqué los commands en `kit/commands/vector/` con `ls` y usé las descripciones reales de
      los archivos fuente, no descripciones inventadas.
- [ ] Verifiqué que los paths a `docs/` existen con `ls` antes de incluirlos en el README.
- [ ] Pasé todos los textos del README por `/humanizer` — este paso es obligatorio.
- [ ] El README está en inglés sin excepción (excepto slugs/paths kebab-case).
- [ ] La instalación muestra pasos reales (`go build` / `go install`); `curl | install.sh` está
      marcado como "coming soon / planned".
- [ ] El walkthrough end-to-end está presente como ejemplo concreto y narrativo.
- [ ] La sección Contributing/License dice "TBD" para la licencia (no inventé una).
- [ ] No hay jargon sin contexto (spec, OpenSpec, token routing, board tienen su 1–2 líneas de
      explicación o link a `docs/`).
- [ ] No duplicé el contenido de `docs/vision.md`.
- [ ] La imagen `docs/assets/kanban-reference.png` está referenciada con path correcto y alt text.
- [ ] No prometí features no implementadas como disponibles.
- [ ] No modifiqué ningún archivo fuera de `README.md` (raíz).
- [ ] Ejecuté los comandos de verificación de §8 y todos pasaron.
- [ ] No dejé texto sin humanizar ni TODOs sin justificar en el README.

---

## Open questions

1. **Licencia exacta**: no existe archivo `LICENSE` en el repo. La sección Contributing/License
   del README dice "TBD — ver Open questions". ¿Cuál es la licencia planeada? Responder para una
   fase siguiente; no bloquea este spec.

2. **URL pública del repo**: el paso de instalación necesita un `git clone <url>`. La URL pública
   del repo es TBD — ver Open questions. Si no está lista para revelar, el agente puede usar
   `github.com/<org>/vector` como placeholder y marcarlo como TBD en el README hasta que se
   confirme.

3. **Recuento de `/vector:*` commands**: el brief indicó 12 commands incluyendo `quick`. Al
   verificar el repo, `kit/commands/vector/quick.md` no existe (Glob retorna 11 archivos: raw,
   propose, apply, bug, comment, link, close, archive, status, standup, sync). El agente que
   implemente este spec debe verificar el recuento exacto con `ls kit/commands/vector/*.md` al
   momento de implementar y usar solo los commands presentes en el repo.

4. **Follow-up — Instalador one-liner**: el usuario pidió abrir un `/vector:raw` separado para el
   spec del instalador `curl | install.sh`. Acción de seguimiento pendiente (fuera del scope de
   este spec): `/vector:raw "one-step installer: curl | install.sh for vector CLI distribution"`.
