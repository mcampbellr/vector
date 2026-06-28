# Spec: Comando `/vector:fix` (corregir specs ya especificados)

## 1. Objetivo

Construir `/vector:fix <id>`: la versión Vector-nativa del skill standalone `/fix`. Toma una **nota de corrección** contra un spec **ya en el board** (algo que faltó en la spec, un hallazgo de UAT, un course-correction menor), la **refina** (clasifica si toca spec, código, o ambos), **clarifica solo si hace falta**, **amenda la spec doc cuando corresponde**, **implementa** los cambios de código vía un agente Sonnet, **valida** (tests + build) y **mueve la card por el ciclo de vida** del board mientras dura el fix.

Permite que un dev integre correcciones contra trabajo **ya especificado** sin salir del flujo de Vector, reutilizando la disciplina refiner → clarity-gate → implementer del `/fix` original, pero **integrada al state machine, al meter de tokens y al activity trace** de Vector. A diferencia de `/fix`, las escrituras de dominio pasan por el binario (CLI-owns-writes) y los agentes son **propios de Vector** (no dependen del skill personal del usuario).

## 2. Alcance

### Incluido en esta fase

- Nuevo **project command** `/vector:fix <id>` (`kit/commands/vector/fix.md`, vendored a `cli/internal/scaffold/assets/commands/vector/fix.md` por `go generate`, sembrado por `vector init`/`vector update`).
- Nuevo **subcomando de binario** `vector spec fix <id>` que **registra la corrección** (evento tipado `spec.fixed` con clasificación + archivos + resultado de validación). **No transiciona status**: las transiciones del ciclo van por el subcomando existente `vector spec status` (separación de concerns; el binario sigue siendo el único escritor).
- Dos **agentes propios** en `kit/agents/` (+ vendored a scaffold): `vector-fix-refiner` (Haiku, read-only) y `vector-fix-implementer` (Sonnet), espejo del patrón `vector-spec-refiner`/`vector-spec-validator`.
- **Clasificación** de la corrección por el refiner: `spec-only` / `code-only` / `spec+code` (el implementer la respeta, no la re-cuestiona).
- **Clarity gate**: `CLEAR` → ejecuta directo; `NEEDS_CLARIFICATION` → pregunta vía `AskUserQuestion` antes de implementar.
- **Ciclo de vida del fix** (vía `vector spec status`): al iniciar sobre un spec en `review`, lo mueve a `in-progress` (fix claro, en trabajo) o `needs-attention` (falta info / bloqueado, con `reason`); al terminar el fix vuelve a `review`. Sobre un spec en `open`, entra `open → in-progress` y sale `in-progress → review`. Sobre un spec ya en trabajo (`in-progress`/`needs-attention`), corrige en sitio.
- **Token routing**: graba el uso de cada agente barato vía `vector spec route <id> --model haiku|sonnet --baseline opus`.
- **Work trace**: graba lo que el run tocó vía `vector spec worklog <id>` (enriquece el standup).
- **Sin auto-commit**: deja el working tree para revisión del dev (coherente con `/vector:apply`).

### Fuera de scope

- **Fix suelto sin card** (el modo code-only-sin-card de `/fix`): `/vector:fix` **siempre** opera sobre un `<id>` existente del board.
- **Auto-commit / `git commit`**: diverge de `/fix` a propósito; el dev revisa y commitea (o usa su flujo).
- **Features nuevas o investigación de bugs sin especificar**: scope guard del refiner → rutea a `/vector:raw` (o `/idea`/`/bug`) y se detiene sin escribir.
- **Correcciones a specs en `draft`/`closed`/`archived`**: un draft se amenda re-corriendo `/vector:raw`; `closed`/`archived` no se corrigen aquí. `fix` los rechaza.
- **Que `vector spec fix` haga transiciones de status**: las posee `vector spec status` (la máquina LOCKED). `fix` solo registra el evento de corrección.
- **Gestión de branches/worktrees** y **archive del change de OpenSpec** (responsabilidad del repo/OpenSpec).
- **Cambios en el schema de `SpecState`**: la corrección vive en eventos (additivo), no en campos nuevos del state.
- **Panel web**: el board proyecta el nuevo evento, pero no se diseña UI nueva en esta fase.

El agente no implementa nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Binario: **Go** (módulo único `github.com/mariocampbell/vector`, stdlib; `cli/`). Sin dependencias externas.
- Project command: **Markdown + frontmatter** orquestado por Claude (patrón `kit/commands/vector/apply.md`, `raw.md`).
- Agentes: **Markdown + frontmatter** en `kit/agents/` (patrón `vector-spec-refiner.md`, `vector-summary-writer.md`).

### Versiones relevantes

- Go: `1.26` (citado de `cli/go.mod`).
- OpenSpec: detectado en runtime; no se pinnea versión (modo delegado usa lo que el repo tenga). El fix opera sobre la spec doc y, si el spec tiene `openspec.change`, sobre el artefacto del change.

### Patrones existentes a respetar

- `cli/cmd/vector/main.go` → `runSpec()`: switch de subcomandos `spec *` (`create|list|propose|apply|link|status|close|archive|next|worklog|summarize|route`). Se agrega `case "fix": return runSpecFix(args[1:])`.
- `cli/internal/state/store.go`: `Store` es el **único escritor**, serializado por mutex (`s.mu`); helpers `appendEvent` (asume lock tomado), `writeSpecFile`, `ReadSpec`, `statePath`, y `setStatusTimestamp` (definido aquí, `store.go:344`). `ProposeSpec` (`store.go:302`) es el **patrón a imitar** para un escritor que toma el lock una vez y persiste + emite eventos inline.
- `cli/internal/state/transition.go`: `allowedTransitions` (máquina LOCKED, `transition.go:15`), `CanTransition`, `applyTransition` (toma `s.mu` por sí mismo → **no** llamarlo con el lock ya tomado: `sync.Mutex` no es reentrante), y los transicionadores `ApplySpec`/`CloseSpec`/`ArchiveSpec`/`SetStatus`.
- `cli/internal/state/event.go`: `EventType` const block (`EvtSpecApplied`, `EvtStatusChanged`, `EvtWorkLogged`, `EvtAgentRouted`, …). Se agrega `EvtSpecFixed EventType = "spec.fixed"` + `FixedData`.
- `cli/internal/scaffold/scaffold.go`: `//go:embed all:assets` + `//go:generate sh -c "… cp -R ../../../kit/{commands,agents,vector} assets/"`. El command y agentes nuevos se vendoran corriendo `go generate ./...` en `cli/`.
- Command pattern: el command **orquesta** (refiner, clarity gate, implementer, validación, transiciones vía `status`) y **llama al binario**; el binario es el único escritor del state (CLI-owns-writes). El command nunca edita `.vector/` a mano (lectura del state.json sí está permitida).
- Token routing y work trace vía subcomandos existentes (`route`, `worklog`).
- Git artifacts en inglés kebab-case; el id del spec == nombre del change de OpenSpec.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Binario con `internal/state` (`Store`, `transition.go`, `event.go`, `store.go`), `internal/config`, `internal/scaffold` — ya existen.
- [x] Subcomandos `vector spec route`, `vector spec worklog` y `vector spec status` — ya existen (verificado en `main.go` switch).
- [x] Máquina de estados LOCKED con las transiciones del fix legales: `open→in-progress`, `review→in-progress`, `review→needs-attention`, `in-progress→review`, `needs-attention→in-progress`, `needs-attention→review` (verificado en `allowedTransitions`, `transition.go:15`).
- [x] Patrón de agentes vendored (`kit/agents/` → `scaffold/assets/agents/`) y de sembrado por `init`/`update`.
- [ ] Spec objetivo existente en status **corregible**: `open`/`in-progress`/`needs-attention`/`review` (no `draft`/`closed`/`archived`) — es el input de `fix`, lo provee el usuario.

Si alguna dependencia no existe, el command/binario debe detenerse y reportar exactamente qué falta. No inventar contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

Orquestación en project command con **ruteo a agentes baratos** (token-routing). El command: lee y valida el spec → **refina** (Haiku, read-only) → clarity gate → mueve el ciclo de entrada vía `vector spec status` → **implementa** (Sonnet) → valida tests/build → mueve el ciclo de salida vía `vector spec status` → **registra la corrección** vía `vector spec fix` → graba meter de tokens y work trace → reporta sin commitear. El binario es el único escritor del state; las transiciones siempre pasan por la máquina LOCKED.

### Capas afectadas

- presentation (CLI/command): sí — `kit/commands/vector/fix.md` orquesta; `runSpecFix` en `main.go` parsea flags y reporta.
- application/use-cases: no aplica (no hay capa de use-cases separada en el binario).
- domain (state): sí — `state.FixSpec` (solo registra `spec.fixed`, sin transicionar); `event.go` (`EvtSpecFixed`, `FixedData`).
- data/infrastructure: sí — append a `activity.jsonl` + bump de `UpdatedAt` (vía `Store`, lock único); vendoring del command/agentes en `scaffold`.
- shared/common: no.
- web/: no — el board solo proyecta el nuevo evento (sin UI nueva en esta fase).

### Flujo esperado

1. Dev ejecuta `/vector:fix <id> [nota]`.
2. El command lee `.vector/specs/<id>/state.json` directamente (lectura permitida en el command; CLI-owns-writes aplica solo a escrituras) y valida que exista y que el status sea corregible (no `draft`/`closed`/`archived`).
3. **Refina**: invoca `vector-fix-refiner` (Haiku, read-only) con la nota + la spec doc + `openspec.change` si lo hay → retorna `BRIEF` (`Optimized Fix Title`, `Change Classification`, `Artifacts To Amend`, `Files To Touch`, `Acceptance`, `Risks`, `Clarity Verdict`, `Blocking Clarifying Questions`).
4. **Scope guard**: si el refiner clasifica como feature nueva / bug fresco → ruta a `/vector:raw` (o `/idea`/`/bug`) y se detiene **sin escribir nada**.
5. **Clarity gate**: `CLEAR` → continúa; `NEEDS_CLARIFICATION` → pregunta vía `AskUserQuestion` (batches ≤5) y refold del brief hasta que no quede bloqueante.
6. **Transición de entrada** (vía `vector spec status`, que enforza la máquina LOCKED):
   - `review` → `vector spec status <id> in-progress` (fix claro) o `vector spec status <id> needs-attention --reason "<qué falta>"` (bloqueado).
   - `open` → `vector spec status <id> in-progress`.
   - `in-progress` → sin cambio (continuación). `needs-attention` → resolver el `reason` a `in-progress` cuando empieza el trabajo.
7. **Implementa**: invoca `vector-fix-implementer` (Sonnet) con el brief + clasificación + `SPEC_LANGUAGE` → amenda los artefactos OpenSpec listados (si `spec-only`/`spec+code`), edita el código (si `code-only`/`spec+code`), corre tests+build del paquete afectado y retorna `RESULT` (`status`, `files_touched`, `artifacts_amended`, `validation`).
8. **Gating** (lo posee el command): `RESULT.status == "blocked"` o `validation == "fail"` → surfacea, **no** transiciona a review, pregunta retry/stop. El binario no es el gate: `--validation-result` es metadata informativa del evento.
9. **Transición de salida** (vía `vector spec status`): si el ciclo entró a trabajo desde `review`/`open` y la validación pasó → `vector spec status <id> review`. Si era continuación de trabajo activo, queda en `in-progress` para que el dev luego use `/vector:apply`/`/vector:close`.
10. **Registra la corrección**: `vector spec fix <id> --classification <c> --artifacts <list> --files <list> --validation-result pass` → el binario appendea el evento `spec.fixed` (sin tocar status).
11. **Meter + trace**: `vector spec route` por cada agente barato; `vector spec worklog` con archivos/tareas/nota.
12. **Reporta** sin commitear: id, clasificación, archivos, transiciones (anterior → nueva), resultado de validación, próximo paso.

### Ubicación de archivos nuevos

```txt
kit/
  commands/vector/fix.md            # project command (orquestación)
  agents/vector-fix-refiner.md      # Haiku, read-only
  agents/vector-fix-implementer.md  # Sonnet
cli/internal/scaffold/assets/       # copias vendored (generadas por go generate)
  commands/vector/fix.md
  agents/vector-fix-refiner.md
  agents/vector-fix-implementer.md
cli/cmd/vector/main.go              # runSpecFix
cli/internal/state/store.go         # FixSpec (junto a ProposeSpec)
cli/internal/state/event.go         # EvtSpecFixed + FixedData
```

No crear carpetas nuevas: se reusa la convención existente de `kit/` y `scaffold/assets/`.

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/commands/vector/fix.md` | NUEVO | Project command: validación, refiner, clarity gate, transiciones vía `status`, implementer, gating, registro vía binario, meter/trace, reporte (sin commit) | `kit/commands/vector/apply.md` |
| `kit/agents/vector-fix-refiner.md` | NUEVO | Agente Haiku read-only: clasifica la corrección, lista artefactos/archivos, emite Clarity Verdict + preguntas bloqueantes | `kit/agents/vector-spec-refiner.md` |
| `kit/agents/vector-fix-implementer.md` | NUEVO | Agente Sonnet: amenda artefactos OpenSpec listados, edita el código, corre tests+build, retorna JSON. **No commitea/stagea** | `kit/agents/vector-spec-validator.md` (estructura frontmatter/IO; este edita además) |
| `cli/cmd/vector/main.go` | MODIFICAR | `runSpec()` + `case "fix"` → `runSpecFix()` (flags, validación, reporte JSON/humano) | `runSpecApply`/`runSpecStatus` en el mismo archivo |
| `cli/internal/state/store.go` | MODIFICAR | `FixSpec(...)`: appendea `spec.fixed` inline bajo el lock único; **no** transiciona | `ProposeSpec` (`store.go:302`, mismo patrón de lock+inline) |
| `cli/internal/state/event.go` | MODIFICAR | `EvtSpecFixed EventType = "spec.fixed"` + `type FixedData struct{…}` | `EvtWorkLogged` + `WorkLoggedData` |
| `cli/internal/scaffold/assets/commands/vector/fix.md` | NUEVO (generado) | Copia embebida del command | hermanos `apply.md`/`raw.md` |
| `cli/internal/scaffold/assets/agents/vector-fix-refiner.md` | NUEVO (generado) | Copia embebida del agente | hermanos `vector-spec-refiner.md` |
| `cli/internal/scaffold/assets/agents/vector-fix-implementer.md` | NUEVO (generado) | Copia embebida del agente | hermanos en `assets/agents/` |
| `cli/internal/state/store_test.go` | MODIFICAR | Tests table-driven de `FixSpec` | `store_test.go` existente |

### Detalle por archivo

#### `kit/commands/vector/fix.md`

Acción: NUEVO

Debe implementar:
- Leer `$ARGUMENTS`: primer token = `<id>` (requerido); resto = nota de corrección (opcional; si vacío, pedirla vía `AskUserQuestion`).
- Leer el state.json del spec y validar status ∈ {`open`,`in-progress`,`needs-attention`,`review`}.
- Invocar `vector-fix-refiner` (Haiku); aplicar scope guard y clarity gate.
- Aplicar la transición de entrada vía `vector spec status` (`in-progress` / `needs-attention --reason`).
- Invocar `vector-fix-implementer` (Sonnet); aplicar gating de `RESULT` (el command es el gate).
- Si validó: transición de salida a `review` vía `vector spec status`; luego `vector spec fix <id> …` para registrar la corrección.
- Grabar `vector spec route` por agente y `vector spec worklog` del run.
- Reportar en el idioma del usuario; **no** commitear.

Debe seguir como referencia: `kit/commands/vector/apply.md` (estructura, no-auto-commit, worklog, route) y `/fix` SKILL.md (pasos refiner/clarity/implementer/scope-guard).

No debe incluir: reimplementación de la lógica del binario; edición manual de `.vector/`; creación de branches; auto-commit; transiciones por fuera de `vector spec status`.

#### `kit/agents/vector-fix-refiner.md`

Acción: NUEVO

Debe implementar (frontmatter `model: haiku`, tools read-only `Read, Grep, Glob`): leer la nota + spec doc + artefactos del change, devolver la estructura exacta del brief (clasificación `spec-only|code-only|spec+code`, artefactos a amendar, archivos a tocar, acceptance, riesgos, **Clarity Verdict** `CLEAR|NEEDS_CLARIFICATION`, preguntas bloqueantes). Surfacea ambigüedad; no inventa intención. Espejo de `vector-spec-refiner.md`.

#### `kit/agents/vector-fix-implementer.md`

Acción: NUEVO

Debe implementar (frontmatter `model: sonnet`, tools de edición + Bash de test/build): aplicar el brief aprobado — amendar los artefactos OpenSpec listados (si la clasificación lo pide), editar el código listado, correr el gate de tests+build del paquete afectado, retornar JSON (`status`, `fix_title`, `classification`, `files_touched`, `artifacts_amended`, `validation{result,commands,notes}`, `blocked_reason?`, `extra_edits?`). **No** stagear/commitear/pushear (el command y el dev manejan git). Respeta la convención del repo del usuario (Vector es agnóstico al código).

#### `cli/cmd/vector/main.go`

Acción: MODIFICAR

- `runSpec()`: agregar `case "fix": return runSpecFix(args[1:])`.
- `runSpecFix`: flags `--repo-root`, `--classification <spec-only|code-only|spec+code>`, `--artifacts <comma-list>`, `--files <comma-list>`, `--validation-result <pass|fail>`, `--json`. **No** lleva `--new-status` (las transiciones son de `vector spec status`).
- Validar `<id>` kebab-case y existente; rechazar `draft`/`closed`/`archived`; validar `--classification` y `--validation-result`.
- Llamar `store.FixSpec(...)`; reportar (JSON o humano) id, clasificación, archivos, validación.
- Errores accionables; exit `0`/`1`.

Restricciones: no cambiar `runSpecCreate`/`runSpecApply`/`runSpecStatus`/etc.; no cambiar el schema JSON existente.

#### `cli/internal/state/store.go`

Acción: MODIFICAR

- `FixSpec(id, classification, validationResult string, artifacts, files []string, actor string, now time.Time) (*SpecState, error)`, modelado **exactamente** sobre `ProposeSpec` (`store.go:302`):
  - `s.mu.Lock(); defer s.mu.Unlock()` **una sola vez**; `ReadSpec(id)`.
  - Validar status corregible (no `draft`/`closed`/`archived`) — error accionable si no.
  - `now = now.UTC()`; `spec.UpdatedAt = now`; `writeSpecFile(s.statePath(id), spec)`.
  - `appendEvent(Event{… Type: EvtSpecFixed, Data: json(FixedData{Classification, ValidationResult, Artifacts, Files})})` — `appendEvent` asume el lock ya tomado.
  - **No** llamar `applyTransition` ni `SetStatus` (tomarían `s.mu` de nuevo → deadlock; `sync.Mutex` no es reentrante). `FixSpec` **no** transiciona.
  - Retornar el spec.

Restricciones: no tocar `ApplySpec`/`CloseSpec`/`SetStatus`/`ProposeSpec`; escritura serializada por el lock único; no stampear timestamps de lifecycle (eso es de los transicionadores).

#### `cli/internal/state/event.go`

Acción: MODIFICAR

- Agregar `EvtSpecFixed EventType = "spec.fixed"` al const block.
- Agregar `type FixedData struct { Classification string \`json:"classification"\`; ValidationResult string \`json:"validationResult,omitempty"\`; Artifacts []string \`json:"artifacts,omitempty"\`; Files []string \`json:"files,omitempty"\` }`.

Restricciones: no renombrar eventos existentes; `EventVersion` sin cambios.

---

## 7. API Contract

No aplica — esta feature **no** expone API HTTP (sin endpoints, headers, métodos ni response body; los códigos HTTP no aplican, igual que en `add-propose-command`). La superficie de integración es el **CLI del binario** consumido por el project command (CLI-owns-writes). Contrato del subcomando de registro:

```bash
vector spec fix <id> \
  [--classification spec-only|code-only|spec+code] \
  [--artifacts <comma-list>] [--files <comma-list>] \
  [--validation-result pass|fail] \
  [--repo-root <path>] [--json]
```

Las transiciones de status **no** son de este subcomando; usan el contrato existente:

```bash
vector spec status <id> <in-progress|needs-attention|review> [--reason "<reason>"]
```

Salida `--json` de `vector spec fix` (éxito):

```json
{ "id": "add-foo", "classification": "spec+code",
  "validationResult": "pass",
  "artifacts": ["openspec/changes/add-foo/spec.md"],
  "files": ["cli/internal/state/store.go"],
  "updatedAt": "2026-06-27T..." }
```

Exit: `0` éxito; `1` error. El status anterior/nuevo lo reporta el command a partir de las llamadas a `vector spec status`.

### Endpoints involucrados

No aplica — sin endpoints HTTP. (El board consume el evento `spec.fixed` vía la proyección SSE existente del activity trace, no diseñada en esta fase.)

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `vector spec fix <id>` existe en el switch `runSpec`, parsea flags y valida status corregible (no `draft`/`closed`/`archived`).
- [ ] `FixSpec` appendea el evento `spec.fixed` con `classification` + `validationResult` + archivos/artefactos, bajo el lock único, **sin** transicionar (sigue el patrón `ProposeSpec`, no llama `applyTransition`).
- [ ] El command refina (Haiku), clasifica, y respeta el clarity gate (`CLEAR` → sin preguntas; `NEEDS_CLARIFICATION` → itera).
- [ ] El scope guard rutea a `/vector:raw`/`/idea`/`/bug` sin escribir nada cuando no es una corrección.
- [ ] El ciclo `review/open → in-progress|needs-attention → review` ocurre vía `vector spec status` (verificable en eventos `status.changed`).
- [ ] Un `code-only` sobre un spec en `in-progress` corrige sin forzar status.
- [ ] La validación de tests/build **gatea** la transición de salida en el command (fail → no llama `status review` ni `spec fix`).
- [ ] El command **no** auto-commitea; el reporte avisa que el working tree quedó para revisión.
- [ ] Token routing grabado vía `vector spec route` por cada agente; work trace vía `vector spec worklog`.
- [ ] Los agentes `vector-fix-refiner`/`vector-fix-implementer` quedan vendored en `scaffold/assets/agents/` y se siembran con `init`/`update`.
- [ ] Sin regresiones: `spec create|list|apply|propose|close|status|route|worklog` siguen funcionando.

### Tests requeridos

Agregar o actualizar tests para:

- [ ] `FixSpec` appendea `spec.fixed` con la data esperada y bumpea `UpdatedAt`.
- [ ] `FixSpec` **no** cambia el status del spec.
- [ ] `FixSpec` rechaza spec en `draft`/`closed`/`archived`.
- [ ] `runSpecFix` valida flags/id y `--classification`/`--validation-result` (table-driven).
- [ ] Vendoring: `scaffold` embebe `fix.md` + ambos agentes (extiende el test existente que verifica embedded commands).

### Comandos de verificación

Ejecutar:

```bash
go -C cli generate ./...      # re-vendora kit/ a scaffold/assets
gofmt -l cli
go -C cli vet ./...
go -C cli test ./...
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

No aplica la UX web; aplica la **UX del CLI/command** (texto plano + JSON).

- **Reporte claro**: qué se corrigió (`spec-only`/`code-only`/`spec+code`), status anterior → nuevo, archivos tocados, resultado de validación, y aviso explícito de que **no** se commiteó.
- **Refiner/clarity amable**: mostrar la ambigüedad concreta y pedir confirmación; nunca forzar una suposición.
- **Atomicidad percibida**: la corrección queda registrada + reportada, o se detiene limpiamente sin medias tintas en el state.
- **Navegación**: en éxito sugerir el próximo paso (`/vector:close <id>` si volvió a `review`; "trabajo continúa" si quedó en `in-progress`).
- **Errores accionables**: `spec "<id>" not found`, `spec "<id>" is draft — amend with /vector:raw`, `illegal transition <from> → <to>` (de `vector spec status`), `entering needs-attention requires a reason`.
- **Idioma**: la conversación/reporte en el idioma del usuario; strings del binario en inglés (sección 16).

Las subsecciones de formularios/passwords/loading/accesibilidad de UI no aplican (no hay frontend nuevo).

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas y el agente no debe cuestionarlas ni cambiarlas:

- **Siempre requiere una card existente** (`<id>`): no hay "fix suelto" sin card. *Por qué:* el board es la fuente de verdad; toda corrección deja rastro de dominio. (Confirmado con el usuario.)
- **Ciclo de vida del fix**: un spec en `review` se mueve a `in-progress` (fix claro) o `needs-attention` (falta info, con `reason`) mientras dura el fix, y **vuelve a `review`** al terminar; un spec en `open` entra a `in-progress` y sale a `review`. (Confirmado con el usuario.)
- **`vector spec fix` no transiciona**: solo registra `spec.fixed`. Las transiciones del ciclo van por `vector spec status` (la máquina LOCKED). *Por qué:* separación de concerns, sin duplicar lógica de transición ni arriesgar un deadlock de mutex re-entrante.
- **No auto-commit**: el command deja el working tree para revisión, igual que `/vector:apply` ("apply implementa, no shippea"). Diverge de `/fix` a propósito por coherencia interna de Vector. (Confirmado con el usuario.)
- **Agentes propios en `kit/`**: `vector-fix-refiner` (Haiku) + `vector-fix-implementer` (Sonnet), vendored y sembrados — **no** se delega al skill personal `/fix` del usuario. *Por qué:* self-contained y comercializable día-0; no depende de `~/.dotfiles`. (Confirmado con el usuario.)
- **Nuevo subcomando `vector spec fix` con evento tipado `spec.fixed`**: para que el standup/board distinga una corrección del trabajo original. (Confirmado con el usuario.)
- **Clasificación = refiner (Haiku), no implementer**: el barato decide spec/code/ambos; el caro la respeta. *Por qué:* ahorro de tokens + separación de responsabilidad (token-routing).
- **CLI-owns-writes**: el binario es el único escritor del state; el command nunca edita `.vector/` a mano (state-model).

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, pero no implementarla.

---

## 11. Edge cases

La implementación debe manejar explícitamente:

### Datos inválidos / precondición

- **Spec no encontrado** → `spec "<id>" not found`; no escribe nada.
- **Status no corregible** (`draft`/`closed`/`archived`) → error accionable (`use /vector:raw` para draft; `closed/archived no se corrige aquí`).
- **`<id>` ausente o no kebab-case** → error de validación del command/flag.
- **Nota de corrección vacía** → el command la pide vía `AskUserQuestion`; no procede sin nota.

### Clasificación / scope

- **Refiner detecta feature nueva o bug fresco** → ruta a `/vector:raw`/`/idea`/`/bug`; stop sin escribir.
- **`code-only`** → no amenda spec doc; solo evento `spec.fixed` + edición de código.
- **`spec-only`** → amenda artefactos OpenSpec / spec doc; sin edición de código.

### Transición (las posee `vector spec status`)

- **Transición ilegal** (p. ej. `review → closed`) → `illegal transition review → closed`; el binario la rechaza (máquina LOCKED).
- **`needs-attention` sin `reason`** → `entering needs-attention requires a reason`.
- **Spec ya en `in-progress`/`needs-attention`** → no se fuerza el bump de entrada; se corrige en sitio.

### Implementación

- **Implementer `blocked`** → surfacea `blocked_reason`; no transiciona a review; pregunta retry/stop.
- **Validación `fail`** (no pre-existente) → el command surfacea comandos fallidos; no llama `status review` ni `spec fix`; pregunta retry/stop.

### I/O / concurrencia

- **Spec doc/artefacto inexistente en disco** → `spec doc not found at <path>`; state intacto.
- **Sin permiso de escritura** → error con contexto; state intacto.
- **Dos `fix` del mismo id** → el mutex del `Store` serializa; el segundo ve el spec actualizado.

### No aplica

- **Sin conexión / timeout / respuesta vacía**: no aplican — el comando es CLI/filesystem local, sin HTTP. (Si el gate de tests del usuario hace red, es responsabilidad de su toolchain, fuera del alcance de Vector.)

---

## 12. Estados de UI requeridos

No hay UI web nueva. Estados del **command/binario** (texto + JSON):

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | espera `<id>` + nota | invocar con id/nota |
| loading (refining) | refiner Haiku analiza | esperar |
| clarifying | brecha de ambigüedad | responder preguntas |
| loading (implementing) | implementer Sonnet ejecuta | esperar |
| loading (validating) | corre tests/build | esperar |
| success | clasificación + archivos + status antes/después + aviso "sin commit" | `/vector:close` (si review) o seguir trabajando |
| error | not found / draft\|closed / scope-guard / transición ilegal / implementer falla | corregir id/status o detenerse |
| empty | No aplica — `fix` siempre opera sobre un `<id>` existente; sin id no hay estado vacío que pintar |
| offline | No aplica — CLI local-only, sin red propia |
| disabled | No aplica — sin componentes UI interactivos |

---

## 13. Validaciones

### Validaciones de cliente (command)

| Campo | Regla | Mensaje |
|---|---|---|
| `<id>` | requerido, kebab-case `[a-z0-9-]+` | "id requerido (kebab-case)" |
| `<id>` | existe en `.vector/specs/<id>/state.json` | `spec "<id>" not found` |
| `<id>` | status ∈ {open,in-progress,needs-attention,review} | `spec "<id>" is <status> — no corregible aquí` |
| nota | no vacía | se pide vía `AskUserQuestion` |
| validación | el command **es el gate**: solo llama `status review` + `spec fix` cuando `validation == pass` | — |

### Validaciones de servidor (binario)

| Flag/Campo | Regla | Error |
|---|---|---|
| `--classification` | ∈ {spec-only,code-only,spec+code} | `invalid --classification` |
| `--validation-result` | ∈ {pass,fail}; **informativo** (se registra en el evento; el binario no gatea con él) | `invalid --validation-result` |
| status del spec | corregible (no draft/closed/archived) | `spec "<id>" is <status> — amend with /vector:raw` |

Las transiciones (`in-progress`/`needs-attention`/`review`) las valida `vector spec status` vía `CanTransition` (máquina LOCKED); `entering needs-attention requires a reason` lo emite `applyTransition`.

---

## 14. Seguridad y permisos

- No exponer secrets, tokens ni paths internos sensibles en stdout/eventos.
- No imprimir el spec doc completo; reportar id, paths, clasificación.
- Los artefactos modificados son del repo del usuario; respetar `.gitignore` (el implementer no toca lo ignorado).
- Sin asumir permisos: error con contexto si no se puede escribir spec doc/artefacto; state intacto.
- El implementer (Sonnet) trabaja dentro del repo; no introduce binarios externos ni instala dependencias (si falta una, surfacea el comando exacto y se detiene).
- `.vector/local/` (summaries) sigue gitignored; el evento `spec.fixed` vive en `activity.jsonl` local.

---

## 15. Observabilidad y logging

Usar el `activity.jsonl` existente (additivo, vía `Store`):

- `spec.fixed` — `{classification, validationResult?, artifacts?, files?}`.
- `status.changed` — `{from, to, trigger:"command", reason?}` en cada transición de entrada/salida (emitido por `vector spec status`/`applyTransition`).
- `agent.routed` — vía `vector spec route` por cada agente barato (alimenta el Token Savings Meter).
- `work.logged` — vía `vector spec worklog` (enriquece el standup digest).

No registrar: spec doc completo, secrets, payloads internos irrelevantes.

---

## 16. i18n / textos visibles

El proyecto **no** tiene sistema de i18n. El binario emite strings en **inglés hardcodeado**; el command conversa en el idioma del usuario. Identificadores (documentación, no keys de archivo):

| Identificador (doc) | Texto (hardcoded EN) |
|---|---|
| fix.success | `fixed spec "<id>" (<classification>)` |
| fix.no_commit | `working tree left for review (not committed)` |
| fix.not_found | `spec "<id>" not found` |
| fix.not_fixable | `spec "<id>" is <status> — amend with /vector:raw` |
| fix.invalid_classification | `invalid --classification` |
| fix.invalid_validation | `invalid --validation-result` |

(Los strings de transición — `illegal transition …`, `entering needs-attention requires a reason` — son de `vector spec status`/`applyTransition`, ya existentes.) No hardcodear textos de UI (no hay frontend nuevo).

---

## 17. Performance

- Refiner (Haiku): read-only, bounded (nota + spec doc). Barato por diseño (token-routing).
- Implementer (Sonnet): proporcional a la complejidad del fix; gateado por tests/build.
- `FixSpec`: lock + read + 1 escritura de state.json (bump `UpdatedAt`) + append de evento → operación de bajo costo (<50ms típico).
- Sin I/O redundante: leer el spec una vez, escribir una vez. El sync del board es de bajo consumo (un evento, no recarga total).

---

## 18. Restricciones

El agente no debe:

- Cambiar el schema de `SpecState` (la corrección vive en eventos, no en campos nuevos).
- Hacer que `FixSpec` transicione status, ni llamar `applyTransition`/`SetStatus` desde `FixSpec` (mutex no reentrante → deadlock).
- Transicionar specs por fuera de la máquina LOCKED (`allowedTransitions`); las transiciones van por `vector spec status`.
- Reimplementar la lógica del binario en el command, ni editar `.vector/` a mano.
- Auto-commitear, stagear o pushear (ni el command ni el implementer).
- Delegar a los agentes del skill `/fix` de `~/.dotfiles` (Vector trae los suyos).
- Instalar dependencias nuevas (stdlib Go; agentes son Markdown).
- Cambiar subcomandos existentes de `spec` (solo agregar `fix`).
- Crear features nuevas dentro del fix (scope guard → rutear y detener).
- Ignorar fallos de `gofmt`/`vet`/tests/build.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `cli/cmd/vector/main.go`: `runSpecFix` + `case "fix"`.
- [ ] `cli/internal/state/store.go`: `FixSpec` (patrón `ProposeSpec`, sin transición).
- [ ] `cli/internal/state/event.go`: `EvtSpecFixed` + `FixedData`.
- [ ] `kit/commands/vector/fix.md` (+ vendored en `scaffold/assets/`).
- [ ] `kit/agents/vector-fix-refiner.md` y `vector-fix-implementer.md` (+ vendored).
- [ ] Tests de `FixSpec` y `runSpecFix` (table-driven) + extensión del test de vendoring.
- [ ] Token routing y work trace integrados en el command.
- [ ] `go -C cli generate ./...` corrido; `gofmt`/`vet`/`test` verdes.
- [ ] Reporte sin commit (working tree para revisión).
- [ ] Documentación del command/flujo actualizada donde aplique (`docs/plugin-and-commands.md` lista los `/vector:*`).

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Revisé `docs/domain-contract.md` (transiciones permitidas, mapa comando→escritura) y `.claude/rules/architecture/state-model.md`.
- [ ] Confirmé que `vector spec route`/`worklog`/`status` y la máquina de estados existen y soportan el ciclo del fix.
- [ ] Seguí los patrones reales (`runSpecApply`/`runSpecStatus`, `ProposeSpec` para `FixSpec`, `appendEvent`, lock único, vendoring `go generate`).
- [ ] `FixSpec` no llama `applyTransition`/`SetStatus` (sin deadlock) y no cambia status.
- [ ] Solo modifiqué los archivos listados o justifiqué cualquier excepción.
- [ ] Implementé refiner (Haiku) + clarity gate + scope guard + implementer (Sonnet) con agentes propios.
- [ ] Implementé el ciclo `review/open → in-progress|needs-attention → review` vía `vector spec status`.
- [ ] Gateé la transición de salida con el resultado de validación (en el command).
- [ ] No auto-commiteo (ni command ni implementer).
- [ ] No deleguué a agentes de `~/.dotfiles`.
- [ ] No cambié decisiones tomadas ni el schema del state.
- [ ] Grabé token routing y work trace.
- [ ] Corrí `go -C cli generate`, `gofmt`, `go vet`, `go test`.
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar.
