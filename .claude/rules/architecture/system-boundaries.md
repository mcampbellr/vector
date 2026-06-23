# Architecture — Límites del sistema

> Aplica a: cambios que cruzan workspaces, definen ownership o introducen dependencias entre
> `cli/`, `web/` y `kit/`.

Vector se compone de tres workspaces con responsabilidades disjuntas. Un cambio que necesita
tocar más de uno debe declarar explícitamente la dirección de la dependencia.

## Workspaces y ownership

| Workspace | Stack | Posee |
|-----------|-------|-------|
| `cli/` | Go (módulo único) | El binario: comandos (`/vector init`, `/vector:raw …`), la **API HTTP del board** y el servidor que sirve el panel web embebido. Lee/escribe el JSON de estado. |
| `web/` | React/Next (TS) | El frontend del board kanban. **Consume** la API de `cli/`. No accede al filesystem del usuario ni al JSON directamente. |
| `kit/` | Markdown + assets | El ecosistema distribuible: skills, rules, memorias y `devup` que Vector instala en el repo del usuario. No contiene lógica de runtime de Go/TS. |

## Reglas de dependencia

- **`web/` depende de `cli/`** solo a través de la API HTTP (contrato versionado). Nunca al
  revés en código; `cli/` solo **embebe** los assets buildados de `web/` (ver
  `architecture/distribution-packaging.md`).
- **`kit/` es independiente de cli/web en runtime**: son artefactos que se distribuyen e
  instalan en repos ajenos. `cli/` puede **leer/copiar** el contenido de `kit/` durante
  `/vector init`, pero `kit/` no importa código de `cli/`.
- **El JSON de estado es el único punto de integración de datos** entre el CLI y el board
  (ver `architecture/state-model.md`). Ningún workspace mantiene una copia paralela del estado.

## Fronteras con el repo del usuario

- El repo del usuario es **externo**. Vector solo aporta estructura; toda escritura sobre él
  pasa por `security/destructive-ops-consent.md`.
- Vector no asume techstack del usuario: la detección ocurre en `/vector init` y se registra
  en el estado, nunca se hardcodea.

> Resuelto (ver `docs/domain-contract.md`): `web/` consume la API HTTP de `cli/` con SSE para
> frescura; columnas del board = estado (single-axis). Pendiente solo el detalle fino de los
> endpoints y su versionado, al especificar el panel web.
