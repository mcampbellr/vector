# Add `vector detect-ticket` — detección de ticket y lenguaje en el binario

## Why

`/vector:raw` implementaba los 4 tiers de detección de ticket como prosa de instrucción en el
command, ejecutada sobre el modelo Opus. Ese es el tier caro y el inadecuado para parsing
determinista: regexp matching sobre texto libre no requiere razonamiento de lenguaje natural y
produce falsos positivos difícilmente testeables en producción.

El mismo command tampoco tenía acceso directo a `config.Language`: lo detectaba mediante un
fallback ad-hoc sobre specs existentes. Cualquier cambio a los tiers de detección o a la config
de lenguaje requería editar prosa de instrucción, sin cobertura de tests.

El binario Go ya concentra toda la lógica de detección de ticket para `vector sync`
(`detectTicket` file-based). Extraer la variante texto-libre consolida el conocimiento en un
único lugar testeable, determinista y de costo cero en tokens.

## What changes

- Nueva función `ticketFromShorthands(content string) *state.Ticket` en `ticket.go`: escanea
  texto libre en busca de shorthands `<provider>:<key>` (tier 2, única función genuinamente
  nueva en la capa de detección).
- Nueva función `detectTicketFromText(text, defaultProvider, keyPrefixes)` en `ticket.go`:
  orquesta 3 capas — URL de tracker reconocido (tier 1), shorthand (tier 2), bare key con
  cue-word o prefijo configurado (tiers 3–4, gated en `defaultProvider`).
- Nueva var `shorthandRe` a nivel de paquete en `ticket.go`.
- Nuevo subcomando de primer nivel `vector detect-ticket`: lee texto por stdin (o
  `--text-file`), carga config, llama a `detectTicketFromText`, emite JSON con `ticket`,
  `language`, `defaultTicketProvider`, `ticketKeyPrefixes`.
- `case "detect-ticket"` + `runDetectTicket` + línea en `usage()` en `main.go`.
- Steps 4 y 7 de `kit/commands/vector/raw.md` simplificados: una sola invocación del binario
  reemplaza la detección de lenguaje ad-hoc y los 4 tiers de prosa.
- `cli/internal/scaffold/assets/commands/vector/raw.md` sincronizado (copia vendored).
- Tests `TestTicketFromShorthands` y `TestDetectTicketFromText` (table-driven) en
  `ticket_test.go`.

## Scope

**In:**
- `ticketFromShorthands` y `detectTicketFromText` en `cli/cmd/vector/ticket.go`.
- `var shorthandRe` (package-level) en el mismo archivo.
- Subcomando `vector detect-ticket` con flags `--repo-root`, `--text-file`, `--json`.
- `case "detect-ticket"` en el switch de `main()` y `runDetectTicket`.
- Steps 4 y 7 de `raw.md` (kit + assets vendored).
- Tests nuevos en `ticket_test.go`.
- Actualización de `usage()` con la descripción del nuevo subcomando.

**Out:**
- `detectTicket` (file-based, usado por `sync`) y todos sus helpers — sin tocar.
- Schema de `state.Ticket`, `state.TicketProvider`, `config.Config` — sin cambios.
- Detección de lenguaje desde texto libre — solo se expone `config.Language`.
- Panel web, API HTTP, `vector sync`, `vector init`, cualquier otro subcomando existente.
- Steps 1, 2, 3, 5, 6, 8, 9, 10 y 11 de `raw.md`, Hard Rules y frontmatter — intactos.

Authored spec: `.vector/specs/detect-ticket-and-language-in-binary/spec.md`.
