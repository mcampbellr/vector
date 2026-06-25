# Spec: Auto-detección de tickets por key con defaults configurables (`vector sync`/`/vector:raw`)

## 1. Objetivo

Construir una **extensión de la auto-detección de tickets** (feature hermana `add-ticket-linking`)
para reconocer **keys sueltas sin URL** (p. ej. `MH-1592`) cuando aparecen con contexto suficiente,
usando dos señales deterministas y un **provider por defecto configurable**. Hoy `detectTicket`
(`cli/cmd/vector/ticket.go`) descarta toda key sin URL porque no puede inferir el provider; en UAT
contra el repo somnio se vio que ese repo escribe los tickets como `Ticket: MH-1592.` /
`**Ticket:** MH-1552` / `> Ticket: MH-1611 · Epic MH-1528`, sin URL ni frontmatter — así que
`vector sync` no linkea ninguno.

Esta feature permite que un dev en un repo que usa **un solo tracker** (p. ej. Jira) configure el
provider una vez en `.vector/config.json` y obtenga **auto-link** de esos tickets durante
`vector sync` y `/vector:raw`, y un **link manual ergonómico** (`vector spec link <id> MH-1592` sin
`--provider`), sin imponer convenciones ni hacer hits al tracker externo.

## 2. Alcance

### Incluido en esta fase

- **Config**: dos campos nuevos opcionales en `.vector/config.json`:
  - `defaultTicketProvider` (`jira`|`linear`|`github`|`other`): provider de fallback para keys
    ambiguas detectadas en sync/raw y para el link manual de key suelta.
  - `ticketKeyPrefixes` (`[]string`, p. ej. `["MH"]`): prefijos de proyecto que identifican una key
    como ticket en cualquier parte de la prosa, con alta confianza.
- **Detección "más inteligente" (determinista) en `detectTicket`**, como **fallback DESPUÉS** del
  frontmatter `ticket:` y del escaneo de URLs (orden de precedencia intacto). Se activa **solo si**
  hay `defaultTicketProvider` configurado, y reconoce una key por **cualquiera** de:
  1. **Cue word anclado** al inicio de línea (tolerando `>` blockquote y `**bold**`):
     `Ticket:`, `Issue:`, `Ref:`, `Tracking:`, o el nombre de un provider (`Jira:`/`Linear:`/`GitHub:`),
     seguido de la primera key con formato `[A-Za-z][A-Za-z0-9]*-\d+`.
  2. **Prefijo de proyecto conocido**: una key `^<PREFIX>-\d+` (de `ticketKeyPrefixes`) en cualquier
     parte de la prosa.
- **Denylist** de prefijos que **nunca** son tickets: `ADR`, `RFC` (built-in). Una key con esos
  prefijos se ignora aunque caiga bajo un cue word.
- **Link manual de key suelta** (`vector spec link <id> <key>` sin `--provider`): `parseRef` consulta
  `defaultTicketProvider` para resolver el provider en vez de fallar por ambigüedad.
- **Validación**: `defaultTicketProvider` inválido → **error en `config.Load`** (mensaje accionable).
- **Threading**: `runSync` y `runSpecLink` (y, si aplica, `runSpecCreate`) pasan la config resuelta a
  `detectTicket`/`parseRef`.
- **Tests Go** y actualización de `docs/domain-contract.md` §5.

### Fuera de scope

- **Construir la URL canónica** desde la key (requiere base-url por provider): el `url` queda vacío;
  el board muestra la key sin link. Queda como futuro (nota del design de `add-ticket-linking`).
- **Validación contra el tracker externo** (que el ticket exista): no hay hit de API.
- **Múltiples tickets / array por spec**: se mantiene un único `Ticket` (primera ref detectada).
- **Etiquetas de contexto que NO son el ticket** (`Epic:`, `Story:`, `Sprint`): se ignoran; solo se
  toma la key del cue word de ticket o del prefijo conocido.
- **`unlink` / desasociar**; **cambios a la máquina de estados** (link no afecta status).
- **Inferencia automática del prefijo** (auto-descubrir `MH` sin declararlo): V1 exige declararlo en
  `ticketKeyPrefixes`.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Framework: binario Go (módulo único en `cli/`, stdlib; sin deps externas).
- Lenguaje: Go.
- Package manager: módulos Go (`cli/go.mod`).
- Testing: paquete `testing` estándar, table-driven (ver `ticket_test.go`, `store_test.go`).
- `web/` (React/Vite) y `kit/` (markdown): **sin cambios** en esta fase.

### Versiones relevantes

- Go: `1.26` (`cli/go.mod`).
- Tipos de ticket: `state.TicketProvider` ya existe (`cli/internal/state/types.go:56-64`),
  con `jira|linear|github|other`.

### Patrones existentes a respetar

- **Campo de config con default** ⇒ molde `ApplyMode` + `ResolvedApplyMode()`
  (`cli/internal/config/config.go:64,89-95`): enum string + método `Resolved*`.
- **Validación de enum** ⇒ molde `ApplyMode.Valid()` (`config.go:81`) / `Status.Valid()` /
  `Priority.Valid()` / `TicketProvider` (añadir `Valid()` si no existe).
- **Helpers de detección** ⇒ molde `ticketFromFrontmatter` / `ticketFromProse` (`ticket.go`):
  función pequeña, retorna `*state.Ticket` o `nil`, conservadora ante ambigüedad.
- **Escritura serializada del estado** ⇒ solo `Store` muta `state.json`; `LinkSpec` ya cubre
  idempotencia y precedencia (auto no pisa manual) — **no se re-implementa**.
- Errores envueltos con `fmt.Errorf("…: %w", err)`; sin `panic` en flujo normal.
- `import` de `state` desde `config` es válido (no hay ciclo: `state` no importa `config`).

---

## 4. Dependencias previas

Verificado en el repo (feature hermana ya mergeada en working tree):

- [x] `state.TicketProvider` enum (`jira|linear|github|other`) — `cli/internal/state/types.go:56-64`.
- [x] `Ticket{Provider,Key,URL,Auto}` y `CreateSpecParams.Ticket` — `types.go:104`, `store.go`.
- [x] `Store.LinkSpec` con idempotencia + precedencia (auto no pisa manual) — `cli/internal/state/store.go`.
- [x] `detectTicket`, `parseRef`, `inferProvider`, `ticketFromFrontmatter`, `ticketFromProse` —
      `cli/cmd/vector/ticket.go`.
- [x] `runSync` ya invoca `detectTicket`; `runSpecLink` ya invoca `parseRef` — `cli/cmd/vector/main.go`,
      `spec_transitions.go`.
- [x] `Config` con molde `ApplyMode`/`ResolvedApplyMode` — `cli/internal/config/config.go`.

Es una extensión **aditiva**: no faltan dependencias. Si al implementar `TicketProvider.Valid()` no
existe aún, añadirlo siguiendo el molde de `Status.Valid()`.

---

## 5. Arquitectura

### Patrón a usar

Config-driven detection: el provider por defecto y los prefijos viven en la config (fuente de verdad
persistida); `detectTicket`/`parseRef` los consultan en runtime. Toda escritura sigue por `Store`
(molde write+evento intacto). Sin modelo en el loop: la "inteligencia" de sync es heurística
determinista (regex + listas), respetando `product/token-routing.md` (sync es comando barato).

### Capas afectadas

- presentation (`web/`): **no** — el board ya proyecta y renderiza `card.ticket`.
- application/use-cases (`cli/cmd/vector`): **sí** — threading de config a `detectTicket`/`parseRef`
  en `runSync`/`runSpecLink`/`runSpecCreate`; lógica de cues/prefijos en `ticket.go`.
- domain (`cli/internal/state`): **mínimo** — añadir `TicketProvider.Valid()` si falta; `LinkSpec`
  sin cambios.
- configuration (`cli/internal/config`): **sí** — dos campos nuevos + validación en `Load` +
  método(s) `Resolved*`.
- data/infrastructure: serialización de los campos nuevos en `config.json` (`omitempty`).

### Flujo esperado

1. El dev declara en `.vector/config.json`: `"defaultTicketProvider":"jira"` y opcional
   `"ticketKeyPrefixes":["MH"]`.
2. `vector sync` → por cada change, `detectTicket(change, root, cfg)`:
   a. frontmatter `ticket:` → gana; b. URL de tracker en prosa → gana; c. **(nuevo)** si hay
   `defaultTicketProvider`, cue word anclado o prefijo conocido → key con `provider=default, url:"", auto:true`.
3. La key se persiste vía `CreateSpec(...Ticket{auto:true})` (nuevo) o `LinkSpec(...auto:true)`
   (reconcile), idempotente y sin pisar manual.
4. `vector spec link <id> MH-1592` (manual, sin `--provider`) → `parseRef` usa `defaultTicketProvider`
   → `LinkSpec(...auto:false)`.
5. `GET /api/board` trae la card con `ticket.provider="jira"` poblado (sin cambios de render).

### Ubicación de archivos nuevos

No se crean carpetas nuevas. Cambios dentro de paquetes existentes (`internal/config`,
`cmd/vector`, `internal/state`).

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `cli/internal/config/config.go` | MODIFICAR | `DefaultTicketProvider` + `TicketKeyPrefixes` en `Config`; validación en `Load`; `Resolved*` | `ApplyMode`/`ResolvedApplyMode` (config.go:64,89-95) |
| `cli/cmd/vector/ticket.go` | MODIFICAR | Fallback de detección por cue word/prefijo + denylist en `detectTicket`; `parseRef` consulta default | `ticketFromFrontmatter`/`ticketFromProse` (ticket.go) |
| `cli/cmd/vector/main.go` | MODIFICAR | Threading de `cfg` a `detectTicket` en `runSync` (y `runSpecCreate` si aplica) | invocación actual de `detectTicket` en `runSync` |
| `cli/cmd/vector/spec_transitions.go` | MODIFICAR | `runSpecLink` pasa `defaultTicketProvider` a `parseRef` cuando no hay `--provider` | `runSpecLink` actual |
| `cli/internal/state/types.go` | MODIFICAR (si falta) | `TicketProvider.Valid()` para validar el enum | `Status.Valid()` (types.go:29-35) |
| `cli/internal/config/config_test.go` | MODIFICAR | Load/Resolve + validación de los campos nuevos | tests existentes de config |
| `cli/cmd/vector/ticket_test.go` | MODIFICAR | cues, prefijos, denylist, precedencia de orden, parseRef con default | tests existentes de ticket |
| `docs/domain-contract.md` | MODIFICAR | §5: ampliar la nota `detectTicket`/`auto` con cue words + prefijos + default provider | nota `auto`/`detectTicket` (§5) |

### Detalle por archivo

#### cli/internal/config/config.go

Acción: MODIFICAR

Debe implementar:

- En `Config` (junto a `ApplyMode`):
  - `DefaultTicketProvider state.TicketProvider` con tag `json:"defaultTicketProvider,omitempty"`.
  - `TicketKeyPrefixes []string` con tag `json:"ticketKeyPrefixes,omitempty"`.
- Validación en `Load`: si `DefaultTicketProvider` no es vacío y no es `Valid()` →
  `fmt.Errorf("invalid defaultTicketProvider %q: allowed jira,linear,github,other", v)`. Vacío es válido.
- Método(s) `Resolved*` siguiendo el molde de `ResolvedApplyMode`:
  - `ResolvedDefaultTicketProvider() state.TicketProvider` (retorna el valor válido o `""`).
  - Opcional `NormalizedTicketKeyPrefixes() []string` (trim, upper, sin vacíos).

Restricciones:

- No tocar el resto del esquema de config; preservar `omitempty`.
- No validar base-url (fuera de scope).

#### cli/cmd/vector/ticket.go

Acción: MODIFICAR

Debe implementar:

- Un helper nuevo (molde `ticketFromProse`),
  `ticketFromContext(content string, provider state.TicketProvider, prefixes []string) *state.Ticket`:
  - Cue words (case-insensitive, inicio de línea, opcional `>`/`**`): `Ticket`, `Issue`, `Ref`,
    `Tracking`, `Jira`, `Linear`, `GitHub` → toma la **primera** key `[A-Za-z][A-Za-z0-9]*-\d+`
    tras el cue.
  - Prefijo conocido: key `^(<prefix>)-\d+` (de `prefixes`) en cualquier parte.
  - **Denylist** built-in (`ADR`, `RFC`): descarta esas keys aun bajo cue.
  - Retorna `&state.Ticket{Provider:provider, Key:key, URL:"", Auto:true}`; `nil` si 0 matches o
    conflicto (múltiples keys distintas) — conservador.
- `detectTicket` gana este fallback **después** de frontmatter y URL-prosa, y **solo** si
  `provider != ""`. Firma pasa a recibir lo necesario de config (provider + prefijos) — ver §10.
- `parseRef(ref, forced)` **no cambia de firma**: el caller (`runSpecLink`) pasa el default provider
  como argumento `forced` cuando `--provider` está vacío y la ref es una key suelta ambigua, en vez
  de dejar que falle por ambigüedad.

No debe incluir:

- Llamadas a modelo/LLM ni hits de red.
- Lógica de construcción de URL canónica.

#### cli/cmd/vector/main.go / spec_transitions.go

Acción: MODIFICAR

Cambios requeridos:

- `runSync`: cargar `cfg` (ya se carga) y pasar `defaultTicketProvider` + `ticketKeyPrefixes` a
  `detectTicket`.
- `runSpecCreate`: **sin cambios requeridos** — ya acepta `--ticket` JSON desde `/vector:raw`; la
  detección contextual del texto crudo la hace el command (markdown) antes de invocar al binario.
- `runSpecLink`: hoy no carga config. Añadir al inicio `root, err := resolveRepoRoot(*repoRoot)`
  (molde en `runSpecNext`, mismo archivo) y luego `cfg, err := config.Load(root)` (antes de
  `openStore`; propagar el error al caller). Después, cuando `--provider` está vacío y la ref es key
  suelta, pasar `cfg.ResolvedDefaultTicketProvider()` como `forced` a `parseRef`.

Restricciones:

- No cambiar el contrato de salida JSON de estos comandos.
- No alterar el orden de precedencia de detección existente.

---

## 7. API Contract

No aplica — no hay endpoint HTTP nuevo. `defaultTicketProvider` y `ticketKeyPrefixes` son metadata
local de `.vector/config.json`. `GET /api/board` sigue trayendo `Card.Ticket` igual que hoy
(`internal/board/board.go`), sin versionado nuevo. Única fuente de verdad de config:
`cli/internal/config`.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] Con `defaultTicketProvider:jira`, un change con `Ticket: MH-1592.` (sin URL ni frontmatter) →
      `vector sync` linkea `{provider:jira, key:MH-1592, url:"", auto:true}`.
- [ ] Con `ticketKeyPrefixes:["MH"]`, un change que menciona `MH-1592` sin cue word → se linkea;
      `ADR-007` y `RFC-3` en la misma prosa → **no** se linkean (denylist).
- [ ] Cue words `Issue:`, `**Ticket:**`, `> Ticket:`, `Jira:` → detectan; una key suelta sin cue ni
      prefijo conocido → **no** se detecta.
- [ ] Sin `defaultTicketProvider` configurado → comportamiento actual (no adivina; `nil`).
- [ ] `vector spec link <id> MH-1592` sin `--provider`, con `defaultTicketProvider:jira` → linkea
      manual (`auto:false`); sin default configurado → sigue siendo error accionable.
- [ ] Precedencia de fuentes: frontmatter > URL-prosa > cue/prefijo. La primera que matchea gana.
- [ ] Idempotencia y precedencia auto-vs-manual: sin regresión (cubierto por `LinkSpec`).
- [ ] `config.Load` con `defaultTicketProvider:"jirra"` → error claro; campo omitido → OK.
- [ ] `GET /api/board` trae las cards linkeadas con el provider por defecto (sin regresión de render).
- [ ] Sin regresiones en `sync`/`raw`/`propose`/`apply`/`serve`/`link`.

### Tests requeridos

- [ ] `ticketFromContext`: cada cue word; prefijo conocido; denylist `ADR`/`RFC`; conflicto → nil;
      key suelta sin señal → nil; toma la primera key tras el cue (ignora `Epic`/`Story` en la línea).
- [ ] `detectTicket`: orden frontmatter > URL > cue/prefijo; sin provider → nil.
- [ ] `parseRef`: key suelta + default provider → resuelve; sin default → error.
- [ ] `config`: Load/Resolve de ambos campos; inválido → error; normalización de prefijos.
- [ ] `runSync`/`runSpecLink`: threading de config (integración con config fake en tempdir).

### Comandos de verificación

```bash
gofmt -l cli
go -C cli vet ./...
go -C cli test -race ./...
```

La fase no está completa si alguno falla.

---

## 9. Criterios de UX

No aplica — feature interna de CLI/config + automatización de sync. No hay UI nueva ni cambios
visuales: el board ya renderiza `card.ticket` (responsabilidad de `web/SpecCard.tsx`, sin cambios).
La "UX de CLI" se cubre en §13 (mensajes de error accionables).

---

## 10. Decisiones tomadas

El agente no debe cuestionarlas ni cambiarlas:

- **Detección determinista (sin modelo) en sync**: cues + prefijos + denylist; nada de LLM por sync
  (token-routing). En `/vector:raw` la detección contextual la hace el command (modelo en loop) — esta
  fase no cambia ese camino.
- **Forma de key universal**: `[A-Za-z][A-Za-z0-9]*-\d+`, interpretada por el `defaultTicketProvider`
  configurado (cubre `MH-1592` Jira y `ENG-7` Linear).
- **Link manual de key suelta usa el default**: `parseRef` consulta `defaultTicketProvider` cuando no
  hay `--provider`; sin default sigue siendo error.
- **Config inválida → error en `Load`** (no fallback silencioso): el typo se descubre de inmediato.
- **Solo la etiqueta de ticket / prefijo conocido**: `Epic:`/`Story:`/`Sprint` se ignoran; se toma la
  key del cue de ticket o del prefijo.
- **Denylist built-in `ADR`, `RFC`**: nunca tickets.
- **Orden de precedencia**: frontmatter `ticket:` > URL de prosa > cue/prefijo. Fallback solo si hay
  `defaultTicketProvider`.
- **`auto:true`** para todo lo detectado; **no pisa** manual (`auto:false`) — ya en `LinkSpec`.
- **`url` vacío válido**: key sin host se guarda sin link.
- **Firma de `detectTicket`**: recibe explícitamente el provider por defecto y los prefijos (no hace
  I/O de config interno); el caller los pasa desde la `cfg` ya cargada.
- **Firma de `parseRef`**: no cambia; `runSpecLink` pasa el default como argumento `forced` cuando
  `--provider` está vacío.

Si el agente detecta una alternativa mejor, la reporta como observación, no la implementa.

---

## 11. Edge cases

### Datos / prosa ambigua

- Key suelta sin cue ni prefijo conocido (`… see MH-1558 …`): **no** se detecta.
- Múltiples keys distintas bajo cues/prefijos en los artefactos: **conflicto → nil** (conservador).
- `> Ticket: MH-1611 · Epic MH-1528 · Story 1 of 6`: toma `MH-1611` (primera tras el cue), ignora el resto.
- `ADR-007`, `RFC-3`: denylist → ignorados aun con cue.
- `ticketKeyPrefixes` vacío y sin cue: solo cues operan; sin cues tampoco detecta.
- Key con formato inválido (`Ticket: hello`): no matchea `[A-Za-z][A-Za-z0-9]*-\d+` → nil.

### Config

- `defaultTicketProvider` inválido → `config.Load` error (no continúa con valor corrupto).
- `defaultTicketProvider` omitido → fallback desactivado (comportamiento actual).
- `ticketKeyPrefixes` con espacios/case mixto → normalizar (trim + upper) antes de comparar.

### Estado / precedencia

- Spec ya linkeado manual y detección encuentra otro: **no se pisa** (`LinkSpec` precedencia).
- Spec ya linkeado con el mismo `provider+key+url`: **idempotente**, no re-emite `spec.linked`.
- Spec `closed`/`archived`: linkear permitido (link no es estado-restrictivo).

### Sin conexión / Timeout / Respuesta vacía / Doble submit

No aplica — sin red ni UI; operación local determinista.

---

## 12. Estados de UI requeridos

No aplica — sin UI nueva. El board no agrega estados; una card linkeada muestra su ticket (ya soportado).

---

## 13. Validaciones

### Validaciones de cliente (CLI/config)

| Campo | Regla | Mensaje |
|---|---|---|
| `defaultTicketProvider` | `jira\|linear\|github\|other` o vacío | `invalid defaultTicketProvider "<x>": allowed jira,linear,github,other` |
| `ticketKeyPrefixes[]` | strings no vacíos; normalizados (trim/upper) | (silencioso: vacíos se descartan) |
| key detectada | `[A-Za-z][A-Za-z0-9]*-\d+` y prefijo ∉ denylist | (silencioso: no-match → nil) |
| `vector spec link <id> <key>` sin `--provider` | requiere `defaultTicketProvider` configurado | `ambiguous ticket ref "<key>": pass --provider … (or set defaultTicketProvider)` |

### Validaciones de servidor

No aplica — sin backend remoto; la "validación" es local en `config.Load` y en los helpers de detección.

---

## 14. Seguridad y permisos

No aplica como superficie sensible — el provider y los prefijos son metadata no secreta, commiteada
en `.vector/config.json`. No se exponen tokens ni se hacen hits autenticados. Mantener la regla
general: no loggear payloads ni secretos (no hay aquí).

---

## 15. Observabilidad y logging

- Reusar el mecanismo existente: la detección que linkea emite `spec.linked` (`auto:true`) vía `Store`
  (sin nuevo canal de log).
- Errores de config → stderr con mensaje claro y accionable (molde de errores del CLI).
- No registrar información sensible (no aplica).

---

## 16. i18n / textos visibles

No aplica — proyecto CLI sin i18n; los mensajes del binario van en inglés (convención del repo y
`workflows/git-convention.md`). Textos de error nuevos: el de `defaultTicketProvider` inválido y el de
key ambigua sin default (ver §13), en inglés.

---

## 17. Performance

- Detección: una pasada de regex por línea sobre los 3 artefactos por change; despreciable frente al
  I/O que `sync` ya paga.
- `config.Load`: sin cambios de orden de magnitud.
- Compilar los regex una sola vez a nivel de paquete (`var … = regexp.MustCompile(...)`), como ya hace
  `ticket.go`.

---

## 18. Restricciones

El agente no debe:

- Invocar modelos/LLM ni red en la ruta de `sync`/`detectTicket`.
- Construir URLs canónicas ni validar contra el tracker (fuera de scope).
- Soportar múltiples tickets / array en V1.
- Pisar un ticket manual con detección automática (respeta `LinkSpec`).
- Cambiar la máquina de estados ni el contrato JSON de los comandos.
- Refactorizar helpers no relacionados ni cambiar el orden de precedencia existente.
- Detectar keys bajo `Epic:`/`Story:`/`Sprint` ni keys con prefijo en la denylist.
- Auto-descubrir el prefijo de proyecto (debe declararse).

---

## 19. Entregables

Al finalizar deben quedar:

- [ ] `config.go`: `DefaultTicketProvider` + `TicketKeyPrefixes` + validación en `Load` + `Resolved*`.
- [ ] `types.go`: `TicketProvider.Valid()` (si faltaba).
- [ ] `ticket.go`: `ticketFromContext` (cues + prefijos + denylist) integrado en `detectTicket`;
      `parseRef` consulta el default.
- [ ] `main.go`/`spec_transitions.go`: threading de config a `detectTicket`/`parseRef`.
- [ ] Tests nuevos/actualizados (helpers, config, integración sync/link).
- [ ] `docs/domain-contract.md` §5 actualizado.
- [ ] Gate Go verde (`gofmt`/`vet`/`test -race`).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo y el spec hermano `add-ticket-linking`.
- [ ] Revisé `config.go` (ApplyMode/ResolvedApplyMode), `ticket.go` (detectTicket/parseRef/helpers),
      `types.go` (TicketProvider), `main.go`/`spec_transitions.go` (callers).
- [ ] Confirmé que `state` no es importado por `config` antes de importar (sin ciclo).
- [ ] Implementé los dos campos de config con validación en `Load` y `Resolved*`.
- [ ] Implementé `ticketFromContext` (cues + prefijos + denylist), conservador ante conflicto.
- [ ] Mantuve el orden de precedencia: frontmatter > URL > cue/prefijo, solo con default provider.
- [ ] Extendí `parseRef` para usar el default en key suelta.
- [ ] Threadeé la config a los callers sin cambiar contratos JSON.
- [ ] Agregué tests (incluye denylist ADR/RFC y "Epic/Story se ignoran").
- [ ] Actualicé `docs/domain-contract.md` §5.
- [ ] Ejecuté gofmt, vet, test -race.
- [ ] No agregué deps, no toqué la máquina de estados, no dejé TODOs sin justificar.

---

## Open questions

- **Auto-descubrimiento de prefijo**: ¿valdría inferir el prefijo dominante (no-ADR) en vez de
  declararlo? Diferido a una fase futura (riesgo de falsos positivos).
- **base-url por provider** para llenar `url` (p. ej. `https://<host>/browse/MH-1592`): futuro, ligado
  a la nota del design de `add-ticket-linking`.
- **`vector init` interactivo** para pre-rellenar `defaultTicketProvider`/`ticketKeyPrefixes`:
  fuera de scope; por ahora edición manual de `.vector/config.json`.
