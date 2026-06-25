# 🧭 Vector — Vault de documentación

Índice de navegación. Abre el **Graph view** (`⌘ G`) para ver las relaciones entre notas
(coloreadas por tema).

> **Retomando una sesión?** Empezá por [[status]] — estado actual, qué está construido,
> próximos pasos y gotchas.

## Visión y producto
- [[vision]] — visión general: idea cruda → workflow V1, principios, comandos
- [[vision-raw]] — captura cruda original
- [[commercialization]] — go-to-market / análisis `/biz` (open-core, Token Savings wedge, pricing)

## Diseño técnico
- [[domain-contract]] — **contrato de dominio LOCKED**: estados, columnas=estado, comando→state, web↔cli
- [[plugin-and-commands]] — **commands `/vector:*`** (project commands, namespace por subdirectorio), binario global vs commands per-proyecto, distribución
- [[state-architecture]] — arquitectura de estado: 3 capas, CLI-owns-writes, apply wrapper, ciclo de vida
- [[sync-and-dedup]] — **sync & dedup LOCKED**: identidad=slug, colapso multi-worktree, `supersededBy`, el patrón que evita duplicados
- [[apply-design]] — notas de diseño de **`/vector:apply`** (no LOCKED): recorrido de apply + **autonomía configurable** (auto/ask/always-ask) usando el status traqueado
- [[repo-analysis-synthesis]] — síntesis de **cdr · private-wealth · somnio** y la "forma Vector"
- [[state-and-activity]] — esquemas concretos `state.json` / `activity.jsonl` (Go + ejemplos)

## UI
- [[kanban-ui-reference]] — referencia del board kanban (incluye la imagen)

---

> Estado del proyecto: captura + diseño. **Nada de código de Vector implementado aún.**
> El árbol de docs es la fuente de verdad mientras iteramos.
