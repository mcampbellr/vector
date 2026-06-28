# Design — dispatcher-json-retry

## Decisiones clave

- **Shape-gate en el command, no en el binario**: la capa de orquestación es el command; el
  binario ya valida con `json.Unmarshal` en `runStandupCommit` y en `spec summarize commit`.
  El retry en el command reduce la tasa de error al binario sin duplicar responsabilidades ni
  modificar `cli/`.
- **1 retry fijo, mismo tier (Haiku)**: el fallo de JSON es ruido puntual del LLM, no
  insuficiencia de capacidad del modelo; promover a Sonnet/Opus violaría `product/token-routing.md`.
  Más de un intento multiplica el costo sin garantía proporcional de mejora.
- **Retry con input original + directive de corrección explícita**: el agente ya tiene todo el
  contexto; solo necesita el recordatorio de formato. No se reduce ni modifica el JSON de
  proyección en el retry.
- **Política diferenciada gate / no-gate heredada del comportamiento existente**: standup es
  gate (el marcador no debe avanzar sin digest válido — perder el registro del período es
  irreversible); summary en apply es no-gate (patrón ya establecido en `apply.md` §7: "Empty/
  invalid prose → nothing is written, not a gate").
- **Sin persistencia del evento retry**: el re-spawn es resiliencia de orquestación, no un
  evento del dominio del board. `activity.jsonl` solo registra acciones de dominio; el retry
  no lo es.
- **Sin stripping de prosa**: el command no intenta extraer el JSON si el output contiene prosa
  alrededor. Parsear la respuesta completa; si falla → retry. Enmascarar el problema sería
  silenciar el síntoma.
- **Atomicidad preservada**: el shape-gate ocurre antes del pipe al binario; nunca se pipea
  JSON inválido. El binario o escribe todo o no escribe nada.

## Superficie

- `kit/commands/vector/standup.md`: insertar bloque **"Validate the digest (shape-gate)"**
  entre §2 y §3 — check de `{global: string non-empty, perSpec: array}`, re-spawn con
  directive, política gate (abort + mensaje accionable si doble fallo).
- `kit/commands/vector/apply.md` §7: insertar shape-gate entre el paso 2 y el paso 3 —
  check de `{summary: string non-empty}`, re-spawn con directive, política no-gate (skip +
  nota en §8 si doble fallo).
- `cli/internal/scaffold/assets/commands/vector/standup.md`: copia en sync con `kit/`.
- `cli/internal/scaffold/assets/commands/vector/apply.md`: copia en sync con `kit/`.

No se crean paquetes, carpetas ni abstracciones nuevas. El pattern es inline en los commands.

## Flujo — standup

```
§1 proyección JSON
↓
§2 vector-standup-writer (Haiku)
↓
[NUEVO] shape-gate (intento 1)
  válido → §3 (pipe al binario)
  inválido →
    notar al usuario: "subagent returned invalid JSON — retrying (attempt 2/2)…"
    re-spawn con mismo JSON + directive de corrección
    shape-gate (intento 2)
      válido → §3
      inválido → reportar fallo accionable, abortar (marcador no avanza, nada escrito)
↓
§3 vector standup commit (sin cambios)
↓
§4 report (sin cambios)
```

## Flujo — apply §7

```
§7.1 proyección JSON (vector spec summarize --json)
↓
§7.2 vector-summary-writer (Haiku)
↓
[NUEVO] shape-gate (intento 1)
  válido → §7.3 (pipe al binario)
  inválido →
    re-spawn con mismo JSON + directive de corrección
    shape-gate (intento 2)
      válido → §7.3
      inválido → skip no-gate (nota en §8: "summary skipped: subagent returned invalid JSON twice")
↓
§7.3 vector spec summarize <id> commit (sin cambios, o saltado)
↓
§8 report (sin cambios, salvo nota de skip si aplica)
```

## Contratos de validación

| Subagente | Shape | Condición "válido" |
|---|---|---|
| `vector-standup-writer` | `{global: string, perSpec: [{id: string, summary: string}]}` | JSON parseable; `global` non-empty; `perSpec` array (puede ser `[]`) |
| `vector-summary-writer` | `{summary: string}` | JSON parseable; `summary` non-empty |
| `vector-spec-composer` | TBD | TBD — spec propio |

## Directive de corrección (retry prompt)

```
The previous attempt returned malformed or invalid JSON.
Return ONLY a valid JSON object matching exactly:
<shape exacto del agente>
No preface, no code fences, no trailing text.
```

El JSON de proyección original se re-adjunta sin modificar.

## Strings visibles (hardcoded EN, consistente con el resto del proyecto)

| Contexto | Texto |
|---|---|
| Retry en curso | `subagent returned invalid JSON — retrying (attempt 2/2)…` |
| Doble fallo standup | `standup digest failed: the subagent returned invalid JSON twice; nothing was written and the marker was not advanced. Re-run /vector:standup to retry.` |
| Doble fallo apply summary | `summary skipped: subagent returned invalid JSON twice` |
