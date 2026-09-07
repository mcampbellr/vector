# Design — add-jira-mcp-integration

## Decisiones clave

- **Arquitectura thin, command-driven**: el MCP de Jira lo invoca el command dentro de Claude Code;
  el binario `vector` nunca habla con Jira ni recibe credenciales. Su única responsabilidad es
  persistir el **hecho** del comentario (provider+key) como evento aditivo — igual que `worklog`
  registra trabajo sin ser quien lo hizo.
- **CLI-owns-writes**: el command orquesta (resuelve ticket, invoca MCP, confirma, llama al binario);
  el binario es el único escritor de `.vector/`. El command nunca edita `.vector/` a mano.
- **Evento aditivo, nunca transición**: `ticket.commented` no cambia `status` del spec ni bumpea
  `EventVersion` (análogo a `work.logged`/`sketch.attached`).
- **Draft-then-confirm sin excepción para `comment`**: nunca se postea a Jira sin que el usuario vea
  el texto exacto y confirme explícitamente, por operación, cada vez (mirror `comment.md:56,212-213`).
- **Contenido de Jira = texto no confiable**: nunca se interpola en `Bash(vector *)` como string
  libre (riesgo de command injection); nunca se escribe a `state.json`; `state.Ticket` no gana campos
  (sigue `{Provider,Key,URL,Auto}`). Solo provider+key llegan a `activity.jsonl` (local, gitignored).
- **Detección + degradación explícita del MCP es obligatoria**: primera excepción deliberada a la
  doctrina no-MCP del kit; el command exige detectar las tools de Jira y degradar con un mensaje
  accionable si faltan, nunca fingir un resultado ni fallar en silencio.
- **Jira es el primero de N providers de ticket-MCP**: el diseño (evento genérico, resolución vía
  `state.Ticket`) es reusable por provider; Linear/GitHub quedan fuera de esta fase pero se nombran
  para preservar el posicionamiento agnóstico.

## Superficie

- `cli/internal/state/event.go`: `EvtTicketCommented` + `TicketCommentedData{Provider,Key}`.
- `cli/internal/state/standup.go`: `Store.CommentTicket(id, data, actor, now)` — mirror de `WorkLog`;
  lee el spec, marshalea, `appendEvent`; falla si el spec no existe; nunca toca `state.json`.
- `cli/internal/standup/standup.go`: `case state.EvtTicketCommented` en la proyección (~:112) y en
  `Timeline` (~:169); tolera JSON malformado igual que `WorkLoggedData`.
- `cli/cmd/vector/main.go` + `cli/cmd/vector/{ticket,jira}.go`: `newSpecCommentTicketCmd()`
  (`vector spec comment-ticket [id] --provider --key [--json]`), mirror de `newSpecFixCmd`; valida
  provider con `validProvider` y `--key` no vacío; nunca acepta el texto del comentario.
- `kit/commands/vector/jira.md`: el command (`read`/`comment`/`list`) con hard rules de MCP +
  consentimiento; copia regenerada en `cli/internal/scaffold/assets/commands/vector/jira.md`.

## Flujo

- **`read`** → resuelve ticket vía `state.json` → detecta MCP → invoca tool de lectura → muestra
  (no llama al binario).
- **`comment`** → resuelve ticket → detecta MCP → redacta y muestra draft → `AskUserQuestion` →
  postea vía MCP solo tras confirmación → `vector spec comment-ticket …` registra el hecho → reporta.
- **`list`** → detecta MCP → invoca búsqueda JQL → muestra resultados (no toca estado).

## Open question bloqueante

- **Contrato exacto de nombres de tool del MCP de Jira** (Atlassian remote oficial vs `mcp-atlassian`
  comunitario exponen nombres distintos): sin resolverlo, `jira.md` no puede declarar un
  `allowed-tools` definitivo. **Bloquea completar el markdown del command**; los cambios de Go son
  independientes y pueden avanzar. Ver spec §4 y Open questions.
