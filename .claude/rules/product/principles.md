# Product — Principios

> Aplica a: toda decisión de producto, UX o scope. Si un cambio contradice uno de estos,
> detente y plantéalo.

## Developer-focused, no project-manager-focused

- Vector es para developers que trabajan con Claude Code. La UX prioriza el flujo del dev
  (specs, tokens, repo) sobre rituales de gestión de proyecto.
- El board kanban sirve al dev para administrar specs, no para reporting gerencial.

## Agnóstico al código del usuario

- Vector **organiza y estructura** el manejo con agentes; **no** impone arquitectura,
  framework ni convenciones sobre el repo del usuario.
- Toda detección (techstack, git/commit convention, repo type mono/micro) se descubre en
  `/vector init` y se registra; nunca se asume ni se hardcodea.

## Eficiencia de tokens como feature

- El ahorro de tokens es valor de producto, no detalle técnico. Ver `product/token-routing.md`.

## Comercial desde el día 0

- No es solo para uso personal: instalación simple, distribución y onboarding cuentan desde
  el inicio (`architecture/distribution-packaging.md`).
- Antes de definir el formato objetivo de `/vector init`, se contempla análisis `/biz` +
  estudio de repos de referencia (solo estructura, ver `docs/vision.md`).

## Unifica `devup`

- La herramienta existente `devup` del usuario se incorpora dentro de Vector (lanzar el dev
  local vía bloque `run:` en `.project-structure`). Vive en `kit/`.

> Estado: pendiente — formato objetivo al que `/vector init` reorganiza el repo (pregunta
> abierta #3 del vision), a definir tras análisis de repos de referencia + `/biz`.
