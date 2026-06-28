# Spec: Comando `/vector:research` para investigar viabilidad de ideas

## 1. Objetivo

Construir `/vector:research`: un **project command** del kit que toma una idea cruda (`[text]`),
la **investiga de forma multidisciplinar** (revisiones de viabilidad técnica, seguridad,
marketing y diseño/UX, eligiendo solo las lentes que apliquen), refina la idea con el usuario,
y —tras un **gate explícito de go/no-go**— **emite él mismo un spec completo de 20 secciones**
con el **reporte de viabilidad embebido**, registrándolo como card en estado `draft`.

Esta feature permite que un dev **valide la viabilidad de un feature antes de comprometerse a
especificarlo**: obtener veredictos de varias disciplinas (¿es técnicamente factible?, ¿hay
riesgos de seguridad?, ¿tiene sentido comercial?, ¿requiere diseño?), decidir con esa evidencia
si seguir, y —si decide seguir— quedarse con un spec ya enriquecido por esa investigación. Es
el "hermano exhaustivo" de `/vector:raw`: donde `raw` refina-y-emite, `research`
investiga-evalúa-decide-y-emite.

## 2. Alcance

### Incluido en esta fase

- **Project command `/vector:research`** (`kit/commands/vector/research.md`): orquesta lectura de
  la idea, detección de lentes aplicables, refinamiento con el usuario, las revisiones de
  viabilidad delegadas, la consolidación del veredicto, el **gate go/no-go**, la composición del
  spec de 20 secciones con el reporte embebido, y el registro de la card `draft` vía el binario.
- **Agente revisor de viabilidad `vector-feasibility-reviewer`**
  (`kit/agents/vector-feasibility-reviewer.md`), tier **Sonnet**, **read-only**: parametrizado por
  **lente** (`technical` | `security` | `marketing` | `design`); reúne su propia evidencia
  (Read/Grep/Glob) y emite un veredicto estructurado por lente (`go` / `go-with-risks` / `no-go` +
  confianza `N/10` + hallazgos + riesgos + recomendación).
- **Detección de lentes (auto-detect, en el main loop, barata)**: a partir del texto se eligen las
  lentes a correr. La lente **`technical` (viabilidad técnica) es el núcleo mínimo y corre
  siempre**; `security` / `marketing` / `design` se activan por señales del texto. Si la detección
  es ambigua, se ofrece al usuario ajustar el set vía `AskUserQuestion` (sin forzar).
- **Reutilización del pipeline de `/vector:raw`** (self-contained): el refinamiento usa el agente
  existente `vector-spec-refiner` (**Haiku**) y la validación final usa `vector-spec-validator`
  (**Sonnet**), igual que `raw`. `research` **no** duplica esa lógica ni invoca a `/vector:raw`
  como comando externo: la compone en su propio flujo añadiendo la capa de viabilidad.
- **Gate explícito go/no-go**: tras consolidar los veredictos, el command pregunta al usuario si
  procede a emitir el spec (recomendación derivada del veredicto consolidado, pero **el humano
  decide**). Si el usuario aborta, no se crea card.
- **Spec de 20 secciones + reporte de viabilidad embebido**: el documento usa la plantilla
  canónica (`.claude/vector/spec-template.md`) y **anexa**, después de la §20 (junto a `Open
  questions`, como ya hace el repo), una sección **`## Reporte de viabilidad`** con la tabla de
  veredictos por lente, hallazgos y riesgos.
- **Registro de la card `draft`** reutilizando el subcomando existente
  `vector spec create --status draft --body-file - --json` (el body es el markdown completo,
  reporte incluido). **Sin código Go nuevo.**
- **Contabilidad de tokens (Token Savings Meter)**: registrar un evento `agent.routed` por cada
  paso de agente barato/medio ejecutado, vía el subcomando existente `vector spec route`.
- **Vendoring**: incluir el command y el agente en los assets embebidos del binario
  (`go generate` en `cli/internal/scaffold`).

### Fuera de scope

- **Nuevo código Go, nuevos subcomandos, nuevos eventos de estado ni endpoints/UI nuevos**: se
  reutilizan `vector spec create` (existente) y `vector spec route` (existente). No se crea
  `spec.researched` ni panel de viabilidad en `web/`.
- **Investigación web / fuentes externas** (búsquedas en internet, fetch de competidores, llamadas
  a APIs externas): la investigación es **sobre la idea y el repo**, no un agente de research web.
  (Posible extensión futura; ver Open questions.)
- **Crear el OpenSpec change** (proposal/design/tasks): igual que `raw`, `research` deja la card en
  `draft`; el change se crea luego en `/vector:propose`.
- **Implementar el feature investigado**: `research` nunca escribe código del feature; solo emite
  el spec. La implementación empieza en `/vector:apply`.
- **Lentes adicionales más allá de las cuatro** (`technical`/`security`/`marketing`/`design`):
  legal, accesibilidad-como-lente-separada, performance-como-lente-separada, etc. quedan fuera de
  V1 (la accesibilidad y la performance se cubren dentro de `design` y `technical`
  respectivamente).
- **Persistir el detalle de la detección de lentes en `.vector/config.json`** (cachear qué lentes
  por tipo de idea): fuera de V1.
- **Auto-commit / push** de cualquier artefacto: `research` solo escribe vía el binario (la card y
  el spec doc); no toca git.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- **Project command**: Markdown + frontmatter orquestado por Claude (patrón
  `kit/commands/vector/raw.md`, `comment.md`).
- **Agente revisor**: Markdown del kit (`kit/agents/vector-feasibility-reviewer.md`), tier
  **Sonnet** (token-routing: el juicio de viabilidad es razonamiento real). Patrón de agente:
  `kit/agents/vector-comment-evaluator.md`, `vector-spec-validator.md`.
- **Agentes reutilizados**: `vector-spec-refiner` (**Haiku**, refinamiento) y
  `vector-spec-validator` (**Sonnet**, validación), ya existentes en `kit/agents/`.
- **Binario**: se invoca como cliente vía los subcomandos existentes
  `vector spec create` (`cli/cmd/vector/main.go:568`, `:709`) y
  `vector spec route` (`cli/cmd/vector/main.go:592`). **No** se añade código Go.
- **Detección de lentes / idioma**: lectura de `.vector/config.json` (`config.language`,
  `cli/internal/config/config.go:72`) y del texto de la idea; sin hardcodear stack del usuario.

### Versiones relevantes

- Go: `1.26` (de `cli/go.mod`; no se toca porque no se añade código Go).
- No se introducen dependencias nuevas.

### Patrones existentes a respetar

- **CLI-owns-writes**: el command **nunca** edita `.vector/` a mano; la card y el spec doc se
  crean vía `vector spec create` (`workflows/state-sync-discipline.md`).
- **Self-contained spec authoring**: `raw` ya autora el spec con refiner (Haiku) + validator
  (Sonnet) y registra `draft`; `research` reutiliza ese patrón sin duplicarlo
  (`kit/commands/vector/raw.md`).
- **Token routing** (`product/token-routing.md`): detección de lentes y orquestación baratas en el
  main loop; refinamiento en **Haiku**; revisiones de viabilidad y validación en **Sonnet**.
  Documentar en el command por qué cada paso usa su tier, y registrar `agent.routed` por paso.
- **Agnosticism** (`product/principles.md`): las lentes evalúan la idea y el repo del usuario sin
  asumir techstack; si la detección de lentes es ambigua, preguntar.
- **Reporte en el idioma configurado** (`config.language`, fallback al idioma de la conversación),
  igual que `vector-standup-writer` / `vector-comment-evaluator`. Slugs/rutas/artefactos de git en
  inglés kebab-case.
- **Plantilla canónica de spec**: 20 secciones obligatorias en orden, anexos (`Open questions`,
  `Reporte de viabilidad`) después de la §20 (`.claude/vector/spec-template.md`; patrón de anexo
  visto en `add-comment-evaluator-command/spec.md`).
- **Vendoring**: command y agente se copian a `cli/internal/scaffold/assets/` vía
  `//go:generate` (`cli/internal/scaffold/scaffold.go:13`) y se embeben con `//go:embed all:assets`
  (`scaffold.go:26`).

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Subcomando `vector spec create --title --id --status draft --body-file - --json [--ticket]
      [--priority]` — verificado (`cli/cmd/vector/main.go:568`, `:709`, usage `:948`). `runSpecCreate`
      expone además `--repo`/`--related` (no usados por este command; omitidos a propósito).
- [x] Subcomando `vector spec route <id> --model --baseline --task --tokens-in --tokens-out` —
      verificado (`cli/cmd/vector/main.go:592`).
- [x] Agentes `vector-spec-refiner` (Haiku) y `vector-spec-validator` (Sonnet) —
      verificados (`kit/agents/`).
- [x] Plantilla de spec `.claude/vector/spec-template.md` — verificada.
- [x] Mecanismo de scaffold/vendoring (`//go:generate` + `//go:embed` en
      `cli/internal/scaffold/scaffold.go:13,26`) — verificado.
- [x] `config.language` en el config (`cli/internal/config/config.go:72`, `ResolvedLanguage`
      `:116`) — verificado.
- [x] Patrón de project command (`kit/commands/vector/raw.md`, `comment.md`) y de agente
      (`kit/agents/vector-comment-evaluator.md`) — verificados.
- [x] `.vector/config.json` presente (`vector init` corrido) — verificado en este repo.

Si alguna dependencia no existe, el command se detiene con un mensaje accionable. No inventa
contratos, rutas ni subcomandos.

---

## 5. Arquitectura

### Patrón a usar

**Orquestación por project command + detección barata de lentes + revisiones de viabilidad
delegadas a un agente Sonnet parametrizado por lente + reutilización del pipeline de autoría de
`raw` (refiner Haiku + validator Sonnet) + gate humano + escritura de estado vía el binario.** El
command coordina; cada lente juzga de forma independiente; el binario es el único escritor. Misma
separación que `/vector:comment` (juicio en subagente Sonnet, acción en el main loop) y
`/vector:raw` (autoría self-contained con refiner+validator, registro `draft`).

### Capas afectadas

- **Kit command** (`kit/commands/vector/research.md`): sí — NUEVO. Orquestación completa.
- **Kit agente** (`kit/agents/vector-feasibility-reviewer.md`): sí — NUEVO. Revisor Sonnet por
  lente.
- **Scaffold assets** (`cli/internal/scaffold/assets/…`): sí — generados por `go generate` (copia
  embebida; no edición manual).
- **CLI (código Go)** (`cli/cmd`, `cli/internal/state`, `cli/internal/board`): **no** — se reusan
  `vector spec create` y `vector spec route`; no se añade código.
- **Web** (`web/`): **no** — el reporte va embebido en el spec doc; el card `draft` ya se renderiza
  como cualquier otro.

### Flujo esperado

1. Usuario ejecuta `/vector:research "<idea>"` (si vacío, usa el último mensaje).
2. **Confirmar repo inicializado** y resolver `specPath`/`config.language` de `.vector/config.json`
   (si falta, correr/avisar `vector init`, igual que `raw`).
3. **Detectar lentes** (main loop, barato): `technical` siempre; `security`/`marketing`/`design`
   por señales del texto (ver §13). Mostrar el set elegido; si es ambiguo, `AskUserQuestion` para
   ajustarlo. Mantener `LENSES`.
4. **Refinar** (Haiku): invocar `vector-spec-refiner` con la idea, un spec de ejemplo y la
   plantilla → brief con la ambigüedad por dimensión (`BRIEF`). Registrar `agent.routed`.
5. **Clarificar** con el usuario las dimensiones abiertas del brief (batches ≤5 vía
   `AskUserQuestion`), hasta que la idea quede lista **sin preguntas pendientes** (o el usuario
   marque `TBD`).
6. **Revisar viabilidad** (Sonnet, una invocación por lente en `LENSES`): invocar
   `vector-feasibility-reviewer` con `LENS`, la idea refinada y el contexto del repo. Cada lente
   reúne su evidencia y retorna su veredicto estructurado. Registrar un `agent.routed` por lente.
   (Las lentes pueden correr en paralelo.)
7. **Re-chequear** (main loop): no confiar ciegamente; validar que la evidencia citada por cada
   lente se sostiene; si no, degradar ese veredicto y notarlo.
8. **Consolidar el veredicto**: combinar las lentes en un veredicto global —`no-go` si alguna lente
   crítica es `no-go`; `go-with-risks` si hay riesgos; `go` si todas pasan— con un resumen por
   lente.
9. **Gate go/no-go** (`AskUserQuestion`): presentar el veredicto consolidado y preguntar si procede
   a emitir el spec (recomendación según el veredicto, pero el usuario decide). Opciones: emitir /
   refinar más / abortar. Si aborta → terminar **sin** crear card.
10. **Componer el spec**: 20 secciones de la plantilla (cada `[...]` reemplazado por contenido
    verificado) + anexo `## Reporte de viabilidad` (tabla por lente + hallazgos + riesgos) + anexo
    `## Open questions`. Derivar `title` (≤ ~8 palabras) y `id` kebab-case. Detectar ticket en la
    idea (misma lógica que `raw`); priority solo si la idea lo implica.
11. **Validar** (Sonnet): invocar `vector-spec-validator` con el spec compuesto, el ejemplo, la
    plantilla y el checklist de 20 secciones. Reaccionar al verdict (PASS / WARN / BLOCK, máx. 3
    ciclos). Registrar `agent.routed`.
12. **Registrar la card `draft`**: `vector spec create --title --id [--priority] [--ticket]
    --status draft --body-file - --json <<<spec`. Parsear `id`/`status`/`specDoc`. Nunca bloquear
    la creación por el ticket (si lo rechaza, reintentar sin `--ticket`).
13. **Reportar**: id, `status: draft`, `specDoc`, veredicto consolidado de viabilidad, y el
    siguiente paso (`/vector:propose`).

### Ubicación de archivos nuevos

```txt
kit/commands/vector/research.md                              # project command
kit/agents/vector-feasibility-reviewer.md                    # agente revisor (Sonnet, por lente)
cli/internal/scaffold/assets/commands/vector/research.md     # copia embebida (generada)
cli/internal/scaffold/assets/agents/vector-feasibility-reviewer.md  # copia embebida (generada)
```

No crear carpetas nuevas: ya existe la convención (`kit/commands/vector/`, `kit/agents/`).

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/commands/vector/research.md` | NUEVO | Project command: detección de lentes, refinamiento, revisiones de viabilidad, consolidación, gate, composición del spec + reporte embebido, registro `draft`, routing | `kit/commands/vector/raw.md`, `comment.md` |
| `kit/agents/vector-feasibility-reviewer.md` | NUEVO | Agente Sonnet read-only parametrizado por lente: evidencia propia + veredicto estructurado | `kit/agents/vector-comment-evaluator.md`, `vector-spec-validator.md` |
| `cli/internal/scaffold/assets/commands/vector/research.md` | NUEVO (generado) | Copia embebida del command vía `//go:generate` (`scaffold.go:13`) | siblings `raw.md`, `comment.md` en `assets/` |
| `cli/internal/scaffold/assets/agents/vector-feasibility-reviewer.md` | NUEVO (generado) | Copia embebida del agente vía `//go:generate` | siblings `vector-comment-evaluator.md` en `assets/agents/` |
| `cli/internal/scaffold/scaffold_test.go` | MODIFICAR | El test ya enumera pares específicos (p. ej. `bug.md` + `vector-bug-refiner.md` vía `TestSeedCommandsSeedsBugCommandAndRefiner`); añadir un test análogo para `research.md` + `vector-feasibility-reviewer.md` | `cli/internal/scaffold/scaffold_test.go` (`TestSeedCommandsSeedsBugCommandAndRefiner`) |
| `docs/plugin-and-commands.md` | MODIFICAR (si enumera los `/vector:*`) | Listar `/vector:research` | sección de commands del kit |

### Detalle por archivo

#### `kit/commands/vector/research.md`

Acción: NUEVO

Debe implementar (frontmatter + cuerpo en pasos, según §5):

- **Frontmatter**: `name: research`, `description`, `argument-hint: "[idea-text]"`,
  `user-invocable: true`, `allowed-tools` (Read, Grep, Glob, `Bash(vector *)`, Agent,
  AskUserQuestion).
- **Pasos**: confirmar repo init → detectar lentes (ask si ambiguo) → refinar (Haiku) → clarificar
  → revisar viabilidad por lente (Sonnet, paralelo) → re-chequear evidencia → consolidar veredicto
  → **gate go/no-go** (ask) → componer spec + reporte embebido → validar (Sonnet) → `vector spec
  create --status draft` → reportar + routing.
- **Token routing**: documentar por qué detección/orquestación van al main loop, refinamiento a
  Haiku, revisiones+validación a Sonnet; registrar `agent.routed` por paso de agente.
- **Recordatorio de disciplina de estado**: la card y el spec doc se crean **solo** vía el binario;
  nunca editar `.vector/` a mano.

No debe incluir: investigación web/fuentes externas, creación del OpenSpec change, implementación
del feature, ni código Go.

#### `kit/agents/vector-feasibility-reviewer.md`

Acción: NUEVO

Debe implementar:

- Tier **Sonnet**, **read-only** (Read, Grep, Glob). Recibe la **lente** (`technical` | `security`
  | `marketing` | `design`), la idea refinada y el contexto del repo; reúne su propia evidencia.
- **Rubric por lente** (qué pregunta cada una):
  - `technical`: factibilidad con el stack/arquitectura del repo, esfuerzo aproximado, dependencias
    faltantes, riesgos de integración.
  - `security`: superficie de ataque, manejo de datos/PII/secrets, permisos, operaciones
    destructivas sobre el repo del usuario (`security/destructive-ops-consent.md`).
  - `marketing`: encaje con el producto y la propuesta de valor, diferenciación, sentido comercial
    (`product/principles.md` — comercial desde día 0), público objetivo.
  - `design`: necesidad y complejidad de UI/UX, impacto en el board/panel web, accesibilidad.
- **Salida estructurada** por lente: `LENS`, `VERDICT` (`go` / `go-with-risks` / `no-go`) +
  confianza `N/10`, `FINDINGS` (bullets con evidencia `file:line` cuando aplique), `RISKS`,
  `RECOMMENDATION`.

No debe: editar archivos, implementar, ni aceptar el framing de la idea sin evaluarlo.

#### `cli/internal/scaffold/assets/commands/vector/research.md` y `…/agents/vector-feasibility-reviewer.md`

Acción: NUEVO (generado)

- Se producen ejecutando `go generate ./internal/scaffold` desde `cli/`, que copia
  `kit/{commands,agents}` a `assets/` (`scaffold.go:13`). **No editar a mano**; regenerar.

#### `cli/internal/scaffold/scaffold_test.go`

Acción: MODIFICAR (solo si el test enumera el set de commands/agentes esperados)

- Si valida un set fijo, añadir `research.md` y `vector-feasibility-reviewer.md`. Si solo valida
  presencia + `raw.md`, no requiere cambios. Verificar al implementar.

#### `docs/plugin-and-commands.md`

Acción: MODIFICAR (solo si enumera los `/vector:*`)

- Añadir `/vector:research` a la lista de commands del kit con una línea de descripción.

Restricciones:

- No cambiar el mecanismo de vendoring ni otros assets.
- No tocar código Go fuera del test del scaffold.

---

## 7. API Contract

> **No aplica como API HTTP nueva.** `/vector:research` es un project command; no agrega
> endpoints. Su "contrato" es la **CLI** del binario, ya existente y estable:

- `vector spec create --title "..." --id slug [--repo name] [--priority p] [--ticket '{...}']
  --status draft --body-file - --json` → crea la card `draft` y escribe el spec doc.
- `vector spec route <id> --model m --baseline opus --task "..." --tokens-in N --tokens-out M`
  → registra el evento `agent.routed` (Token Savings Meter).

No se inventan flags ni campos nuevos; se usan los existentes
(`cli/cmd/vector/main.go:568`, `:592`, `:709`, usage `:948`).

### Endpoints involucrados

No aplica — esta fase no añade ni modifica endpoints HTTP (`cli/internal/board/server.go` intacto).

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `/vector:research "<idea>"` corre, detecta las lentes aplicables (con `technical` siempre) y
      muestra el set; ante ambigüedad pregunta sin forzar.
- [ ] El refinamiento usa `vector-spec-refiner` (**Haiku**) y la idea se clarifica con el usuario
      hasta quedar sin preguntas pendientes.
- [ ] Cada lente seleccionada produce un veredicto estructurado vía `vector-feasibility-reviewer`
      (**Sonnet**): `go`/`go-with-risks`/`no-go` + `N/10` + hallazgos + riesgos + recomendación.
- [ ] El main loop re-chequea la evidencia citada y consolida un veredicto global coherente.
- [ ] Hay un **gate go/no-go** explícito: si el usuario aborta, **no** se crea card.
- [ ] El spec emitido tiene las **20 secciones** de la plantilla **+** el anexo
      `## Reporte de viabilidad` con la tabla por lente, y pasa `vector-spec-validator` (PASS o
      WARN aceptado por el usuario).
- [ ] La card queda en `draft` vía `vector spec create` (verificable: `state.json` + `specDoc`
      escrito), nunca por edición manual de `.vector/`.
- [ ] Se registra un `agent.routed` por cada paso de agente ejecutado (refiner + N lentes +
      validator), verificable en `activity.jsonl`.
- [ ] El reporte al usuario usa el **idioma configurado** (`config.language`, fallback a la
      conversación).
- [ ] El command y el agente quedan embebidos tras `go generate` y `vector init` los siembra en un
      repo nuevo.
- [ ] Sin regresiones: commands existentes del kit y subcomandos del binario intactos.

### Tests requeridos

> El command y el agente son Markdown (sin compilación). La validación es por consistencia y, en
> `cli/`, por el test del scaffold.

- [ ] `go -C cli test ./internal/scaffold/...` pasa: los nuevos assets están embebidos.
- [ ] `vector init` en un repo limpio escribe `.claude/commands/vector/research.md` y
      `.claude/agents/vector-feasibility-reviewer.md`.
- [ ] Consistencia del kit: frontmatter válido, kebab-case, tier del agente declarado.

### Comandos de verificación

```bash
go -C cli generate ./internal/scaffold
go -C cli vet ./...
go -C cli test ./internal/scaffold/...
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

> Feature CLI (project command). Las subsecciones de formularios/passwords no aplican (no hay UI
> de formularios).

### CLI (`/vector:research`)

- Progreso legible por fase: lentes detectadas → refinando → revisando (por lente) → consolidando
  → gate → componiendo → validando → registrando.
- Ambigüedad (lentes a correr, dimensiones del brief, gate go/no-go) → `AskUserQuestion` con
  opciones claras (más "Other").
- El veredicto consolidado se presenta antes del gate: resumen por lente (verdict + 1 línea) y
  veredicto global, en el idioma configurado.
- Si el usuario aborta en el gate: mensaje claro de que no se creó card y cómo retomar.
- Al final: id, `status: draft`, `specDoc`, veredicto de viabilidad y siguiente paso
  (`/vector:propose`).

### Loading / Errores / Navegación

- Loading: líneas de progreso por paso. Errores: accionables (p. ej. `repo no inicializado; corre
  vector init`). No deja la terminal en estado ambiguo.

### Accesibilidad

No aplica — salida de terminal; sin componentes visuales. El veredicto usa texto estructurado, no
solo color.

---

## 10. Decisiones tomadas

El agente no debe cuestionar ni cambiar estas decisiones (tomadas con el usuario):

- **Nombre del command: `/vector:research`** (un verbo, kebab-case, alineado al resto del kit).
  *Por qué:* el usuario lo eligió sobre `vet`/`assess`/`investigate` por claridad.
- **Self-contained**: `research` autora el spec él mismo reutilizando el pipeline de `raw`
  (refiner Haiku + validator Sonnet), añadiendo la capa de viabilidad. **No** invoca `/vector:raw`
  como comando externo ni produce solo un reporte. *Por qué:* un solo comando, la investigación
  alimenta el spec directo.
- **Auto-detección de lentes con núcleo mínimo**: `technical` siempre; `security`/`marketing`/
  `design` por señales del texto; ajustable por el usuario si es ambiguo. *Por qué:* token-routing
  — no gastar Sonnet en lentes irrelevantes.
- **Card `draft` + reporte de viabilidad embebido** como anexo del spec (después de la §20). *Por
  qué:* un solo artefacto consistente con que `raw` crea `draft`; el reporte viaja con el spec.
- **Gate explícito go/no-go**: el humano decide si emitir el spec tras ver el veredicto. *Por qué:*
  el usuario quiere control; investigar viabilidad implica poder decir "no va".
- **Revisores en tier Sonnet; refinamiento en Haiku; detección/orquestación en el main loop**. *Por
  qué:* el juicio de viabilidad es razonamiento real (token-routing autoriza el tier caro cuando
  aporta valor); el refinamiento y la investigación barata no.
- **Un agente revisor parametrizado por lente** (no un agente por disciplina). *Por qué:* mantiene
  el kit pequeño y el vendoring simple; cada invocación trae su rubric de lente. (Alternativa
  —agentes especializados por disciplina— en Open questions.)
- **Sin código Go nuevo ni eventos/endpoints nuevos**: reutiliza `vector spec create` y
  `vector spec route`. *Por qué:* esos subcomandos cubren el registro y el meter; agregar
  `spec.researched` o UI sería scope creep.

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación, no la
implementa.

---

## 11. Edge cases

### Datos inválidos

- Idea vacía → usar el último mensaje del usuario; si tampoco hay, `AskUserQuestion` pidiéndola.
- Idea demasiado vaga para revisar → el refinamiento debe clarificarla antes de las lentes; si tras
  clarificar sigue vaga, marcar dimensiones como `TBD — ver Open questions`, no inventar.

### Detección de lentes

- Señales contradictorias/ausentes → correr solo `technical` y ofrecer `AskUserQuestion` para
  añadir lentes. Nunca correr las cuatro "por si acaso" (gasto de tokens).

### Revisión de viabilidad

- Una lente referencia código/símbolos inexistentes → el main loop re-chequea, baja la confianza y
  lo nota.
- Una lente devuelve salida **no parseable** (faltan `VERDICT`/`FINDINGS`) → tratarla como
  `go-with-risks` con nota "lente no concluyente" y ofrecer reintentar esa lente; no inventar un
  veredicto.
- Veredicto consolidado `no-go` → el gate lo refleja y recomienda **no** emitir; el usuario aún
  puede forzar emitir (queda registrado en el reporte embebido).

### Gate / emisión

- Usuario aborta en el gate → terminar sin crear card ni escribir spec doc; confirmar al usuario.
- Validación `BLOCK` tras 3 ciclos → no registrar; surfacer el reporte del validator y detenerse.
- `vector spec create` rechaza el `--ticket` (JSON malformado / provider no inferible) → reintentar
  **sin** `--ticket` y sugerir `/vector:link`.

### Estado / concurrencia

- La escritura de la card/spec doc/route la serializa el binario (mutex del `Store`); el command no
  escribe `.vector/` directamente.

---

## 12. Estados de UI requeridos

> CLI: los "estados" son secuencias de salida en terminal, no una UI con estados visuales.

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | invitación a invocar con la idea | ejecutar `/vector:research "<idea>"` |
| loading | progreso por paso (lentes / refinando / revisando / consolidando / validando / registrando) | esperar |
| pending-decision | `AskUserQuestion` (lentes, dimensiones, gate go/no-go) | elegir opción |
| success | veredicto consolidado + card `draft` creada (id + `specDoc`) | correr `/vector:propose` |
| error | mensaje accionable (repo no init, validación BLOCK, idea irresoluble) | corregir / reintentar / abortar |
| aborted | confirmación de que no se creó card | retomar `/vector:research` cuando quiera |

`empty`/`disabled`/`offline`: No aplica — herramienta CLI local, sin UI persistente ni modo
offline propio.

---

## 13. Validaciones

### Validaciones de cliente (command)

| Campo | Regla | Mensaje |
|---|---|---|
| `$ARGUMENTS` (idea) | no vacío (o último mensaje) | `dame la idea: /vector:research "<texto>"` |
| `LENSES` | `technical` siempre presente; resto ∈ {security, marketing, design} | `lentes seleccionadas: …` |
| veredicto consolidado | derivado de las lentes (no-go si crítica no-go) | `viabilidad: <go|go-with-risks|no-go>` |
| spec compuesto | 20 secciones + anexo `Reporte de viabilidad`; valida con Sonnet | `validación: PASS|WARN|BLOCK` |

### Señales de detección de lentes (heurística del main loop)

| Lente | Se activa cuando la idea menciona / implica |
|---|---|
| `technical` | siempre (núcleo) |
| `security` | auth, datos/PII, secrets, permisos, input externo, escritura/movimiento de archivos del repo del usuario, operaciones destructivas |
| `marketing` | feature de cara al usuario, pricing, growth, onboarding, posicionamiento, valor comercial |
| `design` | UI, board, panel web, visual, interacción, accesibilidad |

### Validaciones de servidor

No aplica — no hay backend remoto. Las validaciones de dominio (estado/transición, creación de
card) viven en `cli/internal/state` y no cambian aquí; el command solo invoca subcomandos
existentes.

---

## 14. Seguridad y permisos

- Los agentes (refiner Haiku, revisores Sonnet, validator Sonnet) son **read-only**; no editan
  archivos ni ejecutan operaciones destructivas.
- La lente `security` evalúa explícitamente riesgos de la idea (incl. operaciones destructivas
  sobre el repo del usuario, `security/destructive-ops-consent.md`), pero **no** ejecuta nada: solo
  reporta.
- No exponer ni registrar secrets/tokens/PII; el reporte embebido es texto de evaluación, no vuelca
  payloads sensibles.
- La escritura de estado es local y serializada por el binario; sin auth (binario local), 401/403
  no aplican.

---

## 15. Observabilidad y logging

- Reusar el mecanismo existente: cada paso de agente deja un `agent.routed` en `activity.jsonl` vía
  `vector spec route` (alimenta el Token Savings Meter); la creación de la card deja su evento de
  estado vía `vector spec create`.
- El command reporta al usuario las lentes, el veredicto y el resultado del registro; no agrega
  logging propio fuera de eso.
- No registrar: secrets, tokens, PII, ni el texto completo del repo en las notas de routing.

---

## 16. i18n / textos visibles

Vector no tiene sistema i18n; los textos del command están en el Markdown. **Idioma del reporte y
del veredicto:** `config.language` por proyecto (`.vector/config.json`), con fallback al idioma de
la conversación, igual que `vector-standup-writer` / `vector-comment-evaluator`. El **spec doc** se
escribe en el idioma de los specs del repo (detectado del ejemplo; en este repo, **español**).
Slugs/rutas/artefactos de git en inglés kebab-case.

| Key (texto del command) | Texto |
|---|---|
| research.lenses.detected | `lentes a investigar:` |
| research.lenses.ambiguous | `no está claro qué lentes aplican; ¿cuáles corro?` |
| research.review.running | `revisando viabilidad (<lente>)…` |
| research.verdict.title | `Veredicto de viabilidad:` |
| research.gate.ask | `¿procedo a emitir el spec?` |
| research.aborted | `no se creó card; puedes retomar cuando quieras` |
| research.created | `card creada en draft:` |

---

## 17. Performance

- Detección de lentes y orquestación: operaciones locales rápidas en el main loop.
- Revisiones de viabilidad: una invocación Sonnet **por lente seleccionada**; correrlas en paralelo
  reduce la latencia. Solo corren las lentes detectadas (no las cuatro siempre) — token-routing.
- Refinamiento (Haiku) y validación (Sonnet) corren una vez cada uno, como en `raw`.
- No re-leer/re-escribir todo el estado: la card se crea una vez; los `route` son aditivos
  (`workflows/state-sync-discipline.md`).

---

## 18. Restricciones

El agente no debe:

- Hardcodear stack/VCS/comandos del repo del usuario; si la detección de lentes falla, preguntar.
- Editar `.vector/` a mano (CLI-owns-writes); la card/spec doc/route van vía `vector spec …`.
- Agregar código Go, eventos de estado (`spec.researched`) ni endpoints/UI nuevos.
- Invocar `/vector:raw` como comando externo (se reutiliza el patrón, no el command).
- Hacer investigación web / llamadas a fuentes externas (fuera de scope V1).
- Crear el OpenSpec change ni implementar el feature investigado.
- Correr las cuatro lentes "por si acaso" cuando solo aplican algunas.
- Emitir el spec sin pasar por el gate go/no-go.
- Refactorizar código no relacionado ni cambiar otros assets del scaffold.
- Ignorar fallos del validator o de los subcomandos del binario.

---

## 19. Entregables

Al finalizar deben quedar:

- [ ] `kit/commands/vector/research.md` (project command).
- [ ] `kit/agents/vector-feasibility-reviewer.md` (agente Sonnet read-only, por lente).
- [ ] Copias embebidas en `cli/internal/scaffold/assets/…` vía `go generate`.
- [ ] `cli/internal/scaffold/scaffold_test.go` actualizado si enumera el set.
- [ ] `go -C cli vet ./...` y `go -C cli test ./internal/scaffold/...` en verde.
- [ ] `vector init` en repo limpio siembra command + agente.
- [ ] `docs/plugin-and-commands.md` actualizado si enumera los `/vector:*`.

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `kit/commands/vector/raw.md` (autoría self-contained + `spec create` + routing) y
      `comment.md` (delegación a Sonnet), y `kit/agents/vector-comment-evaluator.md` (patrón de
      agente).
- [ ] Confirmé que `vector spec create` y `vector spec route` existen y los reuso (sin código Go
      nuevo).
- [ ] El command es agnóstico: no hardcodea stack; pregunta cuando la detección de lentes falla.
- [ ] El agente revisor es Sonnet, read-only y parametrizado por lente; reúne su propia evidencia.
- [ ] Mantuve CLI-owns-writes; card/spec doc/route solo vía el binario.
- [ ] Implementé el gate go/no-go; sin "go" no se crea card.
- [ ] El spec emitido tiene 20 secciones + anexo `Reporte de viabilidad` y pasa el validator.
- [ ] Registré `agent.routed` por cada paso de agente.
- [ ] Corrí `go generate` + `go vet` + el test del scaffold.
- [ ] Verifiqué que `vector init` siembra los nuevos assets.
- [ ] No agregué dependencias, eventos, endpoints ni UI.
- [ ] No dejé TODOs sin justificar.

---

## Reporte de viabilidad

> Esta sección es la **plantilla de runtime** que `/vector:research` rellena al componer cada spec
> (es el anexo embebido). Los slots entre **ángulos** (`<…>`) son ranuras que el command sustituye
> en ejecución — **no** son contenido de spec sin resolver (esos serían `[...]`): el implementador
> debe reproducir esta plantilla literal en el command, no rellenarla aquí. Para *este* spec —el del
> propio command— las lentes **no se corrieron** (el command aún no existe; este spec se autora vía
> `/vector:raw`); ver Open questions.

| Lente | Veredicto | Confianza | Hallazgos clave | Riesgos |
|---|---|---|---|---|
| technical | `<go\|go-with-risks\|no-go>` | `<N/10>` | `<hallazgos>` | `<riesgos>` |
| security | `<go\|go-with-risks\|no-go>` · o `No corrida — no aplica` | `<N/10>` | `<hallazgos>` | `<riesgos>` |
| marketing | `<go\|go-with-risks\|no-go>` · o `No corrida — no aplica` | `<N/10>` | `<hallazgos>` | `<riesgos>` |
| design | `<go\|go-with-risks\|no-go>` · o `No corrida — no aplica` | `<N/10>` | `<hallazgos>` | `<riesgos>` |

**Veredicto consolidado:** `<go\|go-with-risks\|no-go>` — `<resumen de 1–2 líneas>`.

---

## Open questions

- ¿Un agente revisor parametrizado por lente (V1) o agentes especializados por disciplina
  (`vector-security-reviewer`, `vector-marketing-reviewer`, …)? V1: uno parametrizado, por
  simplicidad de kit/vendoring. Revisar si la calidad por disciplina lo justifica.
- ¿Las lentes deben poder hacer **investigación web** (competidores, prior art) en una extensión
  futura, o quedarse siempre sobre la idea + repo? V1: solo idea + repo.
- ¿Persistir en `.vector/config.json` el mapa señal→lente o el set por tipo de idea para no
  re-detectar/re-preguntar? Fuera de V1.
- ¿El veredicto consolidado debe poder mover la card a `needs-attention` en vez de `draft` cuando
  es `go-with-risks`? V1: siempre `draft` (como `raw`); el riesgo vive en el reporte embebido.
- Las cuatro lentes de viabilidad **no se corrieron sobre este spec** (el command `/vector:research`
  aún no existe; este spec se autora vía `/vector:raw`). Al implementar y dogfoodear el command, se
  puede regenerar este spec con su propio reporte de viabilidad lleno.
