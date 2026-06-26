# Design — detect-ticket-from-worktree-name

## Decisiones clave

- **Match por slug exacto** tras quitar `<KEY>-` (precisión sobre recall): en somnio 24/38, 0 falsos
  positivos. Un slug derivado (worktree `mh-1385-resident-work-orders` vs spec
  `resident-work-orders-amendment`) **no** casa. Aceptado.
- **Enumeración multinivel acotada** bajo la raíz de worktrees, tolerando grupos `feat/chore/fix/docs`
  y branches de un nivel (`develop`). No se usa `filepath.Glob` de un solo `*`. La cota es una
  constante nombrada (`worktreeMaxDepth`, p. ej. 3) con comentario del porqué — no un número mágico.
- **Raíz de worktrees = prefijo literal de `changesTemplate()` antes de `[branch]`**. Sin `[branch]` en
  el template (repo no-worktree, p. ej. el propio Vector) → `WorktreeTicketKeys` devuelve mapa vacío y
  la feature es inerte (sin regresión). La comprobación del template vive **dentro** de
  `WorktreeTicketKeys`, no en `runSync` — `runSync` no inspecciona el template.
- **Último fallback en `detectTicket`**, gated en `defaultTicketProvider`. El artefacto
  (frontmatter/URL/cue/prefijo) **siempre gana** sobre el nombre del branch (es más explícito).
- **NO se toca** el glob de lectura de changes (`ChangesDirs`/`compileTemplate`/`FindSpecDocs`): los
  changes ya se leen completos desde la raíz `./openspec/changes`; tocarlo sería riesgo para otros
  repos sin beneficio. Este spec solo añade la señal de ticket.
- **Forma de key universal** `[A-Za-z][A-Za-z0-9]*-\d+`, normalizada a mayúsculas (`mh-1592`→`MH-1592`);
  denylist `ADR`/`RFC` reusada (`denylistedKey`). Worktree `<KEY>` puro (sin slug) → no se asocia.
- **Índice computado una vez** por `runSync` (no por change). Idempotencia y precedencia auto-vs-manual
  ya vienen de `Store.LinkSpec` — no se reimplementan.
- **Errores de I/O del scan por categoría**: error de permisos en un subdirectorio → se omite ese
  subárbol (tolerante, el índice sigue). Error al derivar la raíz del template (config malformada) → se
  propaga al caller. No se mezclan.

## Superficie

- `cli/internal/config/config.go` (NUEVO método): `WorktreeTicketKeys(repoRoot string) map[string]string`
  — deriva la raíz desde el prefijo de `changesTemplate()` antes de `[branch]`; scan multinivel acotado
  (`worktreeMaxDepth`); basename `<KEY>-<slug>`/`<KEY>`; key universal + denylist + upper; mapa slug→key
  (omite duplicados con keys distintas). Read-only; no usa `filepath.Glob` de un nivel; no toca
  `ChangesDirs`/`compileTemplate`. Molde: `ChangesDirs`/`changesTemplate`.
- `cli/cmd/vector/ticket.go`: `detectTicket(change, root, defaultProvider, keyPrefixes, branchKey string)`
  — `branchKey` como último parámetro; tras los tres fallbacks actuales, si nada matcheó,
  `defaultProvider != ""`, `branchKey != ""` y `!denylistedKey(branchKey)` →
  `&state.Ticket{Provider: defaultProvider, Key: branchKey, URL: "", Auto: true}`. No cambia los
  fallbacks previos.
- `cli/cmd/vector/main.go`: `runSync` — antes del loop, si `cfg.ResolvedDefaultTicketProvider() != ""`
  computar `idx := cfg.WorktreeTicketKeys(root)` una vez (mapa vacío si no aplica); pasar `idx[c.Name]`
  como `branchKey` a `detectTicket`. Sin cambios al contrato JSON ni al orden de precedencia.
- `cli/internal/state`: **sin cambios** (`LinkSpec` ya existe).
- `web/`: **sin cambios** — `Card.Ticket` ya se proyecta y renderiza.
- `docs/domain-contract.md` §5: añadir "nombre del worktree por slug" como último recurso en el orden
  de precedencia de `detectTicket`.

## Flujo

1. `vector sync` carga `cfg`; si `cfg.ResolvedDefaultTicketProvider() != ""`, computa
   `idx := cfg.WorktreeTicketKeys(root)` (vacío si el template no trae `[branch]`).
2. Por cada change: `detectTicket(change, root, provider, prefixes, idx[change.Name])`.
3. `detectTicket`: frontmatter → URL → cue/prefijo → **(nuevo)** branchKey gated → `Ticket{auto:true}`.
4. Persiste vía create/reconcile (`LinkSpec`), idempotente y sin pisar manual.

## Constraint pendiente (no abierto en código)

- **Slugs derivados** (worktree con sufijo extra vs change): hoy no casan; match por prefijo o
  normalización de slug se evalúa en una fase futura si el recall lo amerita.
- **Unificación** con `FindSpecDocs`/canonical worktree logic (hoy de un nivel): fuera de scope.
