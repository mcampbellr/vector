# Add /vector:jira MCP integration

## Why

Un developer que trabaja un spec de Vector con un ticket de Jira enlazado (`state.Ticket`,
provider `jira`) tiene que salir de Claude Code para leer el ticket o comentar en él. No existe
ningún comando `/vector:*` que hable con un MCP externo, así que esa capacidad no existe hoy. Al
mismo tiempo, el binario `vector` no debe tocar credenciales de Jira ni volverse un cliente de
Jira: la integración tiene que ser thin y agnóstica, reusable para futuros providers (Linear,
GitHub).

## What changes

- **Nuevo project command `/vector:jira`** (`kit/commands/vector/jira.md`) con tres operaciones
  sobre el **MCP de Jira ya conectado en Claude Code**:
  - `read <spec-id>` — resuelve el ticket enlazado y lo muestra (efímero, no persiste nada).
  - `comment <spec-id> <msg>` — redacta el comentario, **confirma explícitamente** (draft-then-confirm),
    lo postea vía MCP, y registra el hecho vía el binario.
  - `list <jql>` — búsqueda JQL de solo lectura (superficie power-user).
- **Nuevo evento aditivo `ticket.commented`** (`EvtTicketCommented` + `TicketCommentedData{Provider,Key}`)
  en `cli/internal/state/event.go` — sin contenido del ticket.
- **Nuevo método `Store.CommentTicket`** en `cli/internal/state/standup.go` (mirror de `WorkLog`):
  anexa el evento a `activity.jsonl` sin tocar `state.json`.
- **Nuevo subcomando `vector spec comment-ticket <id> --provider jira --key <key> [--json]`** bajo
  `newSpecCmd()`, invocado por el command tras confirmar y publicar en Jira.
- **Nuevo `case state.EvtTicketCommented`** en la proyección y el `Timeline` de standup
  (`cli/internal/standup/standup.go`) para que el evento aparezca en `/vector:standup` y en la
  timeline del board.
- **Detección + degradación explícita del MCP** al inicio del command (mirror del `gh`-availability
  de `comment.md`).

## Scope

- **In**: el command markdown, el `EventType` + payload, el método `Store`, el subcomando cobra, el
  `case` de standup, el consentimiento draft-then-confirm y la detección de MCP.
- **Out**: cliente MCP en Go o cualquier dep externa nueva; persistir contenido del ticket en
  `state.json`/`activity.jsonl`; Linear/GitHub; extender `security/destructive-ops-consent.md`;
  un comando `vector spec show --json`; tocar `/vector:raw`/`/vector:apply`; gestión de auth de Jira
  (vive dentro del MCP).

Authored spec: `.vector/specs/add-jira-mcp-integration/spec.md`.
