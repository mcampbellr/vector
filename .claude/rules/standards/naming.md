# Standards — Naming

> Aplica a: todo el repo. Convenciones de nombres que cruzan workspaces.

## Comandos, skills y flags (kit + cli)

- **kebab-case** para skills, comandos y flags de cara al usuario: `vector-init`,
  `/vector:raw`, `--dry-run`. Sin camelCase, snake_case ni espacios.
- Los comandos de Claude son **project commands** en `kit/commands/vector/*.md` → el
  subdirectorio `vector/` da el namespace con colon, un solo nivel: `/vector:raw`,
  `/vector:status`, … (ver `docs/plugin-and-commands.md`). El binario de terminal
  (`vector init`, `vector serve`, etc.) es superficie aparte — `init` siembra los slash commands,
  no es uno de ellos.

## Identificadores de dominio

- IDs de specs/proyectos: **slug kebab-case**, estables, URL-safe y legibles; == nombre del
  change de OpenSpec al aplicar (ver `docs/schemas/state-and-activity.md`).
- Estados en kebab-case: `open`, `in-progress`, `needs-attention`, `review`, `closed`,
  `archived` (ver `docs/domain-contract.md`); la presentación (uppercase en pills) es
  responsabilidad del frontend.

## Código

- **Go**: idiomático — `PascalCase` exportado, `camelCase` interno, paquetes en minúscula sin
  guiones. Ver `standards/go-conventions.md`.
- **TS/React**: componentes `PascalCase`, hooks `useXxx`, archivos = nombre del componente.
  Ver `standards/typescript-react.md`.

## Git

- Branches en inglés y kebab-case (ver `workflows/git-convention.md` y el global del usuario).

> Resuelto: IDs = slug kebab-case (ver `docs/schemas/state-and-activity.md` y `docs/domain-contract.md`).
