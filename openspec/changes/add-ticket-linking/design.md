# Design — add-ticket-linking

## Decisiones clave

- **CLI-owns-writes**: el binario (`Store`) es el único escritor de `Ticket`; el command orquesta
  y, si el provider es ambiguo, pide `--provider` con `AskUserQuestion`.
- **Detección = frontmatter `ticket:` primero, fallback prosa conservador** (URL completo o key con
  provider reconocible). Raw detecta sobre el texto crudo; sync sobre los artefactos del change.
- **Provider ambiguo (key sin URL): manual exige `--provider`; auto NO linkea** (no adivina).
- **Un solo `Ticket` por spec** (la primera ref detectada); sin array en V1.
- **`auto:true` en raw/sync, `auto:false` en manual**; el auto **no pisa** un ticket manual.
- **Idempotencia** en `LinkSpec`: mismo `provider+key+url` → no-op (no re-emite `spec.linked`).
- **Sin validación contra el tracker** (no hay hit de API); `link` no cambia el estado del spec ni
  crea change OpenSpec.
- **`URL` puede quedar vacío** (key sola sin host): se guarda solo la key; el board muestra la key
  sin link. Futuro: `base-url` por provider en `.vector/config.json`.

## Superficie

- `cli/internal/state/store.go`: `CreateSpecParams.Ticket *Ticket` (siembra inicial + emite
  `EvtSpecLinked`) y `Store.LinkSpec(id, ticket, actor, now)` (lock → read → idempotencia/precedencia
  → write atómico → append evento). Molde: `ProposeSpec`/`ReconcileStatus`.
- `cli/cmd/vector/ticket.go` (NUEVO): `inferProvider`, `parseRef`, `detectTicket` (stdlib).
- `cli/cmd/vector/spec_transitions.go`: `runSpecLink` (`leadingID` + `<ref>` + `--provider`/`--json`).
- `cli/cmd/vector/main.go`: dispatch `case "link"`; `--ticket` (JSON) en `runSpecCreate`; threading
  de `detectTicket` en `runSync` (create vía params, reconcile vía `LinkSpec`).
- `kit/commands/vector/link.md` (NUEVO) + asset embebido regenerado por `go generate`.
- `docs/domain-contract.md` §5: ampliar la nota `auto` (raw/sync `auto:true` vs manual `auto:false`).
- `web/`: **sin cambios** — `Card.Ticket` ya se proyecta (`board.go`) y se renderiza (`SpecCard.tsx`).

## Flujo

- **Manual**: `vector spec link <id> <ref>` → `parseRef`+`inferProvider` (o `--provider`) →
  `LinkSpec(…, auto:false)` → write + `spec.linked`.
- **Auto-raw**: `/vector:raw` detecta ref en prosa → `vector spec create … --ticket <json>` →
  `CreateSpecParams.Ticket{auto:true}`.
- **Auto-sync**: `runSync` por change → `detectTicket` → create (params) o reconcile (`LinkSpec`),
  idempotente y respetando precedencia manual.

## Constraint pendiente (no abierto en código)

Formato del frontmatter `ticket:`: el implementador elige UN formato (sugerido `ticket: <provider>:<key>`
o `ticket: <url>`), lo documenta en `detectTicket` y en `docs/domain-contract.md` antes de mergear.
