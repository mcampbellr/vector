# Tasks — add-windows-support-distribution

## 1. GoReleaser config

- [x] 1.1 Add `windows` to `builds.goos` in `.goreleaser.yml` (currently lines 30-32), keeping `CGO_ENABLED=0` (line 27).
- [x] 1.2 Discriminate the archive format by `goos`, keeping `name_template: "vector_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`. **Implemented via `format_overrides` (goos: windows → `formats: [zip]`) on the single `vector` archive, not two `goos`-keyed sections: the GoReleaser v2 `config.Archive` schema has no top-level `goos` field, so `goreleaser check` rejects the two-section form. `format_overrides` is the canonical v2 mechanism and preserves the intent (tar.gz on Unix, zip on Windows, no overlap/gap).**
- [x] 1.3 Add `release.extra_files` with `- glob: scripts/install.ps1` so the installer is published as a release asset.
- [x] 1.4 Update the header comment: line 2 (4→6 binaries, `darwin/linux/windows × amd64/arm64`) and line 4 (drop "no Windows").
- [x] 1.5 Do NOT change `before.hooks`, `ldflags`, `snapshot`, `checksum.algorithm`, `builds.binary`, or the `name_template`; do NOT add Homebrew/Docker/snapcraft.
- [x] 1.6 `goreleaser check` passes with no errors or warnings; the two `archives` sections cover the three `goos` with no overlap/gap.

## 2. PowerShell installer

- [x] 2.1 `scripts/install.ps1`: verify `$PSVersionTable.PSVersion` ≥ 5.1 as the FIRST operation, aborting with `"PowerShell 5.1+ required."`; `DEBUG=1` → `Set-PSDebug -Trace 1`.
- [x] 2.2 Parse flags `--version <tag>` / `--dry-run` / `--force` (abort if `--version` has no argument); helpers `Write-Info` (`==>`), `Write-Err` (`Error:`), `Invoke-Dry` (`[dry-run]`).
- [x] 2.3 Arch detection via `$env:PROCESSOR_ARCHITECTURE`: `AMD64`→`amd64`, `ARM64`→`arm64`; abort with an actionable message on any other value (never assume `amd64`).
- [x] 2.4 Version resolution: `--version` pass-through, else GitHub Releases API (`.../mcampbellr/vector/releases/latest`) parsed with `ConvertFrom-Json` inside `try/catch`; add `Authorization: Bearer $env:GITHUB_TOKEN` when present. Asset name `vector_$($TAG.TrimStart('v'))_windows_$ARCH.zip`.
- [x] 2.5 Install dir: `$env:VECTOR_INSTALL_DIR` or `$env:LOCALAPPDATA\Programs\Vector`; create with `New-Item -Force`; verify write permission; short-circuit if same version already installed unless `--force`.
- [x] 2.6 Download asset + `checksums.txt` with `Invoke-WebRequest -UseBasicParsing -TimeoutSec 300`; map HTTP 401/403/404/429/5xx to distinct actionable messages.
- [x] 2.7 Mandatory SHA256 verify with `Get-FileHash -Algorithm SHA256` against the matching `checksums.txt` line; abort before copying if it fails or the filename is absent.
- [x] 2.8 `Expand-Archive -Force` to the temp dir; verify `vector.exe` exists (abort otherwise); `Copy-Item` to the install dir; print success with the installed version.
- [x] 2.9 PATH hint if the install dir is not on `$env:PATH` (no automatic PATH mutation); temp-dir cleanup with `Remove-Item -Recurse -Force` in a `finally` block.
- [x] 2.10 Only native PS 5.1 cmdlets; English messages; no ANSI/color; `GITHUB_TOKEN` never logged (even with `DEBUG=1`); HTTPS only.

## 3. Docs

- [x] 3.1 `README.md` `## Installation`: add `### Windows` with the `irm | iex` one-liner (latest only), the two-step method (for inspection and for `--version` with an explanatory note), the default dir `%LOCALAPPDATA%\Programs\Vector\vector.exe`, and a PATH hint.
- [x] 3.2 Update the supported-platforms line (README line 55) to include Windows; do NOT document winget or change the Unix install commands.

## 4. CI review

- [x] 4.1 Confirm `.github/workflows/release.yml` needs no functional change (GoReleaser cross-compiles `windows/amd64` and `windows/arm64` on `ubuntu-latest` with `CGO_ENABLED=0`; Go 1.26 via `go-version-file: cli/go.mod`).
- [x] 4.2 Update the cosmetic header comment (release.yml line 5: 4→6 archives).

## 5. Verification

- [x] 5.1 `goreleaser check` passes on the modified `.goreleaser.yml`.
- [x] 5.2 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go -C cli build -o /dev/null ./cmd/vector` — no errors.
- [x] 5.3 `GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go -C cli build -o /dev/null ./cmd/vector` — no errors.
- [x] 5.4 `go -C cli test ./...` stays green (no Go source change).
- [x] 5.5 (Optional, needs Node for the web build) `goreleaser release --snapshot --clean` produces the 6 archives with no config errors.
- [x] 5.6 Do NOT modify `scripts/install.sh`, add code signing, add package managers, change `name_template`/`checksum.algorithm`/`builds.binary`, or touch `.vector/` state.
