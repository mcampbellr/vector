# Spec: Adoptar cobra + lipgloss en el CLI de Vector

## 1. Objetivo

Reemplazar el dispatch manual (`switch os.Args[1]` + un `flag.NewFlagSet` por subcomando) del
binario `vector` por un árbol de comandos **cobra** (`github.com/spf13/cobra`), y añadir un
paquete nuevo `cli/internal/ui` con **lipgloss** (`github.com/charmbracelet/lipgloss`) que da
estilo (colores, glifos de estado, tablas, pares clave-valor) a la salida **humana** del CLI.

Esta feature permite que un **dev que usa `vector` desde la terminal** obtiene **ayuda
(`--help`) generada automáticamente, completions de shell (`vector completion <shell>`) y una
salida visualmente clara** para el resultado exacto que ya devuelven los comandos actuales,
**sin alterar en absoluto** el contrato `--json` que consumen los project commands `/vector:*`
(`kit/commands/vector/*.md`) — ese stdout debe seguir siendo **byte-idéntico** antes y después
del cambio.

El problema concreto: `cli/cmd/vector/main.go` (~1127 líneas) enruta a mano con
`switch os.Args[1]` (línea 38), cada subcomando construye su propio `flag.NewFlagSet`, y la
ayuda es texto plano hardcodeado en `func usage()` (líneas 1098–1126). No hay completions de
shell, no hay salida con color/estructura, y cada subcomando reimplementa a mano casos borde
del parseo (posicional antes de flags vía el helper `leadingID`, doble-orden en
`spec summarize <id> commit` / `summarize commit <id>`, etc.) porque `flag.FlagSet` de la
stdlib no intercala flags y posicionales.

## 2. Alcance

### Incluido en esta fase

- **Migración completa, en un solo cambio (all-at-once)** del dispatch de `main.go` a un árbol
  `cobra.Command`, cubriendo **todos** los subcomandos y sub-subcomandos hoy documentados en
  `usage()` (`cli/cmd/vector/main.go:1098-1126`): `init`, `update`, `context` (incl. `--for`),
  `sync`, `serve`, `standup` (+ `standup commit`), `spec create|list|propose|apply|fix|link|
  relate|status|close|archive|next|worklog|summarize (+ `summarize commit`)|route|
  attach-sketch`, `detect-ticket`, `version`/`--version`/`-v`, `help`/`-h`/`--help`.
- **Migración 1:1 de cada flag** de cada subcomando (nombre, tipo, default, texto de ayuda) de
  `flag.FlagSet` a las flags nativas de cobra/pflag (`cmd.Flags()`), sin agregar ni quitar
  ninguna flag, sin cambiar ningún default (incluye el caso `context`, cuyo `--json` tiene
  default `true`, a diferencia de todos los demás que son `false` — `cli/cmd/vector/context.go:127`).
- **`var version = "dev"` se preserva tal cual** (`cli/cmd/vector/main.go:29`); `-X
  main.version={{.Version}}` (`.goreleaser.yml:29`) sigue funcionando; `vector version`,
  `vector --version` y `vector -v` imprimen exactamente `vector <version>\n` a stdout,
  exit 0, en cualquier posición de invocación.
- **Nuevo paquete `cli/internal/ui`** (lipgloss): helpers de estilo (`Bold`, `Green`, `Red`,
  `Dim`, `Cyan`), glifos de estado (`Success`, `Info`, `Warning`, `Error`), `Table` y
  `KeyValue` — modelados 1:1 sobre el patrón de referencia externo
  `~/Developer/Personal/flagify/cli/internal/ui/ui.go`. Se aplican **únicamente en la rama de
  salida humana** (nunca dentro de un bloque `if *jsonOut { ... }` / `if jsonOut { ... }`).
- **Ayuda estilizada vía cobra** (`cmd.SetHelpFunc`), modelada sobre
  `~/Developer/Personal/flagify/cli/internal/ui/help.go`, que reemplaza `func usage()`.
  Reemplaza también los mensajes de error de "unknown command"/"unknown flag" con la ayuda de
  cobra allí donde no rompa un exit code documentado (ver §11).
- **`vector completion <bash|zsh|fish|powershell>`** — comando nuevo que envuelve
  `rootCmd.GenBashCompletion` / `GenZshCompletion` / `GenFishCompletion` /
  `GenPowerShellCompletionWithDesc`, modelado sobre
  `~/Developer/Personal/flagify/cli/cmd/completion.go`. Generado on-the-fly; no se embebe nada
  (`.claude/rules/architecture/distribution-packaging.md` no impone requisito de embeber completions).
- **Suite de tests golden/snapshot** que captura el stdout **byte-exacto** de cada comando
  `--json` **antes** de tocar el dispatch, y lo compara **después** de la migración — es el
  gate duro de esta fase (ver §6, §8).
- **Medición de peso del binario release-equivalente**: `CGO_ENABLED=0 go build -ldflags "-s
  -w"` (los mismos flags que `.goreleaser.yml:26-29`), tamaño **antes** vs **después**,
  reportado como criterio de aceptación — **sin umbral duro**; el usuario decide con el número
  en mano ("medir y decidir").
- **Actualización de documentación**: `README.md`, `docs/plugin-and-commands.md`,
  `.claude/rules/architecture/distribution-packaging.md`, `cli/CLAUDE.md`, y el texto de ayuda
  del binario (ahora generado por cobra, ya no `usage()` a mano).
- **Primeras dependencias externas del módulo Go** (`cli/go.mod` hoy no tiene bloque
  `require` — sólo stdlib): `github.com/spf13/cobra` y `github.com/charmbracelet/lipgloss`,
  más sus transitivas resueltas por `go mod tidy`.

### Fuera de scope

- **`huh` y `bubbletea`**: excluidos por completo. El CLI de Vector es no interactivo —
  verificado: no hay `bufio.NewReader`, `ReadPassword` ni ningún prompt en `cli/`. No se agrega
  ningún flujo de pregunta-respuesta en terminal.
- **Fase de coexistencia**: no hay migración incremental ni bandera para alternar entre el
  dispatch viejo y el nuevo. Es un cambio atómico; el repo no debe quedar en un estado híbrido
  mergeado a `main`.
- **Cambiar el contrato `--json`**: ningún campo se renombra, reordena, cambia de tipo, se
  agrega ni se quita en ninguna de las salidas `--json` existentes. El indentado (`"  "`, dos
  espacios, vía `json.MarshalIndent`) y el salto de línea final (`fmt.Println`) se preservan
  porque los helpers `printJSON`/`printJSONValue` (`cli/cmd/vector/main.go:989-1002`) no se
  tocan.
- **Alias de flags cortos nuevos** (p. ej. `-r` para `--repo-root`): cobra los soporta, pero
  esta fase **no** los agrega — se mantiene superficie solo long-form para no ampliar el
  contrato de flags (ver Open questions #2 y Decisiones tomadas).
- **Tocar `internal/state`, `internal/config`, `internal/board`** o cualquier paquete de
  dominio: cobra/lipgloss son una capa de ruteo/adaptador delgada sobre `cmd/vector/`; la
  lógica de negocio de cada `runXxx` (llamadas a `store.*`, `config.*`) permanece intacta.
- **Endpoints HTTP del board** (`internal/board`, `internal/webui`): no se tocan.
- **`golangci-lint`**: no hay config en el repo hoy (`quality/testing-and-review.md`); esta
  fase no introduce una. El gate sigue siendo `gofmt` + `go vet` + `go test`.
- **Windows como plataforma soportada**: `.goreleaser.yml` sólo builda darwin/linux
  (líneas 30-35); la generación de completion `powershell` se incluye porque es texto puro sin
  costo de build cruzado, pero no hay runner de CI en Windows para probarla end-to-end (queda
  cubierta solo por que `GenPowerShellCompletionWithDesc` no retorne error).
- **Salida estilizada para `vector context` / `vector detect-ticket`**: ver Open questions #3 —
  no se decide en esta fase; ambos comandos siguen su comportamiento actual (`context` ya
  tiene `--json` default `true`; ambos están documentados como "for tooling").
- **`scripts/install.ps1` / instalador Windows**: fuera de scope; no se relaciona con este
  cambio (`add-windows-support-distribution` es un spec aparte, visible en el working tree).

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Lenguaje: **Go**, módulo único en `cli/` (`cli/go.mod:1-3`: `module
  github.com/mariocampbell/vector`, `go 1.26`). Hoy **sin dependencias externas** — el
  `go.mod` no tiene bloque `require`; todo es stdlib. Este cambio introduce las **primeras**
  dependencias externas del módulo.
- CLI framework: **cobra** — reemplaza `flag.FlagSet` (stdlib) como router y motor de flags.
- Estilo de salida: **lipgloss** — nuevo, sólo para output humano.
- Config/estado: sin cambios — `cli/internal/config`, `cli/internal/state` (CLI-owns-writes,
  `architecture/state-model.md`).
- Tests: paquete `testing` estándar, table-driven (patrón ya usado en
  `cli/cmd/vector/main_test.go`, `spec_transitions_test.go`, etc.).
- Build/release: GoReleaser v2 (`.goreleaser.yml`), `CGO_ENABLED=0`, `-ldflags "-s -w -X
  main.version={{.Version}}"`, darwin/linux × amd64/arm64.

### Versiones relevantes

- Go: **1.26** (`cli/go.mod:3`) — se mantiene.
- `github.com/spf13/cobra`: **v1.10.2** — versión confirmada en uso en el repo de referencia
  externo `~/Developer/Personal/flagify/cli/go.mod:6`. Al correr `go get
  github.com/spf13/cobra@v1.10.2 && go mod tidy` en `cli/`, `go.sum` fija el árbol exacto;
  si en el momento de implementar existe un patch más nuevo de la v1.10.x, usar ese (cobra
  sigue semver estricto en la v1).
- `github.com/charmbracelet/lipgloss`: **v1.1.0** — misma referencia
  (`~/Developer/Personal/flagify/cli/go.mod:19`, ahí listada como `// indirect` porque en
  flagify sólo llega vía `huh`; en Vector es **dependencia directa** de `cli/internal/ui`).
- Transitivas exactas: **no se re-derivan aquí**. El árbol de flagify (`go.mod` líneas 11-45)
  incluye paquetes que sólo `huh`/`bubbletea` arrastran (`charmbracelet/bubbletea`,
  `charmbracelet/bubbles`, `charmbracelet/huh`, `catppuccin/go`, `mitchellh/hashstructure`,
  `pkg/browser`, etc.) — Vector **no** los necesita porque no adopta huh/bubbletea. El árbol
  real de `cobra` + `lipgloss` solos (sin huh/bubbletea) se resuelve corriendo `go mod tidy`
  durante la implementación; se espera que incluya como mínimo `spf13/pflag` (cobra),
  `mattn/go-isatty`, `charmbracelet/x/ansi`, `charmbracelet/x/term`, `lucasb-eyer/go-colorful`,
  `muesli/termenv` o su sucesor `charmbracelet/colorprofile` (lipgloss) — confirmar el listado
  final contra `cli/go.sum` generado, no contra esta lista.
- `charmbracelet/lipgloss/table` (subpaquete usado por `ui.Table`, ver `ui.go:7` en la
  referencia): vive en el mismo módulo `charmbracelet/lipgloss`, sin versión propia.
- Licencias: cobra es **Apache-2.0**, lipgloss es **MIT** — ambas compatibles con la licencia
  del repo (`LICENSE`, Apache-2.0) y con distribución comercial
  (`architecture/distribution-packaging.md`).

No usar librerías, APIs, flags o patrones que no estén documentados oficialmente o que no estén
ya presentes en el proyecto, salvo lo que este spec autoriza explícitamente (cobra + lipgloss,
nada más).

### Patrones existentes a respetar

- **CLI-owns-writes**: el binario sigue siendo el único escritor de `.vector/config.json` y del
  estado; cobra no cambia quién escribe qué.
- **Naming kebab-case** para flags de cara al usuario (`--repo-root`, `--dry-run`, etc.) — se
  preserva 1:1; cobra/pflag no imponen convención propia.
- **Helpers compartidos que NO se tocan**: `resolveRepoRoot` (`main.go:964-974`),
  `resolveActor` (`main.go:977-987`), `printJSON`/`printJSONValue` (`main.go:989-1002`),
  `leadingID` (`spec_transitions.go:17-22`, aunque su *necesidad* cambia — ver §5 y §11),
  `openStore` (`spec_transitions.go:421-427`).
- **Errores explícitos envueltos con `%w`**: se preserva; los `RunE` de cobra devuelven el
  mismo tipo de error que hoy devuelven los `runXxx`.
- **Token routing**: no aplica — este cambio no invoca modelos ni agentes; es infraestructura
  pura del binario.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Dispatch actual completo y funcional en `cli/cmd/vector/main.go` (switch en línea 38,
      `usage()` en líneas 1098-1126).
- [x] Todos los `runXxx` de negocio ya implementados y testeados: `runInit`/`runUpdate`/
      `runSync`/`runSpec*`/`runDetectTicket` (`main.go`), `runSpecApply/Link/Relate/Status/
      Close/Archive/Next/Fix` (`cli/cmd/vector/spec_transitions.go`), `runContext`
      (`cli/cmd/vector/context.go`), `runServe` (`cli/cmd/vector/serve.go`), `runStandup`/
      `runStandupCommit` (`cli/cmd/vector/standup.go`), `runSpecRoute`
      (`cli/cmd/vector/route.go`), `runSpecAttachSketch` (`cli/cmd/vector/sketch.go`),
      `runSpecSummarize`/`runSpecSummarizeCommit` (`cli/cmd/vector/summarize.go`).
- [x] Tests existentes que invocan estos `runXxx(args []string)` directamente (sin pasar por
      `main()`/`os.Args`): `main_test.go`, `sync_test.go`, `standup_test.go`,
      `init_language_test.go`, `related_test.go`, `ticket_test.go`, `summarize_test.go`,
      `spec_fix_test.go`, `spec_transitions_test.go`, `sketch_test.go`, `context_test.go` — un
      helper compartido `captureStdout(t, func() error {...})` es el patrón de captura usado en
      varios de ellos (p. ej. `init_language_test.go:13`, `context_test.go:42-44`). **Estos
      once archivos quedan dentro del blast radius de esta migración** porque la firma de los
      `runXxx` cambia (ver §5) — el usuario ya autorizó "rewrite the dispatch tests".
- [x] `.goreleaser.yml` con los ldflags `-s -w -X main.version={{.Version}}` (líneas 26-29) —
      referencia para medir el binario release-equivalente.
- [x] Repo de referencia externo `~/Developer/Personal/flagify/cli` con el patrón cobra +
      lipgloss ya en producción (`cmd/root.go`, `cmd/completion.go`, `internal/ui/ui.go`,
      `internal/ui/help.go`) — **no** es parte de este repo, sólo referencia de patrón; no se
      importa código de ahí, se replica el patrón dentro de `cli/`.

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta. No
debe inventar contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

**Cobra como capa de ruteo/adaptador delgada; la lógica de negocio no se mueve.** Cada
subcomando sigue teniendo una función `runXxx` que hace exactamente lo que hoy hace entre el
`fs.Parse(args)` y el `return` (llamadas a `state.Open`, `store.*`, `config.*`,
`printJSON(Value)`); lo único que cambia es **cómo llega esa función a sus valores de flags**:

- **Antes**: `fs := flag.NewFlagSet(...)`; `x := fs.String("x", ..., ...)`; `fs.Parse(args)`;
  el resto del código lee `*x`.
- **Después**: cada comando se construye con una función factory `newXxxCmd() *cobra.Command`
  que registra las mismas flags vía `cmd.Flags().StringVar(&x, "x", ..., ...)` (mismo nombre,
  default, texto de ayuda) y fija `RunE: func(cmd *cobra.Command, args []string) error { ...
  cuerpo actual de runXxx, ya sin fs.Parse ... }`. El cuerpo de negocio entre el parseo y el
  `return` **no cambia una sola línea** salvo la sustitución `*x` → `x` (ya no hay puntero,
  `StringVar` escribe directo en la variable capturada por el closure).
- **Test harness nuevo**: para no perder la capacidad de testear cada comando en aislamiento
  (patrón hoy: `runContext([]string{"--repo-root", root, "--json"})`), cada archivo de test que
  hoy llama a un `runXxx(args)` pasa a llamar a un `execXxxCmd(t, args...)` que construye el
  `cobra.Command` vía su factory, hace `cmd.SetArgs(args)`, `cmd.SetOut(&buf)`,
  `cmd.SetErr(&errBuf)` y `cmd.Execute()`. `captureStdout` se preserva donde el comando sigue
  escribiendo con `fmt.Println`/`fmt.Printf` directo a `os.Stdout` (no a `cmd.OutOrStdout()`) —
  **decisión**: los `RunE` siguen escribiendo con `fmt.Print*` a `os.Stdout`/`os.Stderr`
  directamente (como hoy), no a `cmd.OutOrStdout()`, para que `--json` stdout no dependa de
  wiring de cobra y el gate de bytes-idénticos sea más fácil de razonar; sólo la **ayuda**
  (`cmd.Help()`) y los **mensajes de error de cobra** (flag/comando desconocido) usan
  `cmd.OutOrStderr()`.
- **`main.go`** pasa a ser mínimo: construye el `rootCmd` (`newRootCmd()`), llama
  `rootCmd.Execute()`, y mapea el error resultante a un exit code (0/1/2) replicando la lógica
  actual de `main()` (líneas 31-69) — ver §11 para el mapeo exacto de exit codes.

### Capas afectadas

- **`cmd/vector/` (capa CLI)**: sí — es la capa que se reescribe. Cada archivo que hoy define
  un `runXxx` con `flag.FlagSet` propio pasa a definir además su `newXxxCmd()`.
- **`internal/ui/` (nueva)**: sí — paquete nuevo, sin lógica de dominio, sólo formateo de
  strings para terminal.
- **`internal/state`, `internal/config`, `internal/board`, `internal/openspec`,
  `internal/scaffold`, `internal/standup`, `internal/webui`, `internal/intel`**: **no** — cero
  cambios. Ningún estos paquetes importa `cobra` ni `lipgloss`.
- **`kit/commands/vector/*.md`**: no — los project commands invocan el binario por su
  interfaz externa (`vector spec create --title "..." --json`), que no cambia.

### Flujo esperado (ejemplo: `vector spec apply my-spec --json`)

1. `main()` construye `rootCmd := newRootCmd()` (árbol completo registrado vía `init()` /
   `AddCommand` en cada archivo de comando) y llama `rootCmd.Execute()`.
2. cobra resuelve la ruta `spec → apply`, parsea flags (`pflag`, que sí intercala posicionales
   y flags, a diferencia de `flag`), y llama al `RunE` de `apply` con `args = ["my-spec"]`
   (el positional sobrante tras el parseo de flags).
3. El `RunE` reusa la lógica exacta de hoy: `leadingID`-equivalente sobre `args` para extraer
   `my-spec` como id, abre el store (`openStore`), llama `store.ApplySpec(...)`.
4. Como `--json` es `true`, el `RunE` llama `printJSON(...)` — **sin pasar por ningún helper de
   `internal/ui`** — exactamente igual que hoy.
5. `rootCmd.Execute()` retorna `nil`; `main()` sale con exit 0.

### Flujo esperado (ejemplo: `vector spec apply my-spec`, sin `--json`, TTY)

1-3. Igual que arriba.
4. El `RunE`, en la rama humana, envuelve el mensaje de confirmación con
   `ui.Success(fmt.Sprintf("applied spec %q (status: open → in-progress)\n  change: %s",
   updated.ID, change))` en vez del `fmt.Printf` plano actual — lipgloss decide internamente si
   emite códigos ANSI (TTY sin `NO_COLOR`) o texto plano (pipe, `NO_COLOR=1`, `TERM=dumb`).
5. Exit 0.

### Ubicación de archivos nuevos

```txt
cli/
  cmd/vector/
    root.go          # NUEVO — rootCmd, exit-code mapping, -v/--version/help wiring
    completion.go     # NUEVO — `vector completion <shell>`
    main.go           # MODIFICAR — se reduce a construir rootCmd + mapear exit code
    context.go         # MODIFICAR — runContext → newContextCmd
    serve.go           # MODIFICAR — runServe → newServeCmd
    standup.go          # MODIFICAR — runStandup/runStandupCommit → newStandupCmd (+ child "commit")
    route.go             # MODIFICAR — runSpecRoute → newSpecRouteCmd
    sketch.go             # MODIFICAR — runSpecAttachSketch → newSpecAttachSketchCmd
    summarize.go           # MODIFICAR — runSpecSummarize(+Commit) → newSpecSummarizeCmd (ver §11: doble-orden)
    spec_transitions.go     # MODIFICAR — apply/link/relate/status/close/archive/next/fix → sus newXxxCmd
    *_test.go                # MODIFICAR (11 archivos, ver §4) — nuevo harness execXxxCmd
    testdata/golden/           # NUEVO — snapshots --json byte-exactos
  internal/ui/
    ui.go              # NUEVO — Bold/Green/Red/Dim/Cyan, Success/Info/Warning/Error, Table, KeyValue
    help.go             # NUEVO — cmd.SetHelpFunc styled
    ui_test.go            # NUEVO
```

No se crean carpetas de dominio nuevas; todo vive dentro de `cmd/vector/` (capa CLI existente)
e `internal/ui/` (paquete de presentación nuevo, análogo en rol a `internal/webui/` pero para
terminal en vez de HTTP).

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `cli/go.mod` | MODIFICAR | Añadir `require github.com/spf13/cobra v1.10.2` y `require github.com/charmbracelet/lipgloss v1.1.0` (+ transitivas vía `go mod tidy`) | `~/Developer/Personal/flagify/cli/go.mod` (referencia externa) |
| `cli/go.sum` | MODIFICAR (generado) | Checksums de las nuevas dependencias; regenerado por `go mod tidy`, no editar a mano | — |
| `cli/cmd/vector/root.go` | NUEVO | `newRootCmd()`, wiring de `-v/--version/-h/--help`, mapeo de exit codes 0/1/2, `ui.ApplyCustomHelp` | `~/Developer/Personal/flagify/cli/cmd/root.go` |
| `cli/cmd/vector/completion.go` | NUEVO | `vector completion <bash\|zsh\|fish\|powershell>` | `~/Developer/Personal/flagify/cli/cmd/completion.go` |
| `cli/cmd/vector/main.go` | MODIFICAR | Reducir `main()` a construir `rootCmd` + `Execute()` + exit code; eliminar `switch os.Args[1]` (línea 38) y `func usage()` (líneas 1098-1126); `runInit`/`runUpdate`/`runSync`/`runSpecCreate`/`runSpecList`/`runDetectTicket`/`runSpec` pasan a `newInitCmd`/`newUpdateCmd`/`newSyncCmd`/`newSpecCreateCmd`/`newSpecListCmd`/`newDetectTicketCmd`/`newSpecCmd` (parent) | Su propio contenido actual (líneas 75-1096) |
| `cli/cmd/vector/context.go` | MODIFICAR | `runContext` → `newContextCmd`; preservar default `--json=true` (línea 112) y `--for` (línea 114) | Su propio contenido actual |
| `cli/cmd/vector/serve.go` | MODIFICAR | `runServe` → `newServeCmd`; preservar detección de `--port` explícito vía `fs.Visit` (líneas 41-46, reemplazar por `cmd.Flags().Changed("port")`) | Su propio contenido actual |
| `cli/cmd/vector/standup.go` | MODIFICAR | `runStandup`/`runStandupCommit` → `newStandupCmd` con child `AddCommand(newStandupCommitCmd())` (orden único "commit primero", sin ambigüedad — ver §11) | Su propio contenido actual |
| `cli/cmd/vector/route.go` | MODIFICAR | `runSpecRoute` → `newSpecRouteCmd` | Su propio contenido actual |
| `cli/cmd/vector/sketch.go` | MODIFICAR | `runSpecAttachSketch` → `newSpecAttachSketchCmd` | Su propio contenido actual |
| `cli/cmd/vector/summarize.go` | MODIFICAR | `runSpecSummarize`/`runSpecSummarizeCommit` → `newSpecSummarizeCmd`, que **no** delega el "commit" a un cobra child puro sino que preserva el `RunE` con detección manual de ambas órdenes (`args[0]=="commit"` y `id,"commit"`, líneas 56-62) — ver §11 | Su propio contenido actual |
| `cli/cmd/vector/spec_transitions.go` | MODIFICAR | `runSpecApply/Link/Relate/Status/Close/Archive/Next/Fix` → sus `newSpecXxxCmd`; `leadingID` se reusa contra el `args []string` que entrega cobra tras el parseo de flags (ya no es pre-parseo) | Su propio contenido actual |
| `cli/cmd/vector/main_test.go` | MODIFICAR | Nuevas pruebas de dispatch de la raíz: comando desconocido, `spec` sin subverbo, `vector` sin args, exit codes | Su propio contenido actual (patrón table-driven `TestParseArtifacts`) |
| `cli/cmd/vector/context_test.go` | MODIFICAR | `runContext([]string{...})` → `execContextCmd(t, ...)` | Su propio contenido actual (`captureStdout`, línea 42-44) |
| `cli/cmd/vector/standup_test.go` | MODIFICAR | Idem para `runStandup`/`runStandupCommit` | Su propio contenido actual |
| `cli/cmd/vector/sync_test.go` | MODIFICAR | `runSync([]string{"--repo-root", root})` (línea 103) → `execSyncCmd(t, ...)`; cumple el mismo criterio de inclusión de §4 que el resto de los tests que invocan `runXxx` con args | Su propio contenido actual |
| `cli/cmd/vector/init_language_test.go` | MODIFICAR | `runInitQuiet`/`runUpdateQuiet` (líneas 11-19) actualizan su llamada interna | Su propio contenido actual |
| `cli/cmd/vector/related_test.go` | MODIFICAR | Idem para el/los `runXxx` que ejercite | Su propio contenido actual |
| `cli/cmd/vector/ticket_test.go` | MODIFICAR | Sólo si ejercita `runDetectTicket` vía args (las pruebas de `inferProvider`/`parseRef` no cambian, son funciones puras sin flags) | Su propio contenido actual |
| `cli/cmd/vector/summarize_test.go` | MODIFICAR | Cubrir explícitamente ambas órdenes de `commit` tras la migración | Su propio contenido actual |
| `cli/cmd/vector/spec_fix_test.go` | MODIFICAR | `runSpecFix` → `execSpecFixCmd` | Su propio contenido actual |
| `cli/cmd/vector/spec_transitions_test.go` | MODIFICAR (parcial) | Sólo si alguna prueba invoca un `runXxx` con args (las de `parseFixArtifacts` son funciones puras, no cambian) | Su propio contenido actual |
| `cli/cmd/vector/sketch_test.go` | MODIFICAR | `runSpecAttachSketch` → `execSpecAttachSketchCmd` | Su propio contenido actual |
| `cli/cmd/vector/testutil_test.go` | NUEVO | Helper compartido `execCmd(t, factory func() *cobra.Command, args ...string) (stdout, stderr string, err error)` reusado por todos los `*_test.go` de arriba | `cli/cmd/vector/init_language_test.go` (patrón `runInitQuiet`/`captureStdout` a reemplazar/envolver) |
| `cli/cmd/vector/testdata/golden/*.json` | NUEVO | Un archivo por comando `--json` (ver lista en §8), capturado **antes** de tocar el dispatch | No hay análogo previo en el repo; primer snapshot suite del CLI |
| `cli/cmd/vector/golden_test.go` | NUEVO | `TestJSONGoldenUnchanged`: por cada comando `--json`, ejecuta el comando y compara byte a byte contra `testdata/golden/<comando>.json` | `context_test.go` (`TestContextBackwardCompat`, patrón de invocar + parsear JSON) |
| `cli/internal/ui/ui.go` | NUEVO | `Bold/Green/Red/Dim/Cyan`, `Success/Info/Warning/Error`, `Table`, `KeyValue` | `~/Developer/Personal/flagify/cli/internal/ui/ui.go` |
| `cli/internal/ui/help.go` | NUEVO | `ApplyCustomHelp(cmd *cobra.Command)` + función de render de ayuda estilizada | `~/Developer/Personal/flagify/cli/internal/ui/help.go` |
| `cli/internal/ui/ui_test.go` | NUEVO | Tests de que cada helper produce texto no vacío; `Table` incluye los headers; degradación a texto plano fuera de TTY | No hay análogo previo; ver `~/Developer/Personal/flagify/cli/internal/ui/ui.go` como referencia del comportamiento a testear |
| `README.md` | MODIFICAR | Sección "Installation"/"From source": mencionar `vector completion <shell>`; "Commands Reference": no cambia la tabla de `/vector:*` pero se añade una nota sobre `vector --help`/`vector completion` como superficie de terminal | Su propio contenido actual (secciones "Installation", "Commands Reference") |
| `docs/plugin-and-commands.md` | MODIFICAR | La fila "Binario Go" de la tabla de "Dos superficies distintas" (líneas 9-12) gana una nota: ahora expone `--help`/`completion` generados por cobra | Su propio contenido actual |
| `.claude/rules/architecture/distribution-packaging.md` | MODIFICAR | Registrar que el binario ya no es 100% stdlib (primeras deps externas: cobra + lipgloss); completions se generan on-the-fly, no se embeben; nota de peso medido | Su propio contenido actual (sección "Implicaciones para el desarrollo") |
| `cli/CLAUDE.md` | MODIFICAR | Línea "Go (módulo único, stdlib, sin deps externas)" (línea 41-42) deja de ser cierta — actualizar a "Go (módulo único; deps externas: cobra + lipgloss desde adopt-cobra-lipgloss-cli)"; documentar `internal/ui` en la lista de paquetes ("Estado actual") | Su propio contenido actual |

### Detalle por archivo

#### cli/cmd/vector/root.go

Acción: NUEVO

Debe implementar:

- `func newRootCmd() *cobra.Command`: construye un árbol **fresco** en cada llamada (no un
  singleton a nivel de paquete) — necesario para que los tests puedan ejecutar comandos en
  paralelo/aislados sin estado compartido entre invocaciones (a diferencia del patrón de
  flagify, que usa un `var rootCmd` de paquete porque su CLI no se testea invocando el árbol
  completo).
- `Use: "vector"`, `Short`/`Long` tomados literalmente de la primera línea de `usage()`
  (`main.go:1099`): `"vector — developer-focused spec/kanban companion for Claude Code"`.
- `SilenceErrors: true`, `SilenceUsage: true` (patrón de flagify, `root.go:12-13`) porque el
  mapeo de exit codes y el mensaje de error se controlan a mano en `main()`, no por el print
  automático de cobra.
- Registro de todos los `newXxxCmd()` vía `AddCommand`.
- `ui.ApplyCustomHelp(root)` (aplica recursivamente a subcomandos, patrón de
  `~/Developer/Personal/flagify/cli/internal/ui/help.go:41-43`).
- **`-v`/`--version`/subcomando `version`**: cobra ofrece un flag `--version` automático que
  imprime `<name> version <Version>` — formato distinto al actual (`vector <version>`). Debe
  **desactivarse** (no fijar `rootCmd.Version`) e implementarse a mano: un flag persistente
  booleano `-v`/`--version` en un `PersistentPreRunE` de la raíz que, si está seteado, imprime
  `fmt.Println("vector", version)` a stdout y retorna un sentinel (`errHandled` o similar) que
  `main()` traduce a exit 0 sin ejecutar el subcomando; más el subcomando explícito `version`
  con el mismo cuerpo.
- Mapeo de exit codes: `func Execute() (exitCode int)` o el propio `main()` inspecciona el
  error retornado por `rootCmd.Execute()`. Casos: `nil` → 0; error marcado como "uso inválido"
  (comando desconocido, `vector` sin args, `spec` sin subverbo) → 2 (paridad con
  `main.go:34,62`); cualquier otro error de negocio → 1 (paridad con `main.go:67`). Ver §11
  para la lista exhaustiva de casos a exit 2 vs 1.

Debe seguir como referencia:

- `~/Developer/Personal/flagify/cli/cmd/root.go`

No debe incluir:

- `rootCmd.PersistentFlags()` nuevas que no existan hoy (Vector no tiene flags globales tipo
  `--profile`/`--workspace` de flagify; `--repo-root`/`--json` siguen siendo **flags locales
  por subcomando**, no persistentes en la raíz — decisión explícita, ver §10, para no alterar
  el `--help` de comandos que hoy no aceptan `--repo-root`, p. ej. `version`).

#### cli/cmd/vector/completion.go

Acción: NUEVO

Debe implementar:

- `vector completion bash|zsh|fish|powershell`, `Args` validator que exige exactamente un
  argumento de la lista (mensaje de error si falta), `RunE` que despacha a
  `rootCmd.GenBashCompletion(os.Stdout)` / `GenZshCompletion` / `GenFishCompletion(os.Stdout,
  true)` / `GenPowerShellCompletionWithDesc(os.Stdout)`.

Debe seguir como referencia:

- `~/Developer/Personal/flagify/cli/cmd/completion.go` (copiar el patrón casi literal;
  cambiar `flagify` por `vector` en `Long`).

No debe incluir:

- Embebido de los scripts generados (se generan on-the-fly, `architecture/distribution-packaging.md`).

#### cli/internal/ui/ui.go

Acción: NUEVO

Debe implementar:

- `Bold(s string) string`, `Green`, `Red`, `Dim`, `Cyan` — wrappers de `lipgloss.NewStyle()`.
- `Success(msg string) string` (glifo `✓`), `Info` (`●`), `Warning` (`⚠`), `Error` (`✗`).
- `Table(headers []string, rows [][]string) string` vía `github.com/charmbracelet/lipgloss/table`.
- `KeyValue(label, value string) string`.
- Paleta de colores: **TBD — ver Open questions** (#1: reusar los hex tokens de flagify o
  definir paleta propia de Vector). Hasta resolverse, usar los mismos tokens de
  `~/Developer/Personal/flagify/cli/internal/ui/ui.go:11-15` (`cyan #00D4FF`, `green #00CC88`,
  `red #FF6B6B`, `yellow #FFCC00`, `dim #666666`) como placeholder documentado, no como
  decisión de marca definitiva.

Debe seguir como referencia:

- `~/Developer/Personal/flagify/cli/internal/ui/ui.go`

No debe incluir:

- `AddFormatFlag`/`IsJSON`/`PrintJSON` (equivalentes al `format.go` de flagify): Vector usa un
  flag booleano `--json` por comando, no un `--format table|json`; no se replica ese archivo.
- Ningún import de `huh`/`bubbletea`.

#### cli/internal/ui/help.go

Acción: NUEVO

Debe implementar:

- `ApplyCustomHelp(cmd *cobra.Command)` que hace `cmd.SetHelpFunc(customHelp)`.
- `customHelp` con las mismas secciones que la referencia (USAGE, COMMANDS, FLAGS, GLOBAL
  FLAGS, EXAMPLES, hint final) pero con el nombre `vector` en vez de `flagify` y sin sección
  "GLOBAL FLAGS" si `vector` no termina teniendo persistent flags (ver root.go — probablemente
  vacía o ausente, dado que no hay flags globales).
- Salida escrita a `cmd.OutOrStderr()` para preservar que la ayuda de hoy va a **stderr**
  (`usage()` usa `fmt.Fprint(os.Stderr, ...)`, `main.go:1099`) — decisión explícita (ver §11).

Debe seguir como referencia:

- `~/Developer/Personal/flagify/cli/internal/ui/help.go`

No debe incluir:

- Lógica de negocio ni acceso a `internal/state`/`internal/config`.

---

## 7. API Contract

No aplica — no hay ningún endpoint HTTP involucrado en este cambio (`internal/board`,
`internal/webui` no se tocan). El contrato que **sí** es duro aquí es interno: el **shape
byte-exacto del stdout de cada comando `--json`**, consumido por los project commands
`/vector:*` vía subprocess (p. ej. `vector context --json`, `vector spec create ... --json`,
`vector detect-ticket --json`). Comandos con salida `--json` a preservar exactamente:

- `vector init --json` / `vector update --json`
- `vector sync --json`
- `vector context --json` (incl. `--for <command>`)
- `vector standup --json`
- `vector spec create --json`, `spec list --json`, `spec propose --json`, `spec apply --json`,
  `spec link --json`, `spec relate --json`, `spec status --json`, `spec close --json`, `spec
  archive --json`, `spec next --json`, `spec worklog --json`, `spec summarize --json`, `spec
  summarize commit --json`, `spec route --json`, `spec attach-sketch --json`
- `vector detect-ticket --json`

Ningún campo se infiere, renombra, reordena ni cambia de tipo. El indentado (dos espacios,
`json.MarshalIndent(v, "", "  ")`) y el salto de línea final (`fmt.Println`) no cambian porque
`printJSON`/`printJSONValue` (`main.go:989-1002`) no se modifican.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `vector --help`, `vector <cmd> --help` y `vector <cmd> <subcmd> --help` producen ayuda
      generada por cobra (vía `internal/ui`), y `func usage()` fue eliminada de `main.go`.
- [ ] `vector completion bash|zsh|fish|powershell` genera un script sin error para cada shell.
- [ ] Cada subcomando y cada flag documentados en el `usage()` actual (`main.go:1098-1126`)
      siguen existiendo, con el mismo nombre, tipo y default.
- [ ] `vector version`, `vector --version`, `vector -v` imprimen `vector <version>` (stdout,
      exit 0) en cualquier posición de invocación; `-X main.version=v1.2.3` sigue
      sobreescribiendo `version` correctamente (verificado con un build de prueba).
- [ ] **Todos** los golden tests de `--json` (lista de §7) pasan: el stdout de cada comando es
      **byte-idéntico** al capturado antes de la migración.
- [ ] `CGO_ENABLED=0 go build -ldflags "-s -w"` (release-equivalente, **no** el binario de dev
      sin strip) se mide antes y después de la migración; ambos números se reportan
      explícitamente (bytes y % de delta) — sin umbral de aprobación automática; el usuario
      decide con el número.
- [ ] `gofmt -l cli` no lista archivos; `go -C cli vet ./...` limpio; `go -C cli test ./...`
      verde; `go -C cli build ./...` exitoso.
- [ ] `go -C cli mod tidy` no deja `go.sum` sucio (corrido y committeado).
- [ ] Los casos borde de parseo enumerados en §11 (comando desconocido, `spec` sin subverbo,
      `vector` sin args, flag desconocida, doble-orden de `summarize commit`) mantienen el
      exit code documentado hoy.
- [ ] `README.md`, `docs/plugin-and-commands.md`,
      `.claude/rules/architecture/distribution-packaging.md`, `cli/CLAUDE.md` actualizados.

### Tests requeridos

Agregar o actualizar tests para:

- [ ] **Golden `--json`** (`cli/cmd/vector/golden_test.go` + `testdata/golden/*.json`): un caso
      por cada comando listado en §7, comparación byte a byte.
- [ ] **Dispatch de la raíz** (`main_test.go`): `vector` sin args → exit 2; `vector foo`
      (comando desconocido) → exit 2, mensaje a stderr; `vector spec` sin subverbo → exit 1
      (paridad con el error actual `"usage: vector spec <create|list|...>"`); `vector spec
      bogus` (subverbo desconocido) → error, mismo mensaje que hoy
      (`fmt.Errorf("unknown spec subcommand %q", args[0])`, `main.go:633`) o equivalente
      cobra, exit 1.
- [ ] **Flags**: flag desconocida en un subcomando de hoja (p. ej. `spec create --bogus`) →
      exit 1, error a stderr, no ejecuta el `RunE`.
- [ ] **`context --json` default**: `vector context` sin ningún flag emite JSON (default
      `true` preservado).
- [ ] **`serve --port` explícito vs fallback**: `cmd.Flags().Changed("port")` reemplaza
      `fs.Visit` (`serve.go:41-46`) preservando el mismo comportamiento de fallback a puerto
      libre sólo cuando `--port` no fue pasado explícitamente.
- [ ] **`summarize commit` doble-orden**: `vector spec summarize commit <id> ...` y `vector
      spec summarize <id> commit ...` producen el mismo resultado (test explícito de paridad,
      cubriendo el caso ya testeado hoy — verificar en `summarize_test.go` si ya existe y
      extenderlo si no).
- [ ] **`internal/ui`**: cada helper (`Bold/Green/Red/Dim/Cyan/Success/Info/Warning/Error`)
      retorna string no vacío conteniendo el input; `Table` incluye los headers en el render;
      ningún helper de `internal/ui` es invocado dentro de una rama `if jsonOut`/`if *jsonOut`
      (verificable por inspección de los `RunE`, no necesariamente un test automatizado — dejar
      como parte del checklist de revisión, §20).
- [ ] **Version/ldflag**: build de prueba con `-X main.version=v9.9.9-test`, ejecutar `vector
      version` y `vector --version`, assert `vector v9.9.9-test`.

### Comandos de verificación

Ejecutar:

```bash
gofmt -l cli
go -C cli vet ./...
go -C cli test ./...
go -C cli build ./...
go -C cli mod tidy   # y verificar que go.sum queda sin diff tras un segundo `go mod tidy`

# Medición de peso — release-equivalente, NO el binario de dev sin strip:
CGO_ENABLED=0 go -C cli build -ldflags "-s -w" -o /tmp/vector-before ./cmd/vector   # en el commit previo a este cambio
CGO_ENABLED=0 go -C cli build -ldflags "-s -w" -o /tmp/vector-after ./cmd/vector    # tras el cambio
ls -la /tmp/vector-before /tmp/vector-after   # reportar bytes + % delta
```

La fase no está completa si alguno de los comandos de verificación falla, si `gofmt -l` lista
archivos, o si `go mod tidy` deja `go.sum` con diff.

---

## 9. Criterios de UX

Reinterpretado para una CLI (no hay formularios/pantallas): la "UX" es la salida de terminal.

### Salida humana vs `--json`

- Ningún llamado a `internal/ui.*` ocurre dentro de una rama `--json`; el JSON permanece texto
  plano determinista, sin colorear, indistinguible de hoy byte a byte (ver §7, §8).
- lipgloss detecta automáticamente terminal sin color (`NO_COLOR`, `TERM=dumb`, stdout no-TTY
  por pipe/redirección) y degrada a texto plano — comportamiento heredado de la librería
  (`~/Developer/Personal/flagify/cli/internal/ui/ui.go` no implementa detección manual,
  confía en el motor de color de lipgloss). Esto es defensa en profundidad **además** de la
  regla anterior (nunca llamar `ui.*` en la rama `--json`), no un sustituto de ella.

### Mensajes a estilizar (rama humana únicamente)

- Confirmaciones de éxito (`created spec %q`, `applied spec %q`, `linked spec %q`, `related
  spec %q`, `proposed spec %q`, `closed`/`archived spec %q`, `recorded fix for spec %q`) →
  `ui.Success(...)`.
- Mensajes de "no-op"/"sin cambios" (`"spec %q already linked..."`, `"already open (no
  change)"`) → `ui.Info(...)`.
- `fmt.Fprintf(os.Stderr, "warning: ...")` existentes (p. ej. `serve.go:74-76`,
  `context.go:211,251,268,333`, `main.go:831`) → `ui.Warning(...)`, siguen yendo a stderr.
- El branch de error de `main()` (`fmt.Fprintln(os.Stderr, "error:", err)`, `main.go:66`) →
  `ui.Error(err.Error())` a stderr, mismo exit code.
- `vector spec list` (sin `--json`) → `ui.Table([]string{"ID","Status","Priority","Title"},
  rows)` reemplaza el `Printf` de columnas fijas (`main.go:938`).
- Reportes de línea única tipo `label: value` (p. ej. `init`'s `agent prose language: %s`,
  `main.go:167`) → `ui.KeyValue(...)` donde encaje naturalmente sin reescribir la lógica de
  condicionales que decide si la línea se imprime.

### Ayuda

- `vector --help` y `vector <cmd> --help` muestran secciones USAGE/COMMANDS/FLAGS con estilo,
  reemplazando el bloque de texto plano de `usage()`.
- La ayuda se sigue escribiendo a **stderr** (paridad con el comportamiento actual de
  `usage()`), no a stdout — para no romper scripts que hoy asumen stdout limpio salvo para
  `--json`/salida normal de comando.

### Accesibilidad

- No aplica en el sentido de UI web (no hay lectores de pantalla de terminal a considerar más
  allá de que la salida siga siendo texto legible sin color).

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas y el agente no debe cuestionarlas ni cambiarlas:

- **Alcance de adopción = cobra + lipgloss únicamente.** `huh`/`bubbletea` quedan fuera: el
  CLI de Vector es no interactivo (verificado, sin prompts en `cli/`).
- **Migración all-at-once, un solo cambio.** Sin fase de coexistencia ni flag para alternar
  entre dispatch viejo/nuevo.
- **El contrato `--json` es byte-idéntico, sin excepción.** Ningún comando gana ni pierde
  campos JSON en esta fase, aunque cobra facilitaría agregarlos.
- **Sin umbral de peso.** Se mide el binario release-equivalente (`CGO_ENABLED=0 -ldflags "-s
  -w"`, igual que `.goreleaser.yml`), se reporta el delta, y el usuario decide con el número —
  no hay condición de "falla" automática por tamaño.
- **Sólo flags long-form**, sin alias cortos nuevos en esta fase — para no ampliar la
  superficie de flags del CLI más allá de lo que exige la migración 1:1.
- **`cobra completion` on-the-fly, sin embeber.** `architecture/distribution-packaging.md` no
  exige embeber completions; se generan bajo demanda.
- **`internal/state`, `internal/config`, `internal/board` no se tocan.** cobra/lipgloss son
  capa de ruteo/presentación; cero cambios de lógica de dominio.
- **`var version` y el ldflag `-X main.version=...` se preservan literalmente** — es la única
  forma en que GoReleaser inyecta la versión (`.goreleaser.yml:29`).
- **`--repo-root`/`--json` siguen siendo flags locales por subcomando**, no persistentes en la
  raíz — para no alterar el `--help` de comandos que hoy no las tienen (p. ej. `version`).
- **La ayuda sigue yendo a stderr** (paridad con `usage()` actual), no a stdout.
- **Paleta de color**: placeholder = tokens hex de flagify, sujeto a confirmación (Open
  questions #1) — no es una decisión final de marca, es un punto de partida documentado.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación,
pero no implementarla.

---

## 11. Edge cases

La implementación debe manejar explícitamente los siguientes casos de parseo/dispatch —
son el riesgo central de esta migración (cobra/pflag no son un reemplazo mecánico de
`flag.FlagSet`):

### Dispatch de la raíz

- `vector` (sin args) → hoy: `usage()` a stderr + `os.Exit(2)` (`main.go:32-35`). Debe
  preservarse exit 2. cobra por defecto, sin subcomando y sin `Run`/`RunE` en la raíz, imprime
  ayuda con exit 0 — **debe sobreescribirse** explícitamente en `root.go`.
- `vector foo` (comando desconocido) → hoy: `"unknown command %q\n\n"` a stderr + `usage()` +
  exit 2 (`main.go:59-62`). cobra por defecto usa un mensaje distinto (`Error: unknown command
  "foo" for "vector"`) y exit 1 — **el exit code debe forzarse a 2** (invariante duro); el
  texto exacto del mensaje puede adaptarse al formato de cobra siempre que siga yendo a stderr
  y mencione el comando inválido (invariante blando).
- `vector spec` (sin subverbo) → hoy: `runSpec` retorna
  `fmt.Errorf("usage: vector spec <create|list|...>")`, `main()` lo imprime como
  `"error: ..."` y sale con exit **1** (`main.go:598-600`, `65-68`). cobra, para un comando
  padre sin `RunE` propio, por defecto muestra ayuda con exit 0 al invocarlo sin
  subcomando — **debe sobreescribirse**: el `specCmd` padre conserva un `RunE` explícito que
  retorna el mismo error, preservando exit 1 (no exit 0 con ayuda).
- `vector spec bogus` (subverbo desconocido) → hoy: error `"unknown spec subcommand %q"`
  (`main.go:633`), exit 1. cobra/pflag con `AddCommand` ya rechaza subcomandos no registrados
  con un mensaje propio y exit 1 por defecto — verificar que el exit code coincide (1); el
  texto puede diferir.
- `vector -v` / `vector --version` / `vector version` (en cualquier posición) → deben imprimir
  idénticamente `vector <version>` a stdout, exit 0.
- `vector help` / `vector -h` / `vector --help` → ayuda a stderr, exit 0.

### Flags

- Flag desconocida en cualquier subcomando de hoja (p. ej. `vector spec create --bogus`) → hoy:
  error de `flag.FlagSet` ("flag provided but not defined: -bogus") propagado, exit 1. Con
  pflag, comportamiento por defecto equivalente (error de parseo, exit 1) — **verificar
  explícitamente**, no asumir.
- **Default de `--json` no uniforme**: `context` tiene `--json` default `true`
  (`context.go:127`, con `--for` en `context.go:129`); todos los demás lo tienen default `false`. La migración 1:1 debe
  preservar el default *por comando*, no aplicar un default global.
- **Interleaving de flags y posicionales**: `flag.FlagSet` de la stdlib deja de parsear en el
  primer argumento no-flag, por eso hoy existen workarounds manuales (`leadingID`,
  `spec_transitions.go:17-22`, y el peel de un segundo posicional en `link`/`status`). `pflag`
  (motor de cobra) **sí** intercala flags y posicionales de forma nativa. Efecto: invocaciones
  que hoy fallan o requieren un orden estricto (p. ej. `vector spec apply --json my-spec`, flag
  antes del id) **empiezan a funcionar** tras la migración. Esto es una **ampliación superset**
  del lenguaje aceptado, no una regresión — toda invocación válida hoy sigue siendo válida y
  produce la misma salida; no se retira ningún caso. `leadingID` y el peel de segundo posicional
  se reimplementan contra el `args []string` que cobra entrega al `RunE` (los posicionales
  sobrantes tras el parseo de flags), con la misma semántica de "primer/segundo token no-flag
  = id/ref/target".

### `spec summarize commit` — doble orden

- Hoy soporta explícitamente **dos** órdenes: `vector spec summarize commit <id> ...` y
  `vector spec summarize <id> commit ...` (`summarize.go:54-62`, comentario: "the kit commands
  use the latter"). Un cobra `AddCommand("commit")` puro sólo entiende la primera forma
  (subcomando inmediatamente después del padre). **Debe preservarse ambas** — el `RunE` del
  comando `summarize` mantiene la detección manual de las dos órdenes sobre sus `args`
  (equivalente a hoy), en vez de delegar a un cobra child command real para el caso `<id>
  commit`. Documentar esto explícitamente en el código como excepción deliberada al patrón
  "cobra child command" usado en el resto del árbol.

### Peso del binario

- Medir **sólo** el build release-equivalente (`CGO_ENABLED=0 -ldflags "-s -w"`). Medir el
  binario de desarrollo sin strip (el ~9.9M actual sin flags) es un error de medición explícito
  señalado por el usuario — **no** es el número que se reporta como criterio de aceptación.

### Ayuda / stderr vs stdout

- Toda ayuda (`--help`, comando sin subverbo obligatorio que cae a ayuda, etc.) sigue yendo a
  **stderr**, igual que `usage()` hoy. Un script que redirige `2>/dev/null` y espera stdout
  limpio no debe ver cambios.

---

## 12. Estados de UI requeridos

No aplica en el sentido de pantallas/componentes — es un CLI. Estados observables equivalentes:

| Estado | Qué se muestra | Exit code |
|---|---|---|
| Invocación válida, sin `--json` | Salida humana, estilizada con `internal/ui` si hay TTY | 0 |
| Invocación válida, `--json` | JSON plano, byte-idéntico a hoy, sin color | 0 |
| Comando/flag desconocidos | Mensaje de error a stderr | 2 (comando/args raíz) o 1 (flag/subverbo) — ver §11 |
| `--help` / `-h` / sin subverbo obligatorio en un padre | Ayuda estilizada a stderr | 0, salvo `spec` sin subverbo (exit 1, ver §11) |
| Error de negocio (p. ej. spec no existe) | `error: <mensaje>` (estilizado con `ui.Error`) a stderr | 1 |
| Sin TTY / `NO_COLOR` / pipe | Igual que "válida" pero sin códigos ANSI (degradación automática de lipgloss) | igual que el caso base |

El board web (`web/`) no se ve afectado por este cambio; no aplica ningún estado de UI de
navegador.

---

## 13. Validaciones

### Validaciones de cliente (CLI)

Las reglas de negocio existentes (enums de `--status`, `--priority`, `--classification`,
`--artifacts`, formato de `--ticket`/`--related` como JSON, kebab-case de `--change`) **no
cambian** — viven en el cuerpo de cada `runXxx` (ahora `RunE`), no en la capa de parseo de
flags, y esta migración no las toca.

| Campo | Regla | Mensaje |
|---|---|---|
| flag no registrada en un subcomando de hoja | rechazada por pflag al parsear | mensaje de pflag (adaptado de flag stdlib), exit 1 |
| `spec create --classification` (en `spec fix`) | `spec-only\|code-only\|spec+code` | sin cambio (`spec_transitions.go:355-361`) |
| `spec fix --validation-result` | `pass\|fail\|""` | sin cambio (`spec_transitions.go:363-367`) |
| `spec create --artifacts` / `spec fix --artifacts` | `proposal,design,tasks` (tolerante a `.md`/casing) | sin cambio (`canonicalArtifact`, `main.go:731-746`) |

No hay validación de servidor — no hay backend remoto involucrado; toda la validación es local
al binario.

---

## 14. Seguridad y permisos

- cobra y lipgloss son dependencias OSS con licencias compatibles con la licencia del repo
  (Apache-2.0) y con distribución comercial (`architecture/distribution-packaging.md`): cobra
  es Apache-2.0, lipgloss es MIT.
- Ninguna de las dos librerías hace llamadas de red ni telemetría — la generación de
  completions y el renderizado de estilos son 100% locales.
- No se introduce ninguna superficie nueva de secretos: `internal/ui` no maneja tokens,
  credenciales ni PII; sólo formatea texto ya presente en la salida actual.
- No se cambia el modelo de permisos de escritura sobre `.vector/` (CLI-owns-writes, sin
  cambios).

---

## 15. Observabilidad y logging

- El mecanismo de reporte de error sigue siendo el mismo: `main()` imprime `error: <err>` a
  stderr (ahora posiblemente vía `ui.Error`) y sale con el exit code correspondiente. No se
  introduce ningún logger nuevo, ningún archivo de log, ni telemetría.
- Los `warning:` existentes a stderr (`serve.go`, `context.go`, `main.go`) se preservan
  textualmente, sólo ganan estilo (`ui.Warning`) cuando hay TTY.
- No se registra nada sensible — sin cambios respecto a hoy.

---

## 16. i18n / textos visibles

No aplica — no hay sistema de traducciones de agente ni de prosa involucrado en este cambio (a
diferencia del spec `add-agent-prose-language`, que sí cablea un `language` de config). Toda la
ayuda, los mensajes de error y los textos del CLI permanecen **en inglés**, igual que hoy
(`usage()` actual, todos los `fmt.Errorf`/`fmt.Printf` de `cli/`) — es una convención ya
establecida del repo, no algo que este cambio decida.

---

## 17. Performance

- El costo de construir el árbol `cobra.Command` en cada arranque del binario es despreciable
  (registro de structs en memoria, sin I/O); no hay llamadas de red ni de disco adicionales por
  la sola presencia de cobra.
- Los helpers de `internal/ui` son formateo de strings puro (lipgloss no hace I/O salvo
  detectar el perfil de color de la terminal una vez por proceso).
- El costo de **peso del binario** (tamaño en disco, tiempo de descarga en el instalador de un
  paso) es el impacto de performance real de este cambio y está cubierto explícitamente como
  criterio de aceptación en §8 (medido, no gateado por umbral).
- Sin llamadas repetidas ni trabajo pesado nuevo en el hilo principal.

---

## 18. Restricciones

El agente no debe:

- Tocar `cli/internal/state`, `cli/internal/config`, `cli/internal/board`, `cli/internal/
  openspec`, `cli/internal/scaffold`, `cli/internal/standup`, `cli/internal/webui`, `cli/
  internal/intel` — cobra/lipgloss son una capa de ruteo/presentación sobre `cmd/vector/`.
- Cambiar el shape, el orden de campos, el tipo o el indentado de ninguna salida `--json`
  existente.
- Introducir `huh`, `bubbletea`, ni ningún prompt interactivo (`bufio.NewReader`,
  `ReadPassword`, spinners bloqueantes).
- Agregar alias de flags cortos nuevos en esta fase.
- Cambiar ningún exit code documentado en §11 para una invocación que hoy es válida.
- Dejar el repo en un estado híbrido (parte del dispatch en `flag.FlagSet`, parte en cobra)
  mergeado a `main` — es un cambio atómico.
- Modificar `.goreleaser.yml` (los ldflags `-s -w -X main.version=...` no cambian).
- Añadir dependencias de red, telemetría, o cualquier paquete fuera de cobra/lipgloss (y sus
  transitivas resueltas por `go mod tidy`) sin que este spec lo autorice.
- Dejar `go.sum` desactualizado respecto a `go.mod` (correr y committear `go mod tidy`).
- Refactorizar código no relacionado con esta migración (p. ej. no tocar `internal/intel`,
  `internal/openspec`, ni reorganizar paquetes fuera de lo listado en §6).
- Medir el peso del binario en modo dev sin strip como si fuera el número de aceptación.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `cli/go.mod`/`cli/go.sum` con `cobra` + `lipgloss` como primeras dependencias externas
      del módulo, `go mod tidy` limpio.
- [ ] `cli/cmd/vector/root.go` + `completion.go` nuevos; `main.go` reducido a construir
      `rootCmd` + mapear exit codes; `func usage()` eliminada.
- [ ] Los ~10 archivos `cmd/vector/*.go` con lógica de comando migrados a `newXxxCmd()` +
      `RunE`, cuerpo de negocio sin cambios.
- [ ] `cli/internal/ui/ui.go` + `help.go` + tests.
- [ ] `cli/cmd/vector/testutil_test.go` (harness compartido) y los ~10 archivos `*_test.go`
      actualizados a la nueva convención de invocación.
- [ ] `cli/cmd/vector/golden_test.go` + `testdata/golden/*.json` (uno por comando `--json` de
      §7), capturados **antes** del cambio de dispatch y verdes **después**.
- [ ] Medición de peso reportada (bytes antes/después del binario release-equivalente,
      `CGO_ENABLED=0 -ldflags "-s -w"`, sin veredicto automático).
- [ ] `README.md`, `docs/plugin-and-commands.md`,
      `.claude/rules/architecture/distribution-packaging.md`, `cli/CLAUDE.md` actualizados.
- [ ] Gate verde: `gofmt -l`, `go vet`, `go test`, `go build`, `go mod tidy` sin diff.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] No toqué `internal/state`, `internal/config`, `internal/board` ni ningún otro paquete de
      dominio.
- [ ] Capturé los golden `--json` **antes** de tocar el dispatch (no reconstruidos post-hoc a
      partir de la nueva salida — eso invalidaría el gate).
- [ ] Los ~10 archivos de tests que llamaban `runXxx(args)` directamente ahora usan el harness
      `execXxxCmd`/`testutil_test.go`, y siguen cubriendo lo mismo que cubrían antes.
- [ ] `vector version`/`--version`/`-v` y el ldflag `-X main.version=...` verificados con un
      build de prueba.
- [ ] Verifiqué manualmente (o con un script de revisión) que ningún `ui.*` se llama dentro de
      una rama `--json`.
- [ ] Los casos de exit code de §11 (`vector` sin args → 2; comando desconocido → 2; `spec` sin
      subverbo → 1; `spec` subverbo desconocido → 1) están cubiertos por tests y pasan.
- [ ] `summarize commit` funciona en ambas órdenes (`summarize commit <id>` y `summarize <id>
      commit`).
- [ ] `context --json` sigue siendo el único comando con default `true` para `--json`.
- [ ] Medí el binario release-equivalente (`CGO_ENABLED=0 -ldflags "-s -w"`), no el binario dev
      sin strip, y reporté el delta explícitamente.
- [ ] Actualicé README.md, docs/plugin-and-commands.md, distribution-packaging.md, cli/CLAUDE.md.
- [ ] No agregué `huh`, `bubbletea`, ni alias de flags cortos nuevos.
- [ ] Ejecuté `gofmt -l cli`, `go vet`, `go test ./...`, `go build ./...`, `go mod tidy` — todos
      verdes / sin diff.
- [ ] No dejé el repo en un estado híbrido flag/cobra.
- [ ] No dejé logs temporales ni TODOs sin justificar.

---

## Open questions

1. **Paleta de colores lipgloss**: ¿reusar los hex tokens de flagify (`#00D4FF` cyan,
   `#00CC88` green, `#FF6B6B` red, `#FFCC00` yellow, `#666666` dim,
   `~/Developer/Personal/flagify/cli/internal/ui/ui.go:11-15`) o definir una paleta propia de
   Vector? No hay decisión de marca registrada en el repo. Placeholder documentado en §6
   (`cli/internal/ui/ui.go`) hasta resolverse.
2. **Alias de flags cortos**: la decisión por defecto de esta fase es mantener sólo flags
   long-form (§10) para no ampliar la superficie. Reabrir en una fase futura si se quiere UX
   más terse (p. ej. `-r` para `--repo-root`, que cobra soporta nativamente).
3. **`vector context` / `vector detect-ticket` (machine-only)**: ¿reciben también salida
   estilizada con `internal/ui` cuando se invocan sin `--json` (uso interactivo/manual), o
   permanecen en texto plano dado que su consumidor primario es tooling (`context` incluso
   tiene `--json` default `true`)? No se decide en esta fase — ambos comandos quedan fuera del
   alcance de re-estilizado (§2, Fuera de scope) hasta resolver esta pregunta.
