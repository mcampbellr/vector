# Spec: Retry de JSON malformado en el dispatcher de subagentes estructurados

## 1. Objetivo

Construir el mecanismo de **retry en la capa de orquestación del dispatcher** que re-spawnea
un subagente Haiku exactamente una vez cuando su salida JSON estructurada no supera la
validación de shape; si el segundo intento también falla, reporta el error al usuario sin
escribir estado.

Esta feature permite que el dispatcher (el command `/vector:*` que orquesta el subagente)
detecte salidas no deterministas o truncadas del LLM **antes** de pipear al binario, y se
recupere de forma autónoma sin estado corrupto ni pérdida silenciosa del marcador de standup.

## 2. Alcance

### Incluido en esta fase

- **Paso de retry en `kit/commands/vector/standup.md`**: validación del shape
  `{global, perSpec[]}` recibido del `vector-standup-writer` antes de pipear a
  `vector standup commit`; re-spawn único; fallo del segundo → reportar, no avanzar marcador.
- **Paso de retry en `kit/commands/vector/apply.md`**: validación del shape `{summary}`
  recibido del `vector-summary-writer` en el paso §7 de apply; re-spawn único; fallo del
  segundo → no-gate (skip de la escritura del summary, log del error, continuar).
- **Contrato de validación de shape por tipo de salida**: definir, en este spec, qué
  constituye un shape válido para cada subagente existente (`standup-writer`, `summary-writer`)
  y marcar como TBD el del futuro `vector-spec-composer`.
- **Comportamiento diferenciado gate vs no-gate**: standup es gate (el marker no avanza si
  el JSON final falla); summary es no-gate (apply continúa aunque el summary falle).

### Fuera de scope

- Cambios al **binario Go**: el binario (`vector standup commit`, `vector spec summarize … commit`)
  ya valida JSON en `json.Unmarshal` como red de seguridad. Esta feature no toca `cli/`.
- Más de **un retry** por invocación del command: la propuesta es `1 re-spawn`; más intentos
  introducen costo de tokens sin ganancia proporcional.
- Cambios a los **agents** (`vector-standup-writer.md`, `vector-summary-writer.md`): ya dicen
  "emit valid JSON only" y definen su shape; no hay nada que añadir en esta fase.
- Cobertura del **`vector-spec-composer`**: ese agente no existe aún; el shape de su output
  queda como TBD en §7 y se cerrará en su propio spec.
- **Retry de otros errores** (no encontrado el binario, fallo de proyección, error I/O): esos
  caminos ya tienen manejo accionable; solo se retoca el fallo de parse de JSON del agente.
- **Panel web / proyección visual**: no hay cambios de UI.

El agente no implementa nada fuera de este alcance aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- **Project commands**: Markdown + frontmatter orquestado por Claude.
  Patrón: `kit/commands/vector/standup.md`, `kit/commands/vector/apply.md`.
- **Binario**: Go (módulo único `cli/`). **No se modifica en esta fase** — la validación y el
  retry son responsabilidad del command, no del binario.
- **Subagentes Haiku**: `vector-standup-writer` y `vector-summary-writer` (tier barato,
  `product/token-routing.md`); el command es quien decide si re-spawnar.

### Versiones relevantes

- Go: `1.26` (citado de `cli/go.mod`). No relevante para esta fase (sin cambios al binario).
- Model tier del retry: mismo que el intento original — **Haiku** para `standup-writer` y
  `summary-writer`. No promover a Sonnet/Opus en el retry (violaría `token-routing.md`).

### Patrones existentes a respetar

- **CLI-owns-writes**: el command nunca edita `.vector/` directamente; toda escritura pasa
  por el binario. El retry no altera este invariante — el shape check ocurre antes de pipear.
- **Standup es gate, summary es no-gate**: patrón ya establecido en `apply.md` §7
  ("Empty/invalid prose → nothing is written (not a gate); note it and move on"). Esta feature
  **formaliza y extiende** ese patrón con un re-spawn previo.
- **El binario como red de seguridad**: `vector standup commit` ya rechaza JSON inválido con
  `"invalid digest json"` (`standup.go:158–159`); `vector spec summarize … commit` tiene
  validación análoga. El retry en el command reduce la tasa de error al binario, pero no
  elimina esa red.
- **Subagente lanzado con prompt cerrado**: el command pasa el JSON de proyección al agente;
  en el retry pasa el mismo JSON + una directive de corrección explícita (ver §5).
- **Git artifacts en inglés, prosa en español**: esta regla aplica a los artefactos del repo;
  los strings internos del command siguen la convención vigente del proyecto.

---

## 4. Dependencias previas

- [ ] `kit/commands/vector/standup.md` existente y funcional (con los 4 pasos actuales).
- [ ] `kit/commands/vector/apply.md` existente y funcional (con los 8 pasos actuales).
- [ ] `kit/agents/vector-standup-writer.md` y `kit/agents/vector-summary-writer.md`
  embebidos, sin cambios requeridos en esta fase.
- [ ] Binario `vector standup commit` y `vector spec summarize <id> commit` ya implementados
  y estables (sin cambios en esta fase).
- [ ] `docs/orchestration-review.md` §12 como evidencia de la dirección propuesta (fila
  "JSON malformado de subagente": "Dispatcher valida y re-spawnea (1 retry); fallo → reporta,
  no escribe").

Si alguna dependencia no existe, el agente se detiene y reporta exactamente qué falta.

---

## 5. Arquitectura

### Patrón

**Dispatcher con shape-gate inline**: el command valida la salida del subagente contra el
shape esperado antes de pipear al binario. Si la validación falla, re-spawna el subagente
exactamente una vez con un prompt de corrección explícito. Si el segundo intento también falla
el shape-gate, aplica la política de la operación (gate → reportar y abortar; no-gate →
skip y continuar).

### Capas afectadas

- **Project commands** (`kit/commands/vector/standup.md`, `kit/commands/vector/apply.md`):
  sí — se inserta el shape-gate y el re-spawn entre el paso de generación del agente y el
  paso de commit al binario.
- **Binario CLI** (`cli/`): **no** — su validación ya existe y no se modifica.
- **Agentes** (`kit/agents/vector-standup-writer.md`, `kit/agents/vector-summary-writer.md`):
  **no** — ya definen su shape y sus reglas de salida.
- **State** (`cli/internal/state/`): **no** — el retry ocurre antes de que el binario
  escriba cualquier cosa.
- **web/**: **no**.

### Flujo esperado — standup

1. Command corre `vector standup --json` → proyección JSON (paso §1 actual, sin cambios).
2. Command lanza `vector-standup-writer` (Haiku) con el JSON de proyección → respuesta del
   agente (paso §2 actual).
3. **[NUEVO] Shape-gate (intento 1)**: el command valida que la respuesta sea JSON parseable
   con shape `{global: string, perSpec: [{id: string, summary: string}]}`, donde `global` es
   no vacío y `perSpec` es un array (puede ser vacío si el input `perSpec` era vacío).
   - **Válido** → continuar al paso §3 actual (pipe a `vector standup commit …`).
   - **Inválido** → re-spawn del agente (intento 2) con el mismo JSON de proyección más la
     directive explícita:
     ```
     The previous attempt returned malformed or invalid JSON.
     Return ONLY a valid JSON object matching exactly:
     {"global": "<string>", "perSpec": [{"id": "<string>", "summary": "<string>"}]}
     No preface, no code fences, no trailing text.
     ```
4. **[NUEVO] Shape-gate (intento 2)**: valida la segunda respuesta con el mismo check.
   - **Válido** → continuar al paso §3 actual.
   - **Inválido** → reportar al usuario:
     ```
     standup digest failed: the subagent returned invalid JSON twice; nothing was written
     and the marker was not advanced. Re-run /vector:standup to retry.
     ```
     Abortar. El marcador no avanza. **No pipear nada al binario.**
5. Pasos §3 y §4 actuales sin cambios.

### Flujo esperado — apply (paso §7, summary)

1. Command corre `vector spec summarize <id> --json` → proyección JSON (§7 paso 1 actual, sin
   cambios).
2. Command lanza `vector-summary-writer` (Haiku) con el JSON → respuesta del agente (§7 paso
   2 actual).
3. **[NUEVO] Shape-gate (intento 1)**: valida que la respuesta sea JSON parseable con shape
   `{summary: string}`, donde `summary` es no vacío.
   - **Válido** → continuar al §7 paso 3 actual (pipe a `vector spec summarize <id> commit …`).
   - **Inválido** → re-spawn del agente (intento 2):
     ```
     The previous attempt returned malformed or invalid JSON.
     Return ONLY a valid JSON object matching exactly:
     {"summary": "<2–3 sentences>"}
     No preface, no code fences, no trailing text.
     ```
4. **[NUEVO] Shape-gate (intento 2)**: valida la segunda respuesta.
   - **Válido** → continuar al §7 paso 3 actual.
   - **Inválido** → **no-gate**: no pipear al binario; notar brevemente en el reporte de §8:
     ```
     summary skipped: subagent returned invalid JSON twice
     ```
     Continuar a §8 sin interrumpir el flujo de apply.
5. §7 paso 3 y §8 actuales sin cambios (salvo la nota en el reporte si se saltó el summary).

### Contrato de validación de shape por tipo de salida

| Subagente | Shape esperado | Condición "válido" |
|---|---|---|
| `vector-standup-writer` | `{global: string, perSpec: [{id: string, summary: string}]}` | JSON parseable; `global` non-empty; `perSpec` es array (puede ser `[]` si el input tenía `perSpec: []`) |
| `vector-summary-writer` | `{summary: string}` | JSON parseable; `summary` non-empty |
| `vector-spec-composer` | TBD — ver Open questions | TBD |

La validación es **mínima y suficiente**: parseo + presencia de las claves requeridas +
tipos correctos. No se validan valores semánticos (que el `id` de cada `perSpec` matchee el
input, etc.) — eso lo maneja el binario en el commit.

### Ubicación de archivos modificados

Solo se modifican los dos commands de `kit/`. No se crean carpetas nuevas.

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `kit/commands/vector/standup.md` | MODIFICAR | Insertar shape-gate + retry entre §2 y §3 | `kit/commands/vector/apply.md` §7 |
| `kit/commands/vector/apply.md` | MODIFICAR | Insertar shape-gate + retry en §7 paso 2 | `kit/commands/vector/standup.md` §2→§3 (una vez modificado) |
| `cli/internal/scaffold/assets/commands/vector/standup.md` | MODIFICAR (generado) | Copia embebida en sync con `kit/` vía `go generate` | sibling `apply.md` |
| `cli/internal/scaffold/assets/commands/vector/apply.md` | MODIFICAR (generado) | Ídem | sibling `standup.md` |

> Los assets en `cli/internal/scaffold/assets/commands/vector/` son copias generadas de los
> archivos de `kit/`; el mecanismo de sync (`go generate`) los mantiene en par. Si el proceso
> de generación es manual (copy), el agente debe actualizar ambos en el mismo paso.

### Detalle por archivo

#### `kit/commands/vector/standup.md` — MODIFICAR

Cambios requeridos:

- Entre el párrafo "It returns: ```json …```" de §2 y la línea "## 3. Persist via the binary",
  insertar un bloque **"Validate the digest (shape-gate)"** que describe:
  - Chequeo de parseabilidad y presencia de `global` (non-empty) y `perSpec` (array).
  - Si inválido: re-spawn con la directive de corrección (ver §5); segundo check.
  - Si el segundo intento falla: reportar al usuario con el mensaje exacto definido en §5 y
    detener (el marcador no avanza; no pipear nada al binario).
- El paso §3 original ("Persist via the binary") no se modifica en su lógica; se actualiza su
  numeración si el nuevo bloque se introduce como un §2.5 o se renombran los pasos.
- Mantener el tono y el estilo del command actual (imperativo, primera persona del command).

Restricciones:
- No cambiar el paso §1 (proyección), el paso §3 (persist) ni el §4 (report).
- No añadir un tercer intento.
- No cambiar el comportamiento existente cuando el JSON es válido en el primer intento.
- No modificar el agent definition (`kit/agents/vector-standup-writer.md`).

#### `kit/commands/vector/apply.md` — MODIFICAR

Cambios requeridos:

- En §7 ("Summarize what was done"), entre el paso "2. Pass that **exact JSON** …" y el
  paso "3. Pipe its JSON to `vector spec summarize …`", insertar el shape-gate inline:
  - Check de parseabilidad y `summary` non-empty.
  - Si inválido: re-spawn con la directive de corrección.
  - Si el segundo intento falla: skip no-gate (no pipear al binario; nota en §8).
- En §8 ("Report"), añadir que si el summary fue skipped por doble fallo, se menciona
  brevemente en el reporte ("summary skipped: subagent returned invalid JSON twice").

Restricciones:
- No cambiar ningún paso fuera de §7 y §8.
- No cambiar el comportamiento no-gate cuando el JSON es válido en el primer intento.
- Mantener que apply nunca es gate en el summary: el doble fallo solo produce el skip, no un
  abort del flujo de apply.
- No alterar la lógica de §6 (detect external blockers, transition).

---

## 7. API Contract

Sin API surface HTTP — **no aplica**. La interfaz relevante es la interacción entre el command
y el subagente Haiku, que ya está documentada en los agentes correspondientes.

**Shapes de salida (contratos de validación del dispatcher):**

`vector-standup-writer` (fuente: `kit/agents/vector-standup-writer.md` §Output):
```json
{
  "global": "<1–3 short paragraphs>",
  "perSpec": [
    { "id": "<spec id verbatim>", "summary": "<1–2 sentences>" }
  ]
}
```
Condición de validez en el dispatcher: JSON parseable, `global` string non-empty, `perSpec` array.

`vector-summary-writer` (fuente: `kit/agents/vector-summary-writer.md` §Output):
```json
{ "summary": "<2–3 sentences grounded in this spec's events>" }
```
Condición de validez en el dispatcher: JSON parseable, `summary` string non-empty.

`vector-spec-composer`: TBD — ver Open questions.

**Prompt de retry (directive de corrección):**
```
The previous attempt returned malformed or invalid JSON.
Return ONLY a valid JSON object matching exactly:
<shape esperado según el agente>
No preface, no code fences, no trailing text.
```

La directive incluye el shape exacto para el agente en cuestión. El JSON de entrada original
se pasa nuevamente sin modificar (el agente necesita el mismo input para regenerar la salida).

---

## 8. Criterios de éxito

- [ ] Cuando `vector-standup-writer` devuelve JSON válido al primer intento, el flujo es
  idéntico al comportamiento actual (zero delta).
- [ ] Cuando `vector-standup-writer` devuelve JSON inválido al primer intento y válido al
  segundo, el standup se persiste correctamente y el marcador avanza.
- [ ] Cuando `vector-standup-writer` devuelve JSON inválido en ambos intentos, el command
  reporta el error con el mensaje exacto definido, no avanza el marcador, y no escribe nada
  en `.vector/local/standup.json`.
- [ ] Cuando `vector-summary-writer` devuelve JSON inválido en ambos intentos, apply
  continúa, reporta "summary skipped" en §8, y el spec transiciona normalmente (el summary
  skip no es un gate).
- [ ] El retry usa el mismo tier de modelo (Haiku) que el intento original.
- [ ] El JSON de proyección original se pasa sin modificar al retry, junto con la directive
  de corrección.
- [ ] Sin regresiones: `standup`, `apply`, `standup commit`, `spec summarize commit` siguen
  funcionando en el camino happy-path.

### Tests requeridos

Los commands de `kit/` son Markdown (no código Go); los "tests" son verificaciones manuales /
de integración en este contexto. Sin embargo:

- [ ] Verificación manual: simular JSON inválido del agente en standup → confirmar que el
  marcador no avanza y el mensaje de error es correcto.
- [ ] Verificación manual: simular JSON inválido del agente en apply §7 → confirmar que
  apply completa la transición y el reporte menciona el skip.
- [ ] Verificación de no-regresión: correr un flujo completo de standup y apply con output
  válido del agente → ambos completan sin cambios observables.
- [ ] Si en el futuro se añade un test de integración E2E del dispatcher, cubrir:
  - Primer intento inválido, segundo válido (standup): marcador avanza.
  - Ambos inválidos (standup): marcador no avanza; error accionable.
  - Ambos inválidos (apply summary): apply completa; reporte menciona skip.

### Comandos de verificación

```bash
# Verifica que los assets embebidos estén en sync con kit/ (si el sync es vía go generate)
go -C cli generate ./...
# Gate de calidad Go (sin cambios al binario, pero verificar que no se rompe)
gofmt -l cli && go -C cli vet ./... && go -C cli test ./...
```

---

## 9. Criterios de UX

Aplica al **command** (no a UI web):

- **Transparencia del retry**: cuando ocurre un retry, el command debe notar al usuario que
  el primer intento falló y que está reintentando, p. ej.:
  ```
  standup digest: subagent returned invalid JSON — retrying (attempt 2/2)…
  ```
  Esto evita que el usuario piense que el command está colgado.
- **Mensaje de fallo final accionable**: si ambos intentos fallan, el mensaje debe indicar qué
  ocurrió, qué no fue escrito, y la acción a tomar. Formato standup:
  ```
  standup digest failed: the subagent returned invalid JSON twice; nothing was written
  and the marker was not advanced. Re-run /vector:standup to retry.
  ```
  Formato apply summary skip: incluirse en el reporte de §8 sin interrumpir la salida principal,
  de modo que el dev note el issue pero no piense que apply falló.
- **Silencio en el happy path**: cuando el JSON es válido al primer intento, ningún mensaje
  adicional. Zero overhead perceptual.
- **Atomicidad preservada**: el retry nunca resulta en un estado parcialmente escrito; o se
  escribe todo (valid JSON commit) o no se escribe nada (fallo del segundo intento).
- **No prompts adicionales**: el retry es automático, sin `AskUserQuestion`. La política es
  fija (1 retry) y no configurable en V1.

---

## 10. Decisiones tomadas

- **1 solo retry**: más intentos multiplicarían el costo de tokens sin garantía de mejora; el
  fallo repetido sugiere un problema sistémico (input muy largo, modelo saturado) que un
  tercer intento no resuelve. *Por qué:* proporcional al beneficio.
- **Mismo tier de modelo en el retry (Haiku)**: el fallo de JSON no implica que el modelo
  necesite más capacidad; puede ser truncamiento por contexto o ruido puntual. Promover a
  Sonnet/Opus en el retry violaría `product/token-routing.md`. *Por qué:* token efficiency.
- **Retry con el mismo input + directive de corrección**: re-enviar el JSON original más una
  instrucción clara de formato maximiza la probabilidad de output correcto sin inventar
  contexto. *Por qué:* el agente ya tiene toda la info; solo necesita el recordatorio de
  formato.
- **Sin cambios al binario**: el binario ya valida JSON en el commit; el retry es una mejora
  de resiliencia en la capa de orquestación, no en la de persistencia. Mantener la separación
  de responsabilidades. *Por qué:* CLI-owns-writes, pero el dispatcher orquesta.
- **Summary es no-gate también bajo doble fallo**: la política existente en `apply.md` es
  "Empty/invalid prose → nothing is written (not a gate)"; esta feature respeta y extiende ese
  contrato. *Por qué:* el valor del summary no justifica interrumpir el flujo de apply.
- **Standup es gate bajo doble fallo**: sin digest válido el marcador no debe avanzar —
  avanzar el marcador sin digest significaría perder el registro del período para siempre.
  *Por qué:* invariante del design actual de standup.

Si el agente ve una alternativa mejor, la reporta como observación; no la implementa.

---

## 11. Edge cases

- **JSON truncado** (no cierra el último `}`): el parse falla → activa el retry. Esperado y
  cubierto por el shape-gate.
- **JSON con shape incompleto** (p. ej. `{"global": "…"}` sin `perSpec`): el check de
  presencia de claves falla → retry.
- **JSON válido pero `global` vacío** (p. ej. `{"global": "", "perSpec": []}`): `global`
  empty string → shape inválido → retry.
- **JSON válido y `perSpec` vacío** cuando el input tenía specs: el check de presencia del
  array pasa (array vacío es array); el binario `standup commit` reconstruye los campos
  estructurales desde una proyección fresca → puede producir un digest sin per-spec summaries
  (que es la behavior actual si el agente omite specs). No es un fallo del shape-gate; es
  información insuficiente del agente, que ya ocurría antes de este spec.
- **Segundo intento devuelve prosa antes del JSON** (p. ej. `Here is the JSON:\n{…}`): el
  parse de la respuesta completa falla → fallo del segundo intento → política según el comando.
  El command no debe stripear la prosa antes de parsear (eso podría enmascarar el problema).
- **Input de proyección muy largo** (muchos specs, notas largas): puede causar truncamiento
  en Haiku; en el retry el mismo input se re-envía. Si el truncamiento persiste, el doble
  fallo activa la política. El tamaño del input no se reduce automáticamente (TBD para fases
  futuras, ver Open questions).
- **Subagente lanza excepción / error de invocación** (no devuelve nada): el command trata
  la ausencia de output como JSON inválido y activa el retry. Si la segunda invocación también
  falla sin output → política según el comando.
- **Concurrencia** (dos instancias del command corriendo en paralelo): el binario serializa
  escrituras vía mutex; el retry en el command no añade riesgo de escritura duplicada porque
  el shape-gate ocurre antes del pipe al binario.
- **Sin tooling Haiku disponible** (model unavailable): el error de invocación del agente
  sube al command como un fallo de generación, no de JSON. No es el mismo camino que el
  shape-gate; se gestiona por el manejo de errores existente del command.
- **`vector-spec-composer`** (shape TBD): no cubierto en esta fase. Ver Open questions.

---

## 12. Estados de UI requeridos

Estados de salida del command (no UI web):

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| happy path | flujo actual sin mensajes adicionales | — |
| retry en curso | `subagent returned invalid JSON — retrying (attempt 2/2)…` | esperar |
| retry exitoso | digest commited normalmente (mismo output que hoy) | continuar |
| doble fallo (standup) | mensaje de fallo + `Re-run /vector:standup to retry.` | re-ejecutar |
| doble fallo (apply summary) | nota en §8: `summary skipped: …` | ignorar o re-ejecutar |
| idle | invocación normal de `/vector:standup` o `/vector:apply` | ejecutar |
| disabled | No aplica — sin componentes UI interactivos | — |
| offline | No aplica — ambos commands son local-only (sin red) | — |

---

## 13. Validaciones

### Validaciones en el command (shape-gate)

| Output | Campo | Regla | Acción si falla |
|---|---|---|---|
| `vector-standup-writer` | raíz | JSON parseable | retry |
| `vector-standup-writer` | `global` | string presente y non-empty | retry |
| `vector-standup-writer` | `perSpec` | array presente (puede ser `[]`) | retry |
| `vector-standup-writer` | `perSpec[*].id` | string presente | retry |
| `vector-standup-writer` | `perSpec[*].summary` | string presente | retry |
| `vector-summary-writer` | raíz | JSON parseable | retry |
| `vector-summary-writer` | `summary` | string presente y non-empty | retry (no-gate si doble fallo) |

### Validaciones del binario (safety net, sin cambios)

El binario ya valida con `json.Unmarshal` en `runStandupCommit` (`standup.go:158`) y en el
subcomando `spec summarize commit`. Esas validaciones permanecen como red de seguridad de
última capa y no se modifican.

---

## 14. Seguridad y permisos

- El retry re-envía el mismo JSON de proyección que el primer intento: no expone datos
  adicionales ni secrets. El JSON de proyección contiene títulos de specs e IDs; ya se
  enviaba en el primer intento.
- El prompt de corrección del retry no incluye el output fallido del agente (que podría
  contener prosa inesperada) — solo la directive de formato y el input original.
- Sin cambios de permisos de escritura: el retry no escribe nada; la escritura ocurre, como
  antes, en el commit que sigue al shape-gate exitoso.
- No hay secrets en el output de los agentes (`vector-standup-writer`, `vector-summary-writer`
  trabajan sobre datos de actividad/proyección, no sobre credentials ni tokens).

---

## 15. Observabilidad y logging

Usar el `activity.jsonl` existente solo para eventos de dominio. El retry **no** emite un
evento de actividad propio (no es una acción de dominio — es resiliencia de orquestación).

El command puede notar el retry al usuario via stdout (ver §9), pero **no** lo persiste en
`activity.jsonl`. Los fallos del shape-gate son ruido de LLM, no eventos del dominio del board.

Si en el futuro existe un canal de telemetría (p. ej. `agent.routed` para token routing),
se podría emitir un evento `agent.retried` con `{subagent, attempt, outcome}` — TBD,
fuera del scope de esta fase.

No logear:
- El output inválido del agente en disco (podría contener prosa arbitraria).
- Secrets o tokens (no los hay en este flujo).

---

## 16. i18n / textos visibles

**El proyecto no tiene sistema de i18n.** Los strings del command están en inglés hardcodeado,
consistente con el resto del binario y los commands existentes. La tabla siguiente es para
documentación; no es un archivo de traducción.

| Identificador (doc) | Texto (hardcoded EN, en el command) |
|---|---|
| retry.notice | `subagent returned invalid JSON — retrying (attempt 2/2)…` |
| retry.fail.standup | `standup digest failed: the subagent returned invalid JSON twice; nothing was written and the marker was not advanced. Re-run /vector:standup to retry.` |
| retry.fail.summary | `summary skipped: subagent returned invalid JSON twice` |

---

## 17. Performance

- **Costo del retry**: en el camino no-retry (JSON válido al primer intento), impacto = cero.
  En el camino retry: 1 invocación adicional de Haiku, mismo contexto. Haiku es el tier más
  barato; el costo es proporcional a la frecuencia de fallos de JSON, que debería ser baja
  bajo condiciones normales (el agente ya tiene reglas de output estrictas).
- **Sin I/O adicional**: el retry usa el mismo JSON de proyección ya en memoria; no re-lee
  el disco ni re-proyecta events.
- **Sin latencia en el happy path**: el shape-gate es una operación de parse en el contexto
  del command, no una llamada externa. Overhead negligible.
- **Límite de intentos fijo en 1**: garantiza que el overhead de tokens por fallo esté acotado
  (`≤ 2 × costo_haiku_output` en el peor caso por invocación del command).

---

## 18. Restricciones

El agente no debe:
- Añadir cambios al binario Go (`cli/`).
- Modificar los agents (`vector-standup-writer.md`, `vector-summary-writer.md`).
- Promover el tier del modelo a Sonnet u Opus en el retry.
- Introducir más de 1 retry.
- Alterar la política gate/no-gate existente (standup = gate; summary = no-gate).
- Hacer que el fallo del summary interrumpa el flujo de apply.
- Stripear o "arreglar" el output inválido del agente antes de determinar el fallo.
- Pipear JSON inválido al binario bajo ninguna circunstancia.
- Refactorizar pasos no relacionados de standup.md o apply.md.
- Crear abstracciones nuevas (no hay código Go nuevo; el pattern es inline en los commands).
- Cubrir el `vector-spec-composer` (out of scope; TBD en su propio spec).

---

## 19. Entregables

- [ ] `kit/commands/vector/standup.md` modificado con el shape-gate y retry entre §2 y §3.
- [ ] `kit/commands/vector/apply.md` modificado con el shape-gate y retry en §7 paso 2.
- [ ] `cli/internal/scaffold/assets/commands/vector/standup.md` en sync (copia del kit).
- [ ] `cli/internal/scaffold/assets/commands/vector/apply.md` en sync (copia del kit).
- [ ] Contratos de shape documentados en este spec (§7) como referencia para futuros commands.
- [ ] Sin regresiones: `gofmt`/`go vet`/`go test` verdes (sin cambios al binario, solo
  verificación de no-regresión).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `docs/orchestration-review.md` §12 (fila "JSON malformado de subagente").
- [ ] Revisé `kit/commands/vector/standup.md` §2 y §3 actuales.
- [ ] Revisé `kit/commands/vector/apply.md` §7 actual.
- [ ] Revisé `kit/agents/vector-standup-writer.md` y `kit/agents/vector-summary-writer.md`
  (shapes de output y reglas de emisión).
- [ ] Revisé `cli/cmd/vector/standup.go` líneas 155–160 (validación actual del binario).
- [ ] Solo modifiqué los archivos listados en §6.
- [ ] Inserté el shape-gate **entre** la generación del agente y el pipe al binario, sin
  cambiar ninguno de esos dos pasos.
- [ ] Mantuve la política gate (standup) / no-gate (summary) sin alterarla.
- [ ] El retry usa el mismo tier de modelo y el mismo JSON de entrada.
- [ ] El prompt de corrección incluye el shape exacto esperado.
- [ ] No cambié decisiones tomadas ni el schema del state.
- [ ] Ejecuté `gofmt`, `go vet`, `go test` (verificación de no-regresión).
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar (salvo los marcados como TBD).
- [ ] Los assets en `cli/internal/scaffold/assets/commands/vector/` están en sync con `kit/`.

## Open questions

- **Shape de `vector-spec-composer`**: el brief lo menciona como tercer consumidor de JSON
  estructurado, pero el agente no existe aún. El contrato de validación de su output
  (y el command que lo invoca) se define en el spec de `vector-spec-composer`. TBD — ver Open questions.
- **Input muy largo → truncamiento sistemático**: si el JSON de proyección de standup supera
  el contexto de Haiku, el retry con el mismo input también fallará. Mitigaciones posibles
  (truncar `perSpec`, paginación) son fuera de scope V1. TBD para optimización post-lanzamiento.
- **Telemetría del retry**: ¿conviene emitir un evento `agent.retried` en `activity.jsonl`
  para medir la frecuencia de fallos en producción? Fuera de scope V1; dependería de un canal
  de telemetría que hoy no existe.
- **Comportamiento bajo rate-limit del modelo**: si la API de Haiku retorna un error de
  rate-limit (no JSON inválido), ¿debe el retry esperar o fallar inmediatamente? Hoy el
  manejo de errores de invocación del agente ya existe en el command (no es el mismo camino
  que el shape-gate); TBD en la implementación.
