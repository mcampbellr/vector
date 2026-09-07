# Design — fix-standup-omits-worked-specs

## Decisiones clave

- **Corrección solo del lado de la proyección**: se extiende `standup.Project`/`standup.Timeline`
  para consumir el `spec.fixed` que ya se emite; **`Store.FixSpec` no cambia**. Duplicar la emisión
  como `work.logged` produciría doble conteo, rompería la semántica de `spec.fixed` como evento
  puramente informativo, y exigiría migrar logs históricos.
- **Ventana semiabierta `[from, to)` con `to` excluido**: única semántica que permite avanzar el
  marker a `to` sin perder ni duplicar el evento exactamente en el límite. El filtro pasa de
  `e.TS.Before(since)` a `e.TS.Before(since) || !e.TS.Before(until)` (≡ `TS < since || TS >= until`).
- **`to` capturado una sola vez y reutilizado** para filtrar y para avanzar el marker: es la causa
  raíz del leak #2. Cualquier segunda captura de `now` en el commit reintroduce la zona de carrera.
- **Proyección pura y store-free**: `Project`/`Timeline` no leen el store, ni red, ni LLM; filtran
  por el `TS` del propio evento, nunca por wall-clock. La invariante se preserva (el `to` entra por
  parámetro desde el caller, no se captura dentro de la proyección).
- **Payloads malformados se saltan, nunca son fatales**: se replica el guard
  `json.Unmarshal(e.Data, &d) == nil` del caso `work.logged`; `ChangeCount` incrementa igual (el
  evento cuenta aunque no aporte detalle).
- **Campos aditivos (`omitempty`)**: los nuevos campos de `Projection`/`SpecActivity`/`TimelineEvent`
  siguen el patrón de `Work`/`Transitions`/`Ticket`; ningún consumidor que ignore campos
  desconocidos se rompe. No se renombran ni eliminan campos existentes ni del digest persistido
  (`state.StandupDigest`).
- **Sin cambios de versión**: `EventVersion` y `StandupSchemaVersion` intactos — el cambio es
  aditivo sobre cómo se **lee** el log, no sobre su formato de escritura.
- **Wiring generación↔commit del project command = nota de diseño, no requisito**: el binario expone
  el mecanismo (`until` en el JSON + `--until` en `commit`); reescribir
  `kit/commands/vector/standup.md` para transportar el `to` del paso 1 al paso 3 queda fuera de la
  fase obligatoria, salvo que el agente de apply lo juzgue trivial y de bajo riesgo.

## Superficie

- `cli/internal/standup/standup.go` (MODIFICAR): `Project`/`Timeline` ganan el parámetro
  `to time.Time` y el caso `state.EvtSpecFixed`; `SpecActivity`/`TimelineEvent` ganan los campos
  derivados de `FixedData` (`Classification`/`Files`, vía reúso de `WorkLoggedData` o tipo dedicado).
  Referencia: el caso `state.EvtWorkLogged` existente en el mismo switch.
- `cli/cmd/vector/standup.go` (MODIFICAR): `runStandup` captura `to := time.Now().UTC()` una vez y
  lo pasa a `Project`; expone `until` en el JSON (`Projection`). `runStandupCommitBody` reutiliza ese
  `to` (flag `--until` poblado desde el JSON del paso 1) para reproyectar y para `WriteStandup`, de
  modo que el marker avance exactamente a ese valor. `usage()`/help se actualizan si se añade
  `--until`. `--since`/`resolveSince` sin cambios.
- `cli/internal/standup/standup_test.go` (MODIFICAR): decodificación de `spec.fixed` (Project y
  Timeline); frontera semiabierta antes/en/después de `to`; actualizar llamadas existentes a
  `Project`/`Timeline` con un `until` lejano para no romper conteos.
- `cli/cmd/vector/standup_test.go` (MODIFICAR, opcional recomendado): `FixSpec` sin `worklog` →
  `runStandup --json` → assert de la entrada derivada de `FixedData`; escenario de marker semiabierto.

## Flujo esperado (corregido)

1. `/vector:fix <id>` emite `spec.fixed` (`FixSpec`, sin cambios).
2. `/vector:standup` → `vector standup --json`.
3. `runStandup` captura `to` una vez, resuelve `from` (marker o `--since`), llama
   `Project(events, from, to)`.
4. `Project` recorre `from <= TS < to`; para `spec.fixed` decodifica `FixedData` y añade la entrada
   de trabajo (`Classification`/`Files`), además del `ChangeCount` ya existente.
5. `runStandup` serializa la proyección con `until = to`.
6. `vector-standup-writer` redacta el digest con el spec corregido y su detalle.
7. `/vector:standup` pipea el digest a `vector standup commit --digest-file -` pasando `--until` (el
   `to` del paso 3).
8. `runStandupCommit` reproyecta con el mismo `[from, to)` y `WriteStandup(digest, to)` — el marker
   avanza a exactamente ese `to`.
9. El siguiente `vector standup` usa `from = to` previo; ningún evento en `TS == to` se pierde ni se
   cuenta dos veces (excluido antes, incluido ahora).

## Open questions (a resolver por el agente de apply)

1. `SpecTimeline` (`web/`) — ¿renderiza genéricamente por `Type` o necesita un caso para
   `spec.fixed`? Verificar `web/` antes de asumir que no requiere cambios.
2. Campo para `Classification`/`Files`: reúso de `state.WorkLoggedData` (`Note = Classification`) vs.
   `Fixed []state.FixedData` dedicado — ambas cumplen el criterio; documentar la elección en un
   comentario.
3. Nombre exacto de `to`/`until` en `Projection` y el flag `--until`, siguiendo la convención
   `Since`/`--since`.
