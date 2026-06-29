# Tasks — add-fix-command

## 1. State + eventos

- [x] 1.1 `EvtSpecFixed = "spec.fixed"` + `FixedData{Classification, ValidationResult, Artifacts, Files}` en `event.go`.
- [x] 1.2 `Store.FixSpec(...)` modelado sobre `ProposeSpec`: lock único, valida status corregible, bump `UpdatedAt`, `writeSpecFile`, `appendEvent`; **sin** transición.
- [x] 1.3 Tests: `FixSpec` appendea `spec.fixed` con la data y bumpea `UpdatedAt`; no cambia status; rechaza `draft`/`closed`/`archived`.

## 2. Binario

- [x] 2.1 `runSpec()` + `case "fix"` → `runSpecFix(args[1:])`.
- [x] 2.2 Flags `--classification` / `--artifacts` / `--files` / `--validation-result` / `--repo-root` / `--json`; id como positional; validación de id kebab-case y `--classification`/`--validation-result`.
- [x] 2.3 Llamar `store.FixSpec`; reporte JSON/humano; exit `0`/`1`; usage actualizado.
- [x] 2.4 Tests table-driven de `runSpecFix` (flags/id/clasificación/validación).

## 3. Agentes (kit)

- [x] 3.1 `kit/agents/vector-fix-refiner.md` (Haiku, read-only): clasificación + artefactos/archivos + Clarity Verdict + preguntas bloqueantes.
- [x] 3.2 `kit/agents/vector-fix-implementer.md` (Sonnet): amenda artefactos OpenSpec, edita código, corre tests+build, retorna JSON; no commitea.

## 4. Command (kit)

- [x] 4.1 `kit/commands/vector/fix.md`: validación → refiner → scope guard/clarity gate → transiciones vía `status` → implementer → gating → `spec fix` → route/worklog → reporte sin commit.
- [x] 4.2 Vendoring vía `go generate` + sembrado por `vector update`; extender el test que verifica los embedded commands/agents.

## 5. Verificación

- [x] 5.1 `go -C cli generate ./...`, `gofmt -l cli`, `go -C cli vet ./...`, `go -C cli test ./...` verdes.
- [x] 5.2 Sin regresiones en `spec create|list|apply|propose|close|status|route|worklog`.
- [x] 5.3 e2e del ciclo `review/open → in-progress|needs-attention → review` vía `status`, con `spec.fixed` registrado y gateado por la validación.
