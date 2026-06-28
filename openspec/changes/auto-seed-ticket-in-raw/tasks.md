# Tasks — auto-seed-ticket-in-raw

## 1. Detección en `/vector:raw` (`kit/commands/vector/raw.md`, paso 7)

- [x] 1.1 Reemplazar la línea de "Detect a ticket … note it for /vector:link" por un sub-paso de
      detección con precedencia por confianza: URL → shorthand → cue-word(+default) →
      prefijo(+default).
- [x] 1.2 Tier 1 (URL): inferir provider por host (`atlassian.net`/`jira`→jira, `linear.app`→
      linear, `github.com`→github; host desconocido → no sembrar). Extraer key (GitHub
      `owner/repo#N`; Jira/Linear `KEY-123`). Dos URLs distintas → descartar.
- [x] 1.3 Tier 2 (shorthand `<provider>:<key>`): escaneo explícito con semántica de
      `splitShorthand` (provider ∈ jira|linear|github|other). **Nuevo** (no en sync prose).
- [x] 1.4 Tier 3 (cue-word): línea con `ticket|issue|ref|tracking|jira|linear|github :` (regex
      `ticketCueRe`, tolera ws/`>`/`**`) + key; gated por `defaultTicketProvider`. Empate →
      descartar.
- [x] 1.5 Tier 4 (prefijo): `ticketKeyPrefixes` en prosa; gated por `defaultTicketProvider`.
      Empate → descartar.
- [x] 1.6 Omitir keys con prefijo en denylist (`ADR`/`RFC`). Si nada resuelve → no definir
      `TICKET_JSON`.
- [x] 1.7 Construir `TICKET_JSON = {"provider","key","url|","auto":true}`.

## 2. Sembrado y reporte (`raw.md`, pasos 9 y 11)

- [x] 2.1 Paso 9: añadir `[--ticket "$TICKET_JSON"]` al `vector spec create`.
- [x] 2.2 Si el binario rechaza el `--ticket` (JSON inválido/provider desconocido), reintentar la
      creación **sin** `--ticket` (no abortar) y seguir el camino del hint.
- [x] 2.3 Paso 11: `linked <KEY> (<provider>)` si se sembró; `ticket detected but ambiguous — link
      it with /vector:link` si ambiguo/sin gate; sin referencia → no mencionar ticket.

## 3. Asset embebido

- [x] 3.1 `go -C cli generate ./...` para regenerar
      `cli/internal/scaffold/assets/commands/vector/raw.md`; verificar que coincide con la fuente.

## 4. Gate

- [x] 4.1 `gofmt -l cli` (vacío), `go -C cli vet ./...`, `go -C cli test ./...` (ticket_test.go +
      scaffold), `go -C cli build ./...` — todos verdes.
- [x] 4.2 Verificación por ejemplos: URL (jira/linear/github), shorthand, cue-word con/sin
      `defaultTicketProvider`, conflicto de tier, sin referencia, `--ticket` malformado.

## 5. (Opcional, fuera de este change) Config del repo

- [ ] 5.1 Si se quiere que el ejemplo `ticket: MH-1653` auto-siembre en ESTE repo, configurar
      `defaultTicketProvider: jira` en `.vector/config.json` (cambio de config, no de este spec).
