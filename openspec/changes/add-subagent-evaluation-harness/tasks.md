# Tasks — add-subagent-evaluation-harness

## 1. Paquete de dominio `internal/eval`

- [ ] 1.1 `evaluator.go`: tipos `Role` (3 constantes), `Fixture`, `GroundTruth`, `FixtureResult`,
      `RoleRollup`, `GateMode` (`warmup`/`enforcing`) con package doc comment y structs documentados.
- [ ] 1.2 `evaluator.go`: `Run(fixtures, baselines, thresholds)` → `([]FixtureResult, []FixtureResult)`
      (filas por-fixture + rollups por rol/rol×modelo); `score = 0.7·verdictMatch + 0.3·citationValidityRate`.
- [ ] 1.3 `evaluator.go`: `LoadFixtures`, `LoadBaseline`, `LoadThresholds`, `ResolveGateMode`
      (lee `gate-mode.json` con override `VECTOR_EVAL_MODE`), `WriteBaseline` (escritura atómica al
      estilo `config.go:1041-1062`).
- [ ] 1.4 `citations.go`: `ExtractCitations` (gramática de evidencia estructurada
      `[SEVERITY] (confidence: N/10) path:line —` + `path:line` genérico) + `VerifyCitation`
      (`os.Stat` + conteo de líneas; existencia + rango; ausencia de citas → `1.0`).
- [ ] 1.5 `rubrics.go`: `ParseVerdict(role, recordedOutput)` con parser por rol (`## Verdict`
      PASS/PASS_WITH_WARNINGS/BLOCK; `## VERDICT` go/go-with-risks/no-go + N/10; `## VERDICT`
      VÁLIDO Y VALIOSO/VÁLIDO PERO MARGINAL/INVÁLIDO O SIN VALOR + N/10); veredicto no reconocido =
      error de parseo (→ `verdictMatch=false`, motivo en `ScoreBreakdown`). Constantes de peso.

## 2. Tests de dominio

- [ ] 2.1 `evaluator_test.go`: PASS, BLOCK, veredicto no reconocido, regresión vs baseline (falla
      enforcing / no falla warmup), umbral inclusivo (`score == threshold` pasa).
- [ ] 2.2 `citations_test.go`: ambas gramáticas, archivo inexistente, línea fuera de rango, cero citas.
- [ ] 2.3 `rubrics_test.go`: los 3 roles con texto real capturado + sección de veredicto ausente/corrupta.

## 3. Subcomando `vector eval`

- [ ] 3.1 `eval.go`: `newEvalCmd()` con `--role` (`StringArrayVar`, repetible; vacío = 3 roles),
      `--json`, `--update-baselines`, `--repo-root`.
- [ ] 3.2 `RunE`: por rol → `LoadFixtures`/`LoadBaseline`/`LoadThresholds`/`ResolveGateMode` +
      `eval.Run`; `--update-baselines` → `WriteBaseline` (nunca en CI); `--json` → `printJSON`;
      sin `--json` → `ui.Table` (nunca dentro de la rama `if jsonOut`).
- [ ] 3.3 `root.go`: registrar `newEvalCmd()` en `root.AddCommand(...)` sin alterar el resto.

## 4. Gate de CI

- [ ] 4.1 `eval_test.go`: `TestEvalGate` table-driven por rol, consumiendo `eval.Run` directo sobre
      `testdata/eval/` commiteado; `t.Fatalf` en enforcing (mensaje con rol/score/baseline/threshold),
      `t.Logf` en warmup. Nunca llama `--update-baselines` ni escribe bajo `testdata/eval/`.

## 5. Fixtures y datos de test

- [ ] 5.1 `testdata/eval/gate-mode.json` (`{"mode":"warmup"}`) + `thresholds.json` (placeholders
      explícitos, marcados a recalibrar).
- [ ] 5.2 `vector-spec-validator/`: ≥3 fixtures curadas (PASS limpio, BLOCK por sección faltante,
      PASS_WITH_WARNINGS por i18n débil) + `baseline.json`.
- [ ] 5.3 `vector-feasibility-reviewer/`: ≥2 fixtures (go técnico claro, no-go de seguridad) + `baseline.json`.
- [ ] 5.4 `vector-comment-evaluator/`: ≥2 fixtures (válido y valioso, inválido por bikeshedding) + `baseline.json`.

## 6. Documentación

- [ ] 6.1 `cli/internal/eval/README.md`: schema de fixture, cómo capturar/curar `groundTruth`, cómo
      correr `--update-baselines`, qué revisar en el diff, guardarraíl de no incluir secretos.
- [ ] 6.2 `cli/CLAUDE.md`: bullet de `internal/eval` en "Estado actual" + subcomando `eval`.

## 7. Verificación

- [ ] 7.1 `gofmt -l cli` (vacío), `go -C cli vet ./...`, `go -C cli test ./...`, `go -C cli build ./...` verdes.
- [ ] 7.2 `TestEvalGate` corre en warm-up sin bloquear; enforcing bloquea con mensaje accionable.
- [ ] 7.3 `TestJSONGoldenUnchanged` sigue verde (ningún `--json` existente tocado).
