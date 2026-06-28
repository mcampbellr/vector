# Spec: Detectar bloqueo de dependencia externa en /vector:apply

## 1. Objetivo

Construir la orquestación dentro del command `/vector:apply` (`kit/commands/vector/apply.md`)
para que, **al terminar la implementación y antes de la transición final**, el agente detecte
señales de **bloqueo por dependencia externa que no puede resolver por sí mismo** y, si las hay,
transicione la card a `needs-attention` con una razón concreta **en lugar de** a `review`.

Esta feature permite que un **dev** cuya implementación compila y pasa la suite (mockeada) pero
queda gatillada por algo externo —credenciales de terceros, `api_names`/identificadores externos
sin confirmar, datos que debe otro equipo— obtenga una card en `needs-attention` con el motivo
explícito (qué falta + cómo se desbloquea + ref del PR abierto si aplica), en vez de una card
que finge estar lista para review y oculta un bloqueo de acción humana.

El caso motivador real (repo del usuario, no de Vector): `MH-1582` "prospect applications
endpoint" — implementación terminada, PR #367 abierto, build/lint/test verdes, pero los
`api_names` exactos de Zoho CRM quedaron como `TODO(MH-1582)` porque requieren credenciales de
lectura de settings; el run produjo un comentario pidiendo esas credenciales. La card fue a
`review` y hubo que moverla a `needs-attention` a mano. Este change automatiza esa corrección
en el punto de cierre de apply.

## 2. Alcance

### Incluido en esta fase

- **Sub-paso de detección en `apply.md` §6 (Finish), antes de transicionar**: el agente inspecciona
  los artefactos de su propio run (diff del working tree, `tasks.md`/items de aceptación, y los
  artefactos outbound que el run haya generado) y juzga si hay un **bloqueo de dependencia
  externa** según las heurísticas de §11. Cualquiera de las tres señales basta:
  1. El run dejó un `TODO(<ticket>)`/placeholder en código que **gobierna comportamiento de
     runtime** (no cosmético ni test-only) y que depende de un dato/credencial/identificador
     externo aún no provisto.
  2. El run produjo un **artefacto outbound cuyo propósito es pedir algo a un humano/otro equipo**
     (p. ej. "request credentials", "ask X for the api_names", un borrador de comentario de
     ticket pidiendo input).
  3. Un item de `tasks.md`/aceptación es satisfacible **solo contra mocks** y está marcado
     explícitamente como pendiente de dato/credencial real.
- **Guard mecánico de falso-positivo (determinista)**: se **ignoran** los `TODO`/`FIXME` en
  archivos test-only (`*_test.go`, `*.test.*`, dirs `test`/`tests`/`__tests__`) y los comentarios
  cosméticos (refactor/naming/typo), que **nunca** disparan `needs-attention`. (Un `TODO` que
  apunta deliberadamente a otra card/ticket ya trackeado se interpreta como deferral intencional
  y tampoco dispara — esto queda como juicio del agente, no como consulta a `.vector/specs/`.)
- **Routing automático e independiente de `applyMode`**: si se detecta bloqueo, el agente
  ejecuta `vector spec status <id> needs-attention --reason "<motivo>"` —es una salvaguarda de
  integridad del board, no una elección de workflow— en lugar de `vector spec status <id> review`.
  No se pide confirmación aunque `applyMode` sea `ask`/`always-ask`.
- **Razón concreta y accionable**: nombra **qué está pendiente** + **cómo/quién lo desbloquea** +
  **ref del PR abierto** si existe (p. ej. `Zoho CRM api_names pending settings-read credentials;
  unblock by providing creds to fill TODO(MH-1582); PR #367 open`).
- **Reporte de §7 actualizado**: cuando se ruteó a `needs-attention`, el reporte surfacea el
  bloqueo y el motivo en lugar de "ready for review".
- **Documentación de la heurística dentro del propio command** (`apply.md`) para que sea auditable.
- **Regeneración de la copia embebida** `cli/internal/scaffold/assets/commands/vector/apply.md`
  vía `go generate` (re-sembrada por `vector update` / `vector init --force`).

### Fuera de scope

- **Auto-resolver la dependencia externa**: no traer credenciales, no llamar a APIs de terceros,
  no confirmar `api_names`. Solo detectar y marcar.
- **Cambiar la máquina de estados**: `needs-attention` ya existe y es alcanzable; el binario ya
  soporta `--reason`. Este change es solo **CUÁNDO** apply elige `needs-attention` vs `review`.
- **Cualquier cambio en Go**: `runSpecStatus`, `SetStatus`, el struct `Attention` y la proyección
  de standup ya existen y se ejercen sin modificación.
- **Rescanear retroactivamente** cards ya `closed`/`archived`.
- **Lógica de "cubierto por otra card" basada en consultar `.vector/specs/`**: el guard mecánico
  es la exclusión test-only/cosmético; el reconocimiento de un deferral apuntado a otro ticket
  queda como juicio del agente.
- **Múltiples transiciones por run**: una sola transición y una sola razón (que puede enumerar
  los bloqueos si hay más de uno).
- **El hard-stop de §4** (bloqueo de ambigüedad que detiene la implementación) no se toca: este
  change cubre el caso en que la implementación **terminó** pero quedó gatillada por algo externo.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- `/vector:apply` es un **project command markdown** (`kit/commands/vector/apply.md`), no una
  skill ni código Go. El único artefacto que cambia sustancialmente es ese markdown (+ su copia
  embebida). El command lo ejecuta el modelo: la "detección" es **juicio del agente guiado por
  las heurísticas escritas en el command**, no regex hardcodeada en bash.
- Go (módulo único en `cli/`, stdlib): el lado binario ya está implementado y testeado; **no se
  toca**. El command ejerce `vector spec status <id> needs-attention --reason "…"`.
- Embed/scaffold: `cli/internal/scaffold` embebe `kit/{commands,agents,vector}` vía `embed.FS`;
  la copia en `cli/internal/scaffold/assets/commands/vector/apply.md` se regenera con
  `go generate` (directiva en `cli/internal/scaffold/scaffold.go`).

### Versiones relevantes

- Go: **1.26** (`cli/go.mod`) — se ejerce, no se compila lógica nueva.
- Interfaz binaria ya existente: `vector spec status <id> needs-attention --reason "<texto>"`
  (`cli/cmd/vector/spec_transitions.go:142`, `runSpecStatus`).
- Persistencia: `Attention{Reason string, Since time.Time, Source string}`
  (`cli/internal/state/types.go:134`); `Source` ∈ `{"hook","command"}`.

No usar librerías, APIs, flags o patrones que no estén documentados oficialmente o que no estén
ya presentes en el proyecto, salvo que este spec lo autorice explícitamente.

### Patrones existentes a respetar

- **CLI-owns-writes**: el command **nunca** edita `.vector/` a mano; solo construye la razón y la
  pasa a `vector spec status`. El binario valida la transición y persiste.
- **Estructura de pasos de `apply.md`** (§1 select, §2 start, §3 mode, §4 implement, §5 worklog,
  §6 finish, §7 report): el change se localiza en §6 y §7; el resto no cambia.
- **Markdown de los `/vector:*` en inglés** (convención de los project commands). La prosa de
  este spec va en español; el texto del command y los identificadores van en inglés.
- **Token routing** (`product/token-routing.md`): la detección es trabajo de bajo costo dentro
  del mismo run de implementación; no introduce un agente adicional.

---

## 4. Dependencias previas

Antes de iniciar esta fase ya existe (verificado en el repo):

- [x] `vector spec status <id> needs-attention --reason "<texto>"` operativo
      (`cli/cmd/vector/spec_transitions.go:142`; `--reason` parseado y gateado).
- [x] `SetStatus` escribe `Attention{Reason,Since,Source}` en `state.json`
      (`cli/internal/state/types.go:134`).
- [x] Proyección del motivo en standup: `TimelineEvent.Reason`
      (`cli/internal/standup/standup.go:138`) poblado desde `StatusChangedData.Reason`
      (`standup.go:159`). El board lee `needsAttention.reason` de `state.json`.
- [x] `apply.md` con §1–§7, incluido §6 "Finish — transition to review (or closed)" y el
      hard-stop de §4 que ya usa `needs-attention --reason`.
- [x] `applyMode` en `.vector/config.json` (`auto|ask|always-ask`), sin cambios necesarios.
- [x] Scaffold/embed con `go generate` para regenerar la copia del command.

No hay dependencias de estructura faltantes: surfacing en board y standup ya funciona en cuanto
se escribe el `reason`. El change es orquestación en el markdown. Si alguna de estas piezas no
existiera, el agente debe detenerse y reportar exactamente qué falta.

---

## 5. Arquitectura

### Patrón a usar

**Detección en el command (juicio del agente) + binario como único escritor de la transición.**
El command inspecciona los artefactos del run, juzga si hay bloqueo externo según las heurísticas,
construye una razón concreta, y delega la transición a `vector spec status`. No hay lógica de Go
nueva ni edición directa de estado.

### Capas afectadas

- presentation/command (`kit/commands/vector/apply.md`): **sí** — §6 (detección + transición
  condicional) y §7 (reporte del bloqueo).
- Go/CLI (`cli/cmd/vector`, `cli/internal/state`): **no** — se ejercen funciones existentes.
- scaffold (`cli/internal/scaffold/assets/commands/vector/apply.md`): **sí**, pero solo como
  copia regenerada por `go generate` (no edición manual).
- web/board y standup: **no** — la proyección de `needsAttention.reason` ya existe.

### Flujo esperado

1. La implementación termina (§4) y el worklog quedó registrado (§5).
2. **§6, antes de transicionar (sub-paso nuevo)** — el agente evalúa señales de bloqueo externo:
   - ¿Quedó un `TODO(<ticket>)`/placeholder que gobierna runtime y depende de un dato externo?
   - ¿El run generó un artefacto cuyo fin es pedir algo a un humano/otro equipo?
   - ¿Hay un item de `tasks.md`/aceptación satisfecho solo contra mocks y marcado pendiente de
     dato/credencial real?
   - Aplica el guard mecánico: **ignora** TODOs en archivos test-only y comentarios cosméticos.
3. **Si hay bloqueo** → construye razón concreta (qué falta + cómo se desbloquea + ref de PR) y
   ejecuta `vector spec status <id> needs-attention --reason "<razón>"` (automático, sin pedir
   confirmación). Salta a §7 (reporte de bloqueo).
4. **Si está limpio** → `vector spec status <id> review` (o lo deja para `/vector:close` cuando no
   hay nada que verificar), exactamente como hoy. Continúa a §7 (ready for review).

### Ubicación de archivos nuevos

No se crean archivos nuevos. Solo se modifican los dos markdown (fuente + copia embebida). No
crear carpetas ni helpers.

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/commands/vector/apply.md` | MODIFICAR | §6: añadir sub-paso de detección de bloqueo externo + transición condicional a `needs-attention`. §7: surfacear el motivo del bloqueo. Documentar la heurística para que sea auditable. | Patrón de orquestación de `kit/commands/vector/raw.md`; hard-stop existente en `apply.md` §4. |
| `cli/internal/scaffold/assets/commands/vector/apply.md` | REGENERAR | Copia embebida del command; se regenera con `go generate`, no se edita a mano. | Directiva `//go:generate` en `cli/internal/scaffold/scaffold.go`. |

### Detalle por archivo

#### kit/commands/vector/apply.md

Acción: MODIFICAR

**§6 (Finish) — anteponer un sub-paso de detección a la transición actual:**

Debe implementar:

- Un sub-paso "Detect external-dependency blocker" que liste las tres señales (TODO que gobierna
  runtime; artefacto outbound de pedido a humano; item de tasks/aceptación pendiente de dato real)
  y el guard mecánico (excluir test-only/cosmético).
- La regla de routing: cualquier señal presente ⇒ `needs-attention --reason "<motivo>"`
  **automático e independiente de `applyMode`**; ninguna señal ⇒ comportamiento actual (`review`
  o dejar para `/vector:close`).
- La forma de la razón: qué está pendiente + cómo/quién lo desbloquea + ref de PR abierto si lo hay.
- Una nota que documente la heurística (auditable) y aclare la diferencia con el hard-stop de §4
  (aquí la implementación **terminó**; allí se **detuvo** por ambigüedad).

Restricciones:

- No editar `state.json`; solo `vector spec status`.
- No cambiar §1–§5 ni el resto de §7 (mode, tasks, gate, uncommitted changes).
- No introducir regex/bash frágil como mecanismo único: la detección es juicio del agente guiado
  por las señales descritas; el único filtro determinista es el guard test-only/cosmético.
- No auto-resolver el bloqueo ni llamar a servicios externos.

**§7 (Report) — surfacear el bloqueo:**

- Si se ruteó a `needs-attention` → línea explícita con el bloqueo y el `reason` (qué falta +
  unblock + PR), en lugar de "ready for review".
- Si se ruteó a `review` → comportamiento actual.

#### cli/internal/scaffold/assets/commands/vector/apply.md

Acción: REGENERAR

Cambios requeridos:

- Tras editar la fuente en `kit/`, correr `go generate` y verificar que la copia embebida coincide
  byte a byte con la fuente.

Restricciones:

- No editarla a mano; es output de `go generate`.

---

## 7. API Contract

No aplica — no hay endpoint HTTP nuevo ni cambio de contrato de API. El change ejerce un flag CLI
ya existente y documentado en el código:

- `vector spec status <id> needs-attention --reason "<texto>"` — `--reason` es string; se persiste
  en `Attention.Reason` (`cli/internal/state/types.go:134`); la legalidad de la transición la
  valida el binario (`runSpecStatus`, `cli/cmd/vector/spec_transitions.go:142`).

No se infieren campos ni se cambian nombres de propiedades del estado.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] Un run de apply que termina con un `TODO(<ticket>)` que gobierna runtime y depende de un
      dato/credencial externo deja la card en `needs-attention` (no `review`) con `reason` no vacío.
- [ ] Un run que produjo un artefacto outbound pidiendo algo a un humano/equipo (p. ej. "please
      send the credentials") deja la card en `needs-attention` con `reason` no vacío.
- [ ] Un run con un item de `tasks.md`/aceptación marcado pendiente de dato/credencial real
      (satisfacible solo contra mocks) deja la card en `needs-attention`.
- [ ] Un run **limpio** (sin señales) mantiene el comportamiento actual: `review` (o se deja para
      `/vector:close`).
- [ ] El `reason` nombra **qué está pendiente** y **cómo se desbloquea** (más ref de PR si aplica),
      y aparece en el board (`needsAttention.reason`) y en la proyección de standup (`TimelineEvent.Reason`).
- [ ] Guard de falso-positivo: un `TODO`/`FIXME` en archivo test-only o un comentario cosmético
      **no** dispara `needs-attention`; el run va a `review`.
- [ ] El routing a `needs-attention` ocurre **automáticamente, sin pedir confirmación**, sea cual
      sea `applyMode` (`auto`/`ask`/`always-ask`).
- [ ] La `Attention.Source` de la transición disparada por apply es `"command"`.
- [ ] El reporte de §7 surfacea el bloqueo + motivo cuando aplica; "ready for review" cuando limpio.
- [ ] La heurística está documentada dentro de `apply.md` (auditable).
- [ ] La copia embebida `cli/internal/scaffold/assets/commands/vector/apply.md` coincide con la
      fuente tras `go generate`.

### Tests requeridos

El artefacto que cambia es markdown (lo ejecuta el modelo), así que la verificación es **por casos
de ejemplo** documentados en el PR, más la no-regresión del binario que ya tiene tests:

- [ ] Caso: TODO runtime con dependencia externa → `needs-attention`.
- [ ] Caso: artefacto outbound de pedido a humano → `needs-attention`.
- [ ] Caso: item de tasks pendiente de dato real → `needs-attention`.
- [ ] Caso: TODO en archivo test-only / cosmético → `review` (guard).
- [ ] Caso: run limpio → `review` (sin cambios).
- [ ] No-regresión: `cli/internal/state/transition_test.go` y los tests de estado siguen verdes
      (la transición + `--reason` + `Source` ya están cubiertos por el binario).

### Comandos de verificación

Ejecutar:

```bash
go -C cli generate ./...     # regenera scaffold/assets (copia embebida del command)
gofmt -l cli                 # debe salir vacío
go -C cli vet ./...
go -C cli test ./...         # spec_transitions + state tests verdes
go -C cli build ./...        # binario con el asset embebido actualizado
```

La fase no está completa si alguno de estos comandos falla o si la copia embebida no coincide con
la fuente.

---

## 9. Criterios de UX

No hay UI interactiva: el artefacto es un command de CLI ejecutado por el agente. La UX visible es
la salida en texto plano del command y la proyección de la card. Por subsecciones del template:

### Loading

No aplica — no hay interacción asíncrona con UI; la detección y la transición ocurren dentro del
run de apply.

### Formularios

No aplica — no hay formularios.

### Passwords

No aplica — no hay campos de credenciales en la UX del command (y los secretos nunca se escriben
en el `reason`, ver §14).

### Errores

- **Sin pedir confirmación** para el routing (decisión cerrada: automático, independiente de
  `applyMode`).
- Si el binario rechaza la transición, el command **surfacea el error** y no lo enmascara.
- La **razón** mostrada es clara y accionable, nunca genérica ("blocker found" está prohibido).

### Navegación

No aplica — el command no navega; transiciona la card vía `vector spec status` y reporta.

### Accesibilidad

No aplica — salida de CLI en texto plano. La card en `needs-attention` y su `reason` quedan
visibles en el board y en el standup digest (`TimelineEvent.Reason`).

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas y el agente no debe cuestionarlas ni cambiarlas:

- **Detección markdown-only**: vive en `apply.md` como juicio del agente guiado por las señales;
  **no** se añade un helper binario (`vector spec detect-blockers` u otro) ni se modifica Go.
- **Guard de falso-positivo = exclusión test-only/cosmético** (determinista). No se implementa la
  lógica de "cubierto por otra card" consultando `.vector/specs/`; ese reconocimiento queda como
  juicio del agente.
- **Routing automático**, independiente de `applyMode` — es una salvaguarda de integridad del
  board, no una elección de workflow; no se pide confirmación.
- **`Attention.Source = "command"`** para la `needs-attention` disparada por apply (igual que el
  hard-stop de §4).
- **Una sola transición y una sola razón por run** (la razón puede enumerar varios bloqueos) —
  para no componer múltiples transiciones (potencialmente ilegales) en un mismo run.
- **Surfacing reutiliza la plumbing existente**: board lee `needsAttention.reason`, standup lee
  `TimelineEvent.Reason`; no se añade plumbing nueva — porque ambos ya proyectan el motivo sin
  modificación.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, pero
no implementarla.

---

## 11. Edge cases

La implementación (la heurística documentada en el command) debe manejar explícitamente:

### Señales de bloqueo (qué SÍ dispara)

- `TODO(<ticket>)`/placeholder en código de producción que gobierna runtime y depende de un
  dato/credencial/identificador externo aún no provisto (caso MH-1582: `api_names` de Zoho).
- Artefacto outbound generado por el run cuyo propósito es pedir algo a un humano/otro equipo
  (borrador de comentario "please send the credentials", "ask X for the api_names", etc.).
- Item de `tasks.md`/aceptación satisfacible solo contra mocks y marcado explícitamente pendiente
  de dato/credencial real.

### Falsos positivos (qué NO dispara)

- `TODO`/`FIXME` en archivos test-only (`*_test.go`, `*.test.*`, dirs `test`/`tests`/`__tests__`).
- Comentarios cosméticos (refactor, renombrar, typo, tidy-up) sin dependencia externa.
- `TODO` que apunta deliberadamente a otra card/ticket ya trackeado (deferral intencional) —
  juicio del agente, no consulta a `.vector/specs/`.

### Casos de borde de la transición

- **Card ya en `needs-attention`** (p. ej. el dev lo marcó en §4 y apply continuó tras
  desbloquear, pero al cierre persiste otro bloqueo): apply fija/refresca el `reason` con el
  bloqueo vigente; no duplica transición ilegal (el binario valida).
- **Múltiples señales**: una sola transición; el `reason` enumera lo pendiente, liderando con el
  bloqueo que gobierna runtime.
- **`tasks.md` ausente**: se omite esa señal sin error; se evalúan las otras dos.
- **Working tree sin cambios** (continuación pura que solo transiciona): no hay artefactos que
  inspeccionar para las señales 1–2; se evalúa tasks/aceptación si existe.
- **El binario rechaza la transición** (transición ilegal o `--reason` requerido vacío): el
  command surfacea el error del binario y no enmascara el fallo; no edita estado a mano.

### Secretos en la razón

- Si una señal contiene un secreto literal (p. ej. `api_key=...`), la razón **no** lo incluye:
  describe el faltante sin filtrar el valor (ver §14).

### API errors / sin conexión / timeout / respuesta vacía / doble submit

No aplica — orquestación markdown sin I/O de red propio: no hay códigos HTTP (400/401/403/404/409/
422/429/500), offline, timeout ni respuesta vacía/inesperada que manejar. La transición es una
invocación CLI local y síncrona a `vector spec status` (una sola por run), por lo que tampoco hay
caso de doble submit; si esa invocación falla, aplica el caso "el binario rechaza la transición"
de arriba.

---

## 12. Estados de UI requeridos

No aplica — el cambio es de orquestación del command. La card, el board y el timeline ya soportan
`needs-attention` con `reason`; este change no introduce estados de UI nuevos. El estado
`needs-attention` ya existe; lo único nuevo es **cuándo** apply lo elige.

---

## 13. Validaciones

### Validaciones del command (lado agente)

| Campo | Regla | Mensaje/efecto |
|---|---|---|
| Señales de bloqueo | Evaluar las 3 heurísticas sobre los artefactos del run | Si ≥1 presente y no filtrada por el guard → `needs-attention` |
| Guard test-only/cosmético | Excluir TODOs en archivos test-only y comentarios cosméticos | No dispara (va a `review`) |
| `reason` | No vacío y concreto (qué falta + unblock + PR si aplica) | El binario exige `--reason` al entrar a `needs-attention` |
| Secretos en `reason` | Nunca incluir valores de credenciales/keys | Describir el faltante sin el valor |

### Validaciones del binario

`vector spec status <id> needs-attention --reason "<texto>"` valida la legalidad de la transición
y la presencia del motivo, y persiste `Attention{Reason,Since,Source}`. El command no replica esa
validación: delega y reporta el error si lo hay.

---

## 14. Seguridad y permisos

- **No exponer secrets en el `reason`**: si una señal contiene un valor sensible (token, api_key,
  password), la razón describe qué falta sin incluir el valor. El `reason` se persiste en
  `state.json` (committed) y se muestra en board/standup, así que tratar su contenido como público.
- No registrar payloads sensibles ni el diff completo en el `reason` ni en el reporte.
- El change no introduce gates de permiso nuevos: apply ya escribe estado del repo de Vector vía
  el binario (CLI-owns-writes); la detección es de solo lectura sobre artefactos locales.

---

## 15. Observabilidad y logging

- El binario ya emite el evento `status.changed` (con `trigger` y `reason`) al transicionar; la
  detección no añade logging propio más allá del reporte de §7.
- Si el binario rechaza la transición, el command surfacea el error (no lo enmascara) en el reporte.
- No registrar el working-tree diff completo ni el contenido íntegro de los TODOs; solo el resumen
  accionable en el `reason`.

No registrar: credenciales, tokens, datos personales, payloads completos.

---

## 16. i18n / textos visibles

El texto del command (`apply.md`) y los identificadores van **en inglés** (convención de los
`/vector:*`). No hay sistema de i18n de UI que tocar. Textos de referencia (en inglés, redacción
final a criterio en la implementación del command):

| Situación | Texto de referencia |
|---|---|
| Bloqueo detectado (reporte §7) | `external blocker → needs-attention: <reason>` |
| Run limpio (reporte §7) | comportamiento actual ("ready for review" / next-step) |
| Forma del reason | `<what's pending> — <unblock path / who>; PR #<n> open` (cuando aplica) |

No hardcodear textos visibles fuera de lo que el command ya define.

---

## 17. Performance

- La detección es inspección local de artefactos ya disponibles del run (diff, tasks.md); costo
  despreciable y dentro del mismo run de implementación.
- Sin I/O de red nuevo ni agente adicional (respeta `product/token-routing.md`).
- No altera el tier de modelo del flujo de apply.

---

## 18. Restricciones

El agente no debe:

- Modificar Go/binario (`runSpecStatus`, `SetStatus`, `Attention`, `spec_transitions.go`) ni el
  esquema de estado.
- Añadir un helper binario de detección (decisión cerrada: markdown-only).
- Implementar la lógica de "cubierto por otra card" consultando `.vector/specs/`.
- Pedir confirmación para el routing (decisión cerrada: automático, independiente de `applyMode`).
- Cambiar `/vector:sync`, `/vector:link`, `/vector:close`, `/vector:status` ni `applyMode`.
- Auto-resolver el bloqueo (traer credenciales, llamar APIs externas).
- Soportar múltiples transiciones/razones por run.
- Editar `state.json` a mano; solo vía `vector spec status`.
- Cambiar §1–§5 ni el resto del reporte de §7 (mode, tasks, gate, uncommitted changes).
- Introducir regex/bash frágil como mecanismo único de detección.
- Inventar nuevas señales de bloqueo fuera de las tres definidas.
- Dejar el asset embebido desincronizado de la fuente.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `kit/commands/vector/apply.md` con §6 (detección + transición condicional) y §7 (reporte del
      bloqueo) actualizados y con la heurística documentada (auditable).
- [ ] `cli/internal/scaffold/assets/commands/vector/apply.md` regenerado y verificado vs la fuente.
- [ ] Ejemplos en el PR de los casos de §8 (3 disparan, 2 no disparan, 1 limpio).
- [ ] Gate verde: `go generate`, `gofmt -l`, `go vet`, `go test`, `go build`.
- [ ] Sin regresiones en otros pasos de apply ni en otros commands.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo y `kit/commands/vector/apply.md` entero.
- [ ] Confirmé que `vector spec status <id> needs-attention --reason "…"` es la interfaz y que
      **no** se cambia Go (`spec_transitions.go:142`, `types.go:134`).
- [ ] Documenté las 3 señales y el guard test-only/cosmético dentro de `apply.md` (auditable).
- [ ] El routing es automático e independiente de `applyMode`; `Attention.Source = "command"`.
- [ ] La razón es concreta (qué falta + unblock + PR) y nunca filtra secretos.
- [ ] El reporte de §7 surfacea el bloqueo cuando aplica; "ready for review" cuando limpio.
- [ ] No toqué §1–§5 ni el resto de §7, ni otros commands, ni Go, ni el esquema de estado.
- [ ] Regeneré con `go generate` y verifiqué que la copia embebida coincide con la fuente.
- [ ] Ejecuté `gofmt`, `go vet`, `go test`, `go build` — verdes.
- [ ] No dejé `[...]` ni TODOs sin justificar en el command.

---

## Open questions

- Versión exacta de Go objetivo: no es necesaria para este change (no se compila lógica nueva).
  `TBD — ver Open questions` solo si una futura restricción lo exigiera.
- Formato fino del `reason` por tipo de señal (uniforme vs prefijado por tipo) y si debe incluir
  `archivo:línea` del TODO: queda a criterio de la implementación del command; el requisito firme
  es que sea concreto y accionable. No bloqueante.
