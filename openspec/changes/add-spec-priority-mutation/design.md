# Design — add-spec-priority-mutation

## Decisiones clave

- **Store primitive + comando delgado**: toda la validación/mutación/evento vive en
  `Store.SetPriority`; el comando de cobra solo resuelve el store, lee el valor previo, llama a
  la primitiva y formatea la salida. Mismo split que el resto de `spec <verbo>`.
- **`SetPriority` es una mutación lateral de metadata, NO una transición**: como
  `LinkSpec`/`RelateSpec`, no como `SetStatus`/`ApplySpec`/`CloseSpec`. No toca `Status`, no pasa
  por `applyTransition`. Un spec puede ser `urgent` y `needs-attention` a la vez (ejes
  ortogonales).
- **Orden de la primitiva**: validar `priority.Valid()` **antes** del lock → `s.mu.Lock` →
  `ReadSpec` → guarda archivado → guarda no-op → mutar `Priority`+`UpdatedAt` → `writeSpecFile`
  (si falla, retornar **antes** de tocar el activity log) → `appendEvent(priority.changed)`.
  Modelado sobre `ProposeSpec`/`LinkSpec`/`FixSpec`.
- **No-op = éxito idempotente, no error**: mismo valor ⇒ retorna el spec sin mutar, sin escribir,
  sin re-emitir evento (mirror de `LinkSpec` en un re-link idéntico). `UpdatedAt` no avanza.
- **Specs archivados no son target legal**: la prioridad solo ordena **dentro de una columna
  activa** (`priorityRank` → `SelectNext`); un archivado está fuera del board. Error dedicado y
  accionable, mismo estilo que la guarda de `SetStatus`. La guarda de archivado corre **antes**
  que la de no-op.
- **Mensaje de prioridad inválida más explícito** que el de `CreateSpec`: `invalid priority
  "<v>": must be one of urgent|high|normal|low`. `CreateSpec` no se toca (primitivas
  independientes).
- **`SetPriority` retorna solo `(*SpecState, error)`**: el comando captura el valor "antes"
  leyendo el spec por su cuenta (patrón de `newSpecApplyCmd` capturando `change`), no vía un
  segundo valor de retorno — mantiene la firma simétrica con el resto de mutaciones.
- **Re-validación de `--priority` en el comando**: intencional, para que `--dry-run` con un valor
  inválido también falle sin tocar el store. El resto de reglas (archivado, no-op) viven **solo**
  en la primitiva; el comando las reporta, no las reimplementa.
- **Sin cambios de schema ni de `EventVersion`**: `Priority` ya existe (`types.go:128`); es
  puramente una nueva vía de mutación. El evento nuevo usa el mismo envelope `Event`.
- **Sin cambios en `web/`/`kit/`**: `internal/board` ya proyecta `Card.Priority` por SSE; refleja
  el nuevo valor en la siguiente proyección sin código. `--priority` en `propose`/`apply` queda
  para una fase futura.

## Superficie

- `cli/internal/state/event.go`: `EvtPriorityChanged EventType = "priority.changed"` +
  `PriorityChangedData{From, To Priority}` (forma `From`/`To` de `StatusChangedData`, sin
  `Trigger`/`Reason`).
- `cli/internal/state/store.go`: `Store.SetPriority(id string, priority Priority, actor string,
  now time.Time) (*SpecState, error)`.
- `cli/cmd/vector/spec_transitions.go`: `newSpecSetCmd()` (`Use: "set [id]"`, flags `--id`,
  `--priority` requerido, `--repo-root`, `--dry-run`, `--json`).
- `cli/cmd/vector/main.go`: registro en `newSpecCmd()`, `set` en el usage del `RunE` sin
  subverbo, línea nueva en `usage()`.
- `cli/cmd/vector/testutil_test.go`: shim `runSpecSet`.
- Tests: `store_test.go` (5 casos), `spec_transitions_test.go` (validación/dry-run/json),
  `golden_test.go` + `testdata/golden/spec-set.json` (nuevo fixture).

## Contrato `--json`

- Éxito: `{"id","priority","previousPriority"}`.
- No-op: `{"id","priority","note":"already <level>"}`.
- `--dry-run`: `{"id","priority","previousPriority","dryRun":"true"}` (no escribe).

## Open questions

1. Forma del subcomando: se eligió `set <id> --priority` (extensible a `--estimate`, etc.) sobre
   `spec priority <id> <level>`. Revisar si aparecen más mutaciones de un solo campo.
2. Archivados como target: se rechazan; permitirlos por bookkeeping histórico queda abierto.
3. `--priority` como flag de `propose`/`apply`/`status`: fuera de scope; evaluar si reprioritizar
   siempre acompaña una transición.
