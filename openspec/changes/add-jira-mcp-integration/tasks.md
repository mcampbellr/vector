# Tasks — add-jira-mcp-integration

## 1. State + eventos

- [ ] 1.1 `EvtTicketCommented EventType = "ticket.commented"` + `TicketCommentedData{Provider,Key}` en `event.go` (mirror `SketchAttachedData`).
- [ ] 1.2 `Store.CommentTicket(id, data, actor, now)` en `standup.go` (mirror `WorkLog`): lee spec, marshalea, `appendEvent`; nunca toca `state.json`; falla si el spec no existe.
- [ ] 1.3 Tests `state`: round-trip de `TicketCommentedData`; `CommentTicket` anexa el evento correcto y falla sobre spec inexistente.

## 2. Standup / proyección

- [ ] 2.1 `case state.EvtTicketCommented` en la proyección global (~:112) — reusar campo existente de `SpecActivity` si basta, no inventar campo nuevo.
- [ ] 2.2 `case state.EvtTicketCommented` en `Timeline` (~:169) — nota fija tipo `"commented on <provider> <key>"`, nunca el contenido.
- [ ] 2.3 Tests `internal/standup`: el evento se refleja en proyección + `Timeline` (mirror de los de `EvtWorkLogged`).

## 3. Binario

- [ ] 3.1 `newSpecCommentTicketCmd()` (`vector spec comment-ticket [id] --provider --key [--json]`) en `ticket.go` o nuevo `jira.go`, mirror de `newSpecFixCmd`; valida provider (`validProvider`) y `--key` no vacío; nunca acepta el texto del comentario.
- [ ] 3.2 Registrar en `newSpecCmd().AddCommand(...)` y actualizar el usage del padre en `main.go`.
- [ ] 3.3 Tests `cmd/vector`: éxito, `--provider` inválido, `--key` vacío, spec inexistente.

## 4. Command (kit)

- [ ] 4.1 `kit/commands/vector/jira.md` (`read`/`comment`/`list`) con hard rules: draft-then-confirm, detección + degradación de MCP, contenido no confiable, `state.json` nunca recibe contenido de Jira, `list` solo lectura.
- [ ] 4.2 Resolver la Open question del contrato de tool-names del MCP (o documentar el placeholder configurable) antes de dar `jira.md` por completo.
- [ ] 4.3 Regenerar `cli/internal/scaffold/assets/commands/vector/jira.md` (`go generate`); no editar a mano.

## 5. Verificación

- [ ] 5.1 `go generate` / `gofmt -l` / `go vet` / `go test` / `go build` verdes; `TestAssetsMatchKit` pasa.
- [ ] 5.2 e2e: `comment-ticket` registra `ticket.commented` en `activity.jsonl` sin tocar `state.json`; `/vector:standup` y timeline lo reflejan.
- [ ] 5.3 Manual QA: `/vector:jira` degrada explícitamente con el MCP ausente; draft-then-confirm no publica sin confirmación (necesita un MCP de Jira real).
