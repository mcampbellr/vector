# Conditional apply model

## Why

`/vector:apply` siempre implementa en Opus independientemente del tipo de cambio. Cambios
mecánicos (wiring, CRUD, edits localizados en ≤5 archivos sin tocar contratos públicos) no
requieren la capacidad de razonamiento de Opus, pero pagan su precio de tokens y latencia.
El dev no tiene mecanismo para expresar que quiere enrutar la implementación al tier más
económico que preserve la calidad requerida, ni para automatizar esa decisión basada en las
señales del change.

## What changes

- Nuevo tipo `ApplyModel` (`opus` / `sonnet` / `conditional`) con sus constantes, `Valid()` y
  `ResolvedApplyModel()` en `cli/internal/config/config.go`, siguiendo el patrón exacto de
  `ApplyMode`. El campo es `omitempty` — configs sin él mantienen comportamiento actual.
- Validación en `Load()`: si `applyModel` está presente y no es válido, error accionable.
- `vector spec next --json` expone `"applyModel"` (valor resuelto, nunca vacío) junto a
  `"applyMode"`; la salida humana agrega `[applyModel: ...]`.
- `kit/commands/vector/apply.md` recibe una nueva sección `## 3a. Evalúa el tier del modelo`
  que despacha condicionalmente según el valor de `applyModel`:
  — `opus` / `""`: implementa inline (Opus), sin cambio alguno al flujo actual.
  — `sonnet`: despacha siempre a `vector-apply-impl` (Sonnet) sin evaluar criterio.
  — `conditional`: evalúa las cinco señales del criterio mecánico contra los artefactos del
  change (alcance de archivos, contratos API, tipos de dominio, dependencias, decisiones
  abiertas) y despacha a Sonnet si el cambio es mecánico, o implementa inline si no.
- Nuevo agente `kit/agents/vector-apply-impl.md` (model: sonnet, alcance estrecho): recibe
  un brief estructurado con paths a artefactos del change, implementa, corre el gate de
  build/test y retorna JSON estructurado. No transiciona estado ni hace commits.
- Assets vendorizados en `cli/internal/scaffold/assets/` vía `go generate`.

## Scope

**In:**
- Tipo `ApplyModel` + `Valid()` + `ResolvedApplyModel()` + campo `omitempty` + validación en `Load()`.
- Exposición de `"applyModel"` en `vector spec next --json` y salida humana.
- Sección `§3a` y `§4` condicional en `kit/commands/vector/apply.md`.
- Agente `vector-apply-impl.md` (model: sonnet, output JSON).
- Copia vendorizada de ambos artefactos del kit en `cli/internal/scaffold/assets/`.
- Tests de config (table-driven: `Valid`, `ResolvedApplyModel`, `Load` con inválido, backward-compat).
- Actualización de `docs/apply-design.md` §3 para reflejar el nuevo campo.

**Out:**
- Activación por defecto: `applyModel` es opt-in; `vector init`/`update` no incluye el campo.
- Token-meter / evento `agent.routed` por tier (TBD, pendiente de aclarar mecanismo).
- UI web: sin visualización del tier en el board ni en las cards.
- Soporte de un cuarto tier (Haiku): solo `opus`, `sonnet`, `conditional` en esta fase.
- Cambios a `SpecState`, tipos de evento, máquina de estados, API HTTP del board.
- Modificaciones a `vector-standup-writer`, `vector-summary-writer` u otros agentes del kit.

Authored spec: `.vector/specs/conditional-apply-model/spec.md`.
