# Design — security-add-dependency-scanning-ci

## Decisiones clave

- **Workflow dedicado, no un step en el CI existente**: un `.github/workflows/security-scan.yml`
  aislado mantiene el gate de vulnerabilidades separado del pipeline de build/test, con su propio
  trigger (`pull_request` + `schedule` semanal) y permisos mínimos.
- **Dos gates que bloquean, uno advisory**: `govulncheck ./...` (Go) y `npm audit
  --audit-level=high` (web) fallan el build en HIGH+. Un eventual step de Trivy queda advisory
  (`continue-on-error`) para no romper por findings de imagen/FS de baja accionabilidad.
- **`govulncheck` instalado por job, no vendorizado**: `go install
  golang.org/x/vuln/cmd/govulncheck@latest` en el runner; se corre desde `cli/` (donde vive el
  `go.mod`).
- **npm audit desde `web/`**: `npm --prefix web ci` para lockfile determinista y luego `npm
  --prefix web audit --audit-level=high`.
- **Dependabot cubre las tres superficies**: `gomod` (`/cli`), `npm` (`/web`) y `github-actions`
  (`/`), con schedule semanal — automatiza los bump PRs que hoy no existen.
- **Sin tocar dependencias actuales**: este change es solo automatización; los upgrades concretos
  quedan en los specs de bump correspondientes.

## Superficie

- `.github/workflows/security-scan.yml` (nuevo): job(s) de govulncheck + npm audit sobre
  `pull_request` y `schedule` (weekly).
- `.github/dependabot.yml` (nuevo): ecosistemas `gomod` (`/cli`), `npm` (`/web`),
  `github-actions` (`/`).

## Flujo

PR abierto o cron semanal → runner instala `govulncheck` y corre `govulncheck ./...` en `cli/`
→ `npm ci` + `npm audit --audit-level=high` en `web/` → HIGH+ en cualquiera falla el build
(Trivy advisory si se incluye). En paralelo, Dependabot abre PRs de actualización para gomod +
npm + actions según su schedule.

## Riesgos / consideraciones

- `govulncheck@latest` puede introducir ruido si sale una advisory nueva entre corridas; el
  schedule semanal lo detecta aunque no haya PRs.
- `npm audit` puede reportar transitivos sin fix disponible; `--audit-level=high` acota el ruido,
  pero puede requerir un allowlist futuro si bloquea sin remediación.
