# Design — security-harden-board-cors

## Decisiones clave

- **Allowlist reflejada, no wildcard**: `withCORS` deja de emitir `Access-Control-Allow-Origin: *`.
  En su lugar lee el header `Origin` de la request y solo lo refleja (`Access-Control-Allow-Origin:
  <origin>`) cuando matchea el conjunto permitido: `http://localhost:<port>` y
  `http://127.0.0.1:<port>` (el puerto real del servidor), más el origin del dev server de Vite si
  está configurado. Si no matchea, **se omite el header** (no se emite `*` ni el origin) — el
  navegador bloquea la lectura cross-origin.
- **Host-header check como middleware separado**: un wrapper previo a las rutas valida que el
  `Host` de la request sea `localhost` o `127.0.0.1` (con o sin `:<port>`). Cualquier otro Host →
  `403`. Esto cierra DNS-rebinding, donde el `Origin` puede ser legítimo pero el `Host` apunta a un
  dominio del atacante que resuelve a `127.0.0.1`.
- **SSE intacto**: `/api/events` sigue funcionando bajo la política endurecida — la allowlist y el
  Host-check aplican igual a la conexión SSE; se verifica que el streaming no se rompa (el header de
  CORS reflejado es suficiente para el `EventSource` del UI embebido servido desde el mismo origin).
- **Puerto dinámico**: la allowlist se construye a partir del puerto efectivo con el que arranca el
  server (no hardcodear `8787`), para que siga siendo correcta cuando el puerto se resuelve en
  runtime.
- **Alcance localhost-only**: no se agregan tokens de auth, TLS ni cambio de bind address. El
  servidor sigue siendo efímero y read-only sobre `127.0.0.1`; esto es solo endurecimiento del
  control de acceso cross-origin existente.

## Superficie

- `cli/cmd/vector/serve.go` → `withCORS`: reemplazar el wildcard por la lógica de allowlist
  reflejada; encadenar el nuevo middleware de Host-check delante del handler.
- `internal/board` (rutas `/api/board`, `/api/events`, artifact/file endpoints): consumidas a
  través de los middlewares; sin cambios de contrato, solo pasan por el gate.
- Test nuevo (junto a `serve.go` / `internal/board`): cubre allow/deny de `Origin` y `Host`.

## Flujo

Request al board server → middleware Host-check valida `Host ∈ {localhost, 127.0.0.1}[:port]`
(si no, `403`) → `withCORS` inspecciona `Origin`: si matchea la allowlist lo refleja en
`Access-Control-Allow-Origin`, si no omite el header → handler de la ruta responde
(`/api/board`, `/api/events` SSE, file/artifact). El UI embebido y el proxy de Vite (mismo origin
localhost) pasan; una página foránea recibe respuesta sin header CORS y el navegador bloquea la
lectura.

## Riesgos / consideraciones

- **Origin del dev server de Vite**: debe incluirse explícitamente en la allowlist cuando se
  desarrolla el UI fuera del binario; si no, el board no cargará en modo dev. Documentar/derivar
  ese origin de la config existente.
- **Match de puerto/host**: el parsing de `Origin`/`Host` debe tolerar la presencia/ausencia de
  puerto y no confundir `127.0.0.1` con hosts que lo contengan como substring — comparar por
  host+puerto parseados, no por `strings.Contains`.
- **Preflight OPTIONS**: mantener la respuesta correcta a OPTIONS para las rutas que el UI consulta,
  ahora condicionada a la allowlist.
