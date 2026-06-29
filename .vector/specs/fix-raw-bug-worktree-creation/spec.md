# Spec: Aislar cada spec en su worktree branch-per-spec al crearlo (`/vector:raw` y `/vector:bug`)

## 1. Objetivo

Construir, en el orchestration de los comandos `/vector:raw` y `/vector:bug`, un paso previo a
la escritura del spec doc que **resuelva o cree el git worktree branch-per-spec** cuando el
repo del usuario declara un layout bare+worktree (su `spec-path`/`changes-path` resuelto
contiene el placeholder `[branch]`). Hoy esos comandos escriben el doc en la ruta resuelta sin
crear el worktree, dejando un directorio suelto que rompe el aislamiento por rama y bloquea el
flujo posterior (`/vector:propose`, `/vector:apply`).

Esta feature permite que un **developer que usa Vector sobre un repo con layout branch-per-spec**
pueda **crear un spec ya aislado en su propio worktree y rama (`<prefijo>/<slug>`) desde el
primer comando**, para que el spec quede tracked en su rama dedicada y nunca arranque sobre
`code/main`, eliminando el setup manual de worktree que hoy recae en `/vector:apply`.

## 2. Alcance

### Incluido en esta fase

- En `kit/commands/vector/raw.md` y `kit/commands/vector/bug.md`: un paso de **worktree-resolve/create**
  que corre **antes** de escribir el spec doc / invocar `vector spec create --body-file`, condicionado
  a que el config resuelto declare layout worktree (placeholder `[branch]` presente en `spec-path`
  o `changes-path`).
- Comando ejecutado: `git worktree add <worktree-root>/<slug> -b <prefijo>/<slug> <base-branch>`,
  donde `<worktree-root>` es el prefijo literal del template antes de `[branch]` (p. ej. `code`),
  `<base-branch>` viene del config (default `main`) y `<prefijo>` es el prefijo de rama
  configurable (default `feat/`).
- **Idempotencia**: si el slug ya tiene un worktree/rama activos (`git worktree list` lo lista),
  reutilizarlos sin error y sin recrear.
- El spec doc se escribe **dentro** del worktree creado/reutilizado (la ruta resuelta por el
  binario ya cae bajo `<worktree-root>/<slug>/…`), quedando tracked en la rama feature.
- **Inertidad en repos no-worktree**: si el `spec-path` resuelto NO contiene `[branch]` (p. ej. el
  propio repo de Vector, `.vector/specs/<slug>/`), el paso se omite por completo; comportamiento
  idéntico al actual, sin worktree.
- Propagación single-source: editar `kit/commands/vector/{raw,bug}.md` → `go generate
  ./internal/scaffold` → reinstalar binario → `vector update` (ver §10).
- Si el binario debe exponer la información de layout (root, base-branch, prefijo, flag
  `[branch]`) para que el orchestration la consuma sin parsear `.project-structure`/config a mano:
  extender `vector context --json` con esos campos. `TBD — ver Open questions` (Q-A).

### Fuera de scope

- **Recuperación/limpieza de stubs sueltos previos**: directorios `code/<slug>/` dejados por runs
  buggy anteriores NO se detectan, migran ni borran. Son responsabilidad del usuario (decisión
  tomada, §10). Si `git worktree add` falla porque el path suelto ya existe, se surfacea el error
  de git de forma accionable, sin auto-borrar.
- Mover el worktree-resolve/create dentro del binario (`vector spec create`). Decidido: vive en el
  orchestration del comando (§10). El binario sigue worktree-unaware (solo `MkdirAll`+write del doc).
- Tocar `/vector:propose` y `/vector:apply` más allá de heredar el beneficio. La corrección de que
  `propose`/`apply` operen en el worktree correcto se sigue de que el worktree ya exista; cualquier
  ajuste propio de esos comandos es follow-up, no parte de esta fase. `TBD — ver Open questions` (Q-B).
- `/vector:quick` (también crea cards) — fuera de este fix salvo que se decida unificar el paso.
  `TBD — ver Open questions` (Q-C).
- Cambiar el esquema de estado, los endpoints HTTP del board, o el web.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Framework: binario Go (`cli/`, módulo único, stdlib) + comandos markdown del `kit/`.
- Lenguaje: Go 1.26 (binario); Markdown (orchestration de comandos `/vector:*`).
- Package manager: go-modules.
- UI library: No aplica — el cambio es en orchestration de comandos y, condicionalmente, en `vector context`.
- State management: No aplica.
- API client: No aplica.
- Forms: No aplica.
- Validation: No aplica.
- Testing: paquete `testing` estándar de Go (table-driven), + `TestAssetsMatchKit` para drift kit↔assets.

### Versiones relevantes

- Go: 1.26 (de `.vector` intel / `go.mod`; verificar contra `cli/go.mod`).
- Dependencias externas: ninguna (stdlib + `git` invocado como proceso).

### Patrones existentes a respetar

- **Single-source kit→assets→.claude**: `kit/` es la única fuente editable; `cli/internal/scaffold/assets/`
  es copia generada por `go generate`; `.claude/commands/vector/` son copias sembradas por el binario.
  Editar solo en `kit/` (ver `.claude/rules/architecture/distribution-packaging.md`).
- **CLI-owns-writes**: el binario es el único escritor del estado JSON. El orchestration NO escribe
  estado a mano; el worktree es una operación de git sobre el repo del usuario, no sobre el estado.
- **Config aditivo/omitempty, `SchemaVersion` sin cambios**: un field nuevo de config (p. ej. prefijo
  de rama) sigue el patrón de `applyMode`/`changesPath`/`language` (aditivo, retrocompatible).
- **Operaciones sobre el repo del usuario**: `git worktree add` muta el repo del usuario; respetar
  `.claude/rules/security/destructive-ops-consent.md` (ver §14). Es una operación reversible
  (`git worktree remove`), de bajo riesgo, pero debe surfacear fallos en vez de forzar.
- Resolución de `[branch]`: `cli/internal/config/config.go` (`SpecDocPath`,
  `branchPlaceholder`, `deriveChangesPath`) ya resuelve `[branch]`→slug.

---

## 4. Dependencias previas

> Esta sección, en el spec bug-framed, documenta también la **causa raíz deducida**.

Antes de iniciar esta fase debe existir o estar completado:

- [x] `vector spec create --body-file` escribiendo el doc en la ruta config-resuelta
      (`cli/cmd/vector/main.go` `runSpecCreate` ~L771 → `cfg.SpecDocPath` → `store.CreateSpec`).
- [x] Resolución del placeholder `[branch]` en `cli/internal/config/config.go` (`SpecDocPath` ~L372-384,
      `branchPlaceholder` L355, `deriveChangesPath` L819).
- [x] `vector context --json` (`cli/cmd/vector/context.go`) como canal de contexto para el orchestration.
- [x] Flujo de propagación single-source operativo (`go generate ./internal/scaffold` + `TestAssetsMatchKit`).

### Causa raíz deducida (git)

- El defecto entró en el commit **`d91f8a5`** *"feat: powerful /vector:raw, per-repo config, and vector
  update"*: `vector spec create` empezó a escribir el doc en la ubicación config-resuelta — incluyendo
  rutas con `[branch]` resuelto — pero **sin crear el worktree**. `store.CreateSpec` usa `os.MkdirAll`
  (`cli/internal/state/store.go` ~L262-269), creando un directorio común, no un worktree.
- La **conciencia de layout worktree se añadió después y solo al lado de lectura** (`vector sync`):
  commits **`820f283`**, **`e25efde`**, **`5150d00`**, **`ed0988e`**. El lado de **escritura/creación**
  (`vector spec create` y el orchestration de raw/bug) **nunca** se hizo worktree-aware.
- Ninguno de esos commits mapea a un **spec card trackeado** en el board actual (el trabajo
  *powerful /vector:raw* precede al tracking de specs). Por eso **este card se registra sin
  `relatedTo[]`**: los commits se citan aquí como señal de inferencia, no como relación almacenada.
  Una relación `relatedTo` alucinada sería peor que ninguna.

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta. No debe
inventar contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

Orchestration en markdown (comando `/vector:*`) que delega operaciones de git al shell y la
escritura del doc/estado al binario. Sin nuevas abstracciones en Go salvo, condicionalmente, la
extensión de `vector context` para exponer el contexto de layout.

### Capas afectadas

Esta fase puede tocar únicamente las siguientes capas:

- presentation: No aplica (no hay UI).
- application/use-cases: **sí** — el orchestration de `/vector:raw` y `/vector:bug` (`kit/commands/vector/{raw,bug}.md`)
  gana un paso de worktree-resolve/create condicional.
- domain: No aplica (el estado/máquina de estados no cambia).
- data/infrastructure: **condicional** — si se decide exponer el contexto de layout, `vector context`
  (`cli/cmd/vector/context.go`) y `cli/internal/config/config.go` (helper tipo `HasBranchPlaceholder`,
  accesores de root/base-branch/prefijo). `TBD — ver Open questions` (Q-A).
- shared/common: No aplica.

### Flujo esperado

1. El usuario ejecuta `/vector:raw [idea]` o `/vector:bug [report]`.
2. El comando obtiene contexto del repo (`vector context --json`) y determina si el layout es
   bare+worktree (placeholder `[branch]` presente en `spec-path`/`changes-path`).
3. Si **NO** hay `[branch]` (repo no-worktree): se omite el paso de worktree (inerte) y el flujo
   continúa como hoy.
4. Si **sí** hay `[branch]`: el comando resuelve `<worktree-root>` (prefijo antes de `[branch]`),
   `<base-branch>` (config, default `main`) y `<prefijo>` (config, default `feat/`).
5. Si `git worktree list` ya lista `<worktree-root>/<slug>`: reutilizarlo (idempotente).
   Si no: `git worktree add <worktree-root>/<slug> -b <prefijo>/<slug> <base-branch>`.
6. Si `git worktree add` falla porque el path ya existe pero NO es worktree (stub suelto previo):
   abortar con un mensaje accionable (sin auto-borrar — fuera de scope, §2).
7. El comando escribe el spec doc / invoca `vector spec create --body-file`; el doc cae dentro del
   worktree (la ruta resuelta por el binario ya está bajo `<worktree-root>/<slug>/…`) y queda tracked
   en la rama feature.

### Ubicación de archivos nuevos

No se crean carpetas nuevas. Cambios en archivos existentes (ver §6). El flujo de edición es
single-source: `kit/commands/vector/*.md` → `go generate` → assets → `vector update`.

No crear carpetas nuevas si ya existe una convención equivalente en el proyecto.

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/commands/vector/raw.md` | MODIFICAR | Añadir el paso worktree-resolve/create condicional (`[branch]`) antes de escribir el doc | `kit/commands/vector/bug.md` (estructura de pasos) |
| `kit/commands/vector/bug.md` | MODIFICAR | Mismo paso worktree-resolve/create condicional, simétrico a raw | `kit/commands/vector/raw.md` |
| `cli/internal/scaffold/assets/commands/vector/raw.md` | MODIFICAR (generado) | Regenerar vía `go generate` para reflejar el kit | — |
| `cli/internal/scaffold/assets/commands/vector/bug.md` | MODIFICAR (generado) | Regenerar vía `go generate` para reflejar el kit | — |
| `cli/cmd/vector/context.go` | MODIFICAR (condicional) | Exponer contexto de layout (root, base-branch, prefijo, flag `[branch]`) si el orchestration lo necesita | `cli/cmd/vector/context.go` (campos existentes) `TBD` (Q-A) |
| `cli/internal/config/config.go` | MODIFICAR (condicional) | Helper `HasBranchPlaceholder`/accesores de root y prefijo de rama configurable | `deriveChangesPath` (L819) `TBD` (Q-A) |

### Detalle por archivo

#### `kit/commands/vector/raw.md` y `kit/commands/vector/bug.md`

Acción: MODIFICAR

Deben implementar:

- Un paso explícito, previo a la escritura del spec doc, que: (a) detecte layout worktree por
  presencia de `[branch]`; (b) resuelva root/base-branch/prefijo; (c) reutilice o cree el worktree
  con `git worktree add … -b <prefijo>/<slug> <base-branch>`, idempotente; (d) en repos no-worktree,
  sea inerte; (e) ante fallo de git (incl. stub suelto previo), surfacee el error de forma accionable.
- Un recordatorio de que el doc debe quedar **dentro** del worktree (tracked en la rama feature).

No deben incluir:

- Lógica de limpieza/migración de stubs sueltos previos (fuera de scope, §2).
- Escritura de estado a mano (sigue siendo del binario).

#### `cli/cmd/vector/context.go` / `cli/internal/config/config.go`

Acción: MODIFICAR (condicional — solo si el orchestration no puede derivar el contexto de layout sin él)

Cambios requeridos:

- Exponer de forma estructurada: `worktreeRoot`, `baseBranch` (default `main`), `branchPrefix`
  (default `feat/`), y un flag de si el layout usa `[branch]`. `TBD — ver Open questions` (Q-A).

Restricciones:

- No cambiar el comportamiento de resolución de `[branch]` existente.
- Cualquier field nuevo de config es aditivo/omitempty; `SchemaVersion` permanece en 1.
- No refactorizar partes no relacionadas.

---

## 7. API Contract

No aplica — esta fase no introduce ni cambia endpoints HTTP. El único "contrato" relevante es la
forma de `vector context --json` (consumida por el orchestration); si se extiende, los nuevos campos
son aditivos y se documentan en `docs/` correspondiente. No existe `docs/api-contract.md` para este cambio.

### Endpoints involucrados

- Ninguno.

---

## 8. Criterios de éxito

> Contraste **esperado vs. actual** (núcleo del bug):
>
> - **Actual**: con `spec-path = code/[branch]/docs/specs/<slug>/`, crear un spec deja
>   `code/<slug>/docs/specs/<slug>/spec.md` como **directorio suelto**: no aparece en
>   `git worktree list`, no existe la rama `feat/<slug>`, el doc no está aislado y el path suelto
>   bloquea un `git worktree add code/<slug>` posterior.
> - **Esperado**: el comando crea/reutiliza el worktree (`git worktree add code/<slug> -b feat/<slug>
>   <base-branch>`) **antes** de escribir el doc, dejándolo tracked en su rama; en repos sin `[branch]`
>   el paso es inerte.

La implementación se considera correcta cuando:

- [ ] Con `spec-path` que contiene `[branch]`, `/vector:raw` y `/vector:bug` crean un worktree en
      `<worktree-root>/<slug>` con rama `<prefijo>/<slug>` basada en `<base-branch>` **antes** de
      escribir spec.md.
- [ ] El spec doc queda dentro del worktree y tracked en la rama feature (no en `code/main`).
- [ ] Si el worktree/rama ya existen para ese slug, se reutilizan sin error (idempotente).
- [ ] `git worktree list` lista el nuevo path tras la creación; la rama `<prefijo>/<slug>` existe y
      está asociada al worktree.
- [ ] `<base-branch>` se lee del config (default `main`); `<prefijo>` se lee del config (default `feat/`):
      no hay `main` ni `feat/` hardcodeados de forma rígida.
- [ ] En repos sin `[branch]` en `spec-path` (p. ej. el repo de Vector), el comportamiento es idéntico
      al actual: **no** se crea worktree (paso inerte). Sin regresión.
- [ ] Ante un path suelto preexistente que impide `git worktree add`, el comando aborta con un mensaje
      accionable (no auto-borra).
- [ ] `go vet`, `gofmt`, y `go test ./...` verdes; `TestAssetsMatchKit` pasa (assets sincronizados con kit).

### Tests requeridos

Agregar o actualizar tests para:

- [ ] Caso layout worktree: creación produce worktree + rama + doc tracked.
- [ ] Idempotencia: segundo run con el mismo slug reutiliza sin error.
- [ ] Regresión no-worktree: `spec-path` sin `[branch]` no crea worktree.
- [ ] Resolución de `<base-branch>`/`<prefijo>` desde config (default y override).
- [ ] Stub suelto previo → fallo accionable (no silencioso, no auto-borrado).
- [ ] `TestAssetsMatchKit` tras `go generate` (sin drift kit↔assets).

### Comandos de verificación

Ejecutar (forma CWD-agnóstica, desde la raíz del repo):

```bash
go -C cli generate ./internal/scaffold   # primero: regenera assets desde kit/
gofmt -l cli
go -C cli vet ./...
go -C cli test ./...                      # incluye TestAssetsMatchKit
go -C cli build ./...                     # gate de compilación (ruta condicional Q-A)
```

`go generate` debe correr **antes** de los tests para que `TestAssetsMatchKit` valide assets
frescos, no obsoletos. La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

No aplica — el cambio no toca UI del board. La "UX" relevante es la del comando en terminal:

- El paso de worktree debe reportar al usuario qué hace (crear vs. reutilizar el worktree, rama y base).
- Ante fallo de git, el mensaje debe ser accionable (indicar el path suelto y la acción manual sugerida),
  sin dejar el spec a medio registrar de forma ambigua.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas y el agente no debe cuestionarlas ni cambiarlas:

- **Ownership**: la lógica de worktree-resolve/create vive en el **orchestration del comando markdown**
  (`/vector:raw`, `/vector:bug`), **no** en el binario. El binario sigue worktree-unaware
  (`MkdirAll`+write del doc dentro del worktree ya creado).
- **Stub previo**: alcance = **solo prevenir nuevos**. No se detecta, migra ni borra ningún
  `code/<slug>/` suelto dejado por runs anteriores; es responsabilidad del usuario. Ante fallo, error
  accionable, sin auto-borrado.
- **Branch/base**: **configurable con defaults**. Base = `base-branch` del config (default `main`);
  prefijo de rama configurable (default `feat/`); rama = `<prefijo>/<slug>`. Sin hardcodear `main`/`feat/`.
- **Trigger**: la creación de worktree se dispara **solo** cuando el `spec-path`/`changes-path` resuelto
  contiene `[branch]`. En repos no-worktree el paso es **inerte**.
- **Propagación**: edición single-source en `kit/`, luego `go generate` + `vector update`.
- **Sin `relatedTo[]`**: la causa no mapea a un spec card trackeado; no se inventa relación.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, pero no
implementarla.

---

## 11. Edge cases

> Incluye los **pasos de reproducción** del bug (precondición + secuencia que evidencia el defecto).

### Reproducción del defecto (estado actual)

Precondiciones: repo con `.project-structure` → `spec-path = code/[branch]/docs/specs/<slug>/`
(branch-per-spec); git con rama `main`.

1. Ejecutar `/vector:raw [idea]` o `/vector:bug [report]` con un slug nuevo (p. ej.
   `fix-calendar-schedule-editor-divergence`).
2. El comando registra el spec en `draft`.
3. `ls code/<slug>/` → solo `docs/`.
4. `git worktree list` → **NO** lista ese path.
5. `git -C code branch -a` → **NO** existe `feat/<slug>`.
6. `git worktree add code/<slug> -b feat/<slug> main` → **falla**: el path ya existe.

La implementación corregida debe manejar explícitamente:

### Worktree ya existente para el slug

- Reutilizar sin recrear ni error (idempotencia). Verificar contra `git worktree list`.

### Path suelto preexistente (no worktree)

- `git worktree add` falla porque el directorio ya existe. Comportamiento esperado: abortar con
  mensaje accionable (indicar el path y la acción manual). **No** auto-borrar (fuera de scope, §2).

### Repo no-worktree (`spec-path` sin `[branch]`)

- Paso inerte: no crear worktree; comportamiento idéntico al actual. Sin regresión.

### Base branch distinta de `main`

- Si el config declara `base-branch` (p. ej. `master`/`develop`), usarlo. No asumir `main`.

### Repo sin git inicializado / `git` ausente

- El layout worktree implica git. Si `git` falla por entorno (no es repo git, binario ausente),
  surfacear el error de forma accionable; no continuar dejando un stub. `TBD — ver Open questions` (Q-D).

### Rama feature ya existente pero sin worktree asociado

- `git worktree add -b` fallaría por rama duplicada. Comportamiento esperado: detectar y reutilizar
  la rama (`git worktree add <path> <rama-existente>` sin `-b`) o abortar accionable.
  `TBD — ver Open questions` (Q-D).

### Sin conexión / Timeout / Respuesta vacía / Doble submit

- No aplican — operación local de git, sin red ni UI con submit.

---

## 12. Estados de UI requeridos

No aplica — no hay UI en este cambio. La salida es texto del comando en terminal (creó/reutilizó
worktree; o error accionable).

---

## 13. Validaciones

### Validaciones de cliente

| Campo | Regla | Mensaje |
|---|---|---|
| layout (`[branch]`) | Solo crear worktree si el `spec-path`/`changes-path` resuelto contiene `[branch]` | (interno; sin worktree si ausente) |
| `<slug>` | kebab-case, derivado del spec (raw) o `fix-<slug>` (bug) | (reusa validación de slug existente) |
| `<base-branch>` | Debe existir en el repo para basar el worktree | Error accionable de git si no existe |

### Validaciones de servidor

No aplica — no hay servidor involucrado.

---

## 14. Seguridad y permisos

- `git worktree add` **muta el repo del usuario**: aplica `.claude/rules/security/destructive-ops-consent.md`.
  Es una operación **reversible** (`git worktree remove` / `git branch -d`) y de bajo riesgo (crea, no borra),
  por lo que no requiere backup previo como una reorganización; pero **no** debe forzar sobre estado sucio:
  ante conflicto (path suelto, rama duplicada) aborta accionable en vez de sobrescribir.
- No exponer secrets ni tokens (no hay credenciales en este flujo).
- No imprimir contenido sensible; la salida se limita a paths/ramas y errores de git.

---

## 15. Observabilidad y logging

Usar el mecanismo de salida existente del comando (mensajes al usuario en terminal) y los errores
envueltos del binario (`fmt.Errorf("…: %w", err)`).

Registrar/surfacear:

- Creación vs. reutilización del worktree (qué path, qué rama, qué base).
- Fallo de `git worktree add` con el error de git íntegro y la acción sugerida.

No registrar:

- Nada sensible (no hay payloads ni credenciales en este flujo).

---

## 16. i18n / textos visibles

Los mensajes de prosa de los comandos siguen `config.language` (default inglés para artefactos;
la conversación con el usuario en su idioma). Slugs, paths, nombres de rama y artefactos de git
permanecen en inglés kebab-case. No hay catálogo i18n formal en el `kit/`.

| Key | Texto |
|---|---|
| worktree.created | (prosa generada) "Created worktree `<root>/<slug>` on `<prefijo>/<slug>`" |
| worktree.reused | (prosa generada) "Reusing existing worktree for `<slug>`" |
| worktree.error.stub | (prosa generada) error accionable: path suelto preexistente |

---

## 17. Performance

- El paso añade una invocación de git por creación de spec en repos worktree (despreciable).
- Evitar invocaciones de git redundantes: una consulta de `git worktree list` para idempotencia,
  y `git worktree add` solo si no existe.
- En repos no-worktree, costo cero (paso inerte, sin git).

---

## 18. Restricciones

El agente no debe:

- Mover la lógica al binario (decidido: vive en el orchestration del comando).
- Detectar/limpiar/migrar stubs sueltos previos (fuera de scope).
- Hardcodear `main` o `feat/` de forma rígida (deben venir del config con esos defaults).
- Crear worktree en repos sin `[branch]` (debe ser inerte).
- Auto-borrar directorios del usuario ante conflicto.
- Cambiar el esquema de estado, la máquina de estados o los endpoints.
- Editar `cli/internal/scaffold/assets/**` a mano (se regenera con `go generate`).
- Instalar dependencias nuevas (stdlib + `git` proceso).
- Ignorar errores de `gofmt`/`go vet`/tests/`TestAssetsMatchKit`.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `kit/commands/vector/raw.md` y `bug.md` con el paso worktree-resolve/create condicional.
- [ ] Assets regenerados (`go generate ./internal/scaffold`) sin drift (`TestAssetsMatchKit` verde).
- [ ] (Condicional Q-A) `vector context`/`config` exponiendo el contexto de layout, con tests.
- [ ] Tests: layout worktree, idempotencia, regresión no-worktree, defaults/override de base+prefijo,
      stub previo accionable.
- [ ] Edge cases cubiertos (§11).
- [ ] Documentación actualizada si se extiende `vector context` o el config (p. ej. `docs/` y `README`).

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] No existe `docs/api-contract.md` para este cambio (no aplica); confirmé el contrato de `vector context` si lo extendí.
- [ ] Confirmé que las dependencias previas (§4) existen.
- [ ] Solo modifiqué archivos listados en §6 o justifiqué cualquier excepción.
- [ ] Edité solo en `kit/` y regeneré assets con `go generate` (no edité `assets/` a mano).
- [ ] El paso es inerte en repos no-worktree (sin regresión).
- [ ] Implementé idempotencia y el manejo accionable de stub previo.
- [ ] Base y prefijo se leen del config con defaults `main`/`feat/`.
- [ ] No agregué dependencias no autorizadas.
- [ ] No cambié decisiones tomadas (§10).
- [ ] Ejecuté `gofmt -l .`.
- [ ] Ejecuté `go vet ./...`.
- [ ] Ejecuté `go test ./...` (incl. `TestAssetsMatchKit`).
- [ ] No dejé logs temporales ni TODOs sin justificar.

---

## Open questions

- **Q-A — ¿El orchestration necesita que el binario exponga el contexto de layout?** Si el comando
  puede derivar `worktree-root`/`base-branch`/`prefijo`/flag `[branch]` desde `vector context --json`
  actual, no se toca Go. Si no, extender `vector context` + helper en `config.go` (aditivo). `TBD`.
- **Q-B — ¿`/vector:propose` y `/vector:apply` necesitan ajuste propio** una vez que el worktree existe,
  o basta con que hereden el worktree correcto? El reporte indica que hoy operan sobre `code/main`; con
  el worktree creado en raw/bug, verificar si propose/apply lo detectan solos. Posible follow-up. `TBD`.
- **Q-C — ¿`/vector:quick` debe ganar el mismo paso?** También registra cards; queda fuera de este fix
  salvo decisión de unificar. `TBD`.
- **Q-D — Manejo fino de conflictos de git**: rama feature preexistente sin worktree, `git` ausente,
  repo no-git pese a `[branch]` declarado. Definir el contrato exacto de fallo accionable. `TBD`.
- **Q-E — Recuperación de stubs sueltos previos**: decidido fuera de scope ahora; ¿vale un follow-up
  que detecte y ofrezca migrar `code/<slug>/` suelto al worktree? `TBD`.
