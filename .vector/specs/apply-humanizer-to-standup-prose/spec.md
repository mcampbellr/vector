# Spec: Humanizar la prosa del standup writer

## 1. Objetivo

Construir una guía condensada de **humanización de prosa** (derivada de la skill `/humanizer`
del usuario) e incrustarla en el subagente `vector-standup-writer`, de modo que el digest del
standup salga **sin tells de texto generado por IA** desde la primera generación, sin que el dev
tenga que pasar manualmente el resultado por `/humanizer`.

Esta feature permite que un **dev** pueda **leer en voz alta un digest de standup que suena
escrito por una persona** —conciso, en presente, sin inflación de significado, sin colas `-ing`
superficiales, sin regla de tres forzada— para obtener una ceremonia más natural, en el idioma
que el repo declare (español, inglés u otro), reutilizando el catálogo de patrones del
`/humanizer` como base sin depender de esa skill en runtime.

El problema concreto: hoy `vector-standup-writer` (Haiku) produce prosa correcta pero con
muletillas típicas de IA (vocabulario inflado, copula avoidance, negaciones paralelas). El
`/humanizer` del usuario captura 29 patrones anti-IA, pero vive en sus dotfiles personales
(`~/.dotfiles/claude/.claude/skills/humanizer/SKILL.md`) y **no** se distribuye con Vector, así
que un repo de usuario final no lo tiene. La guía debe vendorizarse dentro del agente.

## 2. Alcance

### Incluido en esta fase

- **Bloque de "Prose quality" condensado** dentro de `kit/agents/vector-standup-writer.md`:
  una destilación (~20–30 líneas) de los patrones del `/humanizer` más relevantes para prosa
  corta de standup, escrito en inglés, redactado como directivas accionables (qué evitar / qué
  preferir), no como el catálogo completo de 29 secciones.
- **Humanización siempre activa**: forma parte del system prompt del agente; no hay flag de
  ejecución ni campo de config que la prenda/apague (decisión tomada §10).
- **Agnóstica al idioma**: la guía se escribe en inglés (nombres de patrones: *rule of three*,
  *em-dash overuse*, etc.) pero los patrones son universales y se aplican a la prosa en el idioma
  que el agente esté produciendo. Interopera con el campo `language` de
  `add-agent-prose-language`: el digest sale en el idioma declarado **y humanizado en ese
  idioma**; sin idioma declarado, en el idioma de la conversación, igualmente humanizado.
- **Humanización subtractiva + ritmo, no aditiva**: la guía toma de `/humanizer` los patrones de
  *eliminación* de tells y la *variación de ritmo*, pero **excluye explícitamente** la sección
  "PERSONALITY AND SOUL" de inyectar opiniones/sentimientos, porque chocaría con la hard rule
  existente del agente *"Never invent work"*. La prosa se vuelve más humana quitando ruido de IA,
  no agregando juicios que los eventos no respaldan.
- **Regeneración de la copia embebida** `cli/internal/scaffold/assets/agents/vector-standup-writer.md`
  vía `go generate` (no se edita a mano).

### Fuera de scope

- **Modificar la skill `/humanizer`** del usuario: vive fuera de Vector (dotfiles personales);
  es referencia de patrones, no dependencia de runtime. Vector debe ser autónomo.
- **Cablear humanización en otros agentes** (autor de specs `raw`, validador, etc.). El alcance
  es **solo `vector-standup-writer`**, aunque la guía pueda reusarse después.
- **Flag por ejecución** (`vector standup --humanize` / `--no-humanize`) o **campo de config**
  (`humanizeStandup`): la humanización es siempre activa (§10).
- **Pase post-generación**: no se añade un segundo agente que reescriba el JSON; toda la
  humanización es prompt-driven dentro del único agente existente.
- **Cambiar el shape del JSON de salida** del agente (`{global, perSpec[]}`) ni el contrato de la
  proyección de standup.
- **Archivo de regla compartido** (`kit/rules/prose/…`): la guía vive en el propio agente
  (decisión §10); promoverla a archivo reusable es una posible extensión futura.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Artefacto principal: **subagente markdown** `kit/agents/vector-standup-writer.md`
  (front-matter `model: haiku`, `tools: Read`). No es código Go; es prompt distribuible.
- Comando que lo invoca: `kit/commands/vector/standup.md` (no se modifica en esta fase).
- Embed/scaffold: `cli/internal/scaffold` vendoriza `kit/{commands,agents,vector}` a
  `assets/` vía `embed.FS`; la copia se regenera con la directiva `//go:generate` de
  `cli/internal/scaffold/scaffold.go:13`.
- Modelo del agente: **Haiku** (no cambia; es un constraint de presupuesto de tokens).

### Versiones relevantes

- Go: **1.26** (`cli/go.mod`) — relevante solo para el paso de `go generate`/build; el cambio de
  contenido es markdown.
- `model` del agente: **haiku** (se mantiene).
- Referencia de patrones: `/humanizer` SKILL.md (29 patrones + PERSONALITY AND SOUL), usado como
  fuente de destilación, no importado.

No usar librerías, APIs, flags o patrones que no estén documentados oficialmente o que no estén
ya presentes en el proyecto, salvo que este spec lo autorice explícitamente.

### Patrones existentes a respetar

- **Kit independiente en runtime**: el agente es markdown distribuible; no importa código de
  `cli/`/`web/` (`architecture/system-boundaries.md`).
- **Paridad fuente↔embed**: tras editar `kit/agents/…`, regenerar `assets/` con `go generate`;
  nunca editar la copia embebida a mano (evita drift). Mismo patrón que `add-agent-prose-language`.
- **Token routing** (`product/token-routing.md`): el agente sigue en Haiku; la guía debe ser
  compacta para no inflar el prompt de un agente barato.
- **Idioma de los artefactos**: la prosa del spec sigue al proyecto (español); la guía dentro del
  agente y los nombres de patrones permanecen en inglés (el agente los interpreta; la salida va en
  el idioma configurado). Slugs/rutas/identificadores en kebab-case inglés.
- **No invención** (hard rule actual del agente): la humanización no puede introducir afirmaciones
  no respaldadas por los eventos.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] Subagente `vector-standup-writer` (`kit/agents/vector-standup-writer.md`, `model: haiku`).
- [x] Copia embebida en `cli/internal/scaffold/assets/agents/vector-standup-writer.md`
      (actualmente idéntica a la fuente).
- [x] Directiva `//go:generate` de regeneración de assets (`cli/internal/scaffold/scaffold.go:13`).
- [x] Comando `/vector:standup` (`kit/commands/vector/standup.md`) que invoca al agente.
- [x] `/humanizer` SKILL.md como **referencia** de patrones (no dependencia de runtime).
- [x] (Coexistencia) Spec `add-agent-prose-language`: introduce el campo `language` y la directiva
      `Write the prose in: <language>` en el mismo agente. Esta fase es ortogonal pero toca el
      mismo archivo; ver §10 sobre el orden de edición.

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta. No
debe inventar contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

**Guía vendorizada en el system prompt del agente** (prompt-driven, single-call). La destilación
del `/humanizer` se incrusta como una sección estática del agente `vector-standup-writer.md`. No
hay archivo de regla externo, no hay inyección por el comando, no hay segundo agente. El agente
sigue siendo el único productor de prosa y consume la guía como parte de sus instrucciones.

Razones (alineado con `product/token-routing.md`): el agente corre en Haiku; meter la guía en su
system prompt evita un segundo call (post-pass) o una inyección por-ejecución desde el comando.
El costo es un incremento acotado y único en el prompt del agente.

### Capas afectadas

- presentation (web/board): **no** — el digest es texto/JSON; no hay UI nueva.
- application/CLI (`cli/cmd/vector`): **no** — ningún flag ni dispatch nuevo.
- domain/config (`cli/internal/config`): **no** — no hay campo de config (siempre activa).
- domain/standup (`cli/internal/standup`): **no** — la proyección y su shape no cambian.
- kit (`kit/agents`): **sí** — se añade la sección de prose quality al agente.
- scaffold/embed (`cli/internal/scaffold/assets`): **sí (regeneración)** — copia embebida del
  agente, regenerada por `go generate`.
- data/estado (`.vector/specs`, `activity.jsonl`, digest persistido): **no** — el shape del
  digest no cambia; sigue siendo JSON válido para `vector standup commit`.

### Flujo esperado

1. Dev ejecuta `/vector:standup` (igual que hoy).
2. El comando corre `vector standup --json` y pasa el JSON al subagente (paso 2 del comando, sin
   cambios). Si `add-agent-prose-language` ya está presente, el comando antepone
   `Write the prose in: <language>`.
3. El subagente `vector-standup-writer` (Haiku) genera el digest aplicando **a la vez** su regla
   de no-invención, la directiva de idioma (si la hay) y la **nueva guía de prose quality**:
   produce prosa humanizada en el idioma correspondiente.
4. El agente retorna el mismo shape `{global, perSpec[]}` (sin cambios de contrato).
5. El comando persiste vía `vector standup commit --digest-file -` (sin cambios) y reporta.

### Ubicación de archivos nuevos

No se crean paquetes, carpetas ni archivos nuevos. Solo se edita el agente existente y se
**regenera** su copia embebida. La guía vive *dentro* de `kit/agents/vector-standup-writer.md`.

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `kit/agents/vector-standup-writer.md` | MODIFICAR | Añadir sección "Prose quality" condensada (subtractiva + ritmo, agnóstica al idioma, sin inyectar opiniones); reforzar que la humanización respeta "Never invent work" y el idioma. | Bloque "Hard rules" del mismo archivo; `.vector/specs/add-agent-prose-language/spec.md` (pattern de edición del mismo agente) |
| `cli/internal/scaffold/assets/agents/vector-standup-writer.md` | REGENERAR | Copia embebida del agente; se regenera con `go generate ./internal/scaffold` (no editar a mano). | Directiva `//go:generate` en `scaffold.go:13` |
| `README.md` | MODIFICAR (opcional) | Nota mínima: el digest del standup se humaniza automáticamente (siempre activo, sin config). Solo si el README ya documenta el standup; si no, omitir. | Sección de standup del README (si existe) |

### Detalle por archivo

#### kit/agents/vector-standup-writer.md

Acción: MODIFICAR

Cambios requeridos:

- Añadir una sección nueva (sugerido título `## Prose quality — write like a human`, tras el
  bloque "Hard rules" y antes de "Output — exact shape") con la guía condensada. Debe cubrir, en
  forma de directivas accionables y compactas (no el catálogo completo), al menos estos patrones
  del `/humanizer`, elegidos por su frecuencia en prosa de standup:
  - **Sin inflación de significado/legado**: nada de *marks a pivotal moment*, *key milestone*,
    *represents a shift*, *sets the stage for*. Describe el cambio, no su "importancia".
  - **Sin colas `-ing` superficiales**: no cierres frases con *…, reflecting steady progress* /
    *…, showcasing the work*. Termina la oración en el hecho.
  - **Vocabulario llano**: evita *crucial, pivotal, leverage, robust, seamless, delve,
    underscore, showcase, vibrant, foster, intricate, testament*. Usa palabras normales.
  - **Copula directa**: *is/are/has*, no *serves as / stands as / boasts / features*.
  - **Sin regla de tres forzada** ni *elegant variation* (no cicles sinónimos del mismo spec).
  - **Sin negaciones paralelas ni colas negativas**: no *not just X, it's Y*; no *no blockers*
    pegado al final — escríbelo como cláusula real (*nothing is blocked*).
  - **Estilo plano**: sin em-dashes para "punch" (usa comas/puntos), sin emojis, sin boldface, sin
    comillas tipográficas; presente, conciso; lidera con el resultado y luego la sustancia.
  - **Sin relleno ni hedging** (*at this point in time*, *it's worth noting that*) ni **cierres
    positivos genéricos** (*good momentum*, *on track for great things*).
  - **Varía el ritmo**: mezcla frases cortas y alguna más larga; no todas con la misma forma.
- Añadir una nota explícita de que esta guía es **subtractiva**: humaniza quitando tells de IA y
  variando el ritmo, **no** agregando opiniones, sentimientos ni juicios (eso violaría la hard
  rule "Never invent work"). El agente **no** debe usar la parte "add soul / have opinions" del
  `/humanizer`.
- Añadir una nota de que la guía es **agnóstica al idioma**: los patrones aplican a la prosa en
  el idioma que el agente produce (es/en/otro), no solo en inglés.
- Mantener intactos: el bloque de Input, las demás Hard rules, el Output shape (`{global,
  perSpec[]}`) y la nota de empty period.

Restricciones:

- No cambiar `model: haiku` ni `tools: Read`.
- No reproducir el `/humanizer` completo (560+ líneas): es una destilación compacta.
- No tocar el contrato de salida ni la lógica de empty period (`no activity since last standup`).
- No introducir dependencia de runtime hacia la skill `/humanizer`.
- No colisionar con la regla de idioma de `add-agent-prose-language` si ya está aplicada (ver
  §10): la nueva sección complementa, no reemplaza, esa regla.

#### cli/internal/scaffold/assets/agents/vector-standup-writer.md

Acción: REGENERAR

- Tras editar la fuente en `kit/agents/`, ejecutar `go -C cli generate ./internal/scaffold`
  (o `go -C cli generate ./...`) para refrescar la copia embebida. **No editar a mano.**
- Verificar que la copia embebida queda **idéntica** a la fuente (`diff` sin salida).

#### README.md

Acción: MODIFICAR (opcional)

- Si el README ya tiene una sección de `/vector:standup`, añadir una línea: el digest se humaniza
  automáticamente (siempre activo, sin configuración). Si el README es etapa-visión y no
  documenta el standup, **omitir** este cambio (no crear sección nueva solo para esto).

Restricciones:

- No documentar flags ni config inexistentes (la feature no añade ninguno).

---

## 7. API Contract

No aplica — no se introduce ni cambia ningún endpoint HTTP, ni el shape del JSON de la proyección
de standup, ni el contrato de salida del agente. La entrada del agente sigue siendo el JSON de
`vector standup --json` (con el campo `language` opcional de `add-agent-prose-language`) y la
salida sigue siendo `{global, perSpec[]}`. La guía de humanización es prompt-side, no data-side.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `kit/agents/vector-standup-writer.md` incluye la sección "Prose quality" condensada con los
      patrones listados en §6, marcada como subtractiva y agnóstica al idioma.
- [ ] El agente conserva `model: haiku`, `tools: Read`, su bloque de Input, sus demás Hard rules
      y el Output shape `{global, perSpec[]}` sin cambios.
- [ ] La copia embebida `cli/internal/scaffold/assets/agents/vector-standup-writer.md` es
      **idéntica** a la fuente tras `go generate` (`diff` vacío).
- [ ] `/vector:standup` en un repo con `language: "es"` produce un digest **en español** y
      **humanizado** (sin colas `-ing`, sin inflación, sin regla de tres forzada).
- [ ] `/vector:standup` sin idioma declarado produce el digest en el idioma de la conversación,
      igualmente humanizado.
- [ ] El digest sigue siendo **JSON válido** consumible por `vector standup commit` (la
      humanización no rompe el shape).
- [ ] El edge case de periodo vacío sigue devolviendo exactamente
      `no activity since last standup` (la guía no lo "humaniza" a otra cosa).
- [ ] La humanización **no inventa** trabajo ni sentimiento no presente en los eventos.

### Tests requeridos

La humanización es cualitativa y prompt-driven; no hay aserción unitaria del estilo de la prosa.
La verificación es:

- [ ] **Paridad fuente↔embed** (automatizable): el test/check de scaffold que compara la fuente
      con la copia embebida pasa (o, en su defecto, `diff` manual vacío).
- [ ] **Manual ES/EN**: ejecutar `/vector:standup` con `language: "es"` y sin idioma; leer el
      digest y confirmar ausencia de los tells de §6.
- [ ] **Manual edge cases**: periodo vacío → mensaje exacto sin cambios; actividad mínima (un
      spec, una transición) → resumen tight sin padding ni inflación.
- [ ] **Sanidad de tokens**: confirmar que el prompt del agente crece de forma acotada (la guía
      es ~20–30 líneas), sin saltar de tier (sigue Haiku).

### Comandos de verificación

```bash
go -C cli generate ./...      # regenera la copia embebida del agente
diff kit/agents/vector-standup-writer.md \
     cli/internal/scaffold/assets/agents/vector-standup-writer.md   # debe ser vacío
gofmt -l cli
go -C cli vet ./...
go -C cli test ./...
go -C cli build ./...
```

La fase no está completa si alguno de estos comandos falla, si `gofmt -l` lista archivos, o si la
copia embebida difiere de la fuente.

---

## 9. Criterios de UX

No aplica — no hay UI ni formularios. La única "UX" es la **legibilidad en voz alta** del digest:
debe sonar como prosa de una persona en standup (concisa, presente, sin muletillas de IA). El
flujo del comando (`vector standup`) no cambia; el output es simplemente mejor redactado.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas; el agente no debe cuestionarlas ni cambiarlas:

- **Integración = condensar en el system prompt del agente** (opción A), no archivo de regla
  externo ni agente post-pass. Razón: simplicidad y token routing (Haiku, single-call).
- **Siempre activa**: sin flag por ejecución ni campo de config. La humanización es comportamiento
  por defecto del agente.
- **Agnóstica al idioma**: guía en inglés, patrones universales; se aplican a la prosa en el
  idioma producido. Interopera con `language` de `add-agent-prose-language` (humaniza en ese
  idioma).
- **Subtractiva, no aditiva**: se toman los patrones de eliminación y la variación de ritmo del
  `/humanizer`; se **excluye** "PERSONALITY AND SOUL" (inyectar opiniones) por chocar con
  "Never invent work".
- **`/humanizer` es referencia, no dependencia**: la guía se vendoriza dentro del agente; Vector
  no lee la skill personal en runtime.
- **Solo `vector-standup-writer`** en esta fase; reuso por otros agentes queda como extensión.
- **Orden de edición vs. `add-agent-prose-language`**: ambas specs editan el mismo archivo. Si
  `add-agent-prose-language` ya se aplicó, esta fase **añade** la sección de prose quality y
  **respeta** la regla de idioma ya presente. Si no se ha aplicado, esta fase no introduce la
  regla de idioma (es otra spec); solo añade prose quality. No se duplica ni reescribe la regla de
  idioma de la otra spec.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, pero
no implementarla.

---

## 11. Edge cases

### Periodo vacío

- `perSpec` vacío → el agente debe seguir devolviendo exactamente
  `{ "global": "no activity since last standup", "perSpec": [] }`. La guía de humanización **no**
  debe reescribir ese mensaje fijo (es un literal que el comando/flujo espera).

### Actividad mínima

- Un solo spec, una sola transición, sin `work` → el resumen debe ser tight (1 frase), sin padding
  ni inflación. La humanización aquí es especialmente importante: no meter significado inventado.

### Idioma

- `language: "es"` declarado → digest en español, humanizado en español (patrones universales).
- Sin idioma → idioma de la conversación, humanizado igual.
- La guía nunca traduce spec ids (siguen verbatim, regla existente).

### Conflicto con no-invención

- Si humanizar "naturalmente" tentara a añadir una opinión o sentimiento (p. ej. *solid progress*,
  *frustrating blocker*) → **prohibido**: la no-invención manda. La prosa se aligera quitando
  tells, no agregando color no respaldado por los eventos.

### Tells residuales

- Si pese a la guía la prosa conserva algún tell → degradación suave: el digest sigue siendo
  válido y útil; no se rompe nada. La calidad es objetivo, no un gate que bloquee el commit.

### JSON

- La guía aplica al **contenido** de los strings `global`/`summary`, nunca a la estructura JSON:
  el output debe seguir siendo JSON válido sin prosa fuera de los campos.

---

## 12. Estados de UI requeridos

No aplica — el cambio no introduce ni modifica componentes de UI. El digest es texto dentro de un
JSON; el board (StandupView, timeline) es read-only y no se ve afectado.

---

## 13. Validaciones

No aplica — la humanización no introduce validaciones de campos ni de servidor. El único
"invariante" verificable es que la salida del agente siga siendo JSON válido con el shape
`{global, perSpec[]}` (lo valida el binario en `vector standup commit`, sin cambios).

---

## 14. Seguridad y permisos

- La guía es metadata de prompt no sensible (sin secretos, tokens ni PII). No se loguea nada
  sensible.
- No se añaden permisos: el agente sigue con `tools: Read` y no escribe estado.
- No se introduce dependencia externa ni acceso a archivos del usuario fuera de los artefactos del
  kit ya scaffoldeados.

---

## 15. Observabilidad y logging

No aplica — no se añade logging. La humanización es una propiedad cualitativa de la prosa, no un
evento de estado observable. El pipeline de proyección/commit del standup no cambia.

---

## 16. i18n / textos visibles

- La guía de humanización se documenta **en inglés** dentro del agente (nombres de patrones:
  *rule of three*, *em-dash overuse*, etc.); su interpretación es multiidioma.
- La **prosa generada** sale en el idioma resuelto (campo `language` de `add-agent-prose-language`
  o idioma de la conversación), humanizada en ese idioma.
- No hay sistema de traducciones de UI que tocar; el digest no es texto de interfaz.

---

## 17. Performance

- **Token cost**: el único costo es el incremento del system prompt del agente por la guía
  (~20–30 líneas). Acotado y único por call; el agente **no** cambia de tier (sigue Haiku). No se
  añade un segundo call (se descartó el post-pass por costo).
- **Latencia**: sin cambios materiales; no hay I/O ni call adicional.
- **Sin trabajo pesado** en el hilo principal; el binario no llama a ningún LLM.

---

## 18. Restricciones

El agente no debe:

- Cambiar el shape del JSON de salida (`{global, perSpec[]}`) ni el contrato de la proyección.
- Afectar la proyección, el commit o el avance del marker del standup.
- Reproducir el `/humanizer` completo dentro del agente (debe ser una destilación compacta).
- Introducir dependencia de runtime hacia la skill `/humanizer` (vive en dotfiles personales).
- Añadir flags (`--humanize`), campos de config (`humanizeStandup`) ni un segundo agente.
- Inyectar opiniones, sentimientos o juicios no respaldados por los eventos (no-invención manda).
- Traducir spec ids ni reescribir el literal `no activity since last standup`.
- Editar a mano la copia embebida del scaffold (debe regenerarse con `go generate`).
- Duplicar o reescribir la regla de idioma de `add-agent-prose-language`.
- Cablear humanización en agentes distintos de `vector-standup-writer` en esta fase.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] Sección "Prose quality" condensada en `kit/agents/vector-standup-writer.md` (subtractiva,
      agnóstica al idioma, respeta no-invención), con el resto del agente intacto.
- [ ] Copia embebida `cli/internal/scaffold/assets/agents/vector-standup-writer.md` regenerada y
      verificada idéntica a la fuente.
- [ ] (Opcional) Nota en `README.md` si ya documenta el standup.
- [ ] Verificación manual ES/EN del digest sin tells de §6, más edge cases (vacío, mínimo).
- [ ] Gate verde: `go generate`, `diff` fuente↔embed vacío, `gofmt -l`, `go vet`, `go test`,
      `go build`.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Añadí solo la sección de prose quality; no toqué Input, Output shape ni las demás Hard rules.
- [ ] La guía es una destilación compacta (~20–30 líneas), no el `/humanizer` completo.
- [ ] Marqué la guía como subtractiva (sin opiniones) y agnóstica al idioma.
- [ ] Respeté la regla de idioma de `add-agent-prose-language` si ya estaba presente; no la dupliqué.
- [ ] Conservé `model: haiku`, `tools: Read` y el literal `no activity since last standup`.
- [ ] Regeneré la copia embebida con `go generate` y verifiqué `diff` vacío contra la fuente.
- [ ] Confirmé que el digest sigue siendo JSON válido con shape `{global, perSpec[]}`.
- [ ] No añadí flags, config, dependencias externas ni un segundo agente.
- [ ] Verifiqué manualmente la prosa en ES/EN y los edge cases (vacío, actividad mínima).
- [ ] Ejecuté `go generate`, `gofmt`, `go vet`, `go test`, `go build` — todos verdes.
- [ ] No dejé logs temporales ni TODOs sin justificar.

---

## Open questions

1. **Promoción a archivo de regla reusable**: si en una fase posterior otros agentes de prosa
   (autor de specs `raw`, etc.) necesitan la misma humanización, ¿se extrae la guía a
   `kit/rules/prose/humanization-patterns.md` y se referencia desde varios agentes? Esta fase la
   deja embebida en el standup-writer; la extracción es una extensión, no parte del alcance.
2. **Nota en README**: el README es etapa-visión y puede no documentar el standup; queda a
   criterio de la implementación si añadir la línea o omitirla (marcado opcional en §6).

> Resueltas en clarificación: integración = condensar en el agente; humanización siempre activa;
> guía agnóstica al idioma (interopera con `language` de `add-agent-prose-language`); subtractiva
> (excluye "PERSONALITY AND SOUL"); scope = solo `vector-standup-writer`.
