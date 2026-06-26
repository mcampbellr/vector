# Tasks — extend-ticket-auto-detection

## 1. Config (`cli/internal/config`)

- [x] 1.1 `Config` gana `DefaultTicketProvider state.TicketProvider` (`json:"defaultTicketProvider,omitempty"`) y `TicketKeyPrefixes []string` (`json:"ticketKeyPrefixes,omitempty"`). Importar `state` (sin ciclo).
- [x] 1.2 `ResolvedDefaultTicketProvider() state.TicketProvider` (valor válido o `""`) y `NormalizedTicketKeyPrefixes() []string` (trim + upper, sin vacíos), molde `ResolvedApplyMode`.
- [x] 1.3 Validación en `Load`: `DefaultTicketProvider` no vacío e inválido → `fmt.Errorf("invalid defaultTicketProvider %q: allowed jira,linear,github,other", v)`. Vacío válido.
- [x] 1.4 Tests: Load/Resolve de ambos campos; inválido → error; omitido → OK; normalización de prefijos.

## 2. State (`cli/internal/state`)

- [x] 2.1 `TicketProvider.Valid()` (jira|linear|github|other) si no existe — molde `Status.Valid()`. `LinkSpec` sin cambios.

## 3. Detección (`cli/cmd/vector/ticket.go`)

- [x] 3.1 `ticketFromContext(content string, provider state.TicketProvider, prefixes []string) *state.Ticket` (molde `ticketFromProse`): cue words anclados (`Ticket`/`Issue`/`Ref`/`Tracking`/`Jira`/`Linear`/`GitHub`, con `>`/`**`, primera key `[A-Za-z][A-Za-z0-9]*-\d+` tras el cue) **O** prefijo conocido (`^<prefix>-\d+`) en cualquier parte; denylist built-in `ADR`/`RFC`; conflicto / 0 matches → `nil`. Retorna `Ticket{provider, key, url:"", auto:true}`.
- [x] 3.2 `detectTicket` recibe `provider` + `prefixes` y aplica `ticketFromContext` como tercer fallback, **después** de frontmatter y URL-prosa, **solo** si `provider != ""`.
- [x] 3.3 Regex compilados a nivel de paquete (`var … = regexp.MustCompile(...)`), como el resto de `ticket.go`.

## 4. Binario / threading

- [x] 4.1 `main.go` `runSync`: pasar `cfg.ResolvedDefaultTicketProvider()` + `cfg.NormalizedTicketKeyPrefixes()` a `detectTicket` (create y reconcile). `runSpecCreate` sin cambios.
- [x] 4.2 `spec_transitions.go` `runSpecLink`: añadir `root, err := resolveRepoRoot(*repoRoot)` y `cfg, err := config.Load(root)` (antes de `openStore`, propagar error); cuando `--provider` vacío y ref es key suelta, pasar `cfg.ResolvedDefaultTicketProvider()` como `forced` a `parseRef`.
- [x] 4.3 No cambiar el contrato JSON de salida ni el orden de precedencia existente.

## 5. Tests (`cli/cmd/vector`)

- [x] 5.1 `ticketFromContext`: cada cue word; prefijo conocido; denylist `ADR`/`RFC`; key suelta sin señal → nil; conflicto → nil; toma la primera key tras el cue e ignora `Epic`/`Story` de la línea.
- [x] 5.2 `detectTicket`: orden frontmatter > URL > cue/prefijo; sin provider → nil.
- [x] 5.3 `parseRef` / `runSpecLink`: key suelta + default → resuelve manual (`auto:false`); sin default → error accionable.
- [x] 5.4 Integración `runSync` con config fake en tempdir (provider + prefijos) → linkea `auto:true`.

## 6. Docs

- [x] 6.1 `docs/domain-contract.md` §5: ampliar la nota `auto`/`detectTicket` con cue words + prefijos + default provider y el link manual de key suelta.

## 7. Verificación

- [x] 7.1 `gofmt -l cli && go -C cli vet ./... && go -C cli test -race ./...` verdes.
- [x] 7.2 Smoke E2E: repo temporal con `defaultTicketProvider:jira` + `ticketKeyPrefixes:["MH"]` y un change con `Ticket: MH-1592` → `vector sync` linkea; `ADR-007` → no.
- [x] 7.3 Sin regresiones en `create`/`sync`/`propose`/`apply`/`serve`/`link`.
