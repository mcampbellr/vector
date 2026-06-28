# Spec: Modelo condicional para `/vector:apply` (`applyModel`)

## 1. Objetivo

Construir el mecanismo de selección de tier de modelo para `/vector:apply`: un campo opt-in
`applyModel` en `.vector/config.json` que permite rutear el paso de implementación a Sonnet
cuando el cambio es mecánico, manteniendo Opus como default (sin regresión).

Esta feature permite que un dev configure Vector para que detecte automáticamente si una tarea
de apply es mecánica (wiring, CRUD, edits localizados) o arquitectónica (contratos públicos,
APIs, domain model), y ejecute el paso de implementación en el tier más económico que preserve
la calidad requerida. La activación es explícita (opt-in): un config sin el campo se comporta
exactamente igual que hoy.

## 2. Alcance

### Incluido en esta fase

- Nuevo tipo `ApplyModel` y campo `applyModel` en `cli/internal/config/config.go`, con
  `ResolvedApplyModel()` que retorna `ApplyModelOpus` por defecto.
- Validación de `ApplyModel` en `Load()` — misma semántica que `DefaultTicketProvider` y
  `ApplyMode`: si el campo está presente pero no es válido, error accionable.
- Exposición de `applyModel` en el output JSON de `vector spec next --json` (junto a
  `applyMode` ya existente).
- Nueva sección **§3a** en `kit/commands/vector/apply.md`: evaluación del criterio mecánico
  **antes** del paso de implementación, con tabla de señales observables.
- Delegación del paso de implementación a un subagente Sonnet (`vector-apply-impl`) cuando
  el tier es Sonnet; implementación inline (Opus) cuando no.
- Nuevo agente `kit/agents/vector-apply-impl.md` (model: sonnet): recibe brief de
  implementación (paths a artefactos del change, repo root, build/test cmds), implementa y
  retorna resultado estructurado en JSON.
- Fallback conservador a Opus cuando los artefactos del change no están disponibles o el
  criterio no puede evaluarse con certeza.

### Fuera de scope

- Activación por defecto: `applyModel` es opt-in; `""` en config → comportamiento actual
  sin cambio (ningún config generado por `vector init`/`update` incluye el campo).
- Cambios al paso de selección (`applyMode`, `vector spec next`) salvo la adición del campo
  `applyModel` al output JSON.
- Token-meter: no se añaden métricas de ahorro por tier en esta fase (derivado de
  `agent.routed` existente; ver Open questions).
- UI web: no hay visualización del tier en el board ni en las cards.
- Soporte multi-tier (Haiku para tasks triviales): solo Opus y Sonnet en esta fase.
- Criterio automático basado en git diff real post-implementación: el criterio se evalúa
  desde los artefactos del change **antes** de implementar, no del diff resultante.
- Modificaciones a `vector-standup-writer`, `vector-summary-writer` u otros agentes del kit.
- Cambios a la máquina de estados, `SpecState`, event types o la API HTTP del board.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca
relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Binario: **Go** (módulo único, stdlib; `cli/`). Sin dependencias externas nuevas.
- Project command: **Markdown** orquestado por Claude (patrón `kit/commands/vector/apply.md`).
- Agente implementador: **Markdown** con frontmatter (patrón
  `kit/agents/vector-standup-writer.md`).

### Versiones relevantes

- Go: `1.26` (de `cli/go.mod`).
- Sin nuevas dependencias externas de Go.

### Patrones existentes a respetar

- `ApplyMode` en `config.go` (líneas 89–112): el patrón tipo → constantes → `Valid()` →
  `Resolved*()` se replica exactamente para `ApplyModel`. No desviarse del patrón.
- `cli/cmd/vector/spec_transitions.go` → `runSpecNext()`: incluye `"applyMode":
  string(mode)` en el mapa JSON; se agrega `"applyModel": string(applyModel)` en el mismo
  lugar.
- **CLI-owns-writes**: el agente `vector-apply-impl` nunca escribe en `.vector/`; solo
  modifica código del repo del usuario. El command principal es el único que llama al binario
  para transiciones de estado (worklog, status, etc.).
- **Token routing declarado**: el tier del agente se declara en el command (`model: sonnet`
  para mecánico, inline Opus para arquitectónico). Ver `product/token-routing.md`.
- Validación en `Load()`: igual que `DefaultTicketProvider` (líneas 191–194): si
  `c.ApplyModel != ""` y `!c.ApplyModel.Valid()`, retornar error accionable.
- `kit/` assets se vendorizan en `cli/internal/scaffold/assets/` vía `go generate`
  (`architecture/distribution-packaging.md`): toda adición a `kit/` requiere su copia en
  `assets/` generada automáticamente.
- Frontmatter de agentes: `name`, `description`, `model`, `tools` (ver
  `kit/agents/vector-standup-writer.md`).

---

## 4. Dependencias previas

- [ ] `/vector:apply` completamente implementado (`kit/commands/vector/apply.md` ✓, binario
  `vector spec apply|status|next|worklog` ✓, `docs/apply-design.md` ✓).
- [ ] `ApplyMode` en `config.go` (existe; es el patrón de referencia para `ApplyModel`).
- [ ] `runSpecNext()` en `cli/cmd/vector/spec_transitions.go` expone `"applyMode"` en JSON
  (existe; referencia para el nuevo campo `"applyModel"`).
- [ ] `kit/agents/vector-standup-writer.md` con frontmatter y estructura (existe; referencia
  para el nuevo agente).
- [ ] `cli/internal/scaffold/assets/agents/` con al menos un agente vendorizado (referencia
  para `vector-apply-impl.md`).

Si alguna dependencia no existe, reportar qué falta. No inventar contratos ni paths.

---

## 5. Arquitectura

### Patrón

Extensión del patrón `ApplyMode` en dos dimensiones: (a) Go — campo en config + tipo +
método resolved + validación + exposición en `runSpecNext`; (b) command — nueva sección §3a
que evalúa el criterio y despacha condicionalmente a un subagente Sonnet para el paso de
implementación, manteniendo el main loop como dispatcher delgado
(`docs/orchestration-review.md` §3).

### Capas afectadas

- **Config** (`cli/internal/config/config.go`): sí — tipo `ApplyModel`, constantes, campo,
  `Valid()`, `ResolvedApplyModel()`, validación en `Load()`.
- **Binario CLI** (`cli/cmd/vector/spec_transitions.go`): sí — `runSpecNext()` incluye
  `"applyModel"` en el JSON de salida.
- **Project command** (`kit/commands/vector/apply.md`): sí — nueva §3a de evaluación de
  tier y §4 condicional (Sonnet subagente o Opus inline).
- **Nuevo agente** (`kit/agents/vector-apply-impl.md`): sí — implementador Sonnet de
  alcance estrecho.
- **web/**: no.
- **state/** (`cli/internal/state/`): no.

### Flujo esperado

1. Dev ejecuta `/vector:apply [id]`.
2. Command selecciona work-item vía `vector spec next --json` (§1 del command, sin cambio).
   El JSON de respuesta incluye ahora `"applyModel"` junto a `"applyMode"`.
3. Command inicia el spec y detecta modo delegate/native (§2–§3 del command, sin cambio).
4. **NUEVO — §3a: Evaluación del tier del modelo:**
   a. Leer `applyModel` del JSON de `vector spec next` ya consumido. En continuaciones
      directas (spec ya `in-progress`, sin pasar por `next`), leer `.vector/config.json`.
   b. Si `""` o `"opus"` → tier = Opus → implementar inline (comportamiento actual).
   c. Si `"sonnet"` → tier = Sonnet → despachar a `vector-apply-impl` (sin evaluar
      criterio).
   d. Si `"conditional"` → leer artefactos del change (`proposal.md`, `design.md`,
      `tasks.md`; o spec doc en modo nativo) y evaluar criterio mecánico:
      - **Mecánico** (TODOS los puntos): (1) ≤5 archivos fuente distintos mencionados en
        los artefactos; (2) ninguna señal de contrato público en los artefactos (ver tabla
        en §3a del command). → tier = Sonnet.
      - **Arquitectónico** (cualquier condición fuera): → tier = Opus inline.
      - **Sin artefactos evaluables** (nativo sin `tasks.md`, spec doc mínimo): → Opus
        (fallback conservador).
5. **Implementar (§4 del command):**
   - Opus inline: paso §4 actual, sin cambio.
   - Sonnet: despachar a `vector-apply-impl` (fresh agent, `model: sonnet`) con brief
     estructurado. El agente implementa y retorna JSON con resultado. El command consume
     ese JSON para §5 (worklog) y §6a (detección de bloqueador).
6. §5 (worklog), §6 (detect blocker + transition), §7 (summary), §8 (report): sin cambio.

### Criterio mecánico — definición explícita

El criterio es evaluación del agente guiada por señales observables (no regex). La tabla
define las señales, no el juicio final — el agente aplica todas las dimensiones:

| Dimensión | Señal mecánica | Señal arquitectónica |
|---|---|---|
| Alcance de archivos | ≤5 paths/nombres de archivo distintos en los artefactos | >5 archivos, o alcance no cuantificable |
| Contratos API/HTTP | no hay cambios a endpoints, response bodies ni rutas | agrega, modifica o elimina endpoints |
| Tipos de dominio | no toca `SpecState`, `Config`, tipos de evento, state machine | modifica estructuras del dominio o del state |
| Dependencias | no agrega imports no triviales ni librerías | agrega libs o dependencias externas |
| Decisiones abiertas | ninguna alternativa o trade-off pendiente en `design.md` | `design.md` tiene decisiones pendientes o alternativas |

Si alguna señal es ambigua → arquitectónico (conservador). La decisión puede anotarse en el
`--note` del worklog para auditoría.

### Ubicación de archivos nuevos/modificados

```txt
kit/
  agents/
    vector-apply-impl.md          ← NUEVO
  commands/vector/
    apply.md                      ← MODIFICAR (§3a + §4 condicional)
cli/
  internal/
    config/
      config.go                   ← MODIFICAR (ApplyModel)
    scaffold/
      assets/
        agents/
          vector-apply-impl.md    ← NUEVO (go generate)
        commands/vector/
          apply.md                ← MODIFICAR (go generate)
  cmd/vector/
    spec_transitions.go           ← MODIFICAR (runSpecNext JSON)
```

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `cli/internal/config/config.go` | MODIFICAR | Tipo `ApplyModel` + constantes + `Valid()` + campo + validación en `Load()` + `ResolvedApplyModel()` | `ApplyMode` (mismo archivo, líneas 89–112) |
| `cli/cmd/vector/spec_transitions.go` | MODIFICAR | `runSpecNext()`: incluir `"applyModel"` en JSON de salida y en salida humana | `"applyMode"` (misma función, líneas 307/315/318) |
| `kit/commands/vector/apply.md` | MODIFICAR | Insertar §3a (evaluación de tier) + §4 condicional (Sonnet subagente o inline) | `kit/commands/vector/propose.md` (detección de modo, dispatch) |
| `kit/agents/vector-apply-impl.md` | NUEVO | Agente Sonnet: recibe brief de implementación, implementa, retorna JSON estructurado | `kit/agents/vector-standup-writer.md` (frontmatter, alcance estrecho, output JSON) |
| `cli/internal/scaffold/assets/commands/vector/apply.md` | MODIFICAR (generado) | Copia vendorizada del command (`go generate`) | sibling `propose.md` en el mismo directorio |
| `cli/internal/scaffold/assets/agents/vector-apply-impl.md` | NUEVO (generado) | Copia vendorizada del agente (`go generate`) | sibling `vector-standup-writer.md` en `assets/agents/` |

### Detalle por archivo

#### `cli/internal/config/config.go` — MODIFICAR

Cambios requeridos:

- Agregar tipo `ApplyModel string` con constantes (después de la sección `ApplyMode`,
  siguiendo la misma estructura):

  ```go
  type ApplyModel string

  const (
      ApplyModelOpus        ApplyModel = "opus"        // siempre Opus (default cuando vacío)
      ApplyModelSonnet      ApplyModel = "sonnet"       // siempre Sonnet
      ApplyModelConditional ApplyModel = "conditional"  // evalúa criterio mecánico en runtime
  )

  func (m ApplyModel) Valid() bool {
      switch m {
      case ApplyModelOpus, ApplyModelSonnet, ApplyModelConditional:
          return true
      }
      return false
  }
  ```

- Agregar campo `ApplyModel ApplyModel` con tag `json:"applyModel,omitempty"` en `Config`,
  después del campo `ApplyMode`. La etiqueta `omitempty` garantiza compatibilidad hacia atrás:
  configs legacy sin el campo cargan sin error.

- Agregar método:
  ```go
  func (c *Config) ResolvedApplyModel() ApplyModel {
      if c.ApplyModel.Valid() {
          return c.ApplyModel
      }
      return ApplyModelOpus
  }
  ```

- En `Load()`: validar `c.ApplyModel` si no está vacío, análogamente a la validación de
  `c.DefaultTicketProvider` (líneas 191–194). Error: `"invalid applyModel %q:
  allowed opus,sonnet,conditional"`.

Restricciones: no cambiar ningún campo existente de `Config`; no alterar `ApplyMode` ni
ningún otro método `Resolved*()`. El campo es `omitempty`: configs legacy cargan sin
migración.

#### `cli/cmd/vector/spec_transitions.go` — MODIFICAR

Cambios requeridos:

- En `runSpecNext()`: después de resolver `mode` (línea 299 aprox.), resolver también
  `applyModel` desde `cfg.ResolvedApplyModel()` cuando el config esté disponible. Incluir
  `"applyModel": string(applyModel)` en el mapa JSON junto a `"applyMode"` (líneas 307 y
  315), y en la salida humana junto a `[applyMode: ...]` (línea 318).
- Cuando no hay work-item (`nothing actionable`), incluir igualmente `"applyModel"` en el
  JSON de respuesta (valor resuelto del config, o `"opus"` si config no disponible).

Restricciones: no cambiar el formato de ningún campo existente; no cambiar ningún otro
subcomando; la carga del config ya existe en `runSpecNext` (líneas 295–301 aprox.).

#### `kit/commands/vector/apply.md` — MODIFICAR

Cambios requeridos:

- Insertar nueva sección **`## 3a. Evalúa el tier del modelo`** entre §3 (Detect the mode)
  y §4 (Implement) actuales. Debe incluir:
  - Instrucción de leer `applyModel` del JSON de `vector spec next` ya consumido (o de
    `.vector/config.json` en continuaciones directas sin `next`).
  - Tabla de dispatch por valor (`"opus"` → inline; `"sonnet"` → Sonnet siempre;
    `"conditional"` → evaluar criterio).
  - Tabla de señales del criterio mecánico (las cinco dimensiones de §5 de este spec).
  - Instrucción explícita de fallback conservador a Opus ante artefactos ausentes o señales
    ambiguas.
  - Cuando tier = Sonnet: despachar a `vector-apply-impl` con brief estructurado; no
    implementar inline; consumir el JSON resultado para §5 y §6a.

- Modificar **`## 4. Implement`**:
  - Añadir al inicio: "Si el tier fue asignado a Sonnet en §3a, omitir esta sección: la
    implementación ya está delegada al subagente."
  - El cuerpo existente permanece intacto para el path Opus inline.

El resto del command (§1, §2, §3, §5, §6, §7, §8) permanece sin cambio alguno.

Restricciones: no cambiar la numeración de §1–§3 ni §5–§8; no auto-commitear (el command
actual no lo hace y este spec no lo cambia); no escribir `.vector/` directamente.

#### `kit/agents/vector-apply-impl.md` — NUEVO

Frontmatter requerido:
```yaml
---
name: vector-apply-impl
description: Implementador de alcance estrecho para /vector:apply — recibe un brief de implementación (paths a artefactos del change, repo root, build/test cmds), implementa el cambio siguiendo tasks.md/proposal.md/design.md, y retorna un JSON estructurado. Corre en Sonnet; spawneado por /vector:apply cuando el criterio mecánico lo indica. No transiciona estado ni hace commits.
model: sonnet
tools: Read, Edit, Write, Bash
---
```

Debe implementar:

- Leer el brief estructurado recibido vía prompt: `spec_id`, paths absolutos a
  `proposal.md`, `design.md`, `tasks.md` (o spec doc en modo nativo), `repo_root`,
  `build_cmd`, `test_cmd`, modo (`delegate`/`native`), `openspec_change` (si delegate).
- Leer los artefactos del change de disco (los paths del brief; omitir los que no existan).
- Implementar el código siguiendo `tasks.md`/`proposal.md`/`design.md` (o spec doc en
  nativo), marcando checkboxes conforme avanza.
- Respetar las convenciones del repo del usuario (agnóstico: no imponer arquitectura).
- Correr el gate de build/test del repo usando `build_cmd`/`test_cmd`; detenerse y reportar
  si falla.
- Detectar bloqueadores externos (mismos tres signals del §6a de `apply.md`): si detectado,
  `"blocked": true` en el resultado.
- **NO** llamar al binario `vector` para transiciones de estado.
- **NO** hacer git commits.
- **NO** editar `.vector/`.
- Retornar SOLO un JSON en la forma descrita en §7.

Seguir como referencia:
- `kit/agents/vector-standup-writer.md` (frontmatter, hard rules, output JSON estricto).
- `kit/commands/vector/apply.md` §4–§6a (lógica de implementación que el agente hereda).

No debe incluir: selección de work-item, detección de modo, worklog, transición de estado,
resumen post-acción. Solo implementa.

---

## 7. API Contract

Sin nueva API surface HTTP — no aplica para esta feature. Las interfaces relevantes son:

**Output de `vector spec next --json` (adición):**

```json
{
  "id": "add-foo",
  "status": "open",
  "priority": "normal",
  "title": "Add foo feature",
  "applyMode": "ask",
  "applyModel": "conditional"
}
```

`applyModel` siempre presente en el JSON (nunca vacío: si config vacío o inválido →
`"opus"`, por `ResolvedApplyModel()`). Cuando no hay work-item, el campo se incluye igual
(valor resuelto del config).

**Salida humana de `vector spec next` (adición):**

```
next: add-foo  (open · normal)  [applyMode: ask | applyModel: conditional]
  Add foo feature
```

**Brief para el agente `vector-apply-impl` (entrada, via prompt):**

```
spec_id: add-foo
proposal: /abs/path/openspec/changes/add-foo/proposal.md
design: /abs/path/openspec/changes/add-foo/design.md
tasks: /abs/path/openspec/changes/add-foo/tasks.md
repo_root: /abs/path
build_cmd: go build ./...
test_cmd: go test ./...
mode: delegate
openspec_change: add-foo
```

En modo nativo sin `tasks.md`, el campo `tasks` se omite y se incluye `spec_doc` con el
path al spec doc del state.

**Resultado del agente `vector-apply-impl` (salida JSON):**

```json
{
  "files_changed": ["cli/internal/config/config.go", "cli/cmd/vector/spec_transitions.go"],
  "tasks_completed": ["Add ApplyModel type", "Add ResolvedApplyModel()"],
  "tasks_pending": ["Update vendored assets"],
  "build_passed": true,
  "test_passed": true,
  "blocked": false,
  "note": "Implemented ApplyModel type + field + ResolvedApplyModel(); 2 files touched. Gate green."
}
```

Si `"blocked": true`, el campo `note` describe el bloqueador con la forma prescrita por §6a
de `apply.md` (qué está pendiente + cómo desbloquearlo). El command consume este JSON para
`vector spec worklog --files ... --tasks ... --note ...` y para §6a.

Exit del agente: JSON válido en todos los casos. Errores no recuperables → JSON con
`"files_changed": []`, `"tasks_completed": []`, `"build_passed": false`, `"test_passed":
false`, `"blocked": false`, y `note` con la descripción del error.

---

## 8. Criterios de éxito

- [ ] `ApplyModel` tiene tres constantes (`opus`/`sonnet`/`conditional`) con `Valid()`
  correcto (true para los tres, false para cualquier otro valor).
- [ ] `ResolvedApplyModel()` retorna `ApplyModelOpus` cuando `applyModel` está vacío o es
  inválido (sin acceso al config en `runSpecNext`: usar `ApplyModelOpus` como default).
- [ ] `Load()` falla con error accionable ante `applyModel` con valor inválido en JSON.
- [ ] Config legacy (sin campo `applyModel`) carga sin error (`omitempty`).
- [ ] `vector spec next --json` incluye `"applyModel"` con el valor resuelto, nunca vacío.
- [ ] `vector spec next` (salida humana) muestra `[applyModel: ...]` junto a `[applyMode: ...]`.
- [ ] Con `applyModel = ""`: comportamiento de apply idéntico al actual (cero regresión).
- [ ] Con `applyModel = "opus"`: implementación inline (sin subagente `vector-apply-impl`).
- [ ] Con `applyModel = "sonnet"`: despacha siempre a `vector-apply-impl` (sin evaluar
  criterio).
- [ ] Con `applyModel = "conditional"` + cambio mecánico (≤5 archivos, sin señales de
  contrato): despacha a `vector-apply-impl` (Sonnet).
- [ ] Con `applyModel = "conditional"` + cambio arquitectónico: implementa inline (Opus).
- [ ] Con `applyModel = "conditional"` + artefactos ausentes o no evaluables: implementa
  inline (Opus, fallback conservador).
- [ ] Agente `vector-apply-impl` retorna JSON con todas las claves requeridas.
- [ ] El command consume el JSON del agente para worklog y §6a (detección de bloqueador).
- [ ] Sin regresiones: `spec create|list|propose|sync|init|update|close|archive|status`,
  `vector serve`, `vector standup` siguen funcionando sin cambio.

### Tests requeridos

- [ ] `cli/internal/config/config_test.go`: `ResolvedApplyModel()` retorna `ApplyModelOpus`
  para `""`, para `"opus"` explícito, y ante valor inválido (table-driven).
- [ ] `cli/internal/config/config_test.go`: `ApplyModel.Valid()` retorna true para los tres
  valores válidos y false para `""`, `"haiku"`, `"SONNET"`, `"auto"`.
- [ ] `cli/internal/config/config_test.go`: `Load()` retorna error para un config con
  `"applyModel": "haiku"` (inválido).
- [ ] `cli/internal/config/config_test.go`: config sin campo `applyModel` carga
  correctamente — backward-compat (no error, campo vacío en el struct).
- [ ] Test de `runSpecNext` (unit o integración, según patrón del proyecto): el JSON de
  salida incluye `"applyModel"` con el valor resuelto cuando el config tiene el campo, y
  `"opus"` cuando no.

### Comandos de verificación

```bash
gofmt -l cli && go -C cli vet ./... && go -C cli test ./...
```

---

## 9. Criterios de UX

Aplica al command `/vector:apply` y al output del binario (no a UI web):

- **Transparencia de ruteo**: el command reporta en §8 (Report) qué tier se usó y por qué
  (`"routed to Sonnet (conditional: 3 files, no contract signals)"` / `"implemented inline
  (Opus default)"`). Sin visibilidad del tier el resultado no es auditable.
- **Sin prompts adicionales**: la evaluación del criterio y la decisión de tier son
  silenciosas. El command no interrumpe al usuario para confirmar el modelo; la activación
  de la feature es explícita (set manual de `applyModel` en config).
- **Fallback accionable**: si el agente `vector-apply-impl` falla (JSON malformado, crash),
  el command surfacea el error claramente con mensaje del tipo `[apply] impl agent failed:
  <descripción> — retry with applyModel: opus`. El spec permanece en su estado actual; el
  worklog no se appenda (no hay resultado para consumir).
- **Continuaciones**: si el spec ya está `in-progress` y se retoma sin pasar por `next`,
  el tier se evalúa de la misma manera leyendo el config directamente — sin diferencia de
  comportamiento observable para el usuario.
- **Sin sorpresas en el path Opus**: cuando `applyModel` está vacío o es `"opus"`, el
  command no menciona el tier en ningún lugar. El comportamiento es idéntico al actual.
- **Accesibilidad**: el tier elegido y la razón son visibles en texto plano en el reporte
  final (§8 del command); el resultado JSON del subagente no se imprime completo en el
  reporte (solo el resumen del worklog).

---

## 10. Decisiones tomadas

- **Nombre `applyModel`, no `applyTier` ni `implementModel`**: consistente con la
  convención de campo del mismo dominio (`applyMode`). Nombrar el campo por la decisión que
  configura (qué modelo para apply), no por la categoría abstracta.
- **Tres valores, no booleano**: `"opus"` (fuerza Opus), `"sonnet"` (fuerza Sonnet),
  `"conditional"` (evalúa). El booleano `"force-sonnet"` no cubre el caso `"siempre Opus
  sin evaluar"` y el booleano `"enable-routing"` no cubre `"siempre Sonnet"`. Tres valores
  ortogonales son más expresivos y coherentes con la semántica de `applyMode`.
- **Criterio N=5 archivos como punto de partida**: límite inicial pendiente de calibración
  post-observación. Ver Open questions.
- **Fresh agent para Sonnet, no fork**: el fork hereda el modelo del padre (Opus) y el
  parámetro `model` se ignora en forks (`docs/orchestration-review.md` §3). Para rutear
  al tier Sonnet se usa `Agent({ model: "sonnet", ... })` con un fresh agent (pasa los
  paths de artefactos en el prompt; el agente los lee desde disco).
- **Sin nuevo event type**: el tier de implementación no es un evento de dominio del spec
  (no cambia el estado del board). Se anota en el `--note` del worklog si el dev lo desea.
- **`omitempty` en JSON**: el campo no aparece en configs legacy → backward-compatible sin
  migración de schema.
- **Fallback conservador**: cualquier ambigüedad en el criterio → Opus. Nunca degradar a
  Sonnet en caso de duda (el riesgo de calidad en cambios arquitectónicos es el único riesgo
  medio de la feature, según `docs/orchestration-review.md` §15 y §14 fila #7).
- **`applyModel` en `runSpecNext`, no en `vector context`**: el campo ya existe en el
  config; exponerlo en `next` (que el command ya consume) es el menor cambio posible y
  consistente con `applyMode`.

Si el agente ve una alternativa mejor, la reporta como observación; no la implementa.

---

## 11. Edge cases

### Evaluación del criterio (modo `conditional`)

- **Sin artefactos** (nativo sin `tasks.md`, spec doc mínimo sin tasks): criterio no
  evaluable → fallback Opus. El command lo menciona en el reporte.
- **`tasks.md` vacío** (cero items): no hay archivos que contar → criterio (1) es
  trivialmente ≤5 pero no informativo; en ausencia de información real → Opus (conservador).
- **Señales ambiguas**: p. ej. `proposal.md` menciona "API" en contexto de consumidor
  (el cambio *usa* una API, no la *modifica*) → señal arquitectónica → Opus (falso negativo
  aceptable; la mitigación está en la conservatividad del criterio, no en su recall).
- **>5 archivos en un solo paquete**: el count es por path de archivo, no por paquete.
  Sin excepción al criterio de alcance.
- **Artefactos en worktree distinto al main**: los paths absolutos del brief deben
  resolverse en el mismo worktree donde viven los artefactos (ya resuelto en §3 detect mode
  del command). Verificar al implementar que los paths del brief sean los mismos que los del
  change actual. TBD — ver Open questions.

### Fallo del subagente `vector-apply-impl`

- **JSON malformado o incompleto**: el command no intenta parsear parcialmente; surfacea el
  error con mensaje accionable; el spec permanece en su estado actual; sugiere reintentar con
  `applyModel: opus`.
- **El agente sale con error no recuperable**: idem al anterior; el worklog no se appenda.
- **El agente retorna `"blocked": true`**: el command trata esto exactamente como una señal
  §6a → `vector spec status <id> needs-attention --reason "<note del agente>"`. No error.
- **Gate de build/test falla dentro del agente** (`build_passed: false` o `test_passed:
  false`): el agente lo incluye en `note`; el command lo surfacea en el reporte antes de
  la transición y NO transiciona a `review`. El spec queda en `in-progress`.

### Config inválido

- **`applyModel: "haiku"` en config**: `Load()` retorna error accionable antes de llegar
  al command. El binario nunca inicia el apply.
- **`applyModel: "CONDITIONAL"` (mayúsculas)**: inválido → error en `Load()`. Los valores
  son lowercase; no hay normalización (igual que `ApplyMode`).
- **`applyModel` ausente + `applyMode` presente**: backward-compat. `ResolvedApplyModel()`
  retorna `"opus"`; la lógica de selección (`applyMode`) no cambia.

### Continuación (`in-progress`)

- **Spec ya `in-progress`, `applyModel` cambió en config desde el último run**: el nuevo
  valor se respeta en la siguiente corrida. El tier no se cachea por-spec (se lee el config
  actual cada vez).
- **Spec ya `in-progress`, artefactos del change cambiaron entre runs**: re-evalúa el
  criterio con el estado actual de los artefactos. El tier puede cambiar entre continuaciones
  (deliberado: refleja el estado real del change).

### Sin HTTP surface

Los códigos HTTP (400/401/403/404/409/422/429/500) no aplican a este command.

---

## 12. Estados de UI requeridos

Estados observables del command durante el paso de evaluación y despacho (no UI web):

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| evaluating-tier | Silencioso — lectura de artefactos y evaluación del criterio (<200ms) | Esperar |
| routed-sonnet | `[apply] routing to Sonnet (mechanical: N files, no contract signals)` | Esperar al subagente |
| routed-opus | Silencioso — implementación inline, comportamiento actual | Esperar |
| fallback-opus | `[apply] no evaluable artifacts — falling back to Opus` | Esperar |
| agent-running | Subagente `vector-apply-impl` implementando (visible por el harness) | Esperar |
| agent-success | JSON consumido; continúa con §5 worklog + §6 transition | Continuar flujo |
| agent-failed | `[apply] impl agent failed: <desc> — retry with applyModel: opus` | Corregir y reintentar |
| blocked | Bloqueador externo (`"blocked": true`) → `needs-attention` (§6a del command) | Proveer dependencia + `/vector:apply <id>` para retomar |
| disabled | `applyModel = ""` o `"opus"` — no hay evaluación ni subagente | — (comportamiento actual) |
| offline | No aplica — CLI local-only, sin dependencia de red | — |

---

## 13. Validaciones

### Validaciones de config (Go, en `Load()`)

| Campo | Regla | Error |
|---|---|---|
| `applyModel` | si presente: uno de `opus`, `sonnet`, `conditional` (case-sensitive) | `invalid applyModel %q: allowed opus,sonnet,conditional` |

### Validaciones del command (§3a)

| Entrada | Regla | Comportamiento |
|---|---|---|
| `applyModel` de `vector spec next --json` | siempre presente (binario lo resuelve) | si vacío por error inesperado → defensivo: Opus |
| Artefactos del change (`tasks.md`, etc.) | evalúa solo si `conditional` y archivos accesibles | si inaccesibles → Opus |
| JSON resultado del subagente | estructura válida con todas las claves requeridas | si malformado → error accionable, spec en `in-progress` |

### Validaciones del agente `vector-apply-impl`

- Recibe brief: si falta una clave requerida → retorna JSON de error con `note` describiendo
  la clave faltante, resto de campos con valores vacíos/false. No aborta sin retornar JSON.
- Paths de artefactos: si un archivo no existe (p. ej. sin `design.md`) → lo omite y
  continúa con los disponibles. `tasks.md` ausente se nota en `note`.
- El agente nunca llama al binario `vector`; si detecta que lo necesita → no lo hace,
  anota en `note` que no es responsabilidad suya.

---

## 14. Seguridad y permisos

- El agente `vector-apply-impl` **nunca escribe en `.vector/`** (CLI-owns-writes). Solo
  modifica archivos del repo del usuario bajo `repo_root`.
- El brief al subagente incluye paths de artefactos del change, no el contenido de
  `state.json` completo (puede contener paths internos o datos de tickets sensibles).
- El `note` del resultado del subagente se persiste en `activity.jsonl` vía `vector spec
  worklog`: no incluir secrets, tokens, ni valores de credenciales. Describir el bloqueo
  sin el valor (p. ej. `"missing Stripe secret key"`, no el valor).
- Sin dependencias de red en el agente: opera solo sobre el filesystem local y comandos del
  repo. Si un test o build requiere red y falla → el gate falla; reportar como fallo de
  gate, no como error del agente.
- `go generate` y `//go:embed all:assets` — sin cambio al mecanismo existente de distribución
  (`architecture/distribution-packaging.md`).
- El nuevo campo `applyModel` en `config.json` no expone información sensible; es una
  preferencia de configuración.

---

## 15. Observabilidad y logging

Usar `activity.jsonl` existente vía `vector spec worklog` y `vector spec route`:

- El `--note` del worklog debe incluir el tier usado y la razón cuando tier ≠ Opus por
  defecto: `"implemented on sonnet (conditional: 3 files, no contract signals)"` o
  `"implemented on sonnet (applyModel: sonnet, forced)"`. Cuando es Opus inline, el note
  puede omitir el tier (silencioso, comportamiento actual).
- Si `"blocked": true` en el resultado del subagente → el `--reason` de `vector spec
  status needs-attention` describe el bloqueador según §6a del command, sin cambio.

No loggear:

- Contenido completo de los artefactos del change.
- Paths internos de `.vector/` no relevantes para el diagnóstico.
- Credenciales o tokens del entorno de CI/CD.

Telemetría del token-meter: TBD — ver Open questions (si el spawn de `vector-apply-impl`
genera un evento `agent.routed` automáticamente o si el command debe emitir `vector spec
route --model sonnet <id>` explícitamente).

---

## 16. i18n / textos visibles

**El proyecto no tiene sistema de i18n.** El binario y el command emiten strings en inglés
hardcodeado, consistente con el resto del CLI. La tabla es documentación de los strings, no
keys de un archivo de traducciones.

| Identificador (doc) | Texto |
|---|---|
| apply.tier.routed_sonnet | `[apply] routing to Sonnet (mechanical: {N} files, no contract signals)` |
| apply.tier.fallback_opus | `[apply] no evaluable artifacts — falling back to Opus` |
| apply.tier.agent_failed | `[apply] impl agent failed: {err} — retry with applyModel: opus` |
| config.invalid_apply_model | `invalid applyModel "{v}": allowed opus,sonnet,conditional` |
| next.json.apply_model | (clave JSON) `applyModel` |
| next.human.apply_model | `[applyModel: {v}]` (parte de la línea de salida humana) |

---

## 17. Performance

- **Cero overhead cuando `applyModel = ""`**: ninguna evaluación, ningún subagente, ningún
  cambio en el path crítico. `ResolvedApplyModel()` es un switch sobre una string.
- **Evaluación del criterio** (`conditional`): 2–3 lecturas de archivo (artefactos del
  change) + evaluación del agente → <200ms overhead antes de implementar. Los artefactos
  ya están disponibles en el contexto del command desde §3 (detect mode), por lo que el
  comando no los re-lee desde cero: pasa los paths al brief del subagente.
- **Ahorro en Sonnet**: el paso de implementación en Sonnet para cambios mecánicos es
  típicamente 2–4× más barato y más rápido que en Opus para el rango esperado (≤5 archivos,
  ~3–10k tokens de implementación). Este ahorro es el beneficio primario de la feature
  (`docs/orchestration-review.md` §8, fila #7: "ahorro alto").
- **Sin I/O redundante**: el brief para el subagente reutiliza los paths ya conocidos del
  change; el main loop no carga los contenidos de los artefactos en su propio contexto (pasan
  por path entre el command y el subagente, siguiendo el patrón del dispatcher delgado de
  `docs/orchestration-review.md` §4).

---

## 18. Restricciones

El agente no debe:

- Activar `applyModel` por defecto en ningún config generado por `vector init` o `vector
  update` (el campo no debe aparecer en el template de config inicial).
- Agregar el campo `applyModel` al output de ningún subcomando salvo `vector spec next`.
- Crear un cuarto valor de `ApplyModel` (p. ej. `"haiku"`): solo `opus`, `sonnet`,
  `conditional` en esta fase.
- Hacer que el agente `vector-apply-impl` llame al binario `vector`, haga git commits o
  edite `.vector/` (CLI-owns-writes invariant).
- Cambiar el comportamiento de `applyMode` (selección del work-item): es ortogonal a
  `applyModel` (tier de implementación).
- Modificar `SpecState`, tipos de evento en `state/event.go`, ni la máquina de estados.
- Instalar dependencias externas de Go.
- Modificar los §1, §2, §3, §5, §6, §7 y §8 del command `apply.md` salvo lo descrito
  explícitamente en §6.
- Cambiar la semántica o el nombre de ninguna constante de `ApplyMode` existente.

**Permitido y necesario:**
- Actualizar el string de uso de `runSpec()` / `usage()` en `main.go` si es necesario para
  reflejar `applyModel` en la documentación inline.
- Agregar el campo `applyModel` al stub `Config{}` de cualquier test que ya testea
  `runSpecNext`.
- Actualizar `docs/apply-design.md` para reflejar la adición del campo `applyModel` (§3 de
  ese doc, "Config `applyMode` en `.vector/config.json`").

---

## 19. Entregables

- [ ] Tipo `ApplyModel` + constantes + `Valid()` + campo + `ResolvedApplyModel()` en
  `config.go`.
- [ ] Validación de `applyModel` en `Load()` con error accionable.
- [ ] `vector spec next --json` incluye `"applyModel"` con valor resuelto; salida humana
  incluye `[applyModel: ...]`.
- [ ] Config legacy sin campo carga sin error (backward-compat verificado con test).
- [ ] Tests de config (table-driven: `ResolvedApplyModel`, `Valid`, `Load` con inválido,
  backward-compat).
- [ ] `kit/commands/vector/apply.md` con §3a de evaluación y §4 condicional.
- [ ] `kit/agents/vector-apply-impl.md` (model: sonnet, alcance estrecho, output JSON).
- [ ] Assets vendorizados en `cli/internal/scaffold/assets/` (vía `go generate`).
- [ ] `docs/apply-design.md` §3 actualizado para reflejar `applyModel`.
- [ ] Sin regresiones: `gofmt -l cli`, `go -C cli vet ./...`, `go -C cli test ./...` verdes.

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `docs/apply-design.md` §3 y Open questions, y `docs/orchestration-review.md`
  §8 (fila #7), §14 (fila #7) y §15 (§7).
- [ ] Confirmé que `ApplyMode` existe en `config.go` y lo usé como patrón exacto para
  `ApplyModel` (tipo, constantes, `Valid()`, `Resolved*()`).
- [ ] Confirmé que `runSpecNext()` está en `spec_transitions.go` y ya expone `"applyMode"`
  en el JSON (referencia para el nuevo campo).
- [ ] Confirmé que el frontmatter de `vector-standup-writer.md` es la referencia para el
  nuevo agente `vector-apply-impl.md`.
- [ ] Solo modifiqué los archivos listados en §6 o lo justifiqué.
- [ ] Implementé `Valid()` + `ResolvedApplyModel()` siguiendo el patrón `ApplyMode`.
- [ ] Implementé validación en `Load()` con el patrón `DefaultTicketProvider`.
- [ ] Agregué `"applyModel"` al JSON y salida humana de `runSpecNext()`.
- [ ] Inserté §3a en `apply.md` sin alterar §1–§3 ni §5–§8.
- [ ] Escribí `vector-apply-impl.md` con frontmatter correcto, alcance estrecho, output JSON
  con todas las claves requeridas.
- [ ] Vendorizé los assets nuevos/modificados vía `go generate`.
- [ ] NO activé `applyModel` en ningún config generado por `init`/`update`.
- [ ] NO cambié `ApplyMode`, `SpecState`, event types, máquina de estados.
- [ ] Actualicé `docs/apply-design.md` §3.
- [ ] Ejecuté `gofmt -l cli`, `go -C cli vet ./...`, `go -C cli test ./...`.
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar.

## Open questions

- **N de archivos para el criterio mecánico**: N=5 es el punto de partida. ¿Calibrar basado
  en observación de applies reales post-lanzamiento? TBD — marcar para revisión en P3.
- **Token-meter para el subagente**: ¿el spawn de `vector-apply-impl` genera un evento
  `agent.routed` automáticamente, o el command debe emitir `vector spec route --model sonnet
  <id>` explícitamente para registrarlo en `activity.jsonl`? TBD — verificar el mecanismo
  de `runSpecRoute` en `main.go` y cómo lo invoca `apply.md` actualmente.
- **Paths de artefactos en bare+worktree**: cuando el change vive en un worktree distinto
  al main, los paths absolutos del brief deben resolverse en el worktree correcto. ¿El
  command ya tiene esos paths resueltos al final de §3 (detect mode)? TBD — verificar que
  `openspec.change` + la lógica de `ChangesDir` en `config.go` ya los deja disponibles.
- **Hint en `vector update`**: ¿`vector update` debería informar al usuario que `applyModel`
  está disponible si no está configurado? TBD — no en esta fase; candidato a P3.
