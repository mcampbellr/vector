# Design — single-source-kit-assets

## Decisiones clave

- **`kit/` es la única fuente editable**: `assets/` es una copia vendorizada generada por
  `//go:generate`; `.claude/` es una copia sembrada por el binario vía `vector update`. Ninguna
  de las dos debe editarse a mano. Esta es la invariante central de la fase.
- **`git rm --cached` antes de `.gitignore`**: los archivos ya rastreados no quedan cubiertos
  por nuevas reglas de `.gitignore` a menos que se desrastreen primero. El orden es obligatorio:
  (1) `git rm --cached`, (2) añadir reglas al `.gitignore`, (3) commitear ambos en el mismo
  cambio. Desrastrear sin borrar del disco (los archivos siguen siendo necesarios para el
  dogfooding local hasta que `vector update` los resiembre).
- **No gitignorear `.claude/` completo**: solo `.claude/agents/` y `.claude/commands/vector/`
  quedan excluidos. El resto del directorio (CLAUDE.md, rules/, projects/, vector/) contiene
  contexto de instrucciones legítimamente versionado y no debe tocarse.
- **`assets/` permanece tracked**: es el snapshot del último `go generate` commiteado. Eso
  permite que el CI lo compare con el resultado de re-generar en un directorio limpio. No se
  agrega a `.gitignore`.
- **Gate local = test, no solo CI**: `TestAssetsMatchKit` lee los archivos de `kit/` desde la
  ruta relativa `../../../kit/` (el working directory de `go test` es el directorio del paquete)
  y los compara byte a byte contra los assets embebidos. Si alguien edita `assets/` a mano o
  olvida correr `go generate`, el test falla antes del merge. CI (cuando exista) corre el mismo
  invariante a nivel de repositorio con `git diff --exit-code`.
- **Solo el bloque de comentario de `scaffold.go`**: la lógica de `SeedCommands`, `writeSeed`,
  `CommandPaths`, `writeFileAtomic` y las constantes de acción son correctas y no se modifican.
  El único cambio de código en Go es el comentario de paquete; el test nuevo va en
  `scaffold_test.go`.
- **CI es TBD**: el repo no tiene `.github/workflows/`. La decisión de activar GitHub Actions
  queda abierta (Open questions §1 del spec). `TestAssetsMatchKit` cubre el mismo invariante
  localmente; el workflow es aditivo, no bloqueante para el resto de la fase.

## Superficie

- `.gitignore`: nuevas reglas en sección propia al final del archivo.
- `cli/internal/scaffold/scaffold.go`: solo el bloque de comentario de paquete (antes de
  `package scaffold`); ninguna línea de código.
- `cli/internal/scaffold/scaffold_test.go`: función `TestAssetsMatchKit` nueva; tests
  existentes sin cambios.
- `.claude/rules/architecture/distribution-packaging.md`: subsección nueva dentro de
  "Implicaciones para el desarrollo".
- `.claude/projects/-Users-mariocampbell-Developer-vector/memory/MEMORY.md`: paso 4
  (`vector update`) en la nota de reinstalación existente.
- `.github/workflows/ci.yml`: NUEVO, TBD — job `scaffold-drift-check`.

## Flujo canónico (post-fase)

```
kit/<agente|cmd>.md  ──edit──▶  go generate ./internal/scaffold
                                       │
                          cli/internal/scaffold/assets/  (actualizado)
                                       │
                          go install ./cmd/vector  (binario reinstalado)
                                       │
                          vector update  (siembra .claude/ desde el binario)
                                       │
                          .claude/agents/ + .claude/commands/vector/  (actualizados)
```

Git no rastrea `.claude/agents/` ni `.claude/commands/vector/`. Rastrea `assets/` como
snapshot del estado generado. `TestAssetsMatchKit` detecta drift entre `kit/` y `assets/`
antes del merge.

## Detalle del test `TestAssetsMatchKit`

- Itera sobre `assets.ReadDir` (embed.FS) para obtener la lista de archivos embebidos.
- Por cada archivo, lee el contenido del embed y lo compara con `os.ReadFile` de la ruta
  correspondiente en `kit/` (resolución: `../../../kit/<subpath>`).
- `t.Errorf` con nombre del archivo diferente y la acción correctiva
  (`"run go generate ./internal/scaffold"`).
- No usa fixtures ni symlinks; solo el embed existente y los archivos reales de `kit/`.
- El test es determinista y sin I/O de red; corre en < 10ms.
