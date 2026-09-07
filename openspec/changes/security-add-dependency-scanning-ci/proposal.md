# Add dependency/vulnerability scanning to CI

## Why

The `/security-audit` run on 2026-07-01 flagged a LOW-severity gap (two folded findings: "no CI
vuln scanning" + "govulncheck not installed"). `.github/workflows/` has no dependency-vulnerability
gate and there is no `.github/dependabot.yml`, so vulnerable Go or npm dependencies (a future
cobra/lipgloss/happy-dom advisory) can land and ship undetected, with no automated bump PRs. The
audit's own Go CVE assessment had to be done by reasoning over `go list -m all` because
`govulncheck` was not available.

## What changes

- Add a CI job (own workflow or a step in the existing one) that runs on **PR** and a **weekly
  schedule**:
  - `go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...` from `cli/`.
  - `npm --prefix web ci && npm --prefix web audit --audit-level=high`.
  - (Optional) a Trivy filesystem scan, advisory-only.
- Add `.github/dependabot.yml` covering the `cli/` gomod, the `web/` npm, and the `github-actions`
  ecosystems.
- Gating decision: **govulncheck + npm audit HIGH+ fail the build**; Trivy stays advisory.

## Scope

- **In**: the CI automation (govulncheck + npm audit gate on PR + weekly schedule) and the
  Dependabot config for gomod + npm + actions.
- **Out**: pinning/upgrading current dependencies — those are handled by the specific bump specs
  (e.g. `security-bump-happy-dom-rce`). This change adds the automation, not the individual fixes.
