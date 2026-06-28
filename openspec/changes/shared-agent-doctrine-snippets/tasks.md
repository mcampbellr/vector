# Tasks — shared-agent-doctrine-snippets

## 1. Ficheros `_shared/` (nuevos)

- [ ] 1.1 Leer los seis agentes fuente en `kit/agents/` para extraer el texto exacto a compartir
  (no de memoria).
- [ ] 1.2 Crear `kit/agents/_shared/citation-discipline.md` con la doctrina "Cite, don't guess"
  completa y genérica (sin frontmatter YAML, en inglés).
- [ ] 1.3 Crear `kit/agents/_shared/prose-rules.md` con "Never invent work" + bloque
  "Prose quality — write like a human" completo (sin frontmatter YAML).
- [ ] 1.4 Crear `kit/agents/_shared/refiner-base.md` con "Preserve the user's language" +
  "Be terse" (sin frontmatter YAML).
- [ ] 1.5 Verificar que ninguno de los tres ficheros contiene frontmatter YAML (`name:`, `model:`,
  `tools:`).

## 2. Modificación de agentes

- [ ] 2.1 Modificar `kit/agents/vector-spec-refiner.md`: añadir sección `## Shared doctrine`
  antes de `## Hard rules` con directiva de carga a `citation-discipline.md` y `refiner-base.md`;
  eliminar los bullets inline correspondientes.
- [ ] 2.2 Modificar `kit/agents/vector-bug-refiner.md`: igual que 2.1 — mismos dos ficheros
  `_shared/`.
- [ ] 2.3 Modificar `kit/agents/vector-spec-validator.md`: añadir `## Shared doctrine` con
  directiva de carga a `citation-discipline.md`; eliminar bullet "Cite, don't hand-wave" inline.
- [ ] 2.4 Modificar `kit/agents/vector-comment-evaluator.md`: igual que 2.3.
- [ ] 2.5 Modificar `kit/agents/vector-summary-writer.md`: añadir `## Shared doctrine` con
  directiva de carga a `prose-rules.md`; eliminar bullet "Never invent work" y sección
  "Prose quality" inline.
- [ ] 2.6 Modificar `kit/agents/vector-standup-writer.md`: igual que 2.5.
- [ ] 2.7 Verificar que ninguno de los seis agentes modificados conserva los strings extraídos
  inline (greps de comprobación por agente).

## 3. Sincronización de assets embebidos

- [ ] 3.1 Ejecutar `go generate ./...` desde `cli/internal/scaffold/`.
- [ ] 3.2 Verificar que `cli/internal/scaffold/assets/agents/_shared/` fue creado y contiene
  `citation-discipline.md`, `prose-rules.md` y `refiner-base.md`.
- [ ] 3.3 Verificar que los seis agentes modificados en `assets/agents/` coinciden con los de
  `kit/agents/` (no editados a mano).

## 4. Test de consistencia estructural

- [ ] 4.1 Añadir `TestSharedFilesExist` en `cli/internal/scaffold/scaffold_test.go`: verifica
  que `assets/agents/_shared/` contiene los tres ficheros esperados.
- [ ] 4.2 Añadir `TestSharedDoctrineNotInlined` en `cli/internal/scaffold/scaffold_test.go`:
  para cada agente modificado, afirma que el contenido del fichero en `assets/agents/` no
  contiene los strings canónicos extraídos (e.g., `"Cite, don't guess"`, `"Prose quality"`,
  `"Preserve the user's language"`).
- [ ] 4.3 Verificar que los tests existentes (`TestSeedCommands`, etc.) no se rompieron.

## 5. Dogfooding local

- [ ] 5.1 Actualizar `.claude/agents/` del propio repo de Vector ejecutando `vector update` o
  copiando directamente desde `cli/internal/scaffold/assets/agents/`.
- [ ] 5.2 Verificar que `.claude/agents/_shared/` existe y contiene los tres ficheros.

## 6. Verificación final

- [ ] 6.1 Ejecutar `gofmt -l cli && go -C cli vet ./... && go -C cli test ./...` — todo verde,
  sin regresiones.
- [ ] 6.2 Ejecutar greps de ausencia inline sobre cada agente modificado en `kit/agents/`
  (ver comandos en spec §8).
- [ ] 6.3 Confirmar que ningún agente cambió su tier de modelo, esquema de salida ni lista de
  tools.
- [ ] 6.4 Documentar las Open questions #1–#5 del spec como TBD en sus secciones
  correspondientes si no fueron resueltas antes de implementar.
