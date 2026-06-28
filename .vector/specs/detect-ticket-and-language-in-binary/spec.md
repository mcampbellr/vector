# Spec: Ticket y lenguaje — detección en el binario Go

## 1. Objetivo

Construir `vector detect-ticket`: el subcomando que concentra toda la lógica de detección de
ticket y exposición de lenguaje configurado en el binario Go, eliminando la reimplementación de
esa lógica como prosa de prompt en `kit/commands/vector/raw.md` §7.

Esta feature permite que el command `/vector:raw` delegue la detección de ticket al binario
(cero tokens; determinista; testeable en Go) en lugar de ejecutarla en prosa sobre el modelo
Opus — que es el tier caro y el inadecuado para parsing determinista. Al mismo tiempo, expone
`config.Language` en el mismo JSON de respuesta, eliminando también la detección de lenguaje
de proyecto como paso extra en el command.

## 2. Alcance

### Incluido en esta fase

- Nueva función `detectTicketFromText(text string, defaultProvider state.TicketProvider, keyPrefixes []string) *state.Ticket` en `cli/cmd/vector/ticket.go`: aplica los mismos tiers de detección que usa `detectTicket` (para sync), pero sobre texto libre en vez de artefactos en disco. Tiers en orden: (1) URL de tracker reconocido, (2) shorthand `<provider>:<key>`, (3) bare key con cue-word, (4) bare key con prefijo configurado. Skip ADR/RFC. Ambigüedad → nil.
- Nueva función `ticketFromShorthands(content string) *state.Ticket` en `ticket.go`: escanea texto libre para ocurrencias de `(jira|linear|github|other):<key>` (tier 2 no existe hoy como función Go; es la única función genuinamente nueva en la capa de detección).
- Nuevo subcomando de primer nivel `vector detect-ticket`: recibe texto por stdin (o `--text-file`), carga config, llama a `detectTicketFromText`, y devuelve JSON `{"ticket":…|null,"language":"…","defaultTicketProvider":"…","ticketKeyPrefixes":[…]}`.
- Actualización de `kit/commands/vector/raw.md` y su copia vendored en `cli/internal/scaffold/assets/commands/vector/raw.md`:
  - Step 4 (detect spec language): usar `language` del JSON cuando non-empty; caer al fallback de detección por ejemplo solo cuando vacío.
  - Step 7 (detect ticket): reemplazar la prosa de los 4 tiers con una llamada al binario y parseo del JSON.
- Tests nuevos en `cli/cmd/vector/ticket_test.go`: `TestTicketFromShorthands` y `TestDetectTicketFromText` (table-driven).
- Actualización de `usage()` en `main.go` para incluir `detect-ticket`.

### Fuera de scope

- Refactorizar `detectTicket` (el file-based, usado por `vector sync`): se mantiene intacto; solo comparten los helpers ya existentes (`ticketFromProse`, `ticketFromContext`, etc.).
- Modificar la lógica de cualquier función de detección existente (`ticketFromProse`, `ticketFromContext`, `ticketFromFrontmatter`, `splitShorthand`, `denylistedKey`, `pickSingleKey`, `inferProvider`, `extractKey`).
- Cambiar el schema de `SpecState`, `state.Ticket` ni ningún otro tipo del state.
- Exponer lenguaje como subcomando separado: se incluye en el mismo response de `detect-ticket` por eficiencia (una invocación, una carga de config).
- Implementar detección de lenguaje desde texto libre: solo se expone `config.Language`; si está vacío, `raw.md` mantiene su fallback actual de detección por ejemplo existente.
- Modificar steps 1, 2, 3, 5, 6, 8, 9, 10 ni 11 de `raw.md`, ni sus Hard Rules ni el frontmatter.
- Panel web ni API HTTP: este cambio es CLI-only.
- Cambios en `vector sync`, `vector init`, `vector update`, ni ningún otro subcomando existente.

El agente no implementa nada fuera de este alcance aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Binario: **Go** (módulo único, stdlib; `cli/`). Sin dependencias externas.
- Project command: **Markdown + instrucciones** orquestado por Claude (`kit/commands/vector/raw.md`).

### Versiones relevantes

- Go: `1.26` (citado de `cli/go.mod`).
- Dependencias runtime del subcomando: ninguna. Usa solo `encoding/json`, `flag`, `io`, `os`, `regexp`, `strings` — todos stdlib.

### Patrones existentes a respetar

- `cli/cmd/vector/main.go` → switch de primer nivel en `main()`: `case "detect-ticket"` al mismo nivel que `"sync"`, `"init"`, `"serve"`, `"spec"`. Seguir el patrón `err = runXxx(os.Args[2:])`.
- `cli/cmd/vector/ticket.go`: todas las funciones de detección de ticket viven aquí. Las funciones nuevas (`detectTicketFromText`, `ticketFromShorthands`) van en este mismo archivo.
- Patrón de funciones de detección: `func ticketFromX(content string) *state.Ticket` — nil = no encontrado (sin error). Seguir exactamente esta firma para `ticketFromShorthands`.
- Patrón de subcomando de tooling: `--json` siempre emite JSON a stdout; errores a stderr; exit 1 en error. Ver `runSpecCreate`, `runSync`.
- Regex compilada a nivel de paquete (`var shorthandRe = regexp.MustCompile(…)`) — no compilar en cada llamada. Ver `jiraKeyRe`, `ticketURLRe`, `ticketCueRe`, `frontmatterRe` como ejemplos.
- `gofmt`/`goimports` obligatorio; `go vet` sin warnings.
- Sin `interface{}`/`any`; la respuesta JSON se tipea como `struct` nombrado.
- Contextos (`context.Context`): no necesario — el subcomando es síncrono y sin I/O de red.
- Assets vendored: `cli/internal/scaffold/assets/commands/vector/raw.md` se regenera con `go generate`; no se edita a mano.
- Tests: paquete `testing` estándar, table-driven. Ver `TestInferProvider` y `TestDetectTicket` como modelo.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `cli/cmd/vector/ticket.go` con `ticketFromProse`, `ticketFromContext`, `splitShorthand`, `denylistedKey`, `pickSingleKey`, `inferProvider` (ya existen, verificados en el repo).
- [x] `cli/internal/config/config.go` con campos `Language`, `DefaultTicketProvider`, `TicketKeyPrefixes` y métodos `ResolvedLanguage()`, `ResolvedDefaultTicketProvider()`, `NormalizedTicketKeyPrefixes()` (ya existen, verificados).
- [x] `cli/internal/state/types.go` con `state.Ticket`, `state.TicketProvider`, constantes `TicketJira`, `TicketLinear`, `TicketGitHub`, `TicketOther` (ya existen).
- [x] `cli/cmd/vector/ticket_test.go` con suite de tests existentes que no deben romperse.
- [x] `kit/commands/vector/raw.md` con steps 4 y 7 en su forma actual (a modificar en esta fase).

Si alguna dependencia no existe, el agente se detiene y reporta qué falta. No inventa contratos.

---

## 5. Arquitectura

### Patrón

Extracción de lógica determinista del prompt a Go. El binario concentra la decisión; el command Claude solo invoca una vez y parsea el JSON resultado. Un solo subcomando provee tanto el ticket detectado como el lenguaje configurado — evitando dos invocaciones del binario con carga de config duplicada.

### Capas afectadas

- **Binario CLI** (`cli/cmd/vector/main.go`): sí — dispatch `case "detect-ticket"` + `runDetectTicket`.
- **ticket.go** (`cli/cmd/vector/ticket.go`): sí — `detectTicketFromText` + `ticketFromShorthands`.
- **ticket_test.go** (`cli/cmd/vector/ticket_test.go`): sí — tests nuevos.
- **Project command** (`kit/commands/vector/raw.md`): sí — steps 4 y 7 simplificados.
- **Assets vendored** (`cli/internal/scaffold/assets/commands/vector/raw.md`): sí — copia sincronizada vía `go generate`.
- **state**: no.
- **config**: no (ya tiene los campos; se lee, no se escribe).
- **web/**: no.
- **board server / API HTTP**: no.

### Flujo esperado

1. `/vector:raw` ejecuta `vector detect-ticket --repo-root $REPO_ROOT --json` con el texto de `$RAW_IDEA` por stdin.
2. El subcomando intenta `config.Load(root)`. Si el archivo no existe, usa `Config{}` vacío — no es error.
3. Llama a `detectTicketFromText(rawText, cfg.ResolvedDefaultTicketProvider(), cfg.NormalizedTicketKeyPrefixes())`.
4. Serializa y emite JSON a stdout.
5. `/vector:raw` parsea el JSON: si `language` non-empty → úsalo en step 4 (lenguaje del proyecto detectado); si `ticket` non-null → úsalo como `TICKET_JSON` en step 7.

### Lógica de `detectTicketFromText`

Aplica las siguientes capas **en orden**; se detiene en la primera que resuelve:

1. **Tier 1 — URL de tracker reconocido:** `ticketFromProse(text)`. Ya excluye providers `other` (el filter `if !ok || provider == state.TicketOther { continue }` ya está en la implementación). Devuelve nil si hay ambigüedad o no hay URLs reconocidas. Si non-nil: set `t.Auto = true`, return.
2. **Tier 2 — Shorthand `<provider>:<key>`:** `ticketFromShorthands(text)` (función nueva). Si non-nil: set `t.Auto = true`, return.
3. **Tiers 3+4 — Bare key con cue-word o prefijo** (gated: solo si `defaultProvider != ""`): `ticketFromContext(text, defaultProvider, keyPrefixes)`. Retorna con `Auto:true` ya seteado (comportamiento actual de esa función). Return as-is.
4. Si ningún tier resuelve: nil.

Sin tier de frontmatter (el raw idea no es un artefacto OpenSpec) y sin tier de branchKey (no aplica fuera de sync). Estos tiers existen en `detectTicket` (file-based) pero no en `detectTicketFromText`.

### Lógica de `ticketFromShorthands`

Escanea `content` con `shorthandRe` (regex package-level). Para cada match, usa `splitShorthand` (ya existente) para parsear el shorthand. Acumula candidatos; si todos los no-vacíos son idénticos en `(provider, key)` → retorna ese ticket sin Auto (el caller setea Auto). Si dos distintos → nil. Regex captura `(jira|linear|github|other):<key>` con el mismo set de providers que `splitShorthand` reconoce — para consistencia.

### Ubicación de archivos afectados

```txt
cli/cmd/vector/
  main.go          — case "detect-ticket" + usage() + runDetectTicket
  ticket.go        — detectTicketFromText + ticketFromShorthands + shorthandRe var
  ticket_test.go   — TestTicketFromShorthands + TestDetectTicketFromText

kit/commands/vector/
  raw.md           — steps 4 y 7 simplificados

cli/internal/scaffold/assets/commands/vector/
  raw.md           — copia regenerada vía go generate
```

---

## 6. Archivos a crear o modificar

La lista es exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `cli/cmd/vector/main.go` | MODIFICAR | `case "detect-ticket"` en switch + `runDetectTicket` + `usage()` actualizado | `runSync`, `runSpecCreate`, patrón `--json` |
| `cli/cmd/vector/ticket.go` | MODIFICAR | `shorthandRe` var + `ticketFromShorthands` + `detectTicketFromText` | `ticketFromProse`, `ticketFromContext`, `detectTicket` |
| `cli/cmd/vector/ticket_test.go` | MODIFICAR | `TestTicketFromShorthands` + `TestDetectTicketFromText` (table-driven) | `TestInferProvider`, `TestDetectTicket` |
| `kit/commands/vector/raw.md` | MODIFICAR | Steps 4 y 7 simplificados: llamar al binario + parsear JSON | `kit/commands/vector/propose.md`, `kit/commands/vector/sync.md` |
| `cli/internal/scaffold/assets/commands/vector/raw.md` | MODIFICAR (regenerado) | Copia vendored sincronizada vía `go generate` | sibling `propose.md`, `sync.md` en assets/ |

### Detalle por archivo

#### `cli/cmd/vector/main.go` — MODIFICAR

Cambios requeridos:

- En `main()`, al switch principal, agregar antes de `case "help"`:
  ```go
  case "detect-ticket":
      err = runDetectTicket(os.Args[2:])
  ```
- En `usage()`: agregar una línea para `detect-ticket` (tooling; texto breve: "detect a ticket and language from text (JSON output)").
- Función `runDetectTicket(args []string) error`:
  - Flags: `--repo-root string` (default: autoresolve), `--text-file string` (path al texto o "-" para stdin; default "-"), `--json bool` (no-op para compatibilidad futura — el subcomando siempre emite JSON).
  - Resolver el repo root con `resolveRepoRoot(*repoRoot)`.
  - Intentar `config.Load(root)`; si falla con "no such file", usar `&config.Config{}` vacío. Si falla con otro error (config corrupta), propagarlo como error.
  - Leer el texto de stdin o de `*textFile` (`io.ReadAll`). Error de I/O → error con contexto.
  - Llamar `detectTicketFromText(string(text), cfg.ResolvedDefaultTicketProvider(), cfg.NormalizedTicketKeyPrefixes())`.
  - Serializar y emitir JSON (struct tipado, no `map[string]interface{}`).
  - Exit 0 siempre que no haya error de I/O o config.

Restricciones: no cambiar ningún `case` existente del switch; no cambiar la descripción de subcomandos existentes en `usage()`; no tocar `main()` más allá del nuevo case.

#### `cli/cmd/vector/ticket.go` — MODIFICAR

Cambios requeridos:

- **Var regex (package-level)**:
  ```go
  // shorthandRe matches provider:key shorthands in prose. Provider set must
  // stay in sync with splitShorthand's valid-provider check.
  var shorthandRe = regexp.MustCompile(`\b(jira|linear|github|other):([A-Za-z][A-Za-z0-9/\-#]*)`)
  ```
- **`ticketFromShorthands(content string) *state.Ticket`**: sigue el mismo patrón que `ticketFromProse` — itera matches, usa `splitShorthand` para normalizar y validar, aplica lógica "un solo candidato distinto o nil". Los matches con key vacía se ignoran.
- **`detectTicketFromText(text string, defaultProvider state.TicketProvider, keyPrefixes []string) *state.Ticket`**: orquesta las tres capas descritas en §5. Setea `t.Auto = true` tras tiers 1 y 2; tier 3+4 ya retorna con `Auto:true`.

Restricciones: no modificar `detectTicket` ni ninguna de sus funciones helper (`ticketFromProse`, `ticketFromContext`, `ticketFromFrontmatter`, `pickSingleKey`, `denylistedKey`, `splitShorthand`, `inferProvider`, `extractKey`, `parseRef`, `normalizeURL`, `refHost`, `validProvider`). No reordenar las vars regex existentes — agregar `shorthandRe` al grupo existente con comentario.

#### `cli/cmd/vector/ticket_test.go` — MODIFICAR

Agregar después de los tests existentes:

- **`TestTicketFromShorthands`** (table-driven):

  | Caso | Input | wantProvider | wantKey | wantNil |
  |---|---|---|---|---|
  | un shorthand jira | `"See jira:ACME-12 for details"` | jira | ACME-12 | false |
  | shorthand linear | `"Tracked as linear:ENG-7"` | linear | ENG-7 | false |
  | duplicado idéntico | `"jira:ACME-12 and again jira:ACME-12"` | jira | ACME-12 | false |
  | duplicado distinto | `"jira:ACME-12 and jira:ACME-99"` | — | — | true |
  | providers distintos | `"jira:ACME-12 and linear:ENG-7"` | — | — | true |
  | sin shorthands | `"no tickets here"` | — | — | true |
  | provider desconocido | `"slack:ACME-1"` | — | — | true (no matchea regex) |

- **`TestDetectTicketFromText`** (table-driven; incluye campos `defaultProvider`, `keyPrefixes`):

  | Caso | Text | defaultProvider | keyPrefixes | wantProvider | wantKey | wantAuto | wantNil |
  |---|---|---|---|---|---|---|---|
  | tier 1 URL jira | `"https://acme.atlassian.net/browse/MH-12"` | "" | [] | jira | MH-12 | true | false |
  | tier 1 URL linear | `"linear.app/t/issue/ENG-7/title"` (URL completa) | "" | [] | linear | ENG-7 | true | false |
  | tier 1 URL other host → nil | `"https://example.com/tickets/42"` | "" | [] | — | — | — | true |
  | tier 1 dos URLs distintas → nil | texto con dos URLs jira distintas | "" | [] | — | — | — | true |
  | tier 2 shorthand | `"Track jira:ACME-99"` | "" | [] | jira | ACME-99 | true | false |
  | tier 2 sobre tier 3 (URL presente, tier 1 resuelve primero) | URL jira + cue "ticket: MH-1" | "jira" | [] | jira | MH-12 | true | false (tier 1 gana) |
  | tier 3 cue-word, defaultProvider set | `"Ticket: MH-5 is the tracking item"` | "jira" | [] | jira | MH-5 | true | false |
  | tier 3, defaultProvider vacío → nil | mismo texto | "" | [] | — | — | — | true |
  | tier 4 prefix key | `"Worked on MH-1592 today"` | "jira" | ["MH"] | jira | MH-1592 | true | false |
  | ADR denylist | `"Ticket: ADR-5"` | "jira" | [] | — | — | — | true |
  | RFC denylist | `"ref: RFC-12"` | "jira" | [] | — | — | — | true |
  | texto vacío → nil | `""` | "" | [] | — | — | — | true |

Restricciones: no modificar ni eliminar ningún test existente. Agregar solo; no renombrar funciones de test existentes.

#### `kit/commands/vector/raw.md` — MODIFICAR

Solo los steps 4 y 7. El resto del archivo permanece exactamente igual.

**Step 4** (actualmente: "Detect the spec language from the example / existing specs"):

Reemplazar por:

> Invoke `vector detect-ticket --repo-root $REPO_ROOT --json` piping `$RAW_IDEA` as stdin (this also runs step 7 below in one call — consolidate into a single binary invocation and store the result as `DETECT_JSON`). If `DETECT_JSON.language` is non-empty, use it as `SPEC_LANGUAGE` — done. If empty, fall back to the current logic: glob the configured `specPath` and detect language from an existing spec example; default English.

**Step 7** (actualmente: el bloque de detección con los 4 tiers en prosa):

Reemplazar por:

> **Detect a ticket** from `DETECT_JSON` (computed in step 4 — do not call the binary again):
> if `DETECT_JSON.ticket` is non-null, set `TICKET_JSON = DETECT_JSON.ticket` (already shaped as `{provider,key,url,auto}`). Otherwise leave `TICKET_JSON` unset.
>
> The binary handles all detection tiers (URL → shorthand → cue-word → prefix), ADR/RFC exclusion, ambiguity, and the `defaultTicketProvider`/`ticketKeyPrefixes` config gates. **Do not reimplement any tier in prose.**

> Token routing: the binary call runs in Go (0 tokens). The removed prose previously ran on Opus.

Restricciones: no cambiar el frontmatter (`name`, `description`, `category`, `tags`); no cambiar Hard Rules; no cambiar ningún otro step. No refactorizar estructura.

---

## 7. API Contract

Sin superficie HTTP. El contrato relevante es la interfaz CLI del subcomando, consumida por `raw.md`:

```bash
# Forma canónica (texto de la idea cruda por stdin):
echo "$RAW_IDEA" | vector detect-ticket --repo-root <path> --json

# Con archivo explícito:
vector detect-ticket --text-file /path/to/raw-idea.txt --repo-root <path> --json

# Sin texto (solo config — para leer language sin idea aún):
vector detect-ticket --repo-root <path> --json < /dev/null
```

Salida JSON (éxito — ticket detectado):
```json
{
  "ticket": {
    "provider": "jira",
    "key": "MH-123",
    "url": "https://acme.atlassian.net/browse/MH-123",
    "auto": true
  },
  "language": "es",
  "defaultTicketProvider": "jira",
  "ticketKeyPrefixes": ["MH"]
}
```

Salida JSON (éxito — sin ticket detectado):
```json
{
  "ticket": null,
  "language": "",
  "defaultTicketProvider": "",
  "ticketKeyPrefixes": []
}
```

- `ticket`: objeto `state.Ticket` o `null`. Nunca ausente (siempre presente en el JSON, aunque sea null).
- `language`: string; `""` si `config.Language` no está configurado.
- `defaultTicketProvider`: string; `""` si no configurado.
- `ticketKeyPrefixes`: array de strings; `[]` si no configurado (nunca `null`).

Exit: `0` en éxito (incluyendo "no ticket detectado"); `1` en error de I/O o config corrupta (mensaje accionable a stderr).

---

## 8. Criterios de éxito

- [ ] `vector detect-ticket` existe, parsea flags y retorna JSON válido en todos los casos.
- [ ] Tier 1 (URL): resuelve jira/linear/github correctamente; ignora `other` (hosts desconocidos); nil en ambigüedad de dos URLs distintas reconocidas.
- [ ] Tier 2 (shorthand): resuelve `jira:ACME-12`, `linear:ENG-7`, etc.; nil en dos shorthands distintos.
- [ ] Tier 3 (cue-word bare key): resuelve con `defaultTicketProvider` set; nil cuando `defaultTicketProvider` vacío.
- [ ] Tier 4 (prefix key): resuelve con `ticketKeyPrefixes` configurados.
- [ ] ADR/RFC en cualquier tier → nil.
- [ ] `language` en el JSON refleja `config.ResolvedLanguage()` (o "" si no configurado).
- [ ] `.vector/config.json` ausente → respuesta JSON con todos los campos vacíos/null, exit 0.
- [ ] `detectTicket` (file-based) y todos sus tests existentes intactos y verdes.
- [ ] `raw.md` step 7 no contiene los tiers como prosa; llama al binario y usa el JSON.
- [ ] `raw.md` step 4 usa `language` del JSON cuando non-empty; cae al fallback cuando vacío.
- [ ] Copia vendored en `assets/commands/vector/raw.md` es idéntica a `kit/commands/vector/raw.md` post-cambio.

### Tests requeridos

- [ ] `TestTicketFromShorthands` — tabla cubriendo: un shorthand, duplicado idéntico, duplicado distinto, providers distintos, sin shorthands, provider desconocido.
- [ ] `TestDetectTicketFromText` — tabla cubriendo: cada tier, orden de precedencia, ADR/RFC denylist, ambigüedad, texto vacío, defaultProvider vacío.
- [ ] Tests existentes en `ticket_test.go` pasan sin modificación.

### Comandos de verificación

```bash
gofmt -l cli && go -C cli vet ./... && go -C cli test ./...
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

Aplica al subcomando de tooling (no a UI web ni al output del command `raw.md`):

- **Solo JSON en stdout:** `vector detect-ticket` es un subcomando de tooling puro. No emite output humano. Siempre serializa la respuesta como JSON, independientemente del flag `--json`.
- **Silencio en éxito sin ticket:** exit 0 + `"ticket": null` cuando no se detecta nada (incluye texto vacío o sin referencias a tracker). No es un error.
- **Error solo en fallo de I/O o config corrupta:** stderr con contexto; exit 1. La ausencia de config es legítima (usuario no ha corrido `vector init` todavía).
- **Sin prompts interactivos:** subcomando de tooling; nunca solicita input adicional.
- **Consolidación en una sola llamada:** `raw.md` consolida el detect de lenguaje (step 4) y el de ticket (step 7) en la misma invocación del binario — un subproceso, una carga de config.
- **Errores accionables:** "read stdin: unexpected EOF" en lugar de mensajes genéricos.

---

## 10. Decisiones tomadas

- **Un único subcomando para ticket + language:** devueltos en el mismo JSON para que `raw.md` haga una sola invocación del binario. Separar en dos subcomandos (`detect-ticket`, `detect-language`) duplicaría la carga de config sin beneficio real.
- **Subcomando de primer nivel (`vector detect-ticket`), no sub-subcomando (`vector spec detect-ticket`):** la detección opera sobre texto libre, no sobre un spec existente del board. Es una utilidad de bajo nivel que `raw.md` necesita antes de que el spec exista. Va al mismo nivel que `sync`, `init`, `serve`.
- **`detectTicketFromText` no reutiliza `detectTicket` (file-based):** el file-based itera artefactos en disco (3 archivos); el text-based opera sobre un string. Son análogos con interfaces distintas; refactorizar `detectTicket` para unificarlos rompería su interfaz sin ganancia.
- **Tier 2 como función separada (`ticketFromShorthands`):** el shorthand scanning en texto libre no existía como función Go. Se agrega siguiendo el mismo patrón que `ticketFromProse` — misma firma, misma convención de nil para "no encontrado/ambiguo".
- **Sin tier de frontmatter en `detectTicketFromText`:** el raw idea no es un artefacto OpenSpec con YAML frontmatter. Incluirlo aportaría falsos positivos sin utilidad.
- **Sin tier de branchKey en `detectTicketFromText`:** el branchKey deriva del nombre del worktree folder; es un fallback específico de sync, sin sentido sobre texto libre.
- **`config.Language` como única fuente de lenguaje expuesta:** el binario no "detecta" el lenguaje del texto — solo expone lo configurado. La detección por ejemplo (globbing de specs existentes) queda en `raw.md` como fallback cuando `language == ""`.
- **Config ausente = defaults vacíos (no error):** `raw.md` puede correr antes de `vector init` en repos nuevos; el subcomando debe ser útil incluso sin config (tiers 1 y 2 no requieren config).

Si el agente detecta una alternativa mejor, la reporta como observación; no la implementa.

---

## 11. Edge cases

La implementación debe manejar explícitamente:

### Texto de entrada

- **Texto vacío** (stdin vacío o sin datos): `detectTicketFromText("", ...)` → nil; respuesta JSON con `"ticket": null`, exit 0.
- **Solo whitespace:** igual que vacío.
- **Texto con múltiples menciones del mismo ticket:** si todas resuelven al mismo `(provider, key)`, retorna ese ticket (no ambigüedad). Verificar que `ticketFromProse` y `ticketFromShorthands` manejan duplicados idénticos correctamente.
- **Texto muy largo:** sin límite artificial de tamaño — las regexes operan sobre el string completo.

### Config

- **`.vector/config.json` ausente:** `config.Load(root)` falla con error de tipo `*os.PathError` (file not found). El subcomando detecta este caso y usa `&config.Config{}` vacío — **no propaga como error**. Respuesta: `language:"", defaultTicketProvider:"", ticketKeyPrefixes:[]`.
- **Config con `defaultTicketProvider` inválido:** `config.Load` retorna error (ya valida el campo). Este sí se propaga → error a stderr, exit 1. Mensaje: el de `config.Load`.
- **`ticketKeyPrefixes` vacío:** tier 4 es inoperativo (`knownPrefixRe(nil)` retorna nil → `ticketFromContext` no matchea prefijos). Solo tiers 1-3 aplican.

### Detección de tickets

- **URL de `other` host:** `ticketFromProse` la filtra (`if !ok || provider == state.TicketOther { continue }`). No cuenta como candidato. Una URL `other` + una URL jira → jira resuelve (solo hay un candidato reconocido).
- **Dos URLs jira distintas en el texto:** `ticketFromProse` → nil (ambigüedad). Correcto — conservative by design.
- **Shorthand `other:TOOL-123` explícito:** `ticketFromShorthands` sí lo matchea (shorthand explícito del usuario ≠ inferencia de host). Ver Open questions para la decisión final.
- **ADR-1, RFC-5 en cue-word:** `denylistedKey` en `ticketFromContext` → skip.
- **Key malformada sin número** (`ENG-` sin dígitos): `bareKeyRe` no matchea (requiere `\d+`); `shorthandRe` tampoco si el key segment está vacío.
- **ADR/RFC en shorthand tier:** `ticketFromShorthands` usa `splitShorthand` → `validProvider` valida el provider; el key no pasa por `denylistedKey`. TBD — ver Open questions si el denylist aplica al shorthand tier.

### I/O

- **stdin no disponible (no-TTY sin pipe):** stdin retorna EOF inmediatamente → texto vacío → ticket null, exit 0. No error.
- **`--text-file` apunta a archivo inexistente:** `os.ReadFile` falla → error a stderr, exit 1.
- **`--repo-root` inválido o no existente:** `resolveRepoRoot` falla → error a stderr, exit 1.
- **Repo root existe pero sin `.vector/config.json`:** `config.Load` falla con not-found → Config{} vacío, continúa.

### Sin HTTP

Los códigos HTTP (400/401/403/404/409/422/429/500) no aplican: este es un subcomando CLI local.

---

## 12. Estados de salida del subcomando

| Estado | Qué devuelve | Acción del usuario |
|---|---|---|
| éxito con ticket detectado | JSON `"ticket":{…}`, `language`, etc. | raw.md usa `TICKET_JSON` |
| éxito sin ticket | JSON `"ticket":null`, campos de config | raw.md deja `TICKET_JSON` unset |
| config ausente | JSON con defaults vacíos, exit 0 | raw.md puede continuar sin ticket/lenguaje configurado |
| error de I/O (stdin/archivo) | mensaje stderr | usuario corrige path o pipe |
| config corrupta (provider inválido) | mensaje stderr | usuario corrige `.vector/config.json` |
| repo root inválido | mensaje stderr | usuario corrige `--repo-root` |

---

## 13. Validaciones

### Input del subcomando

| Campo | Regla | Comportamiento si falla |
|---|---|---|
| `--text-file <path>` (si != "-") | archivo debe existir y ser legible | error a stderr, exit 1 |
| `--repo-root <path>` | si provisto, debe ser un directorio existente (delegado a `resolveRepoRoot`) | error a stderr, exit 1 |
| stdin | no se valida contenido — texto libre | texto vacío = sin ticket (exit 0) |

### `detectTicketFromText` / `ticketFromShorthands`

No devuelven errores — son funciones best-effort. nil ante cualquier ambigüedad o ausencia. La validación estructural ocurre implícitamente en `splitShorthand` (valid provider), `denylistedKey`, `pickSingleKey` y las regexes — todas ya existen y no se modifican.

### Config

La validación de `config.Load` (provider inválido) se propaga como error del subcomando. La ausencia del archivo se convierte en defaults vacíos (no error del subcomando).

---

## 14. Seguridad y permisos

- No exponer secrets ni tokens en stdout. El JSON de respuesta contiene solo: ticket detectado (provider, key, URL pública del tracker), language (string BCP-47 o nombre libre), prefijos de proyecto. Ningún campo es sensible.
- `detectTicketFromText` no escribe al filesystem ni al state: es read-only puro sobre el texto de entrada y la config.
- `.vector/config.json` no contiene secrets (solo convenciones de repo: spec paths, lenguaje, prefijos).
- No loggear el contenido del texto de entrada en ningún output (puede contener código del usuario, ideas de features propietarias, etc.).
- No imprimir la config completa en stdout — solo los campos relevantes para el consumer (`language`, `defaultTicketProvider`, `ticketKeyPrefixes`).

---

## 15. Observabilidad y logging

- **Ningún evento nuevo** en `activity.jsonl`: `vector detect-ticket` es un subcomando de consulta auxiliar, previo a la creación del spec. El evento `spec.created` (ya existente, escrito por `vector spec create`) captura el ticket final una vez que el spec se registra.
- Errores del subcomando: stderr únicamente; no se escriben logs adicionales.
- **Token routing:** la remoción de la prosa de los 4 tiers de `raw.md` elimina el consumo en Opus para parsing determinista. El step 7 actualizado puede registrar esto con `vector spec route` (ya existente en `raw.md` step 10) como una economía: el detect corrió en Go (0 tokens) vs el baseline Opus.

---

## 16. i18n / textos visibles

**El proyecto no tiene sistema de i18n.** El subcomando emite solo JSON (sin strings localizables) y mensajes de error en inglés hardcodeado — consistente con el resto del binario.

| Identificador (doc) | Texto (hardcoded EN) |
|---|---|
| detect_ticket.err_read_file | `"read text file %q: %w"` |
| detect_ticket.err_read_stdin | `"read stdin: %w"` |
| detect_ticket.usage | `"usage: vector detect-ticket [--repo-root path] [--text-file -|path] [--json]"` |

El command `raw.md` sigue conversando en el idioma del usuario. El `language` del JSON es un dato, no un texto visible.

---

## 17. Performance

- Carga de config (`config.Load`): lectura de un archivo JSON de ~200-500 bytes; < 1ms.
- `detectTicketFromText` con texto típico de una idea (100–500 palabras): regex sobre string en memoria; < 1ms.
- `shorthandRe`: compilada a nivel de paquete (`var`), no por llamada.
- Sin I/O de red. Sin llamadas a modelos. Sin goroutines.
- **El beneficio neto de performance** es el del caller (`raw.md`): elimina la ejecución de la lógica de los 4 tiers en Opus, que consumía cientos de input tokens + latencia de modelo, por una invocación de subproceso Go de ~5ms end-to-end.
- `ticketFromShorthands` itera todos los matches de `shorthandRe` — complejidad lineal en el número de matches; no hay riesgo de regresión para textos normales.

---

## 18. Restricciones

El agente no debe:

- Modificar `detectTicket` (file-based, used by sync) ni ninguna de sus funciones helper.
- Modificar `ticketFromProse`, `ticketFromContext`, `ticketFromFrontmatter`, `pickSingleKey`, `denylistedKey`, `splitShorthand`, `inferProvider`, `extractKey`, `parseRef`, `normalizeURL`, `refHost`, `validProvider`.
- Instalar dependencias externas (solo stdlib).
- Cambiar el schema de `state.Ticket`, `state.TicketProvider`, `config.Config`.
- Cambiar steps 1, 2, 3, 5, 6, 8, 9, 10 ni 11 de `raw.md`; no cambiar Hard Rules; no cambiar el frontmatter.
- Editar directamente `cli/internal/scaffold/assets/commands/vector/raw.md` — debe sincronizarse vía `go generate`.
- Crear endpoints HTTP para este subcomando.
- Agregar tiers de detección no contemplados en el brief ni cambiar el orden de los existentes.
- Refactorizar partes de `main.go` o `ticket.go` no relacionadas con los cambios listados.
- Usar `interface{}`/`any` en el struct de respuesta JSON.
- Cambiar el string de `usage` de ningún subcomando existente — solo agregar la línea de `detect-ticket`.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `vector detect-ticket` implementado y operativo.
- [ ] `detectTicketFromText` + `ticketFromShorthands` + `shorthandRe` en `ticket.go`.
- [ ] `runDetectTicket` en `main.go`; `case "detect-ticket"` en el switch; `usage()` actualizado.
- [ ] `TestTicketFromShorthands` + `TestDetectTicketFromText` en `ticket_test.go`.
- [ ] `kit/commands/vector/raw.md`: steps 4 y 7 actualizados; resto del archivo intacto.
- [ ] `cli/internal/scaffold/assets/commands/vector/raw.md`: sincronizado (vía `go generate` o con instrucción explícita de qué correr si el agente no puede ejecutar el generate).
- [ ] Sin regresiones: `detectTicket` file-based y todos sus tests existentes intactos.
- [ ] `gofmt -l cli && go -C cli vet ./... && go -C cli test ./...` verdes.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Revisé `cli/cmd/vector/ticket.go` para entender la firma exacta y el comportamiento de `ticketFromProse`, `ticketFromContext`, `splitShorthand`, `denylistedKey`, `pickSingleKey` antes de usarlos.
- [ ] Revisé `cli/cmd/vector/main.go` (switch principal en `main()`, `usage()`, patrón de `runSync`/`runSpecCreate` para flags y JSON output).
- [ ] Revisé `cli/cmd/vector/ticket_test.go` para no romper tests existentes.
- [ ] Revisé `cli/internal/config/config.go` para usar los métodos correctos (`ResolvedLanguage()`, `ResolvedDefaultTicketProvider()`, `NormalizedTicketKeyPrefixes()`).
- [ ] Revisé `kit/commands/vector/raw.md` steps 4 y 7 para entender exactamente qué reemplazar.
- [ ] Solo modifiqué los archivos listados en §6, o justifiqué cualquier excepción.
- [ ] No modifiqué `detectTicket` ni ninguno de sus helpers.
- [ ] `detectTicketFromText` aplica los 3 tiers en el orden correcto y setea `Auto:true` en tiers 1 y 2.
- [ ] `ticketFromShorthands` usa `shorthandRe` (var a nivel de paquete, no compilada en la función).
- [ ] El struct de respuesta de `runDetectTicket` tipado explícitamente (sin `any`/`interface{}`).
- [ ] La ausencia de `.vector/config.json` produce Config{} vacío — no error del subcomando.
- [ ] Tests cubren los casos de la tabla de §11 (edge cases de ambigüedad, ADR/RFC, texto vacío, defaultProvider vacío).
- [ ] `raw.md` steps 4 y 7 actualizados; el resto del archivo intacto.
- [ ] La copia vendored `assets/commands/vector/raw.md` fue sincronizada.
- [ ] Ejecuté `gofmt -l cli && go -C cli vet ./... && go -C cli test ./...` y están verdes.
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar.

## Open questions

- **Shorthand `other:KEY` en tier 2:** raw.md §7 dice "any other host → skip, never seed `other`" para el tier de URL, pero esa exclusión aplica a inferencia de host, no a shorthands explícitos. ¿Incluye `other` en la regex de `ticketFromShorthands`? — TBD al implementar; decisión recomendada: incluirlo (el shorthand `other:TOOL-1` es una declaración explícita del usuario, semánticamente diferente a un host no reconocido).
- **Denylist ADR/RFC en tier 2 (shorthand):** `ticketFromShorthands` usa `splitShorthand` para validar el provider, pero `denylistedKey` no se aplica al key del shorthand en la implementación actual. ¿Debería `ticketFromShorthands` aplicar el denylist? El caso práctico es esquina (nadie pondría `jira:ADR-5`); decisión recomendada: no aplicar, por consistencia con que el denylist existe solo para bare keys sin contexto de provider.
- **Consolidación de steps 4 y 7 en `raw.md`:** el spec indica una sola invocación del binario que devuelve tanto `language` como `ticket`. El agente debe verificar que combinar los steps en una sola llamada es viable en la instrucción de `raw.md` (la idea raw está disponible desde el step 1). Si por alguna razón el step 4 se ejecuta antes de tener la idea completa, puede necesitarse dos llamadas — TBD al implementar.
