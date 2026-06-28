# Spec: Doctrinas compartidas de agentes (`_shared/`)

## 1. Objetivo

Extraer las doctrinas de comportamiento repetidas casi verbatim entre los seis agentes del kit
(`vector-spec-refiner`, `vector-bug-refiner`, `vector-spec-validator`, `vector-comment-evaluator`,
`vector-summary-writer`, `vector-standup-writer`) a ficheros de referencia bajo
`kit/agents/_shared/`, de modo que cada agente referencie el fichero en vez de inlinear el texto.

Esta mejora permite que un cambio editorial en una doctrina compartida se propague a todos los
agentes afectados editando un solo fichero, eliminando el riesgo de divergencia silenciosa entre
versiones duplicadas.

## 2. Alcance

### Incluido en esta fase

- Tres ficheros nuevos bajo `kit/agents/_shared/`:
  - `citation-discipline.md` — doctrina "Cite, don't guess" (usada por refiner, bug-refiner,
    validator, comment-evaluator).
  - `prose-rules.md` — doctrina "Never invent work" + reglas de prosa humanizada (usadas por
    summary-writer y standup-writer).
  - `refiner-base.md` — doctrina "Preserve user language" + "Be terse" (usadas por refiner y
    bug-refiner).
- Modificaciones a los seis agentes existentes en `kit/agents/` para reemplazar el texto
  inline duplicado con una directiva de carga que referencia el fichero `_shared/` correcto.
- Re-ejecución de `go generate` en `cli/internal/scaffold/` para sincronizar
  `cli/internal/scaffold/assets/agents/` con el estado nuevo de `kit/agents/`.
- Actualización de las copias en `.claude/agents/` del propio repo de Vector (dogfooding) para
  reflejar el nuevo split, ya sea vía `vector update` o copia directa.
- Test de consistencia estructural que valide que ningún agente contiene las cadenas de texto
  extraídas de forma inline (guarda de no-regresión contra reinserción accidental).

### Fuera de scope

- Fusión de agentes entre sí. Sus esquemas de salida, tier de modelo y propósito difieren:
  refiner (brief estructurado) ≠ bug-refiner (8 secciones) ≠ validator (veredicto + scores)
  ≠ comment-evaluator (juicio adversarial) ≠ summary-writer (JSON spec único) ≠ standup-writer
  (JSON multi-spec). No se fusionan.
- Extracción de cualquier contenido que no aparezca casi verbatim en ≥2 agentes. El boilerplate
  "You never write state" de los commands (`kit/commands/`) es un recordatorio crítico por
  command y se mantiene inline (documentado en `docs/orchestration-review.md` §7).
- Cambios al binario Go (`cli/`), a la web (`web/`) ni al mecanismo de embed más allá de
  re-ejecutar `go generate`.
- Cambios al spec-template (`kit/vector/spec-template.md`) ni a las rules de `.claude/rules/`.
- Compresión de contenido que reduzca claridad de los gates adversariales (validator,
  comment-evaluator): si extraer una regla compromete la verificabilidad del gate, se deja
  inline. La claridad de los gates es no negociable.

El agente no implementa nada fuera de este alcance aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Agentes del kit: **Markdown con frontmatter YAML** (`name`, `description`, `model`, `tools`).
  Sin runtime de Go/TS — son artefactos distribuidos.
- Embed del binario: **Go 1.26** (`cli/go.mod:3`), directiva `//go:embed all:assets` en
  `cli/internal/scaffold/scaffold.go:26`.
- Generación de assets: script en `//go:generate` (`cli/internal/scaffold/scaffold.go:13`):
  `rm -rf assets && mkdir -p assets && cp -R ../../../kit/commands ../../../kit/agents ../../../kit/vector assets/`

### Versiones relevantes

- Go: `1.26` (citado de `cli/go.mod:3`).
- Sin dependencias externas nuevas — stdlib únicamente.

### Patrones existentes a respetar

- **Estructura de agente**: frontmatter YAML + secciones Markdown. Ver cualquier fichero de
  `kit/agents/vector-*.md` como referencia.
- **Flat agents, no subdirectorios**: hoy todos los agentes están en `kit/agents/*.md`; los
  ficheros en `kit/agents/_shared/` son **recursos de referencia**, no agentes registrables.
  El prefijo `_` señala que no son ficheros de agente directamente invocables.
- **CLI-owns-writes**: el binario es el único escritor del state; esta spec no toca ninguna
  ruta de escritura de estado.
- **Embed automático por `go generate`**: los ficheros de `kit/agents/` (incluyendo
  subdirectorios) se copian a `assets/agents/` y quedan embebidos en el binario; no se editan
  `assets/` a mano.
- **Seeding per-proyecto**: `vector init`/`vector update` siembra `.claude/agents/` del repo
  del usuario desde los assets embebidos (`cli/internal/scaffold/scaffold.go`). El subdirectorio
  `_shared/` seguirá el mismo path de distribución y llegará al repo usuario como
  `.claude/agents/_shared/`.
- **Naming**: kebab-case para ficheros (`citation-discipline.md`, no `citationDiscipline.md`).
- **Idioma de prose**: los ficheros `_shared/` pueden contener instrucciones en inglés (como
  hoy los agentes); el español aplica al cuerpo de este spec.

---

## 4. Dependencias previas

Antes de iniciar la implementación debe existir:

- [x] Los seis agentes fuente en `kit/agents/` (`vector-spec-refiner.md`,
  `vector-bug-refiner.md`, `vector-spec-validator.md`, `vector-comment-evaluator.md`,
  `vector-summary-writer.md`, `vector-standup-writer.md`) — verificados en `kit/agents/`.
- [x] `cli/internal/scaffold/scaffold.go` con la directiva `//go:generate` y `//go:embed` ya
  funcional — verificado en `cli/internal/scaffold/scaffold.go:13,26`.
- [x] `cli/internal/scaffold/assets/agents/` conteniendo copias actuales de los agentes —
  verificado vía `ls`.
- [x] El análisis de doctrinas compartidas y la propuesta de ficheros `_shared/` documentada en
  `docs/orchestration-review.md` §7 (tabla de duplicaciones, nombres de ficheros sugeridos).
- [ ] Decisión abierta sobre el mecanismo exacto de referencia dentro del fichero de agente
  (ver Open questions #1).

Si alguna dependencia no existe, detener y reportar exactamente qué falta.

---

## 5. Arquitectura

### Patrón

Extracción de constante: texto duplicado se mueve a una única fuente canónica; los consumidores
referencian la fuente en lugar de duplicarla. El mecanismo de referencia es una **directiva de
carga en el frontmatter o en la primera sección del agente**, que indica al agente que lea el
fichero `_shared/` al inicio de su ejecución, antes de procesar el input principal.

### Capas afectadas

- **`kit/agents/`**: sí — creación de `_shared/` + modificación de los seis agentes.
- **`cli/internal/scaffold/`**: sí — re-ejecutar `go generate` para sincronizar `assets/agents/`
  (incluido el nuevo subdirectorio `_shared/`).
- **`.claude/agents/`** (dogfooding en el propio repo de Vector): sí — actualización post
  `go generate` vía `vector update` o copia directa.
- **`cli/cmd/vector/`** (binario): no — no se modifican subcomandos ni lógica Go.
- **`web/`**: no.
- **Otros ficheros de `kit/`**: no — commands y rules no se tocan.

### Flujo de carga en runtime (agente modificado)

1. El command invoca el agente (e.g., `/vector:raw` invoca `vector-spec-refiner`).
2. El agente recibe su fichero de definición como system prompt.
3. La directiva de carga al inicio del agente le indica que ejecute `Read` sobre
   `.claude/agents/_shared/<fichero>.md` antes de procesar el input.
4. El agente lee el fichero `_shared/` y aplica la doctrina como si fuera parte de sus Hard rules.
5. El agente procede con su lógica específica (repo scan, output, etc.).

### Flujo de distribución (embed → usuario)

1. Edición en `kit/agents/_shared/*.md` y `kit/agents/vector-*.md`.
2. `go generate ./...` en `cli/internal/scaffold/` → copia `kit/agents/**` a `assets/agents/**`
   (incluyendo `_shared/`).
3. Build del binario embebe `assets/` completo.
4. `vector update` en el repo del usuario siembra `.claude/agents/_shared/` junto con los
   agentes actualizados.

### Ubicación de archivos nuevos

```txt
kit/agents/
  _shared/
    citation-discipline.md   (NUEVO)
    prose-rules.md           (NUEVO)
    refiner-base.md          (NUEVO)
  vector-spec-refiner.md     (MODIFICAR)
  vector-bug-refiner.md      (MODIFICAR)
  vector-spec-validator.md   (MODIFICAR)
  vector-comment-evaluator.md (MODIFICAR)
  vector-summary-writer.md   (MODIFICAR)
  vector-standup-writer.md   (MODIFICAR)

cli/internal/scaffold/assets/agents/
  (sincronizado por go generate — no editar a mano)

.claude/agents/
  (actualizado por vector update o copia manual post-generate)
```

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo a seguir |
|---|---|---|---|
| `kit/agents/_shared/citation-discipline.md` | NUEVO | Doctrina "Cite, don't guess" canónica, referenciada por refiner / bug-refiner / validator / comment-evaluator | `kit/agents/vector-spec-refiner.md` (sección Hard rules, bullets "Cite, don't guess") |
| `kit/agents/_shared/prose-rules.md` | NUEVO | Doctrina "Never invent work" + reglas de prosa humanizada, referenciada por summary-writer / standup-writer | `kit/agents/vector-standup-writer.md` (sección "Prose quality") |
| `kit/agents/_shared/refiner-base.md` | NUEVO | Doctrina "Preserve user language" + "Be terse", referenciada por refiner / bug-refiner | `kit/agents/vector-spec-refiner.md` (Hard rules, bullets "Preserve" y "Be terse") |
| `kit/agents/vector-spec-refiner.md` | MODIFICAR | Reemplazar inline "Cite, don't guess", "Preserve user language", "Be terse" con directivas de carga a `_shared/` | `kit/agents/vector-bug-refiner.md` (patrón de referencia equivalente) |
| `kit/agents/vector-bug-refiner.md` | MODIFICAR | Reemplazar inline "Cite, don't guess", "Preserve user language", "Be terse" con directivas de carga a `_shared/` | `kit/agents/vector-spec-refiner.md` (patrón de referencia equivalente) |
| `kit/agents/vector-spec-validator.md` | MODIFICAR | Reemplazar inline "Cite, don't guess" con directiva de carga a `_shared/citation-discipline.md` | `kit/agents/vector-spec-refiner.md` |
| `kit/agents/vector-comment-evaluator.md` | MODIFICAR | Reemplazar inline "Cite, don't guess" con directiva de carga a `_shared/citation-discipline.md` | `kit/agents/vector-spec-refiner.md` |
| `kit/agents/vector-summary-writer.md` | MODIFICAR | Reemplazar inline "Never invent work" + sección "Prose quality" con directivas de carga a `_shared/prose-rules.md` | `kit/agents/vector-standup-writer.md` |
| `kit/agents/vector-standup-writer.md` | MODIFICAR | Reemplazar inline "Never invent work" + sección "Prose quality" con directivas de carga a `_shared/prose-rules.md` | `kit/agents/vector-summary-writer.md` |
| `cli/internal/scaffold/scaffold_test.go` | MODIFICAR | Agregar test de consistencia estructural: validar que los agentes modificados no contienen los strings extraídos inline | `cli/internal/scaffold/scaffold_test.go` (tests existentes de seeding) |

### Detalle por archivo

#### `kit/agents/_shared/citation-discipline.md` — NUEVO

Debe implementar:
- El texto completo y canónico de la regla "Cite, don't guess": cuándo citar `path:line`,
  qué escribir cuando no hay evidencia (`Sin evidencia — ver [sección]`), qué se considera
  fabricar.
- Aplicabilidad cross-agente: la regla debe redactarse de forma genérica para que aplique a
  refiner (paths de código), bug-refiner (símbolos sospechosos), validator (ficheros citados en
  el spec) y comment-evaluator (ficheros y líneas del diff).

Seguir como referencia: el bullet "Cite, don't guess" en
`kit/agents/vector-spec-refiner.md` (Hard rules) y su clon en `vector-bug-refiner.md`
y `vector-spec-validator.md` (ídem, "Cite, don't hand-wave").

No debe incluir:
- Reglas específicas de un solo agente (e.g., la instrucción de obtener el diff para
  comment-evaluator es específica de ese agente y no va aquí).

#### `kit/agents/_shared/prose-rules.md` — NUEVO

Debe implementar:
- La regla "Never invent work" en su forma unificada: qué cuentan los eventos, qué no se
  debe asumir, comportamiento ante ausencia de eventos.
- Las reglas de prosa humanizada (la sección "Prose quality — write like a human" de
  `vector-standup-writer.md`): lista substractiva de AI tells a evitar (significance
  inflation, `-ing` tails, plain vocabulary, direct copula, no rule of three, plain style,
  no filler/hedging).
- Nota sobre aplicación language-agnostic (ya presente en standup-writer).

Seguir como referencia: secciones "Hard rules" (bullet "Never invent work") y
"Prose quality" de `kit/agents/vector-standup-writer.md` y
`kit/agents/vector-summary-writer.md`.

No debe incluir:
- Reglas de formato de salida (JSON shape, campos `id`, `summary`) — esas son específicas
  de cada agente.
- La regla de `priorSummary` (difiere levemente entre summary-writer y standup-writer en su
  semántica: uno re-emite sustancia, el otro usa como contexto). TBD — ver Open questions #2.

#### `kit/agents/_shared/refiner-base.md` — NUEVO

Debe implementar:
- "Preserve the user's language": español → español, inglés → inglés, kebab-case siempre en
  inglés.
- "Be terse": cada sección es el mínimo útil, sin filler ni repetición del input del usuario.

Seguir como referencia: bullets correspondientes en Hard rules de
`kit/agents/vector-spec-refiner.md` y `kit/agents/vector-bug-refiner.md`.

No debe incluir:
- "Read-only" (no es doctrina compartida por estos dos — el validator también es read-only
  pero con distinto toolset; comment-evaluator tiene git además). Mantener inline en cada agente.
- "No inference of product intent" — también aparece en ambos refiners pero con matices
  distintos (uno habla de feature intent, el otro de bug expected behavior); TBD — ver
  Open questions #3.

#### `kit/agents/vector-spec-refiner.md` — MODIFICAR

Cambios requeridos:
- En la sección **Hard rules**, reemplazar los bullets "Cite, don't guess", "Preserve the
  user's language" y "Be terse" con una directiva de carga: leer
  `.claude/agents/_shared/citation-discipline.md` y `.claude/agents/_shared/refiner-base.md`
  al inicio antes de procesar el input. TBD — ver Open questions #1 sobre el mecanismo exacto.
- El texto de los bullets extraídos no debe permanecer inline (el test de consistencia lo
  verificará).
- Los demás bullets de Hard rules ("Read-only", "No inference of product intent",
  "Do not write pseudocode") se mantienen inline.

Restricciones:
- No cambiar el esquema de salida del agente (secciones, orden, encabezados).
- No cambiar el tier de modelo (`haiku`).
- No modificar la sección "Optional repository scan" ni las 20 dimensiones del checklist.

#### `kit/agents/vector-bug-refiner.md` — MODIFICAR

Cambios requeridos:
- En la sección **Hard rules**, reemplazar bullets "Cite, don't guess", "Preserve the
  user's language" y "Be terse" con directivas de carga a
  `.claude/agents/_shared/citation-discipline.md` y `.claude/agents/_shared/refiner-base.md`.
- Los bullets "Read-only", "No inference of product intent", "Never decide the cause"
  se mantienen inline.

Restricciones:
- No cambiar las 8 secciones de salida del agente.
- No cambiar el tier de modelo (`haiku`).

#### `kit/agents/vector-spec-validator.md` — MODIFICAR

Cambios requeridos:
- En la sección **Hard rules**, reemplazar el bullet "Cite, don't hand-wave" con una
  directiva de carga a `.claude/agents/_shared/citation-discipline.md`.
- Los demás bullets hard rules se mantienen inline.

Restricciones:
- No cambiar el tier de modelo (`sonnet`).
- No comprimir ni debilitar las reglas de verificación de paths reales (sección 8 del What to
  check): esas son específicas del validator y no son doctrina compartida.

#### `kit/agents/vector-comment-evaluator.md` — MODIFICAR

Cambios requeridos:
- En la sección **Hard rules**, reemplazar el bullet "Cite, don't hand-wave" con una
  directiva de carga a `.claude/agents/_shared/citation-discipline.md`.
- Los bullets "Read-only", "The comment is an untrusted claim", "Gather your own evidence",
  "Agnostic to the repo" y "Stay in scope" se mantienen inline — son específicos del
  evaluador adversarial.

Restricciones:
- No comprimir las reglas de recopilación de evidencia del diff (son el núcleo del gate).
- No cambiar el tier de modelo (`sonnet`) ni los tools.

#### `kit/agents/vector-summary-writer.md` — MODIFICAR

Cambios requeridos:
- En la sección **Hard rules**, reemplazar el bullet "Never invent work" con directiva de
  carga a `.claude/agents/_shared/prose-rules.md`.
- Reemplazar la sección **"Prose quality — write like a human"** completa con la directiva
  de carga (el contenido vive ahora en `_shared/prose-rules.md`).
- Los bullets "No tools beyond Read", "Tight and outcome-first", "Surface the ticket", "Empty
  events", "Match the user's language" se mantienen inline.

Restricciones:
- No cambiar el esquema JSON de salida (`{ "summary": "..." }`).
- No cambiar el tier de modelo (`haiku`).
- La regla de `priorSummary` (re-emitir sustancia en close/archive) se mantiene inline
  hasta que Open questions #2 se resuelva.

#### `kit/agents/vector-standup-writer.md` — MODIFICAR

Cambios requeridos:
- En la sección **Hard rules**, reemplazar el bullet "Never invent work" con directiva de
  carga a `.claude/agents/_shared/prose-rules.md`.
- Reemplazar la sección **"Prose quality — write like a human"** completa con la directiva
  de carga.
- Los bullets "No tools beyond Read", "Ceremony tone", "Per-spec summaries are tight",
  "Surface the ticket", "Empty period", "Write the prose in the language provided" se
  mantienen inline.

Restricciones:
- No cambiar el esquema JSON de salida (`{ "global": "...", "perSpec": [...] }`).
- No cambiar el tier de modelo (`haiku`).

#### `cli/internal/scaffold/scaffold_test.go` — MODIFICAR

Cambios requeridos:
- Agregar un test `TestSharedDoctrineNotInlined` (o similar) que:
  1. Lee cada agente modificado desde `assets/agents/`.
  2. Afirma que el contenido del fichero **no** contiene las strings canónicas extraídas
     (e.g., el literal `Cite, don't guess`, la sección `## Prose quality`, el bullet
     `Preserve the user's language`).
  3. Afirma que el directorio `assets/agents/_shared/` existe y contiene los tres ficheros
     esperados.
- Este test garantiza que una futura edición accidental no re-inline la doctrina sin que
  el build lo detecte.

Seguir como referencia: tests existentes en `cli/internal/scaffold/scaffold_test.go`.

Restricciones:
- No cambiar tests existentes de seeding (`TestSeedCommands`, etc.).
- El test debe correr con `go test ./...` sin setup externo; lee directamente desde `embed.FS`
  o desde el filesystem de test.

---

## 7. API Contract

Sin API surface HTTP — no aplica. Esta feature no expone endpoints ni cambia la interfaz del
binario CLI. El único "contrato" es estructural: el esquema de los ficheros `_shared/` (Markdown
plano sin frontmatter YAML — son recursos de lectura, no agentes registrables) y la directiva de
carga en cada agente modificado (TBD — ver Open questions #1).

---

## 8. Criterios de éxito

- [ ] Los tres ficheros `kit/agents/_shared/*.md` existen y contienen la doctrina extraída
  de forma completa y sin truncar.
- [ ] Los seis agentes modificados **no** contienen los bloques de texto inline extraídos
  (verificable con grep sobre el fichero).
- [ ] Los seis agentes modificados contienen una directiva de carga que referencia el/los
  fichero/s `_shared/` correcto/s.
- [ ] `go generate ./...` en `cli/internal/scaffold/` produce un `assets/agents/_shared/`
  sincronizado sin errores.
- [ ] `cli/internal/scaffold/assets/agents/_shared/` contiene los tres ficheros (`citation-discipline.md`,
  `prose-rules.md`, `refiner-base.md`) tras re-generar.
- [ ] El test de consistencia `TestSharedDoctrineNotInlined` pasa.
- [ ] Los tests existentes de scaffold pasan sin regresiones.
- [ ] Un agente lanzado en un repo con la nueva versión sembrada carga y aplica la doctrina
  del fichero `_shared/` correctamente (verificación manual o via `/vector:raw` en dogfooding).
- [ ] Ninguna de las seis funcionalidades de agente se degrada: refiner sigue generando briefs
  estructurados, validator sigue bloqueando specs débiles, etc.

### Tests requeridos

- [ ] `TestSharedDoctrineNotInlined`: verifica ausencia de strings extraídas en cada agente.
- [ ] `TestSharedFilesExist`: verifica existencia de los tres ficheros `_shared/` en
  `assets/agents/_shared/`.
- [ ] Tests de seeding existentes siguen verdes: `go test ./...` en `cli/internal/scaffold/`.

### Comandos de verificación

```bash
# Desde la raíz del repo
go generate ./cli/internal/scaffold/...
gofmt -l cli && go -C cli vet ./... && go -C cli test ./...

# Verificación manual de ausencia de texto inline (ejemplo):
grep -r "Cite, don't guess" kit/agents/vector-spec-refiner.md && echo "FAIL: still inline" || echo "OK"
grep -r "Prose quality" kit/agents/vector-summary-writer.md && echo "FAIL: still inline" || echo "OK"

# Verificar que _shared/ existe en assets tras go generate:
ls cli/internal/scaffold/assets/agents/_shared/
```

---

## 9. Criterios de UX

Esta feature no tiene UI web. Los criterios de UX aplican a los **dos contextos de uso**
del cambio:

### Developer editando una doctrina

- **Fuente única visible:** al editar `kit/agents/_shared/citation-discipline.md`, el developer
  ve exactamente qué agentes la leen (documentado en el encabezado del fichero `_shared/` y en
  este spec). No hay búsqueda necesaria para encontrar dónde vive la doctrina.
- **Sin ambigüedad sobre qué mantener:** si un agente aún contiene texto inline duplicado, el
  test de consistencia falla y lo reporta claramente antes del merge.

### Agente ejecutándose en runtime

- **Carga explícita y auditable:** la directiva de carga en el agente debe ser legible por un
  humano que revise el fichero del agente, no oculta.
- **Fallo visible, no silencioso:** si el fichero `_shared/` no existe en el repo del usuario
  (porque aún no corrió `vector update`), el agente debe reportar el error de lectura en vez de
  continuar silenciosamente sin la doctrina. TBD — ver Open questions #4 sobre comportamiento de
  fallback.
- **Sin latencia perceptible:** la carga del fichero `_shared/` es una operación `Read` local
  (<100ms); no hay cambio perceptible en el tiempo de respuesta del agente.

### Accesibilidad

- Salida de los agentes en texto plano y JSON (sin cambio respecto al estado actual).

---

## 10. Decisiones tomadas

- **Extracción sin fusión:** los agentes mantienen identidades y esquemas de salida separados.
  Solo se extrae el texto común. *Por qué:* fusionar cambiaría la interfaz de los agentes (su
  nombre, descripción, outputs), generando regresiones en los commands que los invocan.
- **`kit/agents/_shared/` como ubicación:** el subdirectorio `_shared/` dentro de `agents/`
  es consistente con el mecanismo de embed existente (el `go generate` copia `kit/agents/**`
  recursivamente). No se necesita una nueva ruta de embed. *Por qué:* mínimo cambio
  infraestructural; consistente con `architecture/distribution-packaging.md`.
- **Prefijo `_` en el nombre del directorio:** señala que los ficheros no son agentes
  registrables por Claude Code. *Por qué:* convención estándar para recursos internos no
  directamente invocables.
- **Tres ficheros, no uno:** la granularidad por doctrina (citation / prose / refiner-base)
  permite que un agente cargue solo lo que necesita. *Por qué:* la summary-writer no necesita
  citation-discipline; cargar un fichero monolítico sería ineficiente e impreciso.
- **Doctrina de priorSummary queda inline hasta resolución de Open questions #2:** las
  semánticas difieren entre summary-writer y standup-writer; extraer sin resolver el delta
  crearía una fuente de verdad incorrecta para uno de los dos.
- **Ganancia es mantenibilidad, no tokens en runtime:** una corrida de un agente carga solo
  su propio prompt; la duplicación no multiplica tokens de inference. El beneficio es editorial
  (un cambio → todos). *Por qué:* documentado en `docs/orchestration-review.md` §7.

Si el agente ve una alternativa mejor, la reporta como observación; no la implementa.

---

## 11. Edge cases

### Fichero `_shared/` no encontrado en el repo del usuario

- Escenario: el usuario tiene una versión anterior del binario o no ha corrido `vector update`.
  Los agentes sembrados en `.claude/agents/` apuntan a `_shared/` pero el directorio aún no
  existe porque la versión anterior no lo incluía.
- Comportamiento esperado: el agente reporta el error de `Read` con un mensaje claro que
  incluya el path esperado y sugiera correr `vector update`. No debe fallar silenciosamente
  omitiendo la doctrina. TBD — mecanismo concreto, ver Open questions #4.

### Texto inline residual (regresión accidental)

- Escenario: alguien edita un agente y re-inserta el texto inline por error.
- Comportamiento esperado: el test `TestSharedDoctrineNotInlined` falla y bloquea el merge.

### Divergencia entre `kit/agents/` y `assets/agents/` (go generate no re-ejecutado)

- Escenario: se modifica `kit/agents/` pero no se re-ejecuta `go generate`.
- Comportamiento esperado: el build del binario embebe la versión vieja. El developer lo detecta
  comparando `kit/agents/` con `assets/agents/` (diferencia de contenido). Mitigación: documentar
  `go generate` como paso obligatorio antes del build en el pipeline.

### Fichero `_shared/` parseado como agente por Claude Code

- Escenario: Claude Code intenta registrar `_shared/citation-discipline.md` como un agente
  invocable con nombre `_shared:citation-discipline` (si el harness explora subdirectorios).
- Comportamiento esperado: los ficheros `_shared/` no deben tener frontmatter YAML de agente
  (`name:`, `description:`, `model:`, `tools:`). Sin frontmatter válido, Claude Code no los
  registra como agentes. TBD — ver Open questions #5 sobre el comportamiento exacto del harness.

### Doctrina parcialmente extraída

- Escenario: se extrae "Cite, don't guess" de refiner pero no de bug-refiner.
- Comportamiento esperado: el test de consistencia cubre todos los agentes identificados;
  una extracción parcial hace fallar el test para los agentes no procesados.

### Sin HTTP surface

- Los códigos HTTP (400/401/403/404/409/422/429/500) no aplican a este cambio.

---

## 12. Estados de UI requeridos

Sin componente de UI web. Estados relevantes del proceso de implementación y distribución:

| Estado | Descripción | Acción disponible |
|---|---|---|
| idle | `kit/agents/` en estado pre-extracción (duplicación inline presente) | Iniciar implementación |
| in-progress | Ficheros `_shared/` creados; agentes modificados; `go generate` pendiente | Ejecutar `go generate` |
| generated | `assets/agents/_shared/` sincronizado; tests corribles | Correr `go test ./...` |
| success | Tests verdes; no hay inline residual; `vector update` propaga al repo | Cerrar spec |
| error (inline residual) | `TestSharedDoctrineNotInlined` falla | Completar la extracción |
| error (go generate no corrido) | `assets/` desincronizado con `kit/` | Correr `go generate` |
| error (fichero _shared/ no existe en usuario) | `vector update` no corrido en el repo destino | Correr `vector update` |
| disabled | No aplica — sin componente interactivo | — |
| offline | No aplica — operación local, sin red | — |

---

## 13. Validaciones

### Validaciones estructurales (post-implementación)

| Elemento | Regla | Verificación |
|---|---|---|
| `_shared/citation-discipline.md` | Existe en `kit/agents/_shared/` | `ls kit/agents/_shared/citation-discipline.md` |
| `_shared/prose-rules.md` | Existe en `kit/agents/_shared/` | `ls kit/agents/_shared/prose-rules.md` |
| `_shared/refiner-base.md` | Existe en `kit/agents/_shared/` | `ls kit/agents/_shared/refiner-base.md` |
| Agentes modificados | No contienen inline el texto extraído | `TestSharedDoctrineNotInlined` |
| Agentes modificados | Contienen directiva de carga a `_shared/` | grep / test |
| `assets/agents/_shared/` | Sincronizado tras `go generate` | `ls cli/internal/scaffold/assets/agents/_shared/` |
| Ficheros `_shared/` | Sin frontmatter YAML (`name:`, `model:`, `tools:`) | grep / test |

### Validaciones de agente (runtime)

No aplica en sentido formal — los agentes son markdown. La "validación" es la correcta carga del
fichero `_shared/` al inicio de la ejecución, cuyo éxito o fallo se refleja en el comportamiento
del agente (doctrina aplicada o error de Read reportado).

---

## 14. Seguridad y permisos

- Los ficheros `_shared/` son markdown plano, sin secrets ni tokens. No hay riesgo de exposición.
- El mecanismo de embed (`embed.FS`) ya existente incluye cualquier fichero bajo `assets/` —
  no se amplía la superficie de embed.
- El seeding vía `vector init`/`vector update` escribe en `.claude/agents/` del repo del
  usuario: operación existente, bajo las salvaguardas de `security/destructive-ops-consent.md`.
  No se introduce ninguna operación destructiva nueva.
- Los ficheros `_shared/` no contienen instrucciones que amplíen permisos de los agentes
  (sus tools son los declarados en el frontmatter de cada agente, sin cambio).

---

## 15. Observabilidad y logging

El único mecanismo de logging relevante es el comportamiento del agente al cargar (o fallar al
cargar) el fichero `_shared/`:

- **Carga exitosa:** sin evento especial — el agente procede normalmente.
- **Fallo de Read:** el agente debe surfacear el error (path not found, permissions) para que el
  developer entienda que la doctrina compartida no se aplicó.

No se modifica `activity.jsonl` ni el Token Savings Meter — este cambio no genera eventos de
dominio (ninguna spec, ticket ni transición de estado se ve afectada).

---

## 16. i18n / textos visibles

El proyecto no tiene sistema de i18n. Los ficheros `_shared/` contienen instrucciones en inglés
para los agentes (consistente con el idioma de los prompts de agentes existentes). No hay textos
de cara al usuario final que cambien con este spec.

La tabla es únicamente documental de los identifiers internos:

| Identificador (doc) | Contenido |
|---|---|
| `shared.citation-discipline` | Cuerpo de `_shared/citation-discipline.md` — doctrina "Cite, don't guess" |
| `shared.prose-rules` | Cuerpo de `_shared/prose-rules.md` — "Never invent work" + humanización |
| `shared.refiner-base` | Cuerpo de `_shared/refiner-base.md` — "Preserve language" + "Be terse" |

---

## 17. Performance

- **Runtime (por-spawn):** cada agente afectado ejecuta un `Read` adicional al inicio de su
  ejecución para cargar el/los fichero/s `_shared/`. Costo estimado: <100ms de I/O local,
  ~0.3–0.5k tokens de contexto adicional. Marginal — documentado como tal en
  `docs/orchestration-review.md` §7 ("costo de tokens en runtime es marginal").
- **Build:** el `go generate` es ya parte del pipeline de build; el nuevo subdirectorio `_shared/`
  se incluye automáticamente con el mismo script de copia. Sin overhead adicional de build.
- **Sin I/O redundante:** cada agente lee solo los ficheros `_shared/` que necesita (uno o dos),
  no un monolítico completo.
- **Sin caching adicional requerido:** el caching del template/checklist (anotado en
  `docs/orchestration-review.md` §11) es un concern separado y no se mezcla aquí.

---

## 18. Restricciones

El agente no debe:
- Fusionar agentes entre sí ni cambiar su esquema de salida, tier de modelo, ni lista de tools.
- Extraer contenido que no aparezca casi verbatim en ≥2 agentes (no overextract).
- Comprimir reglas que son el núcleo de los gates adversariales (validator, comment-evaluator).
- Añadir frontmatter YAML (`name:`, `model:`, `tools:`) a los ficheros `_shared/`.
- Editar `cli/internal/scaffold/assets/agents/` a mano — ese directorio es generado.
- Cambiar el script `//go:generate` en `cli/internal/scaffold/scaffold.go` (ya cubre
  subdirectorios recursivamente con `cp -R`).
- Modificar `vector-spec-validator.md` de modo que debilite su política de verificación de paths
  reales (secciones 6, 8 del "What to check"), aunque esa regla comparte familia con
  "Cite, don't hand-wave".
- Instalar dependencias externas.
- Resolver Open questions mediante suposición — marcarlas como TBD y detenerse.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `kit/agents/_shared/citation-discipline.md` — doctrina de citación completa.
- [ ] `kit/agents/_shared/prose-rules.md` — "Never invent work" + reglas de prosa.
- [ ] `kit/agents/_shared/refiner-base.md` — "Preserve language" + "Be terse".
- [ ] Los seis agentes modificados sin texto inline extraído.
- [ ] Los seis agentes modificados con directiva de carga a `_shared/` correcto/s.
- [ ] `go generate` corrido; `cli/internal/scaffold/assets/agents/_shared/` sincronizado.
- [ ] Test `TestSharedDoctrineNotInlined` (y `TestSharedFilesExist`) en
  `cli/internal/scaffold/scaffold_test.go` passing.
- [ ] Tests existentes de scaffold y de `cli/` verdes sin regresiones.
- [ ] `.claude/agents/` del repo de Vector actualizado (via `vector update` o copia manual).
- [ ] Open questions documentadas como TBD en el spec si no se resolvieron antes de
  implementar (especialmente #1 sobre el mecanismo de directiva de carga).

---

## 20. Checklist final para el agente

- [ ] Leí este spec completo.
- [ ] Revisé `docs/orchestration-review.md` §7 (tabla de duplicaciones, nombres sugeridos de
  ficheros `_shared/`).
- [ ] Verifiqué que los seis agentes fuente existen en `kit/agents/` y en
  `cli/internal/scaffold/assets/agents/`.
- [ ] Confirmé el texto exacto a extraer en cada agente (leyendo los ficheros, no de memoria).
- [ ] Resolví o marqué como TBD la Open question #1 (mecanismo de directiva de carga) antes
  de modificar los agentes.
- [ ] Extraje únicamente el contenido identificado — no overextracté ni fusioné agentes.
- [ ] Verifiqué que los ficheros `_shared/` no tienen frontmatter YAML.
- [ ] Ejecuté `go generate ./...` en `cli/internal/scaffold/` y verifiqué que `assets/agents/_shared/`
  se creó correctamente.
- [ ] Ejecuté `gofmt -l cli && go -C cli vet ./... && go -C cli test ./...` — todos verdes.
- [ ] Ejecuté los greps de verificación de ausencia inline sobre cada agente modificado.
- [ ] No modifiqué `assets/agents/` a mano.
- [ ] No cambié esquemas de salida, tiers de modelo, ni tools de ningún agente.
- [ ] No dejé TODOs sin justificar ni `[...]` sin reemplazar (o marcados como TBD con su
  Open question referenciada).

## Open questions

1. **Mecanismo exacto de la directiva de carga en el fichero de agente:** ¿cómo se expresa
   la instrucción de cargar `_shared/`? Opciones: (a) una sección `## Shared doctrine` al
   inicio del agente con la instrucción `Read .claude/agents/_shared/citation-discipline.md
   before proceeding`; (b) una entrada en el frontmatter (si el harness lo soporta); (c) un
   bloque de instrucción en la primera sección Hard rules. El mecanismo debe ser reconocido
   y ejecutado por el agente de forma fiable. TBD — requiere confirmar con el harness de
   Claude Code.
2. **Doctrina de `priorSummary` entre summary-writer y standup-writer:** summary-writer
   re-emite la sustancia del prior summary en close/archive; standup-writer lo usa solo como
   contexto de framing. ¿Son suficientemente distintas para mantener inline en cada agente o
   se puede unificar con un párrafo condicional? TBD — decidir al implementar.
3. **"No inference of product intent"** aparece en spec-refiner y bug-refiner con matices
   distintos (feature intent vs. expected bug behavior). ¿Incluir en `refiner-base.md` con
   formulación genérica o mantener inline en cada agente? TBD — requiere lectura comparada
   de ambas formulaciones para confirmar si el delta es relevante.
4. **Comportamiento de fallback cuando `_shared/` no existe en el repo del usuario:**
   ¿el agente reporta el error y se detiene, o continúa sin la doctrina (silenciosamente
   degradado)? La opción recomendada es error visible, pero el mecanismo depende de cómo el
   harness expone fallos de `Read`. TBD — verificar con el harness.
5. **Registro de ficheros `_shared/` como agentes por el harness:** ¿Claude Code indexa
   subdirectorios de `.claude/agents/` como agentes? Si `.claude/agents/_shared/` y su
   contenido fueran indexados, crearían entradas de agente inválidas. TBD — verificar el
   comportamiento del harness antes de decidir el mecanismo de distribución.
