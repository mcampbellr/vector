# Tasks — add-repo-intel-cache

## 1. Paquete `cli/internal/intel` — fingerprint

- [x] 1.1 `Domain` const (`stack`, `deps`, `build`, `workspace`, `structure`) + set autoritativo de
  fuentes por dominio (globs §7), resolución relativa a `repoRoot`.
- [x] 1.2 `DigestDomain(repoRoot, domain) (string, error)`: sha256 sobre el contenido del
  working-tree en orden canónico (paths ordenados), prefijo `sha256:`. `structure` = digest de
  `git ls-files` + conteo untracked-no-ignored + SHA de submódulos.
- [x] 1.3 Hashing de dominios en paralelo (goroutines + `sync.WaitGroup`).
- [x] 1.4 DAG: `stack → deps`, `structure → entry points`; `InvalidatedBy(domain)` = cierre
  transitivo de dependientes.

## 2. Paquete `cli/internal/intel` — artefactos

- [x] 2.1 `repo_intel.go`: genera `repo-intel.json` (packageManager, runtime, frameworks,
  tsconfigPaths) — estructura mínima §7.
- [x] 2.2 `structure.go`: `BuildStructureIndex(repoRoot)` vía `git ls-files` clasificado por
  workspace + entry points (heurística mínima por lenguaje); solo paths, no contenido.
- [x] 2.3 Fallback sin git: walk filtrando `.gitignore` o marcar `structure` como `unavailable`,
  sin crashear. Acotar `structure-index.json` en repos enormes (cap con nota explícita).
- [x] 2.4 `intel.go`: tipos públicos (`Fingerprints`, `RepoIntel`, `StructureIndex`),
  `Load`/`Validate`/`Refresh`, escritura atómica (temp + rename, como `config.Write`).

## 3. `cli/cmd/vector/context.go`

- [x] 3.1 Flags `--refresh` (bool) y `--for <command>` (string) en el `FlagSet` de `runContext`.
- [x] 3.2 Validación on-read vía `intel` de los dominios consumidos (o todos sin `--for`),
  regenerando los caducados.
- [x] 3.3 Mapa estático `commandDomains` en Go (tabla de tiers, §7); proyección del `ContextSlice`
  por command; sin `--for` devolver `ContextOutput` extendido (campo `intel` additivo `omitempty`).
- [x] 3.4 `--for` con command desconocido → error accionable + exit 1.

## 4. Infra

- [x] 4.1 Añadir `.vector/cache/` a `.gitignore`.

## 5. Tests (table-driven, `testing` estándar)

- [x] 5.1 Digest determinista por dominio (orden canónico de paths).
- [x] 5.2 Invalidación on-read: mutar una fuente → mismatch → regenera ese dominio.
- [x] 5.3 Working-tree vs HEAD: edición sin commitear se detecta.
- [x] 5.4 DAG de invalidación (stack→deps; structure→entry points).
- [x] 5.5 Bump de `schemaVersion`/`kitVersion` invalida todo.
- [x] 5.6 Proyección `--for` (slices por command; command desconocido) y `--refresh`.
- [x] 5.7 Edge cases: repo sin git, sin submódulos, cache corrupto, repo sin manifests, escritura
  atómica concurrente.

## 6. Verificación

- [x] 6.1 `gofmt -l cli` vacío, `go -C cli vet ./...`, `go -C cli test ./...`, `go -C cli build ./...` verdes.
- [x] 6.2 `vector context --refresh` genera los tres artefactos JSON válidos; segunda llamada reusa
  (0 reescrituras); backward-compat de `vector context --json` sin flags.
- [x] 6.3 Reinstalar el binario + `vector update` en la raíz del repo.
- [x] 6.4 Doc comment de paquete en `intel` + referencia cruzada a `docs/knowledge-architecture.md`.
