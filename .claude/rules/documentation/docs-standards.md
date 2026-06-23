# Documentation — Estándares de docs

> Aplica a: cambios en `docs/` y en la documentación del repo de Vector.

## Rol de `docs/`

- `docs/` es la **captura de visión y referencia** del proyecto en su fase actual (sin
  código). Fuente de verdad para el "qué" y "por qué" hasta que existan specs ejecutables.
- Archivos actuales: `docs/vision.md` (visión + preguntas abiertas), `docs/vision-raw.md`
  (idea cruda), `docs/kanban-ui-reference.md` (referencia de UI del board),
  `docs/assets/` (imágenes de referencia).

## Reglas

- **No marcar como decidido lo que el vision deja abierto**: las preguntas abiertas (#1–#4 del
  vision) se respetan; las rules que dependen de ellas llevan marcador `> Estado: pendiente`.
- Cuando una decisión abierta se resuelve, actualizar **el doc fuente** (`docs/vision.md`) y la
  rule que la referencia, en el mismo cambio. Evitar que diverjan.
- Distinguir **captura de visión** (puede ser tentativa) de **spec ejecutable** (compromiso).
  Los specs ejecutables del producto vivirán en el modelo OpenSpec/JSON, no en `docs/`.
- Documentación en español para la conversación/visión; artefactos de git en inglés
  (`workflows/git-convention.md`).

## Mantenimiento

- Al cambiar la estructura del repo o un workspace, revisar si el manifest global
  (`.claude/CLAUDE.md`) y los workspace manifests siguen siendo correctos. No duplicar:
  enlazar a la rule en vez de reescribirla.
