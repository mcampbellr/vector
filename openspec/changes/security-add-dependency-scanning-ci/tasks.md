# Tasks — security-add-dependency-scanning-ci

## 1. CI workflow

- [ ] 1.1 Crear `.github/workflows/security-scan.yml` con triggers `pull_request` y `schedule`
  (semanal), permisos mínimos.
- [ ] 1.2 Job Go: `go install golang.org/x/vuln/cmd/govulncheck@latest` + `govulncheck ./...`
  desde `cli/`; falla en HIGH+.
- [ ] 1.3 Job npm: `npm --prefix web ci` + `npm --prefix web audit --audit-level=high`; falla en
  HIGH+.
- [ ] 1.4 (Opcional) step Trivy filesystem scan advisory (`continue-on-error: true`).

## 2. Dependabot

- [ ] 2.1 Crear `.github/dependabot.yml` con ecosistemas `gomod` (`/cli`), `npm` (`/web`) y
  `github-actions` (`/`), schedule semanal.

## 3. Verificación

- [ ] 3.1 `govulncheck ./...` corre limpio localmente en `cli/`.
- [ ] 3.2 CI corre govulncheck y npm audit en cada PR y bloquea en findings HIGH+.
- [ ] 3.3 Dependabot abre update PRs para gomod + npm + actions.
