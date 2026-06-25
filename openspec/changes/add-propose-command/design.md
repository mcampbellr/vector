# Design — add-propose-command

## Decisiones clave

- **CLI-owns-writes**: el command orquesta (detecta modo, genera/delega artefactos, prompts); el
  binario es el único escritor del state (`vector spec propose`).
- **`open` no estampa `StartedAt`**: `open` = el change existe pero el trabajo no arrancó;
  `StartedAt` se setea en `in-progress` (`/vector:apply`). (Corrección del validator.)
- **Adapter, no reimplementación**: delegar a OpenSpec si el repo es un proyecto OpenSpec;
  fallback nativo liviano si no. Paridad total = non-goal.
- **Detección** = "¿el repo es un proyecto OpenSpec?" (existe `openspec/` con estructura), no
  "¿está el CLI `openspec` en PATH?". Un CLI global no implica que el repo use OpenSpec.

## Superficie

- `cli/internal/state/event.go`: `EvtSpecProposed` + `ProposedData{Change, Artifacts}`.
- `cli/internal/state/store.go`: `ProposeSpec(id, openspec, actor, now)` — valida `draft`,
  flipea a `open`, provenance, dos eventos (dentro del mutex).
- `cli/cmd/vector/main.go`: `vector spec propose <id> [--change] [--artifacts] [--dry-run] [--json]`;
  acepta el id como positional inicial aunque sigan flags; idempotente (`already open`).
- `cli/internal/config/config.go`: `ProposeBranch` (override del worktree; reusa `Branch`).
- `kit/commands/vector/propose.md`: el command (adapter delegate/native).

## Flujo

`/vector:propose <id>` → valida draft (`--dry-run`) → resuelve `CHANGE_DIR` (config branch o root)
→ detecta modo → genera artefactos (delegate/native) → `vector spec propose` flipea el state →
reporta + sugiere `/vector:apply`.
