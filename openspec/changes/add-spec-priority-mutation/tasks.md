# Tasks — add-spec-priority-mutation

## 1. State + evento

- [ ] 1.1 `EvtPriorityChanged EventType = "priority.changed"` + `PriorityChangedData{From, To Priority}` en `event.go` (forma `From`/`To`, sin `Trigger`/`Reason`; sin bump de `EventVersion`).
- [ ] 1.2 `Store.SetPriority(id, priority, actor, now) (*SpecState, error)` en `store.go`: valida `priority.Valid()` antes del lock → lock → `ReadSpec` → guarda archivado → guarda no-op (sin evento) → mutar `Priority`+`UpdatedAt` → `writeSpecFile` (retornar antes del log si falla) → `appendEvent`.
- [ ] 1.3 Tests unitarios en `store_test.go`: éxito con evento persistido, no-op sin evento ni bump de `UpdatedAt`, prioridad inválida, spec desconocido, spec archivado.

## 2. Comando (binario)

- [ ] 2.1 `newSpecSetCmd()` en `spec_transitions.go`: `Use: "set [id]"`, flags `--id`, `--priority` (requerido), `--repo-root`, `--dry-run`, `--json`; `leadingID`/`openStore`; re-valida `--priority`; lee spec previo para `previousPriority`; `--dry-run` no toca el store; reporta éxito/no-op humano + JSON (§7).
- [ ] 2.2 Registrar `newSpecSetCmd()` en `newSpecCmd()`, añadir `set` al usage del `RunE` sin subverbo, y la línea de `vector spec set` en `usage()` (`main.go`).
- [ ] 2.3 Shim `runSpecSet` en `testutil_test.go`.
- [ ] 2.4 Tests de CLI en `spec_transitions_test.go`: `TestRunSpecSetValidation` (falta/invalid `--priority`, id vacío/desconocido, archivado, no-op, caso válido), `TestRunSpecSetDryRun`, `TestRunSpecSetJSONShape`.

## 3. Golden

- [ ] 3.1 Caso `spec-set` en `TestJSONGoldenUnchanged` (`golden_test.go`) + fixture `testdata/golden/spec-set.json` generado con `-update-golden` y revisado.

## 4. Verificación

- [ ] 4.1 `gofmt -l cli`, `go -C cli vet ./...`, `golangci-lint run ./cli/...`, `go -C cli test ./...`, `go -C cli build ./...` — todos verdes.
- [ ] 4.2 `vector spec list --json` refleja el nuevo `priority` tras el `set`; board (SSE) lo refleja sin cambios de código.
- [ ] 4.3 Reinstalar el binario global + `vector update` en la raíz del repo (dogfooding).
