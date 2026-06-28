# Spec: Idioma configurable para la prosa de los agentes

## 1. Objetivo

Construir un mecanismo por el cual un repo declara, en `.vector/config.json`, el **idioma en
que los agentes de Vector escriben su prosa**, y cablear ese idioma en el pipeline de
`/vector:standup` para que el digest se genere en el idioma declarado en vez de inferirlo de la
conversación.

Esta feature permite que un **equipo/dev** pueda **fijar el idioma de salida de los resúmenes
generados por agente** (empezando por el daily digest del standup) para obtener un digest
consistente en ese idioma **aunque la conversación con Claude ocurra en otro**.

El problema concreto: hoy el agente `vector-standup-writer` (Haiku) se invoca solo con el JSON
de proyección y su única regla de idioma es *"match the conversation language"*; como nunca
recibe el idioma de la conversación, por defecto produce inglés y no hay forma de fijarlo por
proyecto.

## 2. Alcance

### Incluido en esta fase

- **Campo `language` opcional en `.vector/config.json`** (string libre: tag BCP-47 como `es` /
  `es-MX`, o nombre llano como `Spanish` / `español`). Ausente/vacío = comportamiento actual
  ("match conversation language"). Se añade con `omitempty`; **`SchemaVersion` se mantiene en
  1** (cambio aditivo y retrocompatible: un config previo sin el campo se deserializa con
  `Language == ""`, sin migración ni error — ver `docs/domain-contract.md` no aplica; el patrón
  de campo opcional es el de `applyMode`/`changesPath`).
- **`vector init --language <lang>`**: nuevo flag que escribe `language` en el config al
  inicializar. Documentado en la ayuda (`usage()`) del binario y en `README.md`.
- **`vector update --language <lang>`**: el mismo flag en `update`, como vía **no destructiva**
  para fijar/cambiar el idioma en un repo ya inicializado (preserva el resto del config).
- **Surface del idioma vía la proyección**: `vector standup --json` resuelve `language` desde
  el config y lo expone como un campo **top-level** nuevo en su salida (`standup.Projection`).
  El comando ya consume ese JSON, así que no necesita leer el config por su cuenta y el binario
  sigue siendo el único que toca el config (CLI-owns-writes).
- **`/vector:standup` honra el idioma**: el comando lee `language` del JSON de proyección y lo
  pasa **explícitamente** en el prompt del subagente (`Write the prose in: <language>`); si está
  ausente, no añade la directiva (el agente cae a "match conversation language").
- **Regla de idioma del agente actualizada** en `kit/agents/vector-standup-writer.md` (y su
  copia embebida en `cli/internal/scaffold/assets/agents/…`, re-sembrada por
  `vector update` / `vector init --force`): *"Write the prose in the language provided by the
  command; if none is provided, match the conversation language. Keep spec ids verbatim."*
- **Diseño extensible**: `language` es config genérico de prosa de agentes (no específico de
  standup), de modo que otros agentes generadores de prosa (p. ej. el autor de specs `raw`)
  puedan reusarlo después. **Solo standup se cablea en esta fase.**

### Fuera de scope

- **Traducir spec ids, títulos o cualquier estado persistido**. Los ids van verbatim siempre.
- **Localización del UI/board** (StandupView, timeline, pills): no se traduce nada de la web.
- **i18n de la ayuda/textos del CLI**: la ayuda y los mensajes del binario siguen en inglés.
- **Cablear otros agentes** (raw spec author, etc.) en esta fase: solo standup.
- **Flag `--language` por ejecución** en `/vector:standup` o `vector standup` (override
  per-run): el idioma es atributo del repo, no de la sesión. Se deja como posible extensión.
- **Validación contra una lista de idiomas**: se acepta cualquier string no vacío (pass-through).

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Lenguaje: **Go** (módulo único en `cli/`, solo stdlib, sin deps externas).
- Config: struct `config.Config` serializado a/desde `.vector/config.json`
  (`cli/internal/config/config.go`), escritura atómica (`writeFileAtomic`).
- CLI: parseo con `flag.FlagSet` de la stdlib (un `FlagSet` por subcomando en
  `cli/cmd/vector/main.go`).
- Proyección de standup: `cli/internal/standup` (`type Projection`), serializada a JSON por
  `vector standup --json`.
- Kit distribuible: project command markdown (`kit/commands/vector/standup.md`) + subagente
  markdown (`kit/agents/vector-standup-writer.md`, `model: haiku`, `tools: Read`).
- Embed/scaffold: `cli/internal/scaffold` embebe `kit/{commands,agents,vector}` vía `embed.FS`;
  la copia vendorizada en `cli/internal/scaffold/assets/` se regenera con `go generate`
  (directiva en `cli/internal/scaffold/scaffold.go`).

### Versiones relevantes

- Go: **1.26** (declarado en `cli/go.mod`). El cambio usa solo stdlib ya presente.
- `config.SchemaVersion`: **1** (se mantiene; el cambio es aditivo).
- `standup.StandupSchemaVersion` (digest persistido): 1 (no se toca).

### Patrones existentes a respetar

- **CLI-owns-writes**: el binario es el único escritor de `.vector/config.json` y del estado.
- **Campo opcional con `omitempty`**: igual que `changesPath`, `applyMode`, `kitVersion`,
  `defaultTicketProvider` en `config.Config`.
- **Escritura atómica** vía `config.Write` → `writeFileAtomic` (temp + rename).
- **Naming kebab-case** para flags de cara al usuario (`--language`).
- **Idioma de los artefactos**: la prosa del spec sigue al proyecto (español); slugs, rutas,
  identificadores de código y la ayuda del CLI permanecen en inglés.
- **Token routing**: el cambio no altera el tier del agente (sigue Haiku); solo le da una
  directiva de idioma adicional en el prompt (`product/token-routing.md`).

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `config.Config` con `Load`/`Resolve`/`Write` (`cli/internal/config/config.go`).
- [x] `runInit` con parseo de flags (`cli/cmd/vector/main.go`, línea ~68).
- [x] `runUpdate` que carga y preserva el config existente (`main.go`, línea ~150).
- [x] `runStandup` (`cli/cmd/vector/standup.go:20`) + su `enrichProjection` (`standup.go:90`);
      el dispatch `case "standup"` vive en `main.go:45`.
- [x] `standup.Projection` serializable a JSON (`cli/internal/standup/standup.go`).
- [x] Project command `/vector:standup` (`kit/commands/vector/standup.md`).
- [x] Subagente `vector-standup-writer` (`kit/agents/vector-standup-writer.md`) + copia en
      `cli/internal/scaffold/assets/agents/`.

Si alguna dependencia no existe, el agente debe detenerse y reportar qué falta. No inventar
contratos ni rutas.

---

## 5. Arquitectura

### Patrón a usar

**Config-driven + directiva en el prompt**: el idioma es estado de config (única fuente de
verdad, escrito por el binario). El binario lo **surfacea** en el JSON de proyección que el
comando ya consume; el comando lo traduce a una **directiva en el prompt** del agente (no a un
campo de datos estructurados del contrato del agente). Así el agente permanece desacoplado del
config y la directiva es opcional.

### Capas afectadas

- presentation (web/board): **no** — sin localización de UI.
- application/CLI (`cli/cmd/vector`): **sí** — `--language` en `runInit` y `runUpdate`;
  `runStandup` resuelve `language` del config y lo inyecta en la proyección.
- domain/config (`cli/internal/config`): **sí** — nuevo campo `Language` (string, omitempty) +
  helper `ResolvedLanguage()` (trim).
- domain/standup (`cli/internal/standup`): **sí** — nuevo campo `Language` en `Projection`
  (poblado por el caller, no por el builder de proyección).
- kit (`kit/commands`, `kit/agents`): **sí** — comando lee `language` del JSON y lo pasa;
  agente cambia su regla de idioma.
- data/estado (`.vector/specs`, `activity.jsonl`, digest persistido): **no** — el idioma no se
  persiste en el estado; es metadata de config.

### Flujo esperado

1. Dev ejecuta `vector init --language es` (o `vector update --language es` en un repo ya
   inicializado). El binario persiste `"language": "es"` en `.vector/config.json`.
2. Dev ejecuta `/vector:standup`.
3. El comando corre `vector standup --json`; el binario carga el config, resuelve `language` y
   lo añade como campo top-level del JSON: `{ "since": …, "language": "es", "perSpec": […], "totals": … }`.
4. El comando lee `language` del JSON. Si está presente, antepone al prompt del subagente la
   directiva `Write the prose in: es`. Si está ausente/vacío, no añade directiva.
5. El subagente `vector-standup-writer` (Haiku) genera el digest **en el idioma indicado**
   (o, sin directiva, en el idioma de la conversación), manteniendo spec ids verbatim.
6. El comando persiste el digest vía `vector standup commit` (sin cambios) y reporta.

### Ubicación de archivos nuevos

No se crean paquetes ni carpetas nuevas. Solo se añaden campos/flags a paquetes y archivos
markdown existentes; la copia de assets del scaffold se **regenera** (no se edita a mano).

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `cli/internal/config/config.go` | MODIFICAR | Añadir `Language string \`json:"language,omitempty"\`` a `Config` + helper `ResolvedLanguage()` (trim). `SchemaVersion` se mantiene en 1. | Campo `ApplyMode` + `ResolvedApplyMode()` en el mismo archivo |
| `cli/cmd/vector/main.go` | MODIFICAR | `--language` en `runInit` y `runUpdate` (set `cfg.Language` al persistir); líneas de `usage()` y reporte. | `fs.String("repo-root", …)` y manejo de `cfg.KitVersion` |
| `cli/cmd/vector/standup.go` | MODIFICAR | En `runStandup` (línea 20): tras construir/enriquecer la proyección, cargar el config y asignar `proj.Language = cfg.ResolvedLanguage()` antes de serializar `--json`. | `enrichProjection` en el mismo archivo (línea 90) |
| `cli/internal/standup/standup.go` | MODIFICAR | Añadir `Language string \`json:"language,omitempty"\`` a `Projection` (poblado por el caller). | Campos de `Projection` (`Since`, `Totals`) |
| `kit/commands/vector/standup.md` | MODIFICAR | Paso 2: leer `language` del JSON de proyección y, si está presente, pasar `Write the prose in: <language>` al subagente. | Pasos 1–3 actuales del mismo archivo |
| `kit/agents/vector-standup-writer.md` | MODIFICAR | Reemplazar la hard rule de idioma por la regla command-provided/fallback. | Bloque "Hard rules" del mismo archivo |
| `cli/internal/scaffold/assets/agents/vector-standup-writer.md` | REGENERAR | Copia embebida del agente; se regenera con `go generate` (no editar a mano). | Directiva `//go:generate` en `scaffold.go` |
| `README.md` | MODIFICAR | Documentar `vector init --language` / `vector update --language` y el campo `language` del config. | Sección de stack/uso del README |

### Detalle por archivo

#### cli/internal/config/config.go

Acción: MODIFICAR

Cambios requeridos:
- Añadir al struct `Config` (junto al resto de campos opcionales):
  `Language string \`json:"language,omitempty"\``, con un comentario que explique que es el
  idioma de la prosa generada por agentes (vacío = "match conversation language").
- Añadir método `func (c *Config) ResolvedLanguage() string` que retorne `strings.TrimSpace(c.Language)`.
- **No** cambiar `SchemaVersion` (sigue en 1).

Restricciones:
- No validar el valor contra ninguna lista; cualquier string no vacío post-trim es válido.
- No tocar otros campos ni `Resolve`/`Load`/`Write` salvo lo necesario para el campo.
- Mantener la deserialización de configs previos sin el campo como `Language == ""` (automático).

#### cli/cmd/vector/main.go

Acción: MODIFICAR

Cambios requeridos:
- `runInit`: añadir `language := fs.String("language", "", "prose language for agent output (e.g. es, Spanish; optional)")`.
  Al persistir el config (rama `!cfgExisted || *force`), si `strings.TrimSpace(*language) != ""`
  entonces `cfg.Language = strings.TrimSpace(*language)`. **En `--force` sin `--language`,
  preservar el `Language` existente** (cargar el config previo y arrastrar su `Language`) para
  no borrarlo silenciosamente. Reflejar el idioma en el reporte de salida cuando esté presente.
- `runUpdate`: añadir el mismo flag `--language`. `runUpdate` ya carga el config existente; si
  el flag viene con valor, `cfg.Language = strings.TrimSpace(*language)` antes de `config.Write`.
  Sin el flag, el `Language` existente se preserva (no se toca).
- `usage()`: actualizar las líneas de `vector init` y `vector update` para incluir `[--language lang]`.

Restricciones:
- No cambiar flags existentes ni su comportamiento.
- Solo escribir `Language` cuando el flag trae valor (o al preservar en `--force`).
- El binario sigue siendo el único escritor del config.
- La inyección del idioma en la proyección **no** vive aquí: ocurre en `runStandup`
  (`cli/cmd/vector/standup.go`); en `main.go` solo está el dispatch `case "standup"` (línea 45).

#### cli/cmd/vector/standup.go

Acción: MODIFICAR

Cambios requeridos:
- En `runStandup` (línea 20), tras construir la proyección y llamar a `enrichProjection`
  (línea 45), cargar el config del repo (`config.Load`), resolver `cfg.ResolvedLanguage()` y
  asignarlo a `proj.Language` antes de serializar la salida `--json`. Si el config no existe o
  no declara idioma, `proj.Language` queda vacío (omitempty lo omite).
- **Resolución de Open question #1**: un error al cargar el config **se ignora** para efectos
  del idioma (la proyección no debe depender de él) — `proj.Language` queda vacío y el agente
  cae a su fallback. Es consistente con que `runStandup` ya retorna en errores de store, pero el
  idioma es prescindible. (Si esta resolución cambia, actualizar Open questions.)

Restricciones:
- La asignación se hace **en `runStandup`, después de `enrichProjection`**, no dentro de
  `enrichProjection` (no acoplar el paquete `standup` a `config`). `enrichProjection` no cambia.
- `runStandupCommit` (línea 120) no necesita el idioma (no genera prosa); no se toca su flujo.

#### cli/internal/standup/standup.go

Acción: MODIFICAR

Cambios requeridos:
- Añadir `Language string \`json:"language,omitempty"\`` al struct `Projection`.
- El builder de la proyección **no** resuelve config; deja `Language` vacío. Lo pobla el caller
  (`runStandup`) tras construir la proyección. (Evita que el paquete `standup` importe `config`.)

Restricciones:
- No cambiar los demás campos ni la lógica de proyección.

#### kit/commands/vector/standup.md

Acción: MODIFICAR

Cambios requeridos:
- En el paso 1, notar que el JSON de proyección puede incluir un campo top-level `language`.
- En el paso 2 ("Generate the digest"), antes de invocar al subagente: si el JSON trae
  `language` no vacío, anteponer al prompt la directiva exacta `Write the prose in: <language>`
  (sustituyendo `<language>` por el valor). Si está ausente/vacío, no añadir directiva (el
  agente cae a "match conversation language").

Restricciones:
- No cambiar los pasos 1 (proyección), 3 (commit) ni 4 (report) en su esencia.
- El comando **no** lee `.vector/config.json` por su cuenta; obtiene el idioma del JSON.
- No editar estado a mano.

#### kit/agents/vector-standup-writer.md

Acción: MODIFICAR

Cambios requeridos:
- Reemplazar la hard rule actual *"Match the user's language for the prose (the conversation
  language), but keep spec ids verbatim."* por:
  *"Write the prose in the language provided by the command (it appears in your prompt as
  `Write the prose in: <language>`); if no language is provided, match the conversation
  language. Keep spec ids verbatim regardless of language."*

Restricciones:
- No cambiar el resto de hard rules, el bloque de input ni el output shape (JSON `{global, perSpec}`).

#### cli/internal/scaffold/assets/agents/vector-standup-writer.md

Acción: REGENERAR

- Regenerar con `go generate ./...` (o re-ejecutar la directiva de `scaffold.go`) tras editar
  el archivo fuente en `kit/agents/`. No editar la copia a mano (drift). Verificar que el
  contenido embebido coincide con el fuente.

#### README.md

Acción: MODIFICAR

Cambios requeridos:
- Documentar el flag `--language` de `vector init` y `vector update`, y el campo `language` del
  config (con ejemplos `es` / `Spanish` y la semántica "ausente = idioma de la conversación").
- Nota: el README actual es de etapa-visión y **no** tiene una sección de referencia de CLI;
  añadir una nota mínima donde encaje (o crear una sección breve de uso). La ayuda autoritativa
  vive en `usage()` del binario.

---

## 7. API Contract

No aplica — no se introduce ni cambia ningún endpoint HTTP. El contrato afectado es interno: el
**shape del JSON de `vector standup --json`** gana un campo top-level opcional `language`
(`omitempty`), consumido por el project command. La regla:

- `language`: string opcional. Presente solo cuando el config del repo lo declara. El comando lo
  usa como directiva de idioma para el agente; ningún otro consumidor lo requiere.

No se infieren campos adicionales ni se renombran propiedades existentes de la proyección.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `vector init --language es` escribe `"language": "es"` en `.vector/config.json`.
- [ ] `vector init` sin `--language` no escribe el campo (omitempty); el comportamiento es
      idéntico al actual.
- [ ] `vector update --language fr` sobre un repo inicializado fija `"language": "fr"`
      preservando el resto del config; sin el flag, no toca `language`.
- [ ] `vector init --force` sin `--language` **no borra** un `language` previamente configurado.
- [ ] Un config previo sin `language` (escrito antes de este cambio) **carga sin error** y se
      comporta como hoy (`SchemaVersion` sigue 1; sin migración).
- [ ] `vector standup --json` incluye `"language": "<valor>"` cuando el config lo declara, y lo
      omite cuando no.
- [ ] `/vector:standup` en un repo con `"language": "es"` produce un **digest en español** aunque
      la conversación esté en inglés.
- [ ] `/vector:standup` en un repo **sin** `language` produce el digest en el idioma de la
      conversación (comportamiento sin cambios).
- [ ] La copia del agente en `scaffold/assets/` coincide con la fuente de `kit/agents/`.
- [ ] No hay errores de `gofmt`/`go vet`/linter; tests verdes; el digest sigue siendo JSON válido.

### Tests requeridos

Agregar o actualizar tests para:

- [ ] `config`: round-trip de `Language` (set/omitido); carga de un config legacy sin el campo →
      `Language == ""` sin error; `ResolvedLanguage()` recorta espacios.
- [ ] `runInit`: con `--language es` escribe el campo; sin flag lo omite; `--force` sin flag
      preserva el `language` existente; `--force --language en` lo sobrescribe.
- [ ] `runUpdate`: `--language` fija/cambia el campo preservando el resto; sin flag no lo toca.
- [ ] `runStandup`/proyección: `vector standup --json` incluye `language` cuando el config lo
      declara y lo omite cuando no.

### Comandos de verificación

Ejecutar:

```bash
go -C cli generate ./...   # regenera la copia de assets del scaffold
gofmt -l cli
go -C cli vet ./...
go -C cli test ./...
go -C cli build ./...
```

La fase no está completa si alguno de estos comandos falla o si `gofmt -l` lista archivos.

---

## 9. Criterios de UX

No aplica — no hay UI ni formularios en este cambio. La única "UX" es de CLI: la ayuda
(`usage()`) lista `[--language lang]` para `init`/`update`, y el reporte de `vector init`
muestra el idioma cuando se fijó. El efecto observable de la feature es silencioso: el digest
del standup aparece en el idioma configurado.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas; el agente no debe cuestionarlas ni cambiarlas:

- **El idioma vive en `config.json`, no en un flag por ejecución**: es atributo del repo/equipo.
- **`SchemaVersion` se mantiene en 1**: el campo es aditivo y retrocompatible (omitempty +
  zero-value); no se añade código de migración.
- **El comando obtiene el idioma del JSON de `vector standup --json`** (campo top-level
  resuelto desde config por el binario), **no** leyendo `.vector/config.json` por su cuenta ni
  vía un `vector config get`.
- **`--language` se añade a `init` y a `update`** (init lo fija; update es la vía no destructiva
  para cambiarlo).
- **Pass-through libre del valor**: cualquier string no vacío (BCP-47 o nombre llano) es válido;
  no se valida contra una lista.
- **El idioma se pasa al agente como directiva de prompt** (`Write the prose in: <language>`),
  no como dato estructurado del contrato del agente.
- **Solo se cablea standup** en esta fase; el diseño es genérico para reusar después.
- **Spec ids siempre verbatim**, sin traducir, en cualquier idioma.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, no
implementarla.

---

## 11. Edge cases

### Valor de `--language`

- `--language ""` (vacío) o flag ausente → no se escribe el campo; comportamiento actual.
- `--language "  es  "` (con espacios) → se recorta a `es` antes de escribir.
- Valor con no-ASCII (`español`, `日本語`) → permitido (sin whitelist).

### Config

- `.vector/config.json` ausente al correr `vector update --language` → `update` ya exige config
  existente y aborta con mensaje accionable (`run vector init first`); no se crea idioma huérfano.
- Config legacy (escrito antes del campo) → `Language == ""`, sin error, sin migración.
- Config con JSON inválido → error de carga existente; se propaga (comportamiento actual).

### `vector init --force`

- `--force` sin `--language` sobre un config con `language` → **se preserva** el idioma previo.
- `--force --language en` sobre un config con `language: es` → se sobrescribe a `en`.

### Standup sin idioma en el JSON

- `vector standup --json` sin `language` (config sin campo) → el comando no añade directiva; el
  agente cae a "match conversation language" (comportamiento actual).
- Si el binario no logra resolver el config al proyectar → el error de config **se ignora**
  para el idioma; `Language` queda vacío en la proyección y el agente cae a su fallback (la
  proyección no debe fallar por un idioma prescindible).

### Agente

- Si por bug el agente no recibe la directiva pese a haber idioma configurado → produce el digest
  con su regla fallback (conversación); el standup no se rompe (degradación suave).

---

## 12. Estados de UI requeridos

No aplica — el cambio no introduce ni modifica componentes de UI. El board, la StandupView y la
timeline son read-only y no se ven afectados por este cambio.

---

## 13. Validaciones

### Validaciones de cliente (CLI)

| Campo | Regla | Mensaje |
|---|---|---|
| `--language <val>` | opcional; si viene, se recorta; cualquier string no vacío es válido | (sin validación de contenido; no hay mensaje de rechazo) |
| `config.language` | opcional; string; ausente = comportamiento actual | — |

No hay validación de servidor (no hay backend remoto involucrado). El valor es pass-through.

---

## 14. Seguridad y permisos

- `language` es metadata de config no sensible (sin secretos, tokens ni PII). No se loguea nada
  sensible.
- `.vector/config.json` es versionado/compartido por el equipo (diseño actual); declarar el
  idioma ahí es intencional y seguro.
- No se añaden permisos nuevos: `init`/`update` ya escriben en `.vector/`. La escritura sigue el
  canal serializado y atómico del binario (CLI-owns-writes).
- No se toca el repo del usuario fuera de `.vector/` y los artefactos del kit ya scaffoldeados.

---

## 15. Observabilidad y logging

- `vector init`/`vector update` incluyen el idioma en su reporte de salida cuando se fijó
  (p. ej. una línea `language: es`), usando el mismo mecanismo de impresión existente.
- No se añade logging nuevo en el pipeline del agente. No se registran datos sensibles.

---

## 16. i18n / textos visibles

- La ayuda del CLI (`usage()`) y los mensajes del binario permanecen **en inglés** (la i18n del
  CLI está fuera de scope).
- La directiva al agente (`Write the prose in: <language>`) y la regla del agente se mantienen en
  inglés (el agente las interpreta; la salida que produce sí va en el idioma configurado).
- No hay sistema de traducciones de UI que tocar.

---

## 17. Performance

- Costo despreciable: un campo string extra en config y en la proyección; una llamada
  `config.Load` adicional en `runStandup` (lectura de un archivo pequeño, una vez por standup).
- El prompt del agente crece en ~una línea (directiva de idioma); impacto de tokens despreciable
  frente al JSON de proyección. El tier del agente no cambia (sigue Haiku).
- Sin llamadas repetidas, sin trabajo pesado en el hilo principal.

---

## 18. Restricciones

El agente no debe:

- Cambiar `SchemaVersion` ni añadir lógica de migración.
- Validar el valor de `language` contra una lista de idiomas.
- Hacer que el project command lea `.vector/config.json` directamente (el idioma llega por el
  JSON de proyección).
- Persistir el idioma en el estado del board (`.vector/specs`, `activity.jsonl`, digest).
- Cablear otros agentes además de `vector-standup-writer` en esta fase.
- Añadir un flag `--language` por ejecución a `vector standup` / `/vector:standup`.
- Editar a mano la copia de assets del scaffold (debe regenerarse).
- Traducir spec ids, títulos ni estados persistidos.
- Cambiar el output shape del agente ni el contrato de la proyección más allá del campo añadido.
- Introducir dependencias externas (se mantiene stdlib).

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `Config.Language` (omitempty) + `ResolvedLanguage()` en `cli/internal/config/config.go`
      (`SchemaVersion` intacto en 1).
- [ ] `--language` en `runInit` y `runUpdate`; `runStandup` inyecta `language` en la proyección.
- [ ] `Projection.Language` en `cli/internal/standup/standup.go`.
- [ ] `usage()` y `README.md` documentan el flag y el campo de config.
- [ ] `kit/commands/vector/standup.md` lee `language` del JSON y pasa la directiva al agente.
- [ ] `kit/agents/vector-standup-writer.md` con la nueva regla de idioma + copia de assets
      regenerada y verificada.
- [ ] Tests de config, init, update y proyección añadidos/actualizados.
- [ ] Gate verde: `go generate`, `gofmt -l`, `go vet`, `go test`, `go build`.
- [ ] Documentación del workspace `cli/` revisada si el cambio la afecta (opcional).

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] No apliqué nada fuera del alcance (solo standup; sin validación; sin bump de schema).
- [ ] Confirmé que un config legacy sin `language` carga sin error y se comporta como hoy.
- [ ] `--language` funciona en `init` y `update`; `--force` preserva el idioma si no se pasa el flag.
- [ ] `vector standup --json` incluye `language` solo cuando el config lo declara.
- [ ] El comando obtiene el idioma del JSON (no lee config.json a mano) y pasa la directiva exacta.
- [ ] La regla del agente quedó como "command-provided, fallback conversación, ids verbatim".
- [ ] Regeneré la copia de assets del scaffold y verifiqué que coincide con la fuente.
- [ ] Seguí ejemplos reales del repo (`ApplyMode`/`ResolvedApplyMode`, manejo de `KitVersion`).
- [ ] No añadí dependencias externas ni cambié decisiones tomadas.
- [ ] Ejecuté `go generate`, `gofmt`, `go vet`, `go test`, `go build` — todos verdes.
- [ ] No dejé logs temporales ni TODOs sin justificar.

---

## Open questions

1. **Limpiar un idioma ya configurado**: con `flag.String` no se distingue `--language ""`
   explícito de la ausencia del flag, así que no hay vía por flag para "borrar" el idioma
   (queda editar el config o re-init `--force`). ¿Se necesita un mecanismo explícito de clear?
2. **README sin sección de CLI**: el README es etapa-visión; definir si se crea una sección de
   referencia de CLI mínima o se añade la nota junto al stack. La ayuda autoritativa es `usage()`.

> Resueltas durante validación: (a) un error de `config.Load` en `runStandup` se **ignora** para
> el idioma (proyección con `Language` vacío, el agente cae al fallback); (b) versión de Go =
> **1.26** (`cli/go.mod`).
