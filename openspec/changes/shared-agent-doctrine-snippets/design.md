# Design — shared-agent-doctrine-snippets

## Decisiones clave

- **Extracción sin fusión**: los agentes mantienen identidades, esquemas de salida, tiers de
  modelo y listas de tools separados. Solo se extrae el texto duplicado. Fusionar cambiaría
  la interfaz de los agentes y rompería los commands que los invocan.
- **`kit/agents/_shared/` como ubicación**: el script `go generate` ya copia `kit/agents/**`
  recursivamente (`cp -R`); el subdirectorio `_shared/` queda incluido sin ningún cambio de
  infraestructura. Prefijo `_` señala que los ficheros no son agentes registrables directamente.
- **Sin frontmatter YAML en `_shared/`**: los ficheros son recursos de lectura planos, no
  agentes. Sin `name:`, `model:`, `tools:` evita que Claude Code los intente registrar como
  agentes invocables.
- **Tres ficheros de granularidad por doctrina** (`citation-discipline`, `prose-rules`,
  `refiner-base`): cada agente carga solo lo que necesita. Cargar un fichero monolítico sería
  impreciso (e.g., `vector-summary-writer` no necesita `citation-discipline`).
- **Mecanismo de directiva de carga — sección `## Shared doctrine` al inicio del agente**:
  se coloca antes del primer `## Hard rules` una sección con la instrucción explícita
  `Read .claude/agents/_shared/<file>.md before proceeding`. Es legible por humanos que
  revisen el fichero y ejecutada de forma fiable por el harness. (Open question #1 del spec.)
- **`priorSummary` permanece inline**: sus semánticas difieren entre `vector-summary-writer`
  (re-emite sustancia del prior summary) y `vector-standup-writer` (lo usa como contexto de
  framing); extraer sin resolver el delta crearía una fuente canónica incorrecta para uno de
  los dos. (Open question #2 del spec — TBD.)
- **"No inference of product intent" permanece inline**: la formulación difiere entre
  `vector-spec-refiner` (feature intent) y `vector-bug-refiner` (expected bug behavior); el
  delta es suficientemente relevante para no unificar sin análisis comparado. (Open question #3
  del spec — TBD.)
- **Fallback ante `_shared/` no encontrado — error visible**: si el fichero `_shared/` no
  existe en el repo del usuario (versión antigua sin `vector update`), el agente debe reportar
  el error de `Read` con el path esperado y sugerir `vector update`. Sin continuación silenciosa
  degradada. (Open question #4 del spec — TBD sobre mecanismo exacto según el harness.)
- **`TestSharedDoctrineNotInlined` como guarda de no-regresión**: verifica que los agentes
  modificados no contienen las strings canónicas extraídas; corre con `go test ./...` sin setup
  externo, leyendo desde el filesystem de test.

## Superficie de cambio

- `kit/agents/_shared/citation-discipline.md` — NUEVO: doctrina "Cite, don't guess"
  compartida por refiner / bug-refiner / validator / comment-evaluator.
- `kit/agents/_shared/prose-rules.md` — NUEVO: "Never invent work" + reglas de prosa
  humanizada compartidas por summary-writer / standup-writer.
- `kit/agents/_shared/refiner-base.md` — NUEVO: "Preserve user language" + "Be terse"
  compartidas por spec-refiner / bug-refiner.
- `kit/agents/vector-spec-refiner.md` — MODIFICAR: carga `citation-discipline` + `refiner-base`.
- `kit/agents/vector-bug-refiner.md` — MODIFICAR: carga `citation-discipline` + `refiner-base`.
- `kit/agents/vector-spec-validator.md` — MODIFICAR: carga `citation-discipline`.
- `kit/agents/vector-comment-evaluator.md` — MODIFICAR: carga `citation-discipline`.
- `kit/agents/vector-summary-writer.md` — MODIFICAR: carga `prose-rules`.
- `kit/agents/vector-standup-writer.md` — MODIFICAR: carga `prose-rules`.
- `cli/internal/scaffold/scaffold_test.go` — MODIFICAR: `TestSharedDoctrineNotInlined` +
  `TestSharedFilesExist`.

## Flujo de distribución

`kit/agents/_shared/*.md` editado → `go generate ./...` en `cli/internal/scaffold/` copia
`kit/agents/**` a `assets/agents/**` (incluyendo `_shared/`) → build del binario embebe
`assets/` completo → `vector update` en el repo del usuario siembra `.claude/agents/_shared/`
junto con los agentes actualizados.
