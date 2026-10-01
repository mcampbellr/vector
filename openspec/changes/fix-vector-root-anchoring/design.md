# Design — fix-vector-root-anchoring

## Decisiones clave

- **Resolución en dos niveles, deliberadamente distintos**: (1) walk-up **transparente** para
  comandos generales (`resolveRepoRoot`), que ancla en silencio al ancestro cuando existe; y (2)
  **guard explícito solo en `vector init`**. `init` necesita distinguir "estoy en la raíz canónica"
  de "estoy en un subdirectorio de una raíz canónica" para poder **rechazar** la creación anidada
  en vez de adoptarla en silencio. Compartir el walk-up transparente en `init` habría hecho
  indistinguibles ambos casos.
- **`init` falla, no adopta**: desde un subdirectorio con ancestro válido y sin `--force`, `init`
  retorna error nombrando la ruta absoluta del ancestro y sugiriendo `--force` en el mismo mensaje.
  Elegido sobre silent-adopt por seguridad (regla 2 del reporte crudo).
- **`--repo-root` explícito permanece final**: se salta el walk-up en todos los comandos; la
  precedencia establecida hoy (`main.go:1016-1028`) no cambia.
- **Stray = se salta, no se adopta, no detiene la búsqueda**: un `.vector/` sin `config.json`
  (o con uno que no deserializa) se registra para warning y el walk-up **sigue subiendo**, de modo
  que un stray intermedio nunca bloquea encontrar el ancestro real.
- **El walk-up vive en `internal/config`, no en `cmd/vector`**: respeta la separación existente
  entre "dónde vive el config" (`internal/config`) y "cómo se invoca desde la CLI"
  (`cmd/vector`). `FindAncestorConfig` es de solo lectura (`os.Stat`/`os.ReadFile`), sin efectos
  secundarios, y no decide nada sobre límites de git/worktree.
- **Warnings solo en la rama humana**: `FindAncestorConfig` **retorna** `strayDirs`; no imprime.
  Cada comando decide si los imprime con `ui.Warning`, nunca dentro de un branch `--json`
  (garantía de `--json` byte-idéntico).
- **`doctor adopt` reutiliza el idioma `--force`/dry-run ya establecido** por `newInitCmd`
  (`main.go:65-120`): sin flag imprime el plan sin tocar el filesystem; con `--force` muta.
- **`doctor adopt` mueve (no copia) y borra el stray tras migrar**: la consolidación es destructiva
  sobre el stray, gated por `.claude/rules/security/destructive-ops-consent.md`. El borrado es el
  **último** paso, solo tras migración exitosa — un fallo a mitad deja el stray intacto.
- **Conflicto de slug = abortar esa migración puntual**, reportando el conflicto; nunca
  sobrescribir en silencio (regla 5 de `destructive-ops-consent.md`).
- **Guard exclusivo de `init`**: `vector update` y el resto de comandos usan el walk-up
  transparente sin caso especial.
- **Sin prompt interactivo**: el binario es no interactivo por diseño
  (`.claude/rules/architecture/distribution-packaging.md`); el consentimiento es el flag `--force`,
  complementado por el guardrail de `kit/` para que el agente pida confirmación al usuario antes
  de pasarlo.
- **Sin `relatedTo`** y **`SchemaVersion` sin cambios**: decisiones explícitas del spec (§10).

### Corrección — nearest-wins no basta cuando la raíz del workspace no es git

- **Por qué nearest-wins solo se queda corto**: nearest-wins asume que "el store más cercano" es
  siempre el correcto. Eso es cierto en un repo git simple, pero se rompe en un layout
  bare+worktree donde la **raíz del workspace no es un repo git** y cada worktree lleva su
  **propio** `.vector/config.json` trackeado (`specStore: vector`). Ahí, correr un comando dentro
  de un worktree ancla al store del worktree — que existe y es válido — **shadowing** (ocultando)
  el store canónico de la raíz del workspace que está más arriba. Es exactamente el bug de
  split-board que el propio dogfooding de Vector sufrió: no es un stray (ambos configs son
  válidos), así que la detección de strays existente no lo cubre.
- **Dos pines, precedencia explícita sobre el walk-up**:
  1. **`VECTOR_REPO_ROOT` (env var, efímero)**: cuando está seteada y no vacía, su valor absoluto
     es la raíz; el walk-up se salta por completo. Pensado para scripts/CI que necesitan pinnear
     sin tocar ningún archivo.
  2. **`stateRoot` (campo de config, persistente)**: cuando el config al que el walk-up ancla
     lleva `stateRoot` no vacío, se re-ancla ahí. Se resuelve relativo al directorio de **ese**
     config (no al cwd) cuando no es absoluto, y se valida con `config.Load` antes de aceptarlo —
     un `stateRoot` inválido o que apunta a sí mismo (auto-referencia/loop trivial) se ignora y cae
     al resultado nearest-wins, nunca aborta.
  - **Precedencia completa (mayor→menor)**: `--repo-root` explícito > `VECTOR_REPO_ROOT` >
    `stateRoot` (del config al que ancla el walk-up) > walk-up nearest-wins > `git rev-parse
    --show-toplevel` > `os.Getwd()`. `--repo-root` y el env pin **saltan el walk-up entero**
    (ni siquiera se calculan strays/shadowing); `stateRoot` se evalúa **después** de que el
    walk-up ya encontró un ancestro (es una corrección posterior a ese resultado, no un atajo).
  - **Sin encadenar pines**: `ResolveStateRootPin` valida el target con `Load` pero **no**
    persigue el `stateRoot` del target de forma recursiva — un solo salto. Eso hace imposible un
    loop no trivial (A→B→A) sin necesidad de detectarlo; solo se guarda contra la auto-referencia
    trivial (A→A).
- **Warning de shadowing, no error**: cuando el walk-up ancla a un store pero existe un ancestro
  **más arriba todavía** y ningún pin redirigió, `resolveRepoRootStrays` devuelve un
  `*ShadowNotice` (nunca imprime — la misma disciplina que `strayDirs`). Los call-sites de rama
  humana (`update`, `spec create`, `serve`, `doctor`) lo imprimen con `ui.Warning` nombrando ambas
  rutas y sugiriendo `stateRoot`/`VECTOR_REPO_ROOT`; la rama `--json` de esos mismos comandos
  nunca lo consulta (garantía de `--json` byte-idéntico intacta).
- **`SchemaVersion` sigue en 1**: `stateRoot` es un campo `omitempty` más, siguiendo exactamente
  el precedente de `language`/`applyModel`/`ship` — aditivo, un config legado sin el campo carga
  `StateRoot == ""` y el comportamiento no cambia (nearest-wins puro, como hoy).

## Superficie

- `cli/internal/config/config.go` — nueva `FindAncestorConfig`; `Resolve`/`Load`/`Exists`/`Write`
  intactas en firma y comportamiento.
- `cli/cmd/vector/main.go` — `resolveRepoRoot` (L1018) gana el walk-up; `newInitCmd` (L65) extrae
  su target crudo (p. ej. `rawTargetRoot`) y añade el guard de ancestro antes del bloque
  `cfgExisted && !force` ya existente (L97-102).
- `cli/cmd/vector/doctor.go` — **NUEVO**: `newDoctorCmd()` (scan) + subcomando `adopt`. Layout
  plano, siguiendo `spec_transitions.go` / `ticket.go` / `sketch.go`.
- `cli/cmd/vector/root.go` — registra `newDoctorCmd()` en el bloque `root.AddCommand(...)`
  (L61-72), sin reordenar ni quitar comandos.
- Tests: `cli/internal/config/config_test.go` (MOD), `cli/cmd/vector/root_resolution_test.go`
  (NUEVO), `cli/cmd/vector/doctor_test.go` (NUEVO), `golden_test.go`/`testdata/golden/` (verificar
  sin drift; caso golden para `vector doctor --json` si aplica).
- `kit/agents/_shared/root-anchoring-guardrail.md` (NUEVO) + `kit/commands/vector/{raw,bug,
  quick}.md` + `kit/CLAUDE.md`.
- **No se toca**: `web/`, `internal/board`, el esquema de `SpecState`, ni `internal/state` salvo
  como relocalización de archivos vía `doctor adopt`.

## Flujo

**Walk-up general** (comando sin `--repo-root`, p. ej. `vector spec create` desde `website/`):

1. `resolveRepoRoot("")` obtiene el `cwd` (`website/`).
2. `config.FindAncestorConfig(cwd)` sube hasta encontrar `<raíz>/.vector/config.json` válido →
   retorna `<raíz>` como store canónico.
3. Strays en el camino se acumulan en `strayDirs` y no detienen la búsqueda.
4. El comando opera contra `<raíz>`; en rama humana imprime `ui.Warning` si hubo strays.
5. Sin ancestro → fallback actual (`git rev-parse --show-toplevel` → `os.Getwd()`), sin cambios.

**Guard de `vector init`** (desde `website/`, con `<raíz>/.vector/config.json` existente):

1. `newInitCmd` calcula el target crudo (explícito → git toplevel → cwd) = `website/`.
2. `config.FindAncestorConfig(filepath.Dir(target))` — busca **estrictamente por encima**.
3. `found && !force` → error `an ancestor Vector store already exists at <raíz>; refusing to create
   a nested store at <target> (use --force to override)`. No escribe nada.
4. Con `--force` → flujo actual sin cambios (override deliberado).
5. `init` corrido **en la propia raíz canónica** no dispara el guard (busca estrictamente arriba);
   sigue la lógica `cfgExisted && !force` existente.

**`vector doctor`**:

1. `vector doctor` resuelve la raíz canónica (mismo walk-up) y escanea recursivamente por debajo
   (`filepath.WalkDir`) buscando `.vector/` sin `config.json`. Solo lectura; `ui.Table` en rama
   humana, array en `--json`. Lista vacía → exit 0.
2. `vector doctor adopt <ruta-stray>` sin `--force` → imprime el plan (specs a mover, entradas de
   `activity.jsonl` a anexar, estado local) sin mutar.
3. Con `--force` → mueve los directorios de specs al mismo layout relativo bajo
   `<canónico>/.vector/specs/`, anexa cronológicamente por timestamp las líneas de
   `activity.jsonl`, mueve el estado local restante y borra el `.vector/` stray solo si todo
   completó sin error.

## Open questions

1. **Definición exacta de "config.json válido"** en el walk-up: ¿basta con que deserialice sin
   error (diseño mínimo asumido), o debe además tener `schemaVersion`/`specPath` no vacíos?
   **RESUELTA (implementación)** — válido == `config.Load(dir)` sin error. Reusa la validación ya
   existente (parseo + enums de `defaultTicketProvider`/`applyModel`/`ship.mode`) en vez de
   introducir un segundo criterio que podría divergir. Un `{}` sintácticamente válido cuenta como
   válido; endurecerlo exigiría campos obligatorios y rompería configs antiguas — fuera de fase.
2. **Límite de walk-up vs límites de git/worktree**: el propio workspace de Vector es bare+worktree
   (raíz no-git con `.vector/` + `code/main/.vector/` versionado). El walk-up no debe cruzar
   incorrectamente hacia el `.vector/` de un worktree o repo padre distinto.
   **RE-ABIERTA y RESUELTA con corrección** — la primera resolución ("el más cercano gana, sin
   corte explícito necesario") describía el comportamiento **observado**, pero no lo declaraba
   **deseado**: nearest-wins puro es precisamente el bug de split-board que el propio dogfooding
   de Vector sufrió (`vector spec list` desde `code/root-anchoring/cli` viendo los specs del
   worktree, ocultando los de la raíz del workspace, sin ningún error ni warning). La resolución
   final añade **dos pines con precedencia sobre nearest-wins** (`VECTOR_REPO_ROOT` env,
   `stateRoot` config field — ver "Corrección" arriba) más un **warning de shadowing** cuando
   nearest-wins ancla sin que ningún pin lo redirija. Nearest-wins **sigue siendo el default sin
   pin** (sin regresión para repos de un solo store); lo que cambia es que ahora hay una vía
   explícita para anclar al ancestro correcto en un layout multi-store, y el usuario es avisado
   cuando no lo ha hecho.
3. **Merge de `activity.jsonl` en `doctor adopt`**: se asume append cronológico simple por
   timestamp; deduplicación/reordenamiento sofisticado queda `TBD`. **Implementado como append +
   reordenado estable por `ts`, conservando cada línea verbatim** (sin re-encoding, sin pérdida de
   campos desconocidos); las líneas no parseables se conservan y ordenan primero (`ts` cero). La
   deduplicación sigue `TBD`.
4. **Backup explícito del stray antes de borrarlo**: hoy el contenido queda movido (no perdido)
   antes del borrado, sin backup independiente adicional. Si la regla 1 de
   `destructive-ops-consent.md` exige uno explícito aquí, `TBD`. **Sigue `TBD`** — mitigado por el
   orden (mover primero, borrar último) y por el dry-run por defecto, pero no hay copia aparte.
5. **Semántica de conflicto parcial** en `doctor adopt --force`: se asume "aborta esa migración
   puntual, continúa con el resto"; confirmar si debe ser todo-o-nada. **RESUELTA como se asumía**,
   con un refuerzo: ante cualquier conflicto el stray **no** se borra, porque el spec conflictivo
   sigue viviendo solo ahí. El usuario resuelve y reejecuta `adopt`.
6. **Enforcement de scoping sobre el campo `repo`** (ya existente y cableado end-to-end): si Vector
   debe *forzar* que los specs de subproyecto lo usen dentro del store raíz en vez de tolerar
   stores anidados, y cómo la doc lo sanciona como mecanismo único. Fuera de esta fase. `TBD`.
## Rebaseline — workspace root is authoritative

This section replaces the nearest-wins strategy described below. Root discovery collects all valid
physical ancestors and selects the outermost workspace store. Inner `--repo-root`,
`VECTOR_REPO_ROOT`, and `stateRoot` values fail rather than creating or selecting a split board.
`init --force` does not override this invariant. `doctor adopt` uses an exclusive canonical-root
lock, performs all collision checks before its first migration, writes activity via temp-file rename,
and rolls completed artifact moves back if a later migration fails.
