# Vector — Visión (idea cruda, sin implementar)

> Estado: captura inicial. **No implementar nada todavía.** Este doc registra la idea
> tal como fue descrita para poder seguir la conversación. Pendiente: imagen del UI del
> kanban + ejemplo del board web.

## Qué es

Una herramienta que trabaja **en conjunto con Claude Code** para organizar proyectos que
implementan tickets + scrum/kanban, pero **developer-focused**. Permite trabajar con
OpenSpec, pero organizado de una manera muy específica donde:

- El código del usuario recibe un "improvement" (la herramienta es agnóstica al código; el
  repo del usuario es aparte — Vector solo aporta **estructura** de manejo con agentes).
- Se manejan muy bien los **tokens de Claude**, usando agentes más "baratos" para tareas
  triviales o investigación que no requiere lógica.

La propuesta es un **ecosistema** (skills + memorias + rules) que estandariza la
organización del repo y facilita el trabajo en equipo.

## Principios

- **Developer-focused**, no project-manager-focused.
- **Agnóstico al código del usuario**: Vector organiza/estructura; no impone arquitectura.
- **Eficiencia de tokens**: ruteo de tareas a agentes baratos vs. caros según complejidad.
- **Comercialización/distribución desde el día 0** (no es solo para uso personal).
- Unifica e incorpora una herramienta ya existente del usuario llamada **`devup`**.

## Techstack

- **CLI: Go** (decidido).
- **Panel web: PENDIENTE** (preguntar al usuario — ver Preguntas abiertas).
- Otras stacks: a definir según se necesite.

## Workflow ideal — V1

1. **Instalación desde GitHub**: un script (`curl … | install.sh` o similar) para instalar
   desde el CLI sin pasos manuales.
2. **`/vector init`** dentro de Claude, en el repo elegido. Detecta cómo está organizado:
   - Checks (no exhaustivos): techstack, git convention, commit convention, versions,
     repo type (mono / micro / etc.).
   - Luego **pide permiso EXPLÍCITO** para crear un **backup del estado actual** (ignorando
     lo del `.gitignore` o similar en cada stack) antes de reorganizar al formato que Vector
     requiere (formato a definir tras analizar repos de referencia + `/biz`).
3. **JSON de estado/record**: lleva el registro de lo que pasa. Ej.: al crear un "spec" con
   `/vector:raw [text]` (equivalente al actual `/idea` del usuario, pero alineado a cómo
   Vector guarda specs).
4. **Administración sobre el JSON**: a los specs se les puede agregar, p. ej., el link del
   ticket correspondiente, etc.
   - Regla: cada vez que Claude hace algo con un rule, se le **recuerda mantener el JSON
     up to date**.
   - **Panel web local**: levantar en un puerto disponible y poco usado un panel donde el
     dev administra Vector: un **board (kanban)** que muestra el estado actual de cada spec
     abierto. (Ejemplo concreto del board: PENDIENTE — el mensaje original se cortó aquí.)

## Investigación pendiente (alto valor, bajo consumo)

- Cómo implementar de forma **eficiente y de bajo consumo** el panel web local sincronizado
  con el JSON de estado, y el patrón para mantener ese JSON actualizado por los rules.

## Comandos (nomenclatura tentativa)

- `/vector init` — detectar y estructurar el repo.
- `/vector:raw [text]` — crear un spec (equivalente a `/idea` actual).

## Conceptos heredados

- **`devup`**: herramienta ya implementada por el usuario; misma idea a **unificar** dentro
  de Vector. (Relacionado con el skill local `devup-setup`: lanzar el dev local vía bloque
  `run:` en `.project-structure`.)
- **OpenSpec**: el modelo de specs sobre el que se apoya el flujo.

## Repos de referencia a estudiar (solo estructura, NO código)

Objetivo: revisar estructura, puntos en común y mejoras, **enfocado únicamente en la
estructura de manejo con agentes** (sistema de documentación, no el código).

- `/Users/mariocampbell/Developer/Personal/cdr/`
- `/Users/mariocampbell/Developer/Personal/private-wealth/`
- `/Users/mariocampbell/Developer/somnio/`

## Preguntas abiertas

1. **Stack del panel web** (board kanban): ¿qué tecnología? (ver opciones en la conversación).
2. **`/biz`**: el usuario pidió usar `/biz` para "hacer una lista" sobre los 3 repos. Aclarar
   el foco (¿análisis de comercialización de Vector vs. análisis de estructura de docs?) y si
   se ejecuta ahora o después — dado el "aún no hagamos nada".
3. Formato objetivo al que `/vector init` reorganiza el repo (definir tras el análisis).
4. Nombre/forma exacta del JSON de estado y su esquema.
