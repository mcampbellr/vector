# `.claude/` — Sistema de instrucciones de Vector

Arquitectura **scoped CLAUDE.md**: contexto pequeño, enfocado y sin duplicación, que escala
con el repo (CLI + API + web + kit).

## Navegación

- **`CLAUDE.md`** — manifest global. Pequeño: qué es Vector, principios cross-repo y el
  **índice de rules**. Empieza por aquí solo para ubicarte.
- **`rules/`** — conocimiento por concern, una responsabilidad por archivo. Cada rule declara
  "Aplica a: …" en su encabezado. Léela solo cuando coincide con tu tarea.
  - `architecture/` · `standards/` · `product/` · `security/` · `quality/` · `workflows/` ·
    `documentation/`
- **Workspace manifests** (`cli/CLAUDE.md`, `web/CLAUDE.md`, `kit/CLAUDE.md`) — cada uno
  declara su stack/rol y **enlaza** las rules relevantes (no las copia). **Trabajando dentro
  de un workspace, empieza por su manifest.**

## Reglas de mantenimiento

- Mantén el manifest global pequeño: si una guía aplica a un solo concern, va en una rule.
- No dupliques: enlaza a la rule o al global del usuario (`~/.claude/CLAUDE.md`) en vez de
  reescribir.
- Una decisión abierta del vision se marca `> Estado: pendiente`; al resolverse, actualiza el
  doc fuente (`docs/`) y la rule en el mismo cambio.
