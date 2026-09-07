# Design — enforce-web-embed-integrity

## Decisiones clave

- **Placeholder trackeado hace decidible la distinción** "no buildeado" vs "roto": al dejar de
  trackear el `index.html` hasheado y trackear `index.placeholder.html` (sin refs a `/assets/*`),
  el checkout limpio no tiene refs que satisfacer → el guard pasa; las refs hasheadas solo
  aparecen tras un `web build` real, donde los assets deben existir o el guard falla. Esto
  **revierte** la doctrina runtime-only de `distribution-packaging.md`.
- **`ValidateAssets` se reutiliza sin duplicar** como núcleo compartido entre el warning de
  runtime (sin cambios) y el nuevo test de build-time. Corta-circuita retornando `nil` cuando
  `dist/index.html` está ausente del embed (`webui.go:35-52`) — ese es el mecanismo del pase en
  checkout limpio, no la evaluación del set vacío del placeholder.
- **`make install` / `scripts/dev-install.sh` = único path sancionado** de reinstalación desde
  fuente; aborta antes de sobrescribir `$VECTOR_INSTALL_DIR` si el guard falla. El warning de
  runtime queda como red secundaria (no se remueve).
- **CI `go` job sin cambios** (no corre `web build`) y verde — el guard es decidible en ese
  contexto.
- **Rechazado**: trackear `dist/assets/`; usar `go:generate`; mantener `index.html` hasheado
  gateando el test con un build-marker (solo enmascara; deja el checkout limpio vulnerable a
  board en blanco vía bare `go build`).

## Superficie

- `Makefile` (raíz): targets `web-build` → `embed` → `guard` → `build` → `install` encadenados
  por dependencia, abort-on-fail; `VECTOR_INSTALL_DIR ?= $(HOME)/.local/bin/vector`.
- `scripts/dev-install.sh`: equivalente portable (`set -euo pipefail`), diferenciado de
  `scripts/install.sh` (release).
- `cli/internal/webui/webui_test.go`: `TestEmbeddedBoardIntegrity` (caso "no buildeado" pasa; caso
  "ref dangling" vía `fstest.MapFS` falla).
- `cli/internal/webui/dist/index.placeholder.html`: nuevo, trackeado, sin refs a `/assets/*`.
- `cli/internal/webui/webui.go`: fallback al placeholder en `spaHandler.ServeHTTP`
  (`webui.go:124-146`) cuando falta `index.html`; reconciliar el doc-comment del paquete
  (`webui.go:1-5`).
- `.gitignore`: ignorar `cli/internal/webui/dist/index.html`; corregir el comentario stale de
  `:15-16`.
- `.claude/rules/architecture/distribution-packaging.md`: nueva doctrina (placeholder + guard +
  `make install` canónico).

## Flujo

`make install` → `npm --prefix web run build` → re-embed (`rm -rf` + `cp -R`) →
`go test ./internal/webui -run TestEmbeddedBoardIntegrity` (aborta si falla) → `go build -o
$VECTOR_INSTALL_DIR` → instala. Off-path (bare `go build` sin `web build`) → el binario embebe el
placeholder honesto, no un blanco; el warning de runtime sigue disparando.

## Open questions

- Fijar o no una versión de Node/npm pineada (`.nvmrc`/`engines`) — hoy no existe. Ver spec §Open
  questions.
