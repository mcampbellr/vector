# Design — add-ship-command

## Decisiones clave

- **CLI-owns-writes**: el command orquesta (contexto, selección, precondición, secuencia de git/gh,
  PR-text, idempotencia, registro, reporte); el binario es el único escritor de `.vector/config.json`
  y del state. `ship.md` nunca edita `.vector/` a mano (lectura de `state.json` sí).
- **La orquestación de git vive en el kit, no en el binario** (D7): el binario solo posee escrituras
  de state (`config set-ship`, `spec pr`) y lecturas (`context`, `spec list`); commit/rebase/push/
  `gh pr create`/clipboard/colisión de untracked son orquestación en `ship.md` — no existe un
  `vector spec ship` driver de git. Mismo patrón que `/vector:apply` separa selección/orquestación
  (kit) de escritura de estado (binario).
- **PR ≠ Ticket** (D2): el slot `Ticket` (una por spec, `vector spec link`) es el tracker
  operacional; una PR es un concepto distinto. Campo `PR *PullRequest` nuevo + evento `pr.opened` +
  `vector spec pr` como único escritor, idempotente en la URL, **sin** transición de status.
- **`Store.RecordPR` modelado sobre `LinkSpec`**: toma `s.mu` una sola vez, `ReadSpec`, idempotencia
  en `pr.URL` (misma URL → no-op `(false,nil)`, sin evento duplicado; URL distinta → reemplaza +
  segundo `pr.opened`), bump `UpdatedAt`, `writeSpecFile`, `appendEvent`. **No** transiciona status
  (metadata pura, como `LinkSpec`/`RelateSpec`).
- **Ship config = `vector config set-ship`** (D3): mirror de `set-jira-mcp` pero con merge
  **incremental** por campo (cada flag opcional toca solo su campo; "empty = don't change"). Bloque
  `Ship *ShipConfig` opcional (`omitempty`), **opt-in estricto** — no escrito por `init`/`update`,
  igual que `applyModel`/`sketchEnabled`. `SchemaVersion` en 1.
- **Exclusiones de commit = default estático + doc del spec dinámico + override** (D6):
  `DefaultShipExcludeGlobs = ["openspec/"]` (path fijo real); el doc del propio spec se excluye
  dinámicamente vía `cfg.SpecDocPath(repoRoot, id)` (p. ej. `.vector/specs/<id>/spec.md` — **nunca**
  un literal `docs/specs/spec.md`, que no corresponde a ningún path real porque el spec-doc siempre
  resuelve por `<slug>`); `ship.excludeGlobs` añade globs extra. El command nunca hace `git add -A`.
- **Auth bootstrap opt-in, nunca automático** (D4): `ship.authBootstrap` es un spec que el usuario
  configura deliberadamente (path a sourcear / SSH alias); ausente, ship nunca sourcea nada implícito
  (nunca un `.envrc` del repo sin config explícita). El contenido nunca se loguea. Si la auth no se
  resuelve, detenerse y preguntar — nunca adivinar ni forzar.
- **PR-text inline** (D5): `ship.md` incluye el contrato de prosa de PR directamente (título
  Conventional Commits < 70 chars, resumen WHY, checklist de test-plan, inglés, clipboard
  best-effort) — sin depender de ninguna skill personal externa (regla de distribución).
- **Idempotencia en dos pasos** (D8): leer `pr` de `state.json`; confirmar/reparar contra el remoto
  con `gh pr list --head <branch> --base <base> --json url,number,state`. PR existente → surfacear +
  re-registrar, nunca duplicar. Varias PRs para la rama → mostrar todas y preguntar.
- **Nunca force-push; sin scanner de secretos propio** (D10): ship se apoya estructuralmente en las
  exclusiones y delega la detección de secretos al gate del repo (gitleaks/CI/pre-commit); si un hook
  bloquea el commit, reporta y se detiene.
- **Sin cambios de schema**: `config.SchemaVersion`, `state.SchemaVersion` y `state.EventVersion` se
  mantienen en 1; los campos nuevos (`Ship`, `PR`) y el evento (`pr.opened`) son aditivos/`omitempty`.

## Superficie

- `cli/internal/config/config.go`: `ShipConfig` + `Config.Ship *ShipConfig`, `ShipMode` (ask|auto) +
  `Valid()`, `DefaultShipExcludeGlobs`, y resolutores `ResolvedShipMode`/`ResolvedShipDraft`/
  `ResolvedShipExcludeGlobs`/`ResolvedShipBaseBranch(fallback)`/`ResolvedShipAuthBootstrap`;
  validación de `ship.mode` en `Load`.
- `cli/cmd/vector/config.go`: `runConfigSetShip` + dispatch `case "set-ship"` en `runConfig`.
- `cli/cmd/vector/context.go`: `ShipContext` + `ContextOutput.Ship *ShipContext`, poblado solo cuando
  `cfg.Ship != nil` (mirror de `JiraContext`).
- `cli/internal/state/types.go`: `PullRequest{URL,Number,Draft,OpenedAt}` + `SpecState.PR *PullRequest`.
- `cli/internal/state/event.go`: `EvtPROpened = "pr.opened"` + `PROpenedData{URL,Number,Draft}`.
- `cli/internal/state/store.go`: `Store.RecordPR(id, url, number, draft, actor, now)` — modelado
  sobre `LinkSpec`, sin transición.
- `cli/cmd/vector/spec_transitions.go`: `runSpecPR` (positionals `<id> <url>` + `--number`/`--draft`/
  `--json`), modelado sobre `runSpecLink`; `cli/cmd/vector/main.go`: `case "pr"` en `runSpec` + líneas
  de `usage()` para `spec pr` y `config set-ship`.
- `kit/commands/vector/ship.md` (+ copia vendored en `cli/internal/scaffold/assets/commands/vector/`,
  regenerada por `go generate`, `TestAssetsMatchKit` verde): la orquestación completa.
- `README.md`: fila de `/vector:ship` en la tabla "Commands Reference", inmediatamente después de
  `/vector:apply`.

## Flujo

`/vector:ship [<id>]` → `vector context --json` (una vez: base/mode/draft/excludeGlobs/authBootstrap
o defaults) → selección del spec en `review` (D11) → precondición de status → warning de árbol stale
no bloqueante (D12) → auth bootstrap opt-in (D4) → commit excluyendo `EXCLUDE_GLOBS` + `SPEC_DOC`
(D6, nunca `git add -A`) → rebase sobre `origin/<base>` con manejo de colisión de untracked (D9) →
PR-text inline (D5) → push `-u` (nunca `--force`) → detección de idempotencia (D8) → apertura de PR
draft según `ship.mode` (`ask` confirma / `auto` no) → `vector spec pr <id> <url>` registra `pr` +
`pr.opened` → reporte, próximo paso `/vector:close` (fuera de scope mergear).
