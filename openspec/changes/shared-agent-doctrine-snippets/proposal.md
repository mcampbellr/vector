# Shared agent doctrine snippets (`_shared/`)

## Why

Los seis agentes del kit (`vector-spec-refiner`, `vector-bug-refiner`, `vector-spec-validator`,
`vector-comment-evaluator`, `vector-summary-writer`, `vector-standup-writer`) replican casi
verbatim varios bloques de doctrina de comportamiento — "Cite, don't guess", "Preserve the
user's language", "Be terse", "Never invent work" y las reglas de prosa humanizada. Cada vez
que esa doctrina necesita una corrección editorial, hay que localizar y editar entre dos y cuatro
ficheros, con riesgo real de divergencia silenciosa si alguno queda desincronizado.

El cambio extrae esos bloques a tres ficheros canónicos bajo `kit/agents/_shared/` y hace que
cada agente los cargue mediante una directiva de lectura explícita, en lugar de duplicarlos
inline. Un solo cambio editorial se propaga a todos los consumidores automáticamente.

## What changes

- Tres ficheros nuevos bajo `kit/agents/_shared/`: `citation-discipline.md`,
  `prose-rules.md` y `refiner-base.md`, cada uno con la doctrina canónica correspondiente.
- Seis agentes existentes modificados para reemplazar los bloques duplicados con una directiva
  de carga que referencia el fichero `_shared/` correcto.
- Re-ejecución de `go generate` en `cli/internal/scaffold/` para sincronizar
  `assets/agents/_shared/` con el nuevo estado de `kit/agents/`.
- Actualización de `.claude/agents/` del propio repo de Vector (dogfooding) vía
  `vector update` o copia directa.
- Test de consistencia estructural `TestSharedDoctrineNotInlined` en
  `cli/internal/scaffold/scaffold_test.go` que bloquea regresiones de reinserción inline.

## Scope

- In: creación de los tres ficheros `_shared/`, modificación de los seis agentes, re-generación
  de assets embebidos, actualización del dogfooding local y test de consistencia.
- Out: fusión de agentes entre sí, extracción de contenido que no aparezca casi verbatim en ≥2
  agentes, cambios al binario Go más allá de re-ejecutar `go generate`, cambios al
  `spec-template` o a las rules de `.claude/rules/`, compresión de reglas que son el núcleo
  de los gates adversariales.

Authored spec: `.vector/specs/shared-agent-doctrine-snippets/spec.md`
