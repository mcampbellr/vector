# Spec: Corregir 3 hallazgos LOW del code review de vector-context-cached-setup

## 1. Objetivo

Corregir tres issues de calidad interna (prioridad LOW) detectados en el code review del change
`vector-context-cached-setup`, todos acotados a `cli/`:

1. **`parseMakefile` produce falsos positivos** con asignaciones de variable Make (`:=`, `?=`,
   `+=`) porque `strings.Cut(line, ":")` se dispara sobre el primer `:`, haciendo que una línea
   como `build := go build` sea interpretada como el target `build`.
2. **`runContext` enmascara errores reales de config** (JSON malformado, enum inválido) bajo el
   mensaje genérico `"no .vector/config.json — run vector init first"`, impidiendo que el
   usuario sepa qué salió mal.
3. **`TestDetectBuildCmds` carece de cobertura Python**: no existe ningún caso que valide la
   rama `isPy` del detector, dejando sin ejercitar la lógica de `pyproject.toml`/`setup.py`.

Esta corrección permite que el detector de comandos sea preciso, que los errores de config sean
diagnósticos, y que el gate de calidad cubra la detección Python.

## 2. Alcance

### Incluido en esta fase

- **Fix 1 — `parseMakefile`** (`cli/internal/config/config.go:228–244`): añadir un guard antes
  de `strings.Cut(line, ":")` que descarte cualquier línea que contenga `:=`, `?=` o `+=`
  (asignaciones Make). Targets legítimos (`build:`, `lint:`, `test:`) no contienen esos
  patrones y no se ven afectados.
- **Fix 2 — `runContext`** (`cli/cmd/vector/context.go:50–54`): distinguir
  `errors.Is(err, os.ErrNotExist)` → mantener el mensaje `"run vector init first"` solo para
  ese caso; cualquier otro error de `config.Load` → propagar el error real sin envolver.
  Añadir `"errors"` al bloque de imports de `context.go`.
- **Fix 3 — `TestDetectBuildCmds`** (`cli/internal/config/config_test.go:411–526`): añadir el
  caso `pyproject_only` a la table del test, que escribe solo `pyproject.toml` en `t.TempDir()`
  y espera `build="python -m build"`, `lint=""`, `test="pytest"`. Cubierto vía
  `TestDetectBuildCmds`, sin crear un test dedicado para `parseMakefile`.

### Fuera de scope

- Cambiar la prioridad de detectores (Makefile gana sobre Go, etc.).
- Validar o migrar configs corruptos: no se añade código de repair.
- Refactorizar `parseMakefile` o `DetectBuildCmds` más allá del guard de una línea.
- Cubrir la rama `parseMakefile` con un test unitario separado: basta el caso table-driven en
  `TestDetectBuildCmds`.
- Añadir campos a structs, nuevas features de detección ni endpoints HTTP.
- Cubrir combinaciones Python+Makefile o Python+Go en esta fase.
- Tocar `runDetectTicket`, `runInit`, `runUpdate` u otros subcomandos.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Lenguaje: **Go** (módulo único en `cli/`, solo stdlib, sin deps externas).
- Error handling: `errors.Is` de la stdlib para distinguir sentinel errors.
- Testing: paquete `testing` estándar; tests table-driven con `t.TempDir()`.
- Config: struct `config.Config` serializado a/desde `.vector/config.json`
  (`cli/internal/config/config.go`).

### Versiones relevantes

- Go: **1.26** (declarado en `cli/go.mod`).
- No se añaden dependencias externas; el change usa solo `errors`, `os`, `strings` de stdlib,
  ya presentes en los archivos afectados (o `"errors"` como import nuevo en `context.go`).

### Patrones existentes a respetar

- **Semántica silenciosa de `DetectBuildCmds`**: la función no propaga errores al caller;
  retorna vacíos ante cualquier fallo de I/O. El guard nuevo en `parseMakefile` mantiene esta
  semántica (simplemente ejecuta `continue`).
- **`errors.Is(err, os.ErrNotExist)`**: patrón ya usado en `runDetectTicket`
  (`cli/cmd/vector/main.go:1000–1002`). El fix de `runContext` sigue exactamente ese patrón.
- **Propagación directa sin envolver**: para errores no-ErrNotExist en `runContext`, se
  retorna `err` directamente (sin `fmt.Errorf("… : %w", err)` adicional).
- **Tests table-driven con `t.TempDir()`**: estructura ya establecida en
  `TestDetectBuildCmds` (`config_test.go:411–526`). El caso nuevo sigue la misma forma.
- **Naming kebab-case**: nombre del caso de test: `pyproject_only` (snake_case, convención Go
  para `t.Run` names — no confundir con slugs de surface de usuario).
- **`gofmt`/`go vet`** sin warnings; `go test ./...` verde como gate obligatorio.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Change `vector-context-cached-setup` mergeado: `cli/cmd/vector/context.go` existe con
      `runContext` y la lógica de detección, incluyendo la llamada `config.Load` en las
      líneas 50–54.
- [x] `parseMakefile` en `cli/internal/config/config.go:223–246` — función existente a
      parchear.
- [x] `DetectBuildCmds` en `cli/internal/config/config.go:151–214` — función que invoca
      `parseMakefile` y cuyo comportamiento Python ya existe pero carece de tests.
- [x] `TestDetectBuildCmds` en `cli/internal/config/config_test.go:411–526` — table-driven,
      exportado en el paquete `config` (acceso a `DetectBuildCmds` directamente).
- [x] Patrón `errors.Is(cfgErr, os.ErrNotExist)` en `cli/cmd/vector/main.go:999–1004`
      (`runDetectTicket`) — referencia directa del fix de `runContext`.

### Causas deducidas de cada hallazgo

**Hallazgo 1 — `parseMakefile` falsos positivos**
`strings.Cut(line, ":")` usa el **primer** `:` encontrado. Una línea como `build := go build`
produce `name = "build "`, que `strings.TrimSpace` normaliza a `"build"`, disparando el case
del switch. El guard de prefijo de tabulación/espacio (líneas 229–231) no filtra estas líneas
porque arrancan con el nombre de la variable, no con espacio. La fix correcta es descartar la
línea completa si contiene `:=`, `?=` o `+=` antes de llegar al `Cut`.

**Hallazgo 2 — `runContext` error masking**
El bloque `if err != nil` en `context.go:51–53` construye siempre el mismo mensaje de error
con `fmt.Errorf("no .vector/config.json in %s — run vector init first", root)`,
independientemente de si el error es `os.ErrNotExist` (config ausente) o un error diferente
(JSON malformado, valor de enum inválido en deserialización). El caller no puede distinguir
entre "no inicializado" y "config corrupto".

**Hallazgo 3 — falta cobertura Python**
`TestDetectBuildCmds` cubre Go, Node (npm/pnpm), Makefile, y el caso sin manifiestos, pero no
incluye ningún caso que active `isPy = true`. La rama Python de `build`/`test` en
`DetectBuildCmds` (líneas 190–192, 211–213 de config.go) no está ejercitada por ningún test.

Si alguna dependencia no existe, el agente debe detenerse y reportar qué falta. No inventar
contratos ni rutas.

---

## 5. Arquitectura

### Patrón a usar

**Hotfix acotado**: cambios mínimos en exactamente tres localizaciones, sin refactor periférico,
siguiendo patrones preexistentes en el mismo repo.

### Capas afectadas

- presentation (web/board): **no** — sin UI, sin endpoints HTTP.
- application/CLI (`cli/cmd/vector`): **sí** — `context.go`: corrección del manejo de error
  en `runContext`.
- domain/config (`cli/internal/config`): **sí** — `config.go`: guard en `parseMakefile`;
  `config_test.go`: caso de test Python.
- data/estado (`.vector/specs`, `activity.jsonl`): **no** — los fixes son de lógica de
  detección y error handling, sin tocar persistencia.

### Flujo esperado (post-fix)

**Fix 1 — parseMakefile:**
1. Para cada línea del Makefile, tras los guards de prefijo whitespace (tab/espacio),
   se evalúa el nuevo guard: si la línea contiene `:=`, `?=` o `+=`, se ejecuta `continue`
   y la línea se descarta.
2. Solo las líneas que pasan el guard llegan a `strings.Cut(line, ":")`.
3. Targets legítimos (`build:`, `lint:`, `test:`) no contienen esos patrones y se detectan.
4. Asignaciones (`BUILD_CMD := go build`) se descartan y no producen falso positivo.

**Fix 2 — runContext:**
1. `cfg, err := config.Load(root)` — si `err != nil`:
   - `errors.Is(err, os.ErrNotExist)` → retornar
     `fmt.Errorf("no .vector/config.json in %s — run vector init first", root)`.
   - Cualquier otro error → retornar `err` directamente (sin envolver).
2. Si `err == nil`, el flujo continúa sin cambios.

**Fix 3 — TestDetectBuildCmds:**
1. Se añade el caso `pyproject_only` a la tabla existente.
2. `setup`: escribe un `pyproject.toml` mínimo (p. ej. `[build-system]`) en el `t.TempDir()`.
3. No hay `go.mod`, ni `package.json`, ni `Makefile`, ni `setup.py` en el dir.
4. `DetectBuildCmds` detecta `isPy = true` por la presencia de `pyproject.toml`; retorna
   `build="python -m build"`, `lint=""`, `test="pytest"`.

### Ubicación de archivos

No se crean archivos ni paquetes nuevos. Solo se modifican tres archivos existentes.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `cli/internal/config/config.go` | MODIFICAR | Añadir guard `:=`/`?=`/`+=` en `parseMakefile` (línea ~232, antes del `strings.Cut`). | Guards de whitespace en las líneas 229–231 del mismo archivo |
| `cli/cmd/vector/context.go` | MODIFICAR | Distinguir `os.ErrNotExist` de otros errores en `runContext` (líneas 50–54); añadir `"errors"` a imports. | Patrón `runDetectTicket` en `cli/cmd/vector/main.go:999–1004` |
| `cli/internal/config/config_test.go` | MODIFICAR | Añadir caso `pyproject_only` a la tabla de `TestDetectBuildCmds`. | Casos existentes de la misma tabla (`go_mod_only`, `makefile_with_all_targets`, etc.) |

### Detalle por archivo

#### cli/internal/config/config.go

Acción: MODIFICAR

Cambios requeridos:
- En `parseMakefile`, dentro del bucle `for _, line := range strings.Split(...)` (línea 228),
  inmediatamente después de los dos `continue` de prefijo whitespace (líneas 229–231) y
  **antes** de `name, _, ok := strings.Cut(line, ":")` (línea 232), añadir:
  ```go
  if strings.Contains(line, ":=") || strings.Contains(line, "?=") || strings.Contains(line, "+=") {
      continue
  }
  ```
- No cambiar ninguna otra línea de `parseMakefile` ni de `DetectBuildCmds`.
- El import `"strings"` ya está presente en el archivo; no se añaden imports nuevos.

Restricciones:
- No tocar el switch statement ni los casos `build`/`lint`/`test`.
- No cambiar la semántica de manejo de errores de `parseMakefile` (sigue retornando silenciosamente).
- No refactorizar la función; solo el guard de tres condiciones en la posición descrita.

#### cli/cmd/vector/context.go

Acción: MODIFICAR

Cambios requeridos:
- Añadir `"errors"` al bloque de imports (líneas 3–12). Ya existen `"fmt"` y `"os"` en el mismo
  bloque; `"errors"` va junto a ellos en el grupo de stdlib.
- Reemplazar el bloque `if err != nil { return fmt.Errorf("no .vector/config.json in %s …") }`
  (líneas 51–54) por:
  ```go
  if err != nil {
      if !errors.Is(err, os.ErrNotExist) {
          return err
      }
      return fmt.Errorf("no .vector/config.json in %s — run vector init first", root)
  }
  ```
- El mensaje de error para `os.ErrNotExist` se mantiene idéntico al actual.
- Para cualquier otro error (JSON inválido, enum inválido, permisos), se retorna `err`
  directamente, sin envolver con contexto adicional.

Restricciones:
- No cambiar nada más en `runContext`: ni la lógica de detección concurrente, ni el
  `ContextOutput`, ni la serialización JSON.
- No tocar `ContextOutput` ni ninguna otra función del archivo.
- El patrón exacto a seguir es `runDetectTicket` en `cli/cmd/vector/main.go:999–1004`:
  ```go
  cfg, cfgErr := config.Load(root)
  if cfgErr != nil {
      if !errors.Is(cfgErr, os.ErrNotExist) {
          return cfgErr
      }
      cfg = &config.Config{}
  }
  ```
  La diferencia: en `runContext` la ausencia de config sí es un error (no tiene fallback de
  config vacío), por lo que cuando es `ErrNotExist` se retorna el mensaje accionable.

#### cli/internal/config/config_test.go

Acción: MODIFICAR

Cambios requeridos:
- En la tabla `tests` de `TestDetectBuildCmds` (líneas 412–508), añadir un nuevo caso al final
  de la slice (antes del cierre `}`):
  ```go
  {
      name: "pyproject_only",
      setup: func(root string) {
          if err := os.WriteFile(
              filepath.Join(root, "pyproject.toml"),
              []byte("[build-system]\nrequires = [\"setuptools\"]\n"),
              0o644,
          ); err != nil {
              t.Fatal(err)
          }
      },
      wantBuild: "python -m build",
      wantLint:  "",
      wantTest:  "pytest",
  },
  ```
- No crear un segundo `t.Run` separado ni una función de test nueva para `parseMakefile`.
- No modificar los casos existentes de la tabla.

Restricciones:
- El caso solo crea `pyproject.toml`; no debe crear `setup.py`, `go.mod` ni `package.json`,
  ya que el objetivo es validar la rama `isPy` exclusiva con `pyproject.toml`.
- Mantener el mismo patrón de reporte de error (`t.Errorf`) que usan los demás casos.
- El contenido de `pyproject.toml` es irrelevante para la detección (solo se usa `os.Stat`);
  un contenido mínimo válido es suficiente.

---

## 7. API Contract

No aplica — ninguno de los tres fixes introduce ni modifica endpoints HTTP. Los cambios son
internos a la lógica de detección de `cli/internal/config` y al manejo de errores de
`cli/cmd/vector/context.go`.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

**Fix 1 — parseMakefile:**
- [ ] Una línea `build := go build` en el Makefile **no** activa `mt.build = true`
      (falso positivo eliminado).
- [ ] Una línea `build:` en el Makefile **sí** activa `mt.build = true` (detección legítima
      no regresionada).
- [ ] El caso `pyproject_only` de `TestDetectBuildCmds` pasa sin que un Makefile con
      asignaciones cause `"make build"` inesperado.

**Fix 2 — runContext:**
- [ ] Con `.vector/config.json` ausente: `runContext` retorna un error que contiene
      `"run vector init first"`.
- [ ] Con `.vector/config.json` presente pero con JSON malformado: `runContext` retorna el
      error real de deserialización (`json.Unmarshal`), **no** el mensaje `"run vector init first"`.
- [ ] Con `.vector/config.json` presente pero con enum inválido (p. ej. `applyMode` fuera de
      rango): `runContext` retorna el error de validación real, no el mensaje genérico.

**Fix 3 — TestDetectBuildCmds:**
- [ ] El caso `pyproject_only` se ejecuta como parte de `TestDetectBuildCmds` y pasa:
      `build == "python -m build"`, `lint == ""`, `test == "pytest"`.
- [ ] Los casos existentes de `TestDetectBuildCmds` siguen pasando sin regresión.

**Gate de calidad:**
- [ ] `gofmt -l cli/` no lista ningún archivo modificado.
- [ ] `go -C cli vet ./...` sin warnings.
- [ ] `go -C cli test ./...` verde, incluyendo el caso nuevo.

### Tests requeridos

- [ ] Caso `pyproject_only` en `TestDetectBuildCmds` — rama `isPy` con solo `pyproject.toml`.
- [ ] (Cubierto implícitamente vía el caso `makefile_with_all_targets` + regresión): target
      legítimo `build:` sigue detectado tras el guard nuevo.
- [ ] Para `runContext`: los tests de integración existentes (si los hay en `config_test.go` o
      en `cmd/vector`) deben seguir pasando. No se requieren tests nuevos de `runContext` en
      esta fase, pero el comportamiento de error-masking ya no debe ocurrir.

### Comportamiento esperado vs. actual (pre-fix)

| Escenario | Actual (bug) | Esperado (post-fix) |
|---|---|---|
| Makefile con `build := go build` | `build = "make build"` (falso positivo) | `build = ""` (ignorado) |
| Makefile con `build:` | `build = "make build"` | `build = "make build"` (sin cambio) |
| Config JSON malformado en `runContext` | `"no .vector/config.json — run vector init first"` | error real de `json.Unmarshal` |
| Config ausente en `runContext` | `"no .vector/config.json — run vector init first"` | `"no .vector/config.json — run vector init first"` (sin cambio) |
| `pyproject.toml` solo, sin otros manifiestos | sin test (cobertura ausente) | `build="python -m build"`, `lint=""`, `test="pytest"` |

### Comandos de verificación

Ejecutar:

```bash
gofmt -l cli/
go -C cli vet ./...
go -C cli test ./...
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

No aplica — los tres fixes son internos al binario Go (`cli/`). No hay formularios, pantallas
ni flujos de UI involucrados. El único efecto observable para el usuario es que `vector context`
muestra un error diagnóstico real en vez de un mensaje genérico cuando el config está corrupto;
eso no constituye un cambio de UX intencionado sino la restauración del comportamiento correcto.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas; el agente no debe cuestionarlas ni cambiarlas:

- **Filtrar solo `:=`, `?=`, `+=`** — no `=` simple. Una asignación Make simple (`VAR = valor`)
  no contiene `:` antes del `=`, por lo que `strings.Cut(line, ":")` no produce un match; no
  necesita guardarse. Filtrar `=` simple causaría falsos negativos en targets como `test-race:`
  seguido de `=` en el recipe, aunque el recipe ya se descarta por prefix whitespace.
- **Propagar error directo en `runContext`** — sin envolver con `fmt.Errorf`. El mensaje de
  `json.Unmarshal` o del validador de config ya es suficientemente diagnóstico; añadir un wrapper
  solo añade ruido.
- **No crear test dedicado para `parseMakefile`** — el caso `pyproject_only` cubre la cobertura
  requerida de Python vía `TestDetectBuildCmds`. Un test unitario de `parseMakefile` en aislamiento
  no fue solicitado y el hallazgo de falsos positivos queda cubierto con el caso de asignación Make
  que no produce falso positivo más el caso de target legítimo que sí se detecta.
- **`errors.Is` como espejo de `runDetectTicket`** — el patrón existente en
  `cli/cmd/vector/main.go:999–1004` es la referencia canónica para este tipo de distinción en
  el codebase; no se introduce un patrón nuevo.
- **`SchemaVersion` no se toca** — los fixes no alteran el esquema de `config.Config`.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, no
implementarla.

---

## 11. Edge cases

### Fix 1 — parseMakefile

**Líneas de asignación Make:**
- `build := go build` → contiene `:=` → `continue` → `mt.build` permanece `false`. Correcto.
- `BUILD_CMD ?= docker build` → contiene `?=` → `continue` → sin efecto en los targets.
- `EXTRA_FLAGS += -v` → contiene `+=` → `continue` → sin efecto.
- `build: deps` → no contiene `:=`/`?=`/`+=` → llega al `Cut` → `name = "build"` → match.

**Líneas de comentario:**
- `# VAR := value` → `strings.Contains(line, ":=")` es `true` → `continue`. Correcto; un
  comentario no es un target.

**Makefile ausente o ilegible:**
- `os.ReadFile` retorna error → `parseMakefile` retorna `makefileTargets{}` (todos false) sin
  cambio respecto al comportamiento actual. El guard nuevo nunca se ejecuta.

**Target con `:=` en comentario inline:**
- `build: ## override := value` → la línea contiene `:=` → descartada por el guard. Falso
  negativo. Riesgo aceptado (documentado en el BRIEF); el test del caso `makefile_with_all_targets`
  con target `build:` limpio verifica que el caso normal funciona. Un target con `":="` en su
  comentario inline es un patrón muy inusual.

**Pasos de reproducción del bug original:**
1. Crear un Makefile con `build := go build` (sin target real `build:`).
2. Llamar a `DetectBuildCmds(root)`.
3. Pre-fix: retorna `build = "make build"` (falso positivo).
4. Post-fix: retorna `build = ""` (correcto).

### Fix 2 — runContext

**Config ausente:**
- `config.Load` retorna un error que satisface `errors.Is(err, os.ErrNotExist)` →
  se retorna `fmt.Errorf("no .vector/config.json in %s — run vector init first", root)`.
  Comportamiento idéntico al actual.

**JSON malformado:**
- `.vector/config.json` existe pero contiene `{invalid json` → `config.Load` retorna un error
  de `json.Unmarshal` que **no** satisface `errors.Is(err, os.ErrNotExist)` → se retorna `err`
  directamente. Post-fix, el usuario ve el error real.

**Enum inválido en config:**
- `.vector/config.json` con `"applyMode": "unknown"` → `config.Load` retorna error de validación
  → mismo caso que JSON malformado; se propaga.

**Pasos de reproducción del bug original:**
1. Escribir `.vector/config.json` con contenido `{not valid json}`.
2. Ejecutar `vector context`.
3. Pre-fix: salida de error: `"no .vector/config.json in <root> — run vector init first"`.
4. Post-fix: salida de error real del parser JSON.

### Fix 3 — TestDetectBuildCmds (pyproject_only)

**Solo pyproject.toml, sin otros manifiestos:**
- `isPy = true` (por `os.Stat("pyproject.toml") == nil`), `isGo = false`, `node` vacío,
  `mk` todo-false → `build = "python -m build"`, `lint = ""`, `test = "pytest"`.

**Solo setup.py (fuera de este spec):**
- Otro caso posible (`setup.py` solo) no se añade en esta fase (fuera de scope). `isPy` se
  activa por `e1 == nil || e2 == nil`; el caso pyproject_only cubre el OR.

---

## 12. Estados de UI requeridos

No aplica — ninguno de los tres fixes introduce ni modifica componentes de UI. El board kanban
y el panel web no son afectados.

---

## 13. Validaciones

### Validaciones internas (Go)

| Componente | Regla | Comportamiento ante incumplimiento |
|---|---|---|
| `parseMakefile` — línea de Makefile | Si contiene `:=`, `?=` o `+=`: descartar | `continue`; sin error; sin side effects |
| `runContext` — config ausente | `errors.Is(err, os.ErrNotExist)` | Retornar mensaje `"run vector init first"` |
| `runContext` — config corrupto | Cualquier otro error de `config.Load` | Retornar `err` directamente |
| `TestDetectBuildCmds` — pyproject_only | `pyproject.toml` solo en dir → `isPy = true` | `wantBuild = "python -m build"`, `wantLint = ""`, `wantTest = "pytest"` |

No hay validaciones de servidor (no hay backend remoto involucrado en estos fixes).

---

## 14. Seguridad y permisos

- Los tres fixes son de lógica interna (parsing, error routing, tests). No se manejan secrets
  ni tokens.
- `parseMakefile` lee el Makefile del repo del usuario con `os.ReadFile`; la semántica de fallo
  silencioso se mantiene, sin exponer rutas ni contenidos en mensajes de error.
- `runContext`: propagar el error real de `config.Load` puede mostrar el path del archivo en
  el error. Este path ya era visible en la mayoría de los errores de I/O y no constituye un
  leak de información sensible.
- No se añaden permisos, ni escrituras al repo del usuario, ni cambios en `.vector/`.

---

## 15. Observabilidad y logging

- No se añaden mecanismos de logging nuevos. Los tres fixes no tocan el mecanismo de reporte
  de salida estándar del binario.
- El único cambio observable en salida es el de `runContext`: ante config corrupto, el mensaje
  de error en stderr pasa de ser genérico a ser el error real del parser/validador. Esto mejora
  la diagnósticabilidad sin añadir logging estructurado.

---

## 16. i18n / textos visibles

No aplica — los mensajes de error del CLI permanecen en inglés (la i18n del CLI está fuera de
scope y no existe sistema de traducciones de UI en esta capa). El único mensaje que se mantiene
literalmente es `"no .vector/config.json in %s — run vector init first"` (ya existente).

---

## 17. Performance

- **Fix 1**: el guard añade hasta tres llamadas `strings.Contains` por línea de Makefile. El
  costo es despreciable dado que los Makefiles son archivos pequeños y la función ya hace I/O.
- **Fix 2**: un `errors.Is` adicional por invocación de `runContext` — costo cero relevante.
- **Fix 3**: solo añade un caso más al test existente. Sin impacto en el binario de producción.
- Sin llamadas de API repetidas, sin bloqueos de hilo principal.

---

## 18. Restricciones

El agente no debe:

- Cambiar la prioridad de detectores en `DetectBuildCmds` (Makefile > Go > Node > Python).
- Filtrar `=` simple (asignación estilo POSIX sin modificador) — solo `:=`, `?=`, `+=`.
- Envolver el error real de `config.Load` con `fmt.Errorf` adicional en el path no-ErrNotExist
  de `runContext`.
- Crear un test unitario separado para `parseMakefile`: el hallazgo se cubre vía
  `TestDetectBuildCmds`.
- Modificar archivos fuera de los tres listados en §6 sin justificación explícita.
- Añadir dependencias externas (se mantiene stdlib).
- Refactorizar `parseMakefile`, `DetectBuildCmds` ni `runContext` más allá de los cambios
  puntuales descritos.
- Cambiar `SchemaVersion` ni añadir código de migración.
- Tocar la lógica de `runDetectTicket`, `runInit`, `runUpdate` u otros subcomandos.
- Ignorar errores de `gofmt`/`go vet`/tests.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `cli/internal/config/config.go` con el guard `:=`/`?=`/`+=` en `parseMakefile`
      (una inserción de ~3 líneas antes del `strings.Cut`).
- [ ] `cli/cmd/vector/context.go` con el import `"errors"` añadido y el bloque de error de
      `runContext` corregido para distinguir `os.ErrNotExist` de otros errores.
- [ ] `cli/internal/config/config_test.go` con el caso `pyproject_only` añadido a la tabla
      de `TestDetectBuildCmds`.
- [ ] Gate verde: `gofmt -l cli/` sin archivos listados, `go -C cli vet ./...` sin warnings,
      `go -C cli test ./...` con todos los tests verdes incluyendo el caso nuevo.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Solo modifiqué los tres archivos listados en §6 (o justifiqué cualquier excepción).
- [ ] El guard en `parseMakefile` va **antes** del `strings.Cut`, después de los guards de
      whitespace existentes (líneas 229–231 de `config.go`).
- [ ] El guard filtra `:=`, `?=` y `+=` — no `=` simple.
- [ ] En `runContext`: el import `"errors"` fue añadido a `context.go`.
- [ ] En `runContext`: `os.ErrNotExist` → mensaje `"run vector init first"`; cualquier otro
      error → `return err` directo (sin `fmt.Errorf` wrapper adicional).
- [ ] El caso `pyproject_only` de `TestDetectBuildCmds` solo crea `pyproject.toml` (sin
      `setup.py`, `go.mod` ni `package.json`).
- [ ] El caso `pyproject_only` espera exactamente: `build="python -m build"`, `lint=""`,
      `test="pytest"`.
- [ ] Los casos existentes de `TestDetectBuildCmds` siguen pasando (no regresión).
- [ ] No refactoricé `parseMakefile`, `DetectBuildCmds` ni `runContext` más allá de lo
      estrictamente descrito.
- [ ] No cambié la prioridad de detectores ni la semántica silenciosa de `DetectBuildCmds`.
- [ ] Seguí el patrón `runDetectTicket` (`main.go:999–1004`) como referencia para el fix de
      `runContext`.
- [ ] Ejecuté `gofmt -l cli/`, `go -C cli vet ./...`, `go -C cli test ./...` — todos verdes.
- [ ] No dejé logs temporales ni TODOs sin justificar.
