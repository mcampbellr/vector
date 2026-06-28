# Token Meter Accuracy — provenance de eventos y badge "Estimated"

## Why

El Token Savings Meter del board muestra ahorros de tokens calculados a partir de conteos
auto-reportados por los commands orquestadores (`/vector:raw`, `/vector:bug`). Esos conteos
son estimaciones, no mediciones reales del harness de Claude Code. El board no distingue
entre ambas calidades de dato, lo que expone el wedge comercial de Vector a una pérdida de
credibilidad: si el meter parece exacto cuando no lo es, cualquier discrepancia erosiona la
confianza del usuario. Esta fase introduce **provenance de datos** en el evento `agent.routed`
y propaga esa señal hasta el board para que el meter sea honesto sin sobre-prometer.

## What changes

- Campo `Precision string` (`"actual"` | `"estimated"`) en `AgentRoutedData` (`event.go`),
  serializado con `omitempty`; ausente en eventos anteriores se trata como `"estimated"` en
  runtime (retrocompatibilidad sin migración del log).
- Flag `--precision actual|estimated` en `vector spec route` (`route.go`); default `"estimated"`
  preserva el comportamiento actual de todos los kit commands sin requerirles ningún cambio.
- Validación y normalización de `precision` en `RouteAgent` (`standup.go`): `""` → `"estimated"`;
  valor desconocido → error accionable.
- Campo `Precision string` en `TokenSavings` (`board.go`) con semántica de peor caso: el rollup
  es `"estimated"` si cualquier evento del conjunto no es `"actual"`; `"actual"` solo si todos
  son exactos; `""` si no hay eventos.
- Badge `Estimated` en el Token Savings Meter del panel web cuando
  `tokenSavings.precision === "estimated"`. Neutral (texto gris, sin icono de alerta),
  accesible (`aria-label`), inline con el valor de ahorro.
- Guía de `--precision actual` añadida en el paso de recording de token routing en
  `kit/commands/vector/raw.md` y `kit/commands/vector/bug.md`.
- Copias vendored en `cli/internal/scaffold/assets/commands/vector/` regeneradas vía
  `go generate`.
- `docs/domain-contract.md` §3 actualizado con el campo `precision` y su semántica.

## Scope

- In: `AgentRoutedData.Precision`, flag `--precision`, `TokenSavings.Precision` con rollup de
  peor caso, badge web `Estimated`, guía en kit commands, actualización del domain contract.
- Out: captura automática de tokens del harness de Claude Code, cambios en la fórmula
  `CostUSD`/`SavedUSD` o tabla de precios, badge de precision por spec individual en las cards,
  migración de `activity.jsonl` existente, desglose de provenance evento-a-evento en el board,
  cambios breaking en `GET /api/board`.

Authored spec: `.vector/specs/token-meter-accuracy/spec.md`.
