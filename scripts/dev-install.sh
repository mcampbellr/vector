#!/usr/bin/env bash
#
# Vector from-source dev installer (the make-less twin of `make install`).
#
#   scripts/dev-install.sh
#
# Builds web/, re-embeds it into the Go embed dir, runs the build-time integrity
# guard, and only then compiles + installs the binary. The guard aborts BEFORE
# overwriting the installed binary if the embed would be broken (blank board), so
# a reinstall from a worktree without a fresh web build can never silently ship a
# broken board.
#
# NOT to be confused with scripts/install.sh: that one downloads a prebuilt
# RELEASE binary from GitHub (no toolchain needed). THIS script builds from the
# working tree and requires a Go toolchain + Node/npm. Same VECTOR_INSTALL_DIR
# contract, different flow.
#
# Env:  VECTOR_INSTALL_DIR   full path to the installed binary
#                            (default: $HOME/.local/bin/vector)
#       DEBUG=1              enable `set -x` trace

set -euo pipefail

if [ "${DEBUG:-}" = "1" ]; then
  set -x
fi

VECTOR_INSTALL_DIR="${VECTOR_INSTALL_DIR:-$HOME/.local/bin/vector}"

# Run from the repo root regardless of the caller's CWD.
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

info() { printf '==> %s\n' "$1"; }
err() { printf 'Error: %s\n' "$1" >&2; }

# --- preflight: toolchain check (fail before touching anything) ---------------

for tool in node npm go; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    err "'$tool' is required to build vector from source but was not found on PATH."
    err "Install it, or use scripts/install.sh to download a prebuilt release instead."
    exit 1
  fi
done

# --- 1. build web -------------------------------------------------------------

info "Building web board (npm --prefix web run build)"
npm --prefix web run build

# --- 2. re-embed --------------------------------------------------------------

info "Re-embedding web/dist into cli/internal/webui/dist"
rm -rf cli/internal/webui/dist/assets cli/internal/webui/dist/index.html
cp -R web/dist/. cli/internal/webui/dist/

# --- 3. build-time integrity guard (aborts before install on failure) ---------

info "Running embed integrity guard (TestEmbeddedBoardIntegrity)"
go -C cli test ./internal/webui/... -run TestEmbeddedBoardIntegrity -v

# --- 4. compile + install -----------------------------------------------------

mkdir -p "$(dirname "$VECTOR_INSTALL_DIR")"
info "Building binary -> $VECTOR_INSTALL_DIR"
go -C cli build -o "$VECTOR_INSTALL_DIR" ./cmd/vector

info "Installed vector to $VECTOR_INSTALL_DIR"
info "Restart any running 'vector serve' to pick up the new binary."
