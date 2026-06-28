# Spec: Subcomando `vector context` — setup cacheado por sesión

## 1. Objetivo

Construir `vector context`: un subcomando del binario Go que devuelve en **una sola llamada**
el contexto de setup del repo `{examplePath, language, buildCmd, lintCmd, testCmd, applyMode,
ticketDetected?}`, eliminando que cada command del kit re-derive de forma independiente la
misma información al arrancar.

Esta feature permite que los commands del kit (`/vector:raw`, `/vector:bug`, `/vector:apply`,
`/vector:comment`) consuman setup ya resuelto en vez de repetir globeos, lecturas de manifest y
detecciones de lenguaje por cada invocación, reduciendo tokens de orquestación y latencia de
arranque.

## 2. Alcance

### Incluido en esta fase

- Nuevo **subcomando de binario** `vector context` (`runContext`) que retorna el contexto de
  setup en JSON o en texto humano legible.
- Nuevos campos en `config.json` para los valores estables detectados **una vez** en `vector
  init`/`update`: `buildCmd`, `lintCmd`, `testCmd`. Estos campos se detectan de los manifests
  del repo del usuario (Makefile, go.mod, package.json, pyproject.toml, etc.) y se persisten
  en `.vector/config.json`.
- Detección de manifests paralela con goroutines en Go (dentro de `runInit`/`runUpdate`):
  lectura concurrente de los archivos de manifest para derivar build/lint/test commands.
- Actualización de los cuatro commands del kit afectados para que consuman `vector context
  --json` en su primer paso en vez de re-derivar: `raw.md`, `bug.md`, `apply.md`,
  `comment.md`.
- `examplePath` se resuelve **en runtime** por `vector context` (glob sobre `specPath` —
  fresquísimo, no cacheado): el ejemplo más reciente se prefiere; si no hay ninguno, devuelve
  `""`.
- `language` se lee de `config.Language` (ya existente; ya cacheado en `init`).
- `applyMode` se lee de `config.ResolvedApplyMode()` (ya existente).
- `ticketDetected` (booleano) indica si hay `defaultTicketProvider` configurado (proxy de "la
  detección de tickets está activa en este repo").
- Copia vendorizada de los commands actualizados en `cli/internal/scaffold/assets/commands/
  vector/` (vía `go generate`, igual que el resto del kit).
- Tests unitarios para la lógica de detección de manifests y para `runContext`.

### Fuera de scope

- Cache en memoria entre invocaciones del binario (el binario es stateless; el cache es
  `config.json` en disco).
- Detección automática de `language` desde el sistema operativo o el entorno del usuario; la
  detección sigue siendo la actual (ejemplo de spec existente, default inglés).
- Subcomando `vector context set` para editar campos individualmente (los campos se setean vía
  `vector init --build-cmd` / `vector update --build-cmd`).
- Invalidación automática de cache cuando cambian los manifests del usuario en tiempo real
  (sería watchers de filesystem — no includio en V1). La invalidación se hace re-corriendo
  `vector update`.
- Panel web / SSE — `vector context` es una lectura síncrona, sin streaming.
- Detección de commands de CI (GitHub Actions, CircleCI, etc.) — solo manifests locales.
- Actualizar commands del kit distintos de los cuatro ya enunciados (sync.md, propose.md,
  close.md, archive.md, link.md, status.md no tienen re-derivación equivalente).

El agente no implementa nada fuera de este alcance aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Binario: **Go** (módulo único, stdlib; `cli/`). Sin dependencias externas.
- Project commands: **Markdown + frontmatter** orquestado por Claude (patrón del kit).
- Concurrencia: `sync.WaitGroup` + goroutines stdlib (patrón habitual en Go sin deps).

### Versiones relevantes

- Go: `1.26` (según `cli/go.mod`; verificar antes de implementar).
- No se agregan dependencias externas — Go stdlib únicamente.

### Patrones existentes a respetar

- `cli/cmd/vector/main.go`: switch de subcomandos top-level en `main()`. Se agrega `case
  "context": err = runContext(os.Args[2:])`. Patrón idéntico al de `"serve"`, `"standup"`, `"spec"`.
- `cli/cmd/vector/main.go` — funciones `runXxx` como archivos separados (`ticket.go`,
  `standup.go`, `spec_transitions.go`): `runContext` va en un nuevo archivo
  `cli/cmd/vector/context.go`.
- `cli/internal/config/config.go`: campos `omitempty` con accessors tipados (`ResolvedApplyMode`,
  `ResolvedLanguage`). Los tres campos nuevos siguen exactamente el mismo patrón.
- `cli/internal/config/Resolve` / `runInit` / `runUpdate`: la detección de config nueva ocurre en
  `runInit`/`runUpdate` al persistir `config.json`; no se re-detecta en runtime.
- `cli/internal/config/writeFileAtomic`: toda escritura de config pasa por aquí; no tocar.
- Flags de binario: `flag.FlagSet` por subcomando; `--repo-root`, `--json`, `--dry-run` son
  consistentes en todos los subcomandos existentes.
- Error reporting: `fmt.Errorf("…: %w", err)` para preservar cadena; mensajes al usuario
  claros y accionables. No se usa `panic` en flujo normal.
- `context.Context`: no aplica en este subcomando (operación local, sin I/O bloqueante de red).
- Pruebas: paquete `testing` estándar; table-driven. Ver `cli/internal/config/config_test.go`
  como referencia.
- CLI-owns-writes: `vector context` es **read-only**; jamás muta estado.
- Kit commands: frontmatter + secciones numeradas; token routing explícito. Ver `raw.md` y
  `apply.md` como referencia para el patrón de consumo de un nuevo subcomando.

---

## 4. Dependencias previas

Antes de iniciar la implementación debe existir o estar completado:

- [ ] `.vector/config.json` presente en el repo del usuario (generado por `vector init`). Sin
  él `vector context` falla con mensaje accionable `"run vector init first"`.
- [ ] `cli/internal/config/config.go` con los tipos `Config`, `ApplyMode`, y los accessors
  existentes (`ResolvedApplyMode`, `ResolvedLanguage`) — ya existe; no se migra el schema.
- [ ] `cli/cmd/vector/main.go` con la estructura de `switch` de subcomandos — ya existe.
- [ ] El patrón de vendorización de assets del kit (`cli/internal/scaffold/assets/commands/
  vector/`; `go generate`) — ya existe para todos los commands actuales.
- [ ] Tests de `cli/internal/config/config_test.go` corriendo en verde antes de modificar el
  paquete.

Si alguna dependencia no existe, el agente se detiene y reporta exactamente qué falta. No
inventa contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

Lectura de config + glob de filesystem + detección de manifests en el binario, expuesto como
subcomando stateless. Los commands del kit consumen el JSON output; CLI-owns-reads (nadie
escribe `config.json` salvo `init`/`update`).

### Capas afectadas

- **`cli/cmd/vector/main.go`** (top-level switch): sí — se agrega `case "context"`.
- **`cli/cmd/vector/context.go`** (nuevo): sí — `runContext` (flags, lectura de config,
  glob, serialización de output).
- **`cli/internal/config/config.go`**: sí — tres campos nuevos (`BuildCmd`, `LintCmd`,
  `TestCmd`), un nuevo accessor (`ResolvedBuildCmds`) y la función de detección de manifests
  (`DetectBuildCmds`), llamada desde `runInit`/`runUpdate`.
- **`cli/internal/config/config_test.go`**: sí — tests para `DetectBuildCmds`.
- **`cli/cmd/vector/main.go`** (`runInit`, `runUpdate`): sí — llamar `config.DetectBuildCmds`
  y persistir los tres campos cuando no estén ya fijados (o cuando `--force`).
- **`kit/commands/vector/raw.md`**: sí — reemplazar los pasos de glob + detect-language por
  `vector context --json`.
- **`kit/commands/vector/bug.md`**: sí — ídem.
- **`kit/commands/vector/apply.md`**: sí — añadir paso inicial `vector context --json` para
  consumir `buildCmd`/`testCmd` en el gate de verificación (paso 4).
- **`kit/commands/vector/comment.md`**: sí — reemplazar el discover de build/lint/test en
  §7a.3 por el valor ya resuelto de `vector context --json`.
- **`cli/internal/scaffold/assets/commands/vector/`**: sí — vendorización (regenerada vía `go
  generate`; no editar a mano).
- **`web/`**: no.
- **`cli/internal/state/`**: no.

### Flujo esperado

1. Un command del kit (ej. `/vector:raw`) arranca y ejecuta `vector context --json` como
   **primer paso**.
2. El binario lee `.vector/config.json` (carga: `config.Load`).
3. Resuelve los valores estables de config: `language` (`ResolvedLanguage`), `applyMode`
   (`ResolvedApplyMode`), `buildCmd`/`lintCmd`/`testCmd` (de los nuevos campos), `ticketDetected`
   (booleano de `ResolvedDefaultTicketProvider() != ""`).
4. Glob del `specPath` para encontrar el primer ejemplo de spec (`examplePath`): concurrente con
   el paso 3 (goroutines), aunque el glob es barato.
5. Serializa el contexto como JSON y escribe a stdout.
6. El command recibe el JSON, extrae los campos que necesita, y continúa sin re-derivar nada.
7. En background (fuera del flujo del command): si `buildCmd`/`lintCmd`/`testCmd` están vacíos
   en config, `vector context` los detecta frescos desde los manifests y los incluye en el JSON
   de output **sin persistirlos** (la persistencia ocurre solo en `init`/`update`). Esto garantiza
   que el primer `vector context` tras un `init` sin manifest-detection ya devuelva algo útil.

### Ubicación de archivos nuevos

```txt
cli/
  cmd/vector/
    context.go          ← NUEVO: runContext
  internal/
    config/
      config.go         ← MODIFICAR: 3 campos, DetectBuildCmds, ResolvedBuildCmds
      config_test.go    ← MODIFICAR: tests para DetectBuildCmds
kit/
  commands/vector/
    raw.md              ← MODIFICAR
    bug.md              ← MODIFICAR
    apply.md            ← MODIFICAR
    comment.md          ← MODIFICAR
cli/internal/scaffold/assets/commands/vector/
    raw.md              ← REGENERAR (go generate)
    bug.md              ← REGENERAR
    apply.md            ← REGENERAR
    comment.md          ← REGENERAR
```

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `cli/cmd/vector/context.go` | NUEVO | `runContext`: flags, carga config, glob examplePath, construye y serializa `ContextOutput` | `cli/cmd/vector/standup.go` (estructura de runXxx con --json) |
| `cli/cmd/vector/main.go` | MODIFICAR | Agregar `case "context": err = runContext(os.Args[2:])` al switch y actualizar `usage()` | `cli/cmd/vector/main.go` (case "standup") |
| `cli/internal/config/config.go` | MODIFICAR | Campos `BuildCmd`, `LintCmd`, `TestCmd` en `Config`; función `DetectBuildCmds(repoRoot)`; accessor `ResolvedBuildCmds` | `cli/internal/config/config.go` (campos `ApplyMode`, `Language`, patrón `Resolved*`) |
| `cli/internal/config/config_test.go` | MODIFICAR | Tests para `DetectBuildCmds` (con filesystem temporal) | `cli/internal/config/config_test.go` (tests de Resolve) |
| `cli/cmd/vector/main.go` (`runInit`) | MODIFICAR | Llamar `DetectBuildCmds` y persistir en config cuando los campos estén vacíos o `--force` | `cli/cmd/vector/main.go` (`runInit`, bloque de cfg/persist) |
| `cli/cmd/vector/main.go` (`runUpdate`) | MODIFICAR | Ídem al re-sembrar el kit | `cli/cmd/vector/main.go` (`runUpdate`) |
| `kit/commands/vector/raw.md` | MODIFICAR | Reemplazar pasos 3+4 (glob + detect-language) por llamada inicial `vector context --json` | `kit/commands/vector/raw.md` (steps 3–4) |
| `kit/commands/vector/bug.md` | MODIFICAR | Ídem (pasos 4+lenguaje del bug) | `kit/commands/vector/bug.md` (paso 4) |
| `kit/commands/vector/apply.md` | MODIFICAR | Agregar paso 0 `vector context --json`; consumir `buildCmd`/`testCmd` en step 4 en vez de detectar | `kit/commands/vector/apply.md` (step 4, gate) |
| `kit/commands/vector/comment.md` | MODIFICAR | Reemplazar §7a.3 discover con los valores de `vector context --json` ya resueltos | `kit/commands/vector/comment.md` (§7a.3) |
| `cli/internal/scaffold/assets/commands/vector/{raw,bug,apply,comment}.md` | REGENERAR | Copias embebidas de los commands actualizados (vía `go generate`) | Sibling files en `assets/commands/vector/` |

### Detalle por archivo

#### `cli/cmd/vector/context.go` — NUEVO

Acción: NUEVO

Debe implementar:

- `ContextOutput` struct con campos JSON: `examplePath string`, `language string`, `buildCmd
  string`, `lintCmd string`, `testCmd string`, `applyMode string`, `ticketDetected bool`.
- `runContext(args []string) error`: flags `--repo-root`, `--json` (default true, dado que el
  principal consumidor es tooling), `--dry-run` (no-op en context, pero consistente con el
  patrón del resto de subcomandos).
- Carga `config.Load(root)` → error accionable si falta ("run `vector init` first").
- Glob de `specPath` para `examplePath`: reusar `config.FindSpecDocs` cuando
  `SpecStore == StoreConvention`; para `StoreVector`, glob de `.vector/specs/*/spec.md`. Primer
  resultado lexicográfico no vacío; vacío si no hay ninguno.
- Si `BuildCmd`/`LintCmd`/`TestCmd` están vacíos en config: llamar `DetectBuildCmds(root)` en
  goroutines concurrentes con el glob; combinar resultados. No persistir (la persistencia es de
  `init`/`update`).
- Output `--json`: `json.MarshalIndent` del struct. Salida humana: una línea por campo.

Debe seguir como referencia:

- `cli/cmd/vector/standup.go` (estructura `runXxx` con flags, carga de config, JSON output).
- `cli/internal/config/config.go` (`ResolvedApplyMode`, patrón de campos `omitempty`).

No debe incluir:

- Escritura de `config.json` (lectura pura).
- Llamadas a `state.Open` o lecturas de `activity.jsonl`.
- Lógica de detección de tickets (solo el booleano `ticketDetected`).

#### `cli/internal/config/config.go` — MODIFICAR

Acción: MODIFICAR

Cambios requeridos:

- En `Config` struct: añadir campos `BuildCmd string \`json:"buildCmd,omitempty"\``, `LintCmd
  string \`json:"lintCmd,omitempty"\``, `TestCmd string \`json:"testCmd,omitempty"\``.
  Campos aditivos y backward-compatible: un `config.json` antiguo sin estos campos los carga
  como `""` sin error.
- Nueva función `DetectBuildCmds(repoRoot string) (build, lint, test string)`: lee los
  manifests del repo del usuario en goroutines concurrentes para derivar los comandos de
  build, lint y test. La heurística de detección prioriza en orden: `Makefile` (buscar targets
  `build:`, `lint:`, `test:`), `go.mod` (inferir `go build ./...`, `golangci-lint run`,
  `go test ./...`), `package.json` (leer campo `scripts.build`/`scripts.lint`/`scripts.test`),
  `pyproject.toml`/`setup.py` (inferir `python -m build`, `ruff check`, `pytest`). Retorna
  strings vacíos si no se puede determinar con confianza (no adivina).
- Nuevo accessor `ResolvedBuildCmds() (build, lint, test string)` en `*Config`: retorna los
  campos si están seteados, else `"", "", ""`. El caller (context.go) completa con
  `DetectBuildCmds` si vacíos.

Restricciones:

- No cambiar la firma ni la semántica de `Resolve`, `Load`, `Write`, `FindSpecDocs`,
  `ChangesDirs`, ni ningún otro accessor existente.
- No romper el schema existente (`omitempty` garantiza backward compat).
- La detección de manifests no hace `os.Stat` sobre más de 5 archivos de manifest por tipo;
  no camina el árbol de directorios (solo el root del repo).
- No exponer `DetectBuildCmds` como parte de la API pública de `config.go` más allá de lo
  necesario (paquete `config`, visible desde `cmd/vector`).

#### `cli/cmd/vector/main.go` — MODIFICAR

Acción: MODIFICAR

Cambios requeridos:

- `main()` switch: añadir `case "context": err = runContext(os.Args[2:])` antes del `case
  "version"`. Orden lógico: init, update, sync, serve, standup, spec, **context**, version,
  help, default.
- `usage()` (o helper de string de uso): incluir `context` en la lista de subcomandos con una
  descripción de una línea: `"context  print repo setup context (example path, language,
  build/lint/test commands)"`.
- `runInit` y `runUpdate`: después de resolver `cfg` y antes de `config.Write`, llamar
  `config.DetectBuildCmds(root)` en goroutines; si los campos están vacíos en `cfg` (o si
  `--force`), setearlos. Patrón: igual a como hoy se setean `cfg.Language` y `cfg.KitVersion`.

Restricciones:

- No cambiar `runSpec`, `runSync`, `runServe`, `runStandup` ni ningún otro `runXxx` existente
  más allá de `runInit` y `runUpdate`.
- No modificar el flag-set de subcomandos existentes.

#### `kit/commands/vector/raw.md` — MODIFICAR

Acción: MODIFICAR

Cambios requeridos:

- Reemplazar el **Paso 2** (confirm init + note specPath) por: ejecutar `vector context --json`
  y guardar el output como `CONTEXT`. Abortar con mensaje accionable si falla.
- Eliminar el **Paso 3** (glob de specPath para encontrar ejemplo) — el valor viene de
  `CONTEXT.examplePath`.
- Eliminar el **Paso 4** (detect spec language) — el valor viene de `CONTEXT.language`.
- Renumerar los pasos subsiguientes (3→2, 4→3, etc.) manteniendo su contenido exacto.
- En el Paso de **Refine** (ahora Paso 4): pasar `CONTEXT.examplePath` donde antes se pasaba
  `SPEC_EXAMPLE_PATH`; pasar `CONTEXT.language` donde antes se pasaba el lenguaje detectado.
- En el Paso de **Validate**: ídem.
- Añadir nota de token routing: "`vector context` es una llamada barata al binario local
  (sin modelo); se corre una vez al arranque."

Restricciones:

- No cambiar la lógica de los pasos de Refine, Validate, Register, Route ni Report.
- No cambiar el frontmatter del command.
- No cambiar el flujo de detección de tickets (paso 7 original → paso 6 renumerado): ese
  flujo es independiente y no usa `CONTEXT`.

#### `kit/commands/vector/bug.md` — MODIFICAR

Acción: MODIFICAR

Cambios requeridos:

- Añadir **Paso 0** (antes de los actuales): ejecutar `vector context --json`, guardar como
  `CONTEXT`.
- Reemplazar el **Paso 4** actual (Find example spec + detect language): suprimir el glob;
  usar `CONTEXT.examplePath` y `CONTEXT.language` directamente.
- Renumerar los pasos afectados.
- Pasar `CONTEXT.examplePath` y `CONTEXT.language` al refiner y al validator donde antes se
  pasaban los valores derivados localmente.

Restricciones:

- No cambiar los pasos de deducción de causa (paso 3 original), ni el flujo de confirm init,
  ni el de registro del draft card.
- No cambiar el frontmatter.

#### `kit/commands/vector/apply.md` — MODIFICAR

Acción: MODIFICAR

Cambios requeridos:

- Añadir **Paso 0** (antes de la selección): ejecutar `vector context --json`, guardar como
  `CONTEXT`.
- En el paso de **Implement** (§4): reemplazar la instrucción abierta "Run the repo's
  test/build gate" por: "Usar `CONTEXT.buildCmd` / `CONTEXT.testCmd` para ejecutar el gate.
  Si ambos son vacíos, detectar desde los manifests del repo (Makefile, go.mod,
  package.json…) o preguntar vía `AskUserQuestion`". Esto preserva el fallback para repos sin
  manifests claros.
- Ajustar la nota de token routing al añadir: "`vector context` es barato (binario local); no
  aumenta el tier de la sesión."

Restricciones:

- No cambiar el flujo de selección (§1), el flujo de transición de estado (§2), la detección
  de modo delegado/nativo (§3), la lógica de `needs-attention` (§6), ni el resumen (§7).
- No cambiar el frontmatter.

#### `kit/commands/vector/comment.md` — MODIFICAR

Acción: MODIFICAR

Cambios requeridos:

- Añadir **Paso 0** (antes del §1 de Parse arguments): ejecutar `vector context --json`, guardar
  como `CONTEXT`.
- En **§7a.3** (Verification gate): reemplazar el discover desde manifests por: "Usar
  `CONTEXT.buildCmd` / `CONTEXT.lintCmd` / `CONTEXT.testCmd`. Si están vacíos en `CONTEXT`,
  caer al discover manual desde manifests (el comportamiento actual como fallback)."
- Ajustar token routing para mencionar que `vector context` es una llamada barata.

Restricciones:

- No cambiar el flujo de parse (§1), resolve branch (§2), diff resolution (§3), spec resolution
  (§4), el evaluator (§5), el verdict (§6), la implementación (§7a), el reply draft (§7b) ni el
  worklog (§8).
- No cambiar el frontmatter.

---

## 7. API Contract

Sin API surface HTTP — no aplica. La interfaz relevante es la **CLI del binario** consumida por
los commands del kit:

```bash
vector context [--repo-root <path>] [--json]
```

Salida `--json` (éxito):

```json
{
  "examplePath": "docs/specs/add-foo/spec.md",
  "language": "es",
  "buildCmd": "go build ./...",
  "lintCmd": "golangci-lint run",
  "testCmd": "go test ./...",
  "applyMode": "ask",
  "ticketDetected": true
}
```

Campos vacíos cuando no se pueden determinar:

```json
{
  "examplePath": "",
  "language": "",
  "buildCmd": "",
  "lintCmd": "",
  "testCmd": "",
  "applyMode": "ask",
  "ticketDetected": false
}
```

Salida humana (sin `--json`):

```
vector context: /path/to/repo
  examplePath:    docs/specs/add-foo/spec.md
  language:       es
  buildCmd:       go build ./...
  lintCmd:        golangci-lint run
  testCmd:        go test ./...
  applyMode:      ask
  ticketDetected: true
```

Exit: `0` éxito; `1` error (mensaje a stderr). El único error esperado es "`.vector/config.json`
not found — run `vector init` first".

---

## 8. Criterios de éxito

- [ ] `vector context --json` retorna un JSON parseable con los siete campos en todos los
  escenarios: repo sin manifests (campos vacíos), repo Go, repo Node, repo Python, repo con
  Makefile.
- [ ] `examplePath` apunta a un spec real que existe en disco (o `""` si no hay ninguno).
- [ ] `buildCmd`/`lintCmd`/`testCmd` se persisten en `config.json` durante `vector init` /
  `vector update` cuando los manifests permiten inferirlos.
- [ ] Un repo sin `config.json` retorna exit `1` con mensaje `"run vector init first"`.
- [ ] `raw.md` y `bug.md` actualizados no globean ni detectan lenguaje por su cuenta; consumen
  `CONTEXT` sin regresión de comportamiento observable.
- [ ] `apply.md` usa `CONTEXT.buildCmd`/`testCmd` cuando están disponibles, con fallback al
  detect manual cuando vacíos.
- [ ] `comment.md` usa `CONTEXT.buildCmd`/`lintCmd`/`testCmd` cuando están disponibles, con
  fallback al discover manual cuando vacíos.
- [ ] Sin regresiones en `vector init`, `vector update`, `vector sync`, `vector spec create`,
  `vector serve`.

### Tests requeridos

- [ ] `DetectBuildCmds` detecta Go (`go.mod` presente): retorna `go build ./...`, lint, test.
- [ ] `DetectBuildCmds` detecta Node (`package.json` con scripts): retorna los scripts.
- [ ] `DetectBuildCmds` detecta Makefile con targets `build:`/`test:`: retorna `make build`,
  `make test`.
- [ ] `DetectBuildCmds` sin manifests: retorna tres strings vacíos (no error).
- [ ] `runContext` sobre un repo sin `config.json`: error con mensaje accionable.
- [ ] `runContext --json` sobre un repo con `config.json` válido: JSON parseable, campos
  presentes.
- [ ] `Config` con campos `buildCmd`/`lintCmd`/`testCmd` vacíos serializa/deserializa sin
  romperse (backward compat).

### Comandos de verificación

```bash
gofmt -l cli && go -C cli vet ./... && go -C cli test ./...
```

---

## 9. Criterios de UX

Aplica al **subcomando del binario** y al **consumo en los commands del kit** (no a UI web):

- **Velocidad de arranque:** el primer paso de cada command del kit que llama `vector context
  --json` debe completar en < 200ms en un repo típico (filesystem local, sin red). La detección
  de manifests con goroutines no debe ser el cuello de botella.
- **Transparencia:** si `buildCmd` es vacío (no se pudo detectar), el command del kit lo informa
  al usuario (una línea) y cae al fallback en vez de fallar silenciosamente.
- **Idempotencia:** llamar `vector context --json` dos veces seguidas en el mismo repo produce
  el mismo output (salvo `examplePath` si un spec fue creado en el intervalo — aceptable por ser
  fresquísimo).
- **Errores accionables:** el único error esperado es `config.json` no encontrado → mensaje claro
  con instrucción de remediación.
- **Sin prompts:** `vector context` nunca llama `AskUserQuestion`; es una lectura silenciosa.
- **Fallback en commands del kit:** si `vector context` falla (binario no encontrado o error),
  el command del kit emite una advertencia de una línea y continúa con el comportamiento anterior
  (re-derivar localmente). No bloquea el flujo.
- **Observabilidad:** la salida humana (sin `--json`) debe ser legible como un one-shot status
  del repo, útil para debugging cuando un command del kit se comporta de forma inesperada.

---

## 10. Decisiones tomadas

- **Cache en disco (`config.json`), no en memoria:** el binario es stateless; la persistencia
  en `config.json` es el único mecanismo de cache. Consistente con el patrón de `applyMode` y
  `language`. *Por qué:* el binario no es un daemon; no hay estado compartido entre
  invocaciones.
- **`examplePath` siempre fresco (glob en runtime):** no se cachea el ejemplo de spec en
  `config.json`. *Por qué:* el ejemplo cambia frecuentemente (cada `/vector:raw` añade uno);
  cachearlo requeriría invalidación, que es la arista más compleja. El glob es barato y
  determinista.
- **Goroutines para la detección de manifests:** `sync.WaitGroup` en `DetectBuildCmds`. *Por
  qué:* los reads de manifest son I/O independientes; la paralelización es stdlib pura, sin
  dependencias.
- **Los commands del kit siguen teniendo fallback al detect manual cuando `CONTEXT` devuelve
  vacío:** no se fuerza a que `vector context` sea la única fuente. *Por qué:* robustez — un
  repo nuevo o un manifest no reconocido no debe romper el flujo.
- **No hay subcomando `vector context set`:** los campos se setean vía `vector init --build-cmd
  / --lint-cmd / --test-cmd` (flags nuevos en `runInit`/`runUpdate`). *Por qué:* consistencia
  con cómo se setea `language`; no proliferar subcomandos.
- **`ticketDetected` es solo un booleano** (no devuelve el provider): el command del kit solo
  necesita saber si la detección de tickets está activa, no el provider. *Por qué:* principio
  de mínima información necesaria; el provider ya está en `config.json` si se necesita.
- **Campos `omitempty` en `Config`:** compatibilidad total con repos que no tengan los nuevos
  campos en `config.json`.

Si el agente detecta una alternativa mejor, la reporta como observación; no la implementa.

---

## 11. Edge cases

### Datos y filesystem

- **`config.json` falta:** exit `1`, `"no .vector/config.json — run vector init first"`.
- **`config.json` corrupto/inparseable:** error `"parse config: <detalle>"`, accionable.
- **`specPath` no existe todavía** (repo recién inicializado, sin specs): `examplePath: ""`.
  No es error.
- **Glob de `specPath` lanza error de I/O:** `examplePath: ""` + log al stderr de una línea
  (`"warning: could not glob specPath: …"`). No falla el comando.
- **Manifest presente pero scripts vacíos** (`package.json` sin `scripts`): `buildCmd: ""`.
  No es error.
- **Manifests múltiples** (ej. `go.mod` + `package.json`): prioridad documentada en §5;
  si ambos aportan comandos, el más específico al repo (Makefile > go.mod > package.json >
  pyproject) gana para build; para lint y test se puede mezclar (ej. Makefile sin lint-target
  + go.mod con `golangci-lint run`). TBD — ver Open questions.
- **Makefile presente pero sin targets relevantes:** no aporta; pasa al siguiente manifest.
- **Directorio de repo sin permisos de lectura en un subdir:** la goroutine que falla retorna
  `""` sin propagar el error a las otras goroutines.

### Commands del kit

- **`vector` no encontrado en PATH:** el command del kit emite `"warning: vector binary not
  found; falling back to local derivation"` y continúa con el comportamiento anterior.
- **`vector context --json` retorna exit `1`:** ídem al anterior — advertencia + fallback.
- **`CONTEXT.examplePath` apunta a un archivo que fue borrado** entre la llamada y su uso:
  el refiner/validator recibe `""` como si no hubiera ejemplo. El glob era correcto en su
  momento; la race condition es aceptable.
- **`CONTEXT.buildCmd` es un comando que el repo ya no tiene** (config viejo, manifest
  cambiado): el command del kit lo ejecuta, falla, y lo reporta al usuario con el error real.
  La solución es correr `vector update`. Esto es un caso de cache stale — ver Open questions
  sobre invalidación.

### Sin HTTP surface

Los códigos HTTP (400/401/403/404/409/422/429/500) no aplican — este subcomando es CLI/filesystem.

---

## 12. Estados de UI requeridos

Estados de salida del subcomando/consumo en los commands del kit (no UI web):

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | `vector context` esperando flags | ejecutar con `--json` o sin flags |
| success | JSON o texto con los siete campos | consumir el output en el command del kit |
| partial | JSON con algunos campos vacíos (manifests no detectados) | continuar con fallback; correr `vector update` para re-detectar |
| error — no init | `"no .vector/config.json — run vector init first"` | correr `vector init` |
| error — config corrupto | `"parse config: <detalle>"` | corregir el config.json manualmente o re-inicializar |
| disabled | No aplica — sin componentes UI interactivos | — |
| offline | No aplica — CLI local-only, sin dependencia de red | — |
| empty | `examplePath: ""` cuando no hay specs creados aún | crear el primer spec con `/vector:raw` |

---

## 13. Validaciones

### Validaciones del binario (`runContext`)

| Campo/condición | Regla | Error |
|---|---|---|
| `config.json` presente | requerido | `"no .vector/config.json — run vector init first"` |
| `config.json` parseable | JSON válido + schema conocido | `"parse config: <detalle>"` |
| flags | solo `--repo-root`, `--json` | error de `flag.FlagSet` estándar |

### Validaciones de `DetectBuildCmds`

| Entrada | Regla | Comportamiento |
|---|---|---|
| `repoRoot` | directorio existente | error si no existe (propagado a caller) |
| `go.mod` presente | archivo válido (basta con existir) | inferir comandos Go |
| `package.json` presente | JSON válido con campo `scripts` | leer scripts o `""` si ausente |
| `Makefile` presente | archivo de texto legible | buscar targets; `""` si ninguno |

Los errores de lectura de manifests individuales no se propagan; el campo correspondiente
queda vacío y el proceso continúa.

### Validaciones de config nuevos campos

| Campo | Regla | Comportamiento en Load |
|---|---|---|
| `buildCmd` | string libre, `omitempty` | carga como `""` si ausente; sin validación |
| `lintCmd` | ídem | ídem |
| `testCmd` | ídem | ídem |

No hay allow-list de comandos: el usuario puede setear lo que quiera vía `vector init`.

---

## 14. Seguridad y permisos

- `vector context` es **read-only**: no muta el repo del usuario, no toca `.vector/`, no
  escribe nada.
- Los manifests del usuario son leídos con permisos normales del proceso; ningún manifest se
  ejecuta. `DetectBuildCmds` hace solo `os.ReadFile` y parsing de texto.
- Los campos `buildCmd`/`lintCmd`/`testCmd` son strings libres almacenados en `config.json`.
  Cuando los commands del kit los ejecutan, el riesgo es equivalente al de ejecutar cualquier
  comando del repo — no se introduce una superficie nueva de inyección (el user ya tiene
  control total del `config.json`).
- No se loguean ni exponen los contenidos completos de los manifests del usuario. Solo se
  extraen los comandos inferidos.
- El path de `examplePath` es repo-relativo; no se exponen paths absolutos del sistema del
  usuario en el JSON de output (a menos que `specPath` sea absoluto, lo que no ocurre en la
  configuración por defecto).

---

## 15. Observabilidad y logging

El mecanismo de logging existente es `activity.jsonl`. `vector context` **no appenda eventos**:
es una consulta de lectura, no una acción de dominio.

Si `DetectBuildCmds` encuentra un manifest pero no puede parsearlo (ej. `package.json` con JSON
inválido), loguear a stderr una línea de warning: `"warning: could not parse <file>: <err>"`.
No registrar en `activity.jsonl` — no es un evento de dominio.

Nada que registrar para los comandos del kit: el consumo de `vector context` no es un evento
de dominio observable en el board.

---

## 16. i18n / textos visibles

**El proyecto no tiene sistema de i18n.** El binario emite strings en **inglés hardcodeado**
(consistente con todos los subcomandos existentes). La tabla siguiente son **identificadores de
documentación** de esos strings, no keys de ningún archivo de traducción.

| Identificador (doc) | Texto (hardcoded EN) |
|---|---|
| context.no_config | `"no .vector/config.json in {root} — run vector init first"` |
| context.header | `"vector context: {root}"` |
| context.field_example | `"  examplePath:    {value}"` |
| context.field_language | `"  language:       {value}"` |
| context.field_build | `"  buildCmd:       {value}"` |
| context.field_lint | `"  lintCmd:        {value}"` |
| context.field_test | `"  testCmd:        {value}"` |
| context.field_apply | `"  applyMode:      {value}"` |
| context.field_ticket | `"  ticketDetected: {value}"` |
| context.warn_glob | `"warning: could not glob specPath: {err}"` |
| context.warn_manifest | `"warning: could not parse {file}: {err}"` |

Los commands del kit conversan en el idioma del usuario (o en `config.Language`); los strings
hardcodeados del binario son siempre inglés.

---

## 17. Performance

- **Lectura de `config.json`**: < 1ms (archivo pequeño, local).
- **Glob de `specPath`**: < 50ms en repos típicos (< 500 specs). No camina subdirectorios
  profundos: el glob es de un solo nivel de `specPath`.
- **Detección de manifests con goroutines**: concurrente, < 5 reads de archivo; < 20ms en
  SSD local.
- **Total `vector context --json`**: target < 100ms en un repo Go o Node típico; < 200ms en
  el worst case.
- **Impacto en commands del kit**: la adición de un paso inicial de < 200ms es aceptable dado
  que elimina globeos y detecciones que hoy se repiten dentro de la sesión de Opus.
- Sin llamadas de red (local-only); sin llamadas a modelos.
- `DetectBuildCmds` no camina el árbol de directorios del usuario — solo lee archivos en la
  raíz del repo. `os.Stat` + `os.ReadFile` por manifest candidate.

---

## 18. Restricciones

El agente no debe:

- Escribir en `config.json` desde `runContext` (read-only).
- Agregar dependencias externas a `cli/` (Go stdlib únicamente).
- Cambiar el schema de `SpecState` ni de `activity.jsonl`.
- Cambiar `runSpec`, `runSync`, `runServe`, `runStandup` ni el router de subcomandos más allá
  de añadir el nuevo `case "context"` y actualizar `runInit`/`runUpdate`.
- Modificar el comportamiento de `config.Resolve`, `config.Load`, `config.Write`, `config.FindSpecDocs`
  ni `config.ChangesDirs`.
- Refactorizar los commands del kit distintos de los cuatro enunciados en §2.
- Cambiar el frontmatter de los commands del kit.
- Cambiar la lógica de detección de tickets en `raw.md` (paso 7 del flujo actual): ese flujo
  es independiente y correcto.
- Instalar nuevas dependencias en los commands del kit.
- Inventar comandos de build/lint/test cuando los manifests no los permiten inferir con
  confianza; retornar `""` y dejar que el command del kit maneje el fallback.
- Ignorar errores de lint/typecheck/tests.
- Introducir un sub-paquete nuevo (`internal/context/`) salvo que el agente justifique que
  `context.go` en `cmd/vector/` no es suficiente. La preferencia es el archivo en `cmd/vector/`.

---

## 19. Entregables

- [ ] `cli/cmd/vector/context.go` implementado con `ContextOutput` y `runContext`.
- [ ] `cli/cmd/vector/main.go`: `case "context"` en el switch y `usage()` actualizado.
- [ ] `cli/internal/config/config.go`: campos `BuildCmd`/`LintCmd`/`TestCmd` + `DetectBuildCmds`
  + `ResolvedBuildCmds`.
- [ ] `cli/internal/config/config_test.go`: tests de `DetectBuildCmds` (table-driven, filesystem
  temporal).
- [ ] `cli/cmd/vector/main.go` (`runInit`, `runUpdate`): detección + persistencia de los nuevos
  campos.
- [ ] `kit/commands/vector/raw.md` actualizado (pasos 2–4 reemplazados por `vector context`).
- [ ] `kit/commands/vector/bug.md` actualizado (paso 4 reemplazado).
- [ ] `kit/commands/vector/apply.md` actualizado (paso 0 + gate actualizado).
- [ ] `kit/commands/vector/comment.md` actualizado (§7a.3 actualizado).
- [ ] Assets vendorizados regenerados (`go generate` o copia manual equivalente hasta que el
  script exista).
- [ ] Sin regresiones: `gofmt`/`go vet`/`go test ./...` verdes.
- [ ] `docs/domain-contract.md` o `docs/orchestration-review.md` §9 mencionan `vector context`
  como pre-hook único — verificar consistencia (no editar si ya está documentado correctamente;
  añadir nota si no lo está).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `cli/internal/config/config.go` y confirmé que los campos `BuildCmd`/`LintCmd`/
  `TestCmd` no existen aún (no hay migración de schema).
- [ ] Revisé `cli/cmd/vector/main.go` y confirmé que el `case "context"` no existe aún.
- [ ] Revisé `kit/commands/vector/raw.md`, `bug.md`, `apply.md`, `comment.md` y mapeé
  exactamente qué pasos se reemplazan y cuáles permanecen intactos.
- [ ] Solo modifiqué los archivos listados en §6 o lo justifiqué.
- [ ] Implementé goroutines en `DetectBuildCmds` con `sync.WaitGroup` (stdlib).
- [ ] `runContext` es read-only y retorna exit `0` / `1` según corresponda.
- [ ] `DetectBuildCmds` retorna strings vacíos (sin error) cuando no puede inferir comandos.
- [ ] Los commands del kit actualizados tienen fallback para cuando `CONTEXT` devuelve campos
  vacíos.
- [ ] No cambié decisiones tomadas en §10.
- [ ] No agregué dependencias externas.
- [ ] Ejecuté `gofmt`, `go vet`, `go test ./...`.
- [ ] Los assets del kit en `cli/internal/scaffold/assets/` están sincronizados con los
  sources en `kit/commands/vector/`.
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar.

---

## Open questions

1. **Prioridad de manifests cuando hay múltiples** (ej. `go.mod` + `Makefile`): ¿el Makefile
   siempre gana, o se mezclan por campo (build de Makefile, lint de go.mod)? La heurística
   propuesta en §11 es mezcla por campo; confirmar antes de implementar. TBD — ver §11.
2. **Flags `--build-cmd` / `--lint-cmd` / `--test-cmd` en `runInit`/`runUpdate`**: ¿se añaden
   en esta misma fase o se dejan para una fase de "configuración avanzada"? El spec los incluye
   como implícitos en la decisión de persistencia; si se omiten, la única forma de setear
   valores manuales sería editar `config.json` a mano. TBD — confirmar con el usuario.
3. **`examplePath` como path relativo vs absoluto en el JSON output**: hoy `SpecDoc.Rel` es
   relativo al repo. El command del kit lo usa para `Read` — necesita el path absoluto. ¿El
   binario devuelve el path relativo (más portátil) o absoluto (más conveniente para el
   command)? Propuesta: relativo en JSON + el command prefija `repoRoot`. TBD.
4. **Invalidación de cache cuando el usuario cambia un manifest** (ej. agrega un `Makefile`
   después del `init`): hoy requiere `vector update`. ¿Es suficiente documentarlo o se necesita
   un mecanismo de detección de staleness (ej. comparar `modtime` del manifest vs `config.json`)?
   V1: documentar + `vector update`. TBD para fases posteriores.
