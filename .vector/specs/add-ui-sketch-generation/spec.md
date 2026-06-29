# Spec: Generar sketches de diseño Excalidraw para specs de UI

## 1. Objetivo

Construir la generación automática de wireframes Excalidraw al final del pipeline de
`/vector:raw` y `/vector:research`: cuando el spec compuesto señala trabajo de UI, el
comando sugiere al dev confirmar la generación; si confirma, un subagente
`vector-ui-ux-designer` (Sonnet) emite un archivo `.excalidraw` JSON válido que el binario
valida y persiste en `.vector/specs/<id>/sketches/`; el board muestra un botón de descarga
en el `SpecDetailsDrawer`; y el watcher SSE actualiza el panel en vivo sin reinicio.

Esta feature permite que un **dev** que acaba de autoral un spec de UI obtenga un
**wireframe descargable en Excalidraw** adjunto al spec, sin salir del flujo de Claude Code,
con opt-out global para quien nunca lo quiera y degradación suave si el agente falla.

## 2. Alcance

### Incluido en esta fase

- **Heurística híbrida de detección de UI**: evaluación del título/cuerpo del spec + §12
  ("Estados de UI") + keywords de capas afectadas (`board`, `drawer`, `web/`, `component`,
  `UI`, `pantalla`, `formulario`, `modal`), ejecutada al TAIL de `/vector:raw` y
  `/vector:research` — después de que el spec body está compuesto, porque la heurística
  necesita el body para operar.
- **Confirmación opt-in por el dev**: si la heurística señala UI, el comando pregunta vía
  `AskUserQuestion` si desea generar el wireframe. Si confirma, spawna el agente async; si
  declina, salta sin error.
- **Opt-out global**: flag `--no-sketch` en `/vector:raw` y `/vector:research` (suprime la
  pregunta para esa ejecución); campo `SketchEnabled *bool` (`json:"sketchEnabled,omitempty"`)
  en `.vector/config.json` (nil/ausente = habilitado; `false` = deshabilitado globalmente).
  Ninguna de las dos rutas hace prompt al dev cuando el opt-out está activo.
- **Nuevo agente embebido `vector-ui-ux-designer`** (model: sonnet): vendoriza el
  conocimiento del formato `.excalidraw` JSON (`{type, version, elements, appState, files}`)
  en su body y emite un documento válido desde los requisitos del spec. Embebido en el
  binario vía `kit/agents/vector-ui-ux-designer.md` + `go generate ./internal/scaffold`
  (guardado por `TestAssetsMatchKit` en `cli/internal/scaffold/scaffold_test.go`).
- **Validación del output antes de persistir**: el binario verifica que el JSON sea bien
  formado y tenga las keys top-level `type`, `version`, `elements` antes de copiar el
  archivo. Salida malformada → rechazo silencioso (soft failure): el spec sigue en draft, el
  sketch simplemente no aterriza.
- **Almacenamiento**: `.vector/specs/<id>/sketches/<name>.excalidraw`. Campo aditivo
  `Sketches []SketchRef` en `SpecState` (omitempty; `SchemaVersion` se mantiene en 1,
  siguiendo el patrón de `QuickWin bool` en `cli/internal/state/types.go` línea 145).
- **Clave de artifact `"sketch"`** añadida a `artifactRelPath`, `validArtifact` y al handler
  `/api/file`; Content-Type `application/octet-stream` + header `Content-Disposition:
  attachment; filename="<name>.excalidraw"`. El prefijo `.vector/specs/<id>/sketches/` queda
  cubierto por el allowed-prefix ya existente en `verifyArtifactPath`.
- **Board/web**: `board.Card` gana `Sketches []SketchRef` (omitempty); `web/src/types/board.ts`
  espeja la nueva forma; `ArtifactKey` amplía con `'sketch'`; `entries.ts` genera una entrada
  descargable por sketch; `SpecArtifactBrowser` detecta entradas tipo sketch y sirve descarga
  directa (no abre el `FilePreviewModal`).
- **Subcomando binario** `vector spec attach-sketch <id> --file <path>`: el agente lo llama
  para registrar el sketch generado. El binario valida la forma JSON, copia el archivo,
  actualiza `state.json` (via `Store.AttachSketch`). El binario es el único escritor.
- **Ejecución async-in-session**: el comando spawna el agente y retorna inmediatamente (el
  draft ya está registrado). Cuando el agente termina llama al subcomando; el watcher de
  `vector serve` detecta el cambio y llama `Broadcast()` → SSE → el board live-updates.
- **Token routing**: `vector-ui-ux-designer` corre en Sonnet (razonamiento de
  layout/jerarquía). El coste se registra vía `vector spec route` (mecanismo existente en
  `cli/cmd/vector/route.go`), siguiendo `product/token-routing.md`.

### Fuera de scope

- Pre-renderizado in-binary a SVG o PNG (no hay runtime JS en el binario Go stdlib-only;
  `cli/go.mod` confirma cero dependencias externas).
- Render live inline del sketch en el board (`@excalidraw/excalidraw` pesa ~2–3 MB; infringe
  la regla de bundle ligero declarada en `web/CLAUDE.md`).
- Daemon/worker background verdadero en el binario (obligaría al binario a realizar llamadas
  LLM propias; hoy el binario hace cero llamadas LLM — `cli/cmd/vector/route.go` y
  `cli/cmd/vector/summarize.go` solo calculan el costo del meter sin invocar ninguna API).
- Sketches para specs sin señal de UI (la heurística filtra; no se pregunta en todo spec).
- Traducción de labels o contenido interno del `.excalidraw`.
- Integración con Figma u otras herramientas de diseño externas.
- Historial de versiones de sketches / tracking de múltiples revisiones del mismo sketch.
- Theming dark-mode del `.excalidraw` (tokens Dracula de `web/src/styles/tokens.css`).
- Thumbnails inline en la card del board o en el encabezado del drawer.
- Previsualización inline del `.excalidraw` en el `FilePreviewModal`.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Lenguaje binario: **Go** (módulo único en `cli/`, stdlib only, cero dependencias externas —
  `cli/go.mod`: `module github.com/mariocampbell/vector`, `go 1.26`, sin go.sum).
- Config: struct `config.Config` serializado a/desde `.vector/config.json`
  (`cli/internal/config/config.go`); escritura atómica vía `writeFileAtomic`.
- CLI: parseo con `flag.FlagSet` de la stdlib; un `FlagSet` por subcomando (patrón de
  `runInit`, `runUpdate` en `cli/cmd/vector/main.go`).
- State: `state.SpecState` / `state.Store` (`cli/internal/state/`); escritura serializada
  y atómica (CLI-owns-writes); `SchemaVersion = 1` (`cli/internal/state/types.go` línea 9).
- Artifact serving: `state.ReadSpecArtifact` / `artifactRelPath` / `verifyArtifactPath`
  (`cli/internal/state/artifact.go`); handler `/api/file` en `cli/internal/board/server.go`.
- Board projection: `board.Card` / `board.Build` (`cli/internal/board/board.go`);
  `board.SchemaVersion = 2` (línea 16).
- Embed/scaffold: `cli/internal/scaffold` embebe `kit/{commands,agents}` vía `embed.FS`;
  copia vendorizada en `cli/internal/scaffold/assets/` regenerada con `go generate`;
  drift detectado por `TestAssetsMatchKit` (`cli/internal/scaffold/scaffold_test.go`).
- Frontend: **React 19 + Vite + TypeScript**, CSS Modules, iconos `lucide-react`. Sin
  librería de componentes (regla de bundle ligero). Versiones declaradas en
  `web/package.json`. Output buildado embebido en el binario Go.
- Excalidraw JSON: formato `{type:"excalidraw", version:2, elements:[...], appState:{...},
  files:{}}`. No se instala `@excalidraw/excalidraw` — el agente lleva el conocimiento
  vendorizado en su body.
- Kit distribuible: agente markdown nuevo (`kit/agents/vector-ui-ux-designer.md`,
  `model: sonnet`, `tools: Read, Write, Bash`) + modificaciones a `raw.md` y `research.md`.

### Versiones relevantes

- Go: **1.26** (declarado en `cli/go.mod`). El cambio usa stdlib exclusivamente.
- `state.SchemaVersion`: **1** (se mantiene; el campo `Sketches` es aditivo con omitempty).
- `board.SchemaVersion`: **2** (declarado en `cli/internal/board/board.go` línea 16). El
  campo `Sketches []SketchRef` en `Card` es aditivo/omitempty — si el web contract exige un
  bump de schema board por este cambio, es TBD — ver Open questions.
- React + Vite + TypeScript: versiones declaradas en `web/package.json` — no leídas
  directamente; el cambio usa solo patrones ya presentes (hooks, TypeScript interfaces).
- Excalidraw JSON format: no hay versión de librería (conocimiento vendorizado en el agente).

### Patrones existentes a respetar

- **CLI-owns-writes**: el binario es el único escritor de `.vector/` y los artifacts. El
  agente `vector-ui-ux-designer` llama al subcomando `vector spec attach-sketch` — nunca
  escribe en `.vector/` directamente.
- **Campo aditivo con `omitempty`**: igual que `QuickWin bool` (`types.go` línea 145) —
  un `SpecState` previo sin `Sketches` deserializa con `Sketches == nil`, sin error ni migración.
- **Escritura atómica** vía el `Store` existente (temp + rename).
- **Naming kebab-case** para flags y subcomandos: `--no-sketch`, `attach-sketch`.
- **Embed del agente** siguiendo el flujo canónico: editar en `kit/agents/`, correr
  `go generate ./internal/scaffold` desde `cli/`, no editar `assets/` a mano.
- **Token routing**: el agente designer corre en Sonnet (razonamiento real, no mecánico);
  la heurística y la orquestación permanecen en el main loop (barato). Documentado en
  `product/token-routing.md`.
- **Soft failure**: el rechazo de un sketch malformado no rompe el flujo de spec authoring.
  El spec queda en draft sin sketch; no se persiste un estado de error.
- **Subcomando de spec** nuevo registrado en el switch interno de `runSpec` en `main.go`.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `state.SpecState` con patrón de campo aditivo omitempty (`QuickWin bool` en
      `cli/internal/state/types.go` línea 145) — el patrón a replicar para `Sketches []SketchRef`.
- [x] `state.ReadSpecArtifact`, `artifactRelPath`, `verifyArtifactPath` en
      `cli/internal/state/artifact.go` — el mecanismo de artifact serving a extender.
- [x] Handler `/api/file` en `cli/internal/board/server.go` (línea 172), `validArtifact`
      (línea 198) — a extender para la clave `"sketch"`.
- [x] `ArtifactKey` en `web/src/api/useFileContent.ts` (union `'spec'|'proposal'|
      'design'|'tasks'`) y `Artifacts` interface en `web/src/types/board.ts` (línea 22) — a
      extender con la nueva clave y tipo.
- [x] `SpecArtifactBrowser`, `entriesFor`, `ArtifactEntry` en
      `web/src/components/SpecDetailsDrawer/` — a extender con entradas de tipo sketch.
- [x] `vector spec route` / `runSpecRoute` (`cli/cmd/vector/route.go`) — existente; el
      routing del agente designer lo usa sin cambios.
- [x] `/vector:raw` (`kit/commands/vector/raw.md`) y `/vector:research`
      (`kit/commands/vector/research.md`) — a extender en su tail.
- [x] Scaffold embed + `go generate` + `TestAssetsMatchKit`
      (`cli/internal/scaffold/scaffold_test.go`) — mecanismo sin cambios; solo se añade el
      nuevo agente a la lista de assets.
- [x] `vector serve` + watchState + `Broadcast` en `cli/cmd/vector/serve.go` — sin
      cambios; el watcher existente detecta cualquier cambio en `.vector/` y llama
      `Broadcast()`.
- [ ] `SketchRef` struct + `Sketches []SketchRef` en `cli/internal/state/types.go` — a crear.
- [ ] `Store.AttachSketch` en `cli/internal/state/` — método nuevo a crear.
- [ ] Subcomando `vector spec attach-sketch` (`cli/cmd/vector/sketch.go`) — a crear.
- [ ] Campo `SketchEnabled *bool` en `cli/internal/config/config.go` — a crear.

Si alguna dependencia no existe, el agente debe detenerse y reportar qué falta. No inventar
contratos ni rutas.

---

## 5. Arquitectura

### Patrón a usar

**Async-in-session + SSE push + CLI-owns-writes**: la heurística y orquestación viven en el
main loop del command (barato). La generación del sketch se delega a un subagente Sonnet que
corre en background dentro de la sesión Claude. Cuando termina, llama al subcomando binario;
el watcher de `vector serve` propaga el cambio vía el SSE ya existente.

### Capas afectadas

- **kit/commands** (`raw.md`, `research.md`): sí — paso TAIL post-registro con detección +
  confirmación + spawn async del agente + registro de routing.
- **kit/agents** (nuevo `vector-ui-ux-designer.md`): sí — agente que genera el `.excalidraw`.
- **cli/internal/state** (`types.go`, `artifact.go`, `store.go`): sí — nuevo `SketchRef`,
  campo `Sketches`, `Store.AttachSketch`, caso `"sketch"` en `artifactRelPath`.
- **cli/internal/board** (`board.go`, `server.go`): sí — `Sketches` en `Card`; `"sketch"` en
  `validArtifact`; Content-Type condicional en `handleFile`.
- **cli/cmd/vector** (nuevo `sketch.go`, modificar `main.go`): sí — subcomando
  `runSpecAttachSketch` y su registro en el dispatch de `runSpec`.
- **cli/internal/config** (`config.go`): sí — campo `SketchEnabled *bool` + helper
  `IsSketchEnabled()`.
- **web/src/types/board.ts**: sí — `SketchRef` interface + `sketches?` en `Card`.
- **web/src/api/useFileContent.ts**: sí — `ArtifactKey` amplía con `'sketch'`.
- **web/src/components/SpecDetailsDrawer/** (`entries.ts`, `entries.test.ts`,
  `SpecArtifactBrowser.tsx`): sí — entradas sketch; el browser sirve descarga directa.
- **presentation (card del board)**: no — la card no muestra badge de sketch; solo el drawer.
- **data/estado** (`.vector/specs/<id>/sketches/`): sí — artifact escrito por el binario.

### Flujo esperado

1. Dev ejecuta `/vector:raw` (o `/vector:research`). El spec draft queda registrado en el
   board vía `vector spec create`.
2. Al TAIL del command (después del registro), la heurística híbrida evalúa el spec
   compuesto con esta **regla de decisión** (V1, conservadora): hay **señal fuerte** (→ se
   pregunta) si y solo si se cumple cualquiera de: (a) la §12 "Estados de UI requeridos" del
   spec compuesto es no-vacía (no es solo "No aplica"), **o** (b) aparecen **2 o más** keywords
   de la lista (`board`, `drawer`, `modal`, `web/`, `component`, `UI`, `pantalla`, `formulario`,
   `card`, `componente`) en título+body. Un solo keyword suelto = **señal débil** → no se
   pregunta (se prefiere falso-negativo a falso-positivo). Sin señal → salta sin preguntar.
3. Si hay señal UI y el opt-out no está activo (`--no-sketch` ausente y `config.SketchEnabled`
   no es `false`) → el command pregunta vía `AskUserQuestion`.
4. Dev declina → el command termina limpiamente; el spec sigue como draft sin sketch.
5. Dev confirma → el command registra el routing estimado (paso 10) y spawna
   `vector-ui-ux-designer` como subagente async, luego retorna inmediatamente (el draft ya está
   en el board; el dev puede seguir trabajando).
6. El agente `vector-ui-ux-designer` (Sonnet) lee el spec compuesto y emite el `.excalidraw`
   JSON con `Write` a un path temporal (p.ej. `.vector/tmp/<id>/sketch.excalidraw`).
7. El agente llama `vector spec attach-sketch <id> --file <path-temporal>` con `Bash`. El
   binario: valida la forma JSON (`{type, version, elements}`), copia el archivo a
   `.vector/specs/<id>/sketches/<name>.excalidraw`, actualiza `state.json` vía
   `Store.AttachSketch` (escribe `SketchRef` en `Sketches`).
8. El watcher de `vector serve` (polling en `.vector/` cada `poll-ms`, default 1 000 ms)
   detecta el cambio en `state.json`, llama `server.Broadcast()` → SSE → el board live-updates.
9. El `SpecDetailsDrawer` muestra el botón/enlace de descarga del sketch recién añadido.
10. El routing se registra en el paso 5 (al hacer spawn, antes de retornar), con tokens
    **estimados** porque el command no espera al agente: `vector spec route <id> --model sonnet
    --baseline opus --task "generate ui sketch" --tokens-in <est> --tokens-out <est>
    --precision estimated`.

### Ubicación de archivos nuevos

```txt
kit/agents/
  vector-ui-ux-designer.md               ← fuente editable del agente (nunca editar el assets/)

cli/internal/scaffold/assets/agents/
  vector-ui-ux-designer.md               ← copia generada por go generate (no editar a mano)

cli/cmd/vector/
  sketch.go                              ← runSpecAttachSketch (nuevo subcomando)

web/src/api/
  useFileContent.ts                      ← ArtifactKey + descarga de sketch (modificar)
```

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/agents/vector-ui-ux-designer.md` | NUEVO | Agente Sonnet con conocimiento Excalidraw vendorizado que emite `.excalidraw` JSON válido desde el spec | `kit/agents/vector-standup-writer.md`, `kit/agents/vector-spec-composer.md` |
| `cli/internal/scaffold/assets/agents/vector-ui-ux-designer.md` | REGENERAR | Copia embebida del agente; se regenera con `go generate ./internal/scaffold` | `cli/internal/scaffold/assets/agents/vector-standup-writer.md` |
| `cli/internal/state/types.go` | MODIFICAR | Añadir `SketchRef` struct + `Sketches []SketchRef` omitempty en `SpecState` | `QuickWin bool` (línea 145) en el mismo archivo |
| `cli/internal/state/artifact.go` | MODIFICAR | Caso `"sketch"` en `artifactRelPath`; allowed-prefix en `verifyArtifactPath` ya cubierto por `.vector/specs/<id>/` | Casos `"spec"`, `"proposal"` (líneas 55–68) en el mismo archivo |
| `cli/internal/state/store.go` | MODIFICAR | Añadir `Store.AttachSketch(id string, ref SketchRef) error` para escribir el sketch y actualizar `state.json` atómicamente | Métodos `RouteAgent`, `CreateSpec` en el mismo archivo |
| `cli/internal/board/board.go` | MODIFICAR | `Sketches []state.SketchRef` omitempty en `Card`; propagación en `toCard` | `QuickWin bool` (línea 57) y `toCard` (línea 214) en el mismo archivo |
| `cli/internal/board/server.go` | MODIFICAR | `"sketch"` en `validArtifact` (línea 198); Content-Type `application/octet-stream` + `Content-Disposition: attachment` en `handleFile` para artifact sketch | `validArtifact` y `handleFile` (líneas 172, 198) en el mismo archivo |
| `cli/cmd/vector/sketch.go` | NUEVO | Subcomando `runSpecAttachSketch`: valida JSON, persiste `.excalidraw`, actualiza state vía `Store.AttachSketch` | `cli/cmd/vector/route.go` (`runSpecRoute`) |
| `cli/cmd/vector/main.go` | MODIFICAR | Registrar `"attach-sketch"` en el switch interno de `runSpec` | Dispatch de otros subcomandos de `spec` en el mismo archivo |
| `cli/internal/config/config.go` | MODIFICAR | Campo `SketchEnabled *bool \`json:"sketchEnabled,omitempty"\`` + helper `IsSketchEnabled() bool` | Campo `Language string` y `ResolvedLanguage()` (líneas 79, 163) en el mismo archivo |
| `web/src/types/board.ts` | MODIFICAR | `SketchRef` interface + campo `sketches?: SketchRef[]` en `Card` | `Artifacts` interface (línea 22) en el mismo archivo |
| `web/src/api/useFileContent.ts` | MODIFICAR | Ampliar `ArtifactKey` con `\| 'sketch'`; manejo de descarga para artifact sketch | El mismo archivo |
| `web/src/components/SpecDetailsDrawer/entries.ts` | MODIFICAR | Flag `download?: boolean` en `ArtifactEntry`; entradas por cada `card.sketches` en `entriesFor` | `entriesFor` y `ArtifactEntry` en el mismo archivo |
| `web/src/components/SpecDetailsDrawer/entries.test.ts` | MODIFICAR | Tests para entradas de tipo sketch (con/sin sketches, múltiples) | Tests `makeCard` existentes en el mismo archivo |
| `web/src/components/SpecDetailsDrawer/SpecArtifactBrowser.tsx` | MODIFICAR | Detectar `entry.download === true` y servir descarga directa; no abrir `FilePreviewModal` para sketches | El mismo archivo |
| `kit/commands/vector/raw.md` | MODIFICAR | Añadir paso 12 TAIL post-registro: heurística UI + `AskUserQuestion` + spawn async + routing | Pasos 1–11 existentes del mismo archivo |
| `kit/commands/vector/research.md` | MODIFICAR | Ídem como paso 15 TAIL post-registro | Pasos 0–14 existentes del mismo archivo |

### Detalle por archivo

#### kit/agents/vector-ui-ux-designer.md

Acción: NUEVO

Debe implementar:

- Front-matter con `name: vector-ui-ux-designer`, `description: <breve>`, `model: sonnet`,
  `tools: Read, Write, Bash`. Necesita `Write` (escribir el `.excalidraw` a un path temporal)
  y `Bash` (invocar `vector spec attach-sketch`) porque en el modelo async-in-session el
  command ya retornó: el propio agente escribe el temp y llama al binario (ver §5, pasos 6–7).
- **Input**: el command pasa el path al spec compuesto (`SPEC_PATH`), el `SPEC_ID` y el path
  temporal de salida (`OUTPUT_PATH`, p.ej. `.vector/tmp/<id>/sketch.excalidraw`) en el prompt.
  El agente lee el spec con `Read`.
- **Doctrina compartida**: leer `.claude/agents/_shared/prose-rules.md` antes de proceder
  (misma instrucción que `vector-standup-writer`).
- **Conocimiento vendorizado del formato Excalidraw**: el agente describe en su body:
  - Estructura top-level: `{type:"excalidraw", version:2, elements:[...], appState:{...}, files:{}}`.
  - Tipos de `elements` más comunes: `rectangle`, `ellipse`, `text`, `arrow`, `diamond`,
    `line`. Propiedades obligatorias por elemento: `id` (UUIDv4 string), `type`, `x`, `y`,
    `width`, `height`, `angle`, `strokeColor`, `backgroundColor`, `fillStyle`,
    `strokeStyle`, `roughness`, `opacity`, `version`, `versionNonce`, `seed`, `groupIds`,
    `frameId`, `boundElements`, `link`, `locked`.
  - `appState` mínimo: `{theme:"light", viewBackgroundColor:"#ffffff"}`.
  - `files`: objeto vacío `{}` cuando no hay imágenes embebidas.
- **Output**: el agente escribe un único `.excalidraw` JSON válido (sin prosa, sin code fence,
  sin texto adicional) en el `OUTPUT_PATH` temporal con `Write`, y luego invoca
  `vector spec attach-sketch <SPEC_ID> --file <OUTPUT_PATH>` con `Bash`. El binario valida y
  persiste; el agente NO escribe en `.vector/specs/<id>/` ni toca `state.json` directamente.
- **Hard rules**: nunca asumir que existe una skill global `~/.claude/` ni un MCP de
  Excalidraw; nunca hacer llamadas de red; escribir SOLO el archivo temporal en `.vector/tmp/`
  (nunca tocar `.vector/specs/` ni `state.json` — eso es responsabilidad del binario).

Debe seguir como referencia:
- `kit/agents/vector-standup-writer.md` (front-matter + shared doctrine + hard rules + output shape exacto).
- `kit/agents/vector-spec-composer.md` (nivel de detalle del body).

No debe incluir:
- Lógica de token routing (la gestiona el command que lo invoca; ver §5 paso 10).
- Escritura directa de `state.json` o de archivos bajo `.vector/specs/` (solo escribe el
  `.excalidraw` temporal en `.vector/tmp/` y delega la persistencia al binario).

#### cli/internal/state/types.go

Acción: MODIFICAR

Cambios requeridos:
- Añadir antes de `SpecState` (o junto a los demás tipos auxiliares):
  ```go
  // SketchRef points to an Excalidraw wireframe sketch generated by the
  // vector-ui-ux-designer agent and stored under .vector/specs/<id>/sketches/.
  // Multiple sketches per spec are supported; each has a unique file name.
  type SketchRef struct {
      Name      string    `json:"name"`      // filename with .excalidraw extension
      CreatedAt time.Time `json:"createdAt"`
  }
  ```
- Añadir a `SpecState` (después de `QuickWin`, junto a los demás campos omitempty):
  ```go
  // Sketches lists Excalidraw wireframe files generated by vector-ui-ux-designer.
  // Additive and omitempty: legacy SpecState without this field loads as nil, no migration.
  Sketches []SketchRef `json:"sketches,omitempty"`
  ```
- `SchemaVersion` (línea 9) **no cambia** (sigue en 1).

Restricciones:
- No modificar ningún otro campo ni la lógica de serialización existente.
- Un `SpecState` previo sin `Sketches` debe deserializar con `Sketches == nil` (automático).

#### cli/internal/state/artifact.go

Acción: MODIFICAR

Cambios requeridos:
- En `artifactRelPath` (línea 53), añadir caso `"sketch"` en el switch:
  ```go
  case "sketch":
      if len(spec.Sketches) == 0 {
          return "", fmt.Errorf("spec %q has no sketch: %w", spec.ID, fs.ErrNotExist)
      }
      // V1: serve the first sketch; multi-sketch URL disambiguation is TBD — ver Open questions.
      return filepath.ToSlash(filepath.Join(".vector", "specs", spec.ID, "sketches", spec.Sketches[0].Name)), nil
  ```
- En `verifyArtifactPath` (línea 90): el prefijo `.vector/specs/<id>/` ya está como primer
  allowed prefix (línea 94): `filepath.Join(repoRoot, ".vector", "specs", spec.ID)`. El
  directorio `sketches/` está anidado bajo él, así que `isUnder` lo cubre sin añadir un
  nuevo prefijo. **Verificar que la lógica `isUnder` cubre el subdirectorio al implementar**
  y añadir el prefijo explícito si se detecta que no queda cubierto.

Restricciones:
- No cambiar el comportamiento de los casos existentes (`spec`, `proposal`, `design`, `tasks`).
- El error de sketch ausente debe ser `fs.ErrNotExist` (el handler lo mapea a 404, no 500).

#### cli/internal/state/store.go

Acción: MODIFICAR

Cambios requeridos:
- Añadir `func (s *Store) AttachSketch(id string, file []byte, ref SketchRef) error`:
  1. Construir el path absoluto del directorio `sketches/`:
     `filepath.Join(s.root, "specs", id, "sketches")`.
  2. `os.MkdirAll(sketchesDir, 0o755)`.
  3. Escribir los bytes del archivo a `filepath.Join(sketchesDir, ref.Name)` vía escritura
     atómica (temp + rename, igual que `writeFileAtomic`).
  4. Leer el `SpecState` existente con `s.ReadSpec(id)`.
  5. Añadir `ref` a `spec.Sketches`.
  6. Persistir el `SpecState` actualizado con escritura atómica del `state.json`.
  7. Retornar error descriptivo en cualquier paso fallido.

Debe seguir como referencia:
- `RouteAgent` y `CreateSpec` en el mismo archivo para el patrón de escritura atómica.

Restricciones:
- Solo escribe bajo `.vector/specs/<id>/sketches/` y actualiza `.vector/specs/<id>/state.json`.
- No llamar a ninguna API LLM.

#### cli/internal/board/board.go

Acción: MODIFICAR

Cambios requeridos:
- Añadir a `Card` (junto a los demás campos omitempty, después de `QuickWin`):
  ```go
  Sketches []state.SketchRef `json:"sketches,omitempty"`
  ```
- En `toCard` (línea 214), propagar el campo:
  ```go
  Sketches: spec.Sketches,
  ```

Restricciones:
- No cambiar los demás campos ni la lógica de proyección.
- Confirmar si `board.SchemaVersion` (actualmente 2, línea 16) requiere bump por este campo
  aditivo — TBD — ver Open questions.

#### cli/internal/board/server.go

Acción: MODIFICAR

Cambios requeridos:
- En `validArtifact` (línea 198), añadir `case "sketch": return true`.
- En `handleFile` (línea 172): hacer el Content-Type condicional. Para `artifact == "sketch"`:
  ```go
  w.Header().Set("Content-Type", "application/octet-stream")
  w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, sketchName))
  ```
  Para otros artifacts: mantener `text/markdown; charset=utf-8` (comportamiento actual).
  El `sketchName` se obtiene leyendo `spec.Sketches[0].Name` (disponible via `ReadSpec` o
  extrayendo del path devuelto por `ReadSpecArtifact`; el diseño exacto queda al implementador
  con la restricción de no duplicar la lectura del spec).

Restricciones:
- No cambiar el comportamiento de los artifacts existentes (`spec`, `proposal`, `design`, `tasks`).
- No exponer el path absoluto interno del archivo en los headers de respuesta.

#### cli/cmd/vector/sketch.go

Acción: NUEVO

Debe implementar:

- `func runSpecAttachSketch(args []string) error`.
- Flags: `--file <path>` (requerido), `--name <nombre-opcional>` (default: `filepath.Base(--file)`),
  `--repo-root`, `--json`.
- Pasos:
  1. Parsear flags y el ID posicional (`leadingID(args)`, patrón de `route.go`).
  2. Leer el archivo desde `--file` (`os.ReadFile`).
  3. Validar que es JSON bien formado (`json.Unmarshal` a `map[string]interface{}`).
  4. Verificar que el mapa contiene las keys `"type"`, `"version"`, `"elements"` (cada
     ausencia produce error: `"not a valid Excalidraw document: missing key \"<key>\""`).
  5. Sanitizar `--name` (rechazar si contiene `/`, `..`, o caracteres peligrosos en nombres
     de archivo de sistema; si pasa, usar como nombre final).
  6. Abrir el Store (`openStore(*repoRoot)`).
  7. Llamar `store.AttachSketch(id, fileBytes, state.SketchRef{Name: name, CreatedAt: time.Now()})`.
  8. Si `--json`: emitir `{"id": <id>, "sketch": <name>}`. Else: `fmt.Printf("attached sketch %q to spec %q\n", name, id)`.

Debe seguir como referencia:
- `cli/cmd/vector/route.go` (`runSpecRoute`) — estructura flags + `openStore` + output.

No debe incluir:
- Llamadas a APIs LLM o externas.
- Escritura fuera de `.vector/specs/<id>/sketches/` y `.vector/specs/<id>/state.json`.

#### cli/cmd/vector/main.go

Acción: MODIFICAR

Cambios requeridos:
- Dentro del switch de subcomandos de `spec` (en la función que dispatcha `vector spec <sub>`),
  añadir:
  ```go
  case "attach-sketch":
      err = runSpecAttachSketch(args[1:])
  ```

Restricciones:
- No cambiar el dispatch de otros subcomandos.
- `attach-sketch` vive bajo `vector spec attach-sketch`, no como comando top-level.

#### cli/internal/config/config.go

Acción: MODIFICAR

Cambios requeridos:
- Añadir al struct `Config` (junto a los demás campos opcionales):
  ```go
  // SketchEnabled controls whether /vector:raw and /vector:research suggest
  // generating Excalidraw sketches. nil (absent) = enabled (default); false =
  // globally suppressed. true is equivalent to nil. Additive and omitempty.
  SketchEnabled *bool `json:"sketchEnabled,omitempty"`
  ```
- Añadir helper:
  ```go
  // IsSketchEnabled reports whether sketch suggestions are active for this repo.
  // nil (unconfigured) and true are both enabled; only explicit false disables.
  func (c *Config) IsSketchEnabled() bool {
      return c.SketchEnabled == nil || *c.SketchEnabled
  }
  ```

Restricciones:
- No cambiar `SchemaVersion` del config (sigue en 1; el campo es aditivo con omitempty).
- No añadir lógica de validación del valor (el `*bool` lo garantiza el tipo).

#### web/src/types/board.ts

Acción: MODIFICAR

Cambios requeridos:
- Añadir la interface (antes de `Card`):
  ```ts
  /** Excalidraw wireframe sketch attached to a spec; mirrors Go state.SketchRef. */
  export interface SketchRef {
    name: string
    createdAt: string
  }
  ```
- Añadir a `Card` (junto a los campos opcionales existentes):
  ```ts
  /** Excalidraw sketches generated by vector-ui-ux-designer; absent when none. */
  sketches?: SketchRef[]
  ```

Restricciones:
- No cambiar los tipos existentes ni el comentario de cabecera del archivo.
- Mantener compatibilidad con instancias de `Card` sin `sketches`.

#### web/src/api/useFileContent.ts

Acción: MODIFICAR

Cambios requeridos:
- `ArtifactKey`: añadir `| 'sketch'` (completitud de tipo: el key existe en el contrato, y
  `entries.ts` lo referencia).
- **No** se añade manejo de blob/fetch en este hook. La descarga de sketch **no pasa por
  `useFileContent`**: es un `<a href="/api/file?spec=<id>&artifact=sketch" download>` nativo
  renderizado por `SpecArtifactBrowser` (ver §9 y el bloque de `SpecArtifactBrowser.tsx`). El
  endpoint sirve el archivo con `Content-Disposition: attachment`, así que el navegador descarga
  sin `res.blob()` ni `URL.createObjectURL`. (Resuelto en validación; ya no es TBD.)

Restricciones:
- No cambiar el comportamiento para los artifact keys existentes (`res.text()` sigue intacto).
- No romper la interfaz pública del hook: solo se amplía el union `ArtifactKey`, sin nuevos campos.

#### web/src/components/SpecDetailsDrawer/entries.ts

Acción: MODIFICAR

Cambios requeridos:
- Extender `ArtifactEntry`:
  ```ts
  export interface ArtifactEntry {
    key: ArtifactKey
    label: string
    download?: boolean  // true → serve as download, not as FilePreviewModal
  }
  ```
- En `entriesFor`: si `card.sketches && card.sketches.length > 0`, añadir una entrada por
  sketch:
  ```ts
  for (const sketch of card.sketches) {
    entries.push({ key: 'sketch', label: sketch.name, download: true })
  }
  ```

Restricciones:
- No cambiar el comportamiento de las entradas `spec`/`proposal`/`design`/`tasks`.
- Mantener la firma `entriesFor(card: Card): ArtifactEntry[]`.

#### web/src/components/SpecDetailsDrawer/entries.test.ts

Acción: MODIFICAR

Cambios requeridos:
- Añadir tests (patrón `makeCard(overrides)` existente):
  - Card con `sketches: [{ name: 'wireframe.excalidraw', createdAt: '...' }]` → genera una
    entrada `{ key: 'sketch', label: 'wireframe.excalidraw', download: true }`.
  - Card con `sketches: []` → no genera entradas de sketch.
  - Card sin `sketches` → no genera entradas de sketch.
  - Card con dos sketches → genera dos entradas, una por sketch.

Restricciones:
- No modificar los tests existentes.
- Los overrides de `makeCard` pueden omitir `sketches` (campo opcional en `Card`).

#### web/src/components/SpecDetailsDrawer/SpecArtifactBrowser.tsx

Acción: MODIFICAR

Cambios requeridos:
- Para entradas con `entry.download === true`, el item de lista **se renderiza como un
  `<a href download>` real** (no un `<button>` que dispara un anchor temporal): `href` =
  `/api/file?spec=${id}&artifact=sketch`, `download={entry.label}`. No se llama `setSelected`
  ni se abre `FilePreviewModal`. El navegador descarga nativamente (sin blob, sin
  `createObjectURL`). El `aria-label` va en este `<a>` renderizado (p.ej.
  `Descargar sketch ${entry.label}`); el ícono `Download` es decorativo (`aria-hidden`).
- Para entradas sin `download` (spec/proposal/design/tasks): comportamiento existente intacto
  (`setSelected(entry)` → `FilePreviewModal`).
- El ícono del item de sketch usa `Download` de `lucide-react` (ya disponible como dep) en vez
  de `FileText` para distinguir visualmente el tipo de acción.

Restricciones:
- No cambiar la lógica de preview del `FilePreviewModal` para los artifacts existentes.
- No abrir el `FilePreviewModal` para sketches (download-only en V1).
- No añadir `@excalidraw/excalidraw` ni ninguna librería de render.

#### kit/commands/vector/raw.md

Acción: MODIFICAR

Cambios requeridos:
- Añadir **después del paso 11** (report), un nuevo **paso 12 — Sketch Excalidraw (tail, opt-in)**:

  1. Si `--no-sketch` está presente en los argumentos, o si el config resuelto tiene
     `sketchEnabled === false` (leer del `CONTEXT` del paso 3 o del config.json), saltar
     directamente — no preguntar.
  2. Evaluar la heurística de UI sobre el spec compuesto leído desde `SPEC_PATH`: buscar
     señales en el título, body, §12 "Estados de UI", y keywords de capas afectadas (`board`,
     `drawer`, `web/`, `component`, `UI`, `pantalla`, `formulario`, `modal`). Si la señal es
     débil o ausente, saltar silenciosamente.
  3. Si la señal es fuerte, preguntar vía `AskUserQuestion`:
     > "¿Generar un wireframe Excalidraw para este spec? `vector-ui-ux-designer` correrá en
     > background y el sketch aparecerá en el drawer del spec cuando termine."
  4. Dev declina → cerrar el comando limpiamente; no registrar error.
  5. Dev confirma → spawnar `vector-ui-ux-designer` como subagente async/background pasando
     `SPEC_PATH` y `SPEC_ID`. El agente emitirá el JSON y llamará `vector spec attach-sketch`.
     No esperar la respuesta (el spec draft ya está registrado).
  6. Registrar el routing: `vector spec route <SPEC_ID> --model sonnet --baseline opus
     --task "generate ui sketch" --tokens-in <est> --tokens-out <est>`.

Restricciones:
- El paso 12 es TAIL: el draft ya está registrado antes de ejecutarlo.
- No reordenar los pasos 1–11 existentes.

#### kit/commands/vector/research.md

Acción: MODIFICAR

Cambios requeridos:
- Añadir **después del paso 14** (report), un nuevo **paso 15 — Sketch Excalidraw (tail, opt-in)**
  con la misma lógica que el paso 12 de `raw.md`: opt-out check → heurística → `AskUserQuestion`
  → spawn async → token routing.

Restricciones:
- El spec ya está registrado (paso 12 de research) antes de la pregunta.
- No reordenar los pasos 0–14 existentes.

---

## 7. API Contract

El contrato HTTP existente (`GET /api/file?spec=<id>&artifact=<key>`) se extiende con el
nuevo valor `artifact=sketch`:

- **Request**: `GET /api/file?spec=<id>&artifact=sketch`
- **Response 200**: bytes del `.excalidraw` (JSON compacto);
  `Content-Type: application/octet-stream`;
  `Content-Disposition: attachment; filename="<sketch-name>.excalidraw"`.
- **Response 400**: `{"error":"missing or unknown artifact query parameter"}` — cuando
  `artifact` no es un valor conocido del enum.
- **Response 404**: `{"error":"artifact \"sketch\" for spec \"<id>\" not found"}` — cuando el
  spec existe pero no tiene sketches (o el archivo en disco no existe).
- **Response 500**: solo en error de I/O inesperado (no en ausencia de sketch).

Para múltiples sketches, la disambiguation de URL (p.ej. `?name=<filename>`) es
TBD — ver Open questions. En V1 (`artifact=sketch` sin parámetro adicional), el handler
sirve el primero en `spec.Sketches`.

El campo `sketches` en `GET /api/board` es un array de `SketchRef` (`{name, createdAt}`) en
el `Card` — serializado si no vacío; omitido si vacío (omitempty Go → campo ausente en JSON).

No hay endpoint de escritura HTTP para sketches (el write pasa exclusivamente por
`vector spec attach-sketch`; nunca por un endpoint HTTP).

Los endpoints existentes (`/api/board`, `/api/events`, `/api/standup`, `/api/activity`,
`/api/summary`) no se modifican.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] En un spec de UI, `/vector:raw` muestra la sugerencia de sketch al final del flujo
      (paso 12), después del registro del draft card.
- [ ] Si el dev declina, el flujo termina limpiamente sin sketch ni error persistido.
- [ ] Si el dev confirma, el agente `vector-ui-ux-designer` produce un `.excalidraw` JSON con
      al menos los campos top-level `type`, `version`, `elements`.
- [ ] El binario (vía `vector spec attach-sketch`) rechaza silenciosamente un JSON malformado
      o sin `type`/`version`/`elements`: imprime el error a stderr y retorna exit 1, sin dejar
      el spec en estado de error.
- [ ] Un sketch válido se persiste en `.vector/specs/<id>/sketches/<name>.excalidraw` y el
      `state.json` del spec tiene el `SketchRef` en `Sketches`.
- [ ] El watcher de `vector serve` detecta el cambio de `state.json` y el board live-updates
      vía SSE sin reinicio del servidor.
- [ ] El `SpecDetailsDrawer` muestra el botón de descarga etiquetado con el nombre del sketch.
- [ ] Hacer clic en el botón descarga el `.excalidraw` en el navegador (no abre el `FilePreviewModal`).
- [ ] `GET /api/file?spec=<id>&artifact=sketch` retorna 200 con `Content-Type:
      application/octet-stream` y `Content-Disposition: attachment`.
- [ ] `GET /api/file?spec=<id>&artifact=sketch` sobre un spec sin sketch retorna 404.
- [ ] Con `--no-sketch` en `/vector:raw`, la sugerencia no aparece.
- [ ] Con `sketchEnabled: false` en `.vector/config.json`, la sugerencia no aparece.
- [ ] Para specs sin señal de UI, la heurística es silenciosa (no pregunta nada).
- [ ] Un `SpecState` existente sin `Sketches` carga sin error y sin migración.
- [ ] `TestAssetsMatchKit` pasa: la copia en `scaffold/assets/agents/` coincide con
      `kit/agents/vector-ui-ux-designer.md`.
- [ ] `gofmt -l cli` no lista archivos; `go vet ./...` y `go test ./...` verdes; typecheck
      TypeScript sin errores; `npm run build` produce un bundle sin advertencias de tamaño
      anómalo.

### Tests requeridos

Agregar o actualizar tests para:

- [ ] `state/types`: round-trip de `SketchRef` y `Sketches` (set / omitido); carga de
      `SpecState` legacy sin el campo → `Sketches == nil` sin error.
- [ ] `state/artifact`: `artifactRelPath("sketch", ...)` con sketch presente → path correcto;
      sin sketches → `fs.ErrNotExist`.
- [ ] `state/store` (o `cmd/vector/sketch_test.go`): `AttachSketch` con JSON válido → archivo
      persistido + state actualizado; JSON inválido → error descriptivo; spec inexistente →
      error.
- [ ] `board/board`: `Build` con spec con sketches proyecta `Card.Sketches` correctamente;
      spec sin sketches proyecta campo ausente.
- [ ] `entries.ts` (vitest): card con sketches genera entradas con `download: true`; card sin
      sketches no genera entradas de sketch; card con dos sketches → dos entradas.
- [ ] Tests existentes en `entries.test.ts` no deben romperse con los nuevos overrides.

### Comandos de verificación

```bash
# Backend (Go) — desde la raíz del repo
go -C cli generate ./internal/scaffold   # regenera la copia de assets del agente
gofmt -l cli                             # no debe listar archivos
go -C cli vet ./...
go -C cli test ./...
go -C cli build ./...

# Frontend (TypeScript)
npm --prefix web run typecheck
npm --prefix web run lint
npm --prefix web test                    # vitest
npm --prefix web run build               # bundle ligero
```

La fase no está completa si alguno de estos comandos falla o si `gofmt -l` lista archivos.

---

## 9. Criterios de UX

### SpecDetailsDrawer — entradas de sketch

- Los sketches aparecen en la misma lista que los otros artifacts del spec dentro de
  `SpecArtifactBrowser`.
- Cada sketch es un item de lista con ícono `Download` (de `lucide-react`) + nombre del
  archivo (`<name>.excalidraw`), visualmente distinto de los items de preview (que usan
  `FileText`).
- **Accesibilidad**: el nombre del archivo se renderiza como texto visible adyacente al ícono
  (el ícono `Download` es decorativo → `aria-hidden`). El elemento es un `<a download>` con
  `aria-label` explícito (p.ej. `Descargar sketch <name>.excalidraw`) para que el screen reader
  anuncie la acción aunque el texto visible cambie.
- Al hacer clic, la descarga se inicia inmediatamente en el navegador (comportamiento nativo
  de `<a download>` apuntando a `/api/file?spec=<id>&artifact=sketch`). No se abre el
  `FilePreviewModal` ni ninguna modal de preview, y **no** pasa por `useFileContent` (es un
  enlace nativo, no un fetch del hook).
- Si no hay sketches, no aparece ninguna sección adicional de sketch — el browser muestra
  los artifacts existentes o el empty state ya existente (`No source files available.`).

### Sugerencia en el command (paso 12 / paso 15)

- La pregunta de `AskUserQuestion` es concisa y accionable: describe qué agente correrá y qué
  pasará cuando termine (sketch disponible en el drawer).
- La sugerencia aparece SOLO al final del flujo (post-registro del draft) y solo si la
  heurística detecta señal UI — nunca interrumpe el flujo principal de spec authoring.
- El dev puede declinar sin consecuencias para el spec.

### Feedback async

- El command retorna inmediatamente tras confirmar; el mensaje final le indica al dev que el
  sketch llegará al drawer cuando el agente termine.
- No hay spinner en el board durante la generación (async fuera del ciclo de UI).
- Cuando el sketch aparece (via SSE live-update), el `SpecDetailsDrawer` lo muestra sin
  animación especial — el re-render del board maneja la frescura.

### Estados de UI relevantes

| Estado | Qué se muestra en el drawer | Acción del usuario |
|---|---|---|
| Sin sketches | Lista de artifacts existentes (sin sección de sketch) | Acciones normales |
| Sketch disponible | Item de lista (`<a download>`) con ícono `Download` + nombre del archivo | Clic para descargar |
| Error de descarga (no-200) | Sin error inline en V1: la descarga es un `<a download>` nativo, no un fetch del hook → el navegador maneja el fallo nativamente | Reintentar (clic de nuevo) |
| Generación en curso (agente async) | Ningún cambio visible en V1 (no hay estado pending) | Esperar |

### Subsecciones no aplicables

- **Formularios / inputs**: No aplica — el feature no introduce ningún formulario ni input de
  texto en el board; la única interacción nueva es el clic de descarga.
- **Passwords / campos sensibles**: No aplica — no hay credenciales ni datos sensibles.
- **Navegación / rutas**: No aplica — no se añaden rutas ni cambia la navegación; todo ocurre
  dentro del `SpecDetailsDrawer` ya existente.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas; el agente no debe cuestionarlas ni cambiarlas:

1. **Agente embebido, sin dependencia de skill global ni MCP**: `vector-ui-ux-designer` vive
   en `kit/agents/` y se vendoriza en el binario. Nunca asume que existe `~/.claude/` con una
   skill de UI o un MCP de Excalidraw. Razón: `architecture/distribution-packaging.md` exige
   que todo agente invocado por un command `/vector:*` esté embebido — no puede depender del
   entorno del usuario. El brief del usuario mencionó `/ui-ux-pro-max` y un "excalidraw mcp"
   como punto de partida; estos son ideas del contexto original, no dependencias del spec.

2. **`.excalidraw` download-only, sin render**: el board muestra un botón de descarga, no una
   thumbnail ni un preview inline. Razón: el render in-binary requeriría un runtime JS
   (`cli/go.mod` sin deps confirma que es stdlib-only); el render inline requeriría
   `@excalidraw/excalidraw` (~2–3 MB), infringe la regla de bundle ligero de `web/CLAUDE.md`.

3. **Async-in-session + SSE, sin daemon/worker verdadero**: el sketch se genera dentro de la
   sesión Claude como subagente async; cuando termina llama al binario; el watcher de
   `vector serve` propaga el cambio. Razón: un daemon en el binario obligaría al binario a
   realizar llamadas LLM propias — hoy el binario hace cero llamadas LLM (confirmado en
   `cli/cmd/vector/route.go` y `cli/cmd/vector/summarize.go`, que solo calculan el costo del
   meter sin invocar ninguna API).

4. **Storage en `.vector/specs/<id>/sketches/<name>.excalidraw`**: el directorio `sketches/`
   está bajo el espacio del spec. Razón: sigue el patrón shard-por-spec del estado; el
   prefijo ya está en los allowed-prefixes de `verifyArtifactPath`.

5. **Schema aditivo sin bump de `state.SchemaVersion`**: `Sketches []SketchRef` se añade con
   `omitempty`, igual que `QuickWin bool` (línea 145 de `types.go`). Razón: es retrocompatible
   — un `SpecState` previo deserializa con `Sketches == nil` sin error. No se añade migración.

6. **Sonnet para `vector-ui-ux-designer`**: el agente corre en Sonnet, no en Haiku. Razón: la
   síntesis de layout/jerarquía visual desde requisitos de spec es razonamiento real, no
   generación mecánica de texto — justifica el tier Sonnet según `product/token-routing.md`.
   La heurística y la orquestación permanecen en el main loop (baratos).

7. **Heurística híbrida con confirmación + opt-out global**: la heurística evalúa el spec al
   TAIL y solo pregunta si detecta señal UI; el opt-out (`--no-sketch` y
   `config.sketchEnabled: false`) suprime la pregunta para quienes nunca la quieren. Razón:
   la generación de sketches es opcional y potencialmente costosa (Sonnet) — forzarla
   rompería `product/token-routing.md`; el developer decide.

8. **Validación del output JSON antes de persistir**: el binario verifica `{type, version,
   elements}` antes de copiar el archivo; salida malformada produce un rechazo silencioso
   (soft failure). Razón: el agente podría emitir output malformado o con prosa adicional;
   persistir un `.excalidraw` roto engañaría al board y al usuario que lo descarga. El
   rechazo silencioso preserva la integridad del spec.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, no
implementarla.

---

## 11. Edge cases

### Heurística de UI

- Spec sin ninguna señal de UI (backend puro, CLI-only) → la heurística no dispara; el paso
  12/15 salta sin preguntar.
- Spec con señal ambigua (p.ej. `component` en contexto Go, no React) → aplica la regla de §5
  paso 2: un único keyword suelto es señal débil y **no** dispara la pregunta (conservador). El
  tuning fino del umbral (pesos, lista de keywords) queda como mejora futura — ver Open
  questions #3, pero la regla de decisión de V1 está definida y es implementable.
- Spec de bug con UI afectada → pregunta normalmente; el dev decide si quiere el sketch.

### Opt-out

- `--no-sketch` presente → ninguna pregunta, salta limpiamente.
- `sketchEnabled: false` en config → ídem.
- `sketchEnabled: null` / ausente → habilitado por defecto (nil = enabled).
- Error al leer config → ignorar, asumir habilitado (soft-fail, no abortar el flujo).

### Agente y generación

- El agente emite JSON malformado → `vector spec attach-sketch` retorna exit 1 con mensaje
  descriptivo; el command captura como soft failure y reporta que el sketch no pudo generarse
  (el spec sigue válido como draft sin sketch).
- El agente emite JSON válido pero sin `type`/`version`/`elements` → ídem, rechazo silencioso.
- La sesión Claude termina antes de que el agente finalice → el sketch simplemente no aterriza
  (no se persiste ningún estado de error en el `state.json`; el spec queda como draft sin
  sketch). No hay estado `sketch-pending` en V1.
- El agente llama `attach-sketch` para un spec ya cerrado o archivado → el binario persiste
  el sketch igual (no se bloquea por status); si el estado terminal impide la escritura, el
  binario retorna error descriptivo.

### Storage

- El directorio `.vector/specs/<id>/sketches/` no existe → `AttachSketch` lo crea vía
  `os.MkdirAll`.
- Nombre de sketch que colisiona con uno ya existente en el mismo spec → TBD — ver Open
  questions (comportamiento V1: sobrescribir; una alternativa es añadir sufijo numérico).
- Archivo temporal (`--file`) no existe → `attach-sketch` retorna error: `"file not found: <path>"`.

### Artifact serving

- `GET /api/file?spec=<id>&artifact=sketch` con spec sin sketches → 404.
- Spec con múltiples sketches → V1 sirve el primero en `Sketches`; la disambiguation de URL
  para sketches adicionales es TBD — ver Open questions.
- Archivo `.excalidraw` en disco eliminado externamente → `ReadSpecArtifact` retorna
  `fs.ErrNotExist` → 404 (el handler lo mapea correctamente; no es un 500).
- **Códigos HTTP no aplicables**: 401/403/409/422/429 **no aplican** — `/api/file` es un
  endpoint local sin capa de auth/sesión, sin semántica de edición concurrente y sin rate
  limiting (binario efímero en `localhost`, ver `serve.go`). Solo se manejan 200 / 400 (artifact
  key inválido) / 404 (spec o archivo ausente) / 500 (error de I/O inesperado).

### Frontend

- Card sin `sketches` (specs existentes antes de este cambio) → `entries.ts` no genera
  entradas de sketch; `SpecArtifactBrowser` no muestra sección extra.
- Descarga falla (red, 500 del servidor) → al ser un `<a download>` nativo (no el hook
  `useFileContent`), no hay error inline en V1: el navegador maneja el fallo nativamente; el
  dev reintenta con otro clic. (Consistente con §9.)
- Descarga que cuelga (I/O lento, stall de red) → comportamiento nativo del navegador; V1 no
  impone timeout ni aborta desde el front (no hay `fetch`/`AbortController` en el path de
  descarga). El handler `/api/file` sirve un archivo local pequeño, así que un cuelgue real es
  improbable; si se necesitara un timeout, sería una mejora futura del handler, no del front.
- Dos sketches en el mismo spec → `entries.ts` genera dos entradas; el browser muestra dos
  botones de descarga.

---

## 12. Estados de UI requeridos

Esta feature modifica `SpecArtifactBrowser` dentro del `SpecDetailsDrawer`. Los estados
relevantes son:

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| Sin sketches | Lista de artifacts existentes (spec, proposal, etc.) sin items de sketch | Acciones normales del drawer |
| Sketch disponible (idle) | Item de lista con ícono `Download` + nombre `.excalidraw` | Clic para descargar |
| Error de descarga | Sin error inline en V1: descarga nativa `<a download>` (no usa `useFileContent`), el navegador maneja el fallo | Reintentar (clic de nuevo) |
| Generación async en curso | Ningún estado visual en V1 (el drawer no sabe que hay un agente corriendo) | Esperar |

La card del board (columnas del kanban) **no** muestra badge de sketch en V1 — la existencia
de sketches solo es visible desde el `SpecDetailsDrawer`.

---

## 13. Validaciones

### Validaciones del binario (`vector spec attach-sketch`)

| Campo | Regla | Comportamiento en fallo |
|---|---|---|
| `<id>` (posicional) | Requerido; spec debe existir en el store | Error: `"spec not found: <id>"` |
| `--file <path>` | Requerido; el archivo debe existir | Error: `"file not found: <path>"` |
| Contenido del archivo | JSON válido (`json.Unmarshal`) | Error: `"invalid JSON: <err>"` |
| Forma top-level | Keys `type`, `version`, `elements` presentes | Error: `"not a valid Excalidraw document: missing key \"<key>\""` |
| `--name <nombre>` | Opcional; sin `/`, `..` ni caracteres peligrosos | Error: `"invalid sketch name: <name>"` |

### Validaciones del agente (`vector-ui-ux-designer`)

- El agente debe emitir únicamente el JSON del `.excalidraw` (sin prosa adicional, sin code
  fence) — hard rule en el agente. La validación final la hace el binario.
- El agente incluye los campos obligatorios de cada elemento Excalidraw para producir un
  documento apto para abrir en Excalidraw directamente.

### Validaciones del frontend

No hay validación de input de usuario (el botón de descarga no tiene formulario). El hook
`useFileContent` surfacea el error de fetch si la respuesta no es 200 **solo para los artifacts
existentes** (spec/proposal/design/tasks). El artifact sketch es un `<a download>` nativo y no
pasa por `useFileContent` — §11 y §12 detallan el comportamiento en fallo.

---

## 14. Seguridad y permisos

- **CLI-owns-writes**: el agente `vector-ui-ux-designer` no escribe en `.vector/` directamente
  — siempre llama a `vector spec attach-sketch`. El binario valida el input antes de persistir.
- **Path traversal eliminado por diseño**: el cliente de `/api/file` nunca envía un path, solo
  un ID y un artifact key — el traversal está eliminado por el contrato. `verifyArtifactPath`
  añade defensa en profundidad verificando que el path resuelto quede bajo los allowed-prefixes
  del spec (ya cubre `.vector/specs/<id>/`).
- **Sanitización del nombre del sketch**: el binario sanitiza `--name` para prevenir inyección
  de path (sin `/`, `..`, ni caracteres peligrosos del sistema de archivos).
- **El `.excalidraw` es JSON no sensible**: no contiene secretos, tokens ni PII. El header
  `Content-Disposition: attachment` fuerza descarga sin ejecución en el browser.
- **No llamadas LLM en el binario**: `vector spec attach-sketch` es puro I/O — no invoca
  ninguna API externa. El binario mantiene su invariante de cero llamadas LLM.
- **Opt-out de config**: `sketchEnabled: false` se persiste en `.vector/config.json`
  (versionado/compartido por el equipo), lo que permite suprimir la feature a nivel de repo
  sin depender de flags individuales en CI.

---

## 15. Observabilidad y logging

- `vector spec attach-sketch` imprime a stdout al completar:
  `attached sketch "<name>" to spec "<id>"`. Con `--json`: `{"id":"<id>","sketch":"<name>"}`.
- Si la validación del JSON falla, el binario imprime a stderr:
  `invalid Excalidraw document: missing key "elements"` (u otro detalle del campo ausente).
- Si el archivo fuente no existe: `file not found: <path>` a stderr, exit 1.
- El command (`raw.md` / `research.md`) reporta en su mensaje final si el sketch fue
  spawneado, si fue saltado por opt-out, o si la heurística no detectó señal UI.
- No se añade logging adicional en el pipeline del agente más allá del comportamiento
  estándar del harness de Claude Code.
- No se loguean los bytes del `.excalidraw` (potencialmente grande; no útil en logs).

---

## 16. i18n / textos visibles

Vector no tiene un sistema de i18n para la web en V1. Los textos visibles son:

| Contexto | Texto | Idioma |
|---|---|---|
| Botón de descarga en `SpecArtifactBrowser` | Nombre del archivo (e.g. `wireframe.excalidraw`) + ícono `Download` | N/A (nombre de archivo; no se traduce) |
| Empty state del browser (existente) | `No source files available.` | Inglés (no modificar) |
| Pregunta de `AskUserQuestion` en el command | Texto descriptivo de la acción y su resultado | Idioma de `config.language` si está configurado; si no, idioma de la conversación (patrón de `vector-standup-writer`) |
| Salida de `attach-sketch` | `attached sketch "<name>" to spec "<id>"` | Inglés (ayuda del CLI siempre en inglés) |
| Errores del binario | `invalid Excalidraw document: missing key "<key>"`, etc. | Inglés |

No hay claves de traducción ni sistema de i18n que tocar. Los textos del UI (web) son inglés
hardcodeado, igual que el resto del board en V1.

---

## 17. Performance

- **Heurística de UI**: evaluación en el main loop sobre el texto del spec ya en memoria —
  O(n) en el tamaño del spec, sin I/O adicional. Costo despreciable.
- **Spawn async del agente**: el command retorna inmediatamente; la generación no bloquea el
  flujo de spec authoring. El dev no espera el sketch para seguir trabajando.
- **`vector spec attach-sketch`**: copia de un archivo JSON (típicamente < 500 KB para un
  wireframe simple) + escritura atómica del `state.json`. I/O puntual.
- **SSE live-update**: el watcher de `vector serve` hace polling en `.vector/` cada `poll-ms`
  (default 1 000 ms). El cambio se detecta y propaga en el próximo tick sin coste adicional
  (el mecanismo ya existía).
- **Frontend**: el campo `sketches` es omitempty → boards sin sketches no transmiten datos
  adicionales. El botón de descarga no carga el archivo hasta el clic (lazy by design).
- **Bundle**: cero dependencias npm nuevas. El peso del bundle no aumenta.
- **Token routing**: `vector-ui-ux-designer` (Sonnet) consume tokens; el coste se registra vía
  `vector spec route` para el Token Savings Meter. La sugerencia es opt-in, así que el gasto
  de tokens del agente solo ocurre cuando el dev lo pide.

---

## 18. Restricciones

El agente no debe:

- **Asumir que existe una skill global `~/.claude/` o un MCP de Excalidraw**: el agente
  `vector-ui-ux-designer` debe ser 100% autónomo con el conocimiento del formato Excalidraw
  vendorizado en su body (`architecture/distribution-packaging.md`).
- **Añadir un runtime JS o proceso persistente al binario Go**: el binario se mantiene
  stdlib-only sin dependencias externas (confirmado por `cli/go.mod` sin go.sum).
- **Instalar `@excalidraw/excalidraw` ni ninguna librería de render de Excalidraw**: prohíbe
  explícitamente `web/CLAUDE.md` por el peso del bundle (~2–3 MB).
- **Editar `cli/internal/scaffold/assets/` a mano**: el directorio es una copia generada;
  siempre se regenera con `go generate ./internal/scaffold`. Drift detectado por
  `TestAssetsMatchKit` (`cli/internal/scaffold/scaffold_test.go`).
- **Hacer bump de `state.SchemaVersion`** por el campo aditivo `Sketches`: omitempty y
  retrocompatible; el SchemaVersion se mantiene en 1.
- **Hacer que el binario llame a una API LLM**: `vector spec attach-sketch` es puro I/O; el
  binario no tiene presupuesto LLM hoy.
- **Escribir en `.vector/` desde el agente directamente**: el agente llama al subcomando; el
  binario es el único escritor (CLI-owns-writes).
- **Añadir thumbnails o render inline** del `.excalidraw` en V1: download-only es la decisión
  tomada en §10.
- **Generar sketches para specs sin señal de UI** ni preguntar en todo spec: la heurística
  filtra; el opt-out lo suprime.
- **Traducir labels o contenido del `.excalidraw`**: el sketch está fuera de scope de i18n.
- **Refactorizar código no relacionado** con el alcance definido en §2.
- **Cambiar las 8 decisiones tomadas** listadas en §10, aunque parezca haber una alternativa
  mejor — reportarla como observación, no implementarla.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `kit/agents/vector-ui-ux-designer.md` — agente Sonnet nuevo con conocimiento Excalidraw
      vendorizado, hard rules claras, output shape exacto (JSON puro sin prosa).
- [ ] `cli/internal/scaffold/assets/agents/vector-ui-ux-designer.md` — copia embebida
      regenerada con `go generate` y verificada contra la fuente (`TestAssetsMatchKit` verde).
- [ ] `cli/internal/state/types.go` — `SketchRef` struct + `Sketches []SketchRef` omitempty
      en `SpecState`. `SchemaVersion` intacto en 1.
- [ ] `cli/internal/state/artifact.go` — caso `"sketch"` en `artifactRelPath` con
      `fs.ErrNotExist` cuando sin sketches.
- [ ] `cli/internal/state/store.go` — método `Store.AttachSketch` que valida, persiste y
      actualiza `state.json` atómicamente.
- [ ] `cli/internal/board/board.go` — `Sketches []state.SketchRef` omitempty en `Card`;
      propagación en `toCard`.
- [ ] `cli/internal/board/server.go` — `"sketch"` en `validArtifact`; Content-Type
      `application/octet-stream` + `Content-Disposition: attachment` en `handleFile`.
- [ ] `cli/cmd/vector/sketch.go` — nuevo `runSpecAttachSketch` (valida, persiste, actualiza
      state; maneja flags `--file`, `--name`, `--repo-root`, `--json`).
- [ ] `cli/cmd/vector/main.go` — dispatch de `"attach-sketch"` en el switch de `runSpec`.
- [ ] `cli/internal/config/config.go` — campo `SketchEnabled *bool` omitempty + helper
      `IsSketchEnabled()`.
- [ ] `web/src/types/board.ts` — `SketchRef` interface + `sketches?` en `Card`.
- [ ] `web/src/api/useFileContent.ts` — `ArtifactKey` ampliado con `| 'sketch'` (completitud
      de tipo); sin blob ni fetch — la descarga pasa por `<a href download>` nativo en
      `SpecArtifactBrowser`.
- [ ] `web/src/components/SpecDetailsDrawer/entries.ts` — `ArtifactEntry.download` flag +
      entradas de sketch en `entriesFor`.
- [ ] `web/src/components/SpecDetailsDrawer/entries.test.ts` — tests de sketch (vitest).
- [ ] `web/src/components/SpecDetailsDrawer/SpecArtifactBrowser.tsx` — descarga directa para
      entradas con `download: true`; `FilePreviewModal` intacto para otros artifacts.
- [ ] `kit/commands/vector/raw.md` — paso 12 TAIL (heurística + opt-in + spawn async + routing).
- [ ] `kit/commands/vector/research.md` — paso 15 TAIL ídem.
- [ ] Tests Go añadidos/actualizados: `state/types`, `state/artifact`, `state/store`,
      `board/board`, `cmd/vector/sketch`.
- [ ] Gate verde: `go generate`, `gofmt -l`, `go vet`, `go test`, `go build`, typecheck web,
      lint web, build web.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo y el alcance de §2.
- [ ] No implementé nada fuera del scope (no render, no daemon, no `@excalidraw/excalidraw`,
      no edición manual de assets/).
- [ ] `vector-ui-ux-designer` lleva el conocimiento Excalidraw vendorizado en su body y no
      asume `~/.claude/` ni MCP externo.
- [ ] `SketchRef` y `Sketches` son omitempty; `state.SchemaVersion` se mantiene en 1; un
      `SpecState` legacy carga sin error.
- [ ] El caso `"sketch"` en `artifactRelPath` retorna `fs.ErrNotExist` cuando `Sketches` está
      vacío (handler lo mapea a 404, no 500).
- [ ] `validArtifact` en `server.go` acepta `"sketch"`; `handleFile` sirve
      `application/octet-stream` + `Content-Disposition: attachment` para sketch.
- [ ] `runSpecAttachSketch` (en `sketch.go`) valida la forma JSON (`Unmarshal` + chequeo de
      `{type, version, elements}`) **antes** de llamar a `Store.AttachSketch`; el `Store` recibe
      bytes ya validados y solo escribe. El rechazo es soft failure (exit 1, no fatal para el spec).
- [ ] `SketchEnabled *bool` diferencia `nil` (habilitado) de `false` (deshabilitado explícito);
      `IsSketchEnabled()` lo booleaniza correctamente.
- [ ] El dispatch de `"attach-sketch"` está registrado dentro de `runSpec` en `main.go`.
- [ ] Las entradas de sketch en `entries.ts` tienen `download: true`; `SpecArtifactBrowser`
      sirve descarga directa sin abrir `FilePreviewModal`.
- [ ] Los pasos TAIL en `raw.md` y `research.md` están AL FINAL del flujo, después del
      registro del draft card (paso 11 / paso 14 respectivamente).
- [ ] Regeneré `cli/internal/scaffold/assets/agents/vector-ui-ux-designer.md` con
      `go generate ./internal/scaffold` y verifiqué que coincide con la fuente.
- [ ] No añadí dependencias externas (Go: cero; npm: cero).
- [ ] No cambié las 8 decisiones tomadas en §10.
- [ ] Ejecuté `go generate`, `gofmt -l`, `go vet`, `go test`, `go build`, typecheck web, lint
      web, build web — todos verdes.
- [ ] No dejé logs temporales ni TODOs sin justificar.

---

## Open questions

1. **Render inline / thumbnail**: la previsualización inline del sketch en el drawer o en la
   card está fuera de scope en V1. ¿En qué condición se desbloquea? Requiere evaluar
   `@excalidraw/excalidraw` vs un renderer propio vs SVG export del lado del agente antes de
   establecer un plan viable que no infrinja la regla de bundle ligero.

2. **Daemon/worker background verdadero**: el modelo async-in-session degrada si la sesión
   Claude se cierra antes de que el agente termine. ¿Hay un modelo viable para hacer la
   generación completamente desacoplada de la sesión Claude? Requeriría que el binario realice
   llamadas LLM propias — hoy explícitamente fuera de scope.

3. **Precisión de la heurística de UI** (regla V1 ya definida en §5 paso 2: §12 no-vacía **o**
   ≥2 keywords). Pendiente solo el *tuning fino*: ¿conviene un scoring ponderado en vez del
   conteo binario? ¿La lista de keywords necesita ajuste tras observar falsos positivos reales?
   El umbral de V1 es implementable; esto es mejora futura, no bloqueante.

4. **Historial de versiones de sketches**: hoy se almacenan múltiples sketches como entradas
   independientes en el array `Sketches`. ¿Se necesita un mecanismo para versionar el mismo
   sketch (v1, v2, ...) con capacidad de comparación, en vez de añadir entradas nuevas?

5. **Dark-mode theming del `.excalidraw`**: los tokens CSS del board son Dracula-based
   (`web/src/styles/tokens.css`). Si en el futuro se renderiza el sketch, ¿cómo se aplica el
   tema? El campo `appState.theme` del formato Excalidraw acepta `"light"` / `"dark"` — el
   agente podría emitirlo en `"dark"` por defecto para coherencia con el board.

6. **Disambiguation de URL para múltiples sketches**: cuando un spec tiene más de un sketch,
   `?artifact=sketch` sirve el primero. ¿Se añade `?name=<filename>` como query param
   adicional en el handler `/api/file`, o se cambia el esquema del `ArtifactKey` frontend a
   un valor compuesto?

7. **`board.SchemaVersion` bump**: el campo `Sketches []SketchRef` en `Card` es aditivo y el
   frontend lo trata como opcional (`sketches?`). ¿El board contract (`board.SchemaVersion = 2`
   en `cli/internal/board/board.go` línea 16) requiere bump por este cambio, o se mantiene
   en 2 al ser un campo omitempty/backward-compatible?

---

## Reporte de viabilidad

> Investigación previa a la emisión (lentes `technical` + `design`, reviewers Sonnet escépticos,
> anclados al repo real). Los veredictos son los emitidos por cada reviewer **antes** de los
> pivotes; las dos decisiones que el lente técnico marcó como inviables (pre-render SVG/PNG
> in-binary y daemon background) fueron **revisadas** y resueltas a `.excalidraw` descargable +
> async-in-session, eliminando los bloqueadores. El riesgo remanente es de diseño.

| Lente | Veredicto | Confianza | Hallazgos clave | Riesgos |
|---|---|---|---|---|
| technical | go-with-risks | 3/10 | Binario Go stdlib-only (`cli/go.mod`, sin `go.sum`) → **sin renderer Excalidraw nativo ni runtime JS**: el pre-render SVG/PNG in-binary es inviable y `@excalidraw/excalidraw` (~2-3MB) rompe el bundle ligero. `vector serve` es efímero sin worker (`serve.go` `watchState`+`Broadcast`) → "background" colapsa a async-in-session. Añadir artifact key `sketch` (`artifact.go`, `server.go`) y `Sketches []SketchRef` omitempty (`types.go:145`, patrón `QuickWin`) es aditivo y localizado. | Pivote a `.excalidraw` descargable (sin render) elegido; async-in-session muere si se cierra la sesión (degrada suave); sin validación, JSON malformado se acumula silencioso → mitigado con validación de output. |
| design | go-with-risks | 5/10 | Patrón drawer+modal encaja (`SpecDetailsDrawer`, `FilePreviewModal`), pero el thumbnail inline en drawer de 440px (`SpecDetailsDrawer.module.css:22`) es ilegible → botón de descarga en su lugar. El contrato de artifacts sirve solo texto (`useFileContent.ts:4`, `res.text()`). Auto-suggest mete fricción (~66% ruido en specs no-UI) → opt-out. Sin precedente de clasificador UI en el repo. | Falsos positivos de detección; techo de fidelidad texto→wireframe dudoso para devs (pueden modelar el layout desde el spec); colisión de colores en dark-mode (`tokens.css` Dracula) si se renderiza a futuro. |
| security | No corrida — no aplica | — | Sin auth/PII/secretos; artifacts locales en `.vector/specs/<id>/sketches/`, mismo modelo de permisos que el spec doc; sin llamadas externas. | — |
| marketing | No corrida — no aplica | — | Capability de workflow interno (dev-focused), no pricing/growth/positioning. | — |

**Veredicto consolidado:** go-with-risks — construible y de valor real (sketches de diseño auto para specs de UI), una vez revisadas las dos decisiones técnicamente inviables (ya resueltas a `.excalidraw` descargable + async-in-session+SSE). El riesgo vivo es de diseño: el ROI y el techo de fidelidad de un wireframe generado por texto para un público de developers, mitigado con opt-out global y artifact download-only en V1.
