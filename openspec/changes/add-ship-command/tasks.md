# Tasks — add-ship-command

## 1. Config (binario)

- [x] 1.1 `ShipConfig{BaseBranch, Mode, Draft *bool, ExcludeGlobs, AuthBootstrap}` + `Config.Ship *ShipConfig` (`omitempty`) en `config.go`; `ShipMode` (ask|auto) + `Valid()`; `DefaultShipExcludeGlobs = ["openspec/"]`.
- [x] 1.2 Resolutores nil-safe: `ResolvedShipMode` (default `ask`), `ResolvedShipDraft` (default `true`), `ResolvedShipExcludeGlobs` (default estático + los del config), `ResolvedShipBaseBranch(fallback)`, `ResolvedShipAuthBootstrap`. Validación de `ship.mode` en `Load`. `SchemaVersion` intacto en 1.
- [x] 1.3 Tests: round-trip de `ShipConfig` (set/omitido); config legacy sin `ship` → `Ship == nil` sin error; resolutores con `Ship` nil/parcial (incl. `ResolvedShipExcludeGlobs()` con `Ship == nil` → `["openspec/"]`); `Load` rechaza `ship.mode` inválido.

## 2. `vector config set-ship` (binario)

- [x] 2.1 `runConfigSetShip` + dispatch `case "set-ship"` en `runConfig`; flags `--base-branch`/`--mode`/`--draft`/`--exclude`/`--auth-bootstrap`/`--repo-root`/`--json`, todos opcionales, merge incremental por campo ("empty = don't change"); `--draft` con `strconv.ParseBool` solo si no-vacío.
- [x] 2.2 Error si ningún flag configurable vino con valor; `config.Load` (falla con `run vector init first`); validar `--mode` antes de aplicar; `changed` por comparación de valor; escribir solo si `changed`; salida `--json` con los valores resultantes.
- [x] 2.3 Tests: escritura incremental por campo; idempotencia (`changed:false`); error sin flags; error con `--mode` inválido.

## 3. Registro de PR (state + binario)

- [x] 3.1 `PullRequest{URL,Number,Draft,OpenedAt}` + `SpecState.PR *PullRequest` (`omitempty`) en `types.go`; `EvtPROpened = "pr.opened"` + `PROpenedData{URL,Number,Draft}` en `event.go`. `SchemaVersion`/`EventVersion` intactos en 1.
- [x] 3.2 `Store.RecordPR(id, url, number, draft, actor, now)` modelado sobre `LinkSpec`: lock único, `ReadSpec`, idempotencia en `pr.URL`, bump `UpdatedAt`, `writeSpecFile`, `appendEvent`; valida `url` no vacía; **sin** transición de status.
- [x] 3.3 `runSpecPR` (`<id> <url>` positionals + `--number`/`--draft`/`--repo-root`/`--json`) modelado sobre `runSpecLink`; valida `id` kebab-case y `url` no vacía; `case "pr"` en `runSpec` + líneas de `usage()`.
- [x] 3.4 Tests: `Store.RecordPR` (primera escritura + `pr.opened`; misma URL no-op sin evento; URL distinta reemplaza + segundo evento; URL vacía → error); `runSpecPR` (escritura + idempotencia vía CLI; error URL vacía; error id no kebab-case).

## 4. `vector context --json` (binario)

- [x] 4.1 `ShipContext{BaseBranch,Mode,Draft,ExcludeGlobs,AuthBootstrap}` + `ContextOutput.Ship *ShipContext` (`omitempty`), poblado solo cuando `cfg.Ship != nil` (con fallback de `baseBranch` al `worktree.baseBranch`); línea correspondiente en el output humano.
- [x] 4.2 Tests: `ship` presente en el JSON cuando el config lo declara (con fallback de baseBranch), ausente cuando no.

## 5. Command (kit)

- [x] 5.1 `kit/commands/vector/ship.md`: §0 contexto → §1 selección (D11) → §2 precondición → §3 warning stale (D12) → §4 auth bootstrap (D4) → §5 commit (D6, excluye `EXCLUDE_GLOBS`+`SPEC_DOC`, nunca `git add -A`) → §6 rebase + colisión untracked (D9) → §7 PR-text inline (D5) → §8 push (nunca `--force`) → §9 idempotencia (D8) → §10 apertura draft según `mode` → §11 `vector spec pr` → §12 reporte.
- [x] 5.2 Vendoring vía `go generate ./internal/scaffold` + sembrado por `vector update`; `TestAssetsMatchKit` verde con la copia de `ship.md`.
- [x] 5.3 Fila de `/vector:ship` en la tabla "Commands Reference" de `README.md`, tras `/vector:apply`.

## 6. Verificación

- [x] 6.1 `go -C cli generate ./internal/scaffold`, `gofmt -l cli`, `go -C cli vet ./...`, `go -C cli test ./...`, `go -C cli build ./...` — todos verdes.
- [x] 6.2 Sin regresiones en `spec create|list|propose|apply|fix|link|relate|status|close|route|worklog` ni en `config set-jira-mcp` / `context`.
- [x] 6.3 e2e: un spec en `review` → `/vector:ship` commitea (excluyendo openspec/ + doc del spec), rebasea, pushea, abre PR draft, y registra `pr`/`pr.opened` sin mover el status; re-ejecutar surfacea el link sin duplicar.
