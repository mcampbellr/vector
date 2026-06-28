# Single-source kit assets

## Why

Los assets del kit (agentes y commands de `/vector:*`) viven actualmente en tres lugares: la
fuente editable en `kit/`, la copia vendorizada generada en `cli/internal/scaffold/assets/`, y
las copias sembradas en `.claude/agents/` y `.claude/commands/vector/` del propio repo Vector.
Los últimos dos grupos se rastrean en git, lo que produce drift silencioso: un cambio en `kit/`
no se refleja en el binario ni en el dogfooding hasta que alguien recuerde correr `go generate`
y `vector update` manualmente. El repo Vector termina comprometiendo artefactos generados como
si fueran fuente, oscureciendo el flujo de propagación y aumentando la superficie de conflictos
de merge.

## What changes

- `kit/` pasa a ser la **única fuente editable** de forma explícita y documentada.
- Los 11 archivos de `.claude/agents/` y `.claude/commands/vector/` que hoy están rastreados se
  desrastrean con `git rm --cached` y pasan a estar cubiertos por `.gitignore`.
- Se añade un test `TestAssetsMatchKit` que falla si `cli/internal/scaffold/assets/` diverge de
  `kit/`, proveyendo un gate local de drift sin necesidad de CI activo.
- El comentario de paquete de `scaffold.go` documenta el flujo canónico de cuatro pasos: editar
  en `kit/` → `go generate` → reinstalar binario → `vector update`.
- La rule `distribution-packaging.md` recibe una subsección con ese flujo, y la nota de Memory
  de reinstalación añade `vector update` como paso obligatorio.
- Un workflow de GitHub Actions (TBD, bloqueado por Open questions §1) correría el drift check
  en CI.

## Scope

**In:**
- `git rm --cached` de los 11 archivos rastreados de `.claude/`.
- Nuevas reglas en `.gitignore` para `.claude/agents/` y `.claude/commands/vector/`.
- `TestAssetsMatchKit` en `scaffold_test.go`.
- Actualización del comentario de paquete de `scaffold.go` (solo el bloque de comentario).
- Actualización de `.claude/rules/architecture/distribution-packaging.md`.
- Actualización de la nota de Memory de reinstalación.
- Workflow `.github/workflows/ci.yml` (si se activa CI — TBD).

**Out:**
- Cambios en la lógica de `SeedCommands`, `writeSeed`, `CommandPaths` o `writeFileAtomic`.
- Modificación del `//go:generate` o del `//go:embed`.
- Cambios de contenido en cualquier archivo de `kit/` o de `cli/internal/scaffold/assets/`.
- Tocar `web/`, `cli/cmd/`, `cli/internal/state/`, `cli/internal/board/` u otros paquetes no relacionados con el scaffold.
- Publicación ni CI de release; solo el check de drift en desarrollo.

Authored spec: `.vector/specs/single-source-kit-assets/spec.md`.
