# Design — conditional-apply-model

## Decisiones clave

- **Nombre `applyModel`, no `applyTier` ni `implementModel`**: consistente con la convención
  del campo hermano `applyMode`. El nombre refleja qué se configura (el modelo para el paso
  de apply), no la categoría abstracta.

- **Tres valores, no booleano**: `"opus"` fuerza Opus, `"sonnet"` fuerza Sonnet,
  `"conditional"` evalúa el criterio. Un booleano no cubre el caso `"siempre Opus sin
  evaluar"` ni `"siempre Sonnet"`. Los tres valores son ortogonales y coherentes con la
  semántica de `applyMode`.

- **Patrón `ApplyMode` replicado exactamente**: tipo string, constantes, `Valid()`,
  campo en `Config`, `Resolved*()`, validación en `Load()`, `omitempty` en el tag JSON.
  Sin desviación del patrón — garantiza coherencia del dominio config y facilita revisión.

- **`omitempty` en el campo JSON**: configs legacy sin `applyModel` cargan sin error y sin
  migración de schema. `ResolvedApplyModel()` retorna `ApplyModelOpus` cuando el campo está
  vacío o es inválido — el default es siempre conservador.

- **Fresh agent para Sonnet, no fork**: un fork hereda el modelo del padre (Opus) y el
  parámetro `model` se ignora. Para rutear al tier Sonnet se usa `Agent({ model: "sonnet" })`
  con un fresh agent; los artefactos se pasan por path en el prompt, no por valor en el
  contexto.

- **Dispatcher delgado**: el command `apply.md` solo evalúa el criterio y despacha; no carga
  el contenido de los artefactos en su propio contexto. Los paths del change ya están
  disponibles al final de `§3` (detect mode) y se reutilizan directamente en el brief del
  subagente.

- **Fallback conservador**: cualquier ambigüedad en el criterio → Opus. Señales ambiguas,
  artefactos ausentes o `tasks.md` vacío → Opus. El riesgo de degradar a Sonnet en un cambio
  arquitectónico supera el coste de un falso negativo (quedarse en Opus).

- **CLI-owns-writes invariant**: el agente `vector-apply-impl` nunca escribe en `.vector/`,
  no llama al binario `vector` y no hace commits. Solo modifica archivos del repo del usuario
  bajo `repo_root`. El command principal es el único que llama al binario para transiciones
  y worklog.

- **Criterio N=5 como punto de partida**: el umbral de archivos es provisional, pendiente de
  calibración post-observación real. Las otras cuatro dimensiones (contratos API, tipos de
  dominio, dependencias, decisiones abiertas) son señales cualitativas evaluadas por el agente.

- **`applyModel` en `runSpecNext`, no en `vector context`**: el command ya consume el JSON de
  `next`; agregar el campo ahí es el cambio mínimo posible y consistente con `applyMode`.

- **Sin nuevo event type**: el tier de implementación no es un evento de dominio del spec. Se
  anota opcionalmente en el `--note` del worklog para trazabilidad.

## Superficie afectada

- `cli/internal/config/config.go` — tipo `ApplyModel`, constantes, campo, `Valid()`,
  `ResolvedApplyModel()`, validación en `Load()`.
- `cli/cmd/vector/spec_transitions.go` — `runSpecNext()`: campo `"applyModel"` en JSON y
  salida humana.
- `kit/commands/vector/apply.md` — nueva `§3a` (evaluación de tier) + `§4` condicional.
- `kit/agents/vector-apply-impl.md` — agente Sonnet nuevo, alcance estrecho, output JSON.
- `cli/internal/scaffold/assets/commands/vector/apply.md` — copia vendorizada (go generate).
- `cli/internal/scaffold/assets/agents/vector-apply-impl.md` — copia vendorizada (go generate).
- `docs/apply-design.md` — §3 actualizado para reflejar `applyModel`.

No tocados: `web/`, `cli/internal/state/`, `SpecState`, tipos de evento, API HTTP.

## Flujo

`/vector:apply [id]`
→ §1: `vector spec next --json` → JSON con `applyMode` + `applyModel` (resuelto)
→ §2: validación draft/selección del work-item (sin cambio)
→ §3: detect mode delegate/native (sin cambio)
→ **§3a (NUEVO):** leer `applyModel` del JSON de `next` (o de `.vector/config.json` en
  continuaciones directas):
  - `""` / `"opus"` → tier = Opus → continúa a §4 inline
  - `"sonnet"` → tier = Sonnet → despacha a `vector-apply-impl` (fresh agent, model: sonnet)
  - `"conditional"` → evalúa criterio mecánico contra artefactos del change:
    - mecánico (todos los puntos) → Sonnet → despacha a `vector-apply-impl`
    - arquitectónico o ambiguo → Opus → continúa a §4 inline
    - sin artefactos evaluables → Opus (fallback conservador)
→ **§4 (MODIFICADO):** si tier = Sonnet, omitir (ya delegado); si Opus, implementa inline
→ §5 worklog: consume JSON del agente (si Sonnet) o el resultado inline (si Opus)
→ §6: detect blocker + transition (sin cambio)
→ §7: summary, §8: report — tier y razón visibles en el reporte final

## Criterio mecánico (cinco dimensiones)

| Dimensión | Señal mecánica | Señal arquitectónica |
|---|---|---|
| Alcance de archivos | ≤5 paths/nombres distintos en los artefactos | >5 archivos o alcance no cuantificable |
| Contratos API/HTTP | sin cambios a endpoints, response bodies ni rutas | agrega, modifica o elimina endpoints |
| Tipos de dominio | no toca `SpecState`, `Config`, tipos de evento, state machine | modifica estructuras del dominio o del state |
| Dependencias | sin imports no triviales ni librerías nuevas | agrega libs o dependencias externas |
| Decisiones abiertas | ninguna alternativa o trade-off pendiente en `design.md` | `design.md` tiene decisiones pendientes o alternativas |

Señal ambigua en cualquier dimensión → arquitectónico (conservador).

## Brief para `vector-apply-impl` (entrada vía prompt)

```
spec_id: <id>
proposal: /abs/path/openspec/changes/<id>/proposal.md
design: /abs/path/openspec/changes/<id>/design.md
tasks: /abs/path/openspec/changes/<id>/tasks.md
repo_root: /abs/path
build_cmd: <cmd>
test_cmd: <cmd>
mode: delegate | native
openspec_change: <id>   # solo en modo delegate
```

En modo nativo sin `tasks.md`, el campo `tasks` se omite y se incluye `spec_doc` con el path
al spec doc del state.

## Output de `vector-apply-impl` (JSON estructurado)

```json
{
  "files_changed": ["..."],
  "tasks_completed": ["..."],
  "tasks_pending": ["..."],
  "build_passed": true,
  "test_passed": true,
  "blocked": false,
  "note": "..."
}
```

Si `"blocked": true`: `note` describe el bloqueador según §6a de `apply.md`.
Si error no recuperable: todas las listas vacías, ambos booleanos false, `note` con el error.
El command consume este JSON para `vector spec worklog` y §6a.
