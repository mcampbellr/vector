# Spec: Cache de inteligencia de repo (clase C) con invalidación por fingerprint por dominio

## 1. Objetivo

Construir la **capa de conocimiento persistente "clase C"** (inteligencia de repo derivada) del
framework de orquestación de Vector: un cache local, gitignored y regenerable, que el **binario
Go** produce y consume, validado por un **fingerprint de contenido (sha256) por dominio** sobre
el working-tree, para que los commands `/vector:*` dejen de re-inspeccionar el repo en cada
invocación (hallazgo #3 de `docs/orchestration-review.md`) **sin** que ningún agente dependa de
información obsoleta que degrade la implementación.

Esta feature permite que **el binario `vector`** pueda **servir contexto de repo ya resuelto y
verificado-fresco** (techstack, framework, runtime, índice de estructura, entry points) a los
commands del kit para **minimizar re-inspección, re-razonamiento y tokens repetidos por command**,
regenerando solo el conocimiento cuyo fingerprint cambió.

Cierra el gap del spec `vector-context-cached-setup`: hoy `vector context` cachea el setup
(build/lint/test, language) en `config.json` pero **solo se invalida re-corriendo `vector update`
manual** — no hay detección automática de obsolescencia. Este spec añade el oráculo de validez.

Reencuadre (de `docs/knowledge-architecture.md` §0) — tres clases de conocimiento que hoy están
mal mezcladas:

- **Clase A** — conocimiento autoral humano (`.claude/rules/`, `CLAUDE.md`), committed.
- **Clase B** — estado de dominio transaccional (`.vector/config.json`, `.vector/specs/<id>/`),
  committed, sharded, lo escribe el binario.
- **Clase C** — inteligencia de repo derivada (**esta feature**), gitignored, regenerable.
- **Clase D** — logs personales/derivados (`.vector/local/`), gitignored.

Esta feature implementa **solo la clase C**. El binario Go es el **único escritor**
(CLI-owns-writes), invariante que no se toca.

---

## 2. Alcance

### Incluido en esta fase (núcleo P1+P2 del roadmap de `docs/knowledge-architecture.md` §11)

- **Estructura `.vector/cache/`** (gitignored, nueva, hermana de `.vector/local/` y
  `.vector/tmp/`) con tres artefactos JSON:
  - `fingerprints.json` — oráculo de validez por dominio.
  - `repo-intel.json` — stack / framework / runtime detail + paths de tsconfig.
  - `structure-index.json` — árbol indexado por workspace + entry points (derivado de
    `git ls-files` clasificado). **Incluido completo** en esta fase.
  - Añadir `.vector/cache/` al `.gitignore`.
- **Fingerprint = content-hash sha256 por dominio sobre el working-tree** (no commit hash de
  HEAD, no mtime). Cinco dominios fijos (`stack`, `deps`, `build`, `workspace`, `structure`) con
  su set autoritativo de fuentes (§5, §7). Hashing de dominios en paralelo (goroutines +
  `sync.WaitGroup`, `crypto/sha256` stdlib).
- **Reglas de invalidación**: recompute-on-read por dominio, fingerprint sobre working-tree, DAG
  de dependencia entre dominios, bump de `schemaVersion`/`kitVersion` invalida todo (§5, §11).
- **Extensión de `vector context`** (subcomando ya existente en `cli/cmd/vector/context.go`):
  - Validar el cache por content-hash por dominio antes de devolver valores derivados; regenerar
    el dominio que caducó.
  - Flag `--refresh`: fuerza regeneración de todos los artefactos y reescribe `fingerprints.json`.
  - Proyección scoped `--for <command>`: devuelve solo el slice que el command necesita, vía un
    **mapa estático command→dominios en el binario** (Go, no en los `.md` por ahora).
  - Backward-compat: `vector context --json` sin flags sigue devolviendo el `ContextOutput`
    actual (campos nuevos additive con `omitempty`).
- **Nuevo paquete `cli/internal/intel`** (un paquete por concern; sin `util`/`common`): lógica de
  fingerprint por dominio + generación de `repo-intel.json` y `structure-index.json` +
  lectura/validación/escritura atómica del cache bajo `.vector/cache/`. Consumido por
  `cli/cmd/vector/context.go`.
- **Anti-patrón "no persistir"** materializado (§10, §18): `dep-graph.json` **no se genera**;
  git metadata se recalcula siempre; tool availability nunca se persiste; convenciones detectadas
  no se auto-escriben a `.claude/`.
- **Tests table-driven** (paquete `testing` estándar) para digest determinista, invalidación
  on-read, DAG, working-tree vs HEAD, bump de versión, proyección `--for`, `--refresh`.

### Fuera de scope (otra fase — follow-up explícito)

- **Cableado de los tiers de validación** (TRUST / LAZY-VALIDATE / FULL-VALIDATE) **dentro de los
  archivos `.md` de los commands del kit** (`raw.md`, `bug.md`, `apply.md`, `comment.md`, etc.) y
  su re-vendorizado vía `go generate`. Esta fase deja el mapa command→dominios y la proyección
  `--for` **listos en el binario**; consumirlos desde los `.md` es la fase siguiente.
- **Granularidad por (dominio × shard de workspace)** en monorepos (`fingerprints.json` indexado
  por `domain/shard`). Esta fase usa fingerprint por dominio a nivel repo; el sharding por
  workspace es follow-up.
- **`board.json` a disco / mover a `cache/`**: **No aplica / inerte** — el ítem P1 #3 del design
  doc (`docs/knowledge-architecture.md` §11, "mover `board.json` bajo `.vector/cache/`") parte de
  una premisa incorrecta: `internal/board` ya sirve el board como **proyección pura en memoria**
  sobre SSE (`GET /api/board`), **nunca lo escribe a disco** (no hay `WriteFile`). No hay archivo
  que mover; el implementador no debe crear esa escritura. No se toca el paquete `board`.
- **`dep-graph.json` como artefacto** — es el anti-patrón (caro pero inestable); queda
  efímero/no-generado.
- Watchers de filesystem / invalidación en tiempo real.
- Daemon de conocimiento (proceso persistente).
- Detección de commands de CI (GitHub Actions, CircleCI, etc.) — solo manifests locales.
- Tocar invariantes: CLI-owns-writes, state machine, sharding per-spec, append-only activity.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Lenguaje: **Go 1.26** (módulo único `github.com/mariocampbell/vector` bajo `cli/`).
- Librerías: **stdlib únicamente** — sin dependencias externas. `crypto/sha256`, `encoding/json`,
  `os`, `path/filepath`, `os/exec` (para `git ls-files`), `sync`, `context`.
- Concurrencia: goroutines + `sync.WaitGroup` (patrón ya usado en `runContext` para glob +
  `DetectBuildCmds` concurrentes, y en la detección de manifests del spec `vector-context-cached-setup`).
- Serialización: JSON (todos los artefactos de cache son JSON; **no Markdown** — los produce y
  consume el binario, 0 tokens de modelo, validados por schema en Go).

### Versiones relevantes

- Go: `1.26` (según `cli/go.mod`).
- `config.SchemaVersion` actual: `1`. `board.SchemaVersion` actual: `2`. El cache introduce su
  **propio** `schemaVersion` independiente (TBD valor inicial — proponer `1`; ver Open questions).

No usar librerías, APIs, flags o patrones que no estén ya presentes en el proyecto, salvo que este
spec lo autorice explícitamente. No se agregan dependencias externas.

### Patrones existentes a respetar

- `cli/cmd/vector/main.go`: switch de subcomandos top-level en `main()`. `vector context` ya tiene
  su `case`; este spec **extiende** `runContext`, no añade un subcomando nuevo.
- Funciones `runXxx` en archivos propios bajo `cli/cmd/vector/` (`context.go`, `standup.go`,
  `ticket.go`). La lógica pesada vive en `internal/`, no en `cmd/`.
- `cli/internal/<concern>`: un paquete por concern (`board`, `config`, `state`, `scaffold`).
  Nuevo paquete `intel` sigue esa convención.
- **Escritura atómica**: persistir vía temp file + `rename`, como `config.Write`
  (`internal/config/config.go`). El binario es el único escritor.
- Errores explícitos envueltos con `fmt.Errorf("…: %w", err)`; nada de `panic` en flujo normal;
  mensajes de CLI claros y accionables (ver `go-conventions.md`).
- Tests con `testing` estándar, table-driven (ver `quality/testing-and-review.md`).
- `go generate ./internal/scaffold` re-vendoriza assets del kit; `TestAssetsMatchKit` guarda el
  drift. **Solo relevante si cambian archivos `.md` del kit** — esta fase no los cambia.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `vector context` (`cli/cmd/vector/context.go`): `runContext` + `ContextOutput`
  `{ExamplePath, Language, BuildCmd, LintCmd, TestCmd, ApplyMode, TicketDetected}`. Existe; este
  spec lo extiende. (Spec `vector-context-cached-setup`, parcialmente implementado.)
- [x] `internal/config.Config` con `Language`, `KitVersion`, `ResolvedBuildCmds()`,
  `DetectBuildCmds`, y la **función a nivel de paquete** `config.Path(repoRoot string) string`
  (`cli/internal/config/config.go:365`) → `<repoRoot>/.vector/config.json`. Binario único
  escritor.
- [x] `.gitignore` con `.vector/local/`, `.vector/tmp/`, `.claude/commands/vector/`.
- [x] Git disponible en runtime (para `git ls-files`); con fallback si el repo no es git (§11).
- [ ] No requiere cambios en el state machine, el paquete `board`, ni en `activity.jsonl`.

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta. No
debe inventar contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

**Productor/consumidor con oráculo de validez.** El binario es el productor (genera artefactos de
cache) y el consumidor (los lee tras validar su fingerprint). El cache es un detalle de
implementación interno: los agentes nunca lo leen directo (§8 reutilización por agentes).

### Capas afectadas

- presentation (CLI): **sí** — `cli/cmd/vector/context.go` (`runContext`) gana flags `--refresh`
  y `--for <command>`, y orquesta validación/regeneración.
- application/use-cases: **sí** — nuevo paquete `cli/internal/intel` con la lógica de fingerprint,
  generación de artefactos y gestión del cache.
- domain: **no** — no toca el state machine ni los tipos de `internal/state`.
- data/infrastructure: **sí** — lectura/escritura atómica de `.vector/cache/*.json`; ejecución de
  `git ls-files`.
- shared/common: **no** — sin paquetes catch-all.

### Flujo esperado (validación on-read de un command que consume contexto)

1. Un command invoca `vector context --for <command> [--json]`.
2. `runContext` resuelve `repoRoot` y carga `config` (como hoy).
3. `intel` determina los dominios que `<command>` consume (mapa estático).
4. Para cada dominio consumido: **recomputar su digest** desde el set autoritativo de fuentes
   (working-tree) y compararlo con el guardado en `fingerprints.json`.
5. Si coincide → reusar el artefacto cacheado del dominio. Si difiere (o falta/corrupto) →
   **regenerar ese dominio** (y, por el DAG, sus dependientes), reescribir su entrada en
   `fingerprints.json` y el artefacto afectado.
6. Proyectar el **slice** correspondiente a `<command>` y emitirlo como JSON.
7. `--refresh` salta el paso 4 y fuerza la regeneración de todos los dominios.

### Flujo de regeneración de un dominio

1. Enumerar el set autoritativo de fuentes del dominio (globs definidos en §7).
2. Hashear cada fuente con sha256 sobre su contenido del working-tree, en orden canónico
   (paths ordenados), concatenando para un digest del dominio.
3. Derivar el artefacto del dominio (p. ej. `structure` → `structure-index.json` vía
   `git ls-files` clasificado; `stack` → `repo-intel.json`).
4. Escritura atómica (temp + rename) del artefacto y de la entrada en `fingerprints.json`.

### Ubicación de archivos nuevos

```txt
cli/
  internal/intel/                 # NUEVO paquete (un concern: inteligencia de repo derivada)
    intel.go                      # tipos públicos + orquestación cache (Load/Validate/Refresh)
    fingerprint.go                # digest sha256 por dominio (working-tree), DAG de dominios
    repo_intel.go                 # generación de repo-intel.json (stack/framework/runtime)
    structure.go                  # generación de structure-index.json (git ls-files clasificado)
    intel_test.go                 # tests table-driven
  cmd/vector/
    context.go                    # MODIFICAR: --refresh, --for, validación on-read vía intel
```

No crear carpetas nuevas si ya existe una convención equivalente. Nombre del paquete: proponer
`intel` (permitir `cache` si el implementador lo justifica — ver Open questions).

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `cli/internal/intel/intel.go` | NUEVO | Tipos públicos del cache (Fingerprints, RepoIntel, StructureIndex), Load/Validate/Refresh, escritura atómica | `cli/internal/config/config.go` |
| `cli/internal/intel/fingerprint.go` | NUEVO | Digest sha256 por dominio sobre working-tree; sets autoritativos; DAG de dependencia | `cli/internal/config/config.go` (DetectBuildCmds) |
| `cli/internal/intel/repo_intel.go` | NUEVO | Genera `repo-intel.json` (stack/framework/runtime/tsconfig paths) | `cli/internal/config/config.go` |
| `cli/internal/intel/structure.go` | NUEVO | Genera `structure-index.json` vía `git ls-files` clasificado por workspace + entry points | — (TBD — ver Open questions) |
| `cli/internal/intel/intel_test.go` | NUEVO | Tests table-driven de toda la lógica | `cli/internal/config/config_test.go` |
| `cli/cmd/vector/context.go` | MODIFICAR | Flags `--refresh`, `--for`; validación on-read; proyección scoped; mapa command→dominios | (el propio archivo) |
| `.gitignore` | MODIFICAR | Añadir `.vector/cache/` | (entradas existentes `.vector/local/`, `.vector/tmp/`) |

### Detalle por archivo

#### cli/internal/intel/fingerprint.go

Acción: NUEVO

Debe implementar:

- `Domain` (enum/const string): `stack`, `deps`, `build`, `workspace`, `structure`.
- Para cada dominio, su **set autoritativo de fuentes** (globs, §7). Resolución de globs relativa
  a `repoRoot`, ignorando lo que `.gitignore` excluya cuando corresponda.
- `DigestDomain(repoRoot, domain) (string, error)`: sha256 sobre el **contenido del working-tree**
  de las fuentes, en orden canónico (paths ordenados lexicográficamente), prefijo `sha256:`.
  Para `structure`: digest de la salida de `git ls-files` (set de archivos rastreados) + conteo de
  untracked-no-ignored + SHA de submódulos (`git submodule status`, si hay).
- Hashing de los dominios solicitados **en paralelo** (goroutines + `sync.WaitGroup`).
- DAG de dependencia: `stack → deps`, `structure → (entry points)`. `InvalidatedBy(domain)`
  devuelve el cierre transitivo de dependientes a invalidar.

Debe seguir como referencia:

- `cli/internal/config/config.go` (concurrencia, escritura atómica, manejo de errores).

No debe incluir:

- Cualquier persistencia de git metadata mutable (branch/HEAD/dirty) ni tool availability.

#### cli/internal/intel/structure.go

Acción: NUEVO

Debe implementar:

- `BuildStructureIndex(repoRoot) (StructureIndex, error)`: ejecuta `git ls-files`, agrupa por
  workspace/dir top-level, clasifica entry points por lenguaje (heurística: `main.go`,
  `cmd/*/main.go`, `package.json#main`/`bin`, `index.ts`, etc. — clasificación exacta TBD por
  lenguaje, ver Open questions). Service boundaries: derivación **best-effort, no bloqueante**.
- Fallback sin git (§11): walk del filesystem filtrando `.gitignore`, o marcar `structure` como
  `unavailable` sin crashear.

Restricciones:

- No cargar el contenido de los archivos del repo en memoria; solo sus paths (el índice es de
  estructura, no de contenido).
- Documentar/manejar el límite de tamaño para repos enormes (§11, §17).

#### cli/cmd/vector/context.go

Acción: MODIFICAR

Cambios requeridos:

- Añadir flags `--refresh` (bool) y `--for <command>` (string) al `FlagSet` de `runContext`.
- Antes de emitir: invocar `intel` para validar los dominios consumidos por el command objetivo
  (o todos, si no se pasa `--for`), regenerando los caducados.
- Mapa estático `commandDomains` (en Go): p. ej. `raw → {stack, workspace}` + examplePath/language;
  `bug → {stack, workspace}`; `apply → {build, stack, deps}`; `comment → {build, stack, deps}`;
  commands TRUST → `{}` (sin validación). (Lista canónica de `docs/knowledge-architecture.md` §6.)
- Proyectar el slice del command y serializarlo; sin `--for`, devolver el `ContextOutput`
  extendido completo (backward-compat).
- `--for` con command desconocido → error accionable (proponer; ver Open questions / edge cases).

Restricciones:

- No cambiar la forma del `ContextOutput` existente para callers que no pasan flags nuevos
  (campos nuevos additive con `omitempty`).
- No llamar `config.Write` (sigue sin escribir config). Sí escribe `.vector/cache/` vía `intel`.

---

## 7. API Contract

No hay API HTTP. El "contrato" de esta feature son (a) los **schemas JSON** de los artefactos de
cache y (b) la **superficie de CLI** de `vector context`. Son la única fuente de verdad para la
implementación; no inferir campos adicionales.

### Superficie CLI

```txt
vector context [--json] [--repo-root <path>] [--refresh] [--for <command>] [--dry-run]
```

- `--refresh`: regenera todos los artefactos de cache y reescribe `fingerprints.json`. Exit 0.
- `--for <command>`: proyecta solo el slice de dominios que `<command>` consume.
- Sin flags nuevos: comportamiento actual (backward-compat).
- Exit 1 con mensaje accionable en stderr si falta/está malformado `.vector/config.json`
  (como hoy) o si `--for` recibe un command desconocido (TBD — ver Open questions).

### `fingerprints.json` (oráculo de validez)

```json
{
  "schemaVersion": 1,
  "kitVersion": "0.0.1-dev",
  "domains": {
    "stack":     { "digest": "sha256:…", "generatedAt": "2026-06-28T08:00:00Z" },
    "deps":      { "digest": "sha256:…", "generatedAt": "2026-06-28T08:00:00Z" },
    "build":     { "digest": "sha256:…", "generatedAt": "2026-06-28T08:00:00Z" },
    "workspace": { "digest": "sha256:…", "generatedAt": "2026-06-28T08:00:00Z" },
    "structure": { "digest": "sha256:…", "generatedAt": "2026-06-28T08:00:00Z" }
  }
}
```

### `repo-intel.json` (2–10 KB)

```json
{
  "schemaVersion": 1,
  "packageManager": "go-modules",
  "runtime": { "name": "go", "version": "1.26" },
  "frameworks": [],
  "tsconfigPaths": [],
  "generatedAt": "2026-06-28T08:00:00Z"
}
```

(Campos exactos de `frameworks`/`tsconfigPaths` por lenguaje: TBD — ver Open questions. Estructura
mínima estable arriba.)

### `structure-index.json` (10–200 KB)

```json
{
  "schemaVersion": 1,
  "workspaces": [
    { "path": "cli", "kind": "go-module", "entryPoints": ["cmd/vector/main.go"] },
    { "path": "web", "kind": "node",       "entryPoints": ["src/main.tsx"] }
  ],
  "generatedAt": "2026-06-28T08:00:00Z"
}
```

(Clasificación exacta de `kind` y heurística de `entryPoints` por lenguaje: TBD — ver Open
questions.)

### Salida de `vector context` (contrato consumido por los commands)

**Sin `--for` (backward-compat estricto):** la salida es el `ContextOutput` **actual sin
cambios** (`{ExamplePath, Language, BuildCmd, LintCmd, TestCmd, ApplyMode, TicketDetected}`), más
un campo **additivo opcional** `intel` (`omitempty`) con un resumen compacto del stack y los
workspaces. Los callers existentes que no leen `intel` no se ven afectados.

```json
{
  "examplePath": ".vector/specs/add-agent-prose-language/spec.md",
  "language": "es",
  "buildCmd": "go -C cli build ./...",
  "lintCmd": "",
  "testCmd": "go -C cli test ./...",
  "applyMode": "ask",
  "ticketDetected": false,
  "intel": {
    "stack": { "packageManager": "go-modules", "runtime": { "name": "go", "version": "1.26" }, "frameworks": [] },
    "workspaces": [ { "path": "cli", "kind": "go-module" }, { "path": "web", "kind": "node" } ]
  }
}
```

**Con `--for <command>` (proyección scoped):** devuelve un `ContextSlice` — solo el slice que el
command consume, según su tier y sus dominios. Forma:

```json
{
  "command": "raw",
  "tier": "lazy-validate",
  "validatedDomains": ["stack", "workspace"],
  "examplePath": ".vector/specs/…/spec.md",
  "language": "es",
  "stack": { "packageManager": "go-modules", "runtime": { "name": "go", "version": "1.26" }, "frameworks": [] }
}
```

Campos por **clase de tier** (la lista canónica command→dominios sale de
`docs/knowledge-architecture.md` §6; el cableado en los `.md` es la fase siguiente):

| Tier | Commands (ejemplos) | `validatedDomains` | Campos del `ContextSlice` |
|---|---|---|---|
| `trust` | `status`, `link`, `close`, `archive`, `standup`, `propose`, `sync` | `[]` | `{command, tier, validatedDomains: []}` — sin payload de repo |
| `lazy-validate` | `raw`, `bug` | `["stack","workspace"]` | `+ examplePath, language, stack` (resumen del stack) — el dominio `workspace` se valida para garantizar la frescura de `examplePath` y del layout, aunque no se proyecte un objeto `workspace` propio en el slice |
| `full-validate` | `apply`, `comment` | `["build","stack","deps"]` | `+ buildCmd, lintCmd, testCmd, stack` |

`stack` (el "stack summary" referido en §8) = el objeto compacto `{packageManager, runtime,
frameworks}` proyectado desde `repo-intel.json`. `--for` con command desconocido → error
accionable + exit 1 (§11).

### Sets autoritativos de fuentes del fingerprint (no el repo entero)

| Dominio | Fuentes (globs, working-tree) | Protege |
|---|---|---|
| `stack` | `**/package.json`, `go.mod`, `pyproject.toml`, `Cargo.toml`, `**/tsconfig*.json`, `next.config.*`, `vite.config.*` | techstack, framework, runtime |
| `deps` | `pnpm-lock.yaml`, `package-lock.json`, `yarn.lock`, `go.sum`, `Cargo.lock`, `poetry.lock` | dependency graph (efímero) |
| `build` | `Makefile`, scripts de `**/package.json`, `turbo.json` | build/lint/test/format commands |
| `workspace` | `pnpm-workspace.yaml`, `turbo.json`, `nx.json`, `go.work`, manifest raíz | mono/micro layout, shards |
| `structure` | digest de `git ls-files` + conteo untracked-no-ignored + SHA de submódulos | índice de árbol, entry points |

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `vector context --refresh` genera `.vector/cache/{fingerprints.json, repo-intel.json,
  structure-index.json}` y son JSON válidos por schema.
- [ ] Una segunda llamada a `vector context` sin cambios en el repo reusa el cache (digests
  coinciden) y **no regenera** (0 reescrituras de artefacto).
- [ ] Editar un manifest en el working-tree (sin commitear) invalida **solo** el/los dominios
  afectados, no todos.
- [ ] Bump del `schemaVersion` del cache (o `kitVersion`) invalida **todo** el cache.
- [ ] `vector context --for raw` devuelve solo el slice esperado (examplePath, language, stack
  summary); `--for apply` devuelve otro slice (build/test cmds, stack).
- [ ] El DAG funciona: invalidar `stack` invalida `deps`; invalidar `structure` invalida sus
  derivados (entry points).
- [ ] `.vector/cache/` está en `.gitignore`; nada del cache se committea.
- [ ] Backward-compat: `vector context --json` sin flags devuelve el `ContextOutput` actual.
- [ ] No hay errores de `go vet`; `gofmt -l .` vacío; tests table-driven verdes.

### Tests requeridos

Agregar tests (paquete `testing` estándar, table-driven) para:

- [ ] Digest determinista por dominio (misma entrada → mismo digest; orden canónico de paths).
- [ ] Invalidación on-read: mutar una fuente → mismatch → regenera ese dominio.
- [ ] Working-tree vs HEAD: edición sin commitear se detecta (no depende de HEAD).
- [ ] DAG de invalidación (stack→deps; structure→entry points).
- [ ] Bump de `schemaVersion`/`kitVersion` invalida todo.
- [ ] Proyección `--for <command>` (slices correctos por command; command desconocido).
- [ ] `--refresh` regenera todos los dominios.
- [ ] Edge cases (§11): repo sin git, sin submódulos, cache corrupto, repo sin manifests.
- [ ] Escritura atómica concurrente (dos generaciones simultáneas no corrompen el archivo).

### Comandos de verificación

Ejecutar desde la raíz del repo (estilo del proyecto, como el spec ejemplo):

```bash
gofmt -l cli
go -C cli vet ./...
go -C cli test ./...
go -C cli build ./...
```

Tras cambios: reinstalar el binario (`go install ./cmd/vector` o el script de la Memory) y correr
`vector update` en la raíz del repo. La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

No aplica — feature de binario CLI sin interfaz gráfica. La "UX" relevante es la del CLI y se
cubre en §7 (superficie de comandos) y §11 (mensajes de error accionables): errores claros en
stderr, exit codes correctos, salida JSON estable. No hay formularios, loading states ni
navegación.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas y el agente no debe cuestionarlas ni cambiarlas:

- **Fingerprint = content-hash sha256 por dominio sobre el working-tree** (no commit hash de HEAD
  — over/under-invalida; no mtime — un clone lo reescribe). Justificación en
  `docs/knowledge-architecture.md` §5.
- **Cache gitignored bajo `.vector/cache/`**; `config.json` (committed) sigue siendo el hogar de
  los hechos estables ratificables (build/lint/test cmds ya existentes). No duplicar esos campos
  en el cache.
- **Formato JSON** para todos los artefactos de cache (no Markdown).
- **Binario Go = único escritor** (CLI-owns-writes), **stdlib únicamente**.
- **`structure-index.json` completo incluido**; **`board.json` fuera** (proyección en memoria de
  `vector serve`, nunca a disco).
- **Cinco dominios fijos**: `stack`, `deps`, `build`, `workspace`, `structure`.
- **Proyección scoped vive en el binario** (mapa command→dominios en Go), **no** en los `.md`
  esta fase.
- **`dep-graph.json` no se genera** (anti-patrón: caro pero inestable y no-barato-de-validar).
- **Tiers por-command y granularidad por-shard** quedan para la fase siguiente.
- **`fingerprints.json`: `schemaVersion` al nivel raíz** (no por-dominio, como bosquejaba el
  design doc §5). Departure consciente: un bump de versión invalida todos los dominios a la vez,
  así que un `schemaVersion` por-dominio sería redundante. `kitVersion` también vive al nivel raíz.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, pero
no implementarla.

---

## 11. Edge cases

La implementación debe manejar explícitamente:

### Repo sin git

- `git ls-files` falla → degradar `structure` a best-effort (walk del filesystem filtrando
  `.gitignore`) **o** marcar el dominio `structure` como `unavailable`. No crashear.

### Sin submódulos

- `git submodule status` vacío → SHA de submódulos vacío; el dominio `structure` sigue válido.

### Manifest editado sin commitear

- El fingerprint sobre working-tree lo detecta → invalida el dominio correspondiente. (Es el caso
  que el commit-hash de HEAD **no** detectaría.)

### Cache ausente o corrupto

- `.vector/cache/` inexistente, o un `*.json` con JSON inválido → tratar como **cache miss** →
  regenerar el dominio/artefacto. No fallar el comando.

### Mismatch de versión

- `schemaVersion` (cache) o `kitVersion` distintos a los esperados → invalidar **todo** el cache
  y regenerar.

### Repo sin manifests

- Sin `package.json`/`go.mod`/etc. → `repo-intel.json` con stack vacío, **válido** (no error).

### `--for <command>` desconocido

- Comportamiento esperado: error accionable en stderr + exit 1 (proponer), o fallback al
  `ContextOutput` completo. Decisión final: TBD — ver Open questions.

### Concurrencia

- Dos `vector context` simultáneos regenerando cache → **escritura atómica** (temp + rename, como
  `config.Write`); el último gana sin corromper el archivo.

### Repo enorme

- `git ls-files` con 100k+ archivos → `structure-index.json` puede exceder 200 KB. Documentar el
  límite y/o aplicar truncación con nota explícita en el artefacto (no truncación silenciosa).
  Estrategia exacta: TBD — ver Open questions / §17.

---

## 12. Estados de UI requeridos

No aplica — sin interfaz gráfica. Los "estados" del comando son exit 0 (éxito, JSON emitido) y
exit 1 (error accionable en stderr), cubiertos en §7 y §11.

---

## 13. Validaciones

### Validaciones de cliente

No aplica — sin formularios ni input de UI. La validación relevante es la de **integridad del
cache** (schema JSON de los artefactos, match de digest), cubierta en §5 y §8.

### Validaciones de servidor

No aplica — sin servidor/API. La "validación" del dominio es la comparación de digest del
fingerprint (§5).

---

## 14. Seguridad y permisos

- No exponer secrets ni tokens: el cache **no** debe persistir contenido sensible. Hashea
  manifests/lockfiles (no secretos) y guarda solo digests + metadata de estructura (paths).
- **No persistir hechos machine-specific** (tool availability, paths absolutos) en artefactos que
  pudieran filtrarse: el cache es gitignored, pero aun así no se committea ni se comparte.
- **No auto-escribir `.claude/`** con convenciones detectadas (requiere ratificación humana —
  `security/destructive-ops-consent.md`).
- El cache es **solo del repo de Vector/del usuario local**; ninguna operación toca el repo del
  usuario fuera de leer sus manifests para hashear.

---

## 15. Observabilidad y logging

- Usar el mecanismo de errores del CLI existente (errores envueltos con `%w`, mensajes en stderr).
- Registrar (a nivel de error de CLI / `--dry-run` verbose, si se añade): dominio regenerado y por
  qué (mismatch / refresh / corrupto), fallo de `git ls-files`, JSON de cache corrupto.
- No registrar contenido de manifests ni paths sensibles de forma verbosa por defecto.
- No introducir un framework de logging nuevo (stdlib / patrón existente).

---

## 16. i18n / textos visibles

No aplica — los textos del CLI (mensajes de error, ayuda de flags) siguen el patrón del binario
(inglés para la superficie técnica del CLI, como el resto de subcomandos). No hay textos de
producto de cara a usuario final en otro idioma. La prosa de agentes (`config.Language`) no la
produce esta feature.

---

## 17. Performance

- **Validar debe ser barato**: hashear el set autoritativo (≈10–30 archivos chicos por dominio),
  no el repo entero. Hashing de dominios en paralelo (goroutines).
- No cargar contenido de archivos del repo en memoria para `structure` (solo paths de
  `git ls-files`).
- Reusar el cache cuando el digest coincide → 0 regeneración (el caso común).
- Repos enormes: acotar `structure-index.json` (límite/truncación con nota, §11). El índice no
  debe degradar `vector context` a segundos.
- No introducir watchers ni procesos persistentes (fuera de scope).

---

## 18. Restricciones

El agente no debe:

- Persistir como **durable**: dependency graph, API routes, git metadata (branch/HEAD/dirty) o
  tool availability. (Anti-patrón explícito — `docs/knowledge-architecture.md` §1.)
- Generar `dep-graph.json` (ni en init ni en context).
- Auto-escribir convenciones detectadas a `.claude/`.
- Commitear nada del cache (debe quedar gitignored).
- Tocar el state machine, el paquete `board`, `activity.jsonl`, ni el sharding per-spec.
- Cambiar la forma del `ContextOutput` existente para callers sin flags nuevos (backward-compat).
- Editar archivos `.md` del kit ni re-vendorizar assets (fuera de scope esta fase).
- Usar fingerprint basado en commit hash de HEAD o en mtime.
- Añadir dependencias externas (stdlib only).
- Inventar el schema exacto de entry-points/frameworks por lenguaje donde está marcado TBD —
  debe dejarlo como Open question o implementar una heurística mínima documentada.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] Paquete `cli/internal/intel` implementado (fingerprint por dominio, repo-intel,
  structure-index, gestión de cache con escritura atómica).
- [ ] `cli/cmd/vector/context.go` extendido (`--refresh`, `--for`, validación on-read, mapa
  command→dominios).
- [ ] `.gitignore` con `.vector/cache/`.
- [ ] Artefactos generados correctamente al correr `vector context --refresh`.
- [ ] Tests table-driven agregados y verdes.
- [ ] `go vet` / `gofmt` / `go test` / `go build` limpios.
- [ ] Binario reinstalado + `vector update` corrido en la raíz.
- [ ] Documentación: nota de paquete (doc comment) en `intel`; referencia cruzada a
  `docs/knowledge-architecture.md`.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo y `docs/knowledge-architecture.md`.
- [ ] Confirmé que `vector context` y `config.Config` existen como se describe.
- [ ] Solo modifiqué archivos listados en §6 o justifiqué cualquier excepción.
- [ ] Seguí los patrones reales del proyecto (escritura atómica, errores envueltos, un paquete por
  concern).
- [ ] El fingerprint es sha256 por dominio sobre working-tree (no HEAD, no mtime).
- [ ] Implementé los cinco dominios y el DAG de invalidación.
- [ ] Implementé `--refresh` y `--for <command>` con el mapa estático en Go.
- [ ] El cache es gitignored y no persiste git metadata / tool availability / dep-graph.
- [ ] Cubrí los edge cases de §11.
- [ ] No cambié decisiones tomadas (§10) ni toqué invariantes (§18).
- [ ] Ejecuté `gofmt`, `go vet`, `go test`, `go build`.
- [ ] Reinstalé el binario y corrí `vector update`.
- [ ] No dejé logs temporales ni TODOs sin justificar; los unknowns están como
  `TBD — ver Open questions`.

---

## Open questions

1. **Valor inicial del `schemaVersion` del cache** — proponer `1` (independiente de
   `config.SchemaVersion=1` y `board.SchemaVersion=2`). Confirmar.
2. **Nombre del paquete** — `cli/internal/intel` (propuesto) vs `cli/internal/cache`. `intel`
   describe el concern (inteligencia de repo); `cache` describe el mecanismo. Decisión del
   implementador.
3. **Heurística exacta de entry points por lenguaje** (Go: `cmd/*/main.go`; Node:
   `package.json#main`/`bin`, `src/index.*`, `src/main.*`; Python: `__main__.py`, `pyproject`
   scripts). Definir el set mínimo soportado en V1; el resto best-effort.
4. **Campos exactos de `repo-intel.json`** (`frameworks`, `tsconfigPaths`) y de `kind` en
   `structure-index.json` por lenguaje. Estructura mínima fijada en §7; el detalle se cierra al
   implementar.
5. **`--for <command>` desconocido**: ¿error accionable + exit 1, o fallback al `ContextOutput`
   completo? Propuesta: error accionable (falla rápido y claro).
6. **Repos enormes**: estrategia de límite para `structure-index.json` (truncación con nota vs
   streaming vs cap configurable). Propuesta V1: cap con nota explícita en el artefacto.
7. **Service boundaries** en `structure-index.json`: derivación best-effort no bloqueante en V1;
   ¿se incluye algún heurístico mínimo o se difiere por completo? Propuesta: diferir, dejar el
   campo opcional vacío.
