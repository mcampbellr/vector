# Design — add-subagent-evaluation-harness

## Decisiones clave

- **100% offline**: CI reproduce (replay) fixtures grabadas; nunca invoca un Claude Code real ni
  ninguna API paga. Razón: Vector orquesta el LLM del usuario, no consume uno propio — invocar API
  en CI rompería esa garantía y añadiría costo/no-determinismo.
- **Fase 1 = solo los 3 agentes juez** (`spec-validator`, `feasibility-reviewer`,
  `comment-evaluator`): son los únicos con contrato de veredicto enumerado, lo único puntuable de
  forma determinística sin comparación semántica. Los otros 10 subagentes (prosa/código) quedan
  para una fase futura.
- **Función de dominio única compartida**: `eval.Run(...)` en `internal/eval` implementa todo el
  scoring; tanto `vector eval` (humano) como `eval_test.go` (gate de CI) la invocan directo → cero
  drift entre "lo que ve el mantenedor" y "lo que bloquea el merge".
- **Gate = regresión-vs-baseline O umbral absoluto, con warm-up no bloqueante**. Umbral inclusivo
  (`score >= threshold` pasa). Toggle commiteado en `gate-mode.json` (override `VECTOR_EVAL_MODE`):
  en `warmup` registra la comparación pero no falla; en `enforcing` `t.Fatalf`.
- **Fixtures capturadas-y-curadas**, no sintéticas: el ground truth ("cuál es el veredicto
  correcto") requiere juicio humano sobre un caso real. Refresco intencional vía
  `vector eval --update-baselines`, revisado en el diff del PR — **nunca** dentro de CI.
- **Score = `0.7·verdictMatch + 0.3·citationValidityRate`** (constantes en `rubrics.go`): la
  coincidencia de veredicto es la señal primaria; la fidelidad de citas es secundaria pero real
  (operacionaliza `citation-discipline.md`). Default documentado y ajustable.
- **Citation-fidelity = existencia + rango de línea** contra el repo actual (no un snapshot
  congelado). Detecta citas que un refactor dejó obsoletas. Ausencia de citas → `1.0` (no penaliza).
  No verifica corrección semántica del claim (eso requeriría el LLM que se busca evitar).
- **Sin cambios en `ci.yml`**: el step `go -C cli test ./...` recoge cualquier `_test.go` nuevo.
- **Read-only sobre el estado del usuario**: el harness no lee ni escribe `.vector/state`,
  `activity.jsonl` ni `.vector/config.json`; solo escribe su reporte (stdout) y, opt-in, baselines
  bajo `testdata/eval/`. `TestJSONGoldenUnchanged` permanece intacto.
- **Sin dependencias nuevas**: stdlib (`encoding/json`, `regexp`, `bufio`, `os`, `path/filepath`,
  `testing`) + cobra/lipgloss ya presentes.

## Superficie

- `cli/internal/eval/evaluator.go` — tipos (`Role`, `Fixture`, `GroundTruth`, `FixtureResult`,
  `RoleRollup`, `GateMode`) + `Run`, `LoadFixtures`, `LoadBaseline`, `WriteBaseline`,
  `LoadThresholds`, `ResolveGateMode` (escritura atómica al estilo `config.Write`).
- `cli/internal/eval/citations.go` — `ExtractCitations` (gramática de evidencia estructurada +
  `path:line` genérico) + `VerifyCitation` (`os.Stat` + conteo de líneas).
- `cli/internal/eval/rubrics.go` — `ParseVerdict(role, recordedOutput)` con un parser por rol +
  constantes de peso del score.
- `cli/cmd/vector/eval.go` — `newEvalCmd()`: `--role` (repetible), `--json`, `--update-baselines`,
  `--repo-root`; tabla `ui.Table` o JSON `printJSON`.
- `cli/cmd/vector/eval_test.go` — `TestEvalGate` table-driven por rol, consumiendo `eval.Run`
  directo; `t.Logf` en warm-up, `t.Fatalf` en enforcing. Nunca escribe baselines.
- `cli/cmd/vector/root.go` — registrar `newEvalCmd()`.
- `cli/cmd/vector/testdata/eval/` — `gate-mode.json`, `thresholds.json` (placeholders explícitos),
  y `<rol>/{baseline.json, *.json}` para los 3 roles.
- `cli/internal/eval/README.md` — proceso de curación paso a paso.
- `cli/CLAUDE.md` — bullet de `internal/eval` + subcomando `eval`.

## Flujo

Un mantenedor captura una salida real de un agente juez y la curra (`groundTruth.expectedVerdict`
+ `recordedOutput` + metadata) como fixture JSON → en cada corrida `eval.Run` carga los fixtures de
un rol, parsea el veredicto (`rubrics.go`), lo compara con el ground truth (`verdictMatch`), extrae
y verifica citas (`citations.go` → `citationValidityRate`), calcula
`score = 0.7·verdictMatch + 0.3·citationValidityRate`, agrega en `RoleRollup` (promedio + desglose
por modelo), y compara contra baseline + threshold. En `warmup` registra; en `enforcing` falla el
subtest del rol. `vector eval` imprime tabla ASCII (o `--json`); `--update-baselines` re-escribe el
baseline del rol para revisión en el PR.

## Open questions (no reabrir decisiones de producto)

1. Valores numéricos exactos de `thresholds.json` por rol — placeholders, recalibrar tras warm-up.
2. Quién/cómo firma la curación del ground truth de `feasibility-reviewer`/`comment-evaluator`.
3. Dónde persiste el histórico de win-rate por rol×modelo más allá del reporte de la corrida.
4. Comparación semántica de prosa libre (los 10 roles fuera de fase 1) — probablemente un LLM juez.
