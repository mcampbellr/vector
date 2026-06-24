# Vector — Comandos `/vector:*` y distribución (LOCKED)

> Cómo se materializan los comandos `/vector:*` y cómo se instalan por proyecto. Verificado
> empíricamente contra cómo lo hace **OpenSpec** (`opsx`) en el sistema: comandos en
> subdirectorio de `.claude/commands/`, **no** un plugin.

## Dos superficies distintas (no confundir)

| Superficie | Qué es | Ejemplos |
|------------|--------|----------|
| **Binario Go** (`cli/`) | Comando de terminal; **único escritor** del state. Global (uno en el `PATH`). | `vector serve`, `vector init`, `vector spec create …` |
| **Commands de Claude** (`kit/`) | Markdown invocado dentro de Claude que **llama al binario**. Per-proyecto, en `.claude/commands/vector/`. | `/vector:raw`, `/vector:status`, … |

Los commands **nunca** editan el JSON directamente: invocan al binario (disciplina
CLI-owns-writes, ver `architecture/state-model.md`).

## El colon viene del **subdirectorio**, no de un plugin

Decisión corregida (la versión previa de este doc asumía que el colon requería un plugin; es
falso). Claude Code namespacea los **project commands** por el subdirectorio bajo
`.claude/commands/`:

- `.claude/commands/vector/raw.md` → se invoca y **se muestra en el palette** como `/vector:raw`.
- El subdirectorio (`vector`) = parte antes del colon; el nombre del archivo (`raw`) = parte
  después. **Un solo nivel**: `/vector:raw` ✅, `/vector:spec:new` ❌.
- El palette muestra `/vector:raw` entero, **sin tag de plugin** (a diferencia de un skill de
  plugin, que se ve `/raw (vector)`). Esto replica el look de `opsx` (`/opsx:apply`, `:propose`, …).

> Por qué commands y no skills de plugin: queremos invocación **explícita** `/vector:*` con el
> namespace visible, igual que OpenSpec. Trade-off aceptado: los project commands no se
> auto-invocan por relevancia (los skills de plugin sí). Para una herramienta de specs, la
> invocación explícita es la correcta.

## Formato de cada command

Cada archivo es markdown con frontmatter + cuerpo-prompt (patrón observado en `opsx`):

```markdown
---
name: "Vector: Raw"
description: Turn a raw idea into a structured Vector spec and register it (status open).
category: Workflow
tags: [vector, spec, capture]
---

<instrucciones para Claude: refinar $ARGUMENTS y llamar al binario `vector …`>
```

- `description` aparece en el palette. `$ARGUMENTS` recibe el texto tras `/vector:raw …`.
- El cuerpo orquesta llamadas al binario `vector`; nunca escribe `.vector/` a mano.

## Decisión: los slash commands bajo el namespace `vector`

`/vector:raw` · `/vector:link` · `/vector:status` · `/vector:daily` · `/vector:apply` ·
`/vector:close` · `/vector:archive`

`init` queda **fuera** de los slash commands: es el subcomando de terminal `vector init` que
bootstrapea el repo y siembra los de arriba (ver §Distribución).

(El mapa de qué escribe cada uno en el state está en `docs/domain-contract.md` §5.)

## Layout (= el `kit/`)

```
kit/                              # fuente versionada en el repo Vector
├── CLAUDE.md
└── commands/
    └── vector/                   # el subdirectorio = namespace del colon
        ├── raw.md                # → /vector:raw  (template ≈ /idea)
        ├── link.md
        ├── status.md
        ├── daily.md
        ├── apply.md
        ├── close.md
        └── archive.md
```

`kit/commands/vector/` es la **fuente distribuible**: el binario la embebe (`embed.FS`) y
`vector init` la escribe en `<repo>/.claude/commands/vector/`. (`init` no vive aquí: es
subcomando del binario, no un slash command.)

## Distribución (V1): instalación **por proyecto**, igual que OpenSpec

Dos artefactos con ciclo de vida distinto:

1. **Binario `vector`** — **global**, una sola vez en el `PATH` (`install.sh` / `go install` /
   brew más adelante). Compartido por todos los repos.
2. **Commands `/vector:*`** — **per-proyecto**, sembrados por el **binario** `vector init`
   (subcomando de terminal, análogo a `openspec init`). El binario **embebe** los commands
   (`kit/commands/vector/*.md` vía `embed.FS`, igual que los assets de `web/`) y los escribe en
   `<repo>/.claude/commands/vector/` del repo del usuario, bajo las salvaguardas de
   `security/destructive-ops-consent.md`. El state `.vector/` también es per-repo.

- **`init` no es un slash command**: es el subcomando de terminal que bootstrapea el repo. No
  puede ser `/vector:init` porque los slash commands no existen hasta que `vector init` los
  siembra (mismo patrón que `openspec init`). Los `/vector:*` son lo que `init` **produce**.
- La fuente versionada `kit/commands/vector/` se compila dentro del binario; en la máquina del
  usuario no hace falta `kit/`. Re-correr `vector init` re-siembra los commands desde la versión
  del binario.
- Sin plugin, sin marketplace, sin `/plugin install`. Un repo "tiene Vector" cuando existen sus
  `.claude/commands/vector/` + (idealmente) `.vector/`.

## Dogfooding en este mismo repo

Vector se usa a sí mismo: `.claude/commands/vector/raw.md` es la copia instalada en este repo
(idéntica a la fuente `kit/commands/vector/raw.md`). Pasos en `docs/uat.md`.

> Pendiente: que `vector init` realice la siembra de commands (hoy la copia es manual);
> versionado de los commands frente a la versión del binario.
