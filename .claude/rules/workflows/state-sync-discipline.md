# Workflows — Disciplina de sincronización del estado

> Aplica a: cualquier rule, skill o comando que cree o modifique specs/board. Operacionaliza
> el invariante de `architecture/state-model.md`.

El JSON de estado es la única fuente de verdad del board. Si se desincroniza, el producto
miente. Esta disciplina es no negociable.

## Regla central

- **Cada vez que Claude ejecuta algo que afecta el dominio (crear/editar/mover un spec,
  añadir un ticket, cambiar estado/etapa/prioridad), actualiza el JSON de estado en el mismo
  paso** — antes de dar la tarea por terminada.
- Las skills del `kit/` deben **recordar explícitamente** mantener el JSON up-to-date como
  parte de su definición (es un requisito de diseño de cada skill, no un afterthought).

## Cómo

- La escritura del JSON pasa por la API/CLI de `cli/` (escritura serializada); ningún
  componente edita el archivo por fuera de ese canal.
- Tras escribir, la frescura del board (`updated N sec ago`) debe reflejar el cambio.
- Sync de bajo consumo: no recargar/reescribir todo el estado por un cambio puntual.

## Verificación de la disciplina

- Una acción de dominio que no deja rastro en el JSON es un bug, aunque el efecto visible
  parezca correcto.

> Estado: pendiente — el mecanismo de recordatorio dentro de cada skill se concreta al diseñar
> el `kit/` y el patrón de sync (ver `docs/vision.md`).
