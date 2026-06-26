# Tasks — add-ticket-linking

## 1. State + eventos

- [x] 1.1 `CreateSpecParams.Ticket *Ticket`: `CreateSpec` lo asigna y emite `EvtSpecLinked` además de `EvtSpecCreated`.
- [x] 1.2 `Store.LinkSpec(id, ticket, actor, now)`: lock → read → idempotencia (mismo provider+key+url) → precedencia (auto no pisa manual) → write atómico + `EvtSpecLinked`.
- [x] 1.3 Tests: `LinkSpec` (write+evento, idempotencia, precedencia); `CreateSpec` con `Ticket` (persiste + emite).

## 2. Parsing / detección (`cli/cmd/vector/ticket.go`)

- [x] 2.1 `inferProvider(ref)` — jira/linear/github por URL; ambiguo (key sola) → `ok=false`.
- [x] 2.2 `parseRef(ref, forced)` — URL → key+url normalizada; key+provider → url canónica; `url` vacío permitido; ref inválida → error.
- [x] 2.3 `detectTicket(change, root)` — frontmatter `ticket:` gana; fallback prosa conservador; ambiguo → nil. Paths design/tasks vía `filepath.Join(root, change.Dir, …)`.
- [x] 2.4 Tests de los tres helpers (incluye prosa ruidosa sin provider → nil).

## 3. Binario

- [x] 3.1 `runSpecLink` en `spec_transitions.go` (`<id>` + `<ref>`, `--provider`/`--repo-root`/`--json`); provider ambiguo sin flag → error accionable.
- [x] 3.2 `main.go`: dispatch `case "link"`; usage; `--ticket` (JSON) en `runSpecCreate`.
- [x] 3.3 Threading en `runSync`: `detectTicket` → create (params, auto:true) o reconcile (`LinkSpec`).
- [x] 3.4 Test `runSpecLink`: error de provider ambiguo; éxito con `--json`.

## 4. Command (kit)

- [x] 4.1 `kit/commands/vector/link.md` (molde `propose.md`): orquesta el binario, pide `--provider` si ambiguo, confirma cambio de ticket existente.
- [x] 4.2 Asset embebido vía `go -C cli generate ./internal/scaffold/`.

## 5. Docs

- [x] 5.1 `docs/domain-contract.md` §5: ampliar nota `auto` (raw/sync `auto:true` vs manual `auto:false`; auto no pisa manual).

## 6. Verificación

- [x] 6.1 `gofmt -l cli && go -C cli vet ./... && go -C cli test -race ./...` verdes.
- [x] 6.2 `GET /api/board` trae `card.ticket` poblado cuando hay ticket y lo omite (`omitempty`) cuando es nil; sin regresión del render.
- [x] 6.3 Sin regresiones en `create`/`sync`/`propose`/`apply`/`serve`.
