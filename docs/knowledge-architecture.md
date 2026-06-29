# Vector — Arquitectura de conocimiento persistente

> Diseño desde primeros principios de la capa de conocimiento persistente del framework de
> orquestación. Objetivo: minimizar inspección de repo, re-razonamiento y tokens repetidos
> **sin** permitir que un agente jamás dependa de información obsoleta que degrade la
> implementación. Extiende `docs/orchestration-review.md` (§3, §9, §11) y el spec
> `vector-context-cached-setup`. Fecha: 2026-06-28.

## 0. Reencuadre: no son dos ubicaciones, son tres clases de conocimiento

La premisa "`.claude` durable vs `.vector` generado" es incompleta frente al repo real. Hay
**tres clases** de conocimiento y el diseño consiste en mapearlas, no en repartir archivos
entre dos carpetas:

| Clase | Origen | ¿Committed? | ¿Quién la produce? | Hoy vive en |
|---|---|---|---|---|
| **A. Conocimiento autoral durable** | Humano lo escribe | ✅ sí | Dev | `.claude/rules/`, `CLAUDE.md` |
| **B. Estado de dominio transaccional** | Acción de dominio | ✅ sí (sharded) | Binario Go | `.vector/config.json`, `.vector/specs/<id>/` |
| **C. Inteligencia de repo derivada** | Inspección del repo | ❌ no (regenerable) | Binario Go | *(no existe aún; solo `language`)* |
| **D. Logs personales / derivados** | Ejecución | ❌ no | Binario Go | `.vector/local/`, `.vector/board.json` |

La pregunta del review ("qué cachear para no re-inspeccionar") es **exclusivamente la clase C**.
Hoy clase C no existe como artefacto: cada command la re-deriva en Opus (hallazgo #3 del
orchestration-review). Las clases A, B, D ya están diseñadas y **no se tocan**.

Regla de oro de ubicación:

- **`.claude/` = solo clase A.** Lo que un humano escribiría y otro humano leería y *confiaría*.
  **Nunca** inteligencia derivada: es regenerable, caduca, y commitearla crea divergencia
  entre "conocimiento committeado" y realidad (un riesgo explícito del brief).
- **`.vector/` = todo lo que posee el binario** (B committed + C/D gitignored). La clase C nueva
  va en `.vector/cache/` (gitignored), hermana de `local/` y `board.json`.

> El test discriminante: si la respuesta correcta a "¿está esto al día?" es *"pregúntale a un
> humano"* → clase A (`.claude/`). Si es *"recalcula un hash y compáralo"* → clase C
> (`.vector/cache/`). Si es *"es trivial, recalcúlalo siempre"* → no se persiste.

---

## 1. Deliverable 1 — Matriz de clasificación de conocimiento

Cada ítem evaluado contra el gate de 5 condiciones del brief (persistir solo si **todas** se
cumplen razonablemente): reusado · estable · barato de validar · caro de redescubrir · valioso
para varios agentes. "Caro de computar" **no** es criterio por sí solo.

| Ítem | Reuso | Estable | Validación barata | Caro redescubrir | Multi-agente | **Veredicto** |
|---|---|---|---|---|---|---|
| Package manager | alto | alto | sí (lockfile) | bajo | sí | **config.json** (B, ratificable) |
| Technology stack (frameworks primarios) | alto | alto | sí (hash manifests) | medio | sí | **cache `stack`** |
| Runtime (node/go/python + versión) | medio | alto | sí | bajo | sí | **config.json** |
| Workspace layout (mono/micro) | alto | alto | sí (hash workspace cfg) | medio | sí | **cache `workspace`** |
| Build / test / lint / format commands | alto | alto | sí (hash manifests) | medio | sí | **config.json** (ya en spec) |
| TS config (paths, strict) | medio | alto | sí (hash tsconfig*) | bajo | parcial | **cache `stack`** (por path) |
| Framework/runtime detection | alto | alto | sí | medio | sí | **cache `stack`** |
| Repository structure (índice de árbol) | alto | medio | sí (`git ls-files` digest) | medio | sí | **cache `structure`** |
| Entry points | medio | medio | parcial (deriva de structure) | medio | parcial | **cache `structure`** (dependiente) |
| Monorepo layout (shards de workspace) | alto | alto | sí | medio | sí | **cache `workspace`** |
| CI config | bajo | medio | sí | bajo | raro | **lazy** (leer on-demand, no cachear) |
| Docker config | bajo | medio | sí | bajo | raro | **lazy** |
| Code ownership (CODEOWNERS) | bajo | medio | sí | bajo | raro | **lazy** |
| Naming conventions | medio | alto | **no** (subjetivo) | medio | parcial | **`.claude/` si autoral**, si no no-persistir |
| Project conventions | alto | alto | **no** (subjetivo) | alto | sí | **`.claude/`** (A — humano ratifica) |
| Service boundaries / domain modules | medio | medio | parcial | alto | parcial | **cache `structure`** coarse; detalle lazy |
| Dependency graph | medio | **bajo** | **no** (hay que reconstruir para saber) | alto | parcial | **NO persistir durable** → efímero lazy |
| API routes | bajo-medio | **bajo** | **no** | alto | parcial | **NO persistir durable** → lazy |
| Git metadata (branch, HEAD, dirty) | alto | **muy bajo** | trivial | trivial | sí | **NUNCA persistir** — recalcular siempre (gratis) |
| Tool availability (`which X`) | alto | medio | sí | trivial | sí | **NUNCA committear** (machine-specific); recalcular o cache machine-local efímero |
| File inventory (deltas) | medio | bajo | sí | bajo | parcial | derivado de `structure`; no artefacto propio |

Tres patrones de rechazo a subrayar (el brief los pide explícitos):

1. **Caro pero inestable y caro de validar** → *anti-patrón*. Dependency graph y API routes son
   caros de redescubrir pero cambian seguido y **no hay forma barata de saber si caducaron sin
   reconstruirlos**. Persistirlos da falsa confianza. Se computan **lazy y efímeros**, jamás
   committed, idealmente con TTL corto y etiqueta `volatile`.
2. **Trivial de recalcular** → git metadata, file inventory. Cachearlos es complejidad neta
   negativa: el costo de invalidar supera el de recomputar. Se recalculan cada corrida (el
   binario lo hace en microsegundos, 0 tokens).
3. **Machine-specific** → tool availability, paths absolutos. Committearlos envenena a otros
   devs. Solo cache local no-committed o recálculo.

---

## 2. Deliverable 2 — Estructura recomendada de `.claude/`

`.claude/` aloja **solo clase A** (autoral, committed, humano-editable). En el repo de Vector
ya es correcto; la regla nueva es **prohibir** que cualquier proceso escriba inteligencia
derivada aquí.

```
.claude/
├── CLAUDE.md                 # manifest scoped (existente)
├── rules/                    # conocimiento por concern (existente)
│   ├── architecture/  standards/  product/  security/  quality/  workflows/
└── commands/vector/          # ⚠ EXCEPCIÓN: sembrado por el binario, GITIGNORED
```

- **Pertenece**: convenciones, estándares de código, arquitectura, conocimiento de dominio,
  guidelines, preferencias de equipo. Todo lo que es *input* humano al sistema.
- **Nunca pertenece**: techstack detectado, índice de estructura, build commands derivados,
  dependency graph, cualquier cosa que el binario pueda regenerar. Si un detector "quiere"
  escribir en `.claude/`, el diseño está mal: va a `.vector/cache/` y, si amerita ser durable,
  se **propone al humano** para que él la ratifique en una rule.
- **La excepción `commands/vector/`** ya es gitignored (sembrado, no autoral). No es clase A;
  es clase C "sembrada" — distribución, no conocimiento. Vive aquí por requerimiento del harness
  de Claude Code, no por ser durable. Bien como está.

Puente A↔C (convenciones auto-detectadas): el detector puede *sugerir* convenciones, pero la
escritura a `.claude/rules/` requiere **ratificación humana**. Auto-escribir convenciones
detectadas viola `security/destructive-ops-consent.md` y crea divergencia silenciosa.

---

## 3. Deliverable 3 — Estructura recomendada de `.vector/`

`.vector/` aloja todo lo que posee el binario. Se separa por **vida** y **git**, no por tema:

```
.vector/
├── config.json               # B — COMMITTED. Pequeño, estable, ratificable por humano.
│                             #     {specPath, language, packageManager, runtime,
│                             #      buildCmd, lintCmd, testCmd, formatCmd, applyMode, kitVersion}
├── specs/<id>/               # B — COMMITTED, sharded (1 archivo/spec → conflictos locales)
│   ├── state.json            #     fuente de verdad del card
│   └── spec.md
├── cache/                    # C — GITIGNORED. Inteligencia derivada, regenerable. ◀ NUEVO
│   ├── fingerprints.json     #     oráculo de validez (digests por dominio)
│   ├── repo-intel.json       #     stack, framework, tsconfig paths, runtime detail
│   ├── structure-index.json  #     árbol indexado (git ls-files clasificado por workspace)
│   └── board.json            #     ← mover aquí el board derivado (hoy en .vector/board.json)
├── local/                    # D — GITIGNORED. Personal append-only.
│   ├── activity.jsonl   summaries.json   standup.json
└── tmp/                      # D — GITIGNORED. Scratch (spec.md compuesto antes de registrar)
```

Decisiones:

- **`config.json` committed** sigue siendo el hogar de los pocos hechos **estables y
  compartibles** (clase B). El spec `vector-context-cached-setup` acierta poniendo
  build/lint/test ahí: son idénticos para todo el equipo, ratificables, y se commitean una vez.
  Se les añade un **guard de fingerprint** (§5) para detectar cuando quedan obsoletos.
- **`cache/` gitignored** es la clase C nueva: lo auto-derivado **volátil o grande**
  (índice de estructura, intel extendida, fingerprints). Nunca committed → 0 conflictos de
  merge, 0 divergencia, 0 bloat en git.
- **Consolidar `board.json` bajo `cache/`**: hoy es `.vector/board.json` gitignored derivado —
  misma clase C. Moverlo unifica "todo lo derivado y regenerable bajo un techo".
- **`.gitignore`**: añadir `.vector/cache/` junto a los ya presentes `.vector/local/`,
  `.vector/tmp/`.

Por qué `config.json` (committed) y `cache/` (gitignored) coexisten sin contradicción: el
criterio no es "derivado vs autoral", es **estable+compartible+ratificable** (→ committed) vs
**volátil+grande+máquina** (→ gitignored). Un `buildCmd` es derivado pero estable y compartible;
un `structure-index.json` es derivado, grande y churns con cada archivo nuevo.

---

## 4. Deliverable 6 — Inventario de artefactos generados

Formato: **JSON, no Markdown.** El brief pide no asumir Markdown — correcto: estos artefactos
los **produce y consume el binario** (0 tokens de modelo para leerlos), se validan por schema en
Go, y solo una *proyección* mínima cruza al modelo. Markdown es para clase A (humano). YAML no
aporta sobre JSON aquí (el binario ya serializa JSON nativo, sin deps).

| Artefacto | Propósito | Productor | Consumidores | Regenera cuando | Invalidación | Tamaño | Vida |
|---|---|---|---|---|---|---|---|
| `config.json` | Hechos estables ratificables | `vector init`/`update` | `vector context`, todos los commands | init/update, edición humana | fingerprint `stack`/`build` mismatch (warn) | <2 KB | hasta cambio de manifests |
| `fingerprints.json` | Oráculo de validez por dominio | binario | `vector context` (validador) | toda regeneración de cache | n/a (es el oráculo) | <2 KB | permanente, se sobrescribe |
| `repo-intel.json` | Stack, framework, tsconfig paths, runtime detail | `vector init`/`update`/`context --refresh` | refiner, composer, validator (vía proyección) | init/update, mismatch dominio `stack` | hash de manifests+lockfiles | 2–10 KB | hasta cambio fingerprint `stack` |
| `structure-index.json` | Árbol indexado por workspace, entry points | binario (`git ls-files` + clasificación) | composer/validator (leen slice por path) | mismatch dominio `structure` | digest de `git ls-files` + untracked-no-ignored | 10–200 KB | hasta cambio del set de archivos |
| `board.json` | Proyección read-only del board | `vector serve` | web (SSE) | cambio en `specs/` | fingerprint de `.vector/specs/` | 5–50 KB | por sesión de serve |
| `dep-graph.json` *(efímero, opcional)* | Relaciones de dependencia para apply/comment | binario, **lazy on-demand** | apply/comment | solo si un command lo pide | TTL corto + lockfile hash; **puede no existir** | variable | volátil, no garantizado |

`dep-graph.json` se lista aparte a propósito: **no se genera en init**, solo cuando un command
de código lo necesita, y se marca `volatile`. Es el caso que el brief advierte: caro pero
inestable → nunca durable.

---

## 5. Deliverable 5 — Estrategia de fingerprint del repositorio

**Recomendación: hash de contenido (sha256) sobre un set autoritativo de archivos, agrupado por
dominio.** No timestamps, no commit hash global.

Por qué se descartan las alternativas:

| Enfoque | Problema | Veredicto |
|---|---|---|
| Git commit hash (HEAD) | Invalida con *cualquier* commit (doc-only incluido) → over-invalidación que mata el cache; y **no** ve ediciones de manifest sin commitear → under-invalidación | ❌ |
| mtime / timestamps | Git no preserva mtime: un `checkout`/`clone` reescribe todos → falsa invalidación masiva | ❌ |
| Directory hash completo | Caro de computar (hashea todo el repo); churns por cualquier archivo irrelevante | ❌ |
| **Content hash de set autoritativo, por dominio** | Determinista, sobrevive a clone, barato (hashea ~10–30 archivos chicos), granular | ✅ |

### Set autoritativo (las fuentes del fingerprint)

Solo los archivos que **determinan** los hechos cacheados, no el repo entero:

- **Manifests**: `package.json` (todos), `go.mod`, `pyproject.toml`/`Cargo.toml`, etc.
- **Lockfiles**: `pnpm-lock.yaml`, `package-lock.json`, `yarn.lock`, `go.sum`, `Cargo.lock`, `poetry.lock`.
- **Workspace**: `pnpm-workspace.yaml`, `turbo.json`, `nx.json`, `go.work`.
- **Toolchain**: `tsconfig*.json`, `.eslintrc*`, `.prettierrc*`, `Makefile`.
- **Framework**: `next.config.*`, `vite.config.*`, etc.
- **Structure**: digest de `git ls-files` (set de archivos rastreados) + conteo de untracked-no-ignored + SHA de submódulos.

### Fingerprint **por dominio** (no uno global)

Esto habilita la invalidación parcial/lazy (§7–8):

| Dominio | Fuentes del hash | Qué protege |
|---|---|---|
| `stack` | manifests + tsconfig + framework configs | techstack, framework, runtime |
| `deps` | solo lockfiles | dependency graph (efímero) |
| `build` | Makefile, scripts de package.json, turbo.json | build/lint/test/format commands |
| `workspace` | workspace configs + manifest raíz | mono/micro layout, shards |
| `structure` | `git ls-files` digest + untracked + submódulos | índice de árbol, entry points |

`fingerprints.json` guarda `{ domain: { digest, generatedAt, schemaVersion }, kitVersion }`.

### Reglas de invalidación

1. Al **leer** cache de un dominio: recomputar su digest (hash de N archivos chicos, ~ms) y
   comparar. Mismatch → ese dominio caducó.
2. **Fingerprint sobre working-tree, no sobre HEAD** → captura ediciones sin commitear de
   manifests (riesgo "cambios ocultos del repo").
3. **DAG de dependencia entre dominios**: `structure → entry points → service boundaries`;
   `stack → deps`. Invalidar un dominio invalida sus dependientes (evita el bug de invalidación
   parcial).
4. **`schemaVersion` y `kitVersion`**: bump del esquema o upgrade del kit invalida *todo* el
   cache (el formato pudo cambiar). Guard barato y total.

---

## 6. Deliverable 7 — Árbol de decisión de verificación

No todo command valida todo. Cada command **declara los dominios que consume** y el **tier de
validación** que exige. El binario (`vector context`) valida solo esa intersección.

```
Comando entra
│
├─ ¿Depende de hechos del repo?
│   └─ NO  → TRUST. Sin fingerprint, sin recálculo.
│           (status, link, close, archive, standup, propose, sync, docs-only)
│
└─ SÍ → ¿El command muta/ejecuta CÓDIGO del repo? (build/test/run)
        │
        ├─ NO (solo genera prosa/spec)  → LAZY-VALIDATE
        │     valida solo dominios consumidos: {stack, workspace, examplePath}
        │     (raw, bug)
        │     mismatch → REBUILD del dominio caduco; el resto se reusa
        │
        └─ SÍ → FULL-VALIDATE
              valida {build, stack, deps} del/los workspace-shard tocados;
              recomputa volátiles (entry points, dep-graph) LAZY si se piden
              (apply, comment)
              mismatch → REBUILD del dominio; ejecutar gate (lint/test) con cmds frescos

REBUILD explícito: `vector init` | `vector update` | `vector context --refresh`
```

Modelo de confianza (resumen): `confianza(command) = f(clase_de_command × frescura_dominio)`.
Un command read-only sobre estado (TRUST) nunca paga validación. Un command que escribe código
(FULL-VALIDATE) nunca confía en cache sin verificar los dominios que tocará. El tier es
**propiedad declarada del command**, no decisión del modelo en runtime.

---

## 7. Deliverable 8 — Estrategia de validación lazy

Granularidad óptima = **(dominio × shard de workspace)**.

- **Por dominio**: un cambio en `package.json` invalida `stack`+`deps`, pero **no** `structure`
  (si no se añadieron archivos) ni `build` (si los scripts no cambiaron). Un cambio en
  documentación no invalida nada.
- **Por shard de workspace** (monorepo): un cambio en `apps/web/package.json` invalida el shard
  `web` de `stack`, **no** el shard `api`. Un request de code-gen de frontend valida
  `stack`/`build` del shard `web` y nada del backend — exactamente lo que pide el brief
  ("un cambio en frontend no invalida el análisis de backend").
- **Lazy de verdad**: la validación de un dominio ocurre **solo si un command lo consume**. Si
  nadie pide `deps` en esta corrida, su fingerprint ni se computa. Un request de docs no toca
  ningún dominio → cero validación, cero recálculo.

Implementación: `fingerprints.json` indexa por `domain` y, en monorepo, por `domain/shard`.
`vector context --for <command>` conoce el mapa command→dominios y valida la intersección, nada
más.

---

## 8. Deliverable — Reutilización por agentes (cómo consumen el conocimiento)

**Recomendación: los agentes nunca leen los archivos de cache directamente. El binario proyecta
un *slice* relevante; los agentes reciben paths + contexto estructurado mínimo.** Alinea con la
regla "transferir por referencia, no por contenido" del orchestration-review (§4) y con el
principio 0-tokens.

Tres capas de acceso:

1. **Binario lee/valida/regenera** el cache (0 tokens de modelo — lo hace Go).
2. **`vector context --json --for <command>`** proyecta **solo** el slice que el command
   necesita (proyección *scoped*): `--for apply` → build/test cmds + shard tocado; `--for raw`
   → examplePath + language + resumen de stack. No el cache completo.
3. **El dispatcher pasa ese JSON chico + paths** a los subagentes. Cada subagente hace `Read`
   **lazy** solo de los paths puntuales que requiere (el composer lee *un* example spec, no el
   `structure-index.json` entero).

Descartado:

- *Leer archivos directamente* → desperdicia tokens (carga el índice completo) y salta la
  validación de frescura.
- *Servicio de conocimiento (daemon)* → overkill; el binario es un CLI stateless, sin proceso
  persistente. `vector serve` es solo para la web.

Resultado: solo el slice mínimo cruza al modelo (minimiza tokens) y el binario garantiza que ese
slice está fresco (maximiza correctitud).

---

## 9. Deliverable — Conocimiento mutable vs inmutable

| Categoría | Ejemplos | Estrategia de almacenamiento |
|---|---|---|
| **Casi-inmutable** (cache largo, validación barata) | package manager, framework primario, language, workspace type, build/lint/test cmds, runtime | `config.json` **committed**, ratificable, guard de fingerprint. Cambian raro; al cambiar → `vector update` |
| **Lento** (cache, fingerprint por dominio) | índice de estructura, tsconfig paths, entry points, service boundaries | `.vector/cache/` **gitignored**, fingerprint por dominio + DAG |
| **Rápido** (nunca durable) | git branch/HEAD/dirty, dependency graph, file deltas, tool availability | **Recalcular por corrida** (git es gratis) o efímero con TTL. **Nunca committed.** Tool availability nunca *committed* (machine-specific) |

El error a evitar: tratar lo rápido como lento (cachearlo → invalidación constante, falsa
confianza) o lo lento como rápido (re-derivar siempre → el problema actual, hallazgo #3).

---

## 10. Deliverable 9 — Evaluación de riesgos

| Riesgo | Cómo se manifiesta | Mitigación |
|---|---|---|
| **Cache obsoleto → impl. incorrecta** | Agente usa build cmd viejo, stack viejo | Fingerprint de contenido por dominio validado en los dominios consumidos; tier FULL-VALIDATE para commands que tocan código; bump de `schemaVersion`/`kitVersion` invalida todo |
| **Over-caching** | Cachear lo que churns (dep graph) → invalidación perpetua, falsa confianza | Gate de 5 condiciones; **excluir explícitamente** dep-graph / git-meta / tool-availability del cache durable |
| **Under-caching** | Re-derivar cada corrida (estado actual) | Cachear el set estable de alto valor; un solo `vector context` |
| **Divergencia committed↔realidad** | Auto-derivado committed que driftea (en `.claude/` o `config.json`) | Solo hechos **estables+ratificables** committed; todo auto+volátil gitignored; fingerprint *surfacea* el drift; **nunca** auto-escribir `.claude/` |
| **Invalidación parcial** | Dominio A invalidado, B depende de A pero no se refresca | DAG de dependencia de dominios; invalidar A invalida sus dependientes |
| **Cambios ocultos del repo** | Ediciones sin commitear, untracked, submódulos | Fingerprint sobre **working-tree** (no HEAD); `structure` incluye untracked-no-ignored + SHA de submódulos |
| **Poisoning machine-specific** | Committear tool availability / paths absolutos | Nunca committear hechos de máquina; cache local o recálculo |
| **Conflictos de merge en cache** | Varios devs tocan el board/índice | Todo lo derivado **gitignored**; capa committed es sharded por-spec (ya localizado) + `config.json` diminuto |

---

## 11. Deliverable 10 — Roadmap priorizado

Alineado con la rama `feat/orchestration-review-p0-p1` y el spec `vector-context-cached-setup`
ya en vuelo.

**P0 — base (en vuelo, spec existente):**
1. `vector context --json` retorna setup estable desde `config.json` (`examplePath`, `language`,
   `buildCmd`, `lintCmd`, `testCmd`, `applyMode`). Detección paralela de manifests en `init`/
   `update`. *(= spec `vector-context-cached-setup`.)*

**P1 — validez (cierra el gap del spec: hoy invalida solo por `vector update` manual):**
2. `fingerprints.json` + validación por content-hash **por dominio** dentro de `vector context`;
   `--refresh` al detectar mismatch. Guard de `schemaVersion`/`kitVersion`.
3. Mover `board.json` bajo `.vector/cache/`; añadir `.vector/cache/` al `.gitignore`.

**P2 — proyección e índice:**
4. `repo-intel.json` + `structure-index.json` como artefactos generados (clase C).
5. Proyección *scoped*: `vector context --for <command>` devuelve solo el slice del command.

**P3 — lazy fino:**
6. `dep-graph.json` / entry points **efímeros on-demand** para `apply`/`comment` (volatile, TTL).
7. Granularidad por **(dominio × shard)** en monorepos; DAG de dependencia de dominios.

**Fuera de scope (no tocar / no hacer):**
- Cachear git metadata, tool availability o dependency graph como **durable**.
- Auto-escribir convenciones detectadas a `.claude/` (requiere ratificación humana).
- Watchers de filesystem / invalidación en tiempo real (re-`vector update` basta).
- Daemon de conocimiento (el binario stateless + `vector context` cubre el caso).
- Tocar los invariantes B: CLI-owns-writes, state machine, sharding per-spec.

---

## Nota de cierre

La palanca no es "cachear lo caro" sino **separar tres clases de conocimiento mal mezcladas** y
cachear únicamente la intersección del gate de 5 condiciones: hechos estables, baratos de
validar, caros de redescubrir, reusados y multi-agente. El techstack, los build commands y el
índice de estructura la cumplen; el dependency graph, el git metadata y la tool availability
**no** — y persistirlos sería el error que el brief advierte. El fingerprint de contenido por
dominio sobre el working-tree es lo que permite que un agente nunca dependa de información
obsoleta sin pagar re-inspección en cada corrida: validar es hashear 10–30 archivos chicos,
redescubrir desde cero es lo que hoy corre en Opus.
