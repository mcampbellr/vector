# Design — token-meter-accuracy

## Decisiones clave

- **Default `"estimated"`, no `"actual"`**: el comportamiento vigente de los kit commands es
  auto-reportar estimaciones. Invertir el default implicaría que los events actuales quedarían
  marcados como exactos sin serlo — exactamente lo que esta feature quiere evitar. El opt-in
  explícito `--precision actual` existe para cuando el harness exponga la señal real; hasta
  entonces, los commands no necesitan modificarse para mantener su comportamiento correcto.

- **Peor caso en el rollup**: un solo evento `"estimated"` marca el meter completo como
  `"estimated"`. Un meter mixto (algunos exactos, algunos estimados) no puede presentarse como
  exacto sin engañar. La política más conservadora es la más honesta, y la honestidad es el
  requisito del wedge comercial.

- **Retrocompatibilidad sin migración**: `activity.jsonl` es append-only (invariante del
  sistema). Los eventos anteriores sin campo `Precision` se deserializan con `""`, que el
  rollup trata como `"estimated"`. No se reescribe el log; la retrocompatibilidad se maneja
  íntegramente en el rollup.

- **`omitempty` en `AgentRoutedData.Precision`**: consistente con `WorkLoggedData.Change` y
  otros campos opcionales del proyecto. Los eventos `"estimated"` (caso típico) no serializan
  el campo → log más compacto; el rollup ya asume `"" → estimated`. Los eventos `"actual"` sí
  lo serializan, siendo los únicos que aportan semántica distinta al default.

- **Validación en `RouteAgent`, no solo en el flag parser**: `RouteAgent` es una función
  pública. La validación de `precision` vive en la función para cubrir cualquier consumidor
  futuro que la llame directamente, no solo `runSpecRoute`.

- **Badge neutral, no alarmante**: `Estimated` en tipografía gris/slate, inline con el valor
  (`$12.34 saved · Estimated`). No hay icono de advertencia, ni rojo, ni naranja. Es una
  aclaración de calidad de dato, no un error. El estado "sin badge" implica exactitud sin
  necesidad de un label `Actual`.

- **`EventVersion` sin cambio**: el campo es aditivo en el struct; la deserialización JSON
  de Go ignora campos ausentes. No hay migración de schema.

- **Sin `precision` en `Card.SavedUSD` del board**: granularidad por spec innecesaria en V1.
  Solo el rollup global lleva el badge. Simplifica implementación y UX.

## Superficie afectada

| Capa | Archivo | Cambio |
|---|---|---|
| Datos | `cli/internal/state/event.go` | `AgentRoutedData.Precision string` con `omitempty` |
| Lógica | `cli/internal/state/standup.go` | `RouteAgent` + parámetro `precision`, normalización, validación |
| Proyección | `cli/internal/board/board.go` | `TokenSavings.Precision` + lógica de peor caso en `rollupSavings` |
| CLI | `cli/cmd/vector/route.go` | Flag `--precision actual\|estimated`; campo en `--json` output |
| Kit | `kit/commands/vector/raw.md` | Guía `--precision actual` en step 10 |
| Kit | `kit/commands/vector/bug.md` | Ídem |
| Embed | `cli/internal/scaffold/assets/commands/vector/raw.md` | Regenerar vía `go generate` |
| Embed | `cli/internal/scaffold/assets/commands/vector/bug.md` | Regenerar vía `go generate` |
| Web | Token Savings Meter component | Badge `Estimated` condicional |
| Docs | `docs/domain-contract.md` | Campo `precision` en shape de `agent.routed` |

No se crean archivos nuevos. Todos los cambios son modificaciones de archivos existentes.

## Flujo

```
/vector:raw o /vector:bug
  → ejecuta subagente
  → obtiene (o estima) conteo de tokens
  → vector spec route <id> --model haiku --baseline opus
      --tokens-in N --tokens-out M [--precision actual]
  → runSpecRoute: parsea --precision (default "")
  → RouteAgent: normaliza "" → "estimated"; valida; AgentRoutedData{Precision}
  → append evento en activity.jsonl
  → board.Build → rollupSavings: itera eventos, peor-caso
  → TokenSavings{Precision: "estimated"|"actual"|""}
  → GET /api/board expone tokenSavings.precision
  → frontend: precision === "estimated" → badge "Estimated"
```

## API contract (aditivo)

El campo `precision` se agrega al objeto `tokenSavings` ya existente en `GET /api/board`.
No hay breaking change: consumidores que no conocen el campo lo ignoran.

```json
{
  "tokenSavings": {
    "totalSavedUsd": 12.3456,
    "routes": 14,
    "precision": "estimated"
  }
}
```

Valores: `"estimated"` (caso típico V1), `"actual"` (futuro, cuando el harness exponga señal
real), `""` / ausente (meter vacío, sin badge).

## Open questions (heredadas del spec)

1. ¿El harness de Claude Code expone conteos reales al orquestador (env var, output JSON de
   `spawn`, metadata del tool result)? — TBD. Esta fase prepara la infraestructura; la captura
   automática es out of scope.
2. Ruta exacta del componente Token Savings Meter en `web/src/` — el agente implementador debe
   localizarla buscando usos de `tokenSavings` / `totalSavedUsd` antes de editar.
