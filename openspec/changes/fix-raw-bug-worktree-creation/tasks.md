# Tasks — fix-raw-bug-worktree-creation

## 1. Contexto de layout (condicional Q-A)

- [x] 1.1 Evaluar si `vector context --json` ya expone lo necesario para que el orchestration
      derive `worktree-root`/`base-branch`/`prefijo`/flag `[branch]`. **Veredicto Q-A: NO** — el
      `ContextOutput` previo no exponía ninguno de esos campos. Se extiende `context` + `config.go`.
- [x] 1.2 Helper `HasBranchPlaceholder` + accesores `WorktreeRoot`/`BaseBranchOrDefault`/
      `BranchPrefixOrDefault` + campos `BaseBranch`/`BranchPrefix` en
      `cli/internal/config/config.go` (aditivos, `SchemaVersion` sin cambios; sin tocar la
      resolución `[branch]` existente en `SpecDocPath`).
- [x] 1.3 Exponer el bloque `worktree` (`layout`/`root`/`baseBranch`(default `main`)/
      `branchPrefix`(default `feat/`)) en `cli/cmd/vector/context.go`.
- [x] 1.4 Tests de los accesores/context: default y override de base+prefijo; presencia/ausencia
      de `[branch]` (`TestHasBranchPlaceholder`, `TestWorktreeRoot`,
      `TestBaseBranchAndPrefixDefaultsAndOverride`, `TestBaseBranchPrefixRoundTripAndLegacy`,
      `TestContextWorktreeBlock`).

## 2. Orchestration del comando (kit)

- [x] 2.1 `kit/commands/vector/raw.md`: paso 9a worktree-resolve/create condicional antes de
      `vector spec create` — (a) detecta `WT_LAYOUT`; (b) resuelve root/base/prefijo desde el
      contexto; (c) reutiliza o crea con `git worktree add … -b <prefijo>/<slug> <base-branch>`
      (idempotente vía `git worktree list`); (d) inerte en no-worktree; (e) fallo de git (incl.
      stub previo) → error accionable verbatim, sin auto-borrar, sin registrar la card.
- [x] 2.2 `kit/commands/vector/bug.md`: el mismo paso 9a, simétrico a raw.
- [x] 2.3 Recordatorio en ambos (9b) de que el doc queda **dentro** del worktree (tracked en la
      rama feature). Sin lógica de limpieza/migración de stubs; sin escritura de estado a mano.

## 3. Propagación single-source

- [x] 3.1 `go generate ./internal/scaffold` → regenerados
      `cli/internal/scaffold/assets/commands/vector/{raw,bug}.md`.
- [x] 3.2 `TestAssetsMatchKit` verde (sin drift kit↔assets).

## 4. Tests

- [x] 4.1 Layout worktree: la creación produce worktree + rama + doc tracked.
      **Nota:** la creación (`git worktree add`) vive en la orquestación markdown — el binario
      sigue worktree-unaware por decisión de diseño (§Decisiones), así que no hay función Go que
      unit-testear. La detección de layout que la dispara sí está cubierta
      (`TestContextWorktreeBlock/worktree_layout_populated`).
- [x] 4.2 Idempotencia: segundo run reutiliza sin error.
      **Nota:** comportamiento de nivel orquestación (`git worktree list` → reuse en el markdown);
      no Go-unit-testable por la misma razón que 4.1.
- [x] 4.3 Regresión no-worktree: `spec-path` sin `[branch]` no crea worktree
      (`TestContextWorktreeBlock/non-worktree_is_inert`, `TestHasBranchPlaceholder`).
- [x] 4.4 Resolución de `<base-branch>`/`<prefijo>` desde config (default y override)
      (`TestBaseBranchAndPrefixDefaultsAndOverride`, `TestContextWorktreeBlock`).
- [x] 4.5 Stub suelto previo → fallo accionable (no silencioso, no auto-borrado).
      **Nota:** especificado en el paso 9a de ambos comandos (orquestación); no Go-unit-testable.

## 5. Verificación

- [x] 5.1 `go generate ./internal/scaffold` (antes de tests, para `TestAssetsMatchKit`).
- [x] 5.2 `gofmt -l .`, `go vet ./...`, `go test ./...`, `go build ./...` verdes.
- [x] 5.3 Sin regresiones en `spec create|propose|apply|sync` ni en el comportamiento no-worktree
      (suite completa verde; bloque `worktree` aditivo, omitido por consumidores que lo ignoran).
- [x] 5.4 Documentación de la extensión de `vector context`/config (Q-A) en los doc-comments de
      `WorktreeContext`, `HasBranchPlaceholder`, `WorktreeRoot`, `BaseBranch`/`BranchPrefix`.
