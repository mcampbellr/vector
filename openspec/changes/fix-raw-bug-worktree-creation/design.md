# Design — fix-raw-bug-worktree-creation

## Decisiones clave

- **Ownership en el orchestration, no en el binario**: la lógica de worktree-resolve/create vive
  en los comandos markdown (`/vector:raw`, `/vector:bug`). El binario sigue worktree-unaware
  (`MkdirAll` + write del doc dentro del worktree ya creado). Decisión tomada (§10 del spec); no
  se mueve a `vector spec create`.
- **Trigger por `[branch]`**: el paso se dispara **solo** cuando el `spec-path`/`changes-path`
  resuelto contiene el placeholder `[branch]`. En repos no-worktree el paso es **inerte** —
  comportamiento idéntico al actual, costo cero, sin git.
- **Branch/base configurables con defaults**: base = `base-branch` del config (default `main`);
  prefijo de rama configurable (default `feat/`); rama = `<prefijo>/<slug>`. Sin hardcodear
  `main`/`feat/`.
- **Idempotencia**: una consulta `git worktree list`; si `<worktree-root>/<slug>` ya está listado,
  reutilizar sin recrear ni error. Solo `git worktree add` cuando no existe.
- **Conflicto = abortar accionable, nunca auto-borrar**: ante un path suelto preexistente (stub de
  runs buggy previos) que impide `git worktree add`, surfacear el error de git íntegro + la acción
  manual sugerida. La recuperación/limpieza de stubs queda fuera de scope (responsabilidad del
  usuario), alineado con `security/destructive-ops-consent.md` (operación reversible, de bajo
  riesgo, que no fuerza sobre estado sucio).
- **Doc dentro del worktree**: el binario resuelve la ruta bajo `<worktree-root>/<slug>/…`; tras
  crear/reusar el worktree, el `vector spec create --body-file` deja el doc tracked en la rama
  feature, no en `code/main`.
- **CLI-owns-writes intacto**: el worktree es una operación de git sobre el repo del usuario, no
  sobre el estado. El orchestration no escribe estado a mano; el binario sigue siendo el único
  escritor del JSON.
- **Propagación single-source**: editar solo en `kit/`, `go generate ./internal/scaffold`,
  reinstalar binario, `vector update`. Nunca editar `cli/internal/scaffold/assets/**` a mano;
  `TestAssetsMatchKit` detecta drift.

## Superficie

- `kit/commands/vector/raw.md` — paso worktree-resolve/create condicional antes de escribir el doc.
- `kit/commands/vector/bug.md` — el mismo paso, simétrico a raw.
- `cli/internal/scaffold/assets/commands/vector/{raw,bug}.md` — regenerados por `go generate`
  (nunca a mano).
- **(Condicional, Q-A)** `cli/cmd/vector/context.go` — exponer `worktreeRoot`, `baseBranch`
  (default `main`), `branchPrefix` (default `feat/`) y un flag de layout `[branch]`.
- **(Condicional, Q-A)** `cli/internal/config/config.go` — helper `HasBranchPlaceholder` +
  accesores de root/prefijo, aditivos; sin tocar la resolución de `[branch]` existente
  (`SpecDocPath` ~L372-384, `branchPlaceholder` L355, `deriveChangesPath` L819);
  `SchemaVersion` permanece en 1.

## Flujo

1. `/vector:raw [idea]` o `/vector:bug [report]`.
2. El comando obtiene contexto (`vector context --json`) y detecta layout worktree (`[branch]`
   en `spec-path`/`changes-path`).
3. **No** `[branch]` → paso inerte, flujo continúa como hoy.
4. **Sí** `[branch]` → resolver `<worktree-root>` (prefijo antes de `[branch]`), `<base-branch>`
   (config, default `main`), `<prefijo>` (config, default `feat/`).
5. `git worktree list` lista `<worktree-root>/<slug>` → reutilizar (idempotente). Si no →
   `git worktree add <worktree-root>/<slug> -b <prefijo>/<slug> <base-branch>`.
6. `git worktree add` falla por path suelto preexistente (no worktree) → abortar accionable, sin
   auto-borrar.
7. Escribir el doc / `vector spec create --body-file`; cae dentro del worktree, tracked en la rama
   feature.

## Open questions

- **Q-A** — ¿El orchestration puede derivar `worktree-root`/`base-branch`/`prefijo`/flag `[branch]`
  desde el `vector context --json` actual? Si sí, no se toca Go; si no, extender `context` +
  helper en `config.go` (aditivo).
- **Q-B** — ¿`/vector:propose` y `/vector:apply` requieren ajuste propio una vez creado el
  worktree, o basta con heredarlo? Posible follow-up.
- **Q-C** — ¿`/vector:quick` debe ganar el mismo paso? Fuera de este fix salvo decisión de unificar.
- **Q-D** — Contrato fino de fallo accionable: rama feature preexistente sin worktree, `git`
  ausente, repo no-git pese a `[branch]` declarado.
- **Q-E** — Follow-up para detectar/migrar stubs sueltos previos (fuera de scope ahora).
