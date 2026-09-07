# Tasks — security-validate-spec-id-attach-sketch

## 1. Guard centralizado

- [ ] 1.1 En `cli/internal/state/store.go`, agregar `validateSpecID(id string) error` que valide el
  `id` contra el patrón canónico de slug, reutilizando `state.Slug` / la regex de slug existente
  (sin definir un patrón nuevo).
- [ ] 1.2 Devolver un error claro (id no-slug) en caso de mismatch.

## 2. Boundary en AttachSketch

- [ ] 2.1 En `AttachSketch`, llamar `validateSpecID(id)` **antes** de `sketchesDir(id)` y de
  cualquier escritura a disco; propagar el error en mismatch.
- [ ] 2.2 Mantener intacto `sanitizeSketchName` (validación del name del archivo) — id y name se
  validan por separado.

## 3. Writers hermanos

- [ ] 3.1 Identificar los métodos de store co-localizados que construyen un path desde un `id`
  provisto por el caller y hoy dependen de un `ReadSpec` upstream.
- [ ] 3.2 Encaminarlos por el mismo `validateSpecID` para una defensa consistente (no solo
  `AttachSketch`).

## 4. Verificación

- [ ] 4.1 Test unitario: `id` con `../` es rechazado antes de cualquier escritura.
- [ ] 4.2 Test unitario: `id` con `a/b` (separador) es rechazado.
- [ ] 4.3 Test unitario: `id` vacío es rechazado.
- [ ] 4.4 Test de no-regresión: un `id` de slug válido sigue attachando el sketch correctamente
  (sin cambios en el comportamiento existente de los tests de sketch).
