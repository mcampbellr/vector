# Tasks — enforce-web-embed-integrity

## 1. Placeholder + gitignore

- [ ] 1.1 Crear `cli/internal/webui/dist/index.placeholder.html`: HTML5 mínimo, sin refs a
      `/assets/*`, texto "Vector board not built — run `make install`". Trackeado.
- [ ] 1.2 `.gitignore`: añadir `cli/internal/webui/dist/index.html` (ruta exacta, sin capturar el
      placeholder); mantener el ignore de `dist/assets/`.
- [ ] 1.3 Corregir el comentario stale de `.gitignore:15-16` ("committed placeholder index.html")
      para reflejar el nuevo modelo.

## 2. Serve fallback + doc-comment

- [ ] 2.1 `webui.go` `spaHandler.ServeHTTP` (`:124-146`): servir `index.placeholder.html` (200,
      `text/html`) cuando `dist/index.html` no está en el embed. No tocar el 404 de `/assets/*`
      faltantes ni el warning de `serve`.
- [ ] 2.2 Reconciliar el doc-comment del paquete (`webui.go:1-5`) con el mecanismo real
      (placeholder = archivo separado; `index.html` real gitignored).

## 3. Guard de build-time

- [ ] 3.1 `webui_test.go`: `TestEmbeddedBoardIntegrity` — caso "no buildeado" (sin `index.html` en
      el embed) pasa explícitamente; caso "ref dangling" (`fstest.MapFS` con `/assets/*` inexistente)
      falla vía `ValidateAssets`. Autocontenible (sin `npm`/`node`).

## 4. Path canónico de instalación

- [ ] 4.1 `Makefile` (raíz): targets `web-build` → `embed` → `guard` → `build` → `install`,
      abort-on-fail, `VECTOR_INSTALL_DIR ?= $(HOME)/.local/bin/vector`, `.PHONY`.
- [ ] 4.2 `scripts/dev-install.sh`: equivalente portable (`set -euo pipefail`, `mkdir -p` del
      destino, exit no-cero si falta node/npm o falla el guard), diferenciado de `scripts/install.sh`.

## 5. Doctrina

- [ ] 5.1 `.claude/rules/architecture/distribution-packaging.md`: reemplazar la doctrina
      runtime-only por el nuevo modelo (placeholder + guard decidible + `make install` canónico);
      actualizar el bloque "Flujo de edición del frontend" para referenciar el target `embed`.

## 6. Verificación

- [ ] 6.1 `gofmt -l cli` (vacío), `go -C cli vet ./...`, `go -C cli test ./internal/webui/...` verdes.
- [ ] 6.2 `go test ./internal/webui` pasa en checkout limpio (sin `web build`) — prueba la distinción.
- [ ] 6.3 `make install` dry-run completo produce board funcional; `git status` no stagea
      `dist/index.html` ni `dist/assets/`; `index.placeholder.html` trackeado sin diffs espurios.
- [ ] 6.4 Nota de seguimiento: actualizar la Memory `reinstall-vector-binary-after-changes` a
      `make install` (no código del repo).
