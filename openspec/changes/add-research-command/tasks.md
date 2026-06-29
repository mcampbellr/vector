# Tasks — add-research-command

## 1. Feasibility reviewer agent (Sonnet, per lens)

- [x] 1.1 `kit/agents/vector-feasibility-reviewer.md`: frontmatter tier **Sonnet**, read-only
      (Read, Grep, Glob); patrón `vector-comment-evaluator.md` / `vector-spec-validator.md`.
- [x] 1.2 Recibe `LENS` (`technical`|`security`|`marketing`|`design`), la idea refinada y el
      contexto del repo; reúne su propia evidencia.
- [x] 1.3 Rubric por lente: `technical` (factibilidad con stack/arquitectura, esfuerzo,
      dependencias faltantes, riesgos de integración); `security` (superficie de ataque, PII/secrets,
      permisos, operaciones destructivas — `security/destructive-ops-consent.md`); `marketing`
      (encaje con producto/valor, diferenciación, comercial día 0); `design` (necesidad/complejidad
      de UI/UX, impacto en board/panel web, accesibilidad).
- [x] 1.4 Salida estructurada: `LENS`, `VERDICT` (`go`/`go-with-risks`/`no-go`) + `N/10`,
      `FINDINGS` (evidencia `file:line` cuando aplique), `RISKS`, `RECOMMENDATION`. No edita
      archivos, no implementa, no acepta el framing sin evaluarlo.

## 2. Project command `/vector:research`

- [x] 2.1 `kit/commands/vector/research.md`: frontmatter (`name: research`,
      `argument-hint: "[idea-text]"`, `user-invocable: true`, `allowed-tools`: Read, Grep, Glob,
      `Bash(vector *)`, Agent, AskUserQuestion).
- [x] 2.2 Confirmar repo init + resolver `specPath`/`config.language` de `.vector/config.json`
      (si falta, avisar/correr `vector init`, como `raw`). Idea vacía → último mensaje, si no
      `AskUserQuestion`.
- [x] 2.3 Detectar lentes (main loop, barato): `technical` siempre; resto por señales (§13 del
      spec); mostrar el set; ambigüedad → `AskUserQuestion` sin forzar. Mantener `LENSES`.
- [x] 2.4 Refinar (Haiku): invocar `vector-spec-refiner` (idea + ejemplo + plantilla) → `BRIEF`;
      registrar `agent.routed`. Clarificar dimensiones abiertas con el usuario (batches ≤5) hasta
      quedar sin preguntas pendientes (o `TBD`).
- [x] 2.5 Revisar viabilidad (Sonnet, una invocación por lente en `LENSES`, en paralelo): invocar
      `vector-feasibility-reviewer` con `LENS` + idea refinada + contexto; registrar `agent.routed`
      por lente.
- [x] 2.6 Re-chequear (main loop): validar la evidencia `file:line`; degradar veredictos no
      sostenidos; salida no parseable → `go-with-risks` no concluyente, ofrecer reintentar.
- [x] 2.7 Consolidar veredicto global (no-go si una lente crítica es no-go; go-with-risks si hay
      riesgos; go si todas pasan) con resumen por lente.
- [x] 2.8 Gate go/no-go (`AskUserQuestion`): presentar el veredicto consolidado; emitir / refinar
      más / abortar. Abortar → terminar sin crear card ni escribir spec doc; confirmar al usuario.
- [x] 2.9 Token routing documentado (detección/orquestación en main loop; refinamiento Haiku;
      revisiones+validación Sonnet) + recordatorio CLI-owns-writes.

## 3. Composición del spec + reporte embebido

- [x] 3.1 Componer 20 secciones de la plantilla (`.claude/vector/spec-template.md`, cada `[...]`
      resuelto con contenido verificado) + anexo `## Reporte de viabilidad` (tabla por lente +
      hallazgos + riesgos + veredicto consolidado) + anexo `## Open questions`.
- [x] 3.2 Derivar `title` (≤ ~8 palabras) y `id` kebab-case; detectar ticket en la idea (lógica de
      `raw`); priority solo si la idea lo implica.
- [x] 3.3 Validar (Sonnet): invocar `vector-spec-validator` con el spec compuesto + ejemplo +
      plantilla + checklist; reaccionar al verdict (PASS/WARN/BLOCK, máx. 3 ciclos); registrar
      `agent.routed`. `BLOCK` tras 3 ciclos → no registrar, surfacer el reporte y detenerse.

## 4. Registro de la card draft + token meter

- [x] 4.1 `vector spec create --title --id [--priority] [--ticket] --status draft --body-file -
      --json <<<spec`; parsear `id`/`status`/`specDoc`. Nunca editar `.vector/` a mano.
- [x] 4.2 `--ticket` rechazado (JSON malformado / provider no inferible) → reintentar **sin**
      `--ticket` y sugerir `/vector:link`. Nunca bloquear la creación por el ticket.
- [x] 4.3 Reportar: id, `status: draft`, `specDoc`, veredicto consolidado, siguiente paso
      (`/vector:propose`). Idioma del reporte: `config.language`, fallback a la conversación.

## 5. Vendoring + verificación

- [x] 5.1 `go -C cli generate ./internal/scaffold` (copia a `assets/`); no editar los assets a mano.
- [x] 5.2 `cli/internal/scaffold/scaffold_test.go`: añadir el par `research.md` +
      `vector-feasibility-reviewer.md` al esperado si enumera el set (patrón
      `TestSeedCommandsSeedsBugCommandAndRefiner`).
- [x] 5.3 `go -C cli vet ./...` y `go -C cli test ./internal/scaffold/...` en verde.
- [x] 5.4 Verificar que `vector init` en repo limpio siembra `research.md` +
      `vector-feasibility-reviewer.md`.

## 6. Docs

- [x] 6.1 Actualizar `docs/plugin-and-commands.md` con `/vector:research` si enumera los `/vector:*`.
