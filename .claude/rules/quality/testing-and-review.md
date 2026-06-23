# Quality — Tests y review

> Aplica a: todo el repo. Expectativas mínimas de calidad antes de mergear.

## Tests

- **`cli/` (Go)**: paquete `testing` estándar, table-driven donde aplique. Cubrir la lógica de
  estado (lectura/escritura del JSON), detección de repo y la API del board. El manejo del
  estado es crítico: testear invariantes (single source of truth, escritura serializada).
- **`web/` (TS/React)**: testear lógica de proyección de estado y componentes con
  comportamiento no trivial (no snapshots vacíos). Tipos derivados del contrato de la API.
- **`kit/`**: las skills/rules distribuibles se validan por consistencia (kebab-case,
  estructura) y, donde haya lógica, con un caso de ejemplo.

## Gate de calidad

- Go: `gofmt`, `go vet`, linter sin warnings, tests verdes.
- Web: typecheck sin errores, lint sin warnings, build exitoso (necesario para el embed).
- No mergear con tests rojos. Reportar fallos con su salida; no enmascarar.

## Review

- Cambios que cruzan workspaces requieren revisar el impacto en el contrato API (`cli/`↔`web/`)
  y en el esquema del estado.
- Operaciones sobre el repo del usuario: verificar que respetan
  `security/destructive-ops-consent.md`.
- Reusar antes de crear: buscar utilidades/patrones existentes antes de introducir nuevos.

> Estado: pendiente — framework de test de web y umbrales de cobertura se fijan al iniciar
> cada workspace.
