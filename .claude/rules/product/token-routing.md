# Product — Ruteo de tokens

> Aplica a: diseño de commands/agentes del `kit/` y cualquier flujo que invoque modelos.

La eficiencia de tokens es un requisito de primera clase. El principio: **rutear cada tarea al
agente más barato capaz de resolverla**.

## Reglas de ruteo

- **Tareas triviales o de investigación sin lógica** (búsquedas, listados, lectura de
  estructura, resúmenes, clasificación) → agentes baratos.
- **Tareas que requieren lógica, diseño o decisiones de arquitectura** → agentes caros, solo
  cuando aportan valor que el barato no puede.
- Preferir trabajo de **bajo consumo y alto valor**: p. ej. la investigación del patrón de
  sync JSON ↔ board se enfoca en eficiencia (ver `docs/vision.md`).

## Aplicación en el kit

- Las skills que Vector distribuye deben declarar/elegir el tier de agente apropiado por
  paso, no usar el modelo más caro por defecto.
- Documentar en cada skill del `kit/` por qué un paso usa el tier que usa.

## Disciplina de estado

- El ahorro no debe romper la integridad del JSON de estado: un agente barato que toca estado
  igual debe respetar `workflows/state-sync-discipline.md`.

> Estado: pendiente — taxonomía concreta de tiers de agente y el mapa tarea→tier se definen al
> diseñar las primeras skills del `kit/`.
