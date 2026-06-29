#!/usr/bin/env bash
#
# Vector one-step installer.
#
#   curl -fsSL <URL>/install.sh | sh
#
# Detects the platform, resolves the latest release (or a pinned --version),
# downloads the prebuilt binary + checksums.txt from GitHub Releases, verifies
# the SHA256, and installs to ~/.local/bin (or $VECTOR_INSTALL_DIR). No Go
# toolchain, no sudo, no jq. bash 3.2+ compatible (macOS default).
#
# Flags:  --version <tag>   install a specific tag instead of latest
#         --dry-run         print every step without downloading or installing
#         --force           reinstall even if the same version is already present
# Env:    VECTOR_INSTALL_DIR   install target (default: $HOME/.local/bin)
#         GITHUB_TOKEN         optional bearer token for authenticated download
#                              (needed while the repo is private)
#         DEBUG=1              enable `set -x` trace
#
# While the repo is private, anonymous requests return 404/403 — that is the
# expected behavior until the repo is made public ("build now, publish later").

set -euo pipefail

if [ "${DEBUG:-}" = "1" ]; then
  set -x
fi

# --- constants ----------------------------------------------------------------

REPO_OWNER="mcampbellr"
REPO_NAME="vector"
API_LATEST="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
DOWNLOAD_BASE="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download"

# --- output helpers -----------------------------------------------------------

info() { printf '==> %s\n' "$1"; }
warn() { printf 'Warning: %s\n' "$1" >&2; }
err() {
  printf 'Error: %s\n' "$1" >&2
  exit 1
}

# --- cleanup ------------------------------------------------------------------

TMPDIR_VECTOR=""
cleanup() {
  if [ -n "$TMPDIR_VECTOR" ] && [ -d "$TMPDIR_VECTOR" ]; then
    rm -rf "$TMPDIR_VECTOR"
  fi
}
trap cleanup EXIT

# --- flags --------------------------------------------------------------------

VERSION_TAG=""
DRY_RUN=0
FORCE=0

while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      shift
      if [ $# -eq 0 ]; then err "--version requires a tag argument (e.g. --version v0.1.0)."; fi
      VERSION_TAG="$1"
      ;;
    --version=*)
      VERSION_TAG="${1#--version=}"
      ;;
    --dry-run)
      DRY_RUN=1
      ;;
    --force)
      FORCE=1
      ;;
    -h | --help)
      printf 'Usage: install.sh [--version <tag>] [--dry-run] [--force]\n'
      exit 0
      ;;
    *)
      err "Unknown argument: $1"
      ;;
  esac
  shift
done

dry() {
  # Echo an action line, honoring dry-run. Returns 0 if the caller should skip
  # the real operation (i.e. we are in dry-run mode).
  if [ "$DRY_RUN" -eq 1 ]; then
    printf '[dry-run] %s\n' "$1"
    return 0
  fi
  return 1
}

# --- platform detection -------------------------------------------------------

detect_platform() {
  uname_s="$(uname -s)"
  case "$uname_s" in
    Darwin) OS="darwin" ;;
    Linux) OS="linux" ;;
    *) err "Windows is not supported in V1. Only macOS (darwin) and Linux are supported." ;;
  esac

  uname_m="$(uname -m)"
  case "$uname_m" in
    x86_64 | amd64) ARCH="amd64" ;;
    arm64 | aarch64) ARCH="arm64" ;;
    *) err "Unsupported architecture: ${uname_m}. Supported: amd64 (x86_64), arm64 (aarch64)." ;;
  esac

  info "Detected: ${OS} ${ARCH}"
}

# --- http helpers -------------------------------------------------------------

# Selects an available downloader. curl preferred (rich status handling); wget
# is a best-effort fallback.
select_downloader() {
  if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
  elif command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
  else
    err "Neither curl nor wget is available. Install one and retry."
  fi
}

# fetch_to_file URL OUTFILE
# Downloads URL into OUTFILE. Sets HTTP_CODE (curl only) and maps failures to
# the spec's actionable messages, then aborts. Returns on success.
fetch_to_file() {
  fetch_url="$1"
  fetch_out="$2"

  if [ "$DOWNLOADER" = "curl" ]; then
    set +e
    if [ -n "${GITHUB_TOKEN:-}" ]; then
      HTTP_CODE="$(curl -sSL -w '%{http_code}' \
        --connect-timeout 10 --max-time 300 --proto '=https' \
        -H "Authorization: Bearer ${GITHUB_TOKEN}" \
        -o "$fetch_out" "$fetch_url" 2>/dev/null)"
    else
      HTTP_CODE="$(curl -sSL -w '%{http_code}' \
        --connect-timeout 10 --max-time 300 --proto '=https' \
        -o "$fetch_out" "$fetch_url" 2>/dev/null)"
    fi
    curl_exit=$?
    set -e

    if [ "$curl_exit" -ne 0 ]; then
      case "$curl_exit" in
        28) err "Connection to GitHub timed out. Try again later." ;;
        6 | 7) err "Failed to reach GitHub. Check your network connection and try again." ;;
        *) err "Failed to reach GitHub. Check your network connection and try again." ;;
      esac
    fi
    classify_http "$HTTP_CODE"
  else
    # wget fallback: best-effort, coarse error mapping.
    set +e
    if [ -n "${GITHUB_TOKEN:-}" ]; then
      wget -q --timeout=300 --header="Authorization: Bearer ${GITHUB_TOKEN}" \
        -O "$fetch_out" "$fetch_url"
    else
      wget -q --timeout=300 -O "$fetch_out" "$fetch_url"
    fi
    wget_exit=$?
    set -e
    if [ "$wget_exit" -ne 0 ]; then
      HTTP_CODE="000"
    else
      HTTP_CODE="200"
    fi
  fi
}

# classify_http CODE — aborts with the right message for non-2xx HTTP codes.
# Callers that need to special-case 404 (asset/checksums) check HTTP_CODE first.
classify_http() {
  code="$1"
  case "$code" in
    2*) return 0 ;;
    403) err "GitHub API rate limit hit. Try again later or use --version <tag>." ;;
    404) return 0 ;; # caller decides the 404 message (api vs asset vs checksums)
    5*) err "GitHub returned a server error (${code}). Try again later or use --version <tag>." ;;
    *) err "GitHub returned an unexpected response (${code}). Try again later or use --version <tag>." ;;
  esac
}

# --- version resolution -------------------------------------------------------

resolve_version() {
  if [ -n "$VERSION_TAG" ]; then
    TAG="$VERSION_TAG"
    info "Using pinned version: ${TAG}"
    return 0
  fi

  info "Resolving latest version..."
  meta_file="${TMPDIR_VECTOR}/release.json"
  fetch_to_file "$API_LATEST" "$meta_file"

  if [ "$HTTP_CODE" = "404" ]; then
    err "Could not resolve latest version. If the repo is private, it may not be publicly accessible yet."
  fi

  # Extract tag_name without jq: first match of "tag_name": "..."
  TAG="$(grep '"tag_name"' "$meta_file" | head -n 1 | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
  if [ -z "$TAG" ]; then
    err "Could not resolve latest version from GitHub API."
  fi
  info "Latest version: ${TAG}"
}

# --- download + verify --------------------------------------------------------

download_and_verify() {
  VERSION="${TAG#v}"
  ASSET="vector_${VERSION}_${OS}_${ARCH}.tar.gz"
  ASSET_URL="${DOWNLOAD_BASE}/${TAG}/${ASSET}"
  CHECKSUMS_URL="${DOWNLOAD_BASE}/${TAG}/checksums.txt"

  if dry "Would download ${ASSET} from ${ASSET_URL}"; then
    dry "Would download checksums.txt from ${CHECKSUMS_URL}"
    dry "Would verify checksum for ${ASSET}"
    return 0
  fi

  info "Downloading ${ASSET}..."
  asset_path="${TMPDIR_VECTOR}/${ASSET}"
  fetch_to_file "$ASSET_URL" "$asset_path"
  if [ "$HTTP_CODE" = "404" ]; then
    err "No prebuilt binary found for ${OS}/${ARCH} in release ${TAG}."
  fi

  info "Downloading checksums.txt..."
  checksums_path="${TMPDIR_VECTOR}/checksums.txt"
  fetch_to_file "$CHECKSUMS_URL" "$checksums_path"
  if [ "$HTTP_CODE" = "404" ]; then
    err "Could not download checksums.txt. Cannot verify integrity."
  fi

  info "Verifying checksum..."
  verify_checksum "$asset_path" "$checksums_path" "$ASSET"
  info "Checksum OK"
}

verify_checksum() {
  file_path="$1"
  checksums="$2"
  filename="$3"

  expected_line="$(grep " ${filename}\$" "$checksums" | head -n 1)"
  if [ -z "$expected_line" ]; then
    err "Checksum verification failed for ${filename}. The download may be corrupt. Try again."
  fi

  # Run the platform checksum tool from within the temp dir so the basename in
  # checksums.txt resolves. checksums.txt format: "<sha256>  <filename>".
  check_line_file="${TMPDIR_VECTOR}/check.txt"
  printf '%s\n' "$expected_line" >"$check_line_file"

  if [ "$OS" = "darwin" ]; then
    if ! ( cd "$TMPDIR_VECTOR" && shasum -a 256 --check --status "check.txt" ); then
      err "Checksum verification failed for ${filename}. The download may be corrupt. Try again."
    fi
  else
    if ! ( cd "$TMPDIR_VECTOR" && sha256sum --check --status "check.txt" ); then
      err "Checksum verification failed for ${filename}. The download may be corrupt. Try again."
    fi
  fi
}

# --- install ------------------------------------------------------------------

prepare_install_dir() {
  INSTALL_DIR="${VECTOR_INSTALL_DIR:-$HOME/.local/bin}"

  if [ -e "$INSTALL_DIR" ] && [ ! -d "$INSTALL_DIR" ]; then
    err "VECTOR_INSTALL_DIR (${INSTALL_DIR}) is a file, not a directory."
  fi

  if dry "Would create ${INSTALL_DIR} and install vector there"; then
    return 0
  fi

  mkdir -p "$INSTALL_DIR"
  if [ ! -w "$INSTALL_DIR" ]; then
    err "No write permission in ${INSTALL_DIR}. Set VECTOR_INSTALL_DIR to a writable path."
  fi
}

install_binary() {
  if [ "$DRY_RUN" -eq 1 ]; then
    dry "Would extract and install vector to ${INSTALL_DIR}/vector (mode 0755)"
    return 0
  fi

  info "Installing vector to ${INSTALL_DIR}..."
  ( cd "$TMPDIR_VECTOR" && tar -xzf "$ASSET" )
  if [ ! -f "${TMPDIR_VECTOR}/vector" ]; then
    err "Archive ${ASSET} did not contain a 'vector' binary."
  fi
  install -m 0755 "${TMPDIR_VECTOR}/vector" "${INSTALL_DIR}/vector"
}

post_install() {
  if [ "$DRY_RUN" -eq 1 ]; then
    dry "Would verify ${INSTALL_DIR}/vector version"
    return 0
  fi

  installed_version="$("${INSTALL_DIR}/vector" version 2>/dev/null | awk '{print $NF}')"
  if [ "$installed_version" = "dev" ]; then
    warn "installed binary reports version 'dev'. This may indicate a local build, not a release binary."
  fi

  info "vector ${installed_version} installed successfully"

  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
        printf 'Add ~/.local/bin to your PATH: export PATH="$HOME/.local/bin:$PATH"\n'
      else
        printf 'Add %s to your PATH: export PATH="%s:$PATH"\n' "$INSTALL_DIR" "$INSTALL_DIR"
      fi
      ;;
  esac
}

# --- already-installed short-circuit ------------------------------------------

maybe_skip_if_present() {
  if [ "$FORCE" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then
    return 0
  fi
  candidate="${INSTALL_DIR}/vector"
  if [ -x "$candidate" ]; then
    current="$("$candidate" version 2>/dev/null | awk '{print $NF}')"
    if [ "$current" = "$TAG" ] || [ "$current" = "${TAG#v}" ] || [ "v$current" = "$TAG" ]; then
      info "vector ${current} is already installed (use --force to reinstall)"
      exit 0
    fi
  fi
}

# --- main ---------------------------------------------------------------------

main() {
  TMPDIR_VECTOR="$(mktemp -d 2>/dev/null || mktemp -d -t vector-install)"
  select_downloader
  detect_platform
  resolve_version
  prepare_install_dir
  maybe_skip_if_present
  download_and_verify
  install_binary
  post_install
}

main
