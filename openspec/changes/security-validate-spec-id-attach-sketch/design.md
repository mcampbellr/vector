# Design — security-validate-spec-id-attach-sketch

## Decisiones clave

- **Guard centralizado `validateSpecID`**: en lugar de un check inline solo en `AttachSketch`, se
  extrae una función `validateSpecID(id string) error` en `cli/internal/state/store.go` que valida
  el `id` contra el patrón canónico de slug. Se reutiliza `state.Slug` / la regex de slug ya
  existente como única fuente de verdad — no se define un patrón nuevo. Devuelve un error claro
  (id no-slug) en caso de mismatch.
- **Validar en el boundary del writer, antes de tocar disco**: `AttachSketch` invoca
  `validateSpecID(id)` **antes** de `sketchesDir(id)` y de cualquier escritura, de modo que un `id`
  con separadores o `..` se rechaza sin efectos de I/O. La seguridad del path deja de depender del
  `ReadSpec(id)` previo.
- **Cobertura de writers hermanos**: cualquier otro método de store que construya un path a partir
  de un `id` provisto por el caller (los que hoy dependen implícitamente de un `ReadSpec` upstream)
  pasa por el mismo `validateSpecID`, para que la defensa sea consistente y no una excepción local
  de `AttachSketch`.
- **Sin cambios en el name sanitizer**: `sanitizeSketchName` ya es correcto y queda intacto. La
  validación del `id` es complementaria, no lo reemplaza — id (directorio) y name (archivo) se
  validan por separado.
- **Alcance defensa-en-profundidad**: no se reescribe el layout de almacenamiento de sketches ni el
  contrato de `AttachSketch`. Es solo un gate de validación adicional en el boundary.

## Superficie

- `cli/internal/state/store.go`:
  - Nuevo `validateSpecID(id string) error` (reutiliza `state.Slug` / la regex de slug).
  - `AttachSketch` → llamar `validateSpecID(id)` antes de `sketchesDir(id)` / cualquier escritura.
  - Writers hermanos que construyen path desde un `id` del caller → mismo guard.
- Test nuevo (junto a `store.go`, p. ej. `store_test.go`): rechazo de `../`, `a/b` y `id` vacío;
  no-regresión de ids válidos.

## Flujo

`AttachSketch(id, name, ...)` → `validateSpecID(id)` (si el `id` no matchea el patrón de slug →
retorna error, **sin** I/O) → `sanitizeSketchName(name)` (comportamiento actual) → `sketchesDir(id)`
resuelve el directorio dentro de `.vector/specs/<id>/` → escritura del sketch. Un `id` legítimo
(slug válido) recorre el flujo sin cambios; un `id` con `/` o `..` se corta en el guard.

## Riesgos / consideraciones

- **Fuente única del patrón**: usar exactamente `state.Slug` / la regex de slug canónica para no
  divergir del validador que ya usa el resto del dominio (evita que un id aceptado en creación sea
  rechazado en `AttachSketch` o viceversa).
- **No romper ids válidos existentes**: los slugs reales de specs deben seguir pasando — el test de
  no-regresión sobre los sketches existentes cubre esto.
- **Consistencia entre writers**: al centralizar, verificar que ningún writer que construya path
  desde `id` quede fuera del guard; de lo contrario la defensa-en-profundidad tendría un hueco.
