# Windows support in GoReleaser + PowerShell installer

## Why

Vector's day-0 distribution goal (`architecture/distribution-packaging.md`) is a single-step
install with no extra toolchain on the user's machine. Today that path exists only for macOS and
Linux: `.goreleaser.yml` declares `goos: [darwin, linux]` (lines 30-32) and its header comment
(line 4) says explicitly "no Windows"; `scripts/install.sh` aborts at line 105 with
`"Windows is not supported in V1."`. A developer on Windows has **no** install path.

This change closes that gap: extend the GoReleaser pipeline to produce `windows/amd64` and
`windows/arm64` binaries packaged as `.zip`, and add `scripts/install.ps1` — a PowerShell 5.1+
installer that mirrors `install.sh` idiomatically — so a Windows user installs Vector with one
PowerShell one-liner and gets the same compiled, checksum-verified binary as Unix users. The Go
source is untouched (`CGO_ENABLED=0` already cross-compiles to Windows); the Unix pipeline and
`install.sh` are untouched.

## What changes

- **`.goreleaser.yml`** (MODIFY) — add `windows` to `builds.goos` (cross-compiled with the
  existing `CGO_ENABLED=0`); split the single `archives` block into two `goos`-discriminated
  sections (id `unix`: `[tar.gz]` for `[darwin, linux]`; id `windows`: `[zip]` for `[windows]`),
  keeping `name_template` invariant; add `release.extra_files` publishing `scripts/install.ps1`
  as a release asset (needed for the `releases/latest/download/install.ps1` URL); update the
  header comment (line 2: 4→6 binaries; line 4: drop "no Windows").
- **`scripts/install.ps1`** (NEW) — PowerShell 5.1+ installer with feature parity to
  `install.sh`: PS-version check, arch detection via `$env:PROCESSOR_ARCHITECTURE`
  (`AMD64`→`amd64`, `ARM64`→`arm64`, abort otherwise), version resolution via the GitHub
  Releases API (`ConvertFrom-Json` in `try/catch`), download with `-UseBasicParsing
  -TimeoutSec 300`, mandatory SHA256 verification with `Get-FileHash`, `.zip` extraction via
  `Expand-Archive`, install to `$env:LOCALAPPDATA\Programs\Vector` (or `$env:VECTOR_INSTALL_DIR`),
  PATH hint (no automatic PATH mutation), temp-dir cleanup in `finally`. Flags: `--version`,
  `--dry-run`, `--force`. Env: `VECTOR_INSTALL_DIR`, `GITHUB_TOKEN`, `DEBUG`. Only native PS 5.1
  cmdlets; messages in English; no ANSI/color.
- **`README.md`** (MODIFY) — add a `### Windows` subsection under `## Installation`: the
  `irm | iex` one-liner (latest only), the two-step method (for inspection and for `--version`,
  which the PS 5.1 one-liner cannot forward), the default install dir
  `%LOCALAPPDATA%\Programs\Vector\vector.exe`, and a PATH hint; update the supported-platforms
  line (line 55) to include Windows.
- **`.github/workflows/release.yml`** (REVIEW) — confirm no functional change is needed
  (GoReleaser cross-compiles Windows on `ubuntu-latest` with `CGO_ENABLED=0`); update the cosmetic
  header comment (line 5: 4→6 archives).

## Scope

- **In**: the `.goreleaser.yml` edits (goos, two `archives` sections, `extra_files`, header
  comment), `scripts/install.ps1`, the `README.md` Windows subsection + platform line, the
  cosmetic `release.yml` comment update, and local verification (`goreleaser check`,
  `GOOS=windows GOARCH=amd64/arm64 CGO_ENABLED=0` cross-compile, `go -C cli test ./...`).
- **Out**: winget / Chocolatey / Scoop / any package manager (V2 roadmap); Windows 7 and
  PowerShell < 5.1; Windows code signing (SmartScreen); `--version` tag-format validation
  (pass-through); any change to `scripts/install.sh` (its line-105 Windows error stays correct);
  installer message localization; any change to the Go CLI logic, board state, or web components.

Authored spec: `.vector/specs/add-windows-support-distribution/spec.md`.
