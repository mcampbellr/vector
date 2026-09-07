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
   error (diseño mínimo asumido), o debe además tener `schemaVersion`/`specPath` no vacíos? `TBD`.
2. **Límite de walk-up vs límites de git/worktree**: el propio workspace de Vector es bare+worktree
   (raíz no-git con `.vector/` + `code/main/.vector/` versionado). El walk-up no debe cruzar
   incorrectamente hacia el `.vector/` de un worktree o repo padre distinto. Mecanismo exacto:
   `TBD`. **Riesgo alto en este repo — validar antes de dar el fix por bueno.**
3. **Merge de `activity.jsonl` en `doctor adopt`**: se asume append cronológico simple por
   timestamp; deduplicación/reordenamiento sofisticado queda `TBD`.
4. **Backup explícito del stray antes de borrarlo**: hoy el contenido queda movido (no perdido)
   antes del borrado, sin backup independiente adicional. Si la regla 1 de
   `destructive-ops-consent.md` exige uno explícito aquí, `TBD`.
5. **Semántica de conflicto parcial** en `doctor adopt --force`: se asume "aborta esa migración
   puntual, continúa con el resto"; confirmar si debe ser todo-o-nada. `TBD`.
6. **Enforcement de scoping sobre el campo `repo`** (ya existente y cableado end-to-end): si Vector
   debe *forzar* que los specs de subproyecto lo usen dentro del store raíz en vez de tolerar
   stores anidados, y cómo la doc lo sanciona como mecanismo único. Fuera de esta fase. `TBD`.
