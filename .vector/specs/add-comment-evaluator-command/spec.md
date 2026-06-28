# Spec: Comando /vector:comment para evaluar comentarios de PR

## 1. Objetivo

Construir `/vector:comment`: un **project command** del kit que toma un comentario dejado en un
PR o ticket, lo **evalúa críticamente** (con ojo de senior escéptico, tratando el comentario como
un reclamo no verificado y posiblemente escrito por otra herramienta de IA) contra el **diff real**
de la rama, emite un veredicto, e **implementa el cambio solo si es válido, valioso y de bajo
riesgo**. Es el equivalente del skill global `/pr-comment` del usuario, pero **agnosticizado** al
repo del usuario y **distribuible** dentro del binario de Vector.

Esta feature permite que un dev **rechace con confianza comentarios inválidos o superficiales**,
implemente solo los cambios que realmente aportan valor, y —cuando no se implementa— obtenga un
**reply honesto y humanizado** para responder en el hilo del PR. Al estar integrado a Vector,
**vincula el trabajo al spec card** correspondiente (registra `work.logged`), de modo que la
actividad aparece en el standup y la timeline del board.

## 2. Alcance

### Incluido en esta fase

- **Project command `/vector:comment`** (`kit/commands/vector/comment.md`): orquesta parseo del
  comentario, resolución de rama, obtención del diff, delegación al evaluador, reporte del
  veredicto, implementación condicional y generación de reply.
- **Agente evaluador `vector-comment-evaluator`** (`kit/agents/vector-comment-evaluator.md`), tier
  **Sonnet**: hace su propia recolección de evidencia (Read/Grep/Glob/Bash, read-only) y emite un
  veredicto estructurado (categoría + confianza + evidencia `file:line` + AI-red-flags +
  remediación si aplica).
- **Resolución de diff agnóstica**: el command **pregunta al usuario** cómo obtener el diff —
  vía `gh` (si está disponible y hay PR) o `git diff <base>..HEAD` local. No asume GitHub.
- **Detección de comandos de verificación** (`build`/`lint`/`test`): leer `.vector/config.json`
  (stack/run detectados en `vector init`) y manifests del repo; **preguntar al usuario** si no se
  detectan. No hardcodear `pnpm`/`npm`.
- **Integración con el estado (spec-aware)**: resolver el **spec card** asociado al trabajo y,
  al implementar, registrar un evento `work.logged` vía `vector spec worklog` (existente). Ofrecer
  —solo cuando el card está en `review`— moverlo `review → in-progress` vía `vector spec status`.
- **Generación de reply humanizado** (inglés por defecto, idioma del hilo si difiere) cuando el
  veredicto es marginal/inválido o el usuario opta por no implementar; copiado a clipboard.
- **Vendoring**: incluir el command y el agente en los assets embebidos del binario
  (`go generate` en `cli/internal/scaffold`).

### Fuera de scope

- **Postear automáticamente** el reply en el PR/ticket: el command solo redacta y copia; el
  usuario decide pegarlo. (Igual que `/pr-comment`.)
- **Validar el ticket contra el tracker** (Jira/Linear/GitHub) ni llamadas externas más allá de
  `gh` para el diff.
- **Política de "máx 5 files por fase"** y la consulta de `graphify-out/` (son específicas del
  monorepo somnio; Vector no las impone).
- **Reformular o "mejorar" el comentario** antes de evaluarlo: se evalúa tal cual.
- **Nuevos subcomandos Go ni nuevos eventos de estado**: la integración reusa los subcomandos
  existentes (`vector spec worklog`, `vector spec status`, `vector spec list`). No se crea
  `comment.evaluated` ni endpoints nuevos en la API.
- **Nueva UI del board** (panel de comentarios evaluados): la visibilidad es la que ya da
  `work.logged` en standup/timeline. No se toca `web/`.
- **Auto-commit / push** del cambio implementado: se deja el working tree para que el usuario
  revise (alineado a `apply.md`).

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- **Project command**: Markdown + frontmatter orquestado por Claude (patrón
  `kit/commands/vector/apply.md`, `link.md`).
- **Agente evaluador**: Markdown del kit (`kit/agents/vector-comment-evaluator.md`), tier
  **Sonnet** (token-routing: la evaluación escéptica es razonamiento real). Patrón de agente:
  `kit/agents/vector-spec-validator.md`.
- **Binario**: se invoca como cliente vía los subcomandos existentes `vector spec worklog`,
  `vector spec status`, `vector spec list` (Go, stdlib; `cli/`). **No** se añade código Go nuevo.
- **Detección de verificación**: lectura de `.vector/config.json` y manifests del repo
  (`package.json` scripts, `Makefile`, `go.mod`, `pyproject.toml`, etc.).

### Versiones relevantes

- Go: `1.26` (citado de `cli/go.mod`; no se toca en esta fase porque no se añade código Go).
- No se introducen dependencias nuevas.

### Patrones existentes a respetar

- **CLI-owns-writes**: el command **nunca** edita `.vector/` a mano; toda mutación de estado pasa
  por los subcomandos del binario (`workflows/state-sync-discipline.md`).
- **`work.logged` es aditivo**: appendea un evento; no muta `state.json` ni es un gate
  (`kit/commands/vector/apply.md:85`).
- **Token routing**: investigación barata (parseo, resolución de rama, traer el diff) en el main
  loop; evaluación crítica delegada a **Sonnet** y documentada en el command
  (`product/token-routing.md`).
- **Agnosticism**: detectar stack/VCS/comandos; nunca hardcodear. Si no se detecta, preguntar o
  reportar accionablemente (`product/principles.md`).
- **Humanización del reply**: aplicar los patrones anti-IA del skill `humanizer` (sin aperturas
  sicofánticas, sin relleno, sin adjetivos inflados, sin em-dash overuse).
- **Idioma**: el reporte del veredicto sigue el idioma configurado del proyecto (`config.language`
  en `.vector/config.json`), con fallback al idioma de la conversación (auto-detección); artefactos
  de git y el reply del PR en inglés por defecto (`workflows/git-convention.md`). La activación de
  `config.language` en este command es responsabilidad del change `add-agent-prose-language`.
- **Vendoring**: el command y el agente se copian a `cli/internal/scaffold/assets/` vía la
  directiva `//go:generate` (`cli/internal/scaffold/scaffold.go:13`) y se embeben con
  `//go:embed all:assets` (`scaffold.go:26`).

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Subcomando `vector spec worklog <id> [--files] [--tasks] [--note] [--json]` —
      verificado (`cli/cmd/vector/standup.go:199`).
- [x] Subcomando `vector spec status <id> <status> [--reason]` —
      verificado (`cli/cmd/vector/spec_transitions.go:143`).
- [x] Subcomando `vector spec list` (resolución del card) —
      verificado (usage en `cli/cmd/vector/main.go:537`).
- [x] Mecanismo de scaffold/vendoring funcionando (`//go:generate` + `//go:embed` en
      `cli/internal/scaffold/scaffold.go:13,26`) — verificado.
- [x] Patrón de project command (`kit/commands/vector/apply.md`, `link.md`) y de agente del kit
      (`kit/agents/vector-spec-validator.md`) — verificados.
- [x] `.vector/config.json` presente (`vector init` corrido) — verificado en este repo.
- [x] Skill `humanizer` disponible para el reply — verificado (`~/.claude/skills/humanizer`).

Si alguna dependencia no existe, el command se detiene con un mensaje accionable. No inventa
contratos, rutas ni subcomandos.

---

## 5. Arquitectura

### Patrón a usar

**Orquestación por project command + delegación a un agente evaluador escéptico (Sonnet) +
mutación de estado vía subcomandos del binario.** El command coordina; el agente juzga de forma
independiente; el binario es el único escritor del estado. Misma separación que `/vector:apply`
(orquesta e invoca `vector spec …`) y que `/pr-comment` (evaluación en subagente, acción en el
main loop).

### Capas afectadas

- **Kit command** (`kit/commands/vector/comment.md`): sí — NUEVO. Orquestación completa.
- **Kit agente** (`kit/agents/vector-comment-evaluator.md`): sí — NUEVO. Evaluador Sonnet.
- **Scaffold assets** (`cli/internal/scaffold/assets/…`): sí — generados por `go generate`
  (copia embebida; no edición manual).
- **CLI (código Go)** (`cli/cmd`, `cli/internal/state`, `cli/internal/board`): **no** — se reusan
  subcomandos existentes; no se añade código.
- **Web** (`web/`): **no** — sin nueva UI; la visibilidad la da `work.logged` en standup/timeline.

### Flujo esperado

1. Usuario ejecuta `/vector:comment "<texto-del-comentario>" [{spec-id|branch}]`.
2. **Parseo**: separar el texto del comentario (`COMMENT`) del token opcional de
   spec-id/branch/ticket. Si `COMMENT` está vacío → `AskUserQuestion` y detenerse.
3. **Resolver la rama** (`BRANCH`/`WORKTREE`): derivar de forma determinista
   (`git worktree list`, `git branch -a`); si hay cero/varios candidatos o ambigüedad →
   `AskUserQuestion` con los candidatos. Nunca adivinar.
4. **Obtener el diff**: `AskUserQuestion` para elegir el origen — `gh` (si está disponible y hay
   PR abierto: `gh pr list --head <BRANCH> …`, registra `PR_URL`/`BASE`) o `git diff <base>..HEAD`
   local. Si el diff está vacío → reportar y detenerse.
5. **Resolver el spec card** (spec-aware): si se pasó `spec-id`, usarlo; si no, intentar mapear
   `BRANCH`/ticket contra `vector spec list --json`. Si hay un match único → `SPEC_ID`. Si hay
   varios o ninguno → `AskUserQuestion` ofreciendo los candidatos **o** "ninguno" (en cuyo caso el
   command sigue funcionando como evaluador, pero **omite** el `work.logged`).
6. **Evaluar** (Sonnet): invocar el agente `vector-comment-evaluator` con `COMMENT`, `WORKTREE`,
   `BRANCH`, `BASE`, `PR_URL` (si aplica). El agente reúne su propia evidencia y retorna el
   veredicto estructurado.
7. **Verificar el veredicto**: el main loop **no** confía ciegamente — re-chequea la evidencia
   `file:line` citada; si no se sostiene, degrada el veredicto y lo nota.
8. **Reportar** (idioma configurado — `config.language`, fallback al idioma de la conversación):
   veredicto + confianza, evidencia, AI-red-flags, y qué haría falta si es válido.
9. **Elegir acción** (`AskUserQuestion`, construida según el veredicto):
   - **VÁLIDO Y VALIOSO + bajo riesgo** → ofrecer Implementar (recomendado) / Redactar reply / Nada.
   - **Marginal/Inválido/Alto riesgo** → ofrecer Redactar reply (recomendado) / Nada (Implementar
     solo si es técnicamente elegible, con plan-y-confirma).
10. **Implementar** (solo si se ganó): editar en `WORKTREE`, detectar y correr la verificación
    (`build`/`lint`/`test`), reportar resultados reales. Tras verificar OK y si hay `SPEC_ID`:
    `vector spec worklog <SPEC_ID> --files … --tasks … --note "comment: <resumen>"`; si el card
    está en `review`, ofrecer `vector spec status <SPEC_ID> in-progress`. **No** auto-commit.
11. **Redactar reply** (si se eligió): generar respuesta grounded, pasarla por `humanizer`,
    copiar a clipboard e imprimir inline. No postear.

### Ubicación de archivos nuevos

```txt
kit/commands/vector/comment.md                          # project command
kit/agents/vector-comment-evaluator.md                  # agente evaluador (Sonnet)
cli/internal/scaffold/assets/commands/vector/comment.md # copia embebida (generada)
cli/internal/scaffold/assets/agents/vector-comment-evaluator.md # copia embebida (generada)
```

No crear carpetas nuevas: ya existe la convención (`kit/commands/vector/`, `kit/agents/`).

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/commands/vector/comment.md` | NUEVO | Project command: parseo, resolución de rama/diff/spec, delegación a Sonnet, reporte, implementación condicional, reply | `kit/commands/vector/apply.md`, `link.md` |
| `kit/agents/vector-comment-evaluator.md` | NUEVO | Agente Sonnet read-only: evidencia propia + veredicto estructurado (rubric de 6 preguntas) | `kit/agents/vector-spec-validator.md` |
| `cli/internal/scaffold/assets/commands/vector/comment.md` | NUEVO (generado) | Copia embebida del command vía `//go:generate` (`scaffold.go:13`) | siblings `apply.md`, `raw.md` en `assets/` |
| `cli/internal/scaffold/assets/agents/vector-comment-evaluator.md` | NUEVO (generado) | Copia embebida del agente vía `//go:generate` | siblings `vector-spec-validator.md` en `assets/agents/` |
| `cli/internal/scaffold/scaffold_test.go` | MODIFICAR (si enumera commands) | Asegurar que el set esperado incluye `comment.md` tras vendoring | `cli/internal/scaffold/scaffold_test.go` |

### Detalle por archivo

#### `kit/commands/vector/comment.md`

Acción: NUEVO

Debe implementar (frontmatter + cuerpo en pasos, espejo agnosticizado de `/pr-comment`):

- **Frontmatter**: `name: comment`, `description`, `argument-hint: "[comment-text] {spec-id|branch}"`,
  `user-invocable: true`, `allowed-tools` (Read, Grep, Glob, Edit, Write, `Bash(git *)`,
  `Bash(gh *)`, `Bash(vector *)`, Agent, AskUserQuestion, Skill).
- **Pasos** según §5 (Flujo): parseo → resolver rama (ask si ambigua) → obtener diff (ask gh/local)
  → resolver spec card (ask; permitir "ninguno") → invocar agente Sonnet → re-chequear evidencia →
  reportar veredicto (idioma configurado) → elegir acción → implementar condicional + verificación detectada +
  `vector spec worklog` → reply humanizado.
- **Token routing**: documentar por qué la evaluación va a Sonnet y la orquestación al main loop.
- **Recordatorio de disciplina de estado**: al implementar con `SPEC_ID`, registrar `work.logged`
  vía el binario; nunca editar `.vector/` a mano.

No debe incluir: lógica somnio-específica (pnpm, graphify-out, máx-5-files, worktree layout fijo),
ni postear el reply automáticamente, ni auto-commit.

#### `kit/agents/vector-comment-evaluator.md`

Acción: NUEVO

Debe implementar:

- Tier **Sonnet**, **read-only** (Read, Grep, Glob, `Bash(git *)`). Reúne su propia evidencia: no
  recibe el diff pre-digerido, sino los inputs crudos (`COMMENT`, `WORKTREE`, `BRANCH`, `BASE`,
  `PR_URL`).
- Rubric de 6 preguntas (factualidad vs el código, accionabilidad, problema real vs bikeshedding,
  ya manejado, contradice convenciones, valor vs costo/riesgo).
- Salida estructurada: `VERDICT` (`VÁLIDO Y VALIOSO` / `VÁLIDO PERO MARGINAL` / `INVÁLIDO O SIN
  VALOR` + `N/10`), `EVIDENCE` (bullets `[SEVERITY] (confidence) file:line — …`), `AI-RED-FLAGS`,
  y `REMEDIATION` (si válido: archivos, enfoque, riesgo, tamaño).

No debe: editar archivos, postear, ni aceptar el framing del comentario sin verificarlo.

#### `cli/internal/scaffold/assets/commands/vector/comment.md` y `…/agents/vector-comment-evaluator.md`

Acción: NUEVO (generado)

- Se producen ejecutando `go generate ./internal/scaffold` desde `cli/`, que copia
  `kit/{commands,agents}` a `assets/` (`scaffold.go:13`). **No editar a mano**; regenerar.

#### `cli/internal/scaffold/scaffold_test.go`

Acción: MODIFICAR (solo si el test enumera el set de commands esperados)

- Si el test valida un set fijo de commands embebidos, añadir `comment.md` (y el agente) al
  esperado. Si solo valida "hay commands" + `raw.md`, no requiere cambios.

Restricciones:

- No cambiar el mecanismo de vendoring ni otros assets.
- No tocar código Go fuera del test del scaffold.

---

## 7. API Contract

> **No aplica como API HTTP nueva.** `/vector:comment` es un project command; no agrega endpoints.
> Su "contrato" es la **CLI** del binario, ya existente y estable:

- `vector spec list --json` → resolver el spec card asociado.
- `vector spec worklog <id> [--files a,b] [--tasks "..."] [--note "..."] [--json]` → registrar el
  trabajo derivado del comentario (evento `work.logged`, aditivo).
- `vector spec status <id> <status> [--reason]` → transición opcional `review → in-progress`.

No se inventan flags ni campos nuevos; se usan los existentes
(`cli/cmd/vector/standup.go:199`, `spec_transitions.go:143`).

### Endpoints involucrados

No aplica — esta fase no añade ni modifica endpoints HTTP (`cli/internal/board/server.go` intacto).

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `/vector:comment "<texto>" [{spec|branch}]` parsea el comentario y resuelve la rama sin asumir
      el layout de worktrees de somnio (pregunta si es ambigua).
- [ ] El usuario elige el origen del diff (`gh` o `git diff` local); sin `gh`, el flujo local
      funciona.
- [ ] Los comandos de verificación (`build`/`lint`/`test`) se detectan de `.vector/config.json` /
      manifests; si no, se piden al usuario (no se hardcodea `pnpm`).
- [ ] El agente **Sonnet** retorna un veredicto estructurado (categoría + `N/10` + evidencia
      `file:line` + AI-red-flags).
- [ ] El veredicto se reporta en el **idioma configurado** (`config.language`, fallback al idioma de la conversación); el main loop re-chequea la evidencia citada.
- [ ] Solo con veredicto **VÁLIDO Y VALIOSO + bajo riesgo** se ofrece implementar por defecto.
- [ ] Al implementar, corre la verificación y reporta el resultado real (no "debería funcionar").
- [ ] Con un `SPEC_ID` resuelto, implementar registra un `work.logged` vía `vector spec worklog`
      (verificable en `activity.jsonl`); sin spec, se omite sin error.
- [ ] Si no se implementa, se genera un reply humanizado (anti-IA) y se copia a clipboard; no se
      postea.
- [ ] El command y el agente quedan embebidos tras `go generate` y `vector init` los siembra en un
      repo nuevo.
- [ ] Sin regresiones: commands existentes del kit y subcomandos del binario intactos.

### Tests requeridos

> El command y el agente son Markdown (sin compilación). La validación es por consistencia y, en
> `cli/`, por el test del scaffold.

- [ ] `go -C cli test ./internal/scaffold/...` pasa: los nuevos assets están embebidos.
- [ ] `vector init` en un repo limpio escribe `.claude/commands/vector/comment.md` y
      `.claude/agents/vector-comment-evaluator.md`.
- [ ] Consistencia del kit: frontmatter válido, kebab-case, tier del agente declarado.

### Comandos de verificación

```bash
go -C cli generate ./internal/scaffold
go -C cli vet ./...
go -C cli test ./internal/scaffold/...   # solo el scaffold: no hay código Go nuevo que testear
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

> Feature CLI (project command). Las subsecciones de formularios/passwords no aplican (no hay UI
> de formularios).

### CLI (`/vector:comment`)

- Reporte claro y conciso en el **idioma configurado** (`config.language`, fallback al idioma de la
  conversación): rama/PR resuelto, veredicto + confianza, evidencia (2–4 bullets `file:line`),
  AI-red-flags (si las hay) y "qué haría falta" si es válido.
- Ambigüedad de rama/spec/origen-de-diff → `AskUserQuestion` con candidatos (más "Other"/"ninguno").
- Sin `gh` → fallback local explícito; nunca fallo silencioso.
- Verificación: progreso legible (`verificando build/lint/test…`) y resultado con exit code; si
  falla, mostrar la salida y **no** marcar como hecho.
- Reply: humanizado, grounded en evidencia, honesto ("válido pero no aporta aquí porque X"),
  copiado a clipboard e impreso para revisión.

### Loading / Errores / Navegación

- Loading: líneas de progreso por paso (resolviendo rama / obteniendo diff / evaluando /
  verificando). Errores: mensajes accionables (p. ej. `no se detectaron comandos de verificación;
  indícalos`). No deja la terminal en estado ambiguo.

### Accesibilidad

No aplica — salida de terminal; sin componentes visuales. El reporte usa texto estructurado, no
solo color, para el veredicto.

---

## 10. Decisiones tomadas

El agente no debe cuestionar ni cambiar estas decisiones (tomadas con el usuario):

- **Spec-aware**: al implementar, el command vincula el trabajo al spec card y registra
  `work.logged`; ofrece `review → in-progress` solo si el card está en `review`. *Por qué:* el
  usuario quiere que el trabajo derivado de comentarios aparezca en standup/timeline; reusar
  `vector spec worklog`/`status` mantiene CLI-owns-writes sin código Go nuevo ni duplicar `apply`.
- **Origen del diff = preguntar al usuario** (`gh` vs `git diff` local) en cada corrida. *Por qué:*
  Vector es agnóstico y no puede asumir GitHub; preguntar evita fallos silenciosos y cubre repos
  sin `gh`.
- **Detección de verificación + fallback a preguntar**. *Por qué:* respeta el agnosticism (no
  hardcodear `pnpm`); si la detección falla, pedir es mejor que adivinar mal.
- **Evaluador en tier Sonnet**. *Por qué:* el juicio escéptico (verificar claims contra el código,
  detectar AI-slop, sopesar valor/riesgo) es razonamiento real; token-routing autoriza el tier caro
  cuando aporta valor que el barato no da. La investigación barata queda en el main loop.
- **Port fiel y agnosticizado de `/pr-comment`** (no rediseño): mismos pasos conceptuales
  (evaluar→veredicto→acción), removiendo lo somnio-específico. *Por qué:* el usuario lo pidió "lo
  mismo que `/pr-comment`, como base".
- **Veredicto antes de implementar; nunca implementar sin VÁLIDO Y VALIOSO + bajo riesgo**. *Por
  qué:* es la esencia de `/pr-comment`; evita "arreglarlo por si acaso".
- **No postear el reply ni auto-commit**. *Por qué:* el usuario mantiene el control de lo que se
  publica y lo que se commitea (alineado a `apply.md` y al global del usuario).
- **Sin código Go nuevo ni eventos/endpoints nuevos**. *Por qué:* los subcomandos existentes
  (`worklog`/`status`/`list`) cubren la integración; agregar `comment.evaluated` o UI sería
  scope creep.

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación, no la
implementa.

---

## 11. Edge cases

### Datos inválidos

- Comentario vacío/solo-espacios → `AskUserQuestion` pidiéndolo; no continuar.
- `{spec|branch}` dado pero ambiguo → listar candidatos y preguntar; no adivinar.
- Comentario demasiado vago para evaluar → veredicto `INVÁLIDO O SIN VALOR` con la razón (no
  inventar un defecto).

### Resolución de rama / diff

- Cero candidatos de rama → preguntar (campo abierto). Varios → listar y preguntar.
- `gh` ausente o sin auth → ofrecer/usar `git diff <base>..HEAD` local; mensaje claro si el usuario
  eligió `gh` y no está disponible.
- `gh` instalado y autenticado pero `gh pr list` **falla con error de red o se cuelga / excede un
  timeout** → mostrar stderr/exit code, ofrecer fallback a `git diff <base>..HEAD` local; **no**
  silenciar ni quedarse colgado indefinidamente.
- Diff vacío (sin cambios) → reportar "no hay cambios que evaluar" y detenerse.

### Resolución del spec card

- Sin match en `vector spec list --json` → ofrecer "ninguno": el command evalúa/implementa pero
  **omite** `work.logged` (sin error).
- Varios candidatos → preguntar cuál vincular.

### Evaluación

- Comentario que referencia código/símbolos inexistentes en el diff → AI-red-flag de probable
  alucinación; baja la confianza.
- El veredicto del subagente contradice la evidencia citada → el main loop re-chequea `file:line` y
  **degrada** el veredicto, notando la discrepancia.
- El agente devuelve una salida **no parseable** (faltan `VERDICT`/`EVIDENCE`, respuesta vacía o
  truncada, campos mal formados) → el main loop trata el resultado como `INVÁLIDO O SIN VALOR`,
  reporta un error recuperable ("la evaluación no produjo un veredicto legible") y **no implementa**;
  ofrece reintentar la evaluación. No inventar un veredicto para rellenar.

### Implementación / verificación

- Verificación (`build`/`lint`/`test`) falla → mostrar la salida, **no** marcar como hecho, y
  preguntar cómo proceder. No registrar `work.logged` de un cambio que no verifica.
- Cambio válido pero grande/arquitectónico/ambiguo → presentar plan y confirmar antes de aplicar;
  no auto-implementar.
- `vector spec worklog`/`status` falla → reportar el error accionable; no silenciar.

### Concurrencia / estado

- La escritura de `work.logged`/transición la serializa el binario (mutex del `Store`); el command
  no escribe `.vector/` directamente.

---

## 12. Estados de UI requeridos

> CLI: los "estados" son secuencias de salida en terminal, no una UI con estados visuales.

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | invitación a invocar con el comentario | ejecutar `/vector:comment "<texto>"` |
| loading | progreso por paso (rama / diff / evaluando / verificando) | esperar |
| success | veredicto + evidencia; o cambio implementado + verificación ✓ (+ `work.logged`) | implementar / reply / cerrar |
| error | mensaje accionable (rama irresoluble, diff vacío, verificación ✗) | corregir / reintentar / abortar |
| pending-decision | `AskUserQuestion` (origen del diff, spec, acción a tomar) | elegir opción |

`empty`/`disabled`/`offline`: No aplica — herramienta CLI local, sin UI persistente ni modo
offline propio.

---

## 13. Validaciones

### Validaciones de cliente (command)

| Campo | Regla | Mensaje |
|---|---|---|
| `$ARGUMENTS` (comentario) | no vacío | `el comentario no puede estar vacío; usa /vector:comment "<texto>"` |
| `{spec|branch}` (opcional) | si se da, único/unambiguo en `git branch -a` o `vector spec list` | `"<arg>" es ambiguo; candidatos: …` |
| origen del diff | `gh` disponible+PR, o `git diff` local válido | `no se pudo obtener el diff; revisa gh auth o el estado de git` |
| comandos de verificación | detectados o provistos por el usuario | `no se detectaron comandos de verificación; indícalos (build/lint/test)` |

### Validaciones de servidor

No aplica — no hay backend remoto. Las validaciones de dominio (estado/transición) viven en
`cli/internal/state` y no cambian aquí; el command solo invoca subcomandos existentes.

---

## 14. Seguridad y permisos

- El agente Sonnet recibe el comentario + accede al diff/código del repo (read-only). No se le
  pasan secrets ni tokens; el diff puede contener código del usuario (normal).
- El reply generado no debe sugerir cambios que violen políticas del repo (p. ej. hardcodear
  secrets); el evaluador debe detectarlo y marcarlo.
- No imprimir ni registrar secrets/tokens/PII; `work.logged.note` es texto corto (≤280 chars,
  truncado por el binario), no vuelca diffs completos.
- La mutación de estado es local y serializada por el binario; sin auth (binario local), 401/403
  no aplican.

---

## 15. Observabilidad y logging

- Reusar el mecanismo existente: la actividad relevante (cambio implementado) deja rastro como
  `work.logged` en `activity.jsonl` vía el binario.
- El command reporta al usuario el veredicto, la verificación y la transición; no agrega logging
  propio fuera de eso.
- No registrar: secrets, tokens, PII, ni el diff completo en la nota del worklog.

---

## 16. i18n / textos visibles

Vector no tiene sistema i18n; los textos del command están en el Markdown. **Idioma del reporte:**
`config.language` por proyecto (en `.vector/config.json`), con fallback al idioma de la conversación
(auto-detección). Esto refleja la misma política que `vector-standup-writer`. La activación de
`config.language` en este command es responsabilidad del change `add-agent-prose-language`; hasta
que ese change aplique, el fallback al idioma de la conversación rige. El **reply del PR** se redacta
en **inglés** por defecto (artefacto de repo), salvo que el hilo esté en otro idioma.

| Key (texto del command) | Texto |
|---|---|
| comment.branch.ambiguous | `rama ambigua; elige una:` |
| comment.diff.source | `¿de dónde obtengo el diff?` |
| comment.diff.empty | `no hay cambios que evaluar` |
| comment.spec.none | `sin spec asociado (no se registrará work.logged)` |
| comment.verify.checking | `verificando…` |
| comment.verdict.title | `Veredicto:` |
| comment.action.ask | `¿qué hago con este comentario?` |
| comment.reply.copied | `reply copiado al portapapeles` |

---

## 17. Performance

- Resolución de rama/spec: operaciones locales rápidas (`git`, `vector spec list`).
- Traer el diff (gh) o computarlo local: O(n) en el tamaño del diff; aceptable.
- Verificación (`build`/`lint`/`test`): depende del repo (puede tardar); reportar progreso, no
  bloquear sin feedback.
- La evaluación Sonnet corre **una vez** por invocación; la orquestación barata no consume tier
  caro (token-routing).

---

## 18. Restricciones

El agente no debe:

- Hardcodear stack/VCS/comandos del repo del usuario; si no detecta, preguntar.
- Editar `.vector/` a mano (CLI-owns-writes); toda mutación vía `vector spec …`.
- Agregar código Go, eventos de estado (`comment.evaluated`) ni endpoints/UI nuevos.
- Postear el reply en el PR/ticket ni auto-commit/push.
- Implementar un comentario sin veredicto `VÁLIDO Y VALIOSO` + bajo riesgo.
- Portar la lógica somnio-específica (pnpm, graphify-out, máx-5-files, layout de worktrees fijo).
- Refactorizar código no relacionado ni cambiar otros assets del scaffold.
- Ignorar fallos de verificación o de los subcomandos del binario.

---

## 19. Entregables

Al finalizar deben quedar:

- [ ] `kit/commands/vector/comment.md` (project command, agnosticizado).
- [ ] `kit/agents/vector-comment-evaluator.md` (agente Sonnet, read-only).
- [ ] Copias embebidas en `cli/internal/scaffold/assets/…` vía `go generate`.
- [ ] `cli/internal/scaffold/scaffold_test.go` actualizado si enumera el set de commands.
- [ ] `go -C cli vet ./...` y `go -C cli test ./internal/scaffold/...` en verde.
- [ ] `vector init` en repo limpio siembra command + agente.
- [ ] Docs: actualizar el índice de commands del kit donde se listen (p. ej.
      `docs/plugin-and-commands.md`) si enumera los `/vector:*`.

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `kit/commands/vector/apply.md` (worklog/transición), `link.md` (resolución de card) y
      `kit/agents/vector-spec-validator.md` (patrón de agente).
- [ ] Confirmé que `vector spec worklog`/`status`/`list` existen y los reuso (sin código Go nuevo).
- [ ] El command es agnóstico: no hardcodea stack/VCS/comandos; pregunta cuando falta.
- [ ] El agente evaluador es Sonnet y read-only; reúne su propia evidencia.
- [ ] Mantuve CLI-owns-writes; el `work.logged` es aditivo y solo si hay spec.
- [ ] Implementé la resolución de rama/diff/spec con `AskUserQuestion` ante ambigüedad.
- [ ] El reply se humaniza (anti-IA) y no se postea; no hay auto-commit.
- [ ] Corrí `go generate` + `go vet` + el test del scaffold.
- [ ] Verifiqué que `vector init` siembra los nuevos assets.
- [ ] No agregué dependencias, eventos, endpoints ni UI.
- [ ] No dejé TODOs sin justificar.

---

## Open questions

- ¿La detección de comandos de verificación debe persistirse en `.vector/config.json` para no
  re-preguntar (cachear `build`/`lint`/`test` por repo), o preguntar cada vez que falte? (Sugerido:
  cachear si el usuario lo confirma; fuera de V1 si añade complejidad.)
- ¿Política de transición exacta tras implementar? V1: `work.logged` siempre (con spec) + ofrecer
  `review → in-progress` solo si el card está en `review`. Confirmar si se desea también un avance
  automático a `needs-attention` en algún caso.
- ¿El `scaffold_test.go` enumera el set de commands (requiere editar) o solo valida presencia?
  Verificar al implementar.
