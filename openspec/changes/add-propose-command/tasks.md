# Tasks — add-propose-command

## 1. State + eventos

- [x] 1.1 `EvtSpecProposed` + `ProposedData` en `event.go`.
- [x] 1.2 `Store.ProposeSpec`: valida `draft`, flipea a `open`, provenance, 2 eventos; sin `StartedAt`.
- [x] 1.3 Tests: `ProposeSpec` (transición + provenance + eventos) y rechazo de no-draft.

## 2. Binario

- [x] 2.1 `vector spec propose <id> [--change] [--artifacts] [--dry-run] [--json]` en `runSpec`.
- [x] 2.2 Id como positional inicial aunque sigan flags; validación de flags (`--artifacts`, `--change`).
- [x] 2.3 Idempotencia (`already open` reporta, no falla); usage actualizado.

## 3. Config

- [x] 3.1 Campo `ProposeBranch` (override del worktree).

## 4. Command (kit)

- [x] 4.1 `kit/commands/vector/propose.md`: adapter delegate/native, resolución de worktree, prompts.
- [x] 4.2 Sembrado vía `go generate` + `vector update`.

## 5. Verificación

- [x] 5.1 `gofmt` / `go vet` / `go test` verdes.
- [x] 5.2 e2e: `propose <id>` flipea draft→open, idempotente, 3 eventos.
- [ ] 5.3 Manual QA: detección delegate vs native en un repo OpenSpec real (pendiente, necesita un proyecto OpenSpec).
