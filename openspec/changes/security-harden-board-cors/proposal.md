# Harden the board server CORS + add a Host-header check

## Why

The `/security-audit` run on 2026-07-01 flagged a MEDIUM-severity finding in
`cli/cmd/vector/serve.go`: `withCORS` sets `Access-Control-Allow-Origin: *` on the local board
server (GET/OPTIONS). Even though the server binds `127.0.0.1` and is read-only, the wildcard lets
**any website open in the user's browser** issue
`fetch('http://127.0.0.1:8787/api/board' | '/api/file' | '/api/activity')` and read local
spec / project / artifact contents cross-origin while `vector serve` runs. The wildcard also does
nothing against **DNS-rebinding** (a page resolving its own host to `127.0.0.1`). This is a
cross-origin exfiltration path for local spec/board/artifact data during any `vector serve`
session.

## What changes

- Replace the wildcard with an **origin allowlist**: reflect `Origin` back only when it matches
  `http://localhost:<port>` / `http://127.0.0.1:<port>` (plus the Vite dev origin when configured);
  otherwise omit the CORS header entirely.
- Add a **Host-header check** middleware that rejects requests whose `Host` is not
  `localhost` / `127.0.0.1[:port]` (e.g. `403`) — closing the DNS-rebinding vector.
- Keep **SSE** (`/api/events`) working under the tightened policy.
- Add a unit test covering allowed vs rejected `Origin` and `Host`.

## Scope

- **In**: localhost-only hardening of the existing ephemeral board server — origin allowlist,
  Host-header check, and the allow/deny test. The embedded UI and Vite dev proxy must still load
  the board and SSE.
- **Out**: auth tokens on the local server, TLS, or changing the bind address. This stays a
  localhost-only hardening of the current server.
