# Tasks — spec-composer-subagent

## 1. Subagente compositor

- [x] 1.1 Crear `kit/agents/vector-spec-composer.md` con frontmatter `name: vector-spec-composer`, `model: sonnet`, `tools: Read, Write, Glob` y cuerpo de instrucción completo (leer template + ejemplo, componer 20 secciones, escribir `OUTPUT_PATH`, devolver confirmación estructurada).
- [x] 1.2 Crear `.claude/agents/vector-spec-composer.md` con contenido byte-a-byte igual al de `kit/agents/`.
- [x] 1.3 Crear `cli/internal/scaffold/assets/agents/vector-spec-composer.md` con contenido byte-a-byte igual al de `kit/agents/`.
- [x] 1.4 Verificar sincronización: `diff kit/agents/vector-spec-composer.md .claude/agents/vector-spec-composer.md` y `diff kit/agents/vector-spec-composer.md cli/internal/scaffold/assets/agents/vector-spec-composer.md` sin diferencias.

## 2. Command raw.md

- [x] 2.1 Reemplazar el bloque inline de composición del paso 7 por la invocación al subagente `vector-spec-composer` (Sonnet) con los inputs: `BRIEF`, `CLARIFICATIONS`, `TEMPLATE_PATH`, `SPEC_EXAMPLE_PATH`, `SPEC_TITLE`, `SPEC_ID`, `SPEC_LANGUAGE`, `OUTPUT_PATH` (`.vector/tmp/<SPEC_ID>/spec.md`); guardar ruta devuelta como `SPEC_PATH`.
- [x] 2.2 Actualizar el paso 9: reemplazar el heredoc por `--body-file "$SPEC_PATH"` en la invocación de `vector spec create`.
- [x] 2.3 Agregar al paso 10 el bloque de routing del compositor: `vector spec route <id> --model sonnet --baseline opus --task "compose spec" --tokens-in <n> --tokens-out <n>`.
- [x] 2.4 Confirmar que los pasos 1–6, 8 y 11 no fueron modificados.

## 3. Command bug.md

- [x] 3.1 Reemplazar el bloque inline de composición del paso 7 por la invocación al subagente `vector-spec-composer` (Sonnet), adaptada al contexto de bug: `BRIEF` del `vector-bug-refiner`, `SPEC_ID` con prefijo `fix-`, campos de bug-framing incluidos en `CLARIFICATIONS`.
- [x] 3.2 Actualizar el paso 9: reemplazar el heredoc por `--body-file "$SPEC_PATH"`.
- [x] 3.3 Agregar al paso 10 el bloque de routing del compositor (mismo patrón que `raw.md`).
- [x] 3.4 Confirmar que los pasos 1–6, 8 y 11 no fueron modificados y que la lógica de detección de tickets (`RELATED_JSON`, bandera `--related`) permanece intacta.

## 4. Test de scaffold

- [x] 4.1 Agregar constante `specComposerAgent = ".claude/agents/vector-spec-composer.md"` en `cli/internal/scaffold/scaffold_test.go` junto a las constantes existentes de agentes.
- [x] 4.2 Agregar test (análogo a `TestSeedCommandsSeedsBugCommandAndRefiner`) que verifique que `SeedCommands` produce `ActionCreated` para `specComposerAgent` y que el archivo existe en el directorio temporal.

## 5. Verificación

- [x] 5.1 `gofmt -l cli` sin output, `go -C cli vet ./...` y `go -C cli test ./...` pasan sin fallos ni warnings.
- [ ] 5.2 QA manual del pipeline: invocar `/vector:raw` con una idea simple y confirmar que el spec compuesto llega al validator vía `SPEC_PATH` (no inline) y el archivo existe en `.vector/tmp/<id>/spec.md`.
- [ ] 5.3 QA manual de `/vector:bug`: confirmar que el pipeline bug (bug-refiner → clarify → compositor → validator → create) completa end-to-end con `--body-file`.
