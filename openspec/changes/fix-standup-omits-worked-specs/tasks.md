# Tasks — fix-standup-omits-worked-specs

## 1. Proyección de `spec.fixed` (leak #1)

- [x] 1.1 En `Project` (`cli/internal/standup/standup.go`, switch ~L108): añadir `case state.EvtSpecFixed` que decodifica `FixedData` (guard `json.Unmarshal(...) == nil`) y añade una entrada de trabajo con `Classification` + `Files` a `SpecActivity` (reúso de `WorkLoggedData` con mapeo documentado, o campo `Fixed` dedicado).
- [x] 1.2 En `Timeline` (mismo archivo, switch ~L171): añadir `case state.EvtSpecFixed` con `Type: "spec.fixed"` y los campos de archivo/clasificación (evaluar `Classification string json:"classification,omitempty"` en `TimelineEvent`).
- [x] 1.3 Mantener `ChangeCount`/`LastChanged` sin cambios; proyección sigue pura y store-free.

## 2. Ventana semiabierta `[from, to)` (leak #2)

- [x] 2.1 Añadir parámetro `to`/`until time.Time` a `Project` y `Timeline`; filtro `TS < since || TS >= until`.
- [x] 2.2 `runStandup` (`cli/cmd/vector/standup.go` ~L31-75): capturar `to := time.Now().UTC()` una sola vez, pasarlo a `Project`, y exponer `until` top-level en `Projection` (análogo a `Since`).
- [x] 2.3 `runStandupCommitBody` (~L179-249): recibir `--until <RFC3339>` y reutilizar ese `to` para reproyectar y para `WriteStandup(digest, to)` — marker avanza a exactamente ese valor; actualizar `usage()`/help. No tocar `--since`/`resolveSince`.
- [x] 2.4 Documentar en comentario la dirección de wiring generación↔commit para `/vector:standup`.

## 3. Tests

- [x] 3.1 `TestProjectDecodesSpecFixed` + `TestTimelineDecodesSpecFixed`: `ChangeCount == 1` Y entrada de trabajo con `Classification`/`Files` recuperables.
- [x] 3.2 `TestProjectHalfOpenBoundary`: eventos en `t-1`/`t`(==`to`)/`t+1` → `t-1` incluido, `t` y `t+1` excluidos.
- [x] 3.3 Actualizar llamadas existentes a `Project`/`Timeline` con un `until` lejano; preservar `TestProjectFiltersBySinceAndGroupsBySpec`, `TestTimelineFlattensSpecEvents`, `TestProjectEmptyPeriodNoPanic`, `TestProjectTitleFromSpecCreated`, `TestParseSince` en verde.
- [x] 3.4 (Opcional recomendado) `TestStandupSurfacesFixedSpecWithoutWorklog` en `cli/cmd/vector/standup_test.go`: `store.FixSpec` sin `worklog` → `runStandup --json` → assert de la entrada de `FixedData` (patrón `state.Open(t.TempDir())` + `captureStdout`).

## 4. Verificación

- [x] 4.1 `gofmt -l cli` sin salida.
- [x] 4.2 `go -C cli vet ./...` verde.
- [x] 4.3 `go -C cli test ./internal/standup/... ./cmd/vector/...` verde.
- [x] 4.4 `go -C cli build ./...` verde.
- [x] 4.5 Verificar `web/ SpecTimeline` (open question #1) antes de asumir que no requiere cambios.
