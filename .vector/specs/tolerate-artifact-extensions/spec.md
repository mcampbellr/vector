# Spec: Tolerar extensión .md y casing en el parseo de --artifacts (propose/fix)

## 1. Objetivo

Corregir los dos parsers de `--artifacts` del CLI (`parseArtifacts` en `cli/cmd/vector/main.go`
y `parseFixArtifacts` en `cli/cmd/vector/spec_transitions.go`) para que acepten de forma
**tolerante** cualquier combinación de mayúsculas/minúsculas y la extensión `.md` opcional, y
actualizar los project commands de `kit/` para que la documentación del flag refleje el
comportamiento real.

Hoy un usuario que escribe `--artifacts proposal.md,Design.MD` obtiene un error aunque la
intención es inequívoca. Tras este cambio, `proposal`, `proposal.md`, `Proposal.md`,
`PROPOSAL` y `  Design.MD  ` son todas entradas válidas que se normalizan internamente al nombre
canónico en minúsculas.

## 2. Alcance

### Incluido en esta fase

- **Normalización tolerante en `parseArtifacts`** (`cli/cmd/vector/main.go:722`): por cada
  segmento de la lista CSV aplicar trim → strip de `.md` insensible a mayúsculas → `ToLower` →
  switch contra los nombres canónicos `proposal | design | tasks`.
- **Normalización equivalente en `parseFixArtifacts`** (`cli/cmd/vector/spec_transitions.go:401`):
  misma lógica de normalización. Además, **`parseFixArtifacts` debe devolver los nombres
  canónicos** (lowercase, sin `.md`), no los valores crudos del input, para que el estado
  persistido sea siempre canónico.
- **Tests table-driven** para ambos parsers:
  - `cli/cmd/vector/main_test.go` (NUEVO) — `TestParseArtifacts`, package `main`.
  - `cli/cmd/vector/spec_transitions_test.go` (NUEVO) — `TestParseFixArtifacts`, package `main`.
- **Actualización de los project commands**:
  - `kit/commands/vector/propose.md` paso 6: aclarar que `--artifacts` acepta los nombres
    canónicos `proposal/design/tasks` y que la extensión `.md` y cualquier casing se toleran.
  - `kit/commands/vector/fix.md` línea ~125: misma aclaración.
- **Re-vending automático** de las copias embebidas:
  - `cli/internal/scaffold/assets/commands/vector/propose.md` — REGENERAR.
  - `cli/internal/scaffold/assets/commands/vector/fix.md` — REGENERAR.
  - La regeneración ocurre mediante `go generate ./internal/scaffold` desde `cli/`; el test
    `TestAssetsMatchKit` en `cli/internal/scaffold/scaffold_test.go` verifica que no haya drift.

### Fuera de scope

- Formalizar la convención de changes liviana (proposal/design/tasks sin deltas) en `docs/`
  — deferido (ver Open questions §1).
- Suavizar o re-redactar el framing "delegate to OpenSpec" en `propose.md` — deferido con §1.
- Eliminar, añadir o modificar cualquier referencia a `openspec validate` en `propose.md` (el
  archivo **no** menciona actualmente `openspec validate`; no se introduce ni se elimina).
- Crear o editar cualquier archivo en `docs/`.
- Tocar la máquina de estados, la transición `draft → open` o cualquier campo del state.json
  no relacionado con el parseo de artefactos.
- Migrar changes existentes al modelo delta de OpenSpec.
- Cambiar el mensaje de error para mencionar explícitamente que `.md` y el casing se toleran
  (se mantiene el mensaje actual `invalid --artifacts %q: allowed proposal,design,tasks`).

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Lenguaje: **Go** (módulo único en `cli/`, stdlib exclusivamente, sin dependencias externas).
- CLI: parseo con `flag.FlagSet` de la stdlib; un `FlagSet` por subcomando en
  `cli/cmd/vector/main.go`.
- Testing: paquete `testing` estándar de Go; tests table-driven, `package main`.
- Kit: project commands en markdown (`kit/commands/vector/*.md`).
- Embed/scaffold: `cli/internal/scaffold` embebe assets de `kit/` vía `embed.FS`; la copia
  vendorizada en `cli/internal/scaffold/assets/` se regenera con `go generate`.

### Versiones relevantes

- Go: **1.26** (declarado en `cli/go.mod`).
- Dependencia de estado: `github.com/mariocampbell/vector/internal/state` — ya presente en
  `cli/go.mod`; `state.ArtifactSet` es el tipo de retorno de `parseArtifacts`.

### Patrones existentes a respetar

- **`package main`** en todos los archivos bajo `cli/cmd/vector/`, incluyendo los archivos de
  test (confirmado en `related_test.go`, `ticket_test.go`, `spec_fix_test.go`).
- **Tests table-driven** con subtests `t.Run`: patrón observado en `related_test.go` y
  `spec_fix_test.go`.
- **Convención de nombres de archivos de test**: `<fuente>_test.go` está al lado del archivo
  fuente que testea (p. ej. `related.go → related_test.go`, `ticket.go → ticket_test.go`,
  `spec_transitions.go → spec_transitions_test.go`). `main.go` → `main_test.go`.
- **`splitCSV`** en `cli/cmd/vector/standup.go:274`: trima cada parte, descarta vacíos y
  retorna `nil` para input vacío. Ya es usado por `parseFixArtifacts`; **no reimplementar**
  trimming en la nueva lógica.
- **Normalización case-insensitive sin deps externas**: usar `strings.ToLower` +
  `strings.TrimSuffix` (o equivalente con `strings.EqualFold` sobre los últimos caracteres)
  de la stdlib.
- **Errores accionables** para el usuario del CLI: `fmt.Errorf("invalid --artifacts %q: …", part)`.
- **Flujo de re-vending (LOCKED)**: editar `kit/` → `go generate ./internal/scaffold` desde
  `cli/` → reinstalar binario → `vector update`. Las copias bajo
  `cli/internal/scaffold/assets/commands/vector/` **nunca se editan a mano**.
- **Naming kebab-case** para flags de cara al usuario; IDs y slugs en inglés.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `parseArtifacts` en `cli/cmd/vector/main.go:722` — función actualmente case-sensitive que
      esta spec modifica. Retorna `state.ArtifactSet`.
- [x] `parseFixArtifacts` en `cli/cmd/vector/spec_transitions.go:401` — función que actualmente
      delega trimming a `splitCSV` pero es case-sensitive. Retorna `([]string, error)`.
- [x] `splitCSV` en `cli/cmd/vector/standup.go:274` — helper de trimming compartido; se
      reutiliza sin cambios.
- [x] `state.ArtifactSet` (campos `Proposal`, `Design`, `Tasks bool`) en
      `cli/internal/state` — tipo de retorno de `parseArtifacts`.
- [x] `kit/commands/vector/propose.md` — project command que llama a `vector spec propose`
      con `--artifacts <created,list>`.
- [x] `kit/commands/vector/fix.md` — project command que llama a `vector spec fix`
      con `--artifacts <comma list>`.
- [x] `cli/internal/scaffold/assets/commands/vector/propose.md` y `fix.md` — copias
      embebidas de los commands anteriores; se regeneran con `go generate`.
- [x] `TestAssetsMatchKit` en `cli/internal/scaffold/scaffold_test.go` — guard que detecta
      drift entre `kit/` y `assets/` antes del merge.
- [x] Go 1.26 (`cli/go.mod`).

Si alguna dependencia no existe, el agente debe detenerse y reportar qué falta. No inventar
contratos ni rutas.

---

## 5. Arquitectura

### Patrón a usar

**Normalización temprana en el parser**: cada función parseadora normaliza su input antes de
comparar. La normalización es determinista, reversible (solo cambia representación, no
semántica) y ocurre antes de cualquier comparación de nombre canónico. El estado persistido
siempre contiene nombres canónicos.

### Capas afectadas

- presentation (web/board): **no** — sin cambios de UI.
- application/CLI (`cli/cmd/vector`): **sí** — `parseArtifacts` y `parseFixArtifacts` se
  actualizan; los archivos de test nuevos viven aquí.
- domain/state (`cli/internal/state`): **no** — `state.ArtifactSet` y los tipos existentes
  no cambian.
- kit (`kit/commands/vector`): **sí** — `propose.md` y `fix.md` se actualizan para documentar
  la tolerancia; sus copias en `cli/internal/scaffold/assets/` se regeneran.
- data/estado (`.vector/specs`, `activity.jsonl`): **no** — el cambio solo normaliza el input;
  el estado ya recibe nombres canónicos (el nuevo comportamiento garantiza que también lleguen
  canonicalizados cuando el input tenga `.md` o mayúsculas).

### Flujo esperado

**Para `parseArtifacts` (propose path):**

1. El project command `/vector:propose` invoca `vector spec propose <id> --change <id> --artifacts proposal.md,Design --json`.
2. El binario recibe la lista `"proposal.md,Design"` y llama a `parseArtifacts`.
3. Por cada segmento: `TrimSpace` → strip de sufijo `.md` (case-insensitive) → `ToLower`.
4. El resultado normalizado `"proposal"` y `"design"` se comparan en el `switch`.
5. Se retorna `state.ArtifactSet{Proposal: true, Design: true}` sin error.
6. El estado se persiste con los valores canónicos de `ArtifactSet`.

**Para `parseFixArtifacts` (fix path):**

1. El project command `/vector:fix` invoca `vector spec fix <id> --artifacts Proposal.md,tasks --json`.
2. El binario llama a `parseFixArtifacts("Proposal.md,tasks")`.
3. `splitCSV` hace el trimming y descarta vacíos → `["Proposal.md", "tasks"]`.
4. Por cada valor: strip de sufijo `.md` (case-insensitive) → `ToLower` → resultado canónico.
5. El `switch` valida contra `"proposal" | "design" | "tasks"`.
6. Se retorna `(["proposal", "tasks"], nil)` — nombres **canónicos**, no el input crudo.

### Ubicación de archivos nuevos

```txt
cli/cmd/vector/
  main_test.go              (NUEVO — TestParseArtifacts)
  spec_transitions_test.go  (NUEVO — TestParseFixArtifacts)
```

No se crean paquetes ni carpetas nuevas. Los archivos de test siguen la convención observada en
el workspace: mismo directorio, mismo package (`package main`).

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `cli/cmd/vector/main.go` | MODIFICAR | Actualizar `parseArtifacts` (línea 722) para normalizar cada segmento: trim → strip `.md` case-insensitive → `ToLower` → switch canónico. | `parseArtifacts` en el mismo archivo (implementación actual) |
| `cli/cmd/vector/spec_transitions.go` | MODIFICAR | Actualizar `parseFixArtifacts` (línea 401) para normalizar cada valor: strip `.md` case-insensitive → `ToLower` → switch canónico. Devolver los valores **canonicalizados**, no los crudos. | `parseArtifacts` actualizada en `main.go` |
| `cli/cmd/vector/main_test.go` | NUEVO | `TestParseArtifacts` table-driven: casos de `.md`, casing, trim, mixto, input vacío, inválidos. Package `main`. | `cli/cmd/vector/related_test.go` |
| `cli/cmd/vector/spec_transitions_test.go` | NUEVO | `TestParseFixArtifacts` table-driven: mismos casos; verificar que el slice retornado contiene nombres canónicos (lowercase, sin `.md`). Package `main`. | `cli/cmd/vector/spec_fix_test.go` |
| `kit/commands/vector/propose.md` | MODIFICAR | En el paso 6, aclarar que `--artifacts` toma los nombres canónicos `proposal/design/tasks` y que la extensión `.md` y cualquier casing se toleran (ambas formas funcionan). | `kit/commands/vector/fix.md` — sección §6 del mismo archivo |
| `kit/commands/vector/fix.md` | MODIFICAR | En la sección §6 (línea ~125), aclarar que `--artifacts` acepta los nombres canónicos y que `.md` y casing se toleran. | `kit/commands/vector/propose.md` actualizado |
| `cli/internal/scaffold/assets/commands/vector/propose.md` | REGENERAR | Copia embebida de `kit/commands/vector/propose.md`. Regenerar con `go generate ./internal/scaffold` desde `cli/`. No editar a mano. | Directiva `//go:generate` en `cli/internal/scaffold/scaffold.go` |
| `cli/internal/scaffold/assets/commands/vector/fix.md` | REGENERAR | Copia embebida de `kit/commands/vector/fix.md`. Regenerar con `go generate ./internal/scaffold` desde `cli/`. No editar a mano. | Directiva `//go:generate` en `cli/internal/scaffold/scaffold.go` |

### Detalle por archivo

#### cli/cmd/vector/main.go

Acción: MODIFICAR

Cambios requeridos:

- En `parseArtifacts` (línea 722), reemplazar el cuerpo del loop:
  ```go
  // ANTES:
  switch strings.TrimSpace(part) {
  case "proposal": ...
  case "design": ...
  case "tasks": ...
  case "": // tolerate empty segments
  default: return a, fmt.Errorf(...)
  }

  // DESPUÉS:
  seg := strings.TrimSpace(part)
  if seg == "" {
      continue // tolerate empty segments (comportamiento actual preservado)
  }
  // strip sufijo .md de forma case-insensitive
  if strings.EqualFold(seg[max(0, len(seg)-3):], ".md") {
      seg = seg[:len(seg)-3]
  }
  seg = strings.ToLower(seg)
  switch seg {
  case "proposal": a.Proposal = true
  case "design":   a.Design = true
  case "tasks":    a.Tasks = true
  default:
      return a, fmt.Errorf("invalid --artifacts %q: allowed proposal,design,tasks", part) // part crudo
  }
  ```
  Nota: el mensaje de error usa `part` (el valor original sin normalizar) para dar contexto
  al usuario. La alternativa `strings.ToLower(strings.TrimSuffix(strings.ToLower(seg), ".md"))`
  es igualmente correcta; usar la que sea más legible para el proyecto.
- `parseArtifacts("")` sigue retornando `ArtifactSet{}` sin error (segmento vacío tolerado).

Restricciones:

- No cambiar la firma de la función (`func parseArtifacts(list string) (state.ArtifactSet, error)`).
- No cambiar el comportamiento de `parseArtifacts("")` (retorna vacío sin error).
- No importar dependencias fuera de la stdlib (el archivo ya usa `strings` y `fmt`).
- No refactorizar partes no relacionadas de `main.go`.

#### cli/cmd/vector/spec_transitions.go

Acción: MODIFICAR

Cambios requeridos:

- En `parseFixArtifacts` (línea 401), tras `splitCSV` y el loop de validación, aplicar la
  misma normalización de strip `.md` case-insensitive + `ToLower` a cada valor antes de
  validar en el switch, y acumular los valores normalizados en un slice separado que se retorna:
  ```go
  func parseFixArtifacts(list string) ([]string, error) {
      vals := splitCSV(list)
      normalized := make([]string, 0, len(vals))
      for _, v := range vals {
          seg := v // splitCSV ya hizo TrimSpace
          if len(seg) >= 3 && strings.EqualFold(seg[len(seg)-3:], ".md") {
              seg = seg[:len(seg)-3]
          }
          seg = strings.ToLower(seg)
          switch seg {
          case "proposal", "design", "tasks":
              normalized = append(normalized, seg)
          default:
              return nil, fmt.Errorf("invalid --artifacts %q: allowed proposal,design,tasks", v) // v crudo
          }
      }
      return normalized, nil
  }
  ```
- El retorno ahora son los **nombres canónicos** (`["proposal", "tasks"]`), no el input crudo
  (`["Proposal.md", "tasks"]`). Esto garantiza que lo que se persiste en el estado es siempre
  canónico.

Restricciones:

- No cambiar la firma (`func parseFixArtifacts(list string) ([]string, error)`).
- No reimplementar el trimming: `splitCSV` ya lo hace; reusar.
- No cambiar `splitCSV` ni `openStore` ni ninguna otra función del archivo.
- El mensaje de error sigue usando el valor crudo (`v`) para dar contexto al usuario.

#### cli/cmd/vector/main_test.go

Acción: NUEVO

Debe implementar:

- `TestParseArtifacts`: función table-driven con subtests `t.Run`, `package main`.
- Casos mínimos requeridos (tabla):

  | Nombre del caso | Input | Resultado esperado | Error esperado |
  |---|---|---|---|
  | `empty string` | `""` | `ArtifactSet{}` | nil |
  | `bare proposal` | `"proposal"` | `ArtifactSet{Proposal:true}` | nil |
  | `proposal.md` | `"proposal.md"` | `ArtifactSet{Proposal:true}` | nil |
  | `Proposal.md` | `"Proposal.md"` | `ArtifactSet{Proposal:true}` | nil |
  | `PROPOSAL` | `"PROPOSAL"` | `ArtifactSet{Proposal:true}` | nil |
  | `Design.MD` | `"Design.MD"` | `ArtifactSet{Design:true}` | nil |
  | `  tasks  ` | `"  tasks  "` | `ArtifactSet{Tasks:true}` | nil |
  | `mixed with ext` | `"proposal.md,design,tasks"` | `ArtifactSet{Proposal:true,Design:true,Tasks:true}` | nil |
  | `all bare` | `"proposal,design,tasks"` | `ArtifactSet{Proposal:true,Design:true,Tasks:true}` | nil |
  | `.md alone` | `".md"` | — | error |
  | `proposal.md.md` | `"proposal.md.md"` | — | error (strip 1 nivel → `proposal.md` → inválido) |
  | `readme` | `"readme"` | — | error |
  | `empty segment tolerated` | `"proposal,,tasks"` | `ArtifactSet{Proposal:true,Tasks:true}` | nil |

Debe seguir como referencia:

- `cli/cmd/vector/related_test.go` — tabla, subtests, `package main`, imports de `state`.

No debe incluir:

- Lógica de integración que requiera disco o store real.
- Imports fuera de `testing` y `github.com/mariocampbell/vector/internal/state`.

#### cli/cmd/vector/spec_transitions_test.go

Acción: NUEVO

Debe implementar:

- `TestParseFixArtifacts`: función table-driven con subtests `t.Run`, `package main`.
- Casos mínimos requeridos (tabla):

  | Nombre del caso | Input | Slice retornado | Error esperado |
  |---|---|---|---|
  | `empty string` | `""` | `nil` | nil |
  | `bare names` | `"proposal,design,tasks"` | `["proposal","design","tasks"]` | nil |
  | `with .md ext` | `"proposal.md,tasks.md"` | `["proposal","tasks"]` | nil |
  | `mixed casing` | `"Proposal.md,DESIGN,Tasks"` | `["proposal","design","tasks"]` | nil |
  | `  Design.MD  ` (con espacios, splitCSV ya trima) | `"  Design.MD  "` | `["design"]` | nil |
  | `invalid name` | `"readme"` | nil | error |
  | `.md alone` | `".md"` | nil | error |
  | `proposal.md.md` | `"proposal.md.md"` | nil | error |
  | `returns canonical not raw` | `"Proposal.md"` | `["proposal"]` | nil (verificar que NO sea `["Proposal.md"]`) |

- Verificar explícitamente que el slice retornado contiene los nombres canónicos (no los valores
  crudos) en el caso `returns canonical not raw`.

Debe seguir como referencia:

- `cli/cmd/vector/spec_fix_test.go` — tabla, subtests, validación de slice retornado.

No debe incluir:

- Store real ni disco; `parseFixArtifacts` es pura (no accede al filesystem).

#### kit/commands/vector/propose.md

Acción: MODIFICAR

Cambios requeridos:

- En el paso 6 (`## Steps`, paso 6), donde aparece:
  ```bash
  vector spec propose <id> --change <id> --artifacts <created,list> --json
  ```
  Añadir una nota aclaratoria (en línea o como sub-punto) que diga:
  > `--artifacts` acepta los nombres canónicos `proposal`, `design`, `tasks`. La extensión
  > `.md` y cualquier casing se toleran: `proposal.md`, `Proposal.MD` y `PROPOSAL` son
  > equivalentes a `proposal`.

Restricciones:

- No cambiar la lógica de los pasos ni el flujo del command.
- No añadir ni eliminar referencias a `openspec validate` (el archivo actualmente no las tiene).
- No re-redactar el framing "delegate to OpenSpec" — fuera de scope.
- Mantener la nota breve (1–3 líneas); no expandir la sección.

#### kit/commands/vector/fix.md

Acción: MODIFICAR

Cambios requeridos:

- En la sección §6 (`## 6. Record the correction`, línea ~125), donde aparece:
  `--artifacts <comma list of proposal,design,tasks amended>`
  Añadir una nota análoga a la de `propose.md`:
  > `--artifacts` acepta `proposal`, `design`, `tasks`. La extensión `.md` y cualquier
  > casing se toleran.

Restricciones:

- No cambiar la lógica del command ni otros pasos.
- Mantener la nota breve; no expandir la sección.

#### cli/internal/scaffold/assets/commands/vector/propose.md y fix.md

Acción: REGENERAR

- Regenerar con `go generate ./internal/scaffold` desde `cli/` tras editar los fuentes en
  `kit/commands/vector/`.
- No editar las copias bajo `assets/` a mano (drift).
- `TestAssetsMatchKit` (`cli/internal/scaffold/scaffold_test.go`) verificará que coinciden;
  debe pasar en verde antes de mergear.

---

## 7. API Contract

No aplica — este cambio no introduce ni modifica ningún endpoint HTTP. El único contrato
afectado es interno al CLI: el comportamiento del flag `--artifacts` en los subcomandos
`vector spec propose` y `vector spec fix`. Dicho contrato es:

- `--artifacts <lista CSV>` acepta cualquier combinación de `proposal`, `design`, `tasks` con
  `.md` opcional y cualquier casing. El binario normaliza y persiste los nombres canónicos.
- El mensaje de error sigue siendo `invalid --artifacts %q: allowed proposal,design,tasks`
  (con el valor original sin normalizar para dar contexto).
- `parseArtifacts("")` → `ArtifactSet{}` sin error (comportamiento existente preservado).
- `parseFixArtifacts("")` → `(nil, nil)` (preservado vía `splitCSV`).

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `parseArtifacts("proposal.md")` retorna `ArtifactSet{Proposal:true}` sin error.
- [ ] `parseArtifacts("Proposal.md,DESIGN,tasks")` retorna `ArtifactSet{Proposal:true, Design:true, Tasks:true}` sin error.
- [ ] `parseArtifacts("  Design.MD  ")` (con espacios) retorna `ArtifactSet{Design:true}` sin error.
- [ ] `parseArtifacts("")` retorna `ArtifactSet{}` sin error (comportamiento actual preservado).
- [ ] `parseArtifacts(".md")` retorna error (`.md` solo no es un nombre válido tras la normalización).
- [ ] `parseArtifacts("proposal.md.md")` retorna error (strip de un solo nivel → `proposal.md` → inválido).
- [ ] `parseArtifacts("readme")` retorna error con el mensaje original sin normalizar.
- [ ] `parseFixArtifacts("Proposal.md,tasks")` retorna `(["proposal","tasks"], nil)` — canónico, no crudo.
- [ ] `parseFixArtifacts("DESIGN")` retorna `(["design"], nil)`.
- [ ] `parseFixArtifacts("")` retorna `(nil, nil)`.
- [ ] `parseFixArtifacts(".md")` retorna error.
- [ ] Los tests nuevos en `main_test.go` y `spec_transitions_test.go` cubren todos los casos de la tabla del §6.
- [ ] `kit/commands/vector/propose.md` y `fix.md` incluyen la nota de tolerancia.
- [ ] Las copias en `cli/internal/scaffold/assets/commands/vector/` coinciden con los fuentes de `kit/`; `TestAssetsMatchKit` pasa.
- [ ] No hay errores de `gofmt`, `go vet` ni del linter; todos los tests pasan; el build es exitoso.

### Tests requeridos

Agregar tests para:

- [ ] `TestParseArtifacts` (NUEVO en `main_test.go`): todas las variantes de la tabla del §6 — `.md`, casing, trim, mixto, vacío, segmento vacío tolerado, inválidos.
- [ ] `TestParseFixArtifacts` (NUEVO en `spec_transitions_test.go`): todas las variantes de la tabla — `.md`, casing, canonicalización verificada, inválidos.

Los tests existentes (`spec_fix_test.go`, `related_test.go`, `ticket_test.go`) no deben
romperse. El test `valid` de `TestRunSpecFixValidation` usa `"design,tasks"` (sin `.md`); debe
seguir pasando.

### Comandos de verificación

Ejecutar desde la raíz del repo:

```bash
go -C cli generate ./internal/scaffold   # regenera assets/; equivalente a go generate ./...
gofmt -l cli                             # lista archivos mal formateados; debe estar vacío
go -C cli vet ./...
go -C cli test ./...
go -C cli build ./...
```

La fase no está completa si alguno de estos comandos falla o si `gofmt -l` lista archivos.

---

## 9. Criterios de UX

No aplica — no hay UI ni formularios en este cambio.

La única "UX" es de CLI: un usuario que antes obtenía un error al pasar `proposal.md` o
`Proposal` ahora obtiene el comportamiento correcto sin error. El mensaje de error para valores
genuinamente inválidos (p. ej. `readme`, `.md`) sigue siendo el mismo mensaje existente:
`invalid --artifacts %q: allowed proposal,design,tasks`.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas; el agente no debe cuestionarlas ni cambiarlas:

- **Normalización case-insensitive**: la comparación de nombres es insensible a mayúsculas.
  `PROPOSAL`, `Proposal`, `proposal` son todos equivalentes.
- **Strip de `.md` case-insensitivo**: los sufijos `.md`, `.MD`, `.Md` se eliminan antes de
  comparar. Solo se elimina un nivel (un único sufijo `.md`).
- **Secuencia de normalización LOCKED**: por cada segmento: (1) `TrimSpace`, (2) strip de
  `.md` case-insensitive si el segmento termina en `.md` (cualquier casing), (3) `ToLower`,
  (4) switch contra `"proposal" | "design" | "tasks"`.
- **Mensaje de error con valor original**: el error reporta el segmento sin normalizar (el
  valor que escribió el usuario) para claridad: `invalid --artifacts %q: allowed proposal,design,tasks`.
- **`parseFixArtifacts` devuelve nombres canónicos**: el retorno es siempre lowercase y sin
  `.md`, para que lo que se persiste en el estado sea canónico y no dependa del formato del
  input.
- **`splitCSV` no se modifica**: el trimming de espacios ya lo hace `splitCSV`; `parseFixArtifacts`
  no lo reimplementa.
- **`parseArtifacts("")` preservado**: retorna `ArtifactSet{}` sin error (caso vacío tolerado,
  comportamiento actual).
- **Sin cambios al mensaje de error** para mencionar `.md`/casing: el mensaje actual se mantiene
  tal cual (ver Open questions §3 — deferido).
- **Scope mínimo**: solo se tocan los dos parsers, sus tests y la documentación del flag en
  los commands. Sin cambios en `docs/`, sin formalización de la convención, sin cambios a la
  máquina de estados.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, no
implementarla.

---

## 11. Edge cases

### Normalización

- **`.md` solo** — input `".md"`: tras strip del sufijo `.md`, el segmento queda `""`. Un
  segmento vacío post-normalización **no** es un nombre válido; debe retornar error.
  Nota de implementación: el segmento original no era vacío (era `".md"`), por lo que el
  `case "":` del loop solo cubre el segmento vacío antes de la normalización. Tras la
  normalización, si el resultado es `""`, debe caer al `default:` y retornar error.
- **`proposal.md.md`** — strip de un solo nivel: `"proposal.md.md"` → strip `.md` →
  `"proposal.md"` → `ToLower` → `"proposal.md"` → no coincide con ningún nombre canónico →
  error. Un solo nivel de strip es correcto.
- **`  Design.MD  `** (con espacios externos) — el `TrimSpace` inicial produce `"Design.MD"`;
  strip `.md` → `"Design"` → `ToLower` → `"design"` → válido.
- **Lista mixta con y sin ext** — `"proposal.md,design,tasks"` → los tres se aceptan.
- **Segmento vacío en medio de lista** — `"proposal,,tasks"` en `parseArtifacts`: el `case ""`
  del loop (antes de la normalización) lo tolera. En `parseFixArtifacts`, `splitCSV` ya
  descarta vacíos antes de que lleguen al loop.

### Input vacío

- `parseArtifacts("")`: `strings.Split("", ",")` produce `[""]`; el segmento único es `""`,
  entra al `case ""` y se tolera. Retorna `ArtifactSet{}` sin error.
- `parseFixArtifacts("")`: `splitCSV("")` retorna `nil`; el loop no itera; retorna `(nil, nil)`.

### Comportamiento previo preservado

- Valores canónicos en minúsculas sin extensión (`proposal`, `design`, `tasks`) siguen
  funcionando exactamente como antes.
- `parseFixArtifacts("design,tasks")` → `(["design","tasks"], nil)` — el test existente
  `TestRunSpecFixValidation` (caso `valid`) no debe romperse.

---

## 12. Estados de UI requeridos

No aplica — el cambio no introduce ni modifica componentes de UI. El board y las vistas del
panel web no se ven afectados.

---

## 13. Validaciones

### Validaciones del parser (CLI)

| Campo | Regla | Comportamiento |
|---|---|---|
| `--artifacts <valor>` | Cada segmento, tras normalización (trim + strip `.md` case-insensitive + `ToLower`), debe ser exactamente `"proposal"`, `"design"` o `"tasks"`. | Error `invalid --artifacts %q: allowed proposal,design,tasks` con el valor original sin normalizar. |
| Segmento vacío en `parseArtifacts` | Se tolera (tolerate empty segments, comportamiento actual). | Sin error; se ignora. |
| Input vacío `""` en `parseArtifacts` | Retorna `ArtifactSet{}` sin error. | Sin error. |
| Input vacío `""` en `parseFixArtifacts` | `splitCSV` retorna `nil`; retorna `(nil, nil)`. | Sin error. |
| `.md` solo | Tras strip: segmento vacío post-normalización → no coincide con ningún nombre → error. | Error. |
| Doble sufijo `.md.md` | Solo se strip un nivel → el resultado no es un nombre canónico → error. | Error. |

No hay validación de servidor (no hay backend remoto involucrado en este parseo).

---

## 14. Seguridad y permisos

- El cambio no introduce rutas de lectura ni escritura nuevas en el filesystem del usuario.
- Los valores de `--artifacts` son nombres de artefactos de dominio, no datos sensibles
  (no hay secretos, tokens ni PII involucrados).
- La normalización no expande el espacio de nombres aceptado de forma insegura: solo los
  tres nombres canónicos son válidos; cualquier otra cosa sigue siendo rechazada con error.
- No se añaden permisos nuevos; los subcomandos `spec propose` y `spec fix` ya tienen acceso
  al estado que modifican.

---

## 15. Observabilidad y logging

- No se añade logging nuevo. El mecanismo de error existente (`fmt.Errorf`) sigue siendo el
  único canal de reporte para entradas inválidas.
- El mensaje de error en caso de valor inválido sigue incluyendo el valor original del usuario
  (`%q` con el segmento sin normalizar), lo que facilita la depuración.

---

## 16. i18n / textos visibles

No aplica — el CLI de Vector no tiene sistema de traducciones. Los mensajes del binario son
en inglés y eso no cambia con este spec.

El único texto visible nuevo es la nota en `kit/commands/vector/propose.md` y `fix.md`, que
forma parte de la documentación del command (en inglés, siguiendo el estilo existente del
archivo).

---

## 17. Performance

- Costo despreciable: la normalización de cada segmento son operaciones de string de tiempo
  constante (trim, len, comparación de 3 chars, `ToLower`).
- No se añaden llamadas al filesystem, a la red ni al store de estado.
- No hay renders ni hilo principal que considerar.

---

## 18. Restricciones

El agente no debe:

- Cambiar la firma de `parseArtifacts` ni de `parseFixArtifacts`.
- Modificar `splitCSV` ni ninguna otra función no mencionada en este spec.
- Cambiar el mensaje de error para mencionar `.md` o casing (fuera de scope; ver Open
  questions §3).
- Crear o editar archivos en `docs/`.
- Tocar la máquina de estados, los tipos de `state.ArtifactSet`, ni la transición `draft → open`.
- Editar a mano las copias en `cli/internal/scaffold/assets/commands/vector/`; solo
  regenerarlas con `go generate`.
- Añadir dependencias externas (se mantiene stdlib Go).
- Refactorizar partes no relacionadas de `main.go` ni de `spec_transitions.go`.
- Aceptar más de un nivel de strip de `.md` (solo se elimina un sufijo).
- Aceptar un segmento vacío post-normalización como válido (un `.md` solo debe dar error).
- Cambiar el comportamiento de `parseArtifacts("")` (debe seguir retornando `ArtifactSet{}` sin error).
- Formalizar la convención de changes liviana ni re-redactar "delegate to OpenSpec".
- Introducir tests de integración que requieran `vector` en PATH; los tests son puros (sin disco
  para los parsers, salvo el test existente de `spec_fix_test.go` que usa store).

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `parseArtifacts` en `cli/cmd/vector/main.go` normaliza case-insensitivamente y tolera `.md`.
- [ ] `parseFixArtifacts` en `cli/cmd/vector/spec_transitions.go` normaliza y devuelve nombres canónicos.
- [ ] `cli/cmd/vector/main_test.go` (NUEVO) con `TestParseArtifacts` table-driven, todos los casos del §6 cubiertos.
- [ ] `cli/cmd/vector/spec_transitions_test.go` (NUEVO) con `TestParseFixArtifacts` table-driven, todos los casos del §6 cubiertos.
- [ ] `kit/commands/vector/propose.md` con nota de tolerancia en el paso 6.
- [ ] `kit/commands/vector/fix.md` con nota de tolerancia en la sección §6.
- [ ] `cli/internal/scaffold/assets/commands/vector/propose.md` y `fix.md` regenerados (coinciden con fuentes de `kit/`).
- [ ] `TestAssetsMatchKit` pasa en verde (sin drift entre `kit/` y `assets/`).
- [ ] Gate verde: `go generate ./internal/scaffold`, `gofmt -l cli` (vacío), `go vet ./...`, `go test ./...`, `go build ./...`.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Verifiqué que `parseArtifacts` y `parseFixArtifacts` están en los archivos y líneas citados.
- [ ] La normalización sigue la secuencia LOCKED: trim → strip `.md` case-insensitive → `ToLower` → switch.
- [ ] `parseFixArtifacts` devuelve los nombres canónicos (no los valores crudos del input).
- [ ] El mensaje de error usa el segmento sin normalizar (`part` / `v`) para dar contexto.
- [ ] `.md` solo y `proposal.md.md` dan error (casos de la tabla del §6).
- [ ] `parseArtifacts("")` sigue retornando `ArtifactSet{}` sin error.
- [ ] `TestRunSpecFixValidation` (existente en `spec_fix_test.go`, caso `valid` con `"design,tasks"`) sigue pasando.
- [ ] Los tests nuevos cubren todos los casos de la tabla: `.md`, casing, trim, mixto, vacío, inválidos.
- [ ] Edité solo los archivos listados en §6; justifiqué cualquier excepción.
- [ ] No edité las copias de `assets/` a mano; las regeneré con `go generate`.
- [ ] Ejecuté `go generate ./internal/scaffold` desde `cli/`; `TestAssetsMatchKit` pasa.
- [ ] No añadí dependencias externas.
- [ ] No toqué `docs/`, la máquina de estados ni el wording "delegate to OpenSpec".
- [ ] Ejecuté `gofmt -l cli` (vacío), `go -C cli vet ./...`, `go -C cli test ./...`, `go -C cli build ./...` — todos verdes.
- [ ] No dejé logs temporales ni TODOs sin justificar.

---

## Open questions

1. **Convención de changes liviana vs. modelo delta**: ¿Vector formaliza la forma liviana
   (proposal/design/tasks sin deltas de OpenSpec) como convención propia documentada en
   `docs/`, o deja abierta la adopción del modelo delta de OpenSpec (specs/<cap>/spec.md con
   Requirements + Scenarios)? Hoy ningún `openspec/changes/*` usa deltas; `docs/status.md`
   líneas 152–154 ya trata el modelo delta como non-goal del adapter. Decisión de producto
   deferida por el usuario.

2. **Wording de `propose.md`**: si se formaliza la forma liviana (Open question §1), ¿matizar
   el framing "delegate to OpenSpec" para reflejar que usa el layout liviano, no el modelo de
   deltas? Deferido junto con §1.

3. **Mensaje de error más descriptivo**: ¿el mensaje de error debe mencionar explícitamente
   que `.md` y casing se toleran (p. ej. `allowed proposal,design,tasks; .md optional`)? Por
   ahora se mantiene el mensaje actual. Deferido.
