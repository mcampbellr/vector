# Tasks — conditional-apply-model

## 1. Config — tipo ApplyModel

- [x] 1.1 Agregar tipo `ApplyModel string` con constantes `ApplyModelOpus`, `ApplyModelSonnet`, `ApplyModelConditional` en `cli/internal/config/config.go`, después de la sección `ApplyMode`.
- [x] 1.2 Implementar `func (m ApplyModel) Valid() bool` siguiendo el patrón de `ApplyMode.Valid()`.
- [x] 1.3 Agregar campo `ApplyModel ApplyModel` con tag `json:"applyModel,omitempty"` en el struct `Config`, después del campo `ApplyMode`.
- [x] 1.4 Implementar `func (c *Config) ResolvedApplyModel() ApplyModel` retornando `ApplyModelOpus` cuando el campo está vacío o es inválido.
- [x] 1.5 Agregar validación en `Load()`: si `c.ApplyModel != ""` y `!c.ApplyModel.Valid()`, retornar error `"invalid applyModel %q: allowed opus,sonnet,conditional"`, siguiendo el patrón de `DefaultTicketProvider`.

## 2. Tests de config

- [x] 2.1 `ApplyModel.Valid()` retorna true para `opus`, `sonnet`, `conditional`; false para `""`, `"haiku"`, `"SONNET"`, `"auto"` (table-driven).
- [x] 2.2 `ResolvedApplyModel()` retorna `ApplyModelOpus` para campo vacío, `ApplyModelOpus` para valor `"opus"` explícito, y `ApplyModelOpus` ante valor inválido (table-driven).
- [x] 2.3 `Load()` retorna error accionable para un config con `"applyModel": "haiku"`.
- [x] 2.4 Config sin campo `applyModel` carga sin error y con campo vacío en el struct (backward-compat).

## 3. Binario — runSpecNext

- [x] 3.1 En `runSpecNext()` (`cli/cmd/vector/spec_transitions.go`): resolver `applyModel` desde `cfg.ResolvedApplyModel()` después de resolver `mode`.
- [x] 3.2 Incluir `"applyModel": string(applyModel)` en el mapa JSON junto a `"applyMode"` (caso work-item encontrado y caso `nothing actionable`).
- [x] 3.3 Incluir `[applyModel: ...]` en la salida humana junto a `[applyMode: ...]`.
- [ ] 3.4 Test o caso de integración: JSON de `runSpecNext` incluye `"applyModel"` con el valor resuelto cuando el config tiene el campo, y `"opus"` cuando no.

## 4. Command kit — apply.md §3a

- [x] 4.1 Insertar sección `## 3a. Evalúa el tier del modelo` entre `§3` (Detect the mode) y `§4` (Implement) en `kit/commands/vector/apply.md`, sin alterar la numeración ni el contenido de §1–§3 ni §5–§8.
- [x] 4.2 La sección §3a debe incluir: instrucción de leer `applyModel` del JSON de `next` (o de `.vector/config.json` en continuaciones directas), tabla de dispatch por valor (`opus`/`sonnet`/`conditional`), tabla de las cinco señales del criterio mecánico, instrucción de fallback conservador a Opus ante artefactos ausentes o señales ambiguas.
- [x] 4.3 Cuando tier = Sonnet: instrucción de despachar a `vector-apply-impl` con el brief estructurado (campos `spec_id`, `proposal`, `design`, `tasks`, `repo_root`, `build_cmd`, `test_cmd`, `mode`, `openspec_change`); no implementar inline; consumir el JSON resultado para §5 y §6a.
- [x] 4.4 Modificar `## 4. Implement`: añadir al inicio la nota "Si el tier fue asignado a Sonnet en §3a, omitir esta sección: la implementación ya está delegada al subagente." El cuerpo existente permanece intacto para el path Opus inline.

## 5. Agente kit — vector-apply-impl.md

- [x] 5.1 Crear `kit/agents/vector-apply-impl.md` con frontmatter correcto: `name: vector-apply-impl`, `description` descriptiva, `model: sonnet`, `tools: Read, Edit, Write, Bash`.
- [x] 5.2 El agente debe leer el brief estructurado recibido vía prompt e identificar los campos requeridos (`spec_id`, paths a artefactos, `repo_root`, `build_cmd`, `test_cmd`, `mode`).
- [x] 5.3 Leer los artefactos del change desde disco usando los paths del brief; omitir los que no existan y anotarlo en `note`.
- [x] 5.4 Implementar el código siguiendo `tasks.md`/`proposal.md`/`design.md` (o spec doc en nativo), marcando checkboxes conforme avanza.
- [x] 5.5 Correr el gate de build/test usando `build_cmd`/`test_cmd`; incluir resultado en `build_passed`/`test_passed`; detenerse y reportar si falla.
- [x] 5.6 Detectar bloqueadores externos (mismas señales del §6a de `apply.md`); si detectado, `"blocked": true` en el resultado.
- [x] 5.7 Retornar SOLO un JSON con las claves: `files_changed`, `tasks_completed`, `tasks_pending`, `build_passed`, `test_passed`, `blocked`, `note`. En error no recuperable: listas vacías, booleans false, `note` con descripción.
- [x] 5.8 Hard rules explícitas: NO llamar al binario `vector`, NO hacer git commits, NO editar `.vector/`.

## 6. Assets vendorizados

- [x] 6.1 Copiar `kit/commands/vector/apply.md` modificado a `cli/internal/scaffold/assets/commands/vector/apply.md` (vía `go generate` o manualmente si el script ya existe).
- [x] 6.2 Copiar `kit/agents/vector-apply-impl.md` nuevo a `cli/internal/scaffold/assets/agents/vector-apply-impl.md` (vía `go generate`).

## 7. Documentación

- [x] 7.1 Actualizar `docs/apply-design.md` §3 ("Config `applyMode` en `.vector/config.json`") para reflejar el nuevo campo `applyModel` y sus tres valores.

## 8. Verificación

- [x] 8.1 `gofmt -l cli` sin output (sin archivos mal formateados).
- [x] 8.2 `go -C cli vet ./...` sin warnings.
- [x] 8.3 `go -C cli test ./...` verde (incluyendo los nuevos tests de config del paso 2).
- [ ] 8.4 Smoke manual: config sin `applyModel` → `vector spec next --json` incluye `"applyModel":"opus"`.
- [ ] 8.5 Smoke manual: config con `applyModel: "conditional"` → `vector spec next --json` incluye `"applyModel":"conditional"`.
- [ ] 8.6 Smoke manual: config con `applyModel: "haiku"` → `vector` falla con error accionable antes de ejecutar el command.
- [ ] 8.7 Verificar que ningún config generado por `vector init` o `vector update` incluye el campo `applyModel`.
