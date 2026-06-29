# Design — one-step-installer-script

## Decisiones clave

- **Tag-triggered release pipeline + installer que consume la GitHub Releases API**: el
  pipeline corre en GitHub Actions orquestado por GoReleaser; el installer es un script bash
  stateless que resuelve versión, descarga el asset de su plataforma, verifica SHA256 e instala
  localmente sin elevar privilegios. (§5 del spec.)
- **Build now, publish later**: toda la infra se construye ahora; el `curl|sh` anónimo público
  solo funciona cuando el repo sea público + tenga LICENSE. Eso es decisión del usuario, no un
  entregable. Mientras el repo sea privado, las requests anónimas devuelven `404`/`403` — y eso
  es el comportamiento correcto, validable solo con un download autenticado.
- **Binarios precompilados, sin compilación desde fuentes**: el installer no requiere Go en la
  máquina del usuario (objetivo día-0 de `distribution-packaging.md`).
- **Naming de assets compartido como contrato**: `vector_<VERSION>_<OS>_<ARCH>.tar.gz` con
  `<VERSION>` = tag sin `v`. El `name_template` de `.goreleaser.yml` y el string que construye
  `install.sh` deben ser **idénticos**; se valida manualmente (`goreleaser release --snapshot`
  vs. la URL que arma el script).
- **Versión inyectable via ldflags**: `const → var version` en `main.go`; fallback `"dev"` para
  builds locales; GoReleaser inyecta `-X main.version={{.Version}}`. `kitVersion` del config es
  un campo de estado del board, independiente — no se toca.
- **SHA256 sin GPG en V1**: `checksums.txt` (formato `<sha256>  <file>`, compatible con
  `shasum -a 256 --check` en Darwin y `sha256sum --check` en Linux), verificado **antes** de
  instalar. Sin GPG (keyserver/trust chain fuera de scope V1).
- **Bash 3.2+ compatible**: sin `declare -A`, sin `mapfile`, sin GNU-isms de `sed`/`grep`
  (macOS trae bash 3.2 por defecto). Sin `jq`, sin `sudo`, sin edición de `.bashrc`/`.zshrc`.
- **Identidad Go module ≠ GitHub owner**: el módulo Go es `github.com/mariocampbell/vector`; el
  remote de GitHub es `mcampbellr/vector`. GoReleaser usa el owner del remote (`mcampbellr`); el
  módulo no se cambia. Verificar `git remote -v` antes de fijar `release.github.owner`.
- **Orden de build obligatorio**: web build → copy dist → `go generate` → `go test` →
  GoReleaser. Invertirlo publica un binario con assets de web desactualizados o rompe
  `TestAssetsMatchKit`. El workflow lleva el orden; los `before.hooks` de GoReleaser lo repiten
  como cinturón de seguridad.

## Superficie

- `scripts/install.sh` (NUEVO): detección plataforma, resolución de versión, descarga +
  verificación SHA256, instalación 0755, sugerencia de PATH, manejo de edge cases de red
  (`--connect-timeout 10 --max-time 300`, transport/timeout/5xx/404/403), trap `EXIT` que limpia
  `mktemp -d`. Strings de usuario en inglés (§16 del spec).
- `.goreleaser.yml` (NUEVO): `version: 2`, `project_name: vector`, `before.hooks`, `builds`
  (`dir: cli`, `main: ./cmd/vector`, 4 targets, ldflags, `CGO_ENABLED=0`), `archives`
  (`tar.gz`, `name_template`), `checksum` (sha256), `release` (`github.owner: mcampbellr`).
- `.github/workflows/release.yml` (NUEVO): `on: push: tags: ['v*']`, `permissions: contents:
  write`, un job `release` en `ubuntu-latest` con los steps en orden estricto.
- `cli/cmd/vector/main.go` (MODIFICAR): `const version = "0.0.1-dev"` → `var version = "dev"`
  (verificar línea ~26 antes de editar); ningún otro cambio.
- `docs/install.md` (NUEVO): documentación en español (requisitos, instalación con nota de repo
  privado, tabla de flags, verificación, PATH, cross-ref a README).

## Open questions

1. **Repo público + LICENSE**: requisitos de habilitación del `curl|sh` anónimo. Decisión del
   usuario (cuándo, qué licencia). Bloquea el criterio de éxito del install público.
2. **URL estable de `install.sh`**: raw GitHub URL directa vs. dominio con redirect; se decide
   al hacer el repo público.
3. **Primer tag de release**: `v0.1.0` vs `v0.0.1` (alineación con `kitVersion`); lo elige el
   usuario al activar el pipeline.
4. **Pin del GoReleaser Action**: recomendación `@v6` (no `latest`) para builds reproducibles;
   versión exacta al implementar. Decisión menor, no bloqueante.
