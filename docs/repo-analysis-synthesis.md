# Vector — Síntesis del análisis de 3 repos (cdr · private-wealth · somnio)

> Análisis read-only de la **estructura de manejo con agentes** (no del código) de tres repos
> reales del usuario, para definir la "forma" a la que `/vector init` debe reorganizar.
> Hecho con 3 agentes Explore en paralelo (~139k tokens en subagentes, fuera del contexto
> principal) — prueba de concepto del ruteo a agentes baratos.

## Patrón común (el "stack ganador" que Vector adopta)

| Patrón | cdr | private-wealth | somnio |
|--------|-----|----------------|--------|
| Monorepo + bare repo + worktree-per-spec (pnpm + Turborepo) | ✅ | ✅ | ✅ |
| OpenSpec `/opsx:explore→propose→apply→archive` (proposal→design→tasks, `.openspec.yaml`) | ✅ | ✅ (vacío) | ✅ (intensivo) |
| Graphify como memoria del codebase, consultar antes de trabajar | ✅ (hook) | ✅ (regla) | ✅ (regla) |
| Docs por madurez: `status/`(READY/WIP/PENDING) · `decisions/`(YYYY-MM-DD) · `roadmap/` · `features/` · `feedback/` · `archive/` · `TODO.md` | ✅ | ✅ | ✅ (+ADR/SPARC) |
| `.project-structure` (devup): bare-repo, branch-prefix, `run:`, tunnel | ✅ | ✅ | ✅ |
| CLAUDE.md jerárquico (root → workspace → worktree) | ✅ | ✅ | ✅ |
| Verification gate (build+lint+test antes de "done") | parcial | parcial | ✅ explícito |

## Inconsistencias detectadas → lo que Vector resuelve

1. **Estado en markdown narrativo, no queryable** — `project-status.md` (cdr/pw) vs
   `.planning/STATE.md` 100+ líneas (somnio) vs `quick-wins.md`. Ninguno tiene JSON / board /
   dashboard live. **→ Wedge central de Vector: JSON por-spec + board + daily notes.**
2. **Doble fuente de verdad para specs**: `openspec/specs/` vs `docs/specs/<slug>/spec.md`
   (cdr y somnio). → Vector fija UNA ubicación.
3. **Capa de planning fragmentada**: GSD `.planning/` solo en somnio; cdr/pw usan vault.
4. **Constraints de agente en dos formatos**: `.claude/rules/` (cdr, pw) vs `AGENTS.md` (somnio).
   → Vector estandariza uno.
5. **Reminders como prosa best-effort** (graphify refresh, context-decay re-read, "mantené el
   status al día") — nunca automatizados. **→ Confirma: hooks deterministas, no memoria del modelo.**
6. **Adopción de OpenSpec dispar**: intensivo (somnio), medio (cdr, 17 changes), montado-pero-
   vacío (pw). → Vector hace el flujo el camino por defecto.
7. **Commits**: Conventional Commits enforced por hook solo en somnio; cdr/pw sin enforcement.
8. **Memoria de sesión inconsistente**: distribuida en STATE.md (somnio), 1 archivo mínimo
   (cdr), graphify-as-memory (pw). → Vector estandariza el handoff entre sesiones.
9. **`settings.json` con hooks ausente en varios `.claude/` root** — desaprovecha hooks globales.

## Hallazgo más relevante para el diseño

somnio documenta el pipeline maestro en `docs/workflow-reference.md`:
`/idea → spec → /openspec-propose → /opsx:apply → /branch-ship`.
**Es casi 1:1 con el flujo de Vector**: `/vector:raw` ≈ `/idea`, `/vector:apply` ≈ `/opsx:apply`.
Vector no inventa el flujo — lo **estandariza, lo hace queryable y le agrega el board + token economics**.

## "Forma Vector" propuesta (destino de `/vector init`)

```
repo/
├── .vector/
│   ├── specs/<id>/spec.md           # spec (OpenSpec) — fuente única (resuelve #2)
│   ├── specs/<id>/state.json        # estado por-spec, commiteado (sharded → sin conflictos)
│   ├── board.json                   # DERIVADO (no commiteado) → alimenta el dashboard
│   └── local/activity.jsonl         # personal, gitignored → /vector:daily (daily notes)
├── .claude/
│   ├── commands/vector/*            # /vector:raw :link :status :daily :apply :close :archive
│   ├── rules/*                      # constraints de agente (formato único — resuelve #4)
│   └── settings.json                # HOOKS: json-up-to-date · graphify-refresh · need-attention
├── .project-structure               # devup unificado (run:, bare-repo, tunnel) — resuelve unificación
└── docs/                            # vault por madurez (status/ deriva de board.json)
```

Decisiones que materializa:
- Estado **sharded por-spec** commiteado (resuelve #1 sin conflictos de merge).
- `board.json` **derivado** → panel web read-only (V1 informativo).
- `activity.jsonl` **local** → daily notes y "qué moviste".
- **Hooks** sustituyen los reminders-prosa de los 3 repos (resuelve #5).
- Adopta worktree-per-spec + docs-por-madurez (patrón común probado).

## Pendiente de decidir con el usuario

- ¿Specs bajo `.vector/specs/` o reutilizar `openspec/` nativo? (compatibilidad con OpenSpec CLI).
- ¿`rules/` vs `AGENTS.md` como formato canónico de constraints?
- ¿Vector enforced Conventional Commits vía hook (como somnio) por defecto?
- Mapeo exacto entre estados del board (open/in progress/need attention/review/closed) y el
  ciclo OpenSpec (proposed/applied/archived).
