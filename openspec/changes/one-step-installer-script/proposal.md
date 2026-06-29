# One-step installer script + per-platform release pipeline

## Why

Vector's day-0 distribution goal (`architecture/distribution-packaging.md`) is a single
`curl -fsSL <url> | sh` install with no Go toolchain on the user's machine. Today there is no
release pipeline and no installer: the only way to get the binary is to build from source. The
binary also hardcodes `const version = "0.0.1-dev"`, so a released build cannot identify its own
version. This change builds the complete distribution infrastructure — installer, GoReleaser
config, GitHub Actions release workflow, and build-time version injection.

The infrastructure is built **now** ("build now, publish later"): the anonymous public
`curl|sh` stays "armed but not live" until the user makes the repo public and adds a LICENSE —
that enablement decision is **not** a deliverable of this change.

## What changes

- **`scripts/install.sh`** (NEW) — bash 3.2+ script: detect OS/arch (`uname`, normalize
  `aarch64→arm64`, `x86_64→amd64`), resolve latest version via the GitHub Releases API (or
  `--version <tag>`), download the platform asset + `checksums.txt`, verify SHA256, install to
  `~/.local/bin/vector` (or `$VECTOR_INSTALL_DIR`) at mode `0755`. No `sudo`, no `jq`, no
  build-from-source. Flags: `--version`, `--dry-run`, `--force`; env: `VECTOR_INSTALL_DIR`,
  `DEBUG=1`.
- **`.goreleaser.yml`** (NEW) — GoReleaser v2: 4 targets (darwin/linux × amd64/arm64),
  `CGO_ENABLED=0`, `ldflags "-s -w -X main.version={{.Version}}"`, `tar.gz` archives with
  `name_template "vector_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`, SHA256 `checksums.txt`, GitHub
  release to owner `mcampbellr`. No Homebrew/Docker/snapcraft, no Windows, no GPG.
- **`.github/workflows/release.yml`** (NEW) — triggered on `v*` tags; strict step order:
  checkout (`fetch-depth: 0`) → setup-go (`go-version-file: cli/go.mod`) → setup-node → web
  build (`npm ci && npm run build`) → copy `web/dist/` into `cli/internal/webui/dist/` →
  `go -C cli generate ./internal/scaffold` → `go -C cli test ./...` → GoReleaser. `permissions:
  contents: write`.
- **`cli/cmd/vector/main.go`** (MODIFY) — change `const version = "0.0.1-dev"` to
  `var version = "dev"` (verify line before editing) so ldflags can inject the tag version at
  build time. No other change.
- **`docs/install.md`** (NEW) — Spanish install docs: requirements, install command with an
  explicit note that anonymous `curl|sh` only works once the repo is public, flags table, PATH
  guidance, post-install verification, cross-reference to `README.md`.

## Scope

- **In**: `scripts/install.sh`, `.goreleaser.yml`, `.github/workflows/release.yml`, the
  `const → var version` edit in `main.go`, `docs/install.md`, and local verification
  (`bash -n`, `goreleaser check`, ldflags injection, `go -C cli test ./...`).
- **Out**: Windows installer (abort with a clear message), build-from-source, making the repo
  public or adding `LICENSE`, editing `README.md` (separate change
  `rewrite-public-readme-humanized`), Homebrew tap / npm shim / other channels, GPG signing,
  an uninstall script, a PR/CI lint workflow, and pushing tags or triggering the pipeline as
  part of implementing this change.

Authored spec: `.vector/specs/one-step-installer-script/spec.md`.
