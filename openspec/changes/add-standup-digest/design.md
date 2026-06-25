# Design — add-standup-digest

## Context

El board ya proyecta `state` read-only y sirve `/api/board` + `/api/events` (SSE). El activity
log es append-only (`Store.AppendEvent`, `Store.ReadEvents`) y hoy solo guarda `status.changed`.
Falta el detalle de "trabajo hecho" para una ceremonia de scrum, y un lugar donde el equipo lo
vea sin que el dev pegue texto.

## Goals / Non-Goals

**Goals:**
- Enriquecer la traza con `work.logged` (aditivo) desde `/vector:apply`.
- Proyección determinista por spec/periodo en un paquete nuevo (`cli/internal/standup`), sin LLM.
- Digest NL global + por-spec generado por agente **Haiku**, persistido y servido por la API.
- StandupView + SpecTimeline en el board, alimentadas por dos endpoints GET read-only.

**Non-Goals:**
- `/vector:daily`, exportación externa (Slack/Jira), plantillas personalizables, burndown.
- Editar/borrar eventos desde la UI; persistir el digest committed/compartido.
- Que el binario Go llame a un LLM o agregue dependencias externas.

## Decisions

- **CLI-owns-writes**: el binario es el único escritor de `activity.jsonl` y `standup.json`; el
  command nunca edita `.vector/` a mano (`workflows/state-sync-discipline.md`).
- **`work.logged` aditivo**: nuevo `EventType` + payload tipado (type switch, sin `any`); no
  altera eventos existentes ni `SpecState`. Un consumidor viejo lo ignora.
- **Ventana por defecto = desde el marcador**; el marcador avanza al persistir el digest
  (flujo de un solo paso). `activity.jsonl` retiene todo, así que avanzar no destruye historial.
- **Proyección determinista**: filtra por el `ts` del evento, nunca `time.Now()`.
- **Generación NL fuera del binario**: la prosa la produce el agente del command (Haiku) →
  binario sin dependencias de LLM/red, distribución de un solo binario (`product/token-routing.md`).
- **Digest + marcador personales/gitignored** (`.vector/local/standup.json`), como `activity.jsonl`.
- **API local read-only**: solo GET; aplican 400 (`since` inválido), 404 (spec inexistente en
  `/api/activity`), 500 (lectura del log). No 401/403/409/422/429.

## Superficie

- `cli/internal/state/event.go`: `EvtWorkLogged` + `WorkLoggedData`.
- `cli/internal/state/store.go`: `WorkLog(...)`, `ReadStandup()/WriteStandup(digest, marker)`.
- `cli/internal/standup/standup.go`: `Project(events, since) Projection` (+ tests).
- `cli/cmd/vector/main.go`: `runStandup` (`standup` / `standup commit`) y `case "worklog"`.
- `cli/internal/board/server.go`: `GET /api/standup`, `GET /api/activity?spec=<id>`.
- `kit/`: `commands/vector/standup.md`, `agents/vector-standup-writer.md`, modify `apply.md`.
- `web/`: `StandupView`, `SpecTimeline`, `useStandup`, `types/standup.ts`.

## Risks / Trade-offs

- **Log grande**: `/api/activity` proyecta on-demand → leer streaming línea a línea; truncar la
  timeline en UI a N eventos (sugerido 20, confirmar al implementar `web/`).
- **Líneas JSONL corruptas**: saltar y continuar (log a stderr), no abortar el resumen.
- **Marcador que avanza**: una corrida sin actividad igual avanza el marcador (corrida válida);
  aceptable porque el historial se conserva en `activity.jsonl`.
- **Espejo manual de tipos** `cli/ ↔ web/` hasta que exista typegen; los tipos TS espejan
  exactamente las formas de §7 del spec.
