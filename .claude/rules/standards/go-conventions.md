# Standards — Convenciones Go

> Aplica a: `cli/` (CLI + API del board). Hereda el global del usuario (strong typing, naming
> descriptivo, APIs oficiales, sin hacks).

## Layout

- Módulo Go único en `cli/`. Layout idiomático: `cmd/` para entrypoints (binario),
  `internal/` para el código no exportable (comandos, API, estado, detección de repos).
- Un paquete por concern (p. ej. `internal/state`, `internal/board`, `internal/detect`,
  `internal/kit`). Sin paquetes catch-all (`util`, `common`) salvo justificación.

## Estilo

- `gofmt`/`goimports` obligatorio; el código debe pasar `go vet` y un linter
  (`golangci-lint`) sin warnings.
- **Errores explícitos**: envolver con `fmt.Errorf("…: %w", err)` para preservar la cadena;
  nada de `panic` en flujo normal. Errores de cara al usuario del CLI: claros y accionables.
- **Sin `interface{}`/`any`** salvo en fronteras de (de)serialización justificadas. Tipar el
  esquema del estado con structs, no con mapas genéricos.
- Nombres descriptivos (sin variables de una letra, regla global del usuario).
- Contextos (`context.Context`) propagados en operaciones de I/O y de la API HTTP.

## Dependencias

- Preferir la librería estándar. Toda dependencia externa se justifica (mantenimiento activo,
  licencia compatible con distribución comercial — ver `architecture/distribution-packaging.md`).
- El frontend embebido se sirve con `embed.FS`; no añadir frameworks pesados para esto.

## Tests

- Tests con el paquete `testing` estándar; table-driven donde aplique. Ver
  `quality/testing-and-review.md`.

> Estado: pendiente — versión de Go objetivo y elección de librería para CLI (p. ej. `cobra`
> vs stdlib `flag`) y router HTTP se deciden al iniciar `cli/`.
