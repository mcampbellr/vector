# Tasks — security-harden-board-cors

## 1. Origin allowlist

- [ ] 1.1 En `cli/cmd/vector/serve.go` (`withCORS`), eliminar `Access-Control-Allow-Origin: *`.
- [ ] 1.2 Construir la allowlist a partir del puerto efectivo del server:
  `http://localhost:<port>` y `http://127.0.0.1:<port>` (+ origin de Vite si está configurado).
- [ ] 1.3 Reflejar `Origin` en `Access-Control-Allow-Origin` solo si matchea la allowlist; en caso
  contrario omitir el header. Comparar por host+puerto parseados, no por substring.
- [ ] 1.4 Mantener la respuesta de preflight `OPTIONS` condicionada a la allowlist.

## 2. Host-header check

- [ ] 2.1 Agregar un middleware que valide `Host ∈ {localhost, 127.0.0.1}[:port]` delante de los
  handlers del board.
- [ ] 2.2 Rechazar con `403` cualquier request cuyo `Host` no matchee (cierra DNS-rebinding).

## 3. SSE

- [ ] 3.1 Verificar que `/api/events` (SSE) sigue funcionando bajo la política endurecida desde el
  UI embebido y el proxy de Vite.

## 4. Verificación

- [ ] 4.1 Test unitario: request con `Origin` foráneo → **sin** header `Access-Control-Allow-Origin`.
- [ ] 4.2 Test unitario: request con `Host` no-localhost → `403`.
- [ ] 4.3 Test unitario: `Origin`/`Host` localhost permitidos → header reflejado y `200`.
- [ ] 4.4 Manual: UI embebido + proxy de Vite cargan board y SSE correctamente.
