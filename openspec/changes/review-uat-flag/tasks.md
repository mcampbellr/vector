# Tasks — review-uat-flag

## 1. State

- [x] 1.1 Campo `NeedsUAT bool` (`json:"needsUat,omitempty"`) en `SpecState` (`internal/state/types.go`).
- [x] 1.2 `CreateSpecParams.NeedsUAT` + asignación en `CreateSpec`; parámetro `needsUAT bool` en
      `ReconcileStatus` (set dentro del mismo lock/write).
- [x] 1.3 Clear del flag cuando el status resultante no es `review`.

## 2. Sync

- [x] 2.1 Helper `syncNeedsUAT(c openspec.Change) bool` en `cmd/vector/main.go`
      (`HasTasks && TasksTotal>0 && TasksDone>0 && TasksDone<TasksTotal && PendingReal==0`).
- [x] 2.2 Pasar el bool en cada rama de `runSync` que crea/reconcilia un change.
- [x] 2.3 Tests: `syncNeedsUAT` (true solo-verificación con ≥1 hecha; false todo done; false con
      trabajo real; false `TasksDone==0`).

## 3. Board

- [x] 3.1 Campo `NeedsUAT` en `Card` (`internal/board/board.go`); copia en `toCard`.
- [x] 3.2 Test de serialización HTTP/SSE (`board` server test): `needsUat:true` presente cuando
      activo, omitido cuando false (verifica `omitempty`).

## 4. Web

- [x] 4.1 `needsUat?: boolean` en la interface `Card` (`web/src/types/board.ts`).
- [x] 4.2 Badge `UAT` en `SpecCard.tsx` (dentro de `<footer className={styles.meta}>`, con
      `title`/`aria-label`) cuando `status==='review' && needsUat`; clase `.uat` en el CSS module.

## 5. Docs

- [x] 5.1 Nota en `docs/domain-contract.md` (§1/§5): `review` puede llevar `needsUat` derivado;
      NO es estado nuevo ni cambia la máquina de estados.

## 6. Verificación

- [x] 6.1 `gofmt -l cli && go -C cli vet ./... && go -C cli test -race ./...` verdes.
- [x] 6.2 `npm --prefix web run typecheck` sin errores.
- [ ] 6.3 Manual UAT: `vector sync` en un repo con un change de solo-verificación deja la card en
      `review` con el badge UAT en el board; `--reconcile` que la devuelve a `in-progress` limpia
      el flag.
