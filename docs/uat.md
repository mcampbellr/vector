# Vector — UAT (lo testeable hoy)

> Qué se puede aceptar manualmente en el estado actual. Crece a medida que aterrizan features.
> Estado: **slice 1** — `internal/state` + `vector spec create|list` + command `/vector:idea`.

## Build

```bash
cd /Users/mariocampbell/Developer/vector
go -C cli build -o bin/vector ./cmd/vector     # produce cli/bin/vector (gitignored)
./cli/bin/vector version                        # -> vector 0.0.1-dev
```

## UAT del binario (corre en un sandbox temporal, no ensucia ningún repo)

```bash
VEC="/Users/mariocampbell/Developer/vector/cli/bin/vector"
SANDBOX=$(mktemp -d); git -C "$SANDBOX" init -q

# 1. crear un spec con prioridad y cuerpo por stdin
"$VEC" spec create --title "New checkout flow" --repo demo --priority high \
  --body-file - --repo-root "$SANDBOX" <<'EOF'
# New checkout flow
## Problem / motivation
Users drop off at payment.
EOF

# 2. crear otro (id auto-derivado del título)
"$VEC" spec create --title "Implement SEO Best Practices" --repo-root "$SANDBOX"

# 3. ver el board
"$VEC" spec list --repo-root "$SANDBOX"

# 4. JSON para tooling
"$VEC" spec create --title "Daily Notes" --repo-root "$SANDBOX" --json

# 5. inspeccionar disco
find "$SANDBOX/.vector" -type f | sort
cat "$SANDBOX/.vector/local/activity.jsonl"

rm -rf "$SANDBOX"
```

### Criterios de aceptación

| # | Acción | Esperado |
|---|--------|----------|
| 1 | `spec create` válido | imprime `created spec "<id>" (status: open)`; crea `.vector/specs/<id>/state.json` |
| 2 | id auto | `new checkout flow` → id `new-checkout-flow` (kebab, slug del título) |
| 3 | `state.json` | `schemaVersion:1`, `status:"open"`, `priority` correcta, `createdAt==updatedAt` (UTC) |
| 4 | `spec.md` | se escribe solo si se pasó `--body-file` |
| 5 | `activity.jsonl` | 1 línea `spec.created` por spec, con `actor` (git user.name) y `data.template:"idea"` |
| 6 | `spec list` | una fila por spec: `id  status  priority  title`, ordenado por id |
| 7 | duplicado | `spec create` del mismo id **falla** con exit≠0 y mensaje claro |
| 8 | id inválido | `--id "Not Kebab"` **falla** (debe ser kebab-case) |
| 9 | prioridad inválida | `--priority foo` **falla** |
| 10 | `--json` | imprime `{id,status,state}` parseable |

### Gate de calidad (dev)

```bash
gofmt -l cli            # vacío = ok
go -C cli vet ./...     # sin warnings
go -C cli test ./...    # verde
```

## UAT de `vector init` (siembra de commands per-proyecto)

Modelo OpenSpec: binario **global** + commands **per-proyecto** sembrados por `vector init`.

1. **Binario en PATH** (global):
   ```bash
   go -C cli generate ./internal/scaffold/    # sync kit/commands -> assets embebidos
   go -C cli build -o ~/.local/bin/vector ./cmd/vector
   vector version   # -> vector 0.0.1-dev
   ```
2. **Sembrar en el repo objetivo**:
   ```bash
   vector init --repo-root <repo>       # o, dentro del repo: vector init
   ```
   Tras sembrar, `/reload-plugins` o reiniciar la sesión para que el palette muestre `/vector:idea`.

### Criterios de aceptación (init)

| # | Acción | Esperado |
|---|--------|----------|
| 1 | `vector init` | crea `.claude/commands/vector/idea.md` (`created`) + esqueleto `.vector/` |
| 2 | `--dry-run` | reporta `created` pero **no escribe** nada |
| 3 | re-`init` | `skipped` (no sobrescribe) |
| 4 | command editado por el usuario + re-`init` | `skipped`; el contenido del usuario se **respeta** |
| 5 | `--force` | `overwritten` |
| 6 | archivos ajenos en `.claude/` (settings, CLAUDE.md, otros commands) | **intactos** |
| 7 | `--json` | `{root, dryRun, files:[{path,action}]}` parseable |

## UAT del command `/vector:idea`

3. **Usar**: invocar `/vector:idea <idea>` → el command refina el texto y llama
   `vector spec create …`; verificar que aparece en `vector spec list` y en
   `.vector/specs/<id>/state.json`.

> El palette muestra `/vector:idea` entero (project command con namespace por subdirectorio),
> no `/raw (vector)`. Sin plugin ni marketplace.

## UAT de standup-digest (release `release-standup-digest`, 2026-06-25)

Cierre de la feature `add-standup-digest`: re-embed del build en el binario, reinstalación, y UAT
exhaustivo. Binario recompilado e instalado en `~/.local/bin/vector` con la SPA embebida y los
subcomandos `standup`/`worklog`.

| # | Caso | Esperado | Resultado |
|---|------|----------|-----------|
| 1 | `npm --prefix web run build` + copia a `cli/internal/webui/dist/` | embed con `index.html` + `assets/*` de producción, sin fuentes/maps/secrets | ✅ |
| 2 | `go -C cli build -o ~/.local/bin/vector ./cmd/vector` + `vector version` | binario nuevo en PATH con `standup`/`worklog` en el help | ✅ |
| 3 | `vector serve` | loguea board/api/events; sirve **embedded** (sin línea `ui: … stale`) | ✅ |
| 4 | `GET /` + `/assets/index-*.js` | index referencia el bundle embebido; bundle contiene la StandupView (`no activity since last standup`) | ✅ |
| 5 | `GET /api/standup` (nunca corrido) | `{}` (200) | ✅ |
| 6 | `GET /api/board` | 200 (sin regresión) | ✅ |
| 7 | `GET /api/activity?spec=add-standup-digest&since=7d` | 200 con timeline (`spec.created`/`status.changed`/`work.logged`) | ✅ |
| 8 | `GET /api/activity?...&since=36h` | `400 {"error":"invalid since: use 24h, today or 7d"}` | ✅ |
| 9 | `GET /api/activity?spec=ghost` | `404 {"error":"spec \"ghost\" not found"}` | ✅ |
| 10 | `vector standup --json` | proyección por spec desde el marcador (5 specs) | ✅ |
| 11 | digest → `vector standup commit --digest-file -` | persiste `.vector/local/standup.json`, avanza marcador; `GET /api/standup` lo refleja | ✅ |
| 12 | `echo 'not json' \| vector standup commit --digest-file -` | `invalid digest json`; **no escribe** ni **avanza marcador** (sha idéntico) | ✅ |
| 13 | `vector standup` tras commit (periodo vacío) | "no activity since `<marcador>`" | ✅ |

**Pendiente de confirmación humana (visual):** abrir el board en el navegador y verificar a ojo el
tab **Standup**, el **SpecTimeline** expandible por card, y los estados loading/success/empty/error
con **retry**. El data layer que los alimenta está verificado arriba (200/`{}`/400/404) y los
labels están en el bundle embebido; falta solo el render visual.

**Nota de sesión:** el command `/vector:standup` y el agente `vector-standup-writer` se sembraron en
`.claude/` con `vector update`; requieren `/reload-plugins` (o reiniciar la sesión) para ser
invocables en una sesión de Claude Code ya abierta.

## Todavía NO testeable (no implementado)

- Detección/reorg de repo + backup/consent en `init` (pregunta abierta; hoy `init` solo siembra).
- Transiciones restantes del contrato: `/vector:link`, `:daily`.
- `install.sh` (instalación day-0).
