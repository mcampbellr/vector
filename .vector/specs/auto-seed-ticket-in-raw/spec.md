# Spec: Auto-sembrar el ticket detectado en /vector:raw

## 1. Objetivo

Construir el cableado para que **`/vector:raw` siembre el link de ticket en el momento de crear
la card**, en vez de solo "notarlo" y delegar a un `/vector:link` manual posterior.

Esta feature permite que un **dev** que pega un raw idea con una referencia de ticket (una URL
de Jira/Linear/GitHub, un shorthand `jira:MH-1653`, o una clave anclada a un cue-word como
`ticket: MH-1653`) obtenga la card **ya linkeada** (`ticket` en `state.json`, `auto:true`) sin un
segundo comando.

El problema concreto: el template del command `/vector:raw` (`kit/commands/vector/raw.md`)
detecta la referencia (paso 7: *"Detect a ticket reference … note it for /vector:link"*) pero la
llamada a `vector spec create` del paso 9 **omite el flag `--ticket`** que el binario ya soporta,
y el paso 11 dice *"say it can be linked with /vector:link"*. El soporte del binario ya existe:
`parseTicketFlag` (`cli/cmd/vector/ticket.go:296`) está documentado como *"decodes the --ticket
JSON ({provider,key,url,auto}) passed by /vector:raw when it detects a ticket in the raw idea
text"*. La brecha es puramente la orquestación en el markdown.

## 2. Alcance

### Incluido en esta fase

- **Detección de ticket en el `RAW_IDEA`** dentro del command `/vector:raw`. Los tiers de URL,
  cue-word y prefijo replican la semántica de `vector sync` (`ticketFromProse`/`ticketFromContext`
  en `cli/cmd/vector/ticket.go`); el tier de **shorthand-en-prosa es una extensión nueva** del
  command (ver §5 — `detectTicket` solo resuelve shorthand en frontmatter vía `parseRef`, no en
  prosa libre). Precedencia por confianza:
  1. **URL** de tracker reconocido (host → provider vía `inferProvider`; jira/linear/github;
     host desconocido = no se siembra). Conflicto entre URLs de tickets distintos → descartar.
     Replica `ticketFromProse`.
  2. **Shorthand `<provider>:<key>`** (p. ej. `jira:MH-1653`) — **lógica nueva del command**
     (escaneo explícito con la semántica de `splitShorthand`); `detectTicket` no escanea prosa
     libre por shorthands.
  3. **Clave anclada a cue-word** (`ticket|issue|ref|tracking|jira|linear|github :` al inicio de
     línea, regex `ticketCueRe`) → bare key + provider, **solo si `defaultTicketProvider` está
     configurado** en `.vector/config.json` (gated, igual que sync).
  4. **Clave con prefijo configurado** (`ticketKeyPrefixes`) en cualquier parte de la prosa,
     **solo si `defaultTicketProvider` está configurado**.
  - Empate del mismo tier (dos keys distintas) → **descartar** (no auto-sembrar); las keys cuyo
    prefijo está en el denylist (`ADR`/`RFC`) se omiten.
- **Sembrado en create time**: cuando la detección es confiable, el command construye
  `{"provider":"<p>","key":"<KEY>","url":"<url-si-aplica>","auto":true}` y lo pasa como
  `--ticket` a `vector spec create` (paso 9). `auto:true` marca la procedencia (distinto de un
  `/vector:link` explícito).
- **Reporte actualizado** (paso 11): `linked <KEY> (<provider>)` cuando se auto-sembró; el hint
  de `/vector:link` solo se muestra en el caso ambiguo/no detectado.
- **No bloquear la creación por el link**: si el JSON resultara inválido o el provider no se
  puede inferir, el command **cae a crear sin `--ticket`** y muestra el hint actual de
  `/vector:link` (comportamiento de hoy).
- **Regeneración del asset embebido** `cli/internal/scaffold/assets/commands/vector/raw.md` vía
  `go generate` (re-sembrado por `vector update` / `vector init --force`).

### Fuera de scope

- **Cualquier cambio en Go**: el binario ya soporta `--ticket` (`parseTicketFlag`,
  `CreateSpecParams.Ticket`, `runSpecCreate`). No se toca `ticket.go`, `store.go` ni el struct
  `Ticket`/`TicketProvider`.
- **Cambiar `/vector:sync`** ni su `detectTicket` (ya auto-linkea; este change es solo `raw`).
- **Pedir `--provider` al usuario** en claves ambiguas: eso queda para `/vector:link` (el command
  `raw` solo auto-detecta, no interroga).
- **Validar el ticket contra el tracker externo** (sin llamadas de API).
- **Múltiples tickets por spec** (uno por spec en V1, como el modelo de estado actual).
- **Nuevos cue-words o patrones** fuera de los que ya usa `ticket.go`.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- `/vector:raw` es un **project command markdown** (`kit/commands/vector/raw.md`), no una skill.
  El único artefacto que cambia sustancialmente es ese markdown (+ su copia embebida).
- Go (módulo único en `cli/`, stdlib): el lado binario ya está implementado y testeado.
- Embed/scaffold: `cli/internal/scaffold` embebe `kit/{commands,agents,vector}` vía `embed.FS`;
  la copia en `cli/internal/scaffold/assets/commands/vector/raw.md` se regenera con `go generate`
  (directiva en `cli/internal/scaffold/scaffold.go`).

### Versiones relevantes

- Go: **1.26** (`cli/go.mod`). No se compila código nuevo; solo se ejerce el flag existente.
- Soporte de ticket ya presente: `state.Ticket{Provider,Key,URL,Auto}`
  (`cli/internal/state/types.go:113`), `TicketProvider` jira|linear|github|other (`types.go:60`),
  `--ticket` en `runSpecCreate` (`cli/cmd/vector/main.go:685`), `parseTicketFlag`
  (`cli/cmd/vector/ticket.go:296`).

### Patrones existentes a respetar

- **Reglas de detección = las de `vector sync`** para URL por host (`inferProvider`/
  `ticketFromProse`), cue-word (`ticketCueRe`), prefijo (`knownPrefixRe`), descarte por conflicto
  (`pickSingleKey`), denylist ADR/RFC (`denylistedKey`). El **shorthand en prosa** (`splitShorthand`)
  es lógica nueva del command (sync solo lo resuelve en frontmatter; ver §5). El command replica
  la **semántica** (no llama a las funciones; el flujo es markdown).
- **`ticketFromContext` ya marca `Auto:true`** en el `*Ticket` que devuelve; el command, al
  construir el JSON, fija `"auto":true` igualmente — no es doble-set, es el mismo invariante.
- **Bare-key gated por `defaultTicketProvider`**: igual que sync, una clave sin URL/shorthand
  solo se siembra si el repo configuró `defaultTicketProvider` (opt-in). Sin él, no se adivina.
- **CLI-owns-writes**: el binario persiste el ticket; el command solo construye el JSON y lo pasa.
- **Markdown del command en inglés** (convención del repo para los `/vector:*`).
- **No editar la copia de assets a mano**: se regenera.

---

## 4. Dependencias previas

Antes de iniciar esta fase ya existe (verificado):

- [x] `vector spec create --ticket '{provider,key,url,auto}'` (`main.go:685`) y `parseTicketFlag`
      (`ticket.go:296`), documentado como el canal de `/vector:raw`.
- [x] `CreateSpecParams.Ticket` y persistencia del `Ticket` en `state.json` (`store.go`,
      `types.go:97/113`), con evento `spec.linked` al crear con ticket.
- [x] Helpers de detección reutilizables como referencia de semántica: `parseRef`,
      `inferProvider`, `ticketFromProse`, `ticketFromContext`, `pickSingleKey`, `denylistedKey`
      (`ticket.go`).
- [x] `config.DefaultTicketProvider` + `config.TicketKeyPrefixes` (`config.go:75–79`).
- [x] Command `/vector:raw` con pasos de detección (7), create (9) y reporte (11)
      (`kit/commands/vector/raw.md`).
- [x] `cli/internal/scaffold` con `go generate` que vendoriza la copia del command.

**No hay dependencias de estructura faltantes.** El change es orquestación en el markdown.

> Nota de fidelidad: el `.vector/config.json` de **este** repo no tiene `defaultTicketProvider`
> set, así que aquí una bare-key con cue-word (p. ej. `ticket: MH-1653`) **no** se auto-siembra
> hasta que se configure ese provider — consistente con el gate de sync. URL y shorthand sí se
> siembran sin configuración previa.

---

## 5. Arquitectura

### Patrón a usar

**Orquestación en el command markdown + binario como escritor.** El command lee el `RAW_IDEA`,
detecta la referencia, construye el JSON del ticket y lo pasa por `--ticket` a
`vector spec create`. El binario valida (`parseTicketFlag`), persiste el `Ticket` y emite
`spec.linked`. No hay lógica nueva en Go.

**Paridad con sync — y dónde NO la hay**: los tiers de URL, cue-word y prefijo replican la
semántica de `detectTicket` (`ticketFromProse` para URLs; `ticketFromContext` para cue-word y
prefijo, gated por `defaultProvider`). El tier de **shorthand `<provider>:<key>` en prosa libre
es una capacidad nueva** del command: `detectTicket` solo resuelve shorthand en el **frontmatter**
de los artefactos (`ticketFromFrontmatter` → `parseRef` → `splitShorthand`), nunca escaneando
prosa libre — `ticketFromProse` solo matchea URLs (`ticketURLRe`). El command debe escanear el
`RAW_IDEA` por shorthands con la semántica de `splitShorthand` (no es replicación de un helper de
prosa existente). Como el flujo no es Go, "reusar la semántica" significa replicar las reglas, no
llamar a las funciones.

### Capas afectadas

- presentation/command (`kit/commands/vector/raw.md`): **sí** — detección, paso de `--ticket`,
  reporte.
- Go/CLI (`cli/cmd/vector`): **no** — el soporte ya existe; se ejerce, no se modifica.
- estado (`cli/internal/state`): **no** — `Ticket` ya se persiste.
- web/board: **no** — la card linkeada ya trae `ticket` poblado en su JSON; sin cambios de UI.

### Flujo esperado

1. `/vector:raw` recibe el `RAW_IDEA`.
2. Tras refinar/componer (pasos 1–7), el command **detecta** una referencia de ticket por
   precedencia: URL → shorthand → cue-word(+default) → prefijo(+default).
3. Si la detección es confiable, resuelve provider/key/url y construye
   `TICKET_JSON = {"provider":…,"key":…,"url":…,"auto":true}` (`url` puede ir vacío).
4. Si es ambigua (bare key sin gate), conflictiva (empate de tier) o vacía → `TICKET_JSON` queda
   sin definir (no se pasa `--ticket`).
5. Valida el spec (paso 8).
6. `vector spec create … [--ticket "$TICKET_JSON"] … --status draft --body-file -`.
7. El binario persiste la card; si vino `--ticket`, persiste el `Ticket` y emite `spec.linked`.
8. Reporte (paso 11): `linked <KEY> (<provider>)` si se sembró; si no, el hint de `/vector:link`.

### Ubicación de archivos nuevos

No se crean paquetes ni archivos nuevos. Solo se modifica el markdown del command y se
**regenera** la copia embebida del scaffold.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/commands/vector/raw.md` | MODIFICAR | Paso 7: sub-paso de detección de ticket (reglas de sync). Paso 9: pasar `--ticket "$TICKET_JSON"` cuando se detectó. Paso 11: reporte `linked <KEY> (<provider>)` / hint solo en ambigüedad. | Reglas en `cli/cmd/vector/ticket.go` (`detectTicket`); estructura de pasos del propio `raw.md` |
| `cli/internal/scaffold/assets/commands/vector/raw.md` | REGENERAR | Copia embebida del command; se regenera con `go generate` (no editar a mano). | Directiva `//go:generate` en `cli/internal/scaffold/scaffold.go` |

### Detalle por archivo

#### kit/commands/vector/raw.md

Acción: MODIFICAR

**Paso 7 (Compose) — añadir sub-paso "Detect a ticket"** que reemplace la línea actual
*"Detect a ticket reference … note it for /vector:link"* por una detección con la **misma
semántica que `vector sync`** (referencia: `cli/cmd/vector/ticket.go`), en este orden de
precedencia (mayor confianza primero; el primero que resuelve gana):

1. **URL** en el `RAW_IDEA` (`https?://…`): inferir provider por host
   (`atlassian.net`/`jira`→jira, `linear.app`→linear, `github.com`→github; otro host → **no**
   se siembra). Extraer la key (GitHub `owner/repo#N`; Jira/Linear `KEY-123`). Si hay **dos URLs
   de tickets distintos** → descartar (ambiguo).
2. **Shorthand `<provider>:<key>`** con provider conocido (jira|linear|github|other).
3. **Clave anclada a cue-word**: línea que empieza con `ticket|issue|ref|tracking|jira|linear|
   github :` seguida de una key tipo `KEY-123`. El `ticketCueRe` real (`ticket.go:213`) además
   **tolera** espacios iniciales, blockquote `>` y negrita `**` alrededor del cue — referirse a
   la regex directamente. Resolver provider con `defaultTicketProvider` del config — **solo si
   está configurado**; si no, no se siembra. Empate de keys → descartar.
4. **Clave con prefijo configurado** (`ticketKeyPrefixes`) en la prosa, con
   `defaultTicketProvider` configurado. Empate → descartar.
- Saltar keys con prefijo en denylist (`ADR`, `RFC`).
- Si nada resuelve confiablemente → no se define `TICKET_JSON`.

Construir `TICKET_JSON = {"provider":"<p>","key":"<KEY>","url":"<url|>","auto":true}` (el `url`
puede ir vacío para una clave sin URL canónica). Guardarlo para el paso 9.

**Paso 9 (Register)** — añadir el flag opcional al `vector spec create`:

```bash
vector spec create \
  --title "<title>" \
  --id "<slug>" \
  [--repo "<repo-name>"] \
  [--priority "<priority>"] \
  [--ticket "$TICKET_JSON"] \
  --status draft \
  --body-file - --json <<'SPEC'
<the full 20-section spec markdown>
SPEC
```

Si el binario rechaza el `--ticket` (JSON inválido / provider desconocido), **reintentar la
creación sin `--ticket`** y seguir el camino del hint (no abortar la creación por el link).

**Paso 11 (Report)** — sustituir la línea de ticket:
- Auto-sembrado: `linked <KEY> (<provider>)`.
- Detectado pero ambiguo/sin gate: `ticket detected but ambiguous — link it with /vector:link`.
- Sin referencia: no mencionar ticket (sin hint gratuito).

Restricciones:
- No inventar cue-words ni patrones fuera de los de `ticket.go`.
- No pedir `--provider` interactivo (eso es de `/vector:link`).
- No tocar el frontmatter del spec ni el resto del flujo (refine, validate, route, report).

#### cli/internal/scaffold/assets/commands/vector/raw.md

Acción: REGENERAR

- Tras editar el fuente en `kit/commands/vector/raw.md`, correr `go -C cli generate ./...` y
  verificar que la copia embebida coincide. No editarla a mano (drift).

---

## 7. API Contract

No aplica — no hay endpoint HTTP nuevo ni cambio de contrato. Se ejerce un flag CLI ya existente
(`vector spec create --ticket`). La card resultante trae `ticket` poblado en su JSON del board
(ya soportado). El "contrato" relevante es el del flag, ya definido por `parseTicketFlag`:
`{"provider":jira|linear|github|other,"key":<no vacío>,"url":<opcional>,"auto":<bool>}`.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `/vector:raw` con una **URL** de Jira/Linear/GitHub en el raw idea → card creada con
      `ticket{provider inferido, key, url, auto:true}` en `state.json`, sin `/vector:link`.
- [ ] `/vector:raw` con **shorthand** `jira:MH-1653` → card con `ticket{jira, MH-1653, auto:true}`.
- [ ] `/vector:raw` con **cue-word** `ticket: MH-1653` **y `defaultTicketProvider` configurado**
      → auto-siembra con ese provider; **sin** `defaultTicketProvider` → no siembra y muestra el
      hint (comportamiento gated, consistente con sync).
- [ ] `/vector:raw` con **bare key sin contexto** → crea sin ticket (silencioso/hint).
- [ ] `/vector:raw` con **dos referencias distintas del mismo tier** → descarta (sin ticket).
- [ ] `/vector:raw` **sin referencia** → comportamiento idéntico al de hoy (sin `--ticket`).
- [ ] Reporte: `linked <KEY> (<provider>)` cuando se sembró; hint solo en ambigüedad/no detección.
- [ ] `--ticket` inválido → la card se crea igual (sin ticket) y se muestra el hint.
- [ ] La copia embebida en `scaffold/assets/commands/vector/raw.md` coincide con la fuente.

### Tests requeridos

El lado binario ya está cubierto por `ticket_test.go` (parse/infer/detect). El cambio es de
orquestación en el markdown, así que la verificación es **por ejemplos**:

- [ ] Caso URL (jira/linear/github) → JSON correcto + persistencia.
- [ ] Caso shorthand → JSON correcto.
- [ ] Caso cue-word con/ sin `defaultTicketProvider` (gate).
- [ ] Caso ambiguo/ conflicto → sin `--ticket`.
- [ ] Caso sin referencia → flujo actual intacto.
- [ ] `--ticket` malformado → fallback a creación sin ticket.

### Comandos de verificación

```bash
go -C cli generate ./...        # regenera la copia embebida del command
gofmt -l cli                    # debe salir vacío
go -C cli vet ./...
go -C cli test ./...            # ticket_test.go y scaffold tests verdes
go -C cli build ./...
```

La fase no está completa si alguno falla o si `gofmt -l` lista archivos.

---

## 9. Criterios de UX

No hay UI ni formularios (es un command de CLI). La única UX visible es el **reporte del command**
mejorado: `linked <KEY> (<provider>)` en lugar de "you can link it later with /vector:link", y el
hint solo cuando la detección es ambigua o no hubo referencia. Por subsección del template:

- **Loading**: No aplica — sin operación asíncrona ni spinner; el flujo es síncrono en CLI.
- **Formularios**: No aplica — no hay formularios; la entrada es el `RAW_IDEA` ya recibido.
- **Passwords**: No aplica — no se manejan credenciales.
- **Errores**: si `vector spec create --ticket` rechaza el JSON, el command **no** falla: reintenta
  sin `--ticket` y crea la card, mostrando el hint de `/vector:link` (degradación suave).
- **Navegación**: No aplica — sin pantallas; tras crear se reporta el id y el próximo paso
  (`/vector:propose`), como hoy.
- **Accesibilidad**: No aplica — salida de texto plano en terminal; sin componentes visuales.

---

## 10. Decisiones tomadas

El agente no debe cuestionarlas ni cambiarlas:

- **Reglas de detección = exactamente las de `vector sync`** (`detectTicket`/helpers en
  `ticket.go`): URL → shorthand → cue-word(+default) → prefijo(+default); denylist ADR/RFC.
- **Precedencia por confianza; empate del mismo tier → descartar** (no adivinar cuál es el "real").
- **Bare-key gated por `defaultTicketProvider`** (opt-in, como sync). Sin él, no se siembra.
- **`auto:true`** marca el origen auto-detectado (distinto de `/vector:link`).
- **No bloquear la creación por el link**: `--ticket` inválido → crear sin ticket + hint.
- **Sin cambios en Go ni en `/vector:sync`/`/vector:link`**: solo el markdown de `raw` + regen.
- **El command no pide `--provider` interactivo**; las claves ambiguas van a `/vector:link`.
- **Spec ids/títulos sin tocar** por la detección de ticket.

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación; no la
implementa.

---

## 11. Edge cases

### Detección

- **URL de tracker conocido** → infiere provider + extrae key + normaliza URL.
- **URL de host desconocido** (`inferProvider` → other vía host no reconocido, o sin host) → no
  se siembra (conservador, como `ticketFromProse` que ignora `other`).
- **Shorthand `provider:key`** con provider válido → se resuelve directo.
- **Cue-word + bare key** con `defaultTicketProvider` set → se siembra; sin él → hint.
- **Bare key con prefijo configurado** (`ticketKeyPrefixes`) + default set → se siembra.
- **Bare key sin cue, sin prefijo, sin default** → no se siembra (ambiguo).
- **Dos URLs / dos keys distintas del mismo tier** → descartar (conflicto).
- **Key en denylist** (`ADR-12`, `RFC-7`) → ignorar (no es ticket).

### Creación

- **`--ticket` malformado o provider no inferible** → crear la card **sin** `--ticket` y mostrar
  el hint (no abortar).
- **Card/id ya existe** → error del binario, comportamiento actual (la detección no lo cambia).

### Reporte

- **Sembrado** → `linked <KEY> (<provider>)`.
- **Detectado pero no sembrable** (ambiguo/sin gate) → hint explícito de `/vector:link`.
- **Sin referencia** → no se menciona ticket.

### Sin conexión / Timeout / Doble submit

No aplica — el cambio es markdown sin I/O de red nuevo; la creación es una invocación CLI
síncrona idempotente por id (un id repetido es error del binario, no doble escritura).

---

## 12. Estados de UI requeridos

No aplica — el cambio es de orquestación del command; el board, la card y la timeline no cambian
de estructura (la card simplemente puede traer `ticket` poblado, ya soportado).

---

## 13. Validaciones

### Validaciones de cliente (command markdown)

| Campo | Regla | Mensaje |
|---|---|---|
| referencia en `RAW_IDEA` | opcional; sin ref = sin ticket | (silencioso) |
| URL/shorthand | parseable + provider conocido | (silencioso; si falla = sin ticket) |
| bare key | requiere cue-word/prefijo **y** `defaultTicketProvider` | (silencioso → hint) |
| conflicto de tier | dos keys distintas → descartar | (silencioso → hint) |
| key | no vacía, no denylist (ADR/RFC) | (silencioso) |

### Validaciones de servidor (binario)

`parseTicketFlag` valida: JSON bien formado, provider ∈ {jira,linear,github,other}, key no vacía;
URL opcional. JSON inválido → error → el command cae a crear sin `--ticket`.

---

## 14. Seguridad y permisos

- Sin secrets ni credenciales: una referencia de ticket (key/URL) es metadata pública del
  proyecto; se persiste en `state.json` (committed), como hoy con `/vector:link`.
- Sin llamadas de red ni a APIs externas del tracker.
- No se añaden permisos: `vector spec create` ya escribe la card; el `--ticket` solo añade un
  campo. CLI-owns-writes intacto.

---

## 15. Observabilidad y logging

- El binario ya emite `spec.linked` al crear con ticket (sin cambios).
- El reporte del command informa si se sembró un ticket (`linked <KEY> (<provider>)`), visible en
  stdout. No se registra nada sensible.

---

## 16. i18n / textos visibles

- Los textos del command markdown van **en inglés** (convención de los `/vector:*`).

| Identificador | Texto |
|---|---|
| raw.ticket.linked | `linked <KEY> (<provider>)` |
| raw.ticket.ambiguous | `ticket detected but ambiguous — link it with /vector:link` |

No hay sistema de i18n de UI que tocar.

---

## 17. Performance

- Detección: matching de texto/regex sobre el `RAW_IDEA` (negligible).
- Sin I/O nuevo: el binario ya carga el config; el command pasa un flag más.
- Sin cambios en el tier de agentes del flujo `raw` (refiner Haiku, validator Sonnet).

---

## 18. Restricciones

El agente no debe:

- Modificar Go: `ticket.go`, `store.go`, `main.go`, el struct `Ticket`/`TicketProvider`.
- Cambiar `/vector:sync` ni su `detectTicket`, ni `/vector:link`.
- Inventar cue-words/patrones fuera de los de `ticket.go`.
- Auto-sembrar una bare key sin el gate de `defaultTicketProvider`.
- Adivinar entre referencias en conflicto (debe descartar).
- Bloquear la creación de la card por un fallo de link.
- Soportar múltiples tickets por spec.
- Pedir `--provider` interactivo en el flujo `raw`.
- Editar a mano la copia de assets del scaffold (debe regenerarse).
- Traducir ids/títulos por efecto de la detección.

---

## 19. Entregables

Al finalizar deben quedar:

- [ ] `kit/commands/vector/raw.md` con: detección en paso 7 (reglas de sync), `--ticket` en paso
      9, reporte en paso 11.
- [ ] Copia embebida `cli/internal/scaffold/assets/commands/vector/raw.md` regenerada y verificada.
- [ ] Ejemplos en el PR que muestren: URL, shorthand, cue-word (con/sin default), conflicto,
      sin referencia, `--ticket` inválido.
- [ ] Gate verde: `go generate`, `gofmt -l`, `go vet`, `go test`, `go build`.
- [ ] Sin regresiones en el resto del flujo `raw` ni en otros commands.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo y `kit/commands/vector/raw.md`.
- [ ] Revisé `cli/cmd/vector/ticket.go` (`detectTicket`, `parseRef`, `inferProvider`,
      `ticketFromProse`, `ticketFromContext`, `pickSingleKey`, `denylistedKey`) para replicar
      su semántica exacta.
- [ ] Confirmé que `vector spec create --ticket` (`main.go:685` / `parseTicketFlag`) es el canal
      y que NO cambio Go.
- [ ] Repliqué la precedencia URL → shorthand → cue-word(+default) → prefijo(+default), con
      descarte de empates y denylist ADR/RFC.
- [ ] Respeté el gate de `defaultTicketProvider` para bare keys (este repo no lo tiene set).
- [ ] El reporte usa `linked <KEY> (<provider>)` y el hint solo en ambigüedad/no detección.
- [ ] `--ticket` inválido → la card se crea sin ticket (no abortar).
- [ ] Regeneré y verifiqué la copia de assets del scaffold.
- [ ] No toqué `/vector:sync`, `/vector:link` ni el código Go.
- [ ] Ejecuté `go generate`, `gofmt`, `go vet`, `go test`, `go build` — verdes.
- [ ] No dejé `[...]` ni TODOs sin justificar.

---

## Open questions

1. **Acceptance del raw idea vs gate de bare-key**: el ejemplo `ticket: MH-1653 → {jira, auto:true}`
   del raw idea solo se cumple si el repo tiene `defaultTicketProvider: jira` (o `MH` en
   `ticketKeyPrefixes`). Este repo no lo tiene. Decisión tomada: **respetar el gate de sync** (no
   sembrar bare keys sin default). ¿Se quiere, aparte, **configurar `defaultTicketProvider: jira`**
   en `.vector/config.json` de este repo para que el ejemplo se cumpla aquí? (Cambio de config,
   no de este spec.)
2. **Formato de cue-word en prosa libre**: `ticketCueRe` ancla el cue al **inicio de línea**. Un
   raw idea con `… see ticket: MH-1653 …` en medio de un párrafo **no** matchea. ¿Suficiente, o
   se quiere un escaneo más laxo (riesgo de falsos positivos)? Recomendación: mantener anclado
   al inicio de línea (paridad con sync).
