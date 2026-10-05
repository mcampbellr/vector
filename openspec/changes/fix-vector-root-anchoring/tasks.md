# Tasks — fix-vector-root-anchoring

## 1. Walk-up en `internal/config`

- [x] 1.1 Implementar `FindAncestorConfig(startDir string) (root string, strayDirs []string, found
      bool)` en `cli/internal/config/config.go`: normaliza `startDir` a absoluto; en cada nivel
      (empezando por `startDir`) comprueba `<dir>/.vector/config.json`; válido (deserializa sin
      error) → `(dir, strayDirs, true)`; `.vector/` sin `config.json` o con uno corrupto → añade
      `dir` a `strayDirs` y sigue subiendo; se detiene en la raíz del filesystem
      (`dir == filepath.Dir(dir)`) retornando `("", strayDirs, false)`.
- [x] 1.2 Solo lectura: sin escrituras ni efectos secundarios; sin decidir nada sobre límites de
      git/worktree (Open question #2).
- [x] 1.3 No cambiar firma ni comportamiento de `Resolve`/`Load`/`Exists`/`Write`.
- [x] 1.4 Tests table-driven en `cli/internal/config/config_test.go`: ancestro encontrado; stray
      saltado (sigue subiendo); múltiples strays en el camino; sin ancestro (detiene en la raíz del
      filesystem); `config.json` inválido en el ancestro (tratado como stray, según Open
      question #1).

## 2. `resolveRepoRoot` con walk-up

- [x] 2.1 En `cli/cmd/vector/main.go:1018`: si `explicit != ""`, sin cambios (walk-up salteado).
- [x] 2.2 Si `explicit == ""`, llamar `config.FindAncestorConfig(cwd)` **antes** de
      `git rev-parse --show-toplevel`; si `found`, retornar ese root.
- [x] 2.3 Exponer `strayDirs` al caller (valor de retorno adicional / campo de resultado) — la
      función **no** imprime; el warning es responsabilidad de la rama humana de cada comando.
- [x] 2.4 Sin ancestro → fallback actual (`git rev-parse --show-toplevel` → `os.Getwd()`) sin
      cambios.
- [x] 2.5 Tests en `cli/cmd/vector/root_resolution_test.go` (NUEVO): walk-up desde un subdirectorio
      anidado varios niveles; `--repo-root` explícito sigue final; sin ancestro cae al fallback.

## 3. Guard de ancestro en `vector init`

- [x] 3.1 Extraer la resolución actual sin walk-up (explícito → git toplevel → cwd) a una función
      separada (p. ej. `rawTargetRoot`) y usarla en `newInitCmd` (`main.go:65`) en lugar del nuevo
      `resolveRepoRoot`.
- [x] 3.2 Antes del bloque `cfgExisted && !force` (L97-102):
      `ancestorRoot, _, found := config.FindAncestorConfig(filepath.Dir(target))`; si
      `found && !force`, retornar `fmt.Errorf("an ancestor Vector store already exists at %s;
      refusing to create a nested store at %s (use --force to override)", ancestorRoot, target)`
      sin escribir nada.
- [x] 3.3 Con `--force`, continuar el flujo actual sin cambios.
- [x] 3.4 El guard es **exclusivo de `init`**: `vector update` y el resto de comandos no lo
      replican.
- [x] 3.5 Tests en `root_resolution_test.go`: error + mensaje nombrando el ancestro sin `--force`;
      `--force` crea el store anidado; sin ancestro por encima, `init` funciona sin cambios (primera
      inicialización); `init` en la propia raíz canónica **no** dispara el guard.

## 4. Comando `vector doctor`

- [x] 4.1 `cli/cmd/vector/doctor.go` (NUEVO): `newDoctorCmd()`, `Use: "doctor"`, sin args → escanea
      recursivamente por debajo de la raíz canónica buscando `.vector/` sin `config.json`; imprime
      `ui.Table` en rama humana / array en `--json`; sin mutaciones; lista vacía → exit 0.
- [x] 4.2 Subcomando `adopt <ruta-stray>`: sin `--force`, imprime el plan (specs a mover, entradas
      de `activity.jsonl` a anexar, estado local) sin tocar el filesystem.
- [x] 4.3 Con `--force`: mover los directorios de specs al mismo layout relativo bajo
      `<canónico>/.vector/specs/`, anexar cronológicamente por timestamp las líneas de
      `activity.jsonl`, mover el estado local restante, y borrar el `.vector/` stray **solo** tras
      migración exitosa (último paso).
- [x] 4.4 Validar la ruta: si no es un `.vector/` stray (no existe, o tiene `config.json`) → error
      claro `"<ruta>" is not a stray .vector/ directory (missing or has config.json)`, sin mutar.
- [x] 4.5 Conflicto de slug ya existente en el canónico → reportar y abortar esa migración puntual;
      nunca sobrescribir en silencio; el resto de specs sí migran (Open question #5).
- [x] 4.6 Sin prompt interactivo (el CLI es no interactivo); el consentimiento es `--force`.
- [x] 4.7 Registrar `newDoctorCmd()` en `cli/cmd/vector/root.go` (L61-72), sin reordenar ni quitar
      comandos existentes.
- [x] 4.8 Tests en `cli/cmd/vector/doctor_test.go` (NUEVO), siguiendo `config_test.go`
      (`t.TempDir()` + `captureStdout`): scan sin mutar; `adopt` sin `--force` (dry-run, sin
      mutación); `adopt --force` (mueve specs + activity + estado local, borra el stray); fallo a
      mitad → stray no borrado.

## 5. Warnings de stray

- [x] 5.1 `ui.Warning` nombrando cada stray saltado **y** la raíz canónica usada, solo en la rama
      humana de cada comando — nunca dentro de un branch `--json`. Implementado como helper
      compartido `warnStrayStores` y cableado en los puntos humanos donde un stray es accionable
      (`update`, `spec create`, `serve`, `doctor`), no en las 16 llamadas a `resolveRepoRoot`:
      repetirlo en cada transición de spec sería ruido sin señal nueva. Los demás comandos siguen
      usando `resolveRepoRoot`, que descarta los strays.
- [x] 5.2 Verificar que ningún spec/actividad se lee ni escribe en un stray.

## 6. Guardrail documentado en `kit/`

- [x] 6.1 `kit/agents/_shared/root-anchoring-guardrail.md` (NUEVO), siguiendo el formato de
      `citation-discipline.md` / `prose-rules.md`: si existe un `.vector/` ancestro, es la base
      única; nunca crear otro `.vector/` en un subdirectorio; `vector doctor` es la vía para
      consolidar strays.
- [x] 6.2 `kit/commands/vector/raw.md` — paso 2 ("Confirm the repo is initialized"): referenciar el
      guardrail antes de invocar `vector init`.
- [x] 6.3 `kit/commands/vector/bug.md` — nota equivalente donde referencia `vector init`/creación
      de specs.
- [x] 6.4 `kit/commands/vector/quick.md` — nota equivalente en el bloque introductorio.
- [x] 6.5 `kit/CLAUDE.md` — nota breve sobre el invariante de raíz única y el guard de `init`, en la
      sección "Contenido".
- [x] 6.6 Propagación single-source si aplica: `go generate ./internal/scaffold` → assets
      regenerados; `TestAssetsMatchKit` verde (nunca editar `cli/internal/scaffold/assets/**` a
      mano).

## 7. Regresión y gate

- [x] 7.1 Test end-to-end: workspace temporal con config en la raíz + un stray anidado
      preexistente; `vector spec create` desde la raíz y desde el subdirectorio del stray aterrizan
      ambos en el store raíz, sin tocar el stray.
- [x] 7.2 Suite golden (`cli/cmd/vector/golden_test.go`) byte-idéntica para los shapes `--json`
      existentes; añadir caso golden para `vector doctor --json` si el proyecto lo requiere.
- [x] 7.3 Gate verde:
      ```bash
      go -C cli generate ./...
      gofmt -l cli
      go -C cli vet ./...
      go -C cli test ./...
      go -C cli build ./cmd/vector
      ```
- [ ] 7.4 (PENDIENTE, post-merge) Reinstalar el binario (`~/.local/bin/vector`) y correr `vector update` en la raíz del
      repo tras cambios en `cli/`/`kit/` (dogfooding usa el binario del PATH).

## 8. Open questions registradas

- [x] 8.1 No inventar respuestas: dejar registradas en `design.md` la definición de "config.json
      válido" (#1), el límite walk-up vs worktree (#2 — **riesgo alto en este repo bare+worktree**),
      el merge de `activity.jsonl` (#3), el backup del stray (#4), la semántica de conflicto parcial
      (#5) y el enforcement futuro del campo `repo` (#6).

## 9. Corrección — pines explícitos sobre nearest-wins (worktree shadowing)

- [x] 9.1 `cli/internal/config/config.go`: añadir `StateRoot string \`json:"stateRoot,omitempty"\``
      al struct `Config` (aditivo, `SchemaVersion` sin cambios).
- [x] 9.2 `Config.ResolveStateRootPin(configDir string) (root string, ok bool)`: resuelve
      `StateRoot` (relativo a `configDir` cuando no es absoluto), valida el target con `Load`;
      `ok == false` para vacío, auto-referencia (target == `configDir`), o target sin
      `.vector/config.json` cargable — el caller cae a nearest-wins, nunca aborta. Sin recursión
      sobre el `stateRoot` del target (un solo salto; sin loop no trivial posible).
- [x] 9.3 `cli/cmd/vector/main.go` — `resolveRepoRootStrays`: gana el env var `VECTOR_REPO_ROOT`
      (salta el walk-up entero cuando está seteada y no vacía) y la re-anclaje vía
      `ResolveStateRootPin` sobre el config al que el walk-up ancla. Precedencia:
      `--repo-root` > `VECTOR_REPO_ROOT` > `stateRoot` > walk-up nearest-wins > git-toplevel > cwd.
- [x] 9.4 Firma ampliada: `resolveRepoRootStrays` retorna además `*ShadowNotice` (nil = sin
      shadowing) cuando el walk-up ancla a un store pero existe un ancestro más arriba y ningún
      pin redirigió. `resolveRepoRoot` (wrapper) descarta strays y shadow igual que antes.
- [x] 9.5 `warnShadow(shadow *ShadowNotice)`: imprime `ui.Warning` nombrando el store del worktree
      y el ancestro shadowed, sugiriendo `stateRoot`/`VECTOR_REPO_ROOT`. Cableado en los mismos
      call-sites humanos que `warnStrayStores` (`update`, `spec create`, `serve`, `doctor`),
      **nunca** dentro de una rama `--json`.
- [x] 9.6 Tests: `cli/internal/config/config_test.go` (`ResolveStateRootPin` — target absoluto,
      relativo, vacío, auto-referencia, target inválido; round-trip `omitempty`) y
      `cli/cmd/vector/root_resolution_test.go` (env var salta el walk-up; `--repo-root` explícito
      sigue ganándole al env var; `stateRoot` re-ancla, tanto absoluto como relativo a su propio
      config; `stateRoot` auto-referenciado o inválido se ignora y cae a nearest-wins; nearest-wins
      sin pin no cambia; shadowing se reporta con las rutas correctas; `vector doctor --json` sin
      rastro de shadowing en la salida).
- [x] 9.7 Gate verde: `go vet ./...` y `go test ./...` (incluye la suite golden) sin drift en los
      shapes `--json` existentes.
## 10. Rebaseline — authoritative workspace root and safe adoption

- [x] 10.1 Collect all valid physical ancestor stores and select the outermost workspace store.
- [x] 10.2 Reject inner `--repo-root`, `VECTOR_REPO_ROOT`, and `stateRoot` targets; `init --force`
      cannot create an inner store.
- [x] 10.3 Add a canonical-root adoption lock, complete preflight, atomic activity replacement, and
      rollback for completed artifact moves after failure.
- [x] 10.4 Cover worktree-root resolution, inner-pin rejection, lock contention, and rollback.
