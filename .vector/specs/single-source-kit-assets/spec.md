# Spec: Fuente única para los assets del kit (single-source-kit-assets)

## 1. Objetivo

Eliminar las tres copias paralelas de agentes y commands del kit y establecer `kit/` como
**única fuente editable**, de modo que ningún artefacto del kit necesite actualizarse en más
de un lugar.

Esta mejora permite que cualquier desarrollador de Vector edite un agente o command en `kit/`
y propague el cambio al binario distribuible y al repo de dogfooding mediante un flujo de un
solo sentido (`go generate` → reinstalar binario → `vector update`), sin tocar manualmente
`cli/internal/scaffold/assets/` ni `.claude/`.

## 2. Alcance

### Incluido en esta fase

- Dejar de rastrear en git los archivos de `kit/` que actualmente se duplican en
  `.claude/agents/` y `.claude/commands/vector/` del propio repo Vector (11 archivos
  rastreados actualmente: 3 agentes + 8 commands).
- Agregar reglas en `.gitignore` para que `git` no rastree ni proponga rastrear
  `.claude/agents/` y `.claude/commands/vector/` en el repo Vector.
- Documentar el flujo canónico en `cli/internal/scaffold/scaffold.go` (cabecera de paquete)
  y en `.claude/rules/architecture/distribution-packaging.md`.
- Actualizar la nota de memoria de reinstalación para incluir el paso `vector update`.
- Agregar un check de CI que detecte drift entre `kit/` y `cli/internal/scaffold/assets/`
  antes del build.
- Verificar que el `//go:generate` existente es completo y correcto (ya copia
  `kit/commands`, `kit/agents` y `kit/vector` → `assets/`).

### Fuera de scope

- Cambiar la lógica de `SeedCommands`, `writeSeed` o `CommandPaths` en `scaffold.go`.
- Cambiar el esquema o la semántica del JSON de estado.
- Tocar archivos de `web/`, `cli/cmd/`, `cli/internal/state/`, `cli/internal/board/` u otros
  paquetes del CLI no relacionados con el scaffold.
- Modificar el contenido de ningún agente o command del `kit/` (esta fase no cambia el
  contenido, solo el flujo de distribución).
- Gestión del spec-template (`kit/vector/spec-template.md`) — ese archivo ya sigue el mismo
  flujo de embed y no requiere cambios.
- Publicación ni CI de release; solo el check de drift en desarrollo.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca
relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Binario: **Go 1.26** (módulo único, stdlib, sin dependencias externas) — `cli/`.
- Embed: `embed.FS` con directiva `//go:embed all:assets` en `scaffold.go`.
- Vendoring de assets: shell script en `//go:generate` (ya existente).
- CI: TBD — ver Open questions §1. No existe `.github/workflows/` actualmente.
- Shell scripting: solo el script inline del `//go:generate`; sin Makefile actualmente.

### Versiones relevantes

- Go: `1.26` (`cli/go.mod`).
- No se agregan dependencias externas.

### Patrones existentes a respetar

- `//go:generate sh -c "rm -rf assets && mkdir -p assets && cp -R ../../../kit/commands ../../../kit/agents ../../../kit/vector assets/"` — directiva ya presente en `scaffold.go`; **no cambiar**.
- `//go:embed all:assets` — directiva ya presente; **no cambiar**.
- `SeedCommands` / `writeSeed` — lógica de seedeo ya correcta; **no cambiar**.
- `.gitignore` del repo Vector — kebab-case, una regla por línea, comentarios explicativos
  alineados al estilo del archivo (`# Sección\nruta/`).
- Artefactos de git en inglés (branch names, commit messages, PR bodies).
- Los agentes de `kit/` son todos los definidos en `kit/agents/` (6 archivos); los commands
  en `kit/commands/vector/` (11 archivos). Ninguno debe enumerarse a mano en código.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `//go:generate` en `cli/internal/scaffold/scaffold.go` — ya presente y correcto.
- [x] `//go:embed all:assets` en `scaffold.go` — ya presente.
- [x] `SeedCommands` implementado con escritura atómica — ya presente con tests.
- [x] `vector update` re-siembra `.claude/` desde el binario instalado — ya implementado
  (`cmd/vector/main.go`).
- [x] `kit/agents/` y `kit/commands/vector/` en sync con `cli/internal/scaffold/assets/`
  (verificado: `diff -r` no produce diferencias en el estado actual de la rama).
- [ ] Decisión sobre la estrategia de CI (TBD — ver Open questions §1). No bloquea las
  otras acciones de esta fase, pero el check de drift queda pendiente hasta que se resuelva.

Si alguna dependencia no existe, el agente se detiene y reporta exactamente qué falta.

---

## 5. Arquitectura

### Patrón

Flujo de un solo sentido: **`kit/` → `go generate` → `assets/` → binario → `vector update` → `.claude/`**.

Ningún paso del flujo puede ser omitido sin que se produzca drift. El repo Vector dogfoodea
el binario instalado, igual que cualquier repo de usuario.

### Capas afectadas

- **`kit/`**: sí — es la única fuente editable; no cambia su contenido, solo se formaliza
  su rol.
- **`cli/internal/scaffold/`**: sí (solo el `.gitignore` y la documentación de la cabecera
  de `scaffold.go`). La lógica de `SeedCommands` y el `//go:generate` no se tocan.
- **`.gitignore`**: sí — se agregan reglas para `cli/internal/scaffold/assets/` (ya podría
  estar excluido; verificar) y para `.claude/agents/` y `.claude/commands/vector/`.
- **`.claude/rules/architecture/distribution-packaging.md`**: sí — se documenta el flujo
  completo de single-source.
- **Memory nota de reinstalación**: sí — se agrega `vector update` como paso obligatorio.
- **`.github/workflows/`**: sí (TBD) — se crea el check de drift si se decide activar CI.
- **`web/`**, **`cli/cmd/`**, **`cli/internal/state/`**: no.

### Flujo de propagación esperado (post-fase)

1. Dev edita un archivo en `kit/agents/<agente>.md` o `kit/commands/vector/<cmd>.md`.
2. Dev corre `go generate ./internal/scaffold` desde `cli/` → actualiza
   `cli/internal/scaffold/assets/`.
3. Dev reinstala el binario (`go install ./cmd/vector` o el script de la Memory).
4. Dev corre `vector update` en la raíz del repo Vector → `SeedCommands` siembra
   `.claude/agents/<agente>.md` y `.claude/commands/vector/<cmd>.md` desde el binario
   (con `Force: true` implícito en update).
5. `.claude/` refleja el cambio sin intervención manual. Git no rastrea esas rutas.
6. CI (TBD) corre `go generate` en un directorio limpio y verifica que `assets/` no
   diverge del estado comprometido.

### Ubicación de archivos nuevos o modificados

```txt
.gitignore                                           ← MODIFICAR
cli/internal/scaffold/scaffold.go                   ← MODIFICAR (solo comentario de paquete)
.claude/rules/architecture/distribution-packaging.md ← MODIFICAR
.claude/projects/.../memory/MEMORY.md               ← MODIFICAR (nota de reinstalación)
.github/workflows/ci.yml                            ← NUEVO (TBD — ver Open questions §1)
```

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `.gitignore` | MODIFICAR | Excluir `.claude/agents/` y `.claude/commands/vector/`; verificar que `cli/internal/scaffold/assets/` también esté excluido si corresponde | `.gitignore` existente (sección `# Vector state:`) |
| `cli/internal/scaffold/scaffold.go` | MODIFICAR | Actualizar el comentario de paquete para documentar el flujo single-source y el paso `vector update` | Cabecera actual del paquete |
| `.claude/rules/architecture/distribution-packaging.md` | MODIFICAR | Agregar subsección "Flujo de edición single-source" con los 4 pasos canónicos | Sección "Implicaciones para el desarrollo" existente |
| `.claude/projects/-Users-mariocampbell-Developer-vector/memory/MEMORY.md` | MODIFICAR | Añadir `vector update` como paso 4 de la nota de reinstalación | Entrada existente "Reinstall vector binary after changes" |
| `.github/workflows/ci.yml` | NUEVO (TBD) | Check de drift: corre `go generate` y verifica que `cli/internal/scaffold/assets/` no diverge del estado comprometido | TBD — ver Open questions §1 |

### Git: desrastrear los 11 archivos de `.claude/` comprometidos

Antes de agregar las reglas al `.gitignore`, los archivos actualmente rastreados deben
retirarse del índice de git **sin borrarlos del disco** (son necesarios para continuar
trabajando antes de la primera ejecución de `vector update`):

```bash
git rm --cached \
  .claude/agents/vector-spec-refiner.md \
  .claude/agents/vector-spec-validator.md \
  .claude/agents/vector-standup-writer.md \
  .claude/commands/vector/apply.md \
  .claude/commands/vector/archive.md \
  .claude/commands/vector/close.md \
  .claude/commands/vector/propose.md \
  .claude/commands/vector/raw.md \
  .claude/commands/vector/standup.md \
  .claude/commands/vector/status.md \
  .claude/commands/vector/sync.md
```

Los 3 agentes y 3 commands que ya no están rastreados (vector-bug-refiner.md,
vector-comment-evaluator.md, vector-summary-writer.md, bug.md, comment.md, link.md) son
evidencia de que `vector update` ya fue corrido en algún momento; no requieren `git rm`.

### Detalle por archivo

#### `.gitignore` — MODIFICAR

Acción: agregar al final una sección nueva con título explicativo:

```
# Kit assets seeded by `vector update` — edit in kit/, not here.
.claude/agents/
.claude/commands/vector/
```

Verificar que `cli/internal/scaffold/assets/` ya está cubierto por alguna regla existente o
agregarlo de forma explícita. **No** agregar `.claude/` completo: el directorio tiene
contenido versionado legítimo (CLAUDE.md, rules/, projects/, vector/spec-template.md).

Restricciones:
- No excluir `.claude/CLAUDE.md`, `.claude/rules/`, `.claude/projects/` ni
  `.claude/vector/`.
- No eliminar reglas existentes.
- Verificar con `git check-ignore -v .claude/agents/vector-standup-writer.md` antes de
  commitear.

#### `cli/internal/scaffold/scaffold.go` — MODIFICAR

Acción: reescribir el comentario de paquete (líneas 1–11) para incluir el flujo de
single-source. El `//go:generate`, el `//go:embed` y toda la lógica permanecen intactos.

Cambios requeridos:
- Añadir al comentario de paquete los 4 pasos del flujo: (1) editar en `kit/`, (2)
  `go generate ./internal/scaffold`, (3) reinstalar binario, (4) `vector update` en el repo.
- Aclarar que `assets/` es una copia vendorizada **generada** y que nunca debe editarse
  directamente.

Restricciones:
- No cambiar ninguna línea de código; solo el bloque de comentario de paquete (block comment
  antes de `package scaffold`).
- No agregar ni quitar imports.
- No modificar `SeedCommands`, `writeSeed`, `CommandPaths` ni las constantes.

#### `.claude/rules/architecture/distribution-packaging.md` — MODIFICAR

Cambios requeridos:
- Agregar una subsección "Flujo de edición single-source (kit → binario → .claude/)" en
  la sección "Implicaciones para el desarrollo" con los 4 pasos canónicos y la nota de que
  `assets/` es generado (no editar a mano) y `.claude/` es gestionado por `vector update`
  (no editar a mano ni rastrear en git).
- Actualizar el marcador `> Estado: pendiente` al final si el flujo de embed ya está activo.

Restricciones:
- No cambiar los principios ni el encabezado de la rule.
- No duplicar contenido ya en `scaffold.go` o en la Memory; enlazar.

#### `.claude/projects/-Users-mariocampbell-Developer-vector/memory/MEMORY.md` — MODIFICAR

Cambios requeridos:
- En la entrada de reinstalación, añadir el paso 4 explícito: "después de reinstalar el
  binario, correr `vector update` en la raíz del repo para actualizar `.claude/agents/` y
  `.claude/commands/vector/`".

Restricciones:
- No eliminar el enlace al archivo de detalle de la Memory.
- No reformatear entradas no relacionadas.

#### `.github/workflows/ci.yml` — NUEVO (TBD)

Aplica solo si se decide activar CI — ver Open questions §1.

Debe implementar:
- Job `scaffold-drift-check`: checkout del repo, instalar Go 1.26, correr
  `go generate ./internal/scaffold` desde `cli/`, verificar con
  `git diff --exit-code cli/internal/scaffold/assets/` que no hay drift.
- Trigger: `push` a `main` y `pull_request` (todas las ramas).

No debe incluir:
- Build del binario completo, tests de web, deploy, ni release.

---

## 7. API Contract

No aplica — esta fase no introduce ni modifica ningún endpoint HTTP. La interfaz relevante es
la CLI del binario (`vector update`) y la directiva `//go:generate`, ambas ya existentes y no
modificadas en su comportamiento.

---

## 8. Criterios de éxito

- [ ] `git ls-files .claude/agents/ .claude/commands/vector/` devuelve cero líneas en el
  repo Vector (ningún archivo de esas rutas está rastreado en git).
- [ ] `.gitignore` tiene reglas para `.claude/agents/` y `.claude/commands/vector/`.
- [ ] `git check-ignore -v .claude/agents/vector-standup-writer.md` imprime la regla que lo
  excluye (no "no output").
- [ ] `go generate ./internal/scaffold` en `cli/` produce `assets/` idéntico al commiteado
  (`git diff --exit-code cli/internal/scaffold/assets/` sale con código 0 tras regenerar).
- [ ] `vector update` en la raíz del repo Vector siembra `.claude/agents/` y
  `.claude/commands/vector/` con el contenido corriente de `kit/` (verificar con `diff -r`).
- [ ] `cli/internal/scaffold/scaffold.go` documenta el flujo single-source en el comentario
  de paquete.
- [ ] `.claude/rules/architecture/distribution-packaging.md` documenta el flujo de edición.
- [ ] La Memory incluye `vector update` como paso obligatorio tras reinstalar.
- [ ] Todos los tests de `cli/internal/scaffold/` pasan (`go -C cli test ./internal/scaffold/...`).
- [ ] No hay regresiones en el resto del CLI (`go -C cli test ./...`).

### Tests requeridos

- [x] `TestSeedCommandsCreatesUnderClaude` — ya existente; debe seguir pasando.
- [x] `TestSeedCommandsSkipsExistingByDefault` — ya existente; debe seguir pasando.
- [x] `TestSeedCommandsForceOverwrites` — ya existente; debe seguir pasando.
- [x] `TestSeedCommandsDryRunWritesNothing` — ya existente; debe seguir pasando.
- [x] `TestSeedCommandsSeedsBugCommandAndRefiner` — ya existente; debe seguir pasando.
- [x] `TestCommandPathsNonEmpty` — ya existente; guarda que el embed no está vacío.
- [ ] **NUEVO**: `TestAssetsMatchKit` — verifica que el contenido de `assets/` bajo
  `embed.FS` coincide byte a byte con los archivos de `kit/` correspondientes. Este test
  fallará si alguien edita `assets/` a mano o si olvida correr `go generate` antes de
  commitear. Implementar con `os.ReadFile` + comparación contra `assets.ReadFile`. Ver
  Open questions §2.

### Comandos de verificación

```bash
# Desde la raíz del repo
go generate ./internal/scaffold   # desde cli/
git diff --exit-code cli/internal/scaffold/assets/   # debe ser vacío
go -C cli test ./...
gofmt -l cli/internal/scaffold/scaffold.go           # debe estar vacío
go -C cli vet ./internal/scaffold/...
git ls-files .claude/agents/ .claude/commands/vector/ # debe estar vacío
```

---

## 9. Criterios de UX

Aplica al flujo de desarrollo (no a UI web):

- **Un solo punto de edición**: cualquier dev que edite `kit/` y siga los pasos canónicos
  no verá inconsistencias entre copies. El mensaje en `scaffold.go` y en la rule debe ser
  suficientemente claro para no requerir búsqueda adicional.
- **Error temprano**: si alguien edita `assets/` a mano (bypass del flujo), el test
  `TestAssetsMatchKit` fallará antes del merge, no en producción.
- **Sin fricción extra**: el flujo ya exigía reinstalar el binario (Memory existente); se
  agrega solo un paso (`vector update`), que ya existe y ya funciona.
- **Diagnóstico accionable**: si el check de drift CI falla, el mensaje debe indicar exactamente
  qué archivos difieren y cómo corregir (`go generate ./internal/scaffold`).
- **Acceso sin CI**: en ausencia de CI (TBD), el test `TestAssetsMatchKit` cubre el mismo
  contrato localmente.

---

## 10. Decisiones tomadas

- **`kit/` = única fuente editable.** `assets/` es copia generada; `.claude/` es copia
  sembrada por el binario. Ninguna de las dos debe editarse a mano. *Por qué:* cualquier otra
  política requiere sync manual con riesgo de drift; la directiva `//go:generate` ya
  implementa el mecanismo.
- **`git rm --cached` para los 11 archivos de `.claude/`.** Se desrastrean del índice sin
  borrarlos del disco, y luego quedan cubiertos por `.gitignore`. *Por qué:* git tracked +
  gitignored no funciona bien; hay que desrastrear primero.
- **No gitignorear `.claude/` completo.** Solo `.claude/agents/` y `.claude/commands/vector/`
  quedan excluidos; el resto (rules, projects, CLAUDE.md, vector/) sigue versionado. *Por qué:*
  el resto es contexto de instrucciones legítimamente versionado.
- **El test `TestAssetsMatchKit` es el gate local de drift.** Fallará si se olvida
  `go generate`. CI (cuando exista) corre el mismo check a nivel de repositorio. *Por qué:*
  complementa; no duplica.
- **No se modifica la lógica de `SeedCommands`.** Ya es correcta; la fase es de proceso,
  no de funcionalidad.

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación; no la
implementa.

---

## 11. Edge cases

### `assets/` editado a mano

- `TestAssetsMatchKit` falla con mensaje que indica el archivo diferente.
- En CI (TBD): el drift check falla con `git diff --exit-code` y el mensaje indica qué
  regenerar.
- Solución: correr `go generate ./internal/scaffold` y recommitear.

### `go generate` corrido pero binario no reinstalado

- `.claude/` no se actualiza hasta que `vector update` se corra con el binario nuevo.
- El test `TestAssetsMatchKit` pasa (assets correcto); la discrepancia es solo en `.claude/`
  local, que no está rastreado en git → no bloquea el CI.
- La Memory actualizada documenta que el orden importa.

### `vector update` corrido antes del `go generate`

- El binario instalado contiene la versión vieja de `assets/`; siembra en `.claude/` la
  versión vieja. No es un bug de datos persistidos, solo una inconsistencia local temporal.
- Solución: seguir el orden documentado: (1) generate, (2) reinstalar, (3) update.

### Archivos de `.claude/` no rastreados que ya existen en disco

- Los 3 agentes y 3 commands que ya no estaban rastreados (vector-bug-refiner.md,
  vector-comment-evaluator.md, vector-summary-writer.md, bug.md, comment.md, link.md)
  ya son "libres" — no requieren `git rm --cached`. El `.gitignore` los excluye
  automáticamente tras agregarse las reglas.

### Otro desarrollador clona el repo y no tiene el binario instalado

- `.claude/agents/` y `.claude/commands/vector/` no existen en disco (están gitignoreados
  y el dev no corrió `vector update`).
- El repo funciona normalmente para desarrollo de Go y web. El dogfooding local requiere
  que el dev instale el binario y corra `vector update`.
- El README o el CLAUDE.md del repo deben guiar al dev nuevamente a `vector init` como
  primer paso (ya documentado; no es nuevo).

### `//go:generate` produce assets vacíos (kit/ borrado por error)

- `TestCommandPathsNonEmpty` ya falla con: `"no embedded commands — go generate vendoring is broken"`.
- No hay regresión adicional por esta fase.

### `.gitignore` agrega `.claude/agents/` pero quedan archivos tracked

- Si `git rm --cached` no se corrió antes de commitear `.gitignore`, `git status` seguirá
  mostrando los 11 archivos como modificados. La regla de `.gitignore` no aplica a archivos
  ya rastreados.
- Solución: correr `git rm --cached` antes del commit de `.gitignore`.

### Sin conexión / sin CI

- No aplica — esta fase es local-only y no requiere red.

---

## 12. Estados de UI requeridos

No aplica a UI web. Los estados relevantes son del flujo de desarrollo:

| Estado | Qué ocurre | Qué puede hacer el dev |
|---|---|---|
| idle | Nada pendiente; `assets/` y `kit/` en sync | Editar en `kit/` |
| generate | Dev corre `go generate ./internal/scaffold` | Esperar (~1s) |
| drift (error) | `assets/` difiere de `kit/`; test `TestAssetsMatchKit` falla | Correr `go generate` y recommitear |
| install | Dev reinstala el binario | Esperar |
| update | Dev corre `vector update` en el repo | Esperar; `.claude/` se actualiza |
| success | `.claude/`, `assets/` y `kit/` en sync; todos los tests pasan | Commitear; abrir PR |
| ci-fail (TBD) | CI detecta drift en `assets/`; PR bloqueada | Correr `go generate` localmente y pushear |

---

## 13. Validaciones

### Validaciones de proceso (check en test)

| Invariante | Regla | Consecuencia si falla |
|---|---|---|
| `assets/` == `kit/` (byte-a-byte) | `TestAssetsMatchKit` lee ambos y compara | Test rojo; no mergear |
| `assets/` no vacío | `TestCommandPathsNonEmpty` | Test rojo; `go generate` está roto |
| Todos los tests de scaffold pasan | `go -C cli test ./internal/scaffold/...` | Gate de calidad existente |

### Validaciones de repo (git)

| Invariante | Verificación | Corrección |
|---|---|---|
| `.claude/agents/` no rastreado | `git ls-files .claude/agents/` devuelve vacío | `git rm --cached` + commitear |
| `.claude/commands/vector/` no rastreado | `git ls-files .claude/commands/vector/` devuelve vacío | `git rm --cached` + commitear |
| `.gitignore` cubre ambas rutas | `git check-ignore -v .claude/agents/x.md` imprime regla | Agregar reglas al `.gitignore` |

### Validaciones de CI (TBD — ver Open questions §1)

| Invariante | Check | Error esperado |
|---|---|---|
| `assets/` en sync tras `go generate` en rama fresca | `git diff --exit-code cli/internal/scaffold/assets/` | Exit 1 con diff del archivo modificado manualmente |

---

## 14. Seguridad y permisos

- No aplica de forma especial — esta fase opera sobre archivos de texto del propio repo Vector,
  no sobre el repo del usuario.
- `SeedCommands` respeta `SeedOptions.Force = false` por defecto: no sobreescribe archivos
  del usuario sin permiso explícito. El comportamiento no cambia.
- El `git rm --cached` desrastreada archivos sin borrarlos del disco: es operación reversible
  (con `git checkout <sha> -- <ruta>` si fuera necesario).
- No se exponen secrets ni tokens; la phase es puramente de archivos de texto.

---

## 15. Observabilidad y logging

- `go generate ./internal/scaffold` no produce output en éxito; produce el diff de `cp` en
  error de permisos (ya comportamiento del shell).
- `TestAssetsMatchKit` (nuevo) emite `t.Errorf` con el nombre del archivo que difiere y
  la acción correctiva (`"run go generate ./internal/scaffold"`).
- CI (TBD): `git diff --exit-code` imprime el diff completo a stdout; el runner de CI lo
  captura como log del step.
- `vector update` ya emite las rutas que sembró / sobreescribió; sin cambios.

No registrar: contenido de los archivos de kit (podrían ser largos y no aportan diagnóstico
adicional al nombre del archivo diferente).

---

## 16. i18n / textos visibles

No aplica — esta fase no introduce textos visibles para el usuario final. Los únicos textos
nuevos son:

- Comentario de paquete en `scaffold.go` (inglés, código interno).
- Mensaje de test `TestAssetsMatchKit` (inglés, solo visible en test output).
- Check CI step name (inglés, solo visible en CI log).
- Entradas en `.gitignore` (comentarios en inglés, consistente con el archivo existente).
- Actualización de la rule `.claude/rules/architecture/distribution-packaging.md` (español,
  consistente con el resto de la rule).
- Actualización de la nota de Memory (inglés, consistente con el archivo existente).

---

## 17. Performance

- `go generate ./internal/scaffold`: copia shell de ~17 archivos de texto (~50 KB total).
  Tiempo estimado < 100ms en cualquier máquina.
- `TestAssetsMatchKit`: comparación de bytes en memoria; sin I/O de red; < 10ms.
- CI drift check: `go generate` + `git diff` en un runner limpio — el cuello de botella es
  la instalación de Go, no el check. Sin impacto en el tiempo de build del binario.
- `vector update` ya existente: sin cambios de performance.

---

## 18. Restricciones

El agente no debe:

- Modificar la lógica de `SeedCommands`, `writeSeed`, `CommandPaths`, `writeFileAtomic` ni
  las constantes `ActionCreated`/`ActionOverwritten`/`ActionSkipped`.
- Cambiar el `//go:generate` ni el `//go:embed` (ya correctos).
- Editar ningún archivo bajo `kit/` ni bajo `cli/internal/scaffold/assets/` (son
  source y copia generada respectivamente; la fase no toca contenido).
- Agregar dependencias externas a `cli/go.mod`.
- Gitignorear `.claude/CLAUDE.md`, `.claude/rules/`, `.claude/projects/`, `.claude/vector/`
  ni ninguna otra ruta de `.claude/` fuera de `agents/` y `commands/vector/`.
- Borrar del disco archivos de `.claude/` (solo desrastrear con `git rm --cached`).
- Cambiar el comportamiento de `vector init` o `vector update` (sin modificar el código;
  si la semántica de update ya es correcta, no hay nada que cambiar).
- Crear carpetas nuevas fuera del scope definido.
- Crear documentación genérica no solicitada.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `.gitignore` con reglas para `.claude/agents/` y `.claude/commands/vector/`.
- [ ] 11 archivos desrastreados de git con `git rm --cached` (y presentes en disco).
- [ ] `cli/internal/scaffold/scaffold.go` con comentario de paquete actualizado.
- [ ] `.claude/rules/architecture/distribution-packaging.md` con subsección del flujo
  single-source.
- [ ] Memory actualizada con el paso `vector update`.
- [ ] `TestAssetsMatchKit` implementado en `scaffold_test.go` y pasando.
- [ ] Todos los tests existentes de scaffold pasando.
- [ ] `go vet` sin warnings en `cli/internal/scaffold/`.
- [ ] `gofmt` limpio en `scaffold.go` y `scaffold_test.go`.
- [ ] `.github/workflows/ci.yml` con drift check (TBD — ver Open questions §1).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Verifiqué el estado actual con `git ls-files .claude/agents/ .claude/commands/vector/`
  y confirmo 11 archivos rastreados.
- [ ] Verifiqué que `diff -r kit/agents/ cli/internal/scaffold/assets/agents/` y
  `diff -r kit/commands/vector/ cli/internal/scaffold/assets/commands/vector/` no producen
  salida (copies en sync antes de la fase).
- [ ] Corrí `git rm --cached` para los 11 archivos listados en §6 antes de modificar
  `.gitignore`.
- [ ] Agregué las reglas a `.gitignore` y verifiqué con `git check-ignore -v`.
- [ ] Actualicé solo el comentario de paquete de `scaffold.go` sin tocar código.
- [ ] Implementé `TestAssetsMatchKit` en `scaffold_test.go` y lo corrí.
- [ ] Actualicé la rule `distribution-packaging.md` y la Memory.
- [ ] Corrí `go -C cli test ./internal/scaffold/...` — todos los tests pasan.
- [ ] Corrí `go -C cli test ./...` — sin regresiones.
- [ ] Corrí `gofmt -l cli/internal/scaffold/` — output vacío.
- [ ] Corrí `go -C cli vet ./internal/scaffold/...` — sin warnings.
- [ ] Verifiqué que `git ls-files .claude/agents/ .claude/commands/vector/` devuelve vacío.
- [ ] No toqué ningún archivo fuera del scope declarado sin justificación explícita.
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar.
- [ ] CI workflow creado o marcado como TBD con la razón documentada en Open questions.

---

## Open questions

1. **CI**: el repo no tiene `.github/workflows/`. ¿Se activa GitHub Actions como parte de
   esta fase o se deja el check de drift como gate solo de test local (`TestAssetsMatchKit`)?
   TBD — requiere decisión del usuario. Si CI se activa, el workflow del §6 es el entregable;
   si no, `TestAssetsMatchKit` es suficiente.

2. **`TestAssetsMatchKit` con `os.ReadFile`**: el test necesita acceder a `kit/` como ruta
   relativa desde el paquete `scaffold`. La ruta desde `cli/internal/scaffold/` hacia
   `kit/` es `../../../kit/`. Confirmar que el test funciona con `go test ./internal/scaffold/`
   corrido desde `cli/` (el working directory de `go test` es el directorio del paquete, así
   que `../../../kit/` es válido). Si hay problemas con rutas relativas en test, alternativa:
   usar `testdata/` con symlinks (pero el `//go:generate` ya hace el sync; symlinks complican).
   TBD — verificar al implementar.

3. **`cli/internal/scaffold/assets/` en `.gitignore`**: actualmente no aparece en `.gitignore`.
   Como es una copia generada commiteada (necesaria para el embed), puede ser correcto que
   esté tracked. Sin embargo, si el CI check falla cuando `assets/` diverge, tener `assets/`
   tracked es el mecanismo correcto (el CI compara contra lo commiteado). No gitignorear
   `assets/` — dejar como está. TBD — confirmar que el criterio es "assets/ commiteado = snapshot
   del último `go generate` corrido", lo cual es el patrón estándar de Go embed vendoring.
