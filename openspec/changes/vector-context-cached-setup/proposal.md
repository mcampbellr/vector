# vector context — setup cacheado por sesión

## Why

Cada command del kit (`/vector:raw`, `/vector:bug`, `/vector:apply`, `/vector:comment`) re-deriva
de forma independiente el mismo contexto de setup al arrancar: globeo de specs, detección de
lenguaje, heurísticas de manifests para inferir comandos de build/lint/test. Esa re-derivación
consume tokens de orquestación de Opus en cada invocación y no aporta valor incremental, dado
que la información no cambia entre commands de la misma sesión.

Sin un punto de entrada único, cualquier mejora en la detección (soporte a nuevos stacks, mejor
heurística de Makefile) exige tocar cuatro commands en paralelo, con riesgo de divergencia. La
latencia de arranque tampoco es despreciable: cada command paga el costo de los globeos aunque
el resultado sea idéntico al del command anterior.

## What changes

- Nuevo subcomando de binario `vector context` (`context.go`) que devuelve en una sola llamada
  el contexto de setup del repo: `examplePath`, `language`, `buildCmd`, `lintCmd`, `testCmd`,
  `applyMode`, `ticketDetected`. Salidas `--json` (tooling) y texto humano legible (debug).
- Tres campos nuevos en `Config` (`BuildCmd`, `LintCmd`, `TestCmd`) con `omitempty` para
  backward compat. Función `DetectBuildCmds(repoRoot)` con goroutines concurrentes que lee
  Makefile, `go.mod`, `package.json`, `pyproject.toml` sin deps externas. Accessor
  `ResolvedBuildCmds()`.
- `runInit` y `runUpdate` llaman `DetectBuildCmds` y persisten los campos cuando están vacíos
  (o con `--force`). `vector context` detecta en runtime sin persistir si los campos siguen
  vacíos tras el `init`.
- `examplePath` siempre fresco: glob sobre `specPath` en runtime, no cacheado.
- Actualización de cuatro commands del kit para consumir `vector context --json` en el primer
  paso en vez de re-derivar: `raw.md`, `bug.md`, `apply.md`, `comment.md`. Los commands
  conservan fallback al detect manual cuando `CONTEXT` devuelve campos vacíos.
- Assets vendorizados regenerados en `cli/internal/scaffold/assets/commands/vector/`.
- Tests unitarios: `DetectBuildCmds` (table-driven, filesystem temporal) y `runContext`
  (ausencia de `config.json`, output JSON parseable).

## Scope

**In:**
- Subcomando `vector context` (read-only, stateless, sin deps externas).
- Campos `BuildCmd`/`LintCmd`/`TestCmd` en `Config` + `DetectBuildCmds` + `ResolvedBuildCmds`.
- Integración de `DetectBuildCmds` en `runInit`/`runUpdate`.
- Actualización de `raw.md`, `bug.md`, `apply.md`, `comment.md` con consumo de `CONTEXT`.
- Tests unitarios de la lógica nueva.

**Out:**
- Cache en memoria entre invocaciones (el binario es stateless; la persistencia es `config.json`).
- Subcomando `vector context set` (los campos se setean vía `vector init`/`vector update`).
- Invalidación automática de cache cuando el usuario cambia un manifest en tiempo real (V1: `vector update`).
- Watchers de filesystem.
- Detección de CI (GitHub Actions, CircleCI, etc.) — solo manifests locales.
- Actualización de commands del kit distintos de los cuatro enunciados.
- Panel web / SSE — `vector context` es lectura síncrona.

Authored spec: `.vector/specs/vector-context-cached-setup/spec.md`.
