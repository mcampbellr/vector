# Tasks — binary-templated-transition-summaries

## 1. Binario — extensión de `summarizeProjection`

- [x] 1.1 Agregar `HasWork bool` (sin omitempty) y `TemplateSummary string` (con omitempty) a
  `summarizeProjection` en `cli/cmd/vector/summarize.go`.
- [x] 1.2 En `runSpecSummarize`, calcular `hasWork` iterando `timelineEvents` buscando
  `te.Type == string(state.EvtWorkLogged)` y poblar `proj.HasWork`.
- [x] 1.3 Si `!hasWork`, poblar `proj.TemplateSummary = buildTemplateSummary(spec.ID, spec.Title,
  timelineEvents)`; dejar vacío cuando `hasWork == true`.

## 2. Binario — función pura `buildTemplateSummary`

- [x] 2.1 Implementar `buildTemplateSummary(id, title string, events []standup.TimelineEvent)
  string` en `cli/cmd/vector/summarize.go`, junto a `hasWorkLoggedAfter`.
- [x] 2.2 Calcular label: `title` si no vacío; si no, `id`.
- [x] 2.3 Scan cronológico: `"spec.proposed"` → `"<label> proposed (draft → open)"`; `"spec.closed"`
  → `"<label> closed"`; `"spec.archived"` → `"<label> archived"` (primer match gana, retornar).
- [x] 2.4 Si no hay ninguno de los anteriores, buscar el último `"status.changed"` con `From != ""`
  y `To != ""` → `"<label>: moved from <from> to <to>"`.
- [x] 2.5 Fallback (slice vacío o sin eventos reconocidos): `fmt.Sprintf("spec %q: no recent
  activity", id)`.

## 3. Tests — `summarize_test.go`

- [x] 3.1 `TestSummarizeProjectionHasWorkFalseWhenNoWorkLogged`: crear spec, appendar solo un
  evento `status.changed`, correr `runSpecSummarize --json`, parsear `summarizeProjection` →
  `hasWork == false`, `templateSummary != ""`.
- [x] 3.2 `TestSummarizeProjectionHasWorkTrueWhenWorkLogged`: crear spec, llamar
  `store.WorkLog`, correr `--json` → `hasWork == true`, `templateSummary == ""`.
- [x] 3.3 `TestBuildTemplateSummary`: tabla-driven con cinco casos:
  - Evento `spec.proposed` → `"… proposed (draft → open)"`.
  - Evento `spec.closed` → `"… closed"`.
  - Evento `spec.archived` → `"… archived"`.
  - Evento `status.changed` (from=`in-progress`, to=`review`) → `"…: moved from in-progress to review"`.
  - Sin eventos → fallback `"spec \"…\": no recent activity"`.
- [x] 3.4 Caso adicional en `TestBuildTemplateSummary`: `title` vacío → usa `id` como label.
- [x] 3.5 Verificar que los tests existentes en `summarize_test.go` siguen verdes sin modificación.

## 4. Project commands — bifurcación `hasWork`

- [x] 4.1 `kit/commands/vector/archive.md` §3: reemplazar el bloque de 3 pasos por la bifurcación
  condicional; camino corto pipa `{"summary":"<templateSummary>"}` a `commit --action archive`;
  camino largo spawna Haiku igual que hoy.
- [x] 4.2 `kit/commands/vector/close.md` §3: mismo cambio con `--action close`.
- [x] 4.3 `kit/commands/vector/status.md` §3: mismo cambio con `--action status`; documentar que
  el safeguard close/archive no aplica aquí (el template siempre se persiste).
- [x] 4.4 `kit/commands/vector/propose.md` §7: mismo cambio con `--action propose`; añadir nota de
  que `hasWork` siempre será `false` para propose (no hay `work.logged` en la transición).
- [x] 4.5 En cada command, loggear el camino tomado en el paso de reporte: `"summary: template (no
  work logged)"` vs `"summary: generated (Haiku)"`.
- [x] 4.6 Manejar el edge case `templateSummary == ""` (bug defensivo): anotar `"no templateSummary
  received, skipping summary"` y continuar sin pipear.
- [x] 4.7 Verificar que §1/§2/§4 de archive, close, status y §1–§6/§8 de propose no cambian.

## 5. Scaffold assets — sync de copias vendorizadas

- [x] 5.1 Copiar `kit/commands/vector/archive.md` actualizado a
  `cli/internal/scaffold/assets/commands/vector/archive.md` (byte-idéntico).
- [x] 5.2 Copiar `kit/commands/vector/close.md` actualizado a
  `cli/internal/scaffold/assets/commands/vector/close.md`.
- [x] 5.3 Copiar `kit/commands/vector/status.md` actualizado a
  `cli/internal/scaffold/assets/commands/vector/status.md`.
- [x] 5.4 Copiar `kit/commands/vector/propose.md` actualizado a
  `cli/internal/scaffold/assets/commands/vector/propose.md`.

## 6. Verificación y gate

- [x] 6.1 `cd cli && gofmt -l .` → sin output.
- [x] 6.2 `go vet ./...` → sin warnings.
- [x] 6.3 `go test ./cmd/vector/... ./internal/state/... ./internal/standup/...` → todos los tests
  verdes, incluyendo los nuevos y los existentes.
- [x] 6.4 Confirmar que las tres copias de cada command (`kit/`, `.claude/commands/vector/`,
  `cli/internal/scaffold/assets/commands/vector/`) están en sync.
