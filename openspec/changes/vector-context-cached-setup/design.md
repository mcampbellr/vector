# Design — vector-context-cached-setup

## Decisiones clave

- **Read-only como invariante fuerte**: `runContext` nunca llama `config.Write` ni toca
  `activity.jsonl`. La única escritura permitida de los nuevos campos es en `runInit`/`runUpdate`.
  Esto preserva la regla CLI-owns-writes y hace el subcomando seguro para invocarse en cualquier
  context del kit sin efectos secundarios.
- **Cache en disco (`config.json`), no en memoria**: el binario es stateless. Los campos
  `BuildCmd`/`LintCmd`/`TestCmd` se persisten en `config.json` con `omitempty` para backward
  compat total: un `config.json` antiguo los carga como `""` sin error ni migración.
- **`examplePath` siempre fresco**: se resuelve con un glob sobre `specPath` en runtime, nunca
  cacheado. Justificación: cada `/vector:raw` puede añadir un spec; cachear requeriría
  invalidación, que es la arista más compleja. El glob es barato (< 50ms, un solo nivel).
- **Fallback en runtime sin persistir**: si `BuildCmd`/`LintCmd`/`TestCmd` están vacíos en
  `config.json` (repo recién inicializado o manifest no reconocido), `runContext` llama
  `DetectBuildCmds` en goroutines y los incluye en el JSON de output sin escribirlos a disco.
  Idempotente; no confunde a `runInit`/`runUpdate`.
- **Goroutines stdlib en `DetectBuildCmds`**: `sync.WaitGroup` + goroutines. Los reads de
  Makefile, `go.mod`, `package.json`, `pyproject.toml` son I/O independientes. Sin dependencias
  externas; stdlib pura. Cada goroutine que falla retorna `""` sin propagar error a las demás.
- **Fallback en los commands del kit**: si `vector context` falla (binario no en PATH o exit 1),
  el command emite una advertencia de una línea y continúa con el comportamiento anterior de
  re-derivación local. Ningún command queda bloqueado por la ausencia del subcomando.
- **Prioridad de manifests por campo**: Makefile con target explícito gana por campo; en su
  ausencia, `go.mod` (Go), `package.json` (Node), `pyproject.toml`/`setup.py` (Python). Un
  repo puede mezclar (ej. Makefile sin lint-target + `go.mod` aporta `golangci-lint run`).
- **`ticketDetected` es solo un booleano**: el command del kit solo necesita saber si la
  detección de tickets está activa; el provider detallado ya está en `config.json`.

## Superficie

- `cli/cmd/vector/context.go` (NUEVO): `ContextOutput` struct (7 campos JSON), `runContext`
  (flags `--repo-root`, `--json`, `--dry-run`). Referencia: `standup.go`.
- `cli/cmd/vector/main.go`: `case "context"` en el switch; `usage()` actualizado; `runInit` y
  `runUpdate` integran `DetectBuildCmds` + persistencia.
- `cli/internal/config/config.go`: campos `BuildCmd`/`LintCmd`/`TestCmd` (`omitempty`);
  `DetectBuildCmds(repoRoot string) (build, lint, test string)`; `ResolvedBuildCmds()`.
- `cli/internal/config/config_test.go`: tests table-driven de `DetectBuildCmds` con
  `t.TempDir()`.
- `kit/commands/vector/raw.md`: reemplaza paso 2 (glob) + paso 3 (detect-language) por
  `vector context --json`; renumera pasos subsiguientes.
- `kit/commands/vector/bug.md`: añade paso 0 `vector context --json`; reemplaza paso 4.
- `kit/commands/vector/apply.md`: añade paso 0; actualiza gate de build/test en §4.
- `kit/commands/vector/comment.md`: añade paso 0; actualiza §7a.3 (verificación gate).
- `cli/internal/scaffold/assets/commands/vector/{raw,bug,apply,comment}.md`: REGENERAR.

## Flujo

`/vector:raw` (ejemplo) → ejecuta `vector context --json` (paso 0/1) → binario lee
`config.json`, resuelve campos estables, glob `specPath`, detecta manifests si vacíos →
devuelve `ContextOutput` JSON a stdout → command extrae `CONTEXT.examplePath`,
`CONTEXT.language` → continúa sin re-derivar. Si `vector context` falla → warning + fallback
al glob/detect local (comportamiento anterior).

`vector init`/`update` → `DetectBuildCmds(root)` en goroutines → si campos vacíos en cfg (o
`--force`) → setea y persiste vía `config.Write`.

## API de salida (`--json`)

```json
{
  "examplePath": "docs/specs/add-foo/spec.md",
  "language": "es",
  "buildCmd": "go build ./...",
  "lintCmd": "golangci-lint run",
  "testCmd": "go test ./...",
  "applyMode": "ask",
  "ticketDetected": true
}
```

Exit `0` éxito; exit `1` error (stderr). Único error esperado: `config.json` ausente.
