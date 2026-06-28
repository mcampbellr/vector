# Revisión arquitectónica — Framework de orquestación de Vector

> Revisión completa del framework de orquestación (commands `/vector:*`, subagentes del
> kit, binario Go, board web). Foco: máxima calidad de orquestación minimizando cómputo,
> latencia y costo, **sin** sacrificar calidad de implementación. Fecha: 2026-06-27.

## 1. Resumen ejecutivo

Vector **no es** un orquestador multi-agente clásico (planner → workflow-builder → fleet).
Es un **patrón de orquestación delgado**: cada `/vector:*` es un prompt markdown ejecutado en
la sesión principal de Claude, que descompone el trabajo en (a) llamadas deterministas al
binario Go y (b) subagentes baratos de alcance estrecho. El binario tiene el **monopolio de
escritura** del estado; los agentes solo leen y producen prosa/artefactos.

El diseño base es sólido y **no debe tocarse** en sus invariantes: CLI-owns-writes, state
machine en el binario, `state.json` shardeado por spec, `activity.jsonl` append-only,
validator adversarial separado del refiner, evaluador escéptico de comentarios con
re-verificación en el main loop. Son aciertos de calidad, no candidatos a optimización.

Los hallazgos reales se concentran en tres ejes, ninguno resuelto por reducir prompts:

1. **El orquestador corre íntegro en Opus.** La disciplina de token-routing se aplica a los
   pasos periféricos (refine, summarize, validate → Haiku/Sonnet) pero el trabajo generativo
   central — componer el spec de 20 secciones en `raw`/`bug` — ocurre inline en el main loop,
   es decir en Opus. Se rutea lo barato y se deja lo caro en el tier más caro. Hallazgo #1.
2. **Lógica determinista ejecutándose en el modelo caro.** `detectTicket` (tiers 1–6) y la
   detección de lenguaje están implementadas como prosa de prompt en `raw.md`, corriendo en
   Opus y **duplicando** lógica que el binario ya tiene en Go para `sync`.
3. **Re-derivación por comando.** Cada comando re-globea specs de ejemplo, re-detecta
   lenguaje, re-descubre build/lint/test, re-lee config. Nada se cachea entre comandos salvo
   `language`.

Más un eje de mantenibilidad: **tres copias** de cada agente/command (`kit/`, `.claude/`,
`cli/internal/scaffold/assets/`) que deben sincronizarse a mano, y **siete doctrinas**
copiadas inline entre agentes.

La propuesta **no es un rediseño**: es la evolución del patrón existente hacia un **dispatcher
delgado** — sesión principal como router + interacción, subagentes tipados con tier declarado
para toda generación/análisis, binario como única fuente de lógica determinista.

## 2. Arquitectura actual

```
SESIÓN PRINCIPAL (Claude Code — modelo del usuario = OPUS 4.8)
  /vector:* (prompt markdown, 51–233 líneas)  ← TODO esto en Opus:
   ├─ parse $ARGUMENTS
   ├─ read .vector/config.json
   ├─ glob example specs + detect language   ← re-derivado por comando
   ├─ git blame/log (bug) — deducción de causa
   ├─ detectTicket tiers 1-6 (raw)           ← lógica determinista
   ├─ AskUserQuestion (clarify loop ≤5)
   ├─ COMPONER spec 20 secciones (raw/bug)   ← GENERACIÓN PESADA EN OPUS
   ├─ IMPLEMENTAR código (apply/comment)     ← genuinamente caro
   └─ reportar (prosa user-facing)
   spawn subagentes ▼            binary calls ▼
  SUBAGENTES (read-only)         BINARIO `vector` (Go) — única escritura
   Haiku: spec-refiner,           spec create/propose/apply/status/close/
     bug-refiner, summary-writer,   archive/link/relate/worklog/route
     standup-writer               state machine (LOCK), activity.jsonl (apnd)
   Sonnet: spec-validator,        board API + SSE, scaffold (embed.FS),
     comment-evaluator              pricing/token meter
                                  → .vector/*.json → WEB BOARD (read-only, SSE)
```

Pipeline real de `/vector:raw` (el caso más representativo):

```
Opus: parse → config → glob+lang → detectTicket
  ▼ spawn
Haiku (refiner): raw idea → brief                       [barato ✓]
  ▼
Opus: AskUserQuestion clarify  →  COMPONER 20 secciones  [⚠ CARO en main loop]
  ▼ spawn
Sonnet (validator): audita spec, re-lee template+example+repo  [correcto ✓]
  ▼  PASS/BLOCK (≤3 ciclos)
Opus: vector spec create --body-file -  →  vector spec route x2  →  report
```

### Costo/contexto/modelo por etapa (raw, estimado)

| Etapa | Inputs | Outputs | Contexto | Modelo | Tokens | Latencia |
|---|---|---|---|---|---|---|
| parse + config + glob + lang | $ARGS, config, fs | vars | bajo | **Opus** | ~1–3k | baja |
| detectTicket tiers | raw text, config | TICKET_JSON | bajo | **Opus** | ~1k | baja |
| refine | raw, example, template | brief | medio | Haiku | ~3–8k | media |
| clarify (AskUserQuestion) | brief | respuestas | bajo | **Opus** | ~1–2k/ronda | alta |
| **componer spec** | brief, template, respuestas | spec 20 secc. | alto | **Opus** | **~8–15k** | media |
| validate | spec, template, example, repo | verdict | alto | Sonnet | ~5–12k | media |
| create + route | spec | card | bajo | **Opus** | ~1k | baja |
| report | todo | prosa | bajo | **Opus** | ~1k | baja |

El cuello generativo (componer) y ~6k de orquestación trivial corren en Opus. Solo
refine+validate están ruteados.

## 3. Arquitectura propuesta (dispatcher delgado)

La sesión principal solo enruta, interactúa (AskUserQuestion) y llama al binario. Toda
generación/análisis va a subagentes con tier declarado. La lógica determinista baja al binario.

```
SESIÓN PRINCIPAL = DISPATCHER DELGADO
  ├─ parse args + resolver ambigüedad (AskUserQuestion)
  ├─ llamar binario (lecturas + escrituras serializadas)
  └─ reportar
   NO compone, NO detecta tickets/lenguaje, NO re-globea
   spawn (tier declarado) ▼      binary ▼
  GENERATIVE BACK                BINARIO (ampliado)
   Haiku: refiner(s), prose       + vector detect-ticket / detect-lang
   Sonnet: spec-composer ◀NUEVO   + vector context (example,lang,cmds) cacheado @init
           spec-validator         summary templado si no hay work.logged
           comment-eval           state machine, API, SSE
   → artefactos por referencia (path), no copiados al main ctx
```

Pipeline propuesto de `/vector:raw`:

```
Dispatcher: parse → vector context (1 call: example, lang, cmds, ticket)
  ▼ spawn Haiku refiner → brief (path)
Dispatcher: AskUserQuestion clarify
  ▼ spawn Sonnet spec-composer (brief + respuestas + template) → spec.md en disco
  ▼ spawn Sonnet spec-validator (path al spec) → verdict (≤3 ciclos, composer corrige)
Dispatcher: vector spec create --body-file <ya en disco> → route → report
```

Diferencias clave: composición sale de Opus → Sonnet; el spec se escribe a disco antes del
create (recuperable); el main context nunca carga las 20 secciones completas (pasan por path
entre subagentes); `detectTicket`/lang/cmds bajan al binario en una llamada cacheada.

## 4. Flujo de información

```
$ARGUMENTS → [dispatcher] ── lee ──▶ config.json (lang, cmds, examplePath cacheados @init)
                                          ▲ vector init: detecta UNA vez, persiste
   vector context (binario) ──▶ {examplePath, language, buildCmd, ticketDetected}
   (pasa PATHS, no contenidos)
        refiner(Haiku) ──reads──▶ repo, example  ──▶ brief.md (disco)
        composer(Sonnet) ──reads──▶ brief.md, template ──▶ spec.md (disco)
        validator(Sonnet) ──reads──▶ spec.md, template, repo ──▶ verdict
        vector spec create --body-file spec.md ──▶ state.json + activity.jsonl
        vector spec route ──▶ telemetría;  board.json (derivado) ──SSE──▶ web
```

Regla de transferencia: entre etapas viajan referencias (paths) + datos estructurados, nunca
documentos completos re-copiados al contexto del orquestador. Hoy el spec compuesto vive en el
contexto de Opus; en la propuesta solo existe en disco y los subagentes lo leen por path.

## 5. Matriz de responsabilidades de agentes

| Agente | Resp. correcta | ¿Overlap? | Modelo actual | Modelo correcto | Veredicto |
|---|---|---|---|---|---|
| spec-refiner | raw → brief | filosofía con bug-refiner | Haiku | Haiku | OK |
| bug-refiner | bug → brief investigación | doctrina con spec-refiner | Haiku | Haiku | OK |
| **spec-composer** *(nuevo)* | brief+respuestas → spec 20 secc. | hoy lo hace el main loop (Opus) | — (Opus inline) | **Sonnet** | **CREAR** |
| spec-validator | auditar spec vs checklist | no | Sonnet | Sonnet | OK |
| comment-evaluator | evaluar comentario vs diff | no | Sonnet | Sonnet | OK |
| summary-writer | prosa por-spec post-acción | 90% con standup-writer | Haiku | Haiku (condicional, §10) | Reducir invocación |
| standup-writer | prosa digest multi-spec | 90% con summary-writer | Haiku | Haiku | OK, extraer prose-rules |

Decisiones: crear `spec-composer` (Sonnet) — único cambio estructural de agentes. No fusionar
refiner/bug-refiner (esquemas 13 vs 8 secciones) ni summary/standup (input shapes distintos);
su solapamiento es de doctrina, no de estructura → extraer prose-rules a fichero referenciado.

## 6. Matriz de clasificación de contexto

| Fuente | Clasificación | Razonamiento | Acción |
|---|---|---|---|
| Spec template (20 secc.) | Siempre (raw/bug) | Define el artefacto | Ya por path ✓ |
| Perfect Spec Checklist | Siempre (refiner+validator) | Contrato de calidad | Extraer a fichero referenciado |
| Example spec | Usualmente (raw/bug) | Tono/idioma/profundidad | Cachear path @init |
| `language` | Siempre | Idioma de prosa | Ya en config ✓ |
| `config.json` | Siempre | Ubicación/comportamiento | OK (JSON pequeño) |
| build/lint/test | Usualmente (apply/comment) | Gate de calidad | Cachear @init |
| Git history | Condicional (bug) | Deducción de causa | OK — por-invocación |
| PR diff | Condicional (comment) | Evidencia | OK — el evaluador lo trae él mismo |
| Repo conventions/code | Condicional | Estilo al implementar | Lazy, solo apply/comment |
| Prior summary | Condicional | Continuidad de prosa | OK — vía binario |
| `.claude/rules` | Rara vez en runtime | Para devs, no agentes kit | No inyectar a subagentes kit |

## 7. Optimización de prompts

La duplicación de doctrinas entre agentes **no** multiplica tokens en una corrida (cada spawn
carga solo su propio prompt). El costo es mantenibilidad, no runtime. Dónde sí hay ganancia:

| Patrón | Dónde | Ganancia | Acción |
|---|---|---|---|
| "Cite, don't guess" | refiner, bug-refiner, validator, comment-eval | Mantenibilidad | `_shared/citation-discipline.md` |
| "Never invent work" + prose-rules (≈95%) | summary + standup | Mant. + leve runtime | `_shared/prose-rules.md` |
| "Preserve language" / "Be terse" (≈95%) | refiner + bug-refiner | Mantenibilidad | `_shared/refiner-base.md` |
| Boilerplate "You never write state" | 9 commands | Mantenibilidad | Mantener (recordatorio crítico) |
| detectTicket tiers como prosa | raw.md | **Runtime real (Opus)** | Mover al binario (§8) |
| Checklist verbatim | validator + refiner | Runtime leve | Referenciar fichero |

No comprimir nada que reduzca claridad de los gates (validator, comment-evaluator).

## 8. Selección de modelos

| Etapa | Capacidad | Modelo recomendado | Tokens | Escalar si… | Bajar si… |
|---|---|---|---|---|---|
| Parse / dispatch | Ninguna | Sesión | <1k | — | — |
| detectTicket / detect-lang | Determinista | **Binario (0 tokens)** | 0 | nunca | — |
| Work-item selection (apply) | Determinista | **Binario** (`spec next`) ✓ | 0 | nunca | — |
| Git cause deduction (bug) | Mecánico + matching | **Haiku** (hoy Opus) | ~3–6k | candidatos ambiguos | — |
| Refine | Estructuración | Haiku ✓ | ~3–8k | idea muy ambigua | — |
| **Compose spec** | Generación estructurada | **Sonnet** (hoy Opus inline) | ~8–15k | validator BLOCK 3× | — |
| Validate / Comment eval | Juicio escéptico | Sonnet ✓ | ~5–12k | — | spec trivial |
| **Implement (apply)** | Razonamiento + código | **Opus/Sonnet** (calidad-crítico) | grande | arquitectura | tarea mecánica → Sonnet |
| Prose summary | Trivial | Haiku ✓ / binario-template | <1k | — | sin work.logged → binario |
| Report | Trivial | Sesión | <1k | — | — |

Varias ejecuciones baratas > una cara (raw): Haiku-refine + Sonnet-compose + Sonnet-validate
produce un spec con gate adversarial por menos costo que una composición Opus de un solo paso
sin auditoría. La calidad sube (el validator atrapa lo que un solo paso no revisa), el costo
baja. Es Pareto, no trade-off. Premium condicional (apply): Opus solo cuando toca
arquitectura/contratos; Sonnet en wiring/CRUD. Exponer como `applyModel` en config.

## 9. Pre-execution hooks

Hoy no hay hooks reales — cada command re-hace el setup. Un único pre-hook determinista en el
binario, `vector context`, que el dispatcher llama una vez:

| Hook | Pertenece | Razón |
|---|---|---|
| Parse | Dispatcher | Trivial |
| Repo-init gate | Binario (`vector context` falla con guía) | Determinista |
| `vector context` → {examplePath, language, build/lint/test, ticketDetected, applyMode} | **Binario, cacheado @init** | Elimina re-derivación; 0 tokens |
| Intent detection | **Eliminar** | El namespace `/vector:X` ya es el intent |
| Workflow planning | **Eliminar** | Cada command es un workflow fijo |
| Agent selection | Dispatcher (estático) | Mapeo command→agente es fijo |

Eliminar todo "planner/router/intent-detector" implícito: el comando explícito ya determina
el flujo. Añadirlo sería overengineering contra el principio del repo.

## 10. Post-execution hooks

| Hook | Pertenece | Acción |
|---|---|---|
| State write / activity append / SSE | Binario ✓ | Mantener |
| Token route | Binario, invocado por command | Mejorar exactitud |
| **Prose summary** | Condicional | Binario templa si la ventana no tiene `work.logged`; Haiku solo si hay trabajo sustantivo |
| Lint/format/test (apply) | Binario detecta; command ejecuta | Usar cmds cacheados |
| Artifact registration | Binario (openspec en propose) ✓ | Mantener |

Summary condicional: hoy apply/archive/close/propose/status invocan summary-writer + 2
round-trips en toda transición. Para archive/close/status/link la ventana suele no tener
`work.logged` (evento estructural: `review → closed`). El binario puede templarlo sin LLM.
Telemetría: el meter se auto-reporta con tokens estimados; débil para un commercialization
wedge — capturar uso real donde el harness lo exponga, si no etiquetar como estimación.

## 11. Caching

| Cache | Lifetime | Invalidación | Costo | Beneficio |
|---|---|---|---|---|
| `language` | Permanente | `vector init --language` | nulo | Ya existe ✓ |
| examplePath + idiom | Hasta nuevo spec idiomático | init/manual | trivial | Elimina glob por-command |
| build/lint/test | Permanente | cambio de manifests | trivial | Elimina re-descubrimiento |
| ticketDetected | Por-invocación | — | — | Determinista en binario, 0 tokens |
| board.json (derivado) | Por-serve | fingerprint .vector/ | bajo | Ya existe ✓ |
| Template / checklist | Estático (embebido) | release del kit | nulo | Referenciar, no inline |
| Composed spec.md | Hasta create | — | bajo | Persistir antes de create → recuperación |

## 12. Recuperación de fallos

| Escenario | Hoy | Propuesta |
|---|---|---|
| Validator BLOCK | Cap 3 ciclos, stop sin registrar ✓ | Mantener; el composer corrige entre ciclos |
| Crash tras componer, antes de create | Spec perdido (vivía en contexto Opus) | Persistir spec.md a disco al componer; create lo lee → reanudable |
| JSON malformado de subagente | No especificado | Dispatcher valida y re-spawnea (1 retry); fallo → reporta, no escribe |
| Binario ausente | Reporta, no edita a mano ✓ | Mantener |
| Apply a mitad | worklog additivo no-gate ✓ | Reanudar desde `spec next` (in-progress prioritario) ✓ |

## 13. Ejecución paralela

La orquestación por-command es un pipeline secuencial (refine → compose → validate → create)
con dependencias reales; poca paralelización intra-command. Donde existe:

| Operación | Paralelizable | Sincronización |
|---|---|---|
| `vector context` (example, lang, cmds, ticket) | Sí (goroutines) | Antes del refiner |
| git blame + log -S + log --grep (bug) | Sí (3 procesos) | Antes de mapear candidatos |
| Lecturas del board API | Sí (stateless) | — |
| refine vs validate | No (validate necesita el spec) | Barrera natural |

Cuello real: latencia humana (AskUserQuestion) y de modelo (compose/validate). La palanca
principal es sacar compose de Opus, no el paralelismo. No introducir fan-out artificial.

## 14. Matriz costo vs calidad

| # | Recomendación | Ahorro | Latencia | Δ Complejidad | Mant. | Riesgo | Δ Calidad | Rec. |
|---|---|---|---|---|---|---|---|---|
| 1 | Compose → subagente Sonnet | Alto | ↓ | +1 agente | + | Bajo (validator gate) | ↑ | **P0** |
| 2 | detectTicket+lang → binario | Medio | ↓ | media | ++ | Bajo | = | **P0** |
| 3 | `vector context` cacheado | Medio | ↓ | media | ++ | Bajo | = | **P1** |
| 4 | Summary templado | Bajo-medio | ↓ | baja | + | Muy bajo | = | **P1** |
| 5 | Colapsar 3 copias | nulo runtime | = | media | +++ | Bajo | = | **P1** |
| 6 | Doctrinas a `_shared/` | leve | = | baja | ++ | Bajo | = | P2 |
| 7 | applyModel condicional | Alto (apply) | ↓ | media | + | **Medio** | =/↓ vigilar | P2 |
| 8 | Token-meter exactitud | nulo | = | media | + | Bajo | = | P3 |
| 9 | Persistir spec.md antes de create | nulo | = | baja | + | Muy bajo | ↑ | (en P0 #1) |
| 10 | Retry JSON malformado | nulo | = | baja | + | Bajo | = | P3 |
| — | Tocar CLI-owns-writes / state machine / sharding | — | — | — | — | **Alto** | ↓ | **NO** |
| — | Fusionar summary+standup | leve | = | +branch | − | Bajo | = | **NO** |

## 15. Verificación de preservación de calidad

**#1 Compose → Sonnet.** El spec es input al implementador; un spec peor degradaría
implementación. Mitigación: el validator (Sonnet) es gate adversarial con cap 3 ciclos — un
spec débil no pasa; hoy el spec Opus ya pasa por ese mismo validator. El composer trabaja
desde un brief refinado + template fijo (menos grados de libertad que generación libre) →
alucinación ≈ igual o menor. **Calidad preservada o mejorada (Pareto).**

**#2 detectTicket → binario.** Lógica determinista (tiers 1–6); en Go es más fiable que como
prosa de prompt y elimina la divergencia latente entre la versión Go (sync) y la prompt (raw).
**Calidad mejorada.**

**#4 Summary templado.** Solo para ventanas sin `work.logged` (transiciones estructurales).
"Movido a closed" no requiere LLM. **Sin pérdida.**

**#7 applyModel — único con riesgo medio.** Sonnet en mecánico: probablemente sin pérdida;
en arquitectónico: posible. Por eso es condicional con criterio explícito (archivos tocados,
contratos públicos), default = Opus, opt-in. Debugging/review idéntico (worklog, commits
atómicos). **No activar por defecto.**

Transversal: los `.claude/rules` no se inyectan a subagentes kit (correcto); los constraints
de dominio viven en el binario (no se toca). Ninguna recomendación se justifica solo por
tokens: #1/#2/#9 mejoran calidad; #3/#4/#5/#6/#8/#10 son neutrales en calidad y positivas en
mantenibilidad/latencia; #7 va con gate + opt-in.

## 16. Roadmap priorizado

**P0 — alto valor, bajo riesgo:**
1. `spec-composer` (Sonnet) + sacar composición del main loop en `raw`/`bug`. Persistir
   `spec.md` a disco al componer (cubre #9 de la matriz).
2. Bajar `detectTicket` y detección de lenguaje al binario (`vector detect-ticket`, exponer
   `language`). Eliminar la lógica-como-prosa de `raw.md`; unificar con `sync`.

**P1 — consolidación:**
3. `vector context`: una llamada con {examplePath, language, build/lint/test, applyMode,
   ticketDetected}, cacheado en `config.json` por `vector init`.
4. Summary templado en binario para transiciones sin `work.logged`.
5. Fuente única de kit: colapsar `kit/` ↔ `.claude/` ↔ `scaffold/assets/` vía `go generate`.

**P2 — calidad de mantenimiento:**
6. Extraer doctrinas compartidas a `kit/agents/_shared/*.md` referenciadas.
7. `applyModel` opt-in (Sonnet/Opus condicional) con criterio explícito y default = Opus.

**P3 — pulido:**
8. Exactitud / etiquetado honesto del token-meter.
9. Retry de subagente ante JSON malformado en el dispatcher.

**Fuera de scope (no tocar):** CLI-owns-writes, state machine LOCKED, sharding per-spec,
append-only activity, separación refiner/validator, evaluador escéptico + spot-check.

## Nota de cierre

El sistema actual está bien diseñado; no requiere rediseño. El patrón "comando explícito →
binario determinista + subagentes baratos, binario dueño del estado" es el correcto para esta
clase de herramienta y escala a comandos nuevos sin cambios arquitectónicos. Las dos palancas
que de verdad mueven costo/latencia sin tocar calidad son **(1) sacar la composición
generativa de Opus** y **(2) devolver la lógica determinista al binario**. El resto es
consolidación incremental.
