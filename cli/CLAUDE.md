# Workspace `cli/` — Manifest

> Carpeta scaffoldeada. **Sin código aún.** Este manifest declara el rol del workspace y
> enlaza las rules relevantes; no las duplica.

## Rol

Módulo Go único que produce **el binario de Vector**: los comandos del CLI (`/vector init`,
`/vector:raw [text]`), la **API HTTP del board** y el servidor que sirve el panel web
embebido. Es el único que lee/escribe el **JSON de estado**.

## Stack

- Go (módulo único). Layout `cmd/` + `internal/`. Frontend de `web/` embebido vía `embed.FS`.

## Depende de / es dependido por

- **Embebe** los assets buildados de `web/` (ver `architecture/distribution-packaging.md`).
- Expone la **API HTTP** que `web/` consume (contrato versionado).
- Puede leer/copiar `kit/` durante `/vector init`. No importa código de `kit/`.

## Rules aplicables (`.claude/rules/`)

- `standards/go-conventions.md` — estilo, layout, errores, deps.
- `standards/naming.md` — comandos/flags kebab-case, IDs de dominio.
- `architecture/system-boundaries.md` — ownership y dependencias.
- `architecture/state-model.md` — el JSON como fuente de verdad (este workspace lo posee).
- `architecture/distribution-packaging.md` — un binario, instalación día 0, embed.
- `security/destructive-ops-consent.md` — backup + permiso antes de tocar el repo del usuario.
- `workflows/state-sync-discipline.md` — mantener el JSON up-to-date en cada acción de dominio.
- `product/domain-model.md` — vocabulario (spec, estado, etapa, prioridad, ticket).
- `quality/testing-and-review.md` — tests del estado/API; gate de calidad.
- `workflows/git-convention.md` — convención git del repo.
