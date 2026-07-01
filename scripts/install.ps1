<#
.SYNOPSIS
  Vector one-step installer for Windows (PowerShell 5.1+).

.DESCRIPTION
  Idiomatic PowerShell mirror of scripts/install.sh. Detects the architecture,
  resolves the latest release (or a pinned --version), downloads the prebuilt
  .zip + checksums.txt from GitHub Releases, verifies the SHA256, and installs
  vector.exe to %LOCALAPPDATA%\Programs\Vector (or $env:VECTOR_INSTALL_DIR).
  No Go toolchain, no admin elevation, no external dependencies.

  Latest (one-liner):
    irm https://raw.githubusercontent.com/mcampbellr/vector/main/scripts/install.ps1 | iex

  Pinned version / inspection (two-step — PS 5.1 one-liners cannot forward args):
    irm https://raw.githubusercontent.com/mcampbellr/vector/main/scripts/install.ps1 -OutFile install.ps1
    .\install.ps1 --version v0.1.0

  Flags:  --version <tag>   install a specific tag instead of latest
          --dry-run         print every step without downloading or installing
          --force           reinstall even if the same version is already present
  Env:    VECTOR_INSTALL_DIR   install target (default: %LOCALAPPDATA%\Programs\Vector)
          GITHUB_TOKEN         optional bearer token for authenticated download
                               (needed while the repo is private)
          DEBUG=1              enable Set-PSDebug -Trace 1

  While the repo is private, anonymous requests return 404/403 — that is the
  expected behavior until the repo is made public ("build now, publish later").
#>

# --- PowerShell version gate (FIRST operation) --------------------------------

if ($PSVersionTable.PSVersion -lt [Version]'5.1') {
    [Console]::Error.WriteLine('Error: PowerShell 5.1+ required.')
    exit 1
}

if ($env:DEBUG -eq '1') { Set-PSDebug -Trace 1 }

$ErrorActionPreference = 'Stop'

# --- output helpers -----------------------------------------------------------

function Write-Info { param([string]$Message) Write-Host "==> $Message" }
function Write-Warn { param([string]$Message) [Console]::Error.WriteLine("Warning: $Message") }
function Write-Err {
    param([string]$Message)
    [Console]::Error.WriteLine("Error: $Message")
    exit 1
}
function Invoke-Dry {
    param([string]$Message)
    if ($DryRun) {
        Write-Host "[dry-run] $Message"
        return $true
    }
    return $false
}

# --- flags --------------------------------------------------------------------

$VersionTag = ''
$DryRun = $false
$Force = $false

$index = 0
while ($index -lt $args.Count) {
    $argument = [string]$args[$index]
    switch -Exact ($argument) {
        '--version' {
            $index++
            if ($index -ge $args.Count) {
                Write-Err '--version requires a tag argument (e.g. --version v0.1.0).'
            }
            $VersionTag = [string]$args[$index]
        }
        '--dry-run' { $DryRun = $true }
        '--force'   { $Force = $true }
        '-h'        { Write-Host 'Usage: install.ps1 [--version <tag>] [--dry-run] [--force]'; exit 0 }
        '--help'    { Write-Host 'Usage: install.ps1 [--version <tag>] [--dry-run] [--force]'; exit 0 }
        default {
            if ($argument -like '--version=*') {
                $VersionTag = $argument.Substring('--version='.Length)
            }
            else {
                Write-Err "Unknown argument: $argument"
            }
        }
    }
    $index++
}

# --- constants ----------------------------------------------------------------

$RepoOwner = 'mcampbellr'
$RepoName = 'vector'
$ApiLatest = "https://api.github.com/repos/$RepoOwner/$RepoName/releases/latest"
$DownloadBase = "https://github.com/$RepoOwner/$RepoName/releases/download"

# --- architecture detection ---------------------------------------------------

switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $Arch = 'amd64' }
    'ARM64' { $Arch = 'arm64' }
    default {
        Write-Err "Unsupported architecture: $($env:PROCESSOR_ARCHITECTURE). Supported: AMD64, ARM64."
    }
}
$Os = 'windows'
Write-Info "Detected: $Os $Arch"

# --- http helper --------------------------------------------------------------

# Invoke-Download URL OUTFILE
# Downloads URL into OUTFILE over HTTPS. Returns an HTTP-ish status: 200 on
# success, the response status code on an HTTP error, or 0 on a network failure.
# The GITHUB_TOKEN, when present, is sent as a bearer header and never printed.
function Invoke-Download {
    param([string]$Url, [string]$OutFile)

    if ($Url -notlike 'https://*') {
        Write-Err "Refusing to download over a non-HTTPS URL: $Url"
    }

    $headers = @{}
    if ($env:GITHUB_TOKEN) { $headers['Authorization'] = "Bearer $env:GITHUB_TOKEN" }

    $previousProgress = $ProgressPreference
    $ProgressPreference = 'SilentlyContinue'
    try {
        Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing -TimeoutSec 300 -Headers $headers | Out-Null
        return 200
    }
    catch [System.Net.WebException] {
        if ($_.Exception.Response) { return [int]$_.Exception.Response.StatusCode }
        return 0
    }
    catch {
        return 0
    }
    finally {
        $ProgressPreference = $previousProgress
    }
}

# Test-HttpCode CODE — aborts with the right message for a non-2xx HTTP code.
# A 404 returns so the caller can supply a context-specific message.
function Test-HttpCode {
    param([int]$Code)
    if ($Code -ge 200 -and $Code -lt 300) { return }
    switch ($Code) {
        401 { Write-Err 'Authentication failed (401). Check your GITHUB_TOKEN.' }
        403 { Write-Err 'GitHub access denied or rate limit hit (403). Try again later or use --version <tag>.' }
        404 { return }
        429 { Write-Err 'GitHub rate limit hit (429). Try again later or use --version <tag>.' }
        0   { Write-Err 'Failed to reach GitHub. Check your network connection and try again.' }
        default {
            if ($Code -ge 500) {
                Write-Err "GitHub returned a server error ($Code). Try again later or use --version <tag>."
            }
            else {
                Write-Err "GitHub returned an unexpected response ($Code). Try again later or use --version <tag>."
            }
        }
    }
}

# --- version resolution -------------------------------------------------------

function Resolve-Version {
    if ($VersionTag) {
        Write-Info "Using pinned version: $VersionTag"
        return $VersionTag
    }

    Write-Info 'Resolving latest version...'
    $metaFile = Join-Path $TempDir 'release.json'
    $code = Invoke-Download -Url $ApiLatest -OutFile $metaFile
    if ($code -eq 404) {
        Write-Err 'Could not resolve latest version. If the repo is private, it may not be publicly accessible yet.'
    }
    Test-HttpCode -Code $code

    $tag = ''
    try {
        $meta = Get-Content -Path $metaFile -Raw | ConvertFrom-Json
        $tag = [string]$meta.tag_name
    }
    catch {
        Write-Err 'Could not parse the GitHub API response.'
    }
    if (-not $tag) { Write-Err 'Could not resolve latest version from GitHub API.' }

    Write-Info "Latest version: $tag"
    return $tag
}

# --- install directory --------------------------------------------------------

function Get-InstallDir {
    if ($env:VECTOR_INSTALL_DIR) {
        $dir = $env:VECTOR_INSTALL_DIR
    }
    else {
        $dir = Join-Path $env:LOCALAPPDATA 'Programs\Vector'
    }

    if ((Test-Path -Path $dir) -and -not (Test-Path -Path $dir -PathType Container)) {
        Write-Err "VECTOR_INSTALL_DIR ($dir) is a file, not a directory."
    }
    return $dir
}

function New-InstallDir {
    param([string]$Dir)
    if (Invoke-Dry "Would create $Dir and install vector there") { return }

    New-Item -ItemType Directory -Path $Dir -Force | Out-Null

    $probe = Join-Path $Dir '.vector-write-test'
    try {
        New-Item -ItemType File -Path $probe -Force | Out-Null
        Remove-Item -Path $probe -Force
    }
    catch {
        Write-Err "No write permission in $Dir. Set VECTOR_INSTALL_DIR to a writable path."
    }
}

# --- already-installed short-circuit ------------------------------------------

function Get-InstalledVersion {
    param([string]$Exe)
    try {
        $line = (& $Exe version 2>$null | Select-Object -First 1)
        if ($line) { return ($line -split '\s+')[-1] }
    }
    catch {}
    return ''
}

function Test-AlreadyInstalled {
    param([string]$Dir, [string]$Tag)
    if ($Force -or $DryRun) { return }

    $exe = Join-Path $Dir 'vector.exe'
    if (-not (Test-Path -Path $exe -PathType Leaf)) { return }

    $current = Get-InstalledVersion -Exe $exe
    if (-not $current) { return }

    $bare = $Tag.TrimStart('v')
    if ($current -eq $Tag -or $current -eq $bare -or "v$current" -eq $Tag) {
        Write-Info "vector $current is already installed (use --force to reinstall)"
        exit 0
    }
}

# --- download + verify --------------------------------------------------------

function Get-Asset {
    param([string]$Tag)
    $version = $Tag.TrimStart('v')
    $asset = "vector_${version}_${Os}_${Arch}.zip"
    $assetUrl = "$DownloadBase/$Tag/$asset"
    $checksumsUrl = "$DownloadBase/$Tag/checksums.txt"

    if ($DryRun) {
        Invoke-Dry "Would download $asset from $assetUrl" | Out-Null
        Invoke-Dry "Would download checksums.txt from $checksumsUrl" | Out-Null
        Invoke-Dry "Would verify checksum for $asset" | Out-Null
        return $null
    }

    Write-Info "Downloading $asset..."
    $assetPath = Join-Path $TempDir $asset
    $code = Invoke-Download -Url $assetUrl -OutFile $assetPath
    if ($code -eq 404) { Write-Err "No prebuilt binary found for $Os/$Arch in release $Tag." }
    Test-HttpCode -Code $code

    Write-Info 'Downloading checksums.txt...'
    $checksumsPath = Join-Path $TempDir 'checksums.txt'
    $code = Invoke-Download -Url $checksumsUrl -OutFile $checksumsPath
    if ($code -eq 404) { Write-Err 'Could not download checksums.txt. Cannot verify integrity.' }
    Test-HttpCode -Code $code

    Write-Info 'Verifying checksum...'
    Test-Checksum -FilePath $assetPath -ChecksumsPath $checksumsPath -FileName $asset
    Write-Info 'Checksum OK'
    return $assetPath
}

# checksums.txt format: "<sha256>  <filename>". Match the line by filename,
# then compare the SHA256 case-insensitively (Get-FileHash returns uppercase).
function Test-Checksum {
    param([string]$FilePath, [string]$ChecksumsPath, [string]$FileName)

    $expected = ''
    foreach ($line in Get-Content -Path $ChecksumsPath) {
        $parts = $line -split '\s+'
        if ($parts.Count -ge 2 -and $parts[-1] -eq $FileName) {
            $expected = $parts[0]
            break
        }
    }
    if (-not $expected) {
        Write-Err "Checksum verification failed for $FileName. The download may be corrupt. Try again."
    }

    $actual = (Get-FileHash -Path $FilePath -Algorithm SHA256).Hash
    if (-not [string]::Equals($actual, $expected, [System.StringComparison]::OrdinalIgnoreCase)) {
        Write-Err "Checksum verification failed for $FileName. The download may be corrupt. Try again."
    }
}

# --- install ------------------------------------------------------------------

function Install-Binary {
    param([string]$AssetPath, [string]$Dir)
    if ($DryRun) {
        Invoke-Dry "Would extract and install vector.exe to $Dir\vector.exe" | Out-Null
        return
    }

    Write-Info "Installing vector to $Dir..."
    $extractDir = Join-Path $TempDir 'extract'
    Expand-Archive -Path $AssetPath -DestinationPath $extractDir -Force

    $exe = Join-Path $extractDir 'vector.exe'
    if (-not (Test-Path -Path $exe -PathType Leaf)) {
        Write-Err "Archive $(Split-Path -Leaf $AssetPath) did not contain a 'vector.exe' binary."
    }
    Copy-Item -Path $exe -Destination (Join-Path $Dir 'vector.exe') -Force
}

function Complete-Install {
    param([string]$Dir)
    if ($DryRun) {
        Invoke-Dry "Would verify $Dir\vector.exe version" | Out-Null
        return
    }

    $exe = Join-Path $Dir 'vector.exe'
    $installedVersion = Get-InstalledVersion -Exe $exe
    if ($installedVersion -eq 'dev') {
        Write-Warn "installed binary reports version 'dev'. This may indicate a local build, not a release binary."
    }
    Write-Info "vector $installedVersion installed successfully"

    $pathEntries = $env:PATH -split ';'
    if ($pathEntries -notcontains $Dir) {
        Write-Host "$Dir is not on your PATH. Add it to run 'vector' from any shell:"
        Write-Host "  [Environment]::SetEnvironmentVariable('Path', `"$Dir;`" + [Environment]::GetEnvironmentVariable('Path','User'), 'User')"
    }
}

# --- main ---------------------------------------------------------------------

$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) "vector-install-$PID"
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

try {
    $tag = Resolve-Version
    $installDir = Get-InstallDir
    New-InstallDir -Dir $installDir
    Test-AlreadyInstalled -Dir $installDir -Tag $tag
    $assetPath = Get-Asset -Tag $tag
    Install-Binary -AssetPath $assetPath -Dir $installDir
    Complete-Install -Dir $installDir
}
finally {
    if (Test-Path -Path $TempDir) {
        Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
