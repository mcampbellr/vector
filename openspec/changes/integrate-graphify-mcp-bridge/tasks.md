# Tasks — integrate-graphify-mcp-bridge

## 1. Paquete `internal/graph`

- [x] 1.1 `cli/internal/graph/graph.go`: `Detect(repoRoot) (Status{Installed, GraphExists}, error)` — `exec.LookPath("python3")` + chequeo del módulo `graphify` + `os.Stat(graphify-out/graph.json)`.
- [x] 1.2 `ServeCommand(repoRoot) (*exec.Cmd, error)` construye (sin ejecutar) `python3 -m graphify.serve graphify-out/graph.json`; error accionable si `Detect` falla.
- [x] 1.3 `BuildCommand(repoRoot) (*exec.Cmd, error)` — esqueleto con `TODO` explícito y guarda de error (invocación exacta **TBD**, Open Q3/Q4).
- [x] 1.4 `graph_test.go`: table-driven de los 3 casos de detección (ambos / solo graphify / ninguno), con `python3`/`graphify` mockeados vía `PATH` de test.

## 2. Subcomando `vector graph`

- [x] 2.1 `cli/cmd/vector/graph.go`: `newGraphCmd()` parent (error de uso sin subverbo, patrón `newSpecCmd`).
- [x] 2.2 `newGraphMCPCmd()` `[--repo-root]`: `graph.Detect` → graceful-fail (stderr + exit ≠ 0) o proxy `cmd.Run()` con `Stdin/Stdout/Stderr` conectados (patrón `signal.NotifyContext` de `serve.go:95` si aplica).
- [x] 2.3 `newGraphBuildCmd()` `[--repo-root] [--json]`: error explícito del esqueleto hasta resolver Open Q3/Q4.
- [x] 2.4 Registrar `newGraphCmd()` en `root.go` (`root.AddCommand(...)`).
- [x] 2.5 `graph_test.go`: `vector graph mcp` sin graphify → exit ≠ 0 + mensaje en stderr (caso real, sin mocks).

## 3. Seed de `.mcp.json`

- [x] 3.1 `scaffold.SeedMCPConfig(repoRoot, opts) (FileResult, error)`: lee/mergea `.mcp.json` (JSON inválido → error accionable), preserva entradas ajenas de `mcpServers`, añade/actualiza solo `vector-graph`, escribe solo si hay cambio real; respeta `DryRun`/`Force`.
- [x] 3.2 Flag `--with-graph-mcp` en `newInitCmd`/`newUpdateCmd`; llamar `SeedMCPConfig` solo con el flag; `usage()` actualizado; reporte con formato `%-12s %s` de `SeedCommands`.
- [x] 3.3 `scaffold_test.go`: crea si no existe; mergea preservando otras entradas; idempotente. `newInitCmd`/`newUpdateCmd`: `--with-graph-mcp` dispara el seed; sin el flag no toca `.mcp.json`.

## 4. Kit (agentes)

- [x] 4.1 `kit/agents/_shared/graph-context.md`: las 7 tools `mcp__vector-graph__*`, regla opt-in / nunca bloqueante, cuándo aportan valor vs. cuándo ignorarlas.
- [x] 4.2 Añadir las 7 tools al frontmatter `tools:` + pointer a `_shared/graph-context.md` en `vector-spec-refiner`, `vector-bug-refiner`, `vector-apply-impl`, `vector-feasibility-reviewer`; en `vector-spec-refiner` complementar la lectura de `graphify-out/GRAPH_REPORT.md` (línea 29) con la consulta interactiva. Sin cambiar tier ni output shape.
- [x] 4.3 Regenerar copias embebidas: `go generate ./internal/scaffold` (nunca a mano); `TestAssetsMatchKit` verde.

## 5. Docs

- [x] 5.1 `README.md`: documentar `vector graph mcp` / `vector graph build`, el flag `--with-graph-mcp`, y que graphify es dependencia **opcional** cuya ausencia no rompe Vector.

## 6. Verificación

- [x] 6.1 `go -C cli generate ./internal/scaffold`, `gofmt -l cli` (sin salida), `go -C cli vet ./...`, `go -C cli test ./...`, `go -C cli build ./...` — todos verdes.
- [x] 6.2 Graceful-fail verificado en esta máquina (graphify ausente): `vector graph mcp` → exit ≠ 0 + stderr accionable.
- [x] 6.3 Regresión cero de los 4 agentes con el server MCP no conectado.

## 7. Open questions

- [x] 7.1 Resolver o trasladar a seguimiento las Open questions del spec (shape de `.mcp.json`, API real de `graphify.serve` y nombres de tool, resolución del intérprete Python, hot-reload, corpus `.vector/`, versionado de `graphify-out/`, JSON corrupto) antes de cerrar la fase.
