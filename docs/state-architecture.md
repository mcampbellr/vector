# Vector — Arquitectura de estado (PROPUESTAS en discusión)

> ⚠️ Estas son **propuestas bajo discusión**, no decisiones cerradas. Capturadas para no
> perder el hilo. Surgen de las 3 dudas abiertas del usuario: (a) ¿commitear el JSON?,
> (b) per-dev / daily notes, (c) `/vector:apply` wrapper de OpenSpec.

## 0. Principio rector: el CLI (Go) es dueño de las escrituras de estado

Claude **nunca** edita el JSON de estado a mano. Solo invoca subcomandos del binario
(`vector status <id> review`, `vector link ...`), que los slash commands envuelven.

Motivos:
- 0 tokens gastados en mutaciones de estado (las hace Go, determinista).
- Imposible corromper el JSON (validación en el binario).
- El "recordatorio de mantener el JSON al día" deja de depender de la memoria del modelo:
  pasa a ser **estructural** (el comando ya lo hace) + un **hook** determinista para los
  casos atados al flujo (ej. auto `need attention`).

Implicación: la regla "recuérdale a Claude actualizar el JSON" se reemplaza por comandos +
hooks. El modelo invoca; no escribe estado.

## 1. ¿Commitear el JSON? → No uno monolítico. 3 capas

Un JSON único commiteado escala mal: conflictos de merge constantes (cada dev lo toca) y
crecimiento sin límite. Separar en capas:

| Capa | Qué contiene | ¿Git? | Formato sugerido |
|------|--------------|-------|------------------|
| **Spec** (fuente de verdad) | contenido del spec (markdown OpenSpec) | ✅ commiteado | `.vector/specs/<id>/spec.md` |
| **Estado compartido** | status, ticket link, startedAt/closedAt, prioridad | ✅ commiteado, **1 archivo por spec** | front-matter del spec *o* `<id>.json` |
| **Personal** | daily notes, qué moviste en el board, reminders | ❌ gitignored (`~/.vector/<repo-id>/`) | JSONL append-only |

Claves:
- **Un archivo por spec**, nunca uno global → conflictos localizados al spec editado
  (mismo patrón que `.changeset/*.md`).
- El "board JSON" global **no se commitea: se deriva** leyendo los archivos por-spec.
- Columna del board = `status` (compartido). Orden dentro de la columna = **computado**
  (prioridad, updatedAt), no un campo manual commiteado (eso genera conflictos).

## 2. Per-dev / "my daily notes"

- **Log de actividad personal**: JSONL append-only local, 1 línea por evento (spec creado,
  cambio de status, nota, reminder). Append-only ⇒ sin conflictos, costo mínimo.
- `/vector:daily`: filtra eventos de hoy + cruza con `git log --author=<dev>` para "qué
  trabajaste". Lo "mío" sale de ahí sin servidor.
- Atribución: lo compartido → autor del commit; lo personal → log local.
- Prompt de notas: **hook** al detectar trabajo sobre un ticket → appendea al log local.

## 3. `/vector:apply` — wrapper delgado sobre OpenSpec

No reimplementar `openspec apply`. Wrapper que:
1. llama `openspec apply <change>`,
2. si ok → `status → in progress` + registra `startedAt`,
3. loguea el evento (compartido + personal).

Se heredan mejoras de OpenSpec; Vector agrega el tracking. Principio: Vector **orquesta y
rastrea**, no reemplaza buenas herramientas.

## Ciclo de vida del spec

`raw → open` · `apply → in progress` · *(hook: surgen preguntas)* `→ need attention` ·
`→ review` · `close → closed` · `archive → fuera del board activo`.

- Estados: `open, in progress, need attention, review, closed` (+ `archived`).
- `need attention` automático cuando surgen preguntas durante el trabajo → **hook**, no
  decisión del modelo. Prioriza ese spec en el board y en `/vector:daily`.

## UI del board (V1)

Meramente **informativo / read-only**: el dev no opera desde ahí. Es donde Claude le muestra
al dev el estado del proyecto. (Ver `docs/kanban-ui-reference.md`.)

## Pendiente de validar con análisis

- Forma exacta a la que `/vector init` reorganiza el repo → definir tras analizar
  cdr / private-wealth / somnio.
- Esquema concreto de los archivos por-spec y del log JSONL.
