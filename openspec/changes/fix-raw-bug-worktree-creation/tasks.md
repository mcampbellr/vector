# Tasks — fix-raw-bug-worktree-creation

## 1. Contexto de layout (condicional Q-A)

- [ ] 1.1 Evaluar si `vector context --json` ya expone lo necesario para que el orchestration
      derive `worktree-root`/`base-branch`/`prefijo`/flag `[branch]`. Documentar el veredicto Q-A.
- [ ] 1.2 Si falta: helper `HasBranchPlaceholder` + accesores de root/base-branch/prefijo en
      `cli/internal/config/config.go` (aditivos, `SchemaVersion` sin cambios; sin tocar la
      resolución `[branch]` existente).
- [ ] 1.3 Si falta: exponer `worktreeRoot`/`baseBranch`(default `main`)/`branchPrefix`(default
      `feat/`)/flag de layout en `cli/cmd/vector/context.go`.
- [ ] 1.4 Tests de los accesores/context: default y override de base+prefijo; presencia/ausencia
      de `[branch]`.

## 2. Orchestration del comando (kit)

- [ ] 2.1 `kit/commands/vector/raw.md`: paso worktree-resolve/create condicional antes de escribir
      el doc — (a) detecta `[branch]`; (b) resuelve root/base/prefijo; (c) reutiliza o crea con
      `git worktree add … -b <prefijo>/<slug> <base-branch>` (idempotente); (d) inerte en
      no-worktree; (e) fallo de git (incl. stub previo) → error accionable, sin auto-borrar.
- [ ] 2.2 `kit/commands/vector/bug.md`: el mismo paso, simétrico a raw.
- [ ] 2.3 Recordatorio en ambos de que el doc queda **dentro** del worktree (tracked en la rama
      feature). Sin lógica de limpieza/migración de stubs; sin escritura de estado a mano.

## 3. Propagación single-source

- [ ] 3.1 `go -C cli generate ./internal/scaffold` → regenerar
      `cli/internal/scaffold/assets/commands/vector/{raw,bug}.md` (nunca a mano).
- [ ] 3.2 `TestAssetsMatchKit` verde (sin drift kit↔assets).

## 4. Tests

- [ ] 4.1 Layout worktree: la creación produce worktree + rama + doc tracked.
- [ ] 4.2 Idempotencia: segundo run con el mismo slug reutiliza sin error.
- [ ] 4.3 Regresión no-worktree: `spec-path` sin `[branch]` no crea worktree.
- [ ] 4.4 Resolución de `<base-branch>`/`<prefijo>` desde config (default y override).
- [ ] 4.5 Stub suelto previo → fallo accionable (no silencioso, no auto-borrado).

## 5. Verificación

- [ ] 5.1 `go -C cli generate ./internal/scaffold` (antes de tests, para `TestAssetsMatchKit`).
- [ ] 5.2 `gofmt -l cli`, `go -C cli vet ./...`, `go -C cli test ./...`, `go -C cli build ./...` verdes.
- [ ] 5.3 Sin regresiones en `spec create|propose|apply|sync` ni en el comportamiento no-worktree.
- [ ] 5.4 Documentación actualizada si se extiende `vector context`/config (Q-A).
