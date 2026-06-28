# Tasks — detect-ticket-and-language-in-binary

## 1. Funciones de detección en `ticket.go`

- [x] 1.1 Agregar `var shorthandRe = regexp.MustCompile(...)` a nivel de paquete, junto al grupo de vars regex existentes, con comentario indicando que el set de providers debe mantenerse sincronizado con `splitShorthand`.
- [x] 1.2 Implementar `ticketFromShorthands(content string) *state.Ticket`: iterar `shorthandRe`, aplicar `splitShorthand` por match, acumular candidatos únicos, retornar el único o nil en ambigüedad/cero.
- [x] 1.3 Implementar `detectTicketFromText(text string, defaultProvider state.TicketProvider, keyPrefixes []string) *state.Ticket`: orquestar tier 1 (`ticketFromProse`), tier 2 (`ticketFromShorthands`), tiers 3+4 (`ticketFromContext`, gated en `defaultProvider != ""`); setear `Auto = true` tras tiers 1 y 2.
- [x] 1.4 Verificar que ninguna función helper existente (`ticketFromProse`, `ticketFromContext`, `splitShorthand`, `denylistedKey`, `pickSingleKey`, `inferProvider`, `extractKey`, etc.) fue modificada.

## 2. Subcomando en `main.go`

- [x] 2.1 Definir `DetectTicketResponse` como struct tipado (`ticket *state.Ticket`, `language`, `defaultTicketProvider string`, `ticketKeyPrefixes []string`) — sin `any`/`interface{}`.
- [x] 2.2 Implementar `runDetectTicket(args []string) error`: flags `--repo-root`, `--text-file` (default `"-"`), `--json`; resolver repo root; cargar config (not-found → `Config{}`, error corrupta → propagar); leer texto; llamar `detectTicketFromText`; serializar y emitir JSON a stdout.
- [x] 2.3 Agregar `case "detect-ticket": err = runDetectTicket(os.Args[2:])` en el switch de `main()` antes de `case "help"`.
- [x] 2.4 Agregar entrada en `usage()` para `detect-ticket` con descripción breve ("detect a ticket and language from text (JSON output)") sin modificar ninguna entrada existente.
- [x] 2.5 Confirmar que `--text-file "-"` lee de stdin y un path explícito lee del archivo; archivo inexistente → error stderr exit 1; stdin EOF → texto vacío → ticket null exit 0.

## 3. Tests en `ticket_test.go`

- [x] 3.1 Implementar `TestTicketFromShorthands` (table-driven) con casos: un shorthand jira, un shorthand linear, duplicado idéntico (retorna), duplicado distinto (nil), providers distintos (nil), sin shorthands (nil), provider desconocido no matchea regex (nil).
- [x] 3.2 Implementar `TestDetectTicketFromText` (table-driven; campos `text`, `defaultProvider`, `keyPrefixes`, `wantProvider`, `wantKey`, `wantAuto`, `wantNil`) con casos: tier 1 URL jira, tier 1 URL linear, tier 1 host desconocido → nil, tier 1 dos URLs distintas → nil, tier 2 shorthand, tier 1 gana sobre tier 2, tier 3 cue-word con defaultProvider, tier 3 sin defaultProvider → nil, tier 4 prefijo configurado, ADR denylist → nil, RFC denylist → nil, texto vacío → nil.
- [x] 3.3 Confirmar que ningún test existente en `ticket_test.go` fue modificado o eliminado; ejecutar suite completa y verificar que todos pasan.

## 4. Actualización de `kit/commands/vector/raw.md`

- [x] 4.1 Step 4: reemplazar la lógica de detección de lenguaje ad-hoc por la invocación `vector detect-ticket --repo-root $REPO_ROOT --json` con `$RAW_IDEA` en stdin; almacenar resultado como `DETECT_JSON`; si `DETECT_JSON.language` es non-empty usarlo como `SPEC_LANGUAGE`, si vacío caer al fallback de globbing existente.
- [x] 4.2 Step 7: reemplazar los 4 tiers en prosa por la lectura de `DETECT_JSON.ticket` (calculado en step 4 — no volver a invocar el binario); si non-null → `TICKET_JSON = DETECT_JSON.ticket`; si null → dejar `TICKET_JSON` unset. Añadir nota de token routing.
- [x] 4.3 Confirmar que frontmatter (`name`, `description`, `category`, `tags`), Hard Rules y steps 1, 2, 3, 5, 6, 8, 9, 10, 11 permanecen byte-a-byte idénticos al original.

## 5. Vendored assets

- [x] 5.1 Regenerar `cli/internal/scaffold/assets/commands/vector/raw.md` vía `go generate` en `cli/` para sincronizar la copia vendored con `kit/commands/vector/raw.md` post-cambio.
- [x] 5.2 Verificar que el contenido del archivo regenerado es idéntico al de `kit/commands/vector/raw.md`.

## 6. Verificación

- [x] 6.1 Ejecutar `gofmt -l cli` y confirmar output vacío.
- [x] 6.2 Ejecutar `go -C cli vet ./...` sin warnings ni errores.
- [x] 6.3 Ejecutar `go -C cli test ./...` con todos los tests en verde, incluyendo los de `detect`, `TestInferProvider`, `TestDetectTicket` y los nuevos.
- [x] 6.4 Smoke test manual: `echo "fix jira:ACME-12 issue" | vector detect-ticket --repo-root <path> --json` produce JSON válido con `ticket.provider == "jira"` y `ticket.key == "ACME-12"`.
- [x] 6.5 Smoke test config ausente: `echo "" | vector detect-ticket --repo-root /tmp/no-vector --json` produce JSON con `ticket: null`, `language: ""`, exit 0.
