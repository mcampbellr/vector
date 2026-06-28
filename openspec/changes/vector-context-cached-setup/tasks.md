# Tasks — vector-context-cached-setup

## 1. Config — campos y detección de manifests

- [x] 1.1 Añadir campos `BuildCmd`, `LintCmd`, `TestCmd string` con tag `json:"...,omitempty"` en `Config` en `cli/internal/config/config.go`.
- [x] 1.2 Implementar `DetectBuildCmds(repoRoot string) (build, lint, test string)` con `sync.WaitGroup` + goroutines: Makefile (targets `build:`/`lint:`/`test:`), `go.mod`, `package.json` (`scripts`), `pyproject.toml`/`setup.py`. Strings vacíos cuando no se puede inferir con confianza.
- [x] 1.3 Añadir accessor `ResolvedBuildCmds() (build, lint, test string)` en `*Config`.
- [x] 1.4 Tests table-driven en `config_test.go` con `t.TempDir()`: Go (`go.mod`), Node (`package.json` con scripts), Makefile con targets, sin manifests (tres vacíos), `package.json` sin campo `scripts`.

## 2. Subcomando `vector context`

- [x] 2.1 Crear `cli/cmd/vector/context.go`: struct `ContextOutput` con los 7 campos JSON (`examplePath`, `language`, `buildCmd`, `lintCmd`, `testCmd`, `applyMode`, `ticketDetected`).
- [x] 2.2 Implementar `runContext(args []string) error`: flags `--repo-root`, `--json` (default `true`), `--dry-run` (no-op, coherencia de patrón).
- [x] 2.3 Carga `config.Load(root)` con error accionable si falta: `"no .vector/config.json in {root} — run vector init first"`.
- [x] 2.4 Glob de `specPath` para `examplePath`: `StoreVector` → `.vector/specs/*/spec.md`; `StoreConvention` → reusar `config.FindSpecDocs`. Primer resultado lexicográfico; `""` si ninguno. Warning a stderr si el glob falla (no aborta).
- [x] 2.5 Si `BuildCmd`/`LintCmd`/`TestCmd` están vacíos en config: llamar `DetectBuildCmds(root)` concurrentemente con el glob (`sync.WaitGroup`). No persistir.
- [x] 2.6 Salida `--json`: `json.MarshalIndent` del struct a stdout. Salida humana: una línea por campo con alineación fija.
- [x] 2.7 Exit `0` éxito; exit `1` error (mensaje a stderr). `vector context` nunca llama `AskUserQuestion`.

## 3. Integración en `main.go`

- [x] 3.1 Añadir `case "context": err = runContext(os.Args[2:])` al switch en `main()` (antes de `case "version"`).
- [x] 3.2 Actualizar `usage()`: incluir `context` con descripción `"print repo setup context (example path, language, build/lint/test commands)"`.
- [x] 3.3 En `runInit`: tras resolver `cfg`, llamar `DetectBuildCmds(root)` y setear `cfg.BuildCmd`/`LintCmd`/`TestCmd` cuando estén vacíos (o `--force`). Persistir antes de `config.Write`.
- [x] 3.4 En `runUpdate`: ídem al re-sembrar el kit.

## 4. Commands del kit

- [x] 4.1 Actualizar `kit/commands/vector/raw.md`: reemplazar paso 2 (glob de specPath) + paso 3 (detect-language) por llamada `vector context --json` → `CONTEXT`; renumerar pasos; propagar `CONTEXT.examplePath` y `CONTEXT.language`; añadir nota de token routing; mantener fallback para cuando `CONTEXT` falle.
- [x] 4.2 Actualizar `kit/commands/vector/bug.md`: añadir paso 0 `vector context --json` → `CONTEXT`; reemplazar paso 4 (glob + detect-language) por consumo de `CONTEXT`; renumerar; propagar valores al refiner/validator.
- [x] 4.3 Actualizar `kit/commands/vector/apply.md`: añadir paso 0 `vector context --json` → `CONTEXT`; en §4 (gate de build/test) usar `CONTEXT.buildCmd`/`testCmd` con fallback explícito al detect manual cuando vacíos; ajustar nota de token routing.
- [x] 4.4 Actualizar `kit/commands/vector/comment.md`: añadir paso 0 `vector context --json` → `CONTEXT`; en §7a.3 usar `CONTEXT.buildCmd`/`lintCmd`/`testCmd` con fallback al discover manual cuando vacíos; ajustar token routing.

## 5. Vendorización de assets

- [x] 5.1 Regenerar `cli/internal/scaffold/assets/commands/vector/raw.md` desde el source actualizado en `kit/`.
- [x] 5.2 Regenerar `cli/internal/scaffold/assets/commands/vector/bug.md`.
- [x] 5.3 Regenerar `cli/internal/scaffold/assets/commands/vector/apply.md`.
- [x] 5.4 Regenerar `cli/internal/scaffold/assets/commands/vector/comment.md`.

## 6. Verificación

- [x] 6.1 `gofmt -l cli && go -C cli vet ./... && go -C cli test ./...` verdes.
- [x] 6.2 `vector context --json` en el repo de Vector devuelve JSON parseable con los 7 campos.
- [x] 6.3 `vector context --json` sobre repo sin `config.json` retorna exit `1` con mensaje accionable.
- [ ] 6.4 `raw.md` y `bug.md` actualizados no globean ni detectan lenguaje por su cuenta en repos Go y Node (sin regresión de comportamiento observable).
- [ ] 6.5 `apply.md` y `comment.md` usan `CONTEXT.buildCmd`/`testCmd`/`lintCmd` cuando disponibles; caen al fallback cuando vacíos.
- [ ] 6.6 Sin regresiones en `vector init`, `vector update`, `vector sync`, `vector spec create`, `vector serve`.
- [x] 6.7 Assets vendorizados en `cli/internal/scaffold/assets/` sincronizados con los sources de `kit/commands/vector/`.
