# Tasks — one-step-installer-script

## 1. Version injection (cli)

- [x] 1.1 Verify the exact line of `const version = "0.0.1-dev"` in `cli/cmd/vector/main.go` (currently ~line 26).
- [x] 1.2 Change `const version = "0.0.1-dev"` to `var version = "dev"`; no other edits to `main.go`.
- [x] 1.3 Verify ldflags injection: `go -C cli build -ldflags "-X main.version=v0.1.0-test" -o /tmp/vector-test ./cmd/vector && /tmp/vector-test version` → `v0.1.0-test`.
- [x] 1.4 Verify fallback: `go -C cli build -o /tmp/vector-dev ./cmd/vector && /tmp/vector-dev version` → `dev`.

## 2. GoReleaser config

- [x] 2.1 `.goreleaser.yml` v2: `project_name: vector`, `before.hooks` (web build + copy dist + `go generate`), `builds` (`dir: cli`, `main: ./cmd/vector`, darwin/linux × amd64/arm64, `CGO_ENABLED=0`, `ldflags "-s -w -X main.version={{.Version}}"`).
- [x] 2.2 `archives` `tar.gz` with `name_template "vector_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`; `checksum` sha256 → `checksums.txt`; `release.github.owner: mcampbellr`, `name: vector`, `prerelease: auto`.
- [ ] 2.3 `goreleaser check` passes; optionally `goreleaser release --snapshot --clean` builds the 4 binaries locally.

## 3. Release workflow

- [x] 3.1 `.github/workflows/release.yml`: `on: push: tags: ['v*']`, `permissions: contents: write`, single `release` job on `ubuntu-latest`.
- [x] 3.2 Steps in strict order: checkout (`fetch-depth: 0`) → setup-go (`go-version-file: cli/go.mod`) → setup-node → web build (`npm ci && npm run build`) → copy `web/dist/*` to `cli/internal/webui/dist/` → `go -C cli generate ./internal/scaffold` → `go -C cli test ./...` → `goreleaser/goreleaser-action@v6` (`args: release --clean`, `GITHUB_TOKEN`).

## 4. Installer script

- [x] 4.1 `scripts/install.sh`: shebang + `set -euo pipefail`, `DEBUG=1`→`set -x`, trap `EXIT` cleaning `mktemp -d`; OS/arch detection + normalization; flag parse (`--version`/`--dry-run`/`--force`); `INSTALL_DIR` default `~/.local/bin` / `$VECTOR_INSTALL_DIR`.
- [x] 4.2 Version resolution via GitHub Releases API (`grep`/`sed`, no `jq`) or `--version`; asset name `vector_<ver>_<os>_<arch>.tar.gz` matching the GoReleaser `name_template`.
- [x] 4.3 Download asset + `checksums.txt` with `--connect-timeout 10 --max-time 300 --proto '=https'`; SHA256 verify before install; install `0755`; `[ -w "$INSTALL_DIR" ]` check; PATH suggestion; final `vector version` check (`dev` → warning, not error).
- [x] 4.4 Edge cases: unsupported OS (incl. Windows) / arch abort with exact messages; 404/403/5xx/transport/timeout handled with actionable stderr messages (§11, §16 of the spec).
- [x] 4.5 `bash -n scripts/install.sh` exits 0; bash 3.2 compatibility (no `declare -A`, no GNU-isms, no `sudo`).

## 5. Docs

- [x] 5.1 `docs/install.md` (Spanish): requirements, install command + explicit private-repo note, flags table (`--version`/`--dry-run`/`--force` + `VECTOR_INSTALL_DIR`), post-install verification, PATH guidance, cross-reference to `README.md`.

## 6. Verification

- [x] 6.1 `go -C cli test ./...` green (incl. `TestAssetsMatchKit`) after the `const → var` change.
- [x] 6.2 Manually confirm the archive `name_template` and the string `install.sh` builds are identical.
- [x] 6.3 Do NOT push tags, create GitHub Releases, edit `README.md`, create `LICENSE`, edit `scaffold/assets/` by hand, or touch `kitVersion`/`.vector/` state.
