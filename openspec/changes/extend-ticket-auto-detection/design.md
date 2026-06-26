# Design — extend-ticket-auto-detection

## Decisiones clave

- **Detección determinista, sin modelo en `sync`**: la "inteligencia" extra son heurísticas (regex +
  listas), no una llamada a LLM. `sync` es un comando barato (`product/token-routing.md`). En
  `/vector:raw` la detección contextual del texto crudo ya la hace el command (modelo en loop) — esta
  fase **no** cambia ese camino.
- **Fallback solo con `defaultTicketProvider` configurado**: sin el campo, `detectTicket` se comporta
  como hoy (no adivina, retorna `nil`). El default es un opt-in explícito del repo.
- **Dos señales, ambas conservadoras**: (a) **cue word** anclado al inicio de línea (tolerando `>` y
  `**`): `Ticket`/`Issue`/`Ref`/`Tracking`/`Jira`/`Linear`/`GitHub`, tomando la **primera** key tras
  el cue; (b) **prefijo de proyecto conocido** (`ticketKeyPrefixes`) en cualquier parte. Cualquiera
  de las dos basta.
- **Denylist built-in `ADR`, `RFC`**: nunca son tickets, aun bajo un cue. El repo de prueba está lleno
  de `ADR-007` como ruido.
- **Forma de key universal** `[A-Za-z][A-Za-z0-9]*-\d+`, interpretada por el `defaultTicketProvider`
  (cubre `MH-1592` Jira y `ENG-7` Linear). No se valida formato por-provider en V1.
- **Orden de precedencia intacto**: frontmatter `ticket:` > URL de prosa > cue/prefijo. La primera
  fuente que matchea gana.
- **Conservador ante conflicto**: 0 matches o múltiples keys distintas → `nil`. `Epic:`/`Story:`/
  `Sprint` en la misma línea se ignoran (solo la key del cue de ticket o del prefijo).
- **Link manual de key suelta**: `parseRef` **no cambia de firma**; `runSpecLink` pasa
  `defaultTicketProvider` como argumento `forced` cuando `--provider` está vacío. Sin default
  configurado, sigue siendo un error accionable.
- **Config inválida → error en `Load`** (no fallback silencioso): el typo (`jirra`) se descubre de
  inmediato, en vez del desconcierto "configuré y no linkea nada".
- **`url` vacío válido**: key sin host se guarda sin link; el board muestra la key. Construir la URL
  canónica (base-url por provider) queda como futuro.
- **Idempotencia y precedencia auto-vs-manual ya existen** en `Store.LinkSpec` — no se reimplementan;
  lo detectado se linkea con `auto:true` y no pisa un manual (`auto:false`).

## Superficie

- `cli/internal/config/config.go`: `Config` gana `DefaultTicketProvider state.TicketProvider`
  (`json:"defaultTicketProvider,omitempty"`) y `TicketKeyPrefixes []string`
  (`json:"ticketKeyPrefixes,omitempty"`); validación en `Load`; `ResolvedDefaultTicketProvider()` y
  `NormalizedTicketKeyPrefixes()` con molde `ResolvedApplyMode` (`config.go:89-95`). `import` de
  `state` válido (sin ciclo: `state` no importa `config`).
- `cli/internal/state/types.go`: `TicketProvider.Valid()` si no existe (molde `Status.Valid()`).
- `cli/cmd/vector/ticket.go` (NUEVO helper): `ticketFromContext(content, provider, prefixes)`
  (molde `ticketFromProse`); integrado en `detectTicket` como tercer fallback (solo si
  `provider != ""`). `parseRef` sin cambio de firma.
- `cli/cmd/vector/main.go`: `runSync` thread-ea `defaultTicketProvider` + `ticketKeyPrefixes` a
  `detectTicket`. `runSpecCreate` sin cambios (ya acepta `--ticket` JSON).
- `cli/cmd/vector/spec_transitions.go`: `runSpecLink` añade `root, _ := resolveRepoRoot(*repoRoot)` +
  `cfg, _ := config.Load(root)` y pasa el default a `parseRef` como `forced`.
- `docs/domain-contract.md` §5: ampliar la nota `auto`/`detectTicket` con cues + prefijos + default.
- `web/`: **sin cambios** — `Card.Ticket` ya se proyecta y renderiza.

## Flujo

- **Sync**: por change → `detectTicket(change, root, provider, prefixes)`: frontmatter → URL → (nuevo)
  cue/prefijo con provider default → `Ticket{auto:true}` → create (params) o reconcile (`LinkSpec`),
  idempotente y respetando precedencia manual.
- **Link manual**: `vector spec link <id> MH-1592` sin `--provider` → `runSpecLink` resuelve el default
  → `parseRef(ref, default)` → `LinkSpec(…, auto:false)`.

## Constraint pendiente (no abierto en código)

Auto-descubrir el prefijo dominante (no declararlo en `ticketKeyPrefixes`) queda como futuro: el
riesgo de falsos positivos (distinguir `MH` ticket de `ADR` doc) exige más señal de la que V1 asume.
