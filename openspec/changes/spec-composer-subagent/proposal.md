# Add vector-spec-composer subagent

## Why

`/vector:raw` y `/vector:bug` componen las 20 secciones del spec en el main loop Opus. Eso
consume entre 8 y 15 k tokens de output del tier más caro por cada spec creado, y el texto
completo del spec permanece en el contexto del loop durante los pasos de validación, creación
y reporte. El coste se paga aunque el spec sea rutinario.

La composición no requiere capacidades de Opus: dado un brief estructurado y las respuestas
del usuario, es un trabajo de generación determinista que Sonnet resuelve con calidad
equivalente. El gate adversarial (validator Sonnet) ya detecta si el output es insuficiente.

## What changes

- Nuevo subagente del kit `vector-spec-composer` (`kit/agents/vector-spec-composer.md`,
  `model: sonnet`, `tools: Read, Write, Glob`) con sus dos copias sincronizadas
  (`.claude/agents/` y `cli/internal/scaffold/assets/agents/`).
- Modificación de `kit/commands/vector/raw.md`: el paso 7 de composición inline se reemplaza
  por una llamada al subagente; el paso 9 pasa a `--body-file "$SPEC_PATH"` en lugar de
  heredoc; el paso 10 agrega el routing del compositor.
- Modificación de `kit/commands/vector/bug.md`: mismos cambios a pasos 7, 9 y 10, adaptados
  al contexto de bug (brief del `vector-bug-refiner`, prefijo `fix-` en el id).
- Nuevo test en `cli/internal/scaffold/scaffold_test.go` que verifica que `SeedCommands`
  siembra `vector-spec-composer` junto a los demás agentes.

## Scope

- In: el subagente compositor, las tres copias sincronizadas, la modificación de `raw.md` y
  `bug.md` (pasos 7, 9, 10), el test de scaffold.
- Out: cambios al binario Go, a `vector-spec-refiner`, `vector-spec-validator`,
  `vector-bug-refiner`, otros commands del kit, el panel web, limpieza automática de
  `.vector/tmp/`.

Authored spec: `.vector/specs/spec-composer-subagent/spec.md`.
