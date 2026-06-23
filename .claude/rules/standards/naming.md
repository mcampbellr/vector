# Standards — Naming

> Aplica a: todo el repo. Convenciones de nombres que cruzan workspaces.

## Comandos, skills y flags (kit + cli)

- **kebab-case** para skills, comandos y flags de cara al usuario: `vector-init`,
  `/vector:raw`, `--dry-run`. Sin camelCase, snake_case ni espacios.
- Comandos del CLI con namespace `vector` (`/vector init`, `/vector:raw [text]`). Mantener la
  nomenclatura del `docs/vision.md` hasta que se decida lo contrario.

## Identificadores de dominio

- IDs de specs/proyectos: estables, URL-safe y legibles. Definir el esquema junto al del JSON
  de estado (`architecture/state-model.md`).
- Estados y etapas se nombran en minúscula sin espacios (`todo`, `progress`, `review`,
  `done`); la presentación (uppercase en pills) es responsabilidad del frontend.

## Código

- **Go**: idiomático — `PascalCase` exportado, `camelCase` interno, paquetes en minúscula sin
  guiones. Ver `standards/go-conventions.md`.
- **TS/React**: componentes `PascalCase`, hooks `useXxx`, archivos = nombre del componente.
  Ver `standards/typescript-react.md`.

## Git

- Branches en inglés y kebab-case (ver `workflows/git-convention.md` y el global del usuario).

> Estado: pendiente — esquema final de IDs de spec/proyecto (depende del JSON de estado).
