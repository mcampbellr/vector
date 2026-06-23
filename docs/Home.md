# 🧭 Vector — Vault de documentación

Índice de navegación. Abre el **Graph view** (`⌘ G`) para ver las relaciones entre notas
(coloreadas por tema).

## Visión y producto
- [[vision]] — visión general: idea cruda → workflow V1, principios, comandos
- [[vision-raw]] — captura cruda original
- [[commercialization]] — go-to-market / análisis `/biz` (open-core, Token Savings wedge, pricing)

## Diseño técnico
- [[domain-contract]] — **contrato de dominio LOCKED**: estados, columnas=estado, comando→state, web↔cli
- [[plugin-and-commands]] — **plugin `vector`** (namespace `/vector:*`), binario vs plugin, distribución
- [[state-architecture]] — arquitectura de estado: 3 capas, CLI-owns-writes, apply wrapper, ciclo de vida
- [[repo-analysis-synthesis]] — síntesis de **cdr · private-wealth · somnio** y la "forma Vector"
- [[state-and-activity]] — esquemas concretos `state.json` / `activity.jsonl` (Go + ejemplos)

## UI
- [[kanban-ui-reference]] — referencia del board kanban (incluye la imagen)

---

> Estado del proyecto: captura + diseño. **Nada de código de Vector implementado aún.**
> El árbol de docs es la fuente de verdad mientras iteramos.
