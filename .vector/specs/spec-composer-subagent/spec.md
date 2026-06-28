# Spec: Subagente `vector-spec-composer` (composición de spec en Sonnet)

## 1. Objetivo

Construir el subagente `vector-spec-composer` (`kit/agents/vector-spec-composer.md`) que
recibe el brief estructurado del refiner + las respuestas de clarificación + los metadatos
del spec y **compone las 20 secciones**, escribiendo el resultado a disco antes de que el
caller invoque `vector spec create`. Modificar `/vector:raw` y `/vector:bug` para delegar su
paso de composición a este nuevo subagente.

Esta feature permite que `/vector:raw` y `/vector:bug` saquen la generación central del spec
(~8–15 k tokens de output) del tier Opus del main loop, enrutándola al tier Sonnet y dejando
al loop principal solo con la orquestación (preguntas, llamadas al binario). El spec compuesto
queda persistido en disco antes del `vector spec create`, recuperable ante un crash.

## 2. Alcance

### Incluido en esta fase

- Nuevo **subagente del kit** `vector-spec-composer` (`kit/agents/vector-spec-composer.md`,
  model: `sonnet`, tools: `Read`, `Write`, `Glob`).
- **Sincronización de copias** del agente en las tres ubicaciones canónicas:
  `.claude/agents/vector-spec-composer.md` y
  `cli/internal/scaffold/assets/agents/vector-spec-composer.md` (vendored vía `go generate`,
  embebido por `//go:embed all:assets` en `cli/internal/scaffold/`).
- **Modificación de `kit/commands/vector/raw.md`**: paso 7 reemplazado por llamada al
  subagente; paso 9 usa `--body-file "$SPEC_PATH"` en lugar de heredoc stdin; paso 10 agrega
  registro de routing del compositor.
- **Modificación de `kit/commands/vector/bug.md`**: mismos cambios a los pasos 7, 9 y 10.
- **Actualización de `cli/internal/scaffold/scaffold_test.go`**: nuevo test que verifica que
  `vector init` siembra `vector-spec-composer` junto con los demás agentes.

### Fuera de scope

- Cambios a `vector-spec-refiner` o `vector-spec-validator` (sin modificar).
- Cambios al binario Go (`cli/cmd/vector/`, `cli/internal/state/`): no hay nuevos subcomandos
  ni campos de estado.
- Web panel: no afecta ninguna proyección de UI.
- Otros commands del kit (`/vector:propose`, `/vector:apply`, `/vector:status`, etc.).
- Limpieza automática del archivo temporal escrito por el compositor (queda en disco como
  artefacto debuggable; el caller puede eliminarlo, pero no es obligatorio en V1).

El agente no implementa nada fuera de este alcance aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- **Subagente kit**: Markdown + frontmatter YAML, idéntico a los agentes hermanos en
  `kit/agents/` (`vector-spec-refiner.md`, `vector-spec-validator.md`, `vector-bug-refiner.md`).
  No hay runtime Go ni TS en el agente — es un artefacto de instrucción para Claude Code.
- **Project commands**: Markdown + frontmatter YAML, idéntico a `kit/commands/vector/raw.md`
  y `kit/commands/vector/bug.md`.
- **Scaffold/embed (Go)**: el agente vendorizado vive en
  `cli/internal/scaffold/assets/agents/` y es embebido por el bloque `//go:embed all:assets`
  en `cli/internal/scaffold/scaffold.go`. Se sincroniza vía `go generate` (mismo mecanismo
  que el resto de agentes del kit). Ver `cli/internal/scaffold/scaffold.go`.

### Versiones relevantes

- Go: `1.26` (citado de `cli/go.mod`).
- Modelo del compositor: `sonnet` (declarado en el frontmatter del agente; equivale al modelo
  de `vector-spec-validator`, ver `kit/agents/vector-spec-validator.md` línea 4).
- Sin dependencias externas nuevas — el embed usa la stdlib Go estándar.

### Patrones existentes a respetar

- **Frontmatter de agentes del kit**: `name`, `description`, `model`, `tools` (ver
  `kit/agents/vector-spec-refiner.md` líneas 1–6 y `kit/agents/vector-spec-validator.md`
  líneas 1–6). El compositor sigue el mismo esquema.
- **Tres ubicaciones sincronizadas**: cada agente del kit vive en `kit/agents/`,
  `.claude/agents/` y `cli/internal/scaffold/assets/agents/`. Los tres archivos tienen el
  mismo contenido. Ver la evidencia en el listado de `cli/internal/scaffold/assets/agents/`.
- **CLI-owns-writes**: el compositor escribe un archivo temporal (el spec doc); el binario es
  el único que escribe el estado (`.vector/specs/<id>/state.json`). El compositor **nunca**
  llama al binario ni al shell.
- **Token routing**: el caller registra el routing del compositor vía
  `vector spec route <id> --model sonnet --baseline opus --task "compose spec" ...`, usando el
  mismo patrón que el refiner (haiku) y el validator (sonnet). Ver `kit/commands/vector/raw.md`
  paso 10.
- **Sin `AskUserQuestion` en el compositor**: todas las clarificaciones ocurren en el main
  loop antes de invocarlo. El compositor es un generador determinista puro.
- **IDs y slugs en kebab-case inglés**; cuerpo del spec en el idioma detectado del proyecto.

---

## 4. Dependencias previas

Antes de implementar esta fase debe existir o estar completado:

- [x] `kit/agents/vector-spec-refiner.md` — el productor del `BRIEF` que el compositor
      consume (ya existe: `kit/agents/vector-spec-refiner.md`).
- [x] `kit/agents/vector-spec-validator.md` — el gate adversarial posterior al compositor
      (ya existe: `kit/agents/vector-spec-validator.md`). No se modifica.
- [x] `kit/commands/vector/raw.md` con paso 7 de composición inline (base sobre la que se
      aplica el cambio; ya existe).
- [x] `kit/commands/vector/bug.md` con paso 7 de composición inline (idem).
- [x] `.claude/vector/spec-template.md` — plantilla de 20 secciones que el compositor lee
      (ya existe: verificado en el sistema de instrucciones).
- [x] `cli/internal/scaffold/scaffold.go` con el bloque `//go:embed all:assets` y el
      mecanismo de `go generate` para vendorizar (ya existe; patrón visible en el listado de
      `assets/agents/`).
- [x] `cli/cmd/vector/main.go` `readBody()`: ya acepta ruta de archivo en la rama `default`
      (`os.ReadFile(path)`, líneas 887–892), sin cambios de binario necesarios.
- [ ] Gitignore o convención de limpieza para `.vector/tmp/` — TBD, ver Open questions §1.

Si alguna dependencia no existe, el agente debe detenerse y reportar qué falta. No debe
inventar contratos.

---

## 5. Arquitectura

### Patrón a usar

**CLI-owns-writes** con subagente de generación. El main loop orquesta; el compositor genera
y escribe **solo el spec doc temporal**; el binario persiste el estado. El compositor es un
nodo del pipeline de agentes baratos, no un actor de dominio.

### Capas afectadas

- **Kit agents** (`kit/agents/`): sí — nuevo `vector-spec-composer.md`.
- **Kit commands** (`kit/commands/vector/`): sí — `raw.md` y `bug.md` modificados (pasos
  7, 9, 10). No se tocan otros commands.
- **Scaffold assets** (`cli/internal/scaffold/assets/agents/`): sí — nuevo archivo vendorizado.
- **Tests de scaffold** (`cli/internal/scaffold/scaffold_test.go`): sí — nuevo test.
- **Binario Go** (`cli/cmd/vector/`, `cli/internal/state/`): **no** — `readBody` ya soporta
  rutas de archivo; no hay nuevos subcomandos ni campos de estado.
- **Web** (`web/`): **no**.
- **`.claude/agents/`**: sí — copia sincronizada del agente (sembrada por `vector init`).

### Flujo esperado (pipeline de `/vector:raw` tras el cambio)

1. Main loop lee `RAW_IDEA` (paso 1).
2. Main loop detecta lenguaje, encuentra ejemplo de spec (pasos 3–4).
3. Main loop invoca **`vector-spec-refiner`** (Haiku) → devuelve `BRIEF` incluye título
   propuesto, id (slug), 20-section scaffold (paso 5).
4. Main loop clarifica con el usuario vía `AskUserQuestion` hasta resolver ambigüedad (paso 6).
5. Main loop deriva `SPEC_TITLE`, `SPEC_ID`, `SPEC_LANGUAGE` del `BRIEF` + clarificaciones;
   detecta ticket; asigna prioridad (paso 7, partes a–c, inline, cheap).
6. Main loop invoca **`vector-spec-composer`** (Sonnet) con `BRIEF`, `CLARIFICATIONS`,
   `TEMPLATE_PATH`, `SPEC_EXAMPLE_PATH`, `SPEC_TITLE`, `SPEC_ID`, `SPEC_LANGUAGE`,
   `OUTPUT_PATH` (`.vector/tmp/<id>/spec.md`). El compositor escribe el spec a
   `OUTPUT_PATH` y devuelve confirmación con la ruta. Main loop guarda `SPEC_PATH` (paso 7d).
   **El main loop no retiene el texto del spec en su contexto.**
7. Main loop invoca **`vector-spec-validator`** (Sonnet) pasando `SPEC_PATH` (ya en disco)
   → veredicto (paso 8). El gate adversarial (cap 3 ciclos) no cambia.
8. Main loop llama `vector spec create ... --body-file "$SPEC_PATH" --json` (paso 9).
9. Main loop registra routing de refiner, compositor y validator via `vector spec route`
   (paso 10).
10. Main loop reporta al usuario (paso 11).

### Ubicación de archivos nuevos

```
kit/
  agents/
    vector-spec-composer.md       ← NUEVO (fuente de verdad del agente)
.claude/
  agents/
    vector-spec-composer.md       ← NUEVO (copia sincronizada, sembrada por init)
cli/
  internal/
    scaffold/
      assets/
        agents/
          vector-spec-composer.md ← NUEVO (vendorizado vía go generate, embebido)
      scaffold_test.go            ← MODIFICAR (nuevo test)
kit/
  commands/
    vector/
      raw.md                      ← MODIFICAR (pasos 7, 9, 10)
      bug.md                      ← MODIFICAR (pasos 7, 9, 10)
```

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `kit/agents/vector-spec-composer.md` | NUEVO | Definición del subagente compositor (Sonnet, Read+Write+Glob) | `kit/agents/vector-spec-refiner.md` |
| `.claude/agents/vector-spec-composer.md` | NUEVO | Copia sincronizada sembrada por `vector init`/`update` | `.claude/agents/vector-spec-refiner.md` |
| `cli/internal/scaffold/assets/agents/vector-spec-composer.md` | NUEVO | Copia vendorizada (embed) vía `go generate` | `cli/internal/scaffold/assets/agents/vector-spec-refiner.md` |
| `cli/internal/scaffold/scaffold_test.go` | MODIFICAR | Agregar test que verifica siembra del compositor | `cli/internal/scaffold/scaffold_test.go` (test `TestSeedCommandsSeedsBugCommandAndRefiner`) |
| `kit/commands/vector/raw.md` | MODIFICAR | Pasos 7, 9 y 10: delegar composición al subagente | `kit/commands/vector/raw.md` (actual) |
| `kit/commands/vector/bug.md` | MODIFICAR | Pasos 7, 9 y 10: delegar composición al subagente | `kit/commands/vector/bug.md` (actual) |

### Detalle por archivo

#### `kit/agents/vector-spec-composer.md` — NUEVO

Acción: NUEVO

Frontmatter:
```yaml
---
name: vector-spec-composer
description: >
  Composes a complete 20-section Vector spec from a structured refiner brief and user
  clarifications. Writes the result to a file path provided by the caller. Pure composer —
  asks no questions, calls no binaries.
model: sonnet
tools: Read, Write, Glob
---
```

Debe implementar:
- **Leer** el template en `TEMPLATE_PATH` y el ejemplo en `SPEC_EXAMPLE_PATH` (si existe).
- **Componer** las 20 secciones en orden, reemplazando cada `[...]` con contenido concreto
  derivado de `BRIEF` + `CLARIFICATIONS`. Para cualquier dimensión sin evidencia suficiente,
  escribir `TBD — ver Open questions` (nunca inventar).
- **Escribir** el spec completo a `OUTPUT_PATH` (el caller provee la ruta absoluta; el
  compositor crea los directorios padre si no existen).
- **Devolver** una confirmación estructurada al caller: path escrito, número de secciones,
  número de marcadores `TBD` detectados.
- Respetar `SPEC_LANGUAGE` para el cuerpo del spec; slugs e identificadores siempre en
  kebab-case inglés.
- **No** llamar al binario `vector`. **No** usar `AskUserQuestion`. **No** editar `.vector/`.

Seguir como referencia: `kit/agents/vector-spec-refiner.md` (estructura del frontmatter,
reglas hard, tono), `kit/agents/vector-spec-validator.md` (profundidad de análisis por
sección).

No debe incluir: lógica de detección de tickets, lógica de prioridad, llamadas al estado,
subagentes anidados.

#### `.claude/agents/vector-spec-composer.md` — NUEVO

Acción: NUEVO

Contenido idéntico a `kit/agents/vector-spec-composer.md`. Esta es la copia que Claude Code
usa en el repo de Vector durante el desarrollo. Se mantiene sincronizada manualmente (o con
`vector update`).

Seguir como referencia: `.claude/agents/vector-spec-refiner.md`.

#### `cli/internal/scaffold/assets/agents/vector-spec-composer.md` — NUEVO

Acción: NUEVO

Contenido idéntico a `kit/agents/vector-spec-composer.md`. Esta es la copia embebida en el
binario vía `//go:embed all:assets` en `cli/internal/scaffold/scaffold.go`. Se genera via
`go generate` (misma cadena que los agentes hermanos ya vendorizados).

Restricciones: contenido byte-a-byte igual al de `kit/agents/`. Si difieren, `vector init`
siembra la versión stale.

#### `cli/internal/scaffold/scaffold_test.go` — MODIFICAR

Acción: MODIFICAR

Cambios requeridos:
- Agregar constante `specComposerAgent = ".claude/agents/vector-spec-composer.md"` junto a
  `bugRefiner` (línea 12 del archivo actual).
- Agregar o extender un test (análogo a `TestSeedCommandsSeedsBugCommandAndRefiner`, línea
  100) que verifique que `SeedCommands` produce un resultado `ActionCreated` para
  `specComposerAgent` y que el archivo existe en el directorio temporal.

Restricciones: no cambiar los tests existentes; no cambiar la lógica de `SeedCommands`; solo
agregar aserciones sobre el nuevo agente.

#### `kit/commands/vector/raw.md` — MODIFICAR

Acción: MODIFICAR

Cambios requeridos (pasos 7, 9 y 10):

**Paso 7** — reemplazar el bloque inline de composición por:

```
7. **Resolver metadatos y componer el spec**:

   a. **Derivar título e id** del `BRIEF` (el refiner ya los propone en
      `## Optimized Change Title` y `## Kebab-case Change Name`). Confirmar con el usuario
      si el paso 6 dejó ambigüedad en el nombre. Guardar como `SPEC_TITLE` y `SPEC_ID`.

   b. **Detectar ticket** en `RAW_IDEA` (misma lógica tier-1→4 del paso 7 original).
      Guardar como `TICKET_JSON` o dejar sin asignar.

   c. **Priority** solo si la idea claramente implica una; si no, omitir (default `normal`).

   d. **Invocar `vector-spec-composer`** (**model: sonnet**, puede escribir un archivo) con:
      - `BRIEF` (salida completa del refiner, paso 5)
      - `CLARIFICATIONS` (todos los pares Q&A del paso 6, en orden)
      - `TEMPLATE_PATH`: ruta absoluta a `.claude/vector/spec-template.md`
      - `SPEC_EXAMPLE_PATH` (del paso 3, o `no example yet`)
      - `SPEC_TITLE`, `SPEC_ID`, `SPEC_LANGUAGE`
      - `OUTPUT_PATH`: `.vector/tmp/<SPEC_ID>/spec.md`

      El subagente escribe el spec completo (20 secciones) a `OUTPUT_PATH` y devuelve
      confirmación con la ruta y el número de marcadores `TBD`. Guardar como `SPEC_PATH`.
      **El main loop no retiene el texto del spec en su contexto — solo el path.**
```

**Paso 9** — cambiar la invocación del binario de:
```bash
vector spec create \
  --title "<title>" \
  --id "<slug>" \
  ...
  --body-file - --json <<'SPEC'
<spec>
SPEC
```
a:
```bash
vector spec create \
  --title "<SPEC_TITLE>" \
  --id "<SPEC_ID>" \
  [--repo "<repo-name>"] \
  [--priority "<priority>"] \
  [--ticket "$TICKET_JSON"] \
  --status draft \
  --body-file "$SPEC_PATH" --json
```
El binario ya acepta rutas de archivo en `--body-file` (ver `readBody()` en
`cli/cmd/vector/main.go` rama `default: os.ReadFile(path)`).

**Paso 10** — agregar el routing del compositor (después del bloque existente del refiner y
antes del del validator):
```bash
# El compositor corrió en Sonnet en lugar del baseline Opus:
vector spec route <id> --model sonnet --baseline opus --task "compose spec" \
  --tokens-in <composer-in> --tokens-out <composer-out>
```

Restricciones: no cambiar pasos 1–6 ni pasos 8, 11; no cambiar las hard rules; no cambiar
la lógica de detección de tickets.

#### `kit/commands/vector/bug.md` — MODIFICAR

Acción: MODIFICAR

Cambios requeridos (pasos 7, 9 y 10): análogos a los de `raw.md`, adaptados al contexto de
bug:

**Paso 7** — el bloque inline "Compose the spec" se reemplaza por la invocación al
`vector-spec-composer` (Sonnet) con los mismos inputs que en `raw.md`, más:
- El `BRIEF` que se pasa es el de `vector-bug-refiner` (estructura diferente al refiner de
  `raw.md`, pero el compositor recibe los datos relevantes de las 8 secciones del bug refiner
  como parte de `CLARIFICATIONS`).
- `SPEC_ID` lleva el prefijo `fix-` (e.g. `fix-login-redirect-loop`), ya propuesto por el
  `vector-bug-refiner`.
- El campo de bug-framing (expected vs actual, reproduction steps) se incluye en `BRIEF`
  o `CLARIFICATIONS` para que el compositor lo ubique en las secciones correctas (§8 y §11).

**Paso 9**: misma sustitución de heredoc por `--body-file "$SPEC_PATH"`.

**Paso 10**: misma adición del routing del compositor.

Restricciones: no cambiar la lógica de deducción de causa (`RELATED_JSON`); no cambiar la
bandera `--related`; no cambiar pasos 1–6 ni 8, 11.

---

## 7. API Contract

Sin superficie HTTP. La **interfaz del subagente** (consumida por el main loop vía la
instrucción del Agent call) es:

**Inputs** (pasados en el prompt del Agent call):

| Campo | Tipo | Descripción |
|---|---|---|
| `BRIEF` | string (markdown) | Salida completa del refiner (refiner de raw o bug) |
| `CLARIFICATIONS` | string (Q&A block) | Todas las respuestas del usuario al paso 6, en orden |
| `TEMPLATE_PATH` | path absoluto | `.claude/vector/spec-template.md` |
| `SPEC_EXAMPLE_PATH` | path absoluto o `no example yet` | Ejemplo de spec del proyecto |
| `SPEC_TITLE` | string | Título ya confirmado (≤ ~8 words) |
| `SPEC_ID` | string | Slug kebab-case confirmado |
| `SPEC_LANGUAGE` | `es` \| `en` | Idioma del cuerpo del spec |
| `OUTPUT_PATH` | path absoluto | Donde el compositor escribe el archivo |

**Output** (devuelto al caller vía el resultado del Agent):

```
Spec written to: <OUTPUT_PATH>
Sections: 20
TBD markers: <n>
```

El compositor no escribe nada más que `OUTPUT_PATH`. Cualquier otro write es un bug.

---

## 8. Criterios de éxito

- [ ] `kit/agents/vector-spec-composer.md` existe con frontmatter `model: sonnet` y
      `tools: Read, Write, Glob`.
- [ ] Los tres archivos del agente (kit, .claude, assets) son byte-a-byte iguales.
- [ ] Invocar el compositor en un Agent call (Sonnet) con un BRIEF + CLARIFICATIONS de prueba
      produce un archivo en `OUTPUT_PATH` con exactamente 20 secciones numeradas.
- [ ] El archivo escrito no contiene `[...]` literales sin reemplazar (solo `TBD — ver Open
      questions` cuando corresponde).
- [ ] `/vector:raw` paso 9 usa `--body-file "$SPEC_PATH"` y no un heredoc.
- [ ] `vector spec create --body-file <ruta>` lee el archivo correctamente (`readBody` rama
      `default` ya existente — sin cambios de binario).
- [ ] `cli/internal/scaffold/scaffold_test.go` tiene un test que verifica que `SeedCommands`
      siembra `vector-spec-composer` con acción `ActionCreated`.
- [ ] El main loop de `/vector:raw` no retiene el texto completo del spec en su contexto (el
      token footprint del loop se reduce en ~8–15 k tokens de output).
- [ ] Paso 10 de `raw.md` y `bug.md` incluye el routing del compositor:
      `vector spec route <id> --model sonnet --baseline opus --task "compose spec" ...`.
- [ ] Sin regresiones: `/vector:raw` y `/vector:bug` completan el pipeline end-to-end
      (refiner → clarify → composer → validator → create) y el spec es funcional.

### Tests requeridos

- [ ] Test de scaffold: `vector-spec-composer` sembrado por `SeedCommands` (análogo al
      test de `vector-bug-refiner` en línea 100 de `scaffold_test.go`).
- [ ] Verificación manual del pipeline (o integración): invocar `/vector:raw` con una idea
      simple y confirmar que el spec compuesto llega al validator vía `SPEC_PATH` (no inline).

### Comandos de verificación

```bash
# Go: formato, vet, tests (incluyendo el nuevo test de scaffold)
gofmt -l cli && go -C cli vet ./... && go -C cli test ./...

# Verificar que los tres archivos del agente son iguales:
diff kit/agents/vector-spec-composer.md .claude/agents/vector-spec-composer.md
diff kit/agents/vector-spec-composer.md cli/internal/scaffold/assets/agents/vector-spec-composer.md
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

Aplica al **subagente** y a los **commands modificados** (no a UI web):

### Comportamiento del compositor

- **Sin preguntas**: el compositor no usa `AskUserQuestion`. Si el input es ambiguo, marca
  la sección como `TBD — ver Open questions` y continúa. La ambigüedad se resolvió en el
  main loop (paso 6).
- **Determinismo**: dados los mismos inputs, produce la misma estructura de 20 secciones.
  El contenido puede variar por el modelo, pero la estructura es invariante.
- **Confirmación visible**: el compositor devuelve una confirmación legible con el path y el
  número de marcadores `TBD`, para que el caller pueda reportarlo al usuario.

### Comportamiento del main loop tras el cambio

- **Sin cambio de UX externa**: el usuario sigue viendo el mismo reporte final (id, status,
  specDoc path, veredicto del validator). El cambio es invisible para él.
- **Transparencia del routing**: el paso 11 de `raw.md` (y `bug.md`) ya menciona el routing
  (refiner=Haiku, validator=Sonnet). Agregar "composer=Sonnet" al reporte es opcional en V1
  pero coherente con la transparencia de token-savings.
- **Crash recovery**: si el main loop cae después de que el compositor escribe `SPEC_PATH`,
  el archivo queda en disco. El usuario puede retomar desde el validator invocando
  directamente, sin re-componer.

### Errores accionables

- Template no encontrado en `TEMPLATE_PATH` → `"template not found at <path>; run vector init"`.
- Fallo de escritura en `OUTPUT_PATH` → `"failed to write spec to <path>: <reason>"`.
- Input incompleto (BRIEF vacío o SPEC_ID vacío) → `"BRIEF or SPEC_ID is empty; check the
  refiner output"`.

---

## 10. Decisiones tomadas

- **Model: Sonnet** para el compositor. Haiku no tiene la capacidad generativa para
  completar fielmente las 20 secciones de un spec con el nivel de detalle requerido (el
  validator Sonnet lo rechazaría). Opus es el tier a evitar (ese es el objetivo). Sonnet
  es el tier correcto. *Por qué*: la disciplina de token-routing del proyecto ya usa Sonnet
  para el validator (gate de calidad); la composición tiene complejidad equivalente.
- **Tools del compositor: `Read`, `Write`, `Glob`** — solo lo necesario. No `Bash`, no
  `Agent`, no `AskUserQuestion`. *Por qué*: el compositor es un generador puro; limitar
  tools reduce el surface de efectos secundarios.
- **Un solo archivo de output** (`OUTPUT_PATH` provisto por el caller). El compositor no
  elige la ruta. *Por qué*: CLI-owns-writes — el binario es la autoridad sobre el repo del
  usuario. El compositor escribe solo el spec doc temporal; el binario mueve/registra.
- **Sin `AskUserQuestion` en el compositor.** *Por qué*: las clarificaciones ocurren en el
  main loop (paso 6). Mezclar preguntas en el compositor rompe el pipeline de agentes y
  degrada la reproducibilidad.
- **Tres ubicaciones sincronizadas** (kit, .claude, assets). *Por qué*: patrón establecido
  para todos los agentes del kit (`vector-spec-refiner.md`, `vector-spec-validator.md`,
  `vector-bug-refiner.md`). Romper la sincronización dejaría `vector init` sembrando un
  agente stale.
- **`--body-file "$SPEC_PATH"`** en lugar de heredoc stdin. *Por qué*: `readBody()` en el
  binario ya soporta la rama `default: os.ReadFile(path)` (líneas 887–892 de
  `cli/cmd/vector/main.go`) — cero cambios de binario, y el spec no ocupa contexto del loop.
- **`OUTPUT_PATH` = `.vector/tmp/<id>/spec.md`** (bajo `.vector/`, gitignored o limpiado
  por el caller). *Por qué*: ruta predecible, en el workspace de Vector, debuggable.
  TBD: gitignore explícito de `.vector/tmp/` — ver Open questions §1.
- **El gate de calidad no cambia**: el validator adversarial (Sonnet, cap 3 ciclos) sigue
  siendo la única garantía de calidad. El compositor no hace auto-validación. *Por qué*: la
  separación de responsabilidades es el principio. El compositor genera; el validator
  rechaza.

Si el agente ve una alternativa mejor, la reporta como observación; no la implementa.

---

## 11. Edge cases

### Input incompleto o mal formado

- **`BRIEF` vacío o ausente**: el compositor devuelve error inmediato sin escribir el archivo.
  El main loop debe propagar el error y no llegar a `vector spec create`.
- **`SPEC_ID` vacío o no kebab-case**: error inmediato. El caller es responsable de validarlo
  antes de invocar el compositor.
- **`TEMPLATE_PATH` no existe**: error con mensaje accionable
  (`"template not found at <path>"`). El spec no se escribe parcialmente.
- **`SPEC_EXAMPLE_PATH` no existe** (pero no es `no example yet`): el compositor cae a
  estilo de template y continúa. No es un error bloqueante — el ejemplo es opcional.
- **`CLARIFICATIONS` vacío** (el usuario no respondió preguntas): el compositor usa solo
  el BRIEF. Marca con `TBD — ver Open questions` las dimensiones sin evidencia. No falla.

### Escritura del archivo

- **`OUTPUT_PATH` directorio padre no existe**: el compositor lo crea (Write tool en Claude
  Code crea directorios padre automáticamente). Si la creación falla (permisos), error con
  contexto.
- **`OUTPUT_PATH` ya existe** (una corrida anterior dejó el archivo): el compositor
  sobreescribe. El main loop controla si se retoma o recompone — el compositor no debe
  preguntar.
- **Write parcial / crash a mitad de la escritura**: el archivo puede quedar incompleto. El
  validator lo detectará (secciones faltantes → BLOCK). El main loop re-invoca el compositor
  si el validator bloquea por razón de "archivo truncado". TBD: si se necesita escritura
  atómica (write-to-tmp + rename) — ver Open questions §2.

### Calidad del spec compuesto

- **Sección sin evidencia en BRIEF ni CLARIFICATIONS**: `TBD — ver Open questions`. Nunca
  inventar contenido de producto. El validator puede BLOCK si la sección es load-bearing;
  el main loop re-itera (cap 3 ciclos, comportamiento ya existente en el validator gate).
- **Más de un marcador `TBD`** en secciones load-bearing (5, 6, 7, 8, 9, 10, 11, 13): el
  compositor los deja y los cuenta en la confirmación. El validator decide si bloqueará.
- **Spec más largo de lo esperado** (e.g. refiner con brief muy rico): el compositor escribe
  todo. No hay límite de longitud impuesto — la calidad importa más que el tamaño.
- **`SPEC_LANGUAGE = es` pero fragmentos del BRIEF están en inglés**: el compositor traduce
  al español el contenido de las secciones; slugs y paths permanecen en inglés.

### Incompatibilidad con el validator

- **Validator bloquea por archivo escrito por el compositor** (BLOCK): el main loop ya tiene
  el loop de re-validación (cap 3 ciclos). El main loop puede re-invocar al compositor con
  las correcciones requeridas del validator como contexto adicional en `CLARIFICATIONS`.
  TBD: si el re-invoke del compositor con fixes es automático o va al main loop — ver Open
  questions §3.

### Concurrencia

- **Dos corridas de `/vector:raw` con el mismo id en paralelo**: ambas escriben a
  `.vector/tmp/<id>/spec.md`. La segunda pisa a la primera. El estado lo serializa el Store
  (mutex), por lo que solo la primera invocación de `vector spec create` con ese id tendrá
  éxito; la segunda recibirá error de id duplicado. No hay pérdida de datos de usuario.

### Sin red / sin acceso a Claude API

- **El subagente no puede correr** (API down, timeout del orchestrator): el main loop recibe
  un error del Agent call. En ese caso, el main loop puede fallback a composición inline
  (comportamiento actual de `/vector:raw`). TBD: si el fallback es automático o requiere
  acción del usuario — ver Open questions §4.

---

## 12. Estados de UI requeridos

Estados del subagente (no UI web):

| Estado | Qué se muestra | Qué puede hacer el caller |
|---|---|---|
| idle | Esperando invocación con inputs | Invocar con BRIEF + CLARIFICATIONS |
| reading | Leyendo template y ejemplo de spec | Esperar |
| composing | Generando las 20 secciones | Esperar |
| writing | Escribiendo `OUTPUT_PATH` | Esperar |
| success | `Spec written to: <path> / Sections: 20 / TBD markers: <n>` | Pasar `SPEC_PATH` al validator |
| error | `template not found` / `write failed` / `BRIEF empty` | Reportar al usuario y detener el pipeline |

Estados de la experiencia del usuario (main loop):

| Estado | Qué ve el usuario | Qué puede hacer |
|---|---|---|
| composing | Ningún cambio visible (el compositor corre en background) | Esperar |
| validator running | `Validating spec...` (igual que antes) | Esperar |
| validator pass | Spec listo, id + specDoc path | Ir a `/vector:propose` |
| validator block | Reporte de bloqueo (igual que antes) | Responder al main loop con fixes |
| error (compositor) | Mensaje de error del compositor | Reintentar o reportar bug |

---

## 13. Validaciones

### Validaciones del compositor (antes de escribir)

| Campo | Regla | Comportamiento ante fallo |
|---|---|---|
| `SPEC_ID` | kebab-case `[a-z0-9-]+`, no vacío | Error inmediato, sin escritura |
| `TEMPLATE_PATH` | Archivo existe y es legible | Error inmediato, sin escritura |
| `BRIEF` | String no vacío, contiene al menos `## Problem` | Error inmediato |
| 20 secciones | Todas presentes en el output generado, en orden | Auto-corrección antes de escribir; si imposible, error |
| Placeholders `[...]` | Ninguno en el output final (solo `TBD — ver...`) | Auto-corrección o marcado TBD |

### Validaciones del compositor (post-escritura, en la confirmación)

| Campo | Regla | Mensaje |
|---|---|---|
| Archivo escrito | Existe y no está vacío | Confirmación positiva |
| Conteo TBD | Número de marcadores `TBD — ver Open questions` | Incluido en la confirmación para que el caller lo reporte |

### Validaciones del validator (sin cambio)

El validator adversarial (`vector-spec-validator.md`, Sonnet) sigue siendo la única gate de
calidad estructural. El compositor no reemplaza ni duplica esta validación.

### Validaciones del binario (sin cambio)

`vector spec create --body-file <path>` valida el cuerpo leído contra las reglas del Store
(id kebab-case, status válido, body no vacío). Ningún cambio requerido.

---

## 14. Seguridad y permisos

- El compositor solo escribe a `OUTPUT_PATH` (la ruta provista por el caller). **No escribe
  en `.vector/specs/`** (dominio del binario) ni en `<repo>/.claude/` (dominio de `vector
  init`).
- El BRIEF puede contener información del repo del usuario (paths, símbolos, fragmentos de
  código). El compositor no logea estos datos; los usa para componer el spec y los persiste
  en `OUTPUT_PATH` (que es el spec mismo, artefacto esperado del usuario).
- Nunca incluir secrets, tokens o variables de entorno en el spec compuesto, aunque el BRIEF
  los mencione accidentalmente. Si el compositor detecta un patrón de secret (key=, token=,
  sk_…, pk_…), omitirlo del spec y marcarlo como `[REDACTED — no incluir en spec]`.
- El compositor corre en el entorno de Claude Code del usuario (mismo modelo de permisos que
  los otros subagentes). Sin aumento de permisos.

---

## 15. Observabilidad y logging

El compositor devuelve su confirmación estructurada al caller (path, sección count, TBD
count). El caller registra el routing con `vector spec route`:

```bash
vector spec route <id> \
  --model sonnet \
  --baseline opus \
  --task "compose spec" \
  --tokens-in <compositor-in> \
  --tokens-out <compositor-out>
```

Esto appendea un `EvtAgentRouted` al `activity.jsonl` local (ver
`cli/internal/state/event.go` líneas 26 y 97–107), contribuyendo al Token Savings Meter del
board. El caller usa los token counts devueltos por el Agent call si los tiene; de lo
contrario, una estimación redondeada al millar (el meter es estimado por diseño).

No registrar en el spec ni en el activity log:
- El BRIEF completo (puede contener fragmentos de código del usuario).
- Las CLARIFICATIONS completas.
- Paths internos sensibles.

---

## 16. i18n / textos visibles

El proyecto no tiene sistema de i18n. Los mensajes de los subagentes están hardcodeados en
inglés (consistente con el resto de la tooling de Vector). El cuerpo del spec se escribe en
`SPEC_LANGUAGE` (detectado del proyecto). La tabla siguiente documenta los strings del
compositor, no keys de traducción:

| Identificador (doc) | Texto (hardcoded EN) |
|---|---|
| composer.success | `Spec written to: {OUTPUT_PATH}` |
| composer.sections | `Sections: 20` |
| composer.tbd | `TBD markers: {n}` |
| composer.error.template | `template not found at {TEMPLATE_PATH}; run vector init` |
| composer.error.write | `failed to write spec to {OUTPUT_PATH}: {reason}` |
| composer.error.brief | `BRIEF or SPEC_ID is empty; check the refiner output` |

---

## 17. Performance

- La composición corre en Sonnet vía el Agent SDK — típicamente 5–15 s para un spec de 20
  secciones, según la longitud del BRIEF. Comparable al tiempo actual de composición en Opus,
  con menor costo por token.
- El write del archivo es I/O local, <10 ms.
- **Reducción de contexto del main loop**: el main loop Opus no retiene las ~8–15 k tokens
  del spec generado. El beneficio de costo se acumula en los pasos posteriores (validator
  call, create call, report) que ahora corren con un contexto más pequeño.
- El compositor lee el template (~1.5 k tokens) y el ejemplo (~3–5 k tokens) una sola vez.
  Sin I/O redundante.
- Sin llamadas de red adicionales (el subagente corre localmente en el Agent SDK; el Write
  tool es I/O local).

---

## 18. Restricciones

El agente no debe:

- **Modificar el binario Go** (`cli/cmd/vector/main.go`, `cli/internal/state/`). `readBody`
  ya soporta rutas de archivo; no hay nuevos subcomandos necesarios.
- **Cambiar `vector-spec-validator.md`** o `vector-spec-refiner.md` o `vector-bug-refiner.md`.
- **Cambiar los pasos 1–6 ni 8, 11 de `raw.md` o `bug.md`**. Solo pasos 7, 9 y 10.
- **Añadir `AskUserQuestion` al compositor**. El compositor es un generador puro.
- **Dar al compositor acceso a `Bash`** o a tools fuera de `Read`, `Write`, `Glob`.
- **Crear carpetas o archivos fuera de las rutas listadas** en §6.
- **Cambiar la lógica de detección de tickets** en `raw.md` (permanece en el main loop).
- **Eliminar el validator gate ni reducir su cap de 3 ciclos**.
- **Pisar el archivo del spec de un spec ya creado** (el compositor escribe a `.vector/tmp/`,
  no a la ubicación final del specDoc; el binario es quien gestiona la ubicación final).
- **Instalar dependencias nuevas** en Go o en el kit.
- **Cambiar el esquema del state** (`SpecState`, `Event`, `activity.jsonl`). El routing del
  compositor se registra con `EvtAgentRouted` (tipo ya existente).

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `kit/agents/vector-spec-composer.md` creado y completo.
- [ ] `.claude/agents/vector-spec-composer.md` creado (contenido igual al de kit).
- [ ] `cli/internal/scaffold/assets/agents/vector-spec-composer.md` creado y vendorizado.
- [ ] Los tres archivos del agente son byte-a-byte iguales (verificado con `diff`).
- [ ] `kit/commands/vector/raw.md` modificado (pasos 7, 9, 10).
- [ ] `kit/commands/vector/bug.md` modificado (pasos 7, 9, 10).
- [ ] `cli/internal/scaffold/scaffold_test.go` actualizado con test del compositor.
- [ ] `gofmt`, `go vet`, `go test ./...` en `cli/` pasan sin warnings ni fallos.
- [ ] `diff kit/agents/vector-spec-composer.md .claude/agents/vector-spec-composer.md` → sin
      diferencias.
- [ ] `diff kit/agents/vector-spec-composer.md cli/internal/scaffold/assets/agents/vector-spec-composer.md`
      → sin diferencias.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Revisé `kit/agents/vector-spec-refiner.md` y `kit/agents/vector-spec-validator.md`
      (agentes hermanos) para heredar el patrón de frontmatter, reglas y tono.
- [ ] Revisé `kit/commands/vector/raw.md` pasos 5–11 completos para aplicar solo los
      cambios a pasos 7, 9 y 10, sin tocar el resto.
- [ ] Revisé `kit/commands/vector/bug.md` pasos 5–11 completos (análogo).
- [ ] Revisé `cli/internal/scaffold/scaffold_test.go` líneas 97–115 para heredar el patrón
      del test de `vector-bug-refiner`.
- [ ] Verifiqué que `readBody()` en `cli/cmd/vector/main.go` (líneas 877–894) ya soporta
      rutas de archivo — sin cambios de binario necesarios.
- [ ] Los tres archivos del agente creados son byte-a-byte iguales.
- [ ] El compositor no llama al binario, no usa `AskUserQuestion`, no escribe fuera de
      `OUTPUT_PATH`.
- [ ] Los pasos 1–6, 8 y 11 de `raw.md` y `bug.md` no fueron modificados.
- [ ] El routing del compositor se registra en el paso 10 de ambos commands.
- [ ] Ejecuté `gofmt -l cli && go -C cli vet ./... && go -C cli test ./...`.
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar en el spec del compositor.
- [ ] No cambié ninguna decisión tomada (§10) sin registrarlo como observación.

---

## Open questions

1. **Gitignore de `.vector/tmp/`**: ¿debe agregarse una entrada en el `.gitignore` del repo
   del usuario (durante `vector init`) o en el `.vector/.gitignore` para que los specs
   temporales no se commiteen accidentalmente? TBD al implementar.

2. **Escritura atómica del compositor**: ¿vale la pena que el compositor escriba a
   `.vector/tmp/<id>/.spec.md.tmp` y luego renombre a `.spec.md` para evitar archivos
   parciales ante crash? En V1, Write directo es suficiente dado que el validator detecta
   specs truncados. TBD si se necesita mayor robustez.

3. **Re-invoke automático del compositor ante BLOCK del validator**: cuando el validator
   bloquea, ¿el main loop re-invoca al compositor con las correcciones como contexto
   adicional (full automation), o el main loop intenta las correcciones inline? V1 propuesto:
   el main loop re-invoca al compositor con el reporte del validator como `CLARIFICATIONS`
   adicionales. TBD al implementar el loop de re-validación.

4. **Fallback inline si el compositor falla**: si el Agent call al compositor falla (timeout,
   API error), ¿el main loop cae a composición inline (comportamiento anterior)? V1
   propuesto: sí, con un warning visible. TBD: definir el comportamiento exacto y si el
   fallback inline debe registrarse de forma diferente en el routing.
