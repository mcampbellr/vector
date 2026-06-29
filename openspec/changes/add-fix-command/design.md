# Design — add-fix-command

## Decisiones clave

- **CLI-owns-writes**: el command orquesta (refiner, clarity gate, transiciones vía `status`,
  implementer, gating, registro, meter/trace); el binario es el único escritor del state. El
  command nunca edita `.vector/` a mano (lectura del state.json sí).
- **`vector spec fix` no transiciona**: solo appendea `spec.fixed`. Las transiciones del ciclo
  van por `vector spec status` (máquina LOCKED) → separación de concerns, sin duplicar lógica
  ni arriesgar deadlock de mutex re-entrante.
- **`FixSpec` modelado sobre `ProposeSpec`**: toma `s.mu` una sola vez, `ReadSpec`, valida status
  corregible, bump `UpdatedAt`, `writeSpecFile`, `appendEvent` (lock ya tomado). **No** llama
  `applyTransition`/`SetStatus`.
- **El command es el gate** de validación: `RESULT.status == "blocked"` o `validation == "fail"`
  → no transiciona a `review` ni llama `spec fix`. `--validation-result` es metadata informativa
  del evento, no un gate del binario.
- **Clasificación = refiner (Haiku)**, no implementer: el barato decide `spec/code/ambos`; el
  caro la respeta (token-routing + separación de responsabilidad).
- **Agentes propios embebidos**: `vector-fix-refiner` (Haiku) + `vector-fix-implementer` (Sonnet),
  vendored y sembrados; no se delega al `/fix` personal del usuario (self-contained, comercial
  día-0).
- **No auto-commit**: el working tree queda para revisión, coherente con `/vector:apply`.
- **Sin cambios de schema**: la corrección vive en eventos (additivo), no en campos nuevos de
  `SpecState`.

## Superficie

- `cli/internal/state/event.go`: `EvtSpecFixed EventType = "spec.fixed"` + `FixedData{Classification,
  ValidationResult, Artifacts, Files}`.
- `cli/internal/state/store.go`: `FixSpec(id, classification, validationResult, artifacts, files,
  actor, now)` — valida status corregible, bump + 1 evento, dentro del mutex; sin transición.
- `cli/cmd/vector/main.go`: `runSpec()` + `case "fix"` → `runSpecFix` con flags
  `--classification` / `--artifacts` / `--files` / `--validation-result` / `--repo-root` / `--json`.
- `kit/commands/vector/fix.md`: orquestación (refiner → clarity → status → implementer → gating →
  fix → route/worklog → reporte sin commit).
- `kit/agents/vector-fix-refiner.md` (Haiku, read-only), `kit/agents/vector-fix-implementer.md`
  (Sonnet) + copias vendored en `scaffold/assets/`.

## Flujo

`/vector:fix <id> [nota]` → lee state.json y valida status corregible → **refina** (Haiku) →
scope guard / clarity gate → **transición de entrada** vía `vector spec status` → **implementa**
(Sonnet) → **gating** del RESULT (command) → **transición de salida** a `review` (si validó) →
`vector spec fix` registra `spec.fixed` → `route`/`worklog` → reporta sin commitear, sugiere
`/vector:close` (si volvió a `review`).
