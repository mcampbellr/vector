# Vector — Manifest global

Vector es una herramienta **developer-focused** que trabaja en conjunto con Claude Code para
organizar proyectos (specs sobre OpenSpec en un board kanban), **agnóstica al código del
usuario**, con foco en **eficiencia de tokens** (ruteo a agentes baratos para tareas
triviales) y **comercializable desde el día 0**. Ver `docs/vision.md`.

> Estado del repo: captura inicial de la visión. **Sin código aún.** Las rules que dependen
> de decisiones abiertas (formato de `/vector init`, esquema del JSON de estado, columnas del
> board) están marcadas como `pendiente`.

## Cómo está organizado este sistema de instrucciones

Arquitectura **scoped**: este manifest es pequeño y solo contiene lo que aplica a TODO el
repo. El conocimiento por concern vive en `.claude/rules/`. Cada workspace (`cli/`, `web/`,
`kit/`) tiene su propio `CLAUDE.md` que **referencia** las rules relevantes, sin duplicarlas.

- **Empieza por el manifest del workspace** en el que trabajas, no por este archivo.
- Lee una rule solo cuando su "cuándo aplica" coincide con tu tarea.
- No dupliques: si algo ya está en una rule o en el global del usuario, enlaza en vez de
  reescribir.

## Herencia

Este repo hereda el global del usuario (`~/.claude/CLAUDE.md`: concisión, strong typing, git
en inglés, one-component-per-file, propuestas cortas, etc.). **No** se redefine aquí. Vector
solo añade lo propio. (Nota: el `CLAUDE.md` del directorio padre `Developer/` es de Flagify y
no aplica a Vector salvo que se trabaje explícitamente con Flagify.)

## Principios cross-repo (no derivables del código)

- **Agnóstico al código del usuario**: Vector aporta estructura de manejo con agentes; nunca
  impone arquitectura sobre el repo del usuario.
- **Eficiencia de tokens** como requisito de primera clase, no optimización tardía.
- **Comercialización desde el día 0**: cada decisión considera distribución/instalación.
- **El JSON de estado es la única fuente de verdad** del board; mantenerlo sincronizado es
  obligatorio tras cada acción que lo afecte.

## Índice de rules (`.claude/rules/`)

### architecture/
- `system-boundaries.md` — límites cli ↔ web ↔ kit y quién posee qué. *Aplica a: cambios que
  cruzan workspaces o definen ownership.*
- `state-model.md` — el JSON de estado/record como fuente de verdad del board. *Aplica a:
  cualquier feature que lea/escriba estado.*
- `distribution-packaging.md` — embed del frontend en el binario Go; instalación día 0.
  *Aplica a: build, release, instalación.*

### standards/
- `go-conventions.md` — estilo Go. *Aplica a: `cli/`.*
- `typescript-react.md` — React/Next del board. *Aplica a: `web/`.*
- `naming.md` — kebab-case de skills/comandos/flags, convención de IDs. *Aplica a: todo el repo.*

### product/
- `principles.md` — developer-focused, agnóstico, comercial día 0. *Aplica a: decisiones de producto/UX.*
- `token-routing.md` — ruteo a agentes baratos vs caros. *Aplica a: diseño de skills/agentes del kit.*
- `domain-model.md` — OpenSpec + kanban (spec, estado, etapa, prioridad, ticket). *Aplica a: cli/web/kit.*

### security/
- `destructive-ops-consent.md` — backup + permiso explícito antes de reorganizar repos del
  usuario. *Aplica a: cualquier operación que escriba/mueva archivos del usuario.*

### quality/
- `testing-and-review.md` — expectativas de tests (Go + web) y review. *Aplica a: todo el repo.*

### workflows/
- `git-convention.md` — commits/PR/branches del repo Vector. *Aplica a: todo git.*
- `state-sync-discipline.md` — recordatorio de mantener el JSON up-to-date. *Aplica a: rules/skills que tocan estado.*

### documentation/
- `docs-standards.md` — rol de `docs/` y evolución de specs. *Aplica a: cambios de documentación.*
