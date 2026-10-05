# Design — integrate-graphify-mcp-bridge

## Decisiones clave

- **Bridge binario + shell-out delegado, ownership híbrido (B+C)**: `vector graph mcp` es el
  `command` real que invoca `.mcp.json` (así el repo del usuario nunca hardcodea `python3`); en
  runtime detecta la dependencia opcional y, si está satisfecha, delega a
  `python3 -m graphify.serve graphify-out/graph.json` como proxy stdio transparente. El **ownership
  del proceso** (arranque, ciclo de vida, cierre) es de **Claude Code**; Vector no supervisa
  procesos hijos (coherente con que hoy no hay spawn de proceso de larga vida en `cli/` —
  `serve.go` solo hace `httpServer.Serve`).
  - Descartada **opción A** (acoplar a `vector serve`): es efímero y solo corre al administrar el
    board (`serve.go:23-27`); dejaría el grafo inaccesible en el caso común (`/vector:apply`,
    `/vector:raw` sin `serve` corriendo).
  - Descartada **opción C** (`vector graph serve` on-demand sin `.mcp.json`): reintroduce el paso
    manual que `.mcp.json` existe para eliminar.
- **CLI-owns-writes extendido a `.mcp.json`**: el binario sigue siendo el único escritor; ningún
  agente escribe `.mcp.json` a mano. `SeedMCPConfig` es aditivo/idempotente (`created`/`overwritten`/
  `skipped`), nunca sobrescribe entradas `mcpServers` ajenas.
- **Escritura de `.mcp.json` = solo con `--with-graph-mcp`**, nunca automática — satisface
  `security/destructive-ops-consent.md` para la primera escritura de Vector fuera de `.claude/`.
- **El binario apunta a sí mismo, no a `python3`**: `.mcp.json` invoca `vector graph mcp`, evitando
  hardcodear el intérprete (`product/principles.md` — agnóstico al repo del usuario).
- **Paquete `internal/graph` agnóstico al dominio de specs**: sin dependencias de
  `internal/config`/`internal/state`, análogo a `internal/openspec` (detección read-only).
- **Diagnóstico solo a stderr**: `vector graph mcp` nunca escribe a stdout salvo el protocolo
  MCP/JSON-RPC proxied; todo mensaje va a stderr para no corromper el canal.
- **Adopción de tools = opt-in en 4 agentes**, sin cambio de tier de modelo; invocar una tool MCP
  no es motivo para subir de tier (`product/token-routing.md`). Doctrina en `_shared/` en vez de
  duplicar la guía por agente.

## Superficie

- `cli/internal/graph/graph.go`: `Detect(repoRoot) (Status, error)` (`exec.LookPath("python3")` +
  chequeo del módulo `graphify` + `os.Stat(graphify-out/graph.json)`); `ServeCommand(repoRoot)
  (*exec.Cmd, error)` → `python3 -m graphify.serve …`; `BuildCommand(repoRoot)` con esqueleto +
  guarda de error explícita (invocación exacta **TBD**, Open Q3/Q4).
- `cli/cmd/vector/graph.go`: `newGraphCmd` (parent), `newGraphMCPCmd` (detect → graceful-fail o
  proxy `cmd.Run()` con stdio conectado, patrón `signal.NotifyContext` de `serve.go:95` si aplica),
  `newGraphBuildCmd` (error explícito hasta wire).
- `cli/cmd/vector/root.go`: registrar `newGraphCmd()` en `root.AddCommand(...)`.
- `cli/cmd/vector/main.go`: flag `--with-graph-mcp` en `newInitCmd`/`newUpdateCmd`; llamada a
  `scaffold.SeedMCPConfig` cuando el flag está presente; `usage()` actualizado.
- `cli/internal/scaffold/scaffold.go`: `SeedMCPConfig(repoRoot, opts) (FileResult, error)` — lee/
  mergea `.mcp.json` (JSON inválido → error accionable, no sobrescribe a ciegas), preserva otras
  entradas, escribe solo si hay cambio real; función independiente del walk de `embedRoot`.
- `kit/agents/_shared/graph-context.md`: doctrina de las 7 tools (`query_graph`, `get_node`,
  `get_neighbors`, `get_community`, `god_nodes`, `graph_stats`, `shortest_path`), opt-in + degradación.
- `kit/agents/{vector-spec-refiner,vector-bug-refiner,vector-apply-impl,vector-feasibility-reviewer}.md`:
  frontmatter `tools:` + pointer a la doctrina; copias vendored en `scaffold/assets/` regeneradas.

## Flujo

`vector init|update --with-graph-mcp` → seed normal de `.claude/` + `SeedMCPConfig` escribe/mergea
la entrada `vector-graph` en `.mcp.json` → (grafo generado manualmente vía `vector graph build`,
**TBD**) → Claude Code lee `.mcp.json` y arranca `vector graph mcp` por stdio → `graph.Detect`: si
falta graphify o `graph.json` → stderr accionable + exit ≠ 0 (**graceful-fail**); si pasa →
shell-out a `python3 -m graphify.serve …` como proxy transparente hasta que el hijo termine →
un agente del kit con las tools en su frontmatter las invoca **opcionalmente**, degradando limpio
si el server no está conectado (precedente `vector-ui-ux-designer.md:36-38`).

## Riesgos / TBD (Open questions del spec)

- Shape exacto de la entrada `.mcp.json` `stdio` project-scope, y comportamiento de consentimiento
  de Claude Code al detectarla (Open Q1/Q2) — no verificados en este entorno.
- API real de `graphify.serve` y firmas de las 7 tools (Open Q3); formato del nombre de tool
  `mcp__vector-graph__<tool>` (Open Q9) — tomados del SKILL.md, no verificados contra el paquete.
- Resolución del intérprete Python: riesgo de **falso negativo** con pipx/venv; alinear con cómo
  graphify resuelve su propio Python (cache `.graphify_python` / `which graphify` + shebang) en vez
  de asumir `python3` pelado (Open Q11).
- Actualización en caliente del grafo (Open Q4), inclusión de `.vector/specs/` en el corpus (Open
  Q7), versionado de `graphify-out/` (Open Q8), y `.mcp.json` con JSON corrupto (Open Q10) — a
  resolver o trasladar a seguimiento antes de cerrar la fase.
