# Spec: Linkear specs a tickets externos (`/vector:link`)

## 1. Objetivo

Construir la capacidad de **asociar un spec del board a un ticket externo** (Jira, Linear,
GitHub, otros) desde tres fuentes: manual (`/vector:link [id] [ticket]`), auto-detección en el
texto crudo de `/vector:raw`, y auto-detección durante `vector sync` sobre los artefactos del
change. El binario escribe `Ticket{provider,key,url,auto}` en `.vector/specs/<id>/state.json` y
emite el evento `spec.linked`.

Esta feature permite que un dev cierre el gap de **trazabilidad**: cada spec sabe a qué ticket
operativo corresponde, y el board lo proyecta como link al tracker — sin imponer convenciones
al repo del usuario (la detección es best-effort y conservadora).

## 2. Alcance

### Incluido en esta fase

- **Binario** `vector spec link <id> <ref> [--provider p] [--json]` (CLI-owns-writes): parsea la
  ref, infiere el provider (o lo fuerza `--provider`), resuelve key+url, persiste vía un nuevo
  `Store.LinkSpec`. `auto:false` (link manual).
- **Command** `/vector:link [id] [ticket]` (`kit/commands/vector/link.md`) que orquesta el
  binario; pide `--provider` con `AskUserQuestion` solo si no puede inferirlo.
- **Auto-link en `/vector:raw`**: detecta un ticket en el **texto crudo** (prosa) y siembra el
  ticket al crear el draft (`auto:true`), vía `CreateSpecParams.Ticket`.
- **Auto-link en `vector sync`**: al proyectar un change, `detectTicket` busca el ticket en los
  artefactos del change — **frontmatter `ticket:` del spec doc primero**, luego fallback a
  escaneo conservador de prosa (URL completo o key con provider reconocible). Linkea `auto:true`.
- **Inferencia de provider**: jira (`*.atlassian.net` / `…/browse/<KEY>`), linear (`linear.app`),
  github (`github.com/<o>/<r>/issues/<N>` o shorthand `o/r#N`). Si **solo hay una key sin URL** y
  el provider es ambiguo → manual exige `--provider`; **auto NO linkea** (conservador, no adivina).
- **Idempotencia**: re-linkear el mismo `provider+key` es no-op (no re-emite evento); cambiar de
  ticket actualiza y re-emite `spec.linked`.
- **Precedencia**: el auto-link (raw/sync) **no pisa** un ticket puesto a mano (`auto:false`).

### Fuera de scope

- **Validación contra el tracker externo** (que el ticket exista): no hay hit de API en V1.
- **Múltiples tickets por spec**: campo `Ticket` único; se linkea la **primera** ref detectada.
- **`unlink` / desasociar**: no en V1 (revertir = `git` del `state.json` o cambiar de ticket).
- **Sync bidireccional** (comentar en Jira/Linear/GitHub desde Vector).
- **API HTTP de escritura** desde el panel web; el board solo **proyecta** el ticket (ya existe).
- **Cambiar la máquina de estados**: `link` no cambia el estado del spec ni crea change OpenSpec.

El agente no debe implementar nada fuera de este alcance, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Go (módulo único en `cli/`, stdlib) + kit (markdown commands). React/Vite en `web/` (sin cambios).
- State: `cli/internal/state` (único escritor del JSON, escritura atómica + lock).

### Versiones relevantes

- Go: `1.26` (`cli/go.mod`).
- React: `19` (`web/package.json`) — no se toca en esta fase.

### Patrones existentes a respetar

- `Ticket{Provider,Key,URL,Auto}` y `TicketProvider` (`jira|linear|github|other`) **ya existen**
  en `cli/internal/state/types.go`; `SpecState.Ticket *Ticket json:"ticket,omitempty"` ya está.
- `EvtSpecLinked` + `SpecLinkedData{Provider,Key,URL,Auto}` **ya existen** en `event.go`.
- Escritura solo por `Store`: el nuevo `LinkSpec` sigue el molde de `ProposeSpec`/`ReconcileStatus`
  (lock → read → write atómico → append de evento).
- Subcomandos del binario: molde en `cli/cmd/vector/spec_transitions.go` (`leadingID`, flagset, `--json`).
- Helpers de sync (`syncStatus`, `syncNeedsUAT`) en `cli/cmd/vector/main.go` — `detectTicket` va al lado.
- Command markdown del kit: molde en `kit/commands/vector/propose.md` (adapter, hard rules, steps).
- `Card.Ticket` ya se proyecta en `cli/internal/board/board.go` y se renderiza en
  `web/src/components/SpecCard/SpecCard.tsx` (icono `Tag` + key). **No rehacer**.

---

## 4. Dependencias previas

Ya existe (verificado):

- [x] `Ticket` + `TicketProvider` en `types.go`; `SpecState.Ticket`.
- [x] `EvtSpecLinked` + `SpecLinkedData` en `event.go`.
- [x] `CreateSpec`/`ReconcileStatus`/`ProposeSpec` como molde de write+evento en `store.go`.
- [x] `Card.Ticket` proyectado (board) y renderizado (`SpecCard`).
- [x] `openspec.Change` con `Dir` (dir repo-relativo) y `ProposalRel` para `detectTicket`
      (`cli/internal/openspec/openspec.go`). **Ojo**: NO hay `DesignRel`/`TasksRel`; design.md y
      tasks.md se construyen desde `Dir` (ver §6).
- [x] `domain-contract.md` §5 ya lista la fila `/vector:link [id] [ticket]`.

**Lo que NO existe todavía y es trabajo de este spec** (no confundir con "ya existe"):
`Store.LinkSpec`, el campo `CreateSpecParams.Ticket`, la emisión de `spec.linked` desde
`CreateSpec`, el flag `--ticket` en `runSpecCreate`, el subcomando `vector spec link`, y los
helpers `inferProvider`/`parseRef`/`detectTicket`. `CreateSpec`/`ReconcileStatus`/`ProposeSpec`
existen **solo como molde de write+evento**, no traen tickets.

No hay dependencias estructurales faltantes — es extensión aditiva.

---

## 5. Arquitectura

### Patrón a usar

Domain-first: el ticket vive en el estado (fuente de verdad); se ingesta en puntos de escritura
(manual, raw, sync) y se proyecta read-only al board. Toda escritura pasa por `Store`.

### Capas afectadas

- domain (Go state): `Store.LinkSpec` (nuevo) + `CreateSpecParams.Ticket` (nuevo). `Ticket` ya existe.
- application (CLI): helpers `parseRef`/`inferProvider`/`detectTicket`; subcomando `runSpecLink`;
  threading en `runSync` y `runSpecCreate`.
- presentation (web): **sin cambios** (ya renderiza el ticket).
- data/infrastructure: serialización de `Ticket` en `state.json` (ya existe).

### Flujo esperado

**Manual (`/vector:link`):**
1. `vector spec link <id> <ref> [--provider p]`.
2. `parseRef(ref)` → key + url; `inferProvider(ref)` (o `--provider`). Si ambiguo y sin flag → error accionable.
3. `Store.LinkSpec(id, &Ticket{provider,key,url,auto:false}, actor, now)`.
4. Escribe `state.json` + emite `spec.linked`; reporta.

**Auto-raw:**
1. `/vector:raw` detecta una ref en el **texto crudo** (prosa).
2. Resuelve provider (si ambiguo → **no** auto-linkea; deja para `/vector:link`).
3. `vector spec create … --ticket <json>` → `CreateSpecParams.Ticket{…,auto:true}` → `CreateSpec` lo persiste.

**Auto-sync:**
1. `runSync` por cada change: `detectTicket(change)` lee frontmatter `ticket:` del spec doc → si no, escaneo de prosa de los artefactos.
2. Si halla ref con provider resoluble → `Ticket{…,auto:true}`.
3. **Precedencia**: si la card ya tiene ticket manual (`auto:false`), no se pisa. Si no, se linkea
   (en `create` vía `CreateSpecParams.Ticket`; en reconcile vía `Store.LinkSpec`, idempotente).

### Ubicación de archivos nuevos

Solo `kit/commands/vector/link.md` es nuevo; el resto modifica archivos existentes. Sin carpetas nuevas.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `cli/internal/state/store.go` | MODIFICAR | `CreateSpecParams.Ticket *Ticket` + `Store.LinkSpec` (write + evento, idempotente, precedencia auto/manual) | `ProposeSpec` (write+evento) / campo `OpenSpec` en `CreateSpecParams` |
| `cli/cmd/vector/spec_transitions.go` | MODIFICAR | `runSpecLink` (parsea id+ref, `--provider`/`--json`, llama `LinkSpec`) | `runSpecApply`/`leadingID` |
| `cli/cmd/vector/main.go` | MODIFICAR | Dispatch `case "link"` en `runSpec`; usage; threading de `detectTicket` en `runSync` | `runSpec` switch + `runSync` |
| `cli/cmd/vector/ticket.go` | NUEVO | `inferProvider`, `parseRef`, `detectTicket` (heurística inline, stdlib) | — |
| `kit/commands/vector/link.md` | NUEVO | Command `/vector:link [id] [ticket]` (orquesta binario, pide provider si ambiguo) | `kit/commands/vector/propose.md` |
| `cli/internal/scaffold/assets/commands/vector/link.md` | NUEVO (generado) | Copia embebida del command (vía `go generate`) | `…/assets/commands/vector/apply.md` |
| `docs/domain-contract.md` | MODIFICAR | Ampliar §5 con `auto:true` en raw/sync vs `auto:false` manual; precedencia | fila `/vector:link` existente |

### Detalle por archivo

#### cli/internal/state/store.go

Acción: MODIFICAR. (Hoy `CreateSpecParams` **no** tiene campo `Ticket` ni existe `LinkSpec`.)

- `CreateSpecParams` gana `Ticket *Ticket` (nuevo, junto a `OpenSpec`/`NeedsUAT`); `CreateSpec` lo
  asigna a `spec.Ticket` antes del write (si no-nil) y, en ese caso, emite **`EvtSpecLinked`**
  (`SpecLinkedData{Provider,Key,URL,Auto}`) **además** del `EvtSpecCreated`.
- Nuevo `LinkSpec(id string, ticket *Ticket, actor string, now time.Time) (*SpecState, error)`:
  - Lock → `ReadSpec` (error si no existe).
  - **Idempotencia**: si `spec.Ticket` ya iguala `provider+key+url`, no-op (retorna sin re-emitir).
  - **Precedencia**: si `ticket.Auto == true` (origen auto) y `spec.Ticket != nil && !spec.Ticket.Auto`
    (existe manual), no-op (no pisar lo manual).
  - Si pasa: setea `spec.Ticket`, `spec.UpdatedAt`, write atómico, append `EvtSpecLinked` con
    `SpecLinkedData` (incluye `auto`).

Restricción: ningún otro punto escribe `Ticket`; solo `LinkSpec` y `CreateSpec` (sembrado inicial).

#### cli/cmd/vector/ticket.go

Acción: NUEVO. Molde estructural: `cli/cmd/vector/spec_transitions.go` (package main, imports
`state`/`config`, helpers puros). Stdlib (`strings`, `regexp` mínimo, `net/url`, `path`):

- `inferProvider(ref string) (state.TicketProvider, bool)` — retorna provider y `ok=false` si
  ambiguo/no resoluble (key sin URL cuyo patrón no fija provider).
- `parseRef(ref, forced state.TicketProvider) (key, url string, err error)` — si URL: extrae key,
  normaliza; si key con `forced`/inferible: construye URL canónica por provider donde sea posible.
  **`url` puede quedar vacío** cuando solo se da una key sin host (p.ej. jira sin base-url): se
  guarda solo la key (decisión §10/§13). Error si formato inválido.
- `detectTicket(change openspec.Change, root string) (*state.Ticket, bool)` — 1) lee frontmatter
  `ticket:` del spec doc del change; 2) si no, escaneo conservador de prosa por URL completo o key
  con provider reconocible. **Paths de artefactos**: `change.ProposalRel` es campo directo; design
  y tasks se construyen como `filepath.Join(root, change.Dir, "design.md"|"tasks.md")` — espejo de
  cómo `openspec.go` arma `filepath.Join(dirAbs, "design.md")` (no hay `DesignRel`/`TasksRel`).
  Provider ambiguo → no retorna (conservador, no adivina).

#### cli/cmd/vector/main.go

Acción: MODIFICAR.

- `runSpec` switch: `case "link": return runSpecLink(args[1:])`; actualizar `usage`.
- `runSpecCreate` gana un flag **`--ticket`** que acepta un `state.Ticket` **serializado JSON**
  (p.ej. `--ticket '{"provider":"jira","key":"MH-1438","url":"…","auto":true}'`). Si presente, se
  `json.Unmarshal` a `*state.Ticket` y se pasa en `CreateSpecParams.Ticket`. Lo usa `/vector:raw`
  tras detectar el ticket en el texto crudo (el command arma el JSON y lo pasa).
- `runSync`: por cada change, `ticket, ok := detectTicket(ch, root)`. Si `ok`:
  - rama **create** → setear `CreateSpecParams.Ticket = ticket` (auto:true) junto a `NeedsUAT`.
  - rama **reconcile** → tras `ReconcileStatus`, llamar `store.LinkSpec(id, ticket, actor, now)`
    (idempotente; respeta precedencia: no pisa manual). No re-emite si ya está igual.

#### cli/cmd/vector/spec_transitions.go

Acción: MODIFICAR. `runSpecLink(args)`: `leadingID` para `<id>`, segundo positional `<ref>`, flags
`--provider`, `--repo-root`, `--json`. Resuelve provider (flag > inferencia); si ambiguo y sin
flag → error: `ambiguous provider; pass --provider jira|linear|github|other`. Llama
`store.LinkSpec(id, &state.Ticket{…,Auto:false}, actor, now)`; reporta.

#### kit/commands/vector/link.md

Acción: NUEVO. Molde `propose.md`. Entrada `$ARGUMENTS = <id> <ticket-ref>`. Si falta id → listar
(`vector spec list`) y preguntar. Parsea la ref; si el binario reporta provider ambiguo, pide
`--provider` con `AskUserQuestion` y reintenta. No edita `state.json` a mano. Idempotente; si la
card ya tiene un ticket distinto, lo informa y confirma antes de cambiarlo.

> Asset embebido: `kit/commands/vector/link.md` es la copia **autoritativa**. El asset
> `cli/internal/scaffold/assets/commands/vector/link.md` se **regenera** corriendo
> `go -C cli generate ./internal/scaffold/` — la directiva `//go:generate` vive en
> `cli/internal/scaffold/scaffold.go` y copia `kit/commands` → `assets/`. No se edita el asset a
> mano; se commitea como artefacto generado (mismo patrón que `apply.md`).

#### docs/domain-contract.md

Acción: MODIFICAR. En §5, ampliar la nota de `auto`: `auto:true` cuando lo detecta raw/sync;
`auto:false` cuando es `/vector:link` manual; el auto no pisa un manual existente.

---

## 7. API Contract

No aplica como endpoint HTTP nuevo. `/vector:link` y `vector spec link` son superficie CLI. El
`GET /api/board` y el SSE ya exponen `Card.Ticket` (sin cambios de ruta ni versionado); las cards
linkeadas simplemente lo traen poblado.

---

## 8. Criterios de éxito

- [ ] `vector spec link <id> <url-jira|linear|github>` infiere provider, extrae key, persiste con `auto:false`.
- [ ] `vector spec link <id> <key>` con provider ambiguo y sin `--provider` → error accionable, no escribe.
- [ ] `--provider` fuerza el provider y permite linkear una key sola.
- [ ] Evento `spec.linked` emitido con el `auto` correcto.
- [ ] Re-linkear el mismo `provider+key` → no-op (no re-emite).
- [ ] Cambiar de ticket → actualiza `state.json` y re-emite.
- [ ] `/vector:raw` con un ticket en el texto → draft con `ticket.auto:true`; sin provider resoluble → sin ticket (no adivina).
- [ ] `vector sync` sobre un change con frontmatter `ticket:` → card linkeada `auto:true`; auto **no** pisa un ticket manual existente.
- [ ] `GET /api/board` trae `Card.Ticket` en las cards linkeadas (sin regresión del render).
- [ ] Sin regresiones en `create`/`sync`/`propose`/`apply`/`serve`.

### Tests requeridos

- [ ] `inferProvider`: jira/linear/github por URL; ambigüedad (key sola) → `ok=false`.
- [ ] `parseRef`: URL → key+url; key+`--provider` → url canónica; ref inválida → error.
- [ ] `detectTicket`: frontmatter `ticket:` gana; fallback prosa (URL/clear key); prosa ruidosa sin provider → nil.
- [ ] `Store.LinkSpec`: write+evento; idempotencia (mismo ticket); precedencia (auto no pisa manual).
- [ ] `CreateSpec` con `Ticket` en params: persiste + emite `spec.linked`.
- [ ] `runSpecLink`: error de provider ambiguo; éxito con `--json`.
- [ ] Serialización del board (`board` server test): `GET /api/board` trae `card.ticket` poblado
      cuando hay ticket y lo **omite** cuando es nil (guarda el `omitempty`, como en `needsUat`).

### Comandos de verificación

```bash
gofmt -l cli && go -C cli vet ./... && go -C cli test -race ./...
```

La fase no está completa si alguno falla.

---

## 9. Criterios de UX

**No aplica — feature interna (CLI + automatización).** El board ya renderiza el ticket
(`SpecCard`: icono `Tag` + key); esta fase no cambia el visual.

Por subsección del template (Loading / Formularios / Passwords / Errores de UI / Navegación):
**No aplica** — no hay UI nueva ni formularios. **Accesibilidad**: el link del ticket ya existente
en `SpecCard` debe seguir teniendo texto legible (la key) y `title` con el URL; no regresionar.

---

## 10. Decisiones tomadas

El agente no debe cuestionarlas:

- **Detección = frontmatter `ticket:` primero, fallback prosa conservador** (URL completo o key con
  provider reconocible). Raw detecta sobre el texto crudo; sync sobre los artefactos del change.
- **Provider ambiguo (key sin URL): manual exige `--provider`; auto (raw/sync) NO linkea** (no adivina).
- **Primera ref detectada; un solo `Ticket` por spec** (no array en V1).
- **`auto:true` en raw/sync, `auto:false` en manual.** El auto **no pisa** un ticket manual existente.
- **Idempotencia** en `LinkSpec`: mismo `provider+key+url` → no-op.
- **Sin validación contra el tracker** (no hit de API); el binario es el único escritor.
- **`link` no cambia el estado del spec ni crea change OpenSpec.**

Si el agente ve una alternativa mejor, la reporta como observación; no la implementa.

---

## 11. Edge cases

- **Key sola ambigua** (`ABC-123`): manual → error pide `--provider`; auto → no linkea.
- **URL de provider conocido** (`*.atlassian.net/browse/MH-1438`, `linear.app/…`, `github.com/o/r/issues/9`):
  infiere provider + extrae key + normaliza URL.
- **Shorthand github `o/r#123`**: provider github, URL `https://github.com/o/r/issues/123`.
- **Spec ya linkeado, mismo ticket**: no-op (idempotente).
- **Spec ya linkeado, ticket distinto (manual)**: el command confirma antes de cambiar; el binario lo actualiza y re-emite.
- **Auto (raw/sync) sobre card con ticket manual**: no pisa (precedencia manual).
- **Múltiples refs en el texto**: se toma la primera; el resto se ignora en V1.
- **Ref inválida / texto sin ticket**: `link` manual → error; raw/sync → simplemente no linkea (silencioso).
- **Spec inexistente**: `link` → error `spec '<id>' not found`.
- **Spec `closed`/`archived`**: se permite linkear (link no es estado-restrictivo).
- **Atomicidad parcial**: `LinkSpec` escribe el `state.json` (write atómico vía temp+rename) y
  luego appendea el evento — mismo orden que `ReconcileStatus`. Si el append falla, el ticket queda
  persistido sin evento (aceptable, consistente con el patrón existente del `Store`).
- **Key sola sin host** (jira sin base-url): `parseRef` deja `url` vacío; se guarda solo la key y
  el board muestra la key sin link (no es error).

---

## 12. Estados de UI requeridos

No aplica (CLI + automatización). El board no agrega estados nuevos: una card linkeada muestra su
ticket (ya soportado); una sin ticket no muestra nada (ya soportado).

---

## 13. Validaciones

**Cliente (CLI):**

| Campo | Regla | Mensaje |
|---|---|---|
| `<id>` | el spec debe existir | `spec '<id>' not found` |
| `<ref>` | URL válida o key parseable | `invalid ticket reference: <ref>` |
| `--provider` | `jira\|linear\|github\|other` | `invalid provider '<x>'` |
| provider inferido | resoluble o forzado | `ambiguous provider; pass --provider jira\|linear\|github\|other` |

**Servidor (Store):** `LinkSpec` exige `ticket != nil` con `Provider` y `Key` no vacíos; **`URL`
puede ser vacío** (key sola sin host). No valida existencia externa.

---

## 14. Seguridad y permisos

No aplica — sin secrets ni datos sensibles. Las URLs de tickets se persisten en `state.json`
(committed); son referencias, no credenciales. No se loggean tokens.

---

## 15. Observabilidad y logging

- Evento `spec.linked` en `activity.jsonl` con `provider/key/url/auto` (el `auto` traza origen
  manual vs automático) — alimenta `/vector:daily`.
- Errores de parsing/provider del CLI → stderr, claros y accionables.
- No loggear nada sensible (no aplica).

---

## 16. i18n / textos visibles

Proyecto sin i18n formal; mensajes del CLI en inglés (convención del repo).

| Identificador (doc) | Texto |
|---|---|
| link.success | `linked <id> → <provider> <key>` |
| link.error.not-found | `spec '<id>' not found` |
| link.error.invalid-ref | `invalid ticket reference: <ref>` |
| link.error.ambiguous | `ambiguous provider; pass --provider jira\|linear\|github\|other` |

---

## 17. Performance

- `parseRef`/`inferProvider`: O(n) sobre la longitud de la ref (negligible).
- `detectTicket` en sync: una pasada por los artefactos del change (≤3 archivos) por change; sin
  regex pesada. Negligible vs el costo de I/O que sync ya paga.
- Un `Ticket` más en `state.json`/`board.json`: negligible.

---

## 18. Restricciones

El agente no debe:

- Cambiar el struct `Ticket` ni el enum `TicketProvider` (ya estables).
- Validar tickets contra APIs externas.
- Imponer metadata nueva obligatoria en el repo del usuario (el frontmatter `ticket:` es opcional).
- Exponer `link` por HTTP.
- Soportar múltiples tickets/array en V1.
- Pisar un ticket manual con auto-detección.
- Refactorizar helpers no relacionados.

---

## 19. Entregables

- [ ] `cli/cmd/vector/ticket.go`: `inferProvider`/`parseRef`/`detectTicket` + tests.
- [ ] `Store.LinkSpec` + `CreateSpecParams.Ticket` + tests (idempotencia, precedencia).
- [ ] Subcomando `vector spec link` + dispatch + usage.
- [ ] Threading en `runSync` (detectTicket → create/link) y `runSpecCreate` (--ticket).
- [ ] `kit/commands/vector/link.md` + asset embebido (`go generate`).
- [ ] `docs/domain-contract.md` §5 ampliado.
- [ ] Gate Go verde (`gofmt`/`vet`/`test -race`).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `types.go`/`event.go` (Ticket/TicketProvider/EvtSpecLinked/SpecLinkedData ya existen).
- [ ] Revisé `store.go` (`ProposeSpec`/`ReconcileStatus`/`CreateSpec` como molde).
- [ ] Revisé `main.go` (`runSpec`, `runSync`, `syncStatus`/`syncNeedsUAT`) y `spec_transitions.go`.
- [ ] Revisé `board.go` + `SpecCard.tsx` (ticket ya proyectado/renderizado; no rehacer).
- [ ] Revisé `kit/commands/vector/propose.md` (molde de command).
- [ ] Implementé parsing + inferencia + detección (frontmatter > prosa).
- [ ] Implementé `LinkSpec` (idempotencia + precedencia auto/manual) y el threading raw/sync.
- [ ] Implementé el subcomando y el command del kit; corrí `go generate`.
- [ ] Agregué tests (parsing, detección, LinkSpec, create-con-ticket, CLI).
- [ ] No agregué validación externa ni metadata obligatoria; no soporté arrays.
- [ ] Ejecuté gofmt, vet, test -race.
- [ ] No dejé `[...]` ni TODOs sin justificar.

---

## Open questions

- **Resuelto en este spec**: con solo una key sin host, `url` queda vacío y se guarda solo la key
  (el board muestra la key sin link). Futuro: un `base-url` por provider en `.vector/config.json`
  permitiría construir la URL canónica.
- Formato del frontmatter `ticket:`: **constraint** — el implementador elige UN formato (sugerido:
  línea simple `ticket: <provider>:<key>` o `ticket: <url>`), lo documenta en un comentario de
  `detectTicket` y lo refleja en `docs/domain-contract.md` antes de mergear el PR. No dejarlo abierto
  en el código.
- ¿Emitir `ticket.unlinked` o equivalente al cambiar de ticket (audit más fino)? Hoy solo `spec.linked`.
