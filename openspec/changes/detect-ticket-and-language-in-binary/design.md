# Design — detect-ticket-and-language-in-binary

## Decisiones clave

- **Un único subcomando para ticket + language:** devolver ambos en el mismo JSON evita que
  `raw.md` haga dos invocaciones del binario con dos cargas de config. `detect-ticket` expone
  `language` como campo adicional de la misma respuesta — cero costo extra.

- **Subcomando de primer nivel (`vector detect-ticket`), no sub-sub (`vector spec detect-ticket`):**
  el subcomando opera sobre texto libre antes de que el spec exista. Pertenece al nivel de
  utilidades (`sync`, `init`, `serve`), no al sub-árbol de `spec`.

- **`detectTicketFromText` independiente de `detectTicket` (file-based):** el file-based
  itera artefactos en disco (frontmatter, branchKey, context files); el text-based opera
  sobre un `string`. Interfaces distintas; unificarlos rompería la del file-based sin ganancia.

- **`ticketFromShorthands` como función separada:** sigue la convención `func ticketFromX(content string) *state.Ticket` del archivo. Es la única función genuinamente nueva en la capa
  de detección; el resto de tiers reutiliza helpers existentes.

- **Sin tier de frontmatter ni branchKey en `detectTicketFromText`:** el raw idea no es un
  artefacto OpenSpec con YAML frontmatter; `branchKey` es un fallback específico de sync.
  Incluirlos introduciría falsos positivos sin utilidad.

- **`config.Language` como única fuente de lenguaje:** el binario expone lo configurado; no
  detecta el idioma del texto de entrada. El fallback por globbing de specs existentes queda
  en `raw.md` para cuando `language == ""`.

- **Config ausente = defaults vacíos, no error:** `raw.md` puede correr antes de `vector init`.
  Solo la config corrupta (provider inválido que `config.Load` rechaza) propaga como error.

- **`shorthandRe` compilada a nivel de paquete:** sigue el patrón de `jiraKeyRe`, `ticketURLRe`,
  `ticketCueRe` — nunca compilar en cada llamada.

- **Struct de respuesta tipado explícitamente:** sin `any`/`interface{}`, consistente con las
  convenciones de `cli/` y el patrón de `runSpecCreate`.

## Arquitectura

### Capas afectadas

| Archivo | Acción | Contenido |
|---|---|---|
| `cli/cmd/vector/ticket.go` | MODIFICAR | `shorthandRe` var · `ticketFromShorthands` · `detectTicketFromText` |
| `cli/cmd/vector/main.go` | MODIFICAR | `case "detect-ticket"` · `runDetectTicket` · línea en `usage()` |
| `cli/cmd/vector/ticket_test.go` | MODIFICAR | `TestTicketFromShorthands` · `TestDetectTicketFromText` |
| `kit/commands/vector/raw.md` | MODIFICAR | Steps 4 y 7 únicamente |
| `cli/internal/scaffold/assets/commands/vector/raw.md` | MODIFICAR (regenerado) | Copia vendored vía `go generate` |

**No afectadas:** `state/`, `config/`, `board/`, `web/`, API HTTP.

### Flujo de `runDetectTicket`

```
stdin / --text-file → io.ReadAll
resolveRepoRoot(--repo-root)
config.Load(root) → Config{} si not-found; error si corrupta
detectTicketFromText(text, cfg.ResolvedDefaultTicketProvider(), cfg.NormalizedTicketKeyPrefixes())
json.NewEncoder(os.Stdout).Encode(DetectTicketResponse{...})
```

### Cascade de `detectTicketFromText`

```
tier 1: ticketFromProse(text)          → URL de tracker reconocido; Other filtrado
tier 2: ticketFromShorthands(text)     → shorthand <provider>:<key>
tier 3+4: ticketFromContext(text, …)   → bare key con cue-word o prefijo (gated: defaultProvider != "")
→ nil si ningún tier resuelve
```
`Auto = true` se setea tras tier 1 y 2; tier 3+4 ya retorna con `Auto:true`.

### Lógica de `ticketFromShorthands`

Itera `shorthandRe.FindAllStringSubmatch(content, -1)`. Por cada match aplica `splitShorthand`
para normalizar. Acumula candidatos únicos (`provider + key`). Si exactamente uno → retorna ese
ticket (sin `Auto`, el caller setea). Si dos distintos o ninguno → nil.

### Contrato de salida del subcomando

```json
{
  "ticket": {"provider":"jira","key":"MH-123","url":"…","auto":true} | null,
  "language": "es" | "",
  "defaultTicketProvider": "jira" | "",
  "ticketKeyPrefixes": ["MH"] | []
}
```

Exit `0` siempre que no haya error de I/O o config corrupta. `ticket: null` no es un error.
`ticketKeyPrefixes` nunca es `null` (siempre array, vacío si no configurado).
