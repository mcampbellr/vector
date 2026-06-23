# Vector — Plugin de Claude y comandos (LOCKED)

> Cómo se materializan los comandos `/vector:*` y cómo se distribuyen. Verificado contra docs
> oficiales de Claude Code (plugins/skills) y la lista de skills de la sesión
> (`marketing-skills:…`, `banana-claude:…` son plugins namespaced).

## Dos superficies distintas (no confundir)

| Superficie | Qué es | Ejemplos |
|------------|--------|----------|
| **Binario Go** (`cli/`) | Comando de terminal; **único escritor** del state | `vector serve`, `vector init`, `vector spec create …` |
| **Plugin de Claude** (`kit/`) | Skills namespaced que se invocan dentro de Claude y **llaman al binario** | `/vector:raw`, `/vector:status`, … |

Los skills del plugin **nunca** editan el JSON directamente: invocan al binario (disciplina
CLI-owns-writes, ver `architecture/state-model.md`).

## El colon = plugin namespace

- `/vector:raw` requiere un **plugin llamado `vector`** (no se logra con subdirectorios de
  `.claude/commands/`). El nombre del plugin (en `plugin.json`) = parte antes del colon; el
  folder del skill = parte después.
- **Un solo nivel de colon**: `/vector:raw` ✅, `/vector:spec:new` ❌.
- Nombre del plugin: estilo npm (minúscula, guiones). Skill tras el colon: kebab-case.

## Decisión: TODOS los comandos bajo el namespace `vector`

Unificado (no se mezcla `/vector init` con espacio):

`/vector:init` · `/vector:raw` · `/vector:link` · `/vector:status` · `/vector:daily` ·
`/vector:apply` · `/vector:close` · `/vector:archive`

(El mapa de qué escribe cada uno en el state está en `docs/domain-contract.md` §5.)

## Layout del plugin (= el `kit/`)

```
kit/                              # fuente del plugin, versionada en el repo Vector
├── .claude-plugin/plugin.json    # { "name": "vector", "version": "…" }
└── skills/
    ├── init/SKILL.md             # → /vector:init
    ├── raw/SKILL.md              # → /vector:raw  (template ≈ /idea)
    ├── link/SKILL.md
    ├── status/SKILL.md
    ├── daily/SKILL.md
    ├── apply/SKILL.md
    ├── close/SKILL.md
    └── archive/SKILL.md
```

- Cada `SKILL.md` lleva `description` para auto-invocación por relevancia (como `/idea`), y su
  cuerpo orquesta llamadas al binario `vector`.

## Distribución (V1): install script registra el plugin local

- El `curl … | install.sh` (ver `architecture/distribution-packaging.md`):
  1. instala el **binario Go** (`vector`),
  2. registra el **plugin `vector`** en el `.claude/` del repo del usuario (copia/enlaza
     `kit/`), de forma reproducible.
- Un solo paso, sin depender de un marketplace.
- **Marketplace de plugins de Claude** = canal adicional opcional más adelante (updates
  versionados, `/plugin install vector`); no requerido para V1.

> Pendiente: forma exacta del registro del plugin por el install script (copia vs symlink vs
> marketplace local) — se define al implementar el packaging.
