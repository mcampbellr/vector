# Vector — UAT (lo testeable hoy)

> Qué se puede aceptar manualmente en el estado actual. Crece a medida que aterrizan features.
> Estado: **slice 1** — `internal/state` + `vector spec create|list` + command `/vector:raw`.

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
   Tras sembrar, `/reload-plugins` o reiniciar la sesión para que el palette muestre `/vector:raw`.

### Criterios de aceptación (init)

| # | Acción | Esperado |
|---|--------|----------|
| 1 | `vector init` | crea `.claude/commands/vector/raw.md` (`created`) + esqueleto `.vector/` |
| 2 | `--dry-run` | reporta `created` pero **no escribe** nada |
| 3 | re-`init` | `skipped` (no sobrescribe) |
| 4 | command editado por el usuario + re-`init` | `skipped`; el contenido del usuario se **respeta** |
| 5 | `--force` | `overwritten` |
| 6 | archivos ajenos en `.claude/` (settings, CLAUDE.md, otros commands) | **intactos** |
| 7 | `--json` | `{root, dryRun, files:[{path,action}]}` parseable |

## UAT del command `/vector:raw`

3. **Usar**: invocar `/vector:raw <idea>` → el command refina el texto y llama
   `vector spec create …`; verificar que aparece en `vector spec list` y en
   `.vector/specs/<id>/state.json`.

> El palette muestra `/vector:raw` entero (project command con namespace por subdirectorio),
> no `/raw (vector)`. Sin plugin ni marketplace.

## Todavía NO testeable (no implementado)

- `vector serve` / panel web / board (sin API ni SSE aún).
- Detección/reorg de repo + backup/consent en `init` (pregunta abierta; hoy `init` solo siembra).
- Transiciones: `/vector:status`, `:link`, `:apply`, `:close`, `:archive`, `:daily`.
- `install.sh` (instalación day-0).
