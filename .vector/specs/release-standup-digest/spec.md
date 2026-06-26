# Spec: Cierre de standup-digest — embed, reinstall y UAT

## 1. Objetivo

Construir el **cierre/release** de la feature `add-standup-digest`: re-embeber el frontend
buildado en el binario Go, recompilar e reinstalar el binario global `vector`, y ejecutar un
**UAT manual exhaustivo** que confirme que la feature funciona end-to-end para el usuario final.

Esta fase permite que un **dev/maintainer** pueda cerrar el ticket `add-standup-digest` (hoy en
`review`) con la garantía de que `/vector:standup` corre completo (proyección → digest Haiku →
persistencia → board) y que la StandupView/SpecTimeline viajan dentro del binario instalado en
el `PATH`. **No** re-implementa la feature: solo empaqueta, instala y verifica.

## 2. Alcance

### Incluido en esta fase

- **Build del frontend**: `npm --prefix web run build` (Vite → `web/dist`).
- **Re-embed**: copiar `web/dist` a `cli/internal/webui/dist/` (la fuente de `//go:embed all:dist`),
  sobre-escribiendo el build anterior, y **commitearlo** (binario + assets versionados juntos).
- **Sync de assets del kit**: `go -C cli generate ./internal/scaffold` (vendoriza
  `kit/{commands,agents,vector}` en los assets embebidos; el embed de `webui` no requiere generate).
- **Recompilar e reinstalar** el binario: `go -C cli build -o ~/.local/bin/vector ./cmd/vector`
  (reemplaza el binario stale del `PATH`, que no tiene `standup`/`worklog`).
- **UAT manual exhaustivo** (criterio de cierre): `vector serve` + board web, flujo completo de
  `/vector:standup`, los edge cases definidos, los estados de UI, y check de no-regresiones.
- **Gate de calidad**: `go vet`, `go test`, `npm typecheck`, `npm build` verdes.
- **Documentar el UAT** en `docs/uat.md` (ya existe) y reflejar el estado en `docs/status.md`.

### Fuera de scope

- **Re-implementar o refactorizar** `add-standup-digest` (ya implementada y gate-verde en
  `feat/board-panel-and-apply`).
- **Abrir PR y mergear** `feat/board-panel-and-apply` a `main` — la rama trae 8 commits (board
  panel, apply, standup…); su merge es un paso aparte, no este cierre (decisión del usuario).
- **Automatizar** el pipeline build→embed→install (`install.sh` / release tooling) — fase futura.
- **Features futuras del standup**: `/vector:daily`, exportación externa, plantillas, burndown.
- **Cambiar el contrato de API**, la máquina de estados o cualquier endpoint/subcomando existente.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- **Binario CLI**: Go (módulo único en `cli/`, stdlib-only). Sirve API HTTP + SPA embebida.
- **Frontend embebido**: React 19 + Vite + TypeScript (`web/`), output `web/dist`, embebido vía
  `embed.FS` en `cli/internal/webui` (`//go:embed all:dist`, `cli/internal/webui/webui.go:19`).
- **Package manager web**: npm (`web/package.json`, scripts `build`/`typecheck`).
- **Agente de digest**: `kit/agents/vector-standup-writer.md`, tier **Haiku** (token-routing).

### Versiones relevantes

- Go: **1.26** (verificado en `cli/go.mod`; toolchain local `go1.26.1`).
- React/Vite: la versión exacta vive en `web/package.json` (no se altera en esta fase).

### Patrones existentes a respetar

- **Distribution-packaging** (`architecture/distribution-packaging.md`): el build de `web/` es
  **etapa previa** al build de `cli/`; versionar binario + assets embebidos juntos para evitar
  drift entre API y frontend. Un solo binario, instalación de un paso.
- **Embed dev/release** (`cli/internal/webui/webui.go`): en dev (`-dev`), `vector serve` puede
  servir un `web/dist` fresco de disco si difiere del embebido; el binario de release sirve el
  embebido. El cierre produce el embebido de release.
- **CLI-owns-writes**: el binario es el único escritor de `.vector/`; el UAT no edita estado a mano.
- **Gate de calidad** (`quality/testing-and-review.md`): no mergear con tests rojos; build de
  `web/` exitoso es necesario para el embed.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Feature `add-standup-digest` implementada y en `review` (commit `1ee8a26`, rama
      `feat/board-panel-and-apply`): paquete `cli/internal/standup`, `cli/cmd/vector/standup.go`,
      handlers `/api/standup`+`/api/activity` (`cli/internal/board/server.go`), command
      `kit/commands/vector/standup.md`, agente `kit/agents/vector-standup-writer.md`, UI
      `web/src/components/StandupView/`+`SpecTimeline/`, hook `web/src/api/useStandup.ts` — verificado.
- [x] Embed de la SPA en `cli/internal/webui` (`webui.go` con `//go:embed all:dist` +
      `cli/internal/webui/dist/index.html` committed) — verificado.
- [x] Gate de la feature verde (gofmt/vet/test Go, typecheck/build web) — verificado en el apply.
- [x] `.vector/` del repo poblado con specs + `activity.jsonl` (incluye `work.logged` de dogfood)
      para tener actividad real que proyectar en el UAT — verificado.
- [x] Toolchain: Go `1.26.x` y npm disponibles en la máquina — verificado (`go1.26.1`).
- [x] `docs/uat.md` y `docs/status.md` existen (destino de documentación) — verificado.

Si alguna dependencia no existe, el agente se detiene y reporta exactamente qué falta. No inventa
contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

**Embed en build-time + verificación funcional.** El frontend se compila a assets estáticos que
se incrustan en el binario Go (`embed.FS`); el flujo de release es **secuencial** (web → assets →
CLI) para garantizar sincronización API↔UI. El cierre se completa con un UAT manual sobre el
binario instalado (no hay código nuevo que diseñar).

### Capas afectadas

- **presentation (`web/`)**: sí — solo **build** (no se modifica código); se verifica visualmente.
- **application/use-cases (`cli/cmd/vector`, `kit/`)**: sí — solo **verificación** de que
  `vector standup`/`worklog`/`serve` y `/vector:standup`/`/vector:apply` corren sin error.
- **domain (`cli/internal/standup`, `cli/internal/state`)**: no — sin cambios; cubierto por tests.
- **data/infrastructure (`cli/internal/webui`)**: sí — el `dist/` embebido se reemplaza por el
  build real y se commitea.
- **shared/common**: no.

### Flujo esperado (release)

1. Build del frontend: `npm --prefix web run build` → `web/dist/` con assets versionados.
2. Re-embed: copiar `web/dist/*` a `cli/internal/webui/dist/` (sobre-escribe el build anterior).
3. Sync assets del kit: `go -C cli generate ./internal/scaffold` (idempotente; `webui` no usa generate).
4. Compilar binario: `go -C cli build -o ~/.local/bin/vector ./cmd/vector` (con la SPA embebida).
5. Verificar instalación: `~/.local/bin/vector version` responde con el binario nuevo.

### Flujo esperado (UAT manual exhaustivo)

1. `vector serve` (API HTTP + SSE + SPA embebida) levanta sin error.
2. Abrir el board en el navegador → cargar → confirmar el **tab Standup** y el **SpecTimeline**
   expandible por card.
3. Registrar actividad (si falta): `/vector:apply` sobre algún spec → emite `work.logged`.
4. Ejecutar `/vector:standup` (default desde el marcador; y con `--since 24h`/`today`/`7d`):
   proyecta → agente Haiku genera digest → `vector standup commit` persiste → marcador avanza →
   `GET /api/standup` retorna el digest → board lo refleja vía SSE.
5. Edge cases (ver §11) y estados de UI (ver §12).
6. Check de no-regresiones (ver §8).

### Ubicación de archivos nuevos

No hay archivos de código nuevos. El único artefacto que cambia en el repo es el contenido de
`cli/internal/webui/dist/` (assets embebidos) y, opcionalmente, la documentación de `docs/`.

```txt
cli/internal/webui/dist/   # reemplazado por el build real de web/dist (committed)
docs/uat.md                # registro de la sesión de UAT (append)
docs/status.md             # estado post-release (update menor)
```

No crear carpetas nuevas; reutilizar la convención existente.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `cli/internal/webui/dist/` (index.html + assets/) | MODIFICAR | Reemplazar el build embebido por el build real de `web/dist` (StandupView/SpecTimeline incluidas) y commitearlo | `cli/internal/webui/webui.go` (`//go:embed all:dist`) |
| `docs/uat.md` | MODIFICAR | Añadir la sección/checklist de UAT de standup-digest con resultados | `docs/uat.md` (estructura existente) |
| `docs/status.md` | MODIFICAR | Reflejar el estado post-cierre (binario reinstalado con standup; ticket en review→listo para close) | `docs/status.md` (sección de binario/recompilación) |
| `~/.local/bin/vector` | REEMPLAZAR | Binario global recompilado con el embed actualizado y los subcomandos `standup`/`worklog` (fuera del repo) | `docs/status.md:18-21` (comando de recompilación) |

### Detalle por archivo

#### `cli/internal/webui/dist/`

Acción: MODIFICAR

Debe contener:

- El output de `npm --prefix web run build` (`index.html` + `assets/*.js`/`*.css` hasheados),
  incluyendo la StandupView, SpecTimeline y el switch de vista del board.

Debe seguir como referencia:

- `cli/internal/webui/webui.go` (el embed lee `dist/` directamente; no requiere `go generate`).

No debe incluir:

- Código fuente de `web/src` ni artefactos de dev (solo el build de producción).

#### `docs/uat.md`

Acción: MODIFICAR

Cambios requeridos:

- Añadir una sección "Standup digest — UAT" con el checklist de §8 (happy path, edge cases,
  estados de UI, no-regresiones) y el resultado (verde/rojo) de cada ítem.

Restricciones:

- No reescribir el UAT de features previas; solo **agregar** la sección de standup.

#### `docs/status.md`

Acción: MODIFICAR

Cambios requeridos:

- Anotar que el binario en `~/.local/bin/vector` quedó recompilado con soporte de standup
  (subcomandos + UI embebida) y que `add-standup-digest` quedó verificado (listo para `/vector:close`).

Restricciones:

- No tocar otras secciones de estado no relacionadas con este cierre.

#### `~/.local/bin/vector`

Acción: REEMPLAZAR

Debe producirse con:

- `go -C cli build -o ~/.local/bin/vector ./cmd/vector` (tras el re-embed y el sync de assets del
  kit `go -C cli generate ./internal/scaffold`), generando el binario con la SPA embebida y los
  subcomandos `standup`/`worklog`. Verificar con `~/.local/bin/vector version`.

Restricciones:

- Es un artefacto **fuera del repo** (en el `PATH` del usuario); no confundirlo con ningún
  artefacto committed. No se versiona en git; solo se reinstala.

---

## 7. API Contract

Vector no usa `docs/api-contract.md`; el contrato `cli/ ↔ web/` vive en `cli/internal/board/*.go`
y se espeja a mano en `web/src/types/`. **Esta fase no cambia el contrato.**

### Endpoints involucrados (ya existentes, solo se verifican)

- `GET /api/standup` → digest persistido del último standup (`{}` si nunca se corrió).
- `GET /api/activity?spec=<id>&since=<24h|today|7d>` → timeline proyectada del spec.
- `GET /api/board` y `GET /api/events` (SSE) → board + frescura (no deben regresar).

Subcomandos del binario (ya existentes): `vector standup [--since …] [--json]`,
`vector standup commit --digest-file <path|->`, `vector spec worklog <id> [--files …] [--tasks …] [--note …]`.

Detalle del contrato: `docs/domain-contract.md` §4 (endpoints) y §5 (mapa comando→escritura). No
inferir campos extra ni renombrar propiedades; los tipos TS en `web/src/types/standup.ts` ya los espejan.

---

## 8. Criterios de éxito

La fase se considera completa cuando:

- [ ] `npm --prefix web run build` completa sin errores (genera `web/dist`).
- [ ] `web/dist/*` copiado a `cli/internal/webui/dist/` (index.html + assets) y committed.
- [ ] `go -C cli generate ./internal/scaffold` corre sin errores (assets del kit sincronizados).
- [ ] `go -C cli build -o ~/.local/bin/vector ./cmd/vector` compila exitosamente.
- [ ] `~/.local/bin/vector version` responde con el binario nuevo (en `PATH`).
- [ ] **UAT visual**: `vector serve` + navegador → StandupView + SpecTimeline renderizan.
- [ ] **UAT flujo**: `/vector:standup` corre end-to-end (proyecta → digest Haiku → persiste →
      marcador avanza → `GET /api/standup` devuelve el digest → board lo refleja por SSE).
- [ ] **UAT edge cases** (ver §11): `--since` inválido, periodo sin actividad, digest json
      inválido que NO escribe ni avanza el marcador, `/api/activity` con spec inexistente (404),
      `?since=` inválido (400).
- [ ] **UAT estados de UI** (ver §12): loading, success, empty, error visibles.
- [ ] **No-regresiones**: `/vector:apply` invoca `worklog` sin error; `apply`/`status`/`close`,
      `/api/board`, `/api/events` (SSE) intactos.
- [ ] **Gate**: `go -C cli vet ./...`, `go -C cli test ./...`, `npm --prefix web run typecheck`,
      `npm --prefix web run build` pasan 100%.
- [ ] Sesión de UAT documentada en `docs/uat.md`; `docs/status.md` actualizado.

### Tests requeridos

No se escriben tests nuevos (la feature ya los trae). En esta fase se **ejecutan/verifican**:

- [ ] `go -C cli test ./...` (incluye `internal/standup`, `internal/state`, `internal/board`).
- [ ] `npm --prefix web run typecheck` (incluye los tipos de `standup.ts`).
- [ ] Al menos una corrida manual completa de `/vector:standup` end-to-end.

### Comandos de verificación

```bash
# Build del frontend + re-embed
npm --prefix web run build
cp -R web/dist/. cli/internal/webui/dist/

# Sync de assets del kit (webui no requiere generate)
go -C cli generate ./internal/scaffold

# Recompilar e instalar el binario
go -C cli build -o ~/.local/bin/vector ./cmd/vector
~/.local/bin/vector version

# Gate de calidad
go -C cli vet ./...
go -C cli test ./...
npm --prefix web run typecheck
npm --prefix web run build
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

> No aplica como diseño — la UX de StandupView, SpecTimeline y del output CLI de `/vector:standup`
> se definió e implementó en `add-standup-digest` §9. Aquí se **verifica** visualmente, no se
> diseña. No hay formularios, passwords ni inputs nuevos.

Verificación de comportamiento (UAT, no implementación):

### Loading

- El board muestra el estado **loading** ("loading standup…") mientras `GET /api/standup` resuelve;
  el SpecTimeline muestra "loading activity…" al expandirse.

### Formularios / Passwords

- No aplica — la feature no tiene formularios ni campos de password.

### Errores

- Error de `GET /api/standup`/`/api/activity` → banner "error loading standup/activity: <razón>"
  con botón **retry** (verificar disparando un fallo de fetch).
- CLI: `--since` inválido → mensaje accionable `invalid --since: use 24h, today or 7d`.

### Navegación

- El switch **Board ↔ Standup** alterna la vista sin romper el kanban; el SpecTimeline es lazy por
  card (no bloquea el resto del board).

### Accesibilidad

- Timeline como lista semántica; el toggle "Activity" expone `aria-expanded`; el status pill lleva
  label (no solo color). Confirmar contraste de la StandupView contra `docs/kanban-ui-reference.md`.

---

## 10. Decisiones tomadas

El agente no debe cuestionar ni cambiar estas decisiones (tomadas con el usuario):

- **El cierre abarca embed + reinstall + UAT** y **no** incluye el merge de la rama a `main`.
  *Por qué:* `feat/board-panel-and-apply` trae 8 commits más allá de standup-digest; su merge es
  un paso aparte que no debe acoplarse a este cierre.
- **UAT exhaustivo como criterio de cierre** (happy path + todos los edge cases + estados de UI +
  no-regresiones). *Por qué:* es la última verificación antes de `/vector:close`; un UAT rápido
  dejaría sin cubrir los edge cases que definen la corrección de la feature.
- **Re-embed manual del build + commit** (no automatización). *Por qué:* la regla
  distribution-packaging exige versionar binario+assets juntos; el pipeline automatizado es fase
  futura, no este cierre.
- **Reinstalar en `~/.local/bin/vector`** vía `go build -o` (método ya documentado en
  `docs/status.md`). *Por qué:* es el canal de instalación vigente del repo; `install.sh` es futuro.
- **No tocar código de la feature**. *Por qué:* ya está gate-verde y en review; este cierre solo
  empaqueta, instala y verifica.

Si el agente detecta una alternativa aparentemente mejor, la reporta como observación, no la implementa.

---

## 11. Edge cases

Estos ya están implementados en `add-standup-digest` §11; en esta fase se **verifican en UAT**:

### Datos inválidos

- `vector standup --since 36h` (o `?since=36h`) → error/`400` `invalid since: use 24h, today or 7d`;
  no proyecta.
- `vector spec worklog` sin `--id` o con spec inexistente → error accionable.

### API errors (API local, solo GET)

- `400`: `?since=` inválido → `{ "error": "invalid since: use 24h, today or 7d" }`.
- `404`: `/api/activity?spec=<inexistente>` → `{ "error": "spec '<id>' not found" }`.
- `500`: error de lectura del log → `{ "error": "could not read activity log" }`.
- **No aplican** `401/403/409/422/429`: binario local, sin auth ni mutación.

### Periodo sin actividad

- Sin eventos desde el marcador → digest `{}` / "no activity since last standup"; el marcador
  **igual avanza** al commitear (corrida válida; el historial se conserva en `activity.jsonl`).

### Respuesta vacía o inesperada

- `activity.jsonl` ausente/vacío → proyección vacía, StandupView **empty**, sin panic.
- Línea JSONL corrupta → se salta (log a stderr) y continúa, no aborta el resumen.

### Digest inválido (caso crítico del cierre)

- `vector standup commit --digest-file -` con **JSON inválido en stdin** → error `invalid digest
  json`; **no escribe el digest y NO avanza el marcador**. Verificar ambos (no-escritura y
  no-avance) explícitamente en UAT.

### Fetch lento / timeout en UI

> No aplica como diseño — heredado de `add-standup-digest` §11. Verificar en UAT que `useStandup`
> (`/api/standup`) y `useSpecActivity` (`/api/activity`) mantienen el estado **loading** mientras
> el fetch resuelve y pasan a **error** con **retry** ante fallo/timeout, sin bloquear el resto del
> board (timeline lazy por card).

### Doble submit / concurrencia

- Lectura del log es read-only (append-only ⇒ segura). La escritura del digest/marcador pasa por
  el mutex del `Store` (serializada). No aplica "doble submit" de formulario (no hay formularios).

---

## 12. Estados de UI requeridos

Verificar en UAT que cada estado renderiza (board web local; sin `disabled`/`offline` propios):

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | StandupView con el último digest persistido (o empty/CTA si nunca corrió) | abrir, expandir timelines |
| loading | spinner + "loading standup…" (y "loading activity…" en el timeline) | esperar |
| success | digest global + tarjetas por spec + timelines expandibles | leer, expandir, copiar |
| error | banner "error loading standup: <razón>" | reintentar (retry) |
| empty | "no activity since last standup" | correr `/vector:standup`, cambiar ventana |

`disabled`/`offline`: No aplica — el board es local y efímero.

---

## 13. Validaciones

> No aplica como diseño — no hay inputs nuevos. Las validaciones ya viven en
> `cli/internal/standup`/`cli/cmd/vector` y se **verifican** en UAT.

### Validaciones de cliente (CLI + web) — a verificar

| Campo | Regla | Mensaje |
|---|---|---|
| `--since` / `?since=` | `24h` \| `today` \| `7d` (o vacío → marcador) | `invalid --since: use 24h, today or 7d` |
| `worklog --id` | kebab-case, spec existente | `spec '<id>' not found` |
| `standup commit --digest-file` | `-` (stdin) o path legible; contenido = JSON válido | `invalid digest json` / `cannot read digest file` |

### Validaciones de servidor

No aplica — no hay backend remoto; la API local valida los query params arriba. La validación de
dominio (estado/transición) vive en `cli/internal/state` y no cambia aquí.

---

## 14. Seguridad y permisos

- `activity.jsonl` y `.vector/local/standup.json` son **personales y gitignored**; el cierre **no**
  los commitea. Verificar en UAT que `standup.json` queda bajo `.vector/local/` y no aparece en `git status`.
- El binario embebe **solo assets de producción** de `web/dist`; no incluir `.env`, fuentes ni
  secrets en el embed. Revisar el contenido copiado a `cli/internal/webui/dist/`.
- El agente Haiku recibe **solo el JSON de la proyección** (no secrets); la API local es read-only
  (sin auth; 401/403 no aplican).
- No registrar payloads sensibles en el digest; `work.logged.note` es texto corto del dev.

---

## 15. Observabilidad y logging

- Reusar el mecanismo existente (stderr del binario; activity log append-only). Esta fase no añade
  logging nuevo.
- En UAT, observar stderr ante: líneas JSONL corruptas saltadas, errores de lectura del log.
- No registrar: secrets, tokens, PII, diffs completos.
- Confirmar que `vector serve` loguea la URL del board y la fuente de la UI (`embedded` tras el
  re-embed; no "stale" salvo que se sirva desde disco con `--web-dir`).

---

## 16. i18n / textos visibles

> Parcialmente no aplica — no se agregan textos nuevos. Vector no tiene sistema i18n; los labels
> de `web/` están en inglés hardcodeado (convención del repo) y se **verifican** en UAT.

| Key (label UI) | Texto (EN) |
|---|---|
| standup.title | `Standup` |
| standup.loading | `loading standup…` |
| standup.empty | `no activity since last standup` |
| standup.period | `since {date}` |
| standup.error | `error loading standup` |
| standup.retry | `retry` |
| timeline.header | `Activity` |
| timeline.more | `show more` |
| timeline.retry | `retry` |

El digest NL lo genera el agente Haiku en el idioma del usuario (ceremonia); los labels de UI
quedan en inglés. Verificar que renderizan correctamente tras el embed.

---

## 17. Performance

- El re-embed no debe inflar el bundle más allá del build normal de Vite (revisar el tamaño
  reportado por `npm run build`; hoy ~217 kB JS / ~15 kB CSS).
- `/api/standup` sirve el digest **ya persistido** (no regenera prosa por request); `/api/activity`
  proyecta on-demand. La generación NL (Haiku) corre solo en `/vector:standup`.
- En UAT, confirmar que abrir el board (`/api/board`, `/api/standup`, `/api/activity`) responde sin
  latencia perceptible con el `activity.jsonl` real del repo.
- No bloquear el board: el SpecTimeline es lazy por card (fetch solo al expandir).

---

## 18. Restricciones

El agente no debe:

- Modificar el código de la feature `add-standup-digest` (eventos, proyección, handlers, UI, kit).
- Cambiar el contrato de API (`/api/standup`, `/api/activity`), la máquina de estados ni el SSE.
- Hacer que el binario Go llame a un LLM (la prosa es del agente del command).
- Agregar dependencias externas (Go stdlib; libs de `web/` ya presentes).
- Commitear `.vector/local/` (activity log / standup.json) ni secrets en el embed.
- Mergear `feat/board-panel-and-apply` a `main` (fuera de scope de este cierre).
- Refactorizar código no relacionado ni cambiar estilos/navegación globales.
- Ignorar fallos de gate (vet/test/typecheck/build) ni edge cases en el UAT.

---

## 19. Entregables

Al finalizar deben quedar:

- [ ] `web/dist` buildado con todas las vistas (StandupView, SpecTimeline, board, meter).
- [ ] `cli/internal/webui/dist/` actualizado con el build real (committed) — no placeholder de dev.
- [ ] `~/.local/bin/vector` recompilado e instalado con la UI embebida y los subcomandos
      `standup`/`worklog`; `vector version` responde.
- [ ] Gate verde: `go vet`, `go test`, `npm typecheck`, `npm build`.
- [ ] UAT manual exhaustivo completado y registrado en `docs/uat.md`:
  - [ ] `vector serve` levanta; board carga con StandupView + SpecTimeline.
  - [ ] `/vector:standup` end-to-end (proyecta → digest → persiste → marcador avanza → board).
  - [ ] Edge cases verificados (since inválido, periodo vacío, digest json inválido sin avance de marcador, 404/400 de API).
  - [ ] Estados de UI verificados (loading/success/empty/error).
  - [ ] No-regresiones (`apply`/`status`/`close`, `/api/board`, `/api/events`).
- [ ] `docs/status.md` actualizado (binario con standup; ticket listo para `/vector:close`).
- [ ] `add-standup-digest` listo para cerrar con `/vector:close` (la transición la hace el usuario).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo (es cierre/release + UAT, no implementación de feature).
- [ ] Revisé la spec original `add-standup-digest` y `docs/domain-contract.md` §4/§5.
- [ ] Confirmé que los archivos de la feature están en `feat/board-panel-and-apply` (no en `main`).
- [ ] Ejecuté `npm --prefix web run build` sin errores.
- [ ] Copié `web/dist` a `cli/internal/webui/dist/` y verifiqué el contenido (index.html + assets).
- [ ] Ejecuté `go -C cli generate ./internal/scaffold` sin errores.
- [ ] Compilé e instalé: `go -C cli build -o ~/.local/bin/vector ./cmd/vector`; `vector version` ok.
- [ ] Ejecuté el gate completo (vet/test/typecheck/build), todo verde.
- [ ] Ejecuté el UAT exhaustivo: `vector serve` + board + `/vector:standup` end-to-end.
- [ ] Probé los edge cases (since inválido, periodo vacío, digest corrupto sin avance de marcador, 404/400).
- [ ] Verifiqué estados de UI (loading/success/empty/error) y no-regresiones.
- [ ] Confirmé que `.vector/local/` no se commitea y que el embed no incluye secrets.
- [ ] Documenté el UAT en `docs/uat.md` y actualicé `docs/status.md`.
- [ ] No dejé archivos temporales ni TODOs sin justificar.

---

## Open questions

- ¿La sesión de UAT requiere artefactos (screenshots/logs) además del checklist en `docs/uat.md`,
  o basta el checklist verde/rojo? (Asumido: checklist; ajustar si se pide evidencia visual.)
- ¿Umbral de performance formal para `/vector:standup` con `activity.jsonl` grande, o best-effort?
  (Asumido: best-effort; no hay benchmark definido.)
- Restauración del placeholder de `cli/internal/webui/dist/`: no se requiere — el embed sirve lo
  que haya en `dist/`; el build de release es el contenido válido a commitear.
