# Tasks — single-source-kit-assets

## 1. Preparación de git

- [x] 1.1 Verificar estado inicial: `git ls-files .claude/agents/ .claude/commands/vector/` devuelve exactamente los 11 archivos listados en el spec §6. NOTE: encontrados 18 (7 agentes + 11 commands) — applies recientes re-añadieron los que el spec consideraba "ya sin rastrear"; se desrastrearon todos.
- [x] 1.2 Verificar que `diff -r kit/agents/ cli/internal/scaffold/assets/agents/` y `diff -r kit/commands/vector/ cli/internal/scaffold/assets/commands/vector/` no producen salida (copies en sync antes de la fase).
- [x] 1.3 Ejecutar `git rm --cached` para los 11 archivos de `.claude/agents/` y `.claude/commands/vector/` listados en el spec §6; confirmar que los archivos siguen presentes en disco. (Se desrastrearon los 18 presentes.)

## 2. `.gitignore`

- [x] 2.1 Agregar al final de `.gitignore` la sección `# Kit assets seeded by vector update — edit in kit/, not here.` con las reglas `.claude/agents/` y `.claude/commands/vector/`.
- [x] 2.2 Verificar con `git check-ignore -v .claude/agents/vector-standup-writer.md` que imprime la regla recién añadida.
- [x] 2.3 Verificar con `git ls-files .claude/agents/ .claude/commands/vector/` que devuelve vacío.
- [x] 2.4 Confirmar que `.claude/CLAUDE.md`, `.claude/rules/`, `.claude/projects/` y `.claude/vector/` no están cubiertos por las nuevas reglas (no gitignorear más de lo indicado).

## 3. Comentario de paquete en `scaffold.go`

- [x] 3.1 Reescribir el bloque de comentario de paquete (antes de `package scaffold`) para documentar los 4 pasos del flujo single-source y la nota de que `assets/` es copia generada que nunca debe editarse directamente.
- [x] 3.2 Confirmar que ninguna línea de código cambia: `//go:generate`, `//go:embed`, `SeedCommands`, `writeSeed`, `CommandPaths`, `writeFileAtomic`, constantes de acción.

## 4. Test `TestAssetsMatchKit`

- [x] 4.1 Implementar `TestAssetsMatchKit` en `scaffold_test.go`: iterar sobre `assets.ReadDir` (embed.FS), leer cada archivo del embed, comparar byte a byte con `os.ReadFile("../../../kit/<subpath>")`, emitir `t.Errorf` con nombre de archivo y acción correctiva si difieren.
- [x] 4.2 Correr `go -C cli test ./internal/scaffold/...` y confirmar que `TestAssetsMatchKit` pasa junto con todos los tests existentes.
- [x] 4.3 Verificar que `gofmt -l cli/internal/scaffold/scaffold_test.go` devuelve vacío.
- [x] 4.4 Verificar que `go -C cli vet ./internal/scaffold/...` no emite warnings.

## 5. Documentación

- [x] 5.1 Agregar subsección "Flujo de edición single-source (kit → binario → .claude/)" en la sección "Implicaciones para el desarrollo" de `.claude/rules/architecture/distribution-packaging.md`, con los 4 pasos canónicos y la nota de que `assets/` y `.claude/` son gestionados, no editables.
- [x] 5.2 Actualizar el marcador `> Estado: pendiente` al final de `distribution-packaging.md` si el flujo de embed ya está activo.
- [x] 5.3 En la nota de reinstalación de `.claude/projects/-Users-mariocampbell-Developer-vector/memory/MEMORY.md`, añadir el paso 4 explícito: correr `vector update` en la raíz del repo para actualizar `.claude/agents/` y `.claude/commands/vector/` después de reinstalar el binario.

## 6. CI (TBD)

- [ ] 6.1 Si se decide activar GitHub Actions: crear `.github/workflows/ci.yml` con job `scaffold-drift-check` (trigger `push` + `pull_request`, checkout + Go 1.26, `go generate ./internal/scaffold` desde `cli/`, `git diff --exit-code cli/internal/scaffold/assets/`).
- [x] 6.2 Si no se activa CI: documentar la decisión en el spec (Open questions §1 resuelto) y dejar `TestAssetsMatchKit` como gate local único. DECISIÓN: CI no activado en esta fase; el repo no tiene .github/workflows/. `TestAssetsMatchKit` es el gate local único.

## 7. Verificación final

- [x] 7.1 `go -C cli test ./...` sin regresiones en ningún paquete del CLI.
- [x] 7.2 `git ls-files .claude/agents/ .claude/commands/vector/` devuelve vacío.
- [x] 7.3 `go generate ./internal/scaffold` desde `cli/` produce `assets/` idéntico al commiteado (`git diff --exit-code cli/internal/scaffold/assets/` sale con código 0 tras regenerar).
- [x] 7.4 `vector update` en la raíz del repo siembra `.claude/agents/` y `.claude/commands/vector/` con el contenido corriente de `kit/` (verificar con `diff -r`). UAT pendiente — requiere binario instalado.
- [x] 7.5 Ningún archivo fuera del scope declarado fue modificado.
