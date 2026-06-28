# Design — spec-composer-subagent

## Decisiones clave

- **Model: Sonnet para el compositor.** La composición de 20 secciones a partir de un brief
  estructurado tiene complejidad equivalente a la del validator (también Sonnet). Haiku no
  alcanza la fidelidad necesaria y sería bloqueado por el gate adversarial. Opus es
  precisamente el tier a evitar — ese es el objetivo del cambio.

- **Compositor como generador puro.** Sin `AskUserQuestion`, sin llamadas al binario, sin
  acceso a `Bash`. Tools limitadas a `Read`, `Write`, `Glob`. Toda ambigüedad llega resuelta
  en `BRIEF` + `CLARIFICATIONS`; lo que no tiene evidencia se marca `TBD — ver Open
  questions`. Esto hace el compositor determinista y sin efectos secundarios más allá del
  archivo de output.

- **CLI-owns-writes, preservada.** El compositor escribe solo `OUTPUT_PATH`
  (`.vector/tmp/<id>/spec.md`), nunca `.vector/specs/<id>/state.json`. El binario sigue
  siendo el único escritor del estado. El caller provee la ruta; el compositor no la elige.

- **Tres copias sincronizadas (`kit/agents/`, `.claude/agents/`, `assets/agents/`).** Patrón
  establecido en `vector-spec-refiner`, `vector-spec-validator` y `vector-bug-refiner`. La
  copia en `assets/` se embebe vía `//go:embed all:assets` en `cli/internal/scaffold/`; es la
  que `vector init` siembra en repos de usuarios. Si las tres copias divergen, `vector init`
  siembra un agente stale — el test de scaffold detecta esta regresión.

- **`--body-file "$SPEC_PATH"` en lugar de heredoc.** `readBody()` en
  `cli/cmd/vector/main.go` ya soporta la rama `default: os.ReadFile(path)` (líneas 887–892)
  sin ningún cambio de binario. El spec no ocupa contexto del main loop Opus en los pasos
  posteriores (validator call, create call, report).

- **`OUTPUT_PATH` = `.vector/tmp/<id>/spec.md`.** Ruta predecible bajo el workspace de
  Vector, separada de la ubicación final del specDoc gestionada por el binario. Queda en disco
  como artefacto debuggable; la limpieza es TBD (gitignore de `.vector/tmp/` o limpieza
  explícita por el caller en V1 posterior).

- **Routing registrado en paso 10 con `EvtAgentRouted`.** El caller usa
  `vector spec route <id> --model sonnet --baseline opus --task "compose spec" ...` para
  appendear el evento al `activity.jsonl`. El tipo de evento (`EvtAgentRouted`) ya existe en
  `cli/internal/state/event.go`. Sin cambios de esquema.

- **Sin cambios al validator gate.** El compositor no hace auto-validación. La separación de
  responsabilidades es el principio: el compositor genera, el validator rechaza. El loop de
  re-validación (cap 3 ciclos) permanece igual; si el validator bloquea, el main loop puede
  re-invocar al compositor con el reporte del validator como `CLARIFICATIONS` adicionales.

## Interfaz del subagente

**Inputs** (prompt del Agent call):

| Campo | Descripción |
|---|---|
| `BRIEF` | Salida completa del refiner (raw o bug) |
| `CLARIFICATIONS` | Pares Q&A del paso 6, en orden |
| `TEMPLATE_PATH` | Ruta absoluta a `.claude/vector/spec-template.md` |
| `SPEC_EXAMPLE_PATH` | Ruta absoluta al ejemplo de spec, o `no example yet` |
| `SPEC_TITLE` | Título confirmado (≤ ~8 words) |
| `SPEC_ID` | Slug kebab-case confirmado |
| `SPEC_LANGUAGE` | `es` \| `en` |
| `OUTPUT_PATH` | Ruta absoluta donde el compositor escribe el archivo |

**Output** estructurado al caller:

```
Spec written to: <OUTPUT_PATH>
Sections: 20
TBD markers: <n>
```

## Flujo del pipeline de `/vector:raw` tras el cambio

`RAW_IDEA` → refiner (Haiku) → clarify → **compositor (Sonnet)** → validator (Sonnet) →
`vector spec create --body-file "$SPEC_PATH"` → routing (paso 10) → reporte al usuario.

El main loop no retiene el texto del spec — solo el path.
