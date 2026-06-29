# Design — add-repo-intel-cache

## Decisiones clave (§10 del spec — no cuestionar)

- **Fingerprint = content-hash sha256 por dominio sobre el working-tree.** No commit-hash de HEAD
  (over/under-invalida y no detecta ediciones sin commitear), no mtime (un clone lo reescribe).
- **Cache gitignored bajo `.vector/cache/`** (clase C, regenerable). `config.json` (committed,
  clase B) sigue siendo el hogar de los hechos estables ratificables (build/lint/test cmds); no se
  duplican esos campos en el cache.
- **Formato JSON** para todos los artefactos (los produce/consume el binario, 0 tokens de modelo,
  validados por schema en Go). No Markdown.
- **Binario Go = único escritor** (CLI-owns-writes), **stdlib únicamente** (sin deps externas).
- **Cinco dominios fijos**: `stack`, `deps`, `build`, `workspace`, `structure`.
- **Proyección scoped en el binario** (mapa `command→dominios` en Go), no en los `.md` esta fase.
- **`structure-index.json` completo incluido**; **`board.json` fuera** (proyección en memoria de
  `vector serve`, nunca a disco — no hay archivo que mover).
- **`dep-graph.json` no se genera** (anti-patrón: caro pero inestable y no-barato-de-validar).
- **`fingerprints.json`: `schemaVersion` + `kitVersion` a nivel raíz** (no por-dominio): un bump
  invalida todos los dominios a la vez, así que un version por-dominio sería redundante.

## Patrón arquitectónico

**Productor/consumidor con oráculo de validez.** El binario produce los artefactos y los consume
tras validar su fingerprint. El cache es un detalle interno: los agentes nunca lo leen directo,
consumen el slice proyectado por `vector context`.

### Flujo de validación on-read (`vector context --for <command>`)

1. `runContext` resuelve `repoRoot` y carga `config` (como hoy).
2. `intel` determina los dominios que `<command>` consume (mapa estático).
3. Por cada dominio consumido: recomputar su digest desde el set autoritativo de fuentes
   (working-tree) y compararlo con `fingerprints.json`.
4. Coincide → reusar el artefacto cacheado. Difiere/falta/corrupto → regenerar ese dominio y (por
   el DAG) sus dependientes, reescribir su entrada en `fingerprints.json` + el artefacto afectado.
5. Proyectar el slice del command y emitirlo como JSON.
6. `--refresh` salta el paso 3 y regenera todos los dominios.

### Flujo de regeneración de un dominio

Enumerar el set autoritativo de fuentes (globs §7) → hashear cada fuente con sha256 sobre su
contenido del working-tree en orden canónico (paths ordenados, prefijo `sha256:`) → derivar el
artefacto (`structure` → `structure-index.json` vía `git ls-files`; `stack` → `repo-intel.json`)
→ escritura atómica (temp + rename, como `config.Write`) del artefacto y de la entrada del
fingerprint.

## Superficie

- **NUEVO `cli/internal/intel/`**:
  - `intel.go` — tipos públicos (`Fingerprints`, `RepoIntel`, `StructureIndex`), `Load`/`Validate`/
    `Refresh`, escritura atómica.
  - `fingerprint.go` — `Domain` const (`stack`/`deps`/`build`/`workspace`/`structure`), sets
    autoritativos, `DigestDomain(repoRoot, domain)`, hashing paralelo, DAG `InvalidatedBy(domain)`.
  - `repo_intel.go` — genera `repo-intel.json` (stack/framework/runtime/tsconfig paths).
  - `structure.go` — `BuildStructureIndex(repoRoot)` vía `git ls-files` clasificado + entry points;
    fallback sin git (walk filtrando `.gitignore`, o marcar `structure` como `unavailable`).
  - `intel_test.go` — tests table-driven.
- **MODIFICAR `cli/cmd/vector/context.go`**: flags `--refresh` / `--for`, validación on-read,
  proyección scoped, mapa estático `commandDomains`. No cambia la forma del `ContextOutput` para
  callers sin flags nuevos (campos additivos `omitempty`). No llama `config.Write`.
- **MODIFICAR `.gitignore`**: añadir `.vector/cache/`.

### Mapa command→dominios (estático en Go, canónico de `docs/knowledge-architecture.md` §6)

| Tier | Commands | `validatedDomains` |
|---|---|---|
| `trust` | status, link, close, archive, standup, propose, sync | `[]` |
| `lazy-validate` | raw, bug | `["stack","workspace"]` |
| `full-validate` | apply, comment | `["build","stack","deps"]` |

## Referencias del proyecto a seguir

- `cli/internal/config/config.go`: concurrencia (`DetectBuildCmds`), escritura atómica
  (`config.Write` = temp + rename), errores envueltos con `%w`, `config.Path(repoRoot)`.
- `cli/cmd/vector/context.go`: `runContext` + `ContextOutput` actual (a extender, no reemplazar).
- `cli/internal/config/config_test.go`: estilo table-driven.

## Open questions (del spec §Open questions — resolver al implementar)

1. `schemaVersion` inicial del cache → proponer `1`.
2. Nombre del paquete: `intel` (propuesto) vs `cache` — decisión del implementador.
3. Heurística de entry points por lenguaje (Go `cmd/*/main.go`; Node `package.json#main|bin`,
   `src/index.*`, `src/main.*`; Python `__main__.py`, pyproject scripts) — set mínimo V1.
4. Campos exactos de `repo-intel.json` (`frameworks`, `tsconfigPaths`) y `kind` por lenguaje.
5. `--for` desconocido → propuesta: error accionable + exit 1.
6. Repos enormes: cap de `structure-index.json` con nota explícita (no truncación silenciosa).
7. Service boundaries en `structure-index.json`: diferir, campo opcional vacío en V1.
