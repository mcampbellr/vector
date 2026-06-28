# Vector

Herramienta developer-focused que trabaja en conjunto con Claude Code para organizar
proyectos (tickets + scrum/kanban) sobre OpenSpec, con foco en eficiencia de tokens
(ruteo a agentes baratos para tareas triviales) y estructura estandarizada para equipos.

> ⚠️ Estado: captura inicial de la idea. **Nada implementado todavía.**
> Ver [`docs/vision.md`](docs/vision.md).

## Stack

- CLI: **Go**
- Panel web (kanban): por definir

## Configuración — idioma de la prosa de los agentes

Los agentes de Vector que generan prosa (hoy el digest de standup) escriben, por defecto, en el
idioma de la conversación. Para fijar un idioma por repo, usa el flag `--language`:

```sh
vector init --language es      # al inicializar el repo
vector update --language es    # set/cambio en un repo ya inicializado
```

El valor es libre (etiqueta BCP-47 como `es`/`es-MX` o un nombre como `Spanish`/`español`); no se
valida contra una lista. Se persiste como el campo `language` en `.vector/config.json`:

```json
{ "language": "es" }
```

Semántica: **ausente o vacío** = los agentes igualan el idioma de la conversación (comportamiento
actual). Cuando está presente, el binario lo expone en la proyección de `vector standup --json` y
`/vector:standup` se lo pasa al agente como directiva, de modo que el digest sale en ese idioma
aunque la conversación esté en otro. Los ids de spec se mantienen verbatim, sin traducir.
`update` nunca borra un idioma ya configurado; un `init --force` sin `--language` lo preserva.
