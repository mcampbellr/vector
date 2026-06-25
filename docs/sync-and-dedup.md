# Vector — Sync & deduplicación (LOCKED)

> Cómo `vector sync` proyecta OpenSpec changes + spec docs al board produciendo **una card por
> work-item, sin duplicados**. Es la razón de ser de Vector: lo que antes se duplicaba, acá queda
> bien. Verificado contra un workspace bare+worktrees real (vanguardhq).

## Principio: identidad = slug

Un work-item se identifica por su **slug**, nunca por `(worktree, slug)` ni por la ruta física.
Toda copia/representación del mismo slug colapsa a **una** card.

## Fuentes que lee sync

| Fuente | Qué aporta |
|--------|-----------|
| `changes/<name>/` (todos los worktrees **+** el árbol root) | work-items activos/archivados → cards |
| `changes/archive/<fecha>-<name>/` | archivados (id sin prefijo de fecha) |
| spec docs en `spec-path` (todos los worktrees) | specs autorados (`/idea` o manual) → `draft` si no hay change |
| `openspec/specs/` (capabilities aplicadas) | **no** se importan (catálogo, no work-items) |

En layouts bare+worktrees los changes suelen estar **repartidos**: los activos en los worktrees y
los archivados en el árbol root. Sync lee **ambos** y colapsa por nombre.

## Selección de la copia canónica (Tipo A — mismo slug en N worktrees)

Cada worktree es un checkout del mismo git, así que un spec doc existe físicamente en varios a la
vez. Para cada slug se elige **una** copia canónica con esta prioridad:

1. el worktree de `branch` (config) — p. ej. `main`,
2. un worktree llamado igual que el slug (un idea/change en progreso, aún no mergeado),
3. el lexicográficamente primero (determinístico).

`branch` es una **preferencia/tie-breaker, no un filtro**: sync lee todos los worktrees, así un
spec/change que vive solo en su propio worktree (trabajo en progreso) **nunca se oculta**.
`specDoc` apunta a la copia canónica (prefiere `main`).

## Dedup cross-slug (Tipo B — spec ↔ change de otro nombre)

Un spec de `/idea` se implementa como un change con **otro slug**; al mergear, el board tendría la
card del change **y** un draft del mismo feature. La regla "mismo slug gana" no alcanza.

- **Mecanismo determinístico:** el spec declara en su **frontmatter** `supersededBy: <change-slug>`
  (o `status: superseded|implemented`). Sync **suprime** ese spec; el change es la única card.
- **Sin inferencia por nombre** (riesgo de falsos positivos). El link viene de metadata explícita.
- **Cómo se pobla sin taggear a mano:** el binario no puede deducirlo (no hay señal estructural
  fiable), así que la **inteligencia vive en el command `/vector:sync`**: en el primer sync lee el
  contenido de los changes, **propone** los matches, el usuario **confirma**, y el command
  **escribe** `supersededBy` en el spec. Persiste → los siguientes syncs son silenciosos.

## Estados (de un change)

`archived` (en archive) · `open` (0 tasks hechas) · `in-progress` (parcial) · `review` (todas
hechas, o solo quedan tasks de **QA/verificación manual**). Sin `tasks.md` parseable → `open`.

## Provenance e idempotencia

- Cards de change: `openspec{change,artifacts}`. Specs sueltos: `draft` con `source:sync`.
- **Aditivo**: re-sync solo agrega lo que falta; `--reconcile` actualiza el status de cards
  sync-owned. Drafts de `/vector:raw` y cualquier card existente **nunca** se tocan.

## El patrón de producto (flujo del usuario)

1. El usuario corre `/vector:sync`.
2. Si hay duplicados (specs cubiertos por changes de otro slug, o copias multi-worktree), Vector
   **lo reporta y propone opciones** — no elige en silencio.
3. El usuario confirma una vez; la decisión se **persiste** (`supersededBy` en el spec).
4. Futuros syncs no re-duplican ni re-preguntan.

> Implementación: `cli/internal/openspec` (lectura), `cli/internal/config` (colapso canónico +
> `supersededBy` + multi-árbol), `cli/cmd/vector` (`runSync`), `kit/commands/vector/sync.md` (el
> match+persist). Mapa comando→state: `docs/domain-contract.md` §5.
