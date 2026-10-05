# Vector — canonical from-source install path.
#
# `make install` is the ONLY sanctioned way to (re)build and reinstall the vector
# binary from source. It ALWAYS rebuilds web/, re-embeds the result, and runs the
# build-time integrity guard BEFORE compiling — so a broken embed aborts the
# install instead of overwriting a working binary with a blank board.
#
# This is NOT the end-user release installer: scripts/install.sh downloads a
# prebuilt release binary from GitHub. This path builds from the working tree and
# needs a Go toolchain + Node/npm. scripts/dev-install.sh is the make-less twin.
#
#   make install                                   # web build -> embed -> guard -> build -> install
#   VECTOR_INSTALL_DIR=/tmp/vector make install    # install elsewhere
#   make guard                                     # just run the embed-integrity guard
#
# VECTOR_INSTALL_DIR is the full path to the installed binary (not a directory).
VECTOR_INSTALL_DIR ?= $(HOME)/.local/bin/vector

.PHONY: install build guard embed web-build

# Rebuild the web board (Vite). Fails loud if node/npm are missing — the install
# never falls back to a stale embed.
web-build:
	npm --prefix web run build

# Re-embed the freshly built web/dist into the Go embed dir. Remove the real
# (gitignored) index.html + assets first, then copy the new build on top. The
# tracked index.placeholder.html is left untouched (web build never produces it).
embed: web-build
	rm -rf cli/internal/webui/dist/assets cli/internal/webui/dist/index.html
	cp -R web/dist/. cli/internal/webui/dist/

# Build-time integrity guard: fails if the just-embedded index.html references
# /assets/* absent from the embed. A non-zero exit here aborts before `build`, so
# a binary with a broken board never reaches $(VECTOR_INSTALL_DIR).
guard: embed
	go -C cli test ./internal/webui/... -run TestEmbeddedBoardIntegrity -v

# Compile the binary only after the guard passes.
build: guard
	go -C cli build -o "$(VECTOR_INSTALL_DIR)" ./cmd/vector

install: build
	@echo "Installed vector to $(VECTOR_INSTALL_DIR)"
	@echo "Restart any running 'vector serve' to pick up the new binary."
