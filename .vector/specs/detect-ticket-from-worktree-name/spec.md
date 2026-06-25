# Spec: Detectar el ticket del worktree por slug en sync (multinivel)

## 1. Objetivo

Construir una **cuarta fuente de detección de ticket** para `vector sync`: cuando un spec/change no
trae ticket en sus artefactos, asociar el spec a su **carpeta de worktree** (`<KEY>-<slug>`) y extraer
la Jira/Linear/GitHub key del nombre de esa carpeta. La enumeración de worktrees es **multinivel**:
recorre las carpetas de agrupación (`feat/`, `chore/`, `fix/`, `docs/`, y branches de un nivel como
`develop`) bajo la raíz de worktrees del repo.

Esta feature permite que un dev en un repo bare+worktree —donde cada branch se llama `<KEY>-<slug>`
(p. ej. `code/feat/mh-1592-payments-period-checkout-camelcase`)— obtenga auto-link del ticket aunque
los artefactos del change no mencionen la key. Es la señal de mayor recall: en el repo de prueba
(somnio) eleva la detección de ~4 a 24 specs, con match exacto y sin falsos positivos.

## 2. Alcance

### Incluido en esta fase

- **Enumerador de worktrees multinivel**: deriva la **raíz de worktrees** del repo desde el prefijo
  literal del template `changesPath` (la parte antes de `[branch]`; p. ej. `code/`) y hace un **scan
  acotado** (profundidad limitada) que encuentra carpetas-branch a uno o varios niveles, tolerando
  carpetas de agrupación (`feat`/`chore`/`fix`/`docs`/…). Read-only.
- **Índice slug → ticket key**: por cada carpeta-branch hoja, si su basename casa `<KEY>-<slug>` o es
  un `<KEY>` puro, registra la key (forma universal `[A-Za-z][A-Za-z0-9]*-\d+`, normalizada a
  mayúsculas), aplicando la **denylist** `ADR`/`RFC`. Match a un change por **slug exacto** (== nombre
  del change) tras quitar el prefijo `<KEY>-`.
- **4ª fuente en `detectTicket`**: como **último fallback** (después de frontmatter, URL en prosa y
  cue/prefijo en artefactos), **solo si** `defaultTicketProvider` está configurado, usa la key del
  worktree para el slug del change y linkea `{provider: default, key, url:"", auto:true}`.
- **Threading**: `runSync` construye el índice **una vez** y pasa la key candidata (por slug) a
  `detectTicket`.
- **Tests Go** y actualización de `docs/domain-contract.md` §5.

### Fuera de scope

- **Cambiar el glob que *lee* changes/spec docs** (`ChangesDirs`/`FindSpecDocs`): los changes ya se
  leen completos desde la raíz `./openspec/changes`; no hay changes perdidos que rescatar. Tocar ese
  glob sería riesgo para otros repos sin beneficio aquí.
- **Match difuso de slug**: solo match exacto tras quitar `<KEY>-`. Un slug que derivó (worktree
  `mh-1385-resident-work-orders` vs spec `resident-work-orders-amendment`) **no** casa (conservador).
- **Worktrees `<KEY>` puros sin slug**: no se pueden asociar a un change específico → se ignoran.
- Construir la **URL canónica** desde la key (base-url por provider, futuro); validación contra el
  tracker; múltiples tickets por spec; auto-descubrir provider sin `defaultTicketProvider`.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Framework: binario Go (módulo único en `cli/`, stdlib; sin deps externas).
- Lenguaje: Go.
- Testing: paquete `testing` estándar, table-driven (`ticket_test.go`, `config_test.go`).
- `web/` y `kit/`: sin cambios.

### Versiones relevantes

- Go: `1.26` (`cli/go.mod`).
- `state.TicketProvider`, `Ticket{Provider,Key,URL,Auto}`, `Store.LinkSpec`, `config.DefaultTicketProvider`
  y los helpers `ticketFromContext`/`denylistedKey`/`pickSingleKey` ya existen (features
  `add-ticket-linking` y `extend-ticket-auto-detection`).

### Patrones existentes a respetar

- **Enumeración de worktrees y captura de `[branch]`** ⇒ molde `ChangesDirs`/`compileTemplate`
  (`cli/internal/config/config.go`): hoy `[branch]`=`(?P<branch>[^/]+)` (un segmento) — esta fase
  **no** lo modifica; añade un scan multinivel propio para la señal de ticket.
- **Forma de key + denylist** ⇒ reusar `bareKeyRe`, `denylistedKey`, `pickSingleKey` de `ticket.go`.
- **Escritura serializada** ⇒ solo `Store` muta `state.json`; `LinkSpec` cubre idempotencia y
  precedencia (auto no pisa manual). No se reimplementa.
- Errores envueltos `fmt.Errorf("…: %w", err)`; sin `panic` en flujo normal.

---

## 4. Dependencias previas

Verificado en el working tree (features hermanas ya implementadas):

- [x] `detectTicket(change, root, defaultProvider, keyPrefixes)` con frontmatter → URL → cue/prefijo —
      `cli/cmd/vector/ticket.go`.
- [x] `bareKeyRe`, `denylistedKey`, `pickSingleKey`, `ticketKeyDenylist` — `ticket.go`.
- [x] `config.DefaultTicketProvider` + `ResolvedDefaultTicketProvider()` — `config.go`.
- [x] `Store.LinkSpec` idempotencia/precedencia — `store.go`.
- [x] `runSync` ya invoca `detectTicket` y carga `cfg` — `cli/cmd/vector/main.go`.
- [x] `config.changesTemplate()` / template con `[branch]` — `config.go`.

Extensión aditiva; sin dependencias faltantes.

---

## 5. Arquitectura

### Patrón a usar

Config-driven, determinista, sin modelo (sync es back-fill barato pero puede indagar más señales
deterministas — `product/token-routing.md`). El índice slug→key se computa una vez por sync y alimenta
a `detectTicket` como último recurso.

### Capas afectadas

- configuration (`internal/config`): nuevo helper de enumeración de worktrees + índice slug→key (lee
  el filesystem read-only; deriva la raíz del template). Posible nuevo método en `Config`.
- application (`cmd/vector`): `runSync` computa el índice y lo pasa a `detectTicket`; `detectTicket`
  gana el 4º fallback.
- domain (`internal/state`): sin cambios (`LinkSpec` ya existe).
- presentation (`web/`): sin cambios.

### Flujo esperado

1. `vector sync` carga `cfg`; si `cfg.ResolvedDefaultTicketProvider() != ""`, computa
   `idx := cfg.WorktreeTicketKeys(root)`. La comprobación de `[branch]` en el template vive **dentro**
   de `WorktreeTicketKeys` (devuelve mapa vacío si no aplica), **no** en `runSync` — `runSync` no
   inspecciona el template.
2. Por cada change: `detectTicket(change, root, provider, prefixes, branchKey)` donde
   `branchKey = idx[change.Name]` (o "").
3. `detectTicket`: frontmatter → URL → cue/prefijo → **(nuevo)** si todo lo anterior dio nil y
   `branchKey != ""` y no está en denylist → `Ticket{provider, branchKey, url:"", auto:true}`.
4. Persiste vía create/reconcile (`LinkSpec`), idempotente y sin pisar manual.

### Ubicación de archivos nuevos

Sin carpetas nuevas. Cambios en `internal/config`, `cmd/vector`.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `cli/internal/config/config.go` | MODIFICAR | `WorktreeTicketKeys(repoRoot) map[string]string` (slug→key): deriva raíz desde el prefijo de `changesTemplate()` antes de `[branch]`, scan multinivel acotado, basename `<KEY>-<slug>`/`<KEY>`, key universal + denylist + upper | `ChangesDirs`/`changesTemplate` (config.go) |
| `cli/cmd/vector/ticket.go` | MODIFICAR | `detectTicket` gana parámetro `branchKey string` y un 4º fallback (último, gated en provider, denylist) | fallbacks actuales en `detectTicket` |
| `cli/cmd/vector/main.go` | MODIFICAR | `runSync` computa el índice una vez (cuando hay default provider + template con `[branch]`) y pasa `index[c.Name]` a `detectTicket` | invocación actual de `detectTicket` en `runSync` |
| `cli/internal/config/config_test.go` | MODIFICAR | Tests del enumerador multinivel + match por slug + denylist + upper | tests de config existentes |
| `cli/cmd/vector/ticket_test.go` | MODIFICAR | **Actualizar las ~10 llamadas existentes a `detectTicket(c, root, …)` añadiendo `""` como 5º arg (`branchKey`)** + tests del 4º fallback: branchKey linkea; vacío → nil; denylist; precedencia (artefacto gana) | tests de ticket existentes |
| `docs/domain-contract.md` | MODIFICAR | §5: añadir "nombre del worktree por slug" al orden de precedencia de `detectTicket` | nota `detectTicket` §5 |

### Detalle por archivo

#### cli/internal/config/config.go

Acción: MODIFICAR

Debe implementar `WorktreeTicketKeys(repoRoot string) map[string]string`:

- Deriva la **raíz de worktrees**: el prefijo literal de `changesTemplate()` hasta el primer
  `[branch]` (p. ej. `code/[branch]/openspec/changes` → `code`). Si el template no contiene
  `[branch]`, devuelve un mapa vacío (repo sin worktrees; feature inerte).
- **Scan multinivel acotado** bajo esa raíz para enumerar carpetas-branch; tolera carpetas de
  agrupación (`feat`/`chore`/`fix`/`docs`/…) y branches de un nivel (`develop`). La cota de profundidad
  se expone como **constante nombrada** (p. ej. `worktreeMaxDepth = 3`) con comentario del porqué — no
  un número mágico.
- Por cada basename de carpeta candidata: extraer `<KEY>` con la forma universal; si el basename es
  `<KEY>-<resto>`, `slug = resto`; si es `<KEY>` puro, no se indexa (no hay slug). Descarta key en
  denylist (`ADR`/`RFC`). Normaliza la parte de proyecto de la key a **mayúsculas** (`mh-1592`→`MH-1592`).
- Mapa `slug → key`. Ante slugs duplicados con keys distintas, omitir esa entrada (ambiguo).

Restricciones: read-only; no usar `filepath.Glob` de un solo nivel (debe ser multinivel); no tocar
`ChangesDirs`/`compileTemplate`.

#### cli/cmd/vector/ticket.go

Acción: MODIFICAR

- `detectTicket(change openspec.Change, root string, defaultProvider state.TicketProvider, keyPrefixes []string, branchKey string) *state.Ticket`:
  añade `branchKey` como último parámetro. Tras los tres fallbacks actuales, si nada matcheó,
  `defaultProvider != ""`, `branchKey != ""` y `!denylistedKey(branchKey)` →
  `&state.Ticket{Provider: defaultProvider, Key: branchKey, URL: "", Auto: true}`.
- No cambia la lógica de los fallbacks previos (artefacto siempre gana sobre branch).

#### cli/cmd/vector/main.go

Acción: MODIFICAR

- En `runSync`, antes del loop de changes: si `cfg.ResolvedDefaultTicketProvider() != ""`, computar
  `idx := cfg.WorktreeTicketKeys(root)` una vez (mapa vacío si no aplica).
- Pasar `idx[c.Name]` como `branchKey` a `detectTicket`.

Restricciones: sin cambios al contrato JSON ni al orden de precedencia.

---

## 7. API Contract

No aplica — sin endpoint HTTP nuevo. La señal es metadata local derivada del filesystem; `GET /api/board`
sigue trayendo `Card.Ticket` igual.

---

## 8. Criterios de éxito

- [ ] Repo bare+worktree con `defaultTicketProvider:jira`: un change `payments-period-checkout-camelcase`
      sin ticket en artefactos, con worktree `code/feat/mh-1592-payments-period-checkout-camelcase` →
      `vector sync` linkea `{jira, MH-1592, url:"", auto:true}`.
- [ ] Worktree multinivel (`code/<tipo>/<branch>`) y de un nivel (`code/develop`) ambos enumerados.
- [ ] Slug que no casa exacto (worktree `mh-1385-resident-work-orders` vs spec
      `resident-work-orders-amendment`) → **sin** ticket (conservador).
- [ ] Worktree `<KEY>` puro (sin slug) → no asociado.
- [ ] Key normalizada a mayúsculas (`mh-1592`→`MH-1592`); denylist `ADR`/`RFC` ignorada.
- [ ] **Precedencia**: si el artefacto trae ticket (frontmatter/URL/cue), ese gana sobre el del branch.
- [ ] Sin `defaultTicketProvider` → no se computa índice ni se usa branch (comportamiento actual).
- [ ] Template sin `[branch]` (repo no-worktree, p. ej. el propio Vector) → feature inerte, sin regresión.
- [ ] Idempotencia/precedencia auto-vs-manual: sin regresión (`LinkSpec`).
- [ ] Sin regresiones en `create`/`sync`/`propose`/`apply`/`serve`/`link`.

### Tests requeridos

- [ ] `WorktreeTicketKeys`: layout multinivel (feat/chore) → mapa correcto; un nivel (develop); denylist;
      upper-normalize; slug duplicado con keys distintas → omitido; template sin `[branch]` → vacío.
- [ ] `detectTicket` con `branchKey`: linkea como último fallback; artefacto gana sobre branch;
      branchKey denylisted → nil; sin provider → nil.
- [ ] `runSync` integración: tempdir con worktrees fake + config con default provider → linkea por branch.

### Comandos de verificación

```bash
gofmt -l cli
go -C cli vet ./...
go -C cli test -race ./...
```

La fase no está completa si alguno falla.

---

## 9. Criterios de UX

No aplica — feature interna de CLI/automatización. Sin UI nueva; el board ya renderiza `card.ticket`.

---

## 10. Decisiones tomadas

El agente no debe cuestionarlas:

- **Match por slug exacto** tras quitar `<KEY>-` (precisión sobre recall): en somnio 24/38, 0 falsos
  positivos. Slugs derivados no casan (aceptado).
- **Enumeración multinivel acotada** bajo la raíz de worktrees, tolerando grupos `feat/chore/fix/docs`
  y branches de un nivel. No se usa `filepath.Glob` de un solo `*`.
- **Raíz de worktrees** = prefijo literal de `changesTemplate()` antes de `[branch]`. Sin `[branch]` →
  feature inerte (no rompe repos no-worktree como el propio Vector).
- **Último fallback**, gated en `defaultTicketProvider`. El artefacto (frontmatter/URL/cue/prefijo)
  **siempre gana** sobre el nombre del branch (es más explícito).
- **NO se toca** el glob de lectura de changes (`ChangesDirs`/`compileTemplate`): los changes se leen
  desde la raíz; este spec solo añade la señal de ticket.
- **Key normalizada a mayúsculas** (forma canónica Jira/Linear); denylist `ADR`/`RFC` reusada.
- **Worktree `<KEY>` puro sin slug** → no se asocia (no hay a qué change mapearlo).
- **Índice computado una vez** por `runSync` (no por change).
- Idempotencia/precedencia auto-vs-manual ya vienen de `LinkSpec`.

Si el agente ve una alternativa mejor, la reporta como observación, no la implementa.

---

## 11. Edge cases

- Worktree multinivel `code/feat/mh-1592-<slug>` y un nivel `code/develop`: ambos enumerados.
- Slug derivado (`mh-1385-resident-work-orders` vs `resident-work-orders-amendment`): no casa → sin link.
- Worktree `<KEY>` puro (`code/feat/MH-1524`): sin slug → no se asocia.
- Mismo slug en dos worktrees con keys distintas: omitido (ambiguo).
- Key con prefijo en denylist (`ADR-7`, `RFC-3`) en el nombre del branch → ignorada.
- Artefacto con ticket + worktree con otra key: gana el artefacto (precedencia).
- Spec ya linkeado manual: `LinkSpec` no lo pisa (precedencia auto-vs-manual).
- Repo sin `[branch]` en template: `WorktreeTicketKeys` → mapa vacío; `detectTicket` ignora branch.
- Profundidad mayor a la cota: no se enumera (acotado para no recorrer el árbol entero).

### Sin conexión / Timeout / Respuesta vacía / Doble submit

No aplica — operación local determinista, sin red ni UI.

---

## 12. Estados de UI requeridos

No aplica — sin UI. (Nota: los specs `archived` no se pintan en el board hoy —`board.go`—, así que un
ticket linkeado en un spec archivado no será visible hasta que exista la vista de archivados; fuera de
scope de este spec.)

---

## 13. Validaciones

### Validaciones de cliente (CLI/config)

| Campo | Regla | Mensaje |
|---|---|---|
| basename de worktree | `<KEY>-<slug>` con `[A-Za-z][A-Za-z0-9]*-\d+`, prefijo ∉ denylist | (silencioso: no-match → no indexa) |
| `defaultTicketProvider` | requerido para activar el fallback | (silencioso: sin él, no se usa branch) |
| slug duplicado, keys distintas | ambiguo | (silencioso: se omite la entrada) |

### Validaciones de servidor

No aplica — sin backend remoto.

---

## 14. Seguridad y permisos

No aplica como superficie sensible — solo lectura de nombres de carpeta y metadata de tracker (no
secreta). No se exponen tokens ni se hacen hits autenticados.

---

## 15. Observabilidad y logging

- La detección que linkea emite `spec.linked` (`auto:true`) vía `Store` (sin canal nuevo).
- **Errores de I/O del scan, por categoría**: error de permisos en un subdirectorio → se **omite ese
  subárbol** (tolerante, el índice sigue). Error al derivar la raíz de worktrees del template (config
  malformada) → **se propaga** al caller. Las dos no se mezclan.
- No registrar información sensible (no aplica).

---

## 16. i18n / textos visibles

No aplica — CLI sin i18n; mensajes del binario en inglés (convención del repo).

---

## 17. Performance

- El índice se computa **una sola vez** por `runSync`, no por change.
- Scan **acotado** en profundidad para no recorrer todo `code/` (que puede contener checkouts grandes);
  solo se listan nombres de carpeta hasta la cota, no su contenido.
- Regex compilados a nivel de paquete (reusa `bareKeyRe`).

---

## 18. Restricciones

El agente no debe:

- Modificar `ChangesDirs`/`compileTemplate`/`FindSpecDocs` ni el glob de lectura de changes/spec docs.
- Recorrer `code/` sin cota de profundidad (riesgo de performance en checkouts grandes).
- Hacer match difuso de slug ni asociar worktrees `<KEY>` puros.
- Construir URL canónica ni validar contra el tracker.
- Pisar un ticket de artefacto o un link manual con el del branch.
- Invocar modelos/LLM ni red.
- Soportar múltiples tickets / array.

---

## 19. Entregables

- [ ] `config.go`: `WorktreeTicketKeys` (scan multinivel + match por slug + denylist + upper).
- [ ] `ticket.go`: `detectTicket` con `branchKey` como 4º fallback.
- [ ] `main.go`: `runSync` computa el índice una vez y lo threadea.
- [ ] Tests (config enumerador + ticket fallback + integración runSync).
- [ ] `docs/domain-contract.md` §5 actualizado.
- [ ] Gate Go verde (`gofmt`/`vet`/`test -race`).

---

## 20. Checklist final para el agente

- [ ] Leí este spec y los hermanos `add-ticket-linking` / `extend-ticket-auto-detection`.
- [ ] Revisé `config.go` (changesTemplate, ChangesDirs, compileTemplate), `ticket.go` (detectTicket,
      denylistedKey, bareKeyRe), `main.go` (runSync).
- [ ] Implementé `WorktreeTicketKeys` con scan multinivel acotado y match por slug exacto.
- [ ] Normalicé la key a mayúsculas y reusé la denylist.
- [ ] Añadí el 4º fallback a `detectTicket` (último, gated, artefacto gana).
- [ ] Threadeé el índice desde `runSync` (computado una vez).
- [ ] No toqué el glob de lectura de changes ni la máquina de estados.
- [ ] Agregué tests (multinivel, un nivel, denylist, precedencia, sin provider, sin `[branch]`).
- [ ] Actualicé `docs/domain-contract.md` §5.
- [ ] Ejecuté gofmt, vet, test -race.
- [ ] No dejé `[...]` ni TODOs sin justificar.

---

## Open questions

- **Cota de profundidad del scan**: constante `worktreeMaxDepth` (p. ej. 3) para cubrir
  `code/<tipo>/<branch>`; si algún repo anida más, se ajusta. Valor trazable vía la constante nombrada.
- **Slugs derivados** (worktree vs change con sufijo extra): hoy no casan; un match por prefijo o
  normalización de slug se evalúa en una fase futura si el recall lo amerita.
- **Unificación futura** con `FindSpecDocs`/canonical worktree logic (hoy de un nivel): fuera de scope.
