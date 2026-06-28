# Design — surface-ticket-in-standup-digest

## Context

El standup digest ya proyecta la actividad por spec (`cli/internal/standup`), la enriquece en la
capa de comando (`enrichProjection`, que ya lee cada `spec` del store para llenar
`Title`/`LastStatus`), la pasa al agente Haiku `vector-standup-writer` para la prosa, y persiste el
digest reconstruido (`runStandupCommit` → `.vector/local/standup.json`). El spec ya puede estar
ligado a un ticket externo (`SpecState.Ticket`, struct `state.Ticket{Provider, Key, URL, Auto}`),
pero nada de ese ticket llega al digest: el spec solo se nombra por su slug interno.

## Goals / Non-Goals

**Goals:**
- Hilar el `Ticket` existente del spec a través del camino del standup ya establecido, sin nueva
  capa ni cambio de firma de `Project`.
- Que el agente lidere con `ticket.key` junto al slug (per-spec + global), solo la key.
- Que el board Standup view muestre un badge de ticket junto al slug, espejando `SpecCard`.

**Non-Goals:**
- Cambiar el slug `id` o la join key del commit; el ticket es aditivo, nunca reemplazo.
- Auto-linkeo de tickets (ya de `/vector:link` y la auto-detección raw/sync).
- URL o nombre de provider en la prosa (key only; la URL vive solo en el `title` del badge).
- Múltiples tickets por spec, re-linkeo, o cambiar el contrato de `vector standup commit`.
- Tocar la lógica del marcador, el activity-log schema, u otros comandos (`/vector:daily`, etc.).

## Decisions

- **Reusar `state.Ticket` y el badge de `SpecCard`**: cero tipos o UX nuevos. El web espeja el
  contrato Go a mano en `web/src/types/standup.ts` (no hay typegen todavía).
- **Enriquecimiento en `enrichProjection`, no en `Project`**: `Project` se mantiene store-free; el
  `Ticket` se llena en el caller que ya tiene acceso al store. `sa.Ticket = spec.Ticket` se asigna
  incondicionalmente (nil para specs sin link es el valor correcto).
- **El ticket llega al digest persistido desde la proyección fresca en commit**, no del payload del
  agente: el `agentDigest` de entrada (`id` + `summary`) no cambia. `runStandupCommit` copia
  `sa.Ticket` en `StandupSpecDigest.Ticket`.
- **Campos aditivos + `omitempty`**: digests viejos deserializan sin cambio; no se bumpea
  `StandupSchemaVersion` salvo que un reviewer lo exija.
- **Slug inmutable como join key**: el output del agente mantiene `perSpec[].id` = slug verbatim;
  la key del ticket nunca va en `id`.
- **Generación NL fuera del binario**: la regla key-next-to-slug vive en el prompt del agente; el
  binario no llama LLM (distribución de un solo binario, `product/token-routing.md`).

## Superficie

- `cli/internal/standup/standup.go`: `Ticket *state.Ticket \`json:"ticket,omitempty"\`` en `SpecActivity`.
- `cli/cmd/vector/standup.go`: `enrichProjection` setea `sa.Ticket = spec.Ticket`; `runStandupCommit`
  añade `Ticket: sa.Ticket` al literal `state.StandupSpecDigest{...}`.
- `cli/internal/state/standup.go`: `Ticket *Ticket \`json:"ticket,omitempty"\`` en `StandupSpecDigest`.
- `kit/agents/vector-standup-writer.md`: input example con `ticket`, hard rule key-next-to-slug
  (key only), reafirmar slug verbatim en `id`, output shape sin cambio.
- `web/src/types/standup.ts`: `ticket?: Ticket` (import `Ticket` de `./board`).
- `web/src/components/StandupView/StandupSpecRow.tsx`: badge junto al `specId` cuando `spec.ticket`
  está presente (texto = `key`, `title` = `url`), espejando `SpecCard.tsx:20`–`23`.

## Risks / Trade-offs

- **Espejo manual de tipos `cli/ ↔ web/`**: hasta que exista typegen, `standup.ts` debe espejar
  exactamente la forma Go; el `Ticket` ya existe en `board.ts`, se reusa.
- **Spec read falla en enrichment**: `enrichProjection` ya hace `continue` cuando `ReadSpec` falla
  → `Ticket` queda nil (slug-only). No se cambia este fallback.
- **Misma key de ticket en varios specs**: permitido (sin constraint de unicidad); cada spec
  reporta su propio ticket independientemente.
- **Ticket malformado/parcial al agente**: el prompt obliga a caer al slug si falta `key` y a
  emitir JSON válido igual; no debe romper la prosa.
