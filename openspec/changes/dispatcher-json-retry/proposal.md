# Dispatcher JSON retry

## Why

Los subagentes Haiku (`vector-standup-writer`, `vector-summary-writer`) emiten JSON
estructurado que el command pipea directamente al binario. Cuando el modelo devuelve JSON
truncado, malformado o rodeado de prosa, el binario rechaza el commit y el usuario queda con
un estado inconsistente: el marcador de standup no avanzó pero tampoco hay un mensaje
accionable claro, o apply abortó en §7 y el spec no transicionó. El problema es puntual (ruido
del LLM, contexto largo), no sistémico, y un único re-spawn resuelve la mayoría de los casos.
Hoy no existe ningún mecanismo de recuperación en la capa de orquestación del command: si el
JSON es inválido, el flujo falla sin intentar corregirse.

## What changes

- **Shape-gate en `standup.md`**: entre el paso de generación del `vector-standup-writer` (§2)
  y el pipe al binario (§3), el command valida el shape `{global, perSpec[]}`. Si falla, hace
  un re-spawn único con la directive de corrección + el mismo JSON de proyección. Si el segundo
  intento también falla, reporta el error al usuario y aborta sin escribir nada ni avanzar el
  marcador.
- **Shape-gate en `apply.md` §7**: entre la generación del `vector-summary-writer` y el pipe a
  `vector spec summarize … commit`, validación del shape `{summary}`. Política no-gate: si ambos
  intentos fallan, el summary se salta y apply continúa; §8 lo menciona en el reporte.
- **Contrato de validación documentado**: reglas mínimas (parseabilidad + presencia de campos +
  tipos) para `vector-standup-writer` y `vector-summary-writer`; `vector-spec-composer` queda
  como TBD a cerrar en su propio spec.
- **Assets embebidos en sync**: `cli/internal/scaffold/assets/commands/vector/standup.md` y
  `apply.md` se actualizan en el mismo paso (copia de `kit/`).

## Scope

**In:**
- Inserción del shape-gate + retry inline en `kit/commands/vector/standup.md` (entre §2 y §3).
- Inserción del shape-gate + retry inline en `kit/commands/vector/apply.md` §7 (entre paso 2 y
  paso 3).
- Mensaje de retry al usuario en stdout mientras ocurre el re-spawn.
- Mensaje de fallo accionable (standup gate) y nota de skip (apply no-gate) cuando el doble
  fallo se produce.
- Actualización de los assets embebidos (`cli/internal/scaffold/assets/commands/vector/`).

**Out:**
- Cambios al binario Go (`cli/`): la validación `json.Unmarshal` existente permanece intacta
  como red de seguridad; no se modifica.
- Cambios a los agentes (`vector-standup-writer.md`, `vector-summary-writer.md`): ya dicen
  "emit valid JSON only"; no hay nada que añadir.
- Más de un retry por invocación: el límite es fijo en 1 re-spawn.
- Promoción de tier de modelo en el retry: el segundo intento usa Haiku, igual que el primero.
- Cobertura de `vector-spec-composer`: shape TBD; se cerrará en su propio spec.
- Reducción automática del input de proyección ante truncamiento sistemático: TBD post-V1.
- Telemetría del retry en `activity.jsonl`: el re-spawn no es un evento de dominio.
- Cambios de UI web o panel.

Authored spec: `.vector/specs/dispatcher-json-retry/spec.md`
