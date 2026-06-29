# Spec: Instalador de un paso (curl | sh) para el CLI de Vector con pipeline de release por plataforma

## 1. Objetivo

Construir la infraestructura completa de distribución de Vector: un script `install.sh` de un
paso que detecta la plataforma del usuario, descarga el binario precompilado correcto desde
GitHub Releases, verifica su integridad SHA256 e instala el binario sin requerir Go ni ningún
runtime adicional; una configuración de GoReleaser que compila y publica cuatro binarios por
plataforma (darwin/linux × amd64/arm64) en un GitHub Release; y un workflow de GitHub Actions
que activa el pipeline completo al pushear un tag `v*`. Incluye la inyección de la versión en
tiempo de compilación (reemplazando el hardcode actual en `main.go`) y la documentación de
instalación en `docs/install.md`.

Esta feature permite que un **developer** pueda instalar Vector ejecutando un único comando
`curl -fsSL <url> | sh` —válido en cuanto el repo sea público— sin compilar desde fuentes,
obteniendo el binario correcto para su sistema operativo y arquitectura con verificación de
integridad automática.

La infraestructura se construye completa ahora ("build now"), pero el `curl|sh` anónimo
público permanece "armed but not live" hasta que el usuario decida hacer el repo público y
añadir la LICENSE — esa es la decisión de habilitación, no un entregable de este spec.

---

## 2. Alcance

### Incluido en esta fase

- **`scripts/install.sh`**: script bash (3.2+) que detecta OS y arquitectura, resuelve la
  última versión via GitHub Releases API (o usa `--version <tag>`), descarga el asset
  correcto, verifica SHA256 contra `checksums.txt`, e instala en `~/.local/bin/vector` o
  `$VECTOR_INSTALL_DIR`. Flags: `--version <tag>`, `--dry-run`, `--force`. Env:
  `$VECTOR_INSTALL_DIR`, `DEBUG=1`.
- **`.goreleaser.yml`**: configuración de GoReleaser v2 para compilar 4 binarios
  (darwin/linux × amd64/arm64), empaquetarlos en tar.gz, generar `checksums.txt` SHA256, e
  inyectar la versión via ldflags al publicar en GitHub Releases.
- **`.github/workflows/release.yml`**: workflow de GitHub Actions activado por tags `v*`; 
  ejecuta el web build, la copia de dist, `go generate` y GoReleaser, en ese orden estricto.
- **Inyección de versión en tiempo de compilación**: cambio de `const version = "0.0.1-dev"`
  a `var version = "dev"` en `cli/cmd/vector/main.go` (actualmente línea 26; verificar antes
  de modificar), con `-ldflags "-X main.version={{.Version}}"` en el build de GoReleaser.
- **`docs/install.md`**: documentación de instalación en español con instrucciones, flags,
  requisitos, nota explícita sobre la restricción del repo privado, y referencia cruzada a
  `README.md`.
- Validación privada del pipeline antes del lanzamiento público (download autenticado con
  token).

### Fuera de scope

- Instalador para Windows (el script aborta con mensaje claro "not supported yet").
- Compilación desde fuentes en el script de instalación.
- Hacer el repo público o añadir `LICENSE` (decisión del usuario; es el paso de habilitación
  del `curl|sh` anónimo público; rastreado en Open questions, no un entregable de este spec).
- Modificar `README.md` (spec separado: `rewrite-public-readme-humanized`; agregar solo una
  referencia cruzada desde `docs/install.md`).
- Homebrew tap, npm shim, ni otros canales de distribución en V1.
- Firma GPG de los binarios en V1 (solo SHA256 sobre HTTPS).
- Script de desinstalación en V1.
- Workflow de CI para PRs/branches (solo release en este spec).
- Pushear tags ni activar el pipeline como parte de implementar este spec.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca
relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Lenguaje principal: **Go 1.26** (declarado en `cli/go.mod`; módulo
  `github.com/mariocampbell/vector`)
- GitHub remote: `github.com/mcampbellr/vector` (verificado en `.git/config`). Nota: el
  módulo Go tiene path `github.com/mariocampbell/vector` — son distintos; GoReleaser usa el
  owner del remote de GitHub (`mcampbellr`), el build usa el módulo Go. El implementador debe
  verificar y alinear ambos.
- Release tool: **GoReleaser v2** (versión exacta TBD — ver Open questions §4; pinnar la versión
  del GoReleaser Action en el workflow)
- CI/CD: **GitHub Actions** (`ubuntu-latest`; activado por push de tags `v*`)
- Script de instalación: **bash 3.2+** (compatible con macOS legacy; sin arrays asociativos,
  sin GNU-isms; solo `curl`/`wget` + coreutils)
- Web build: **Vite ^6.0.0** + **TypeScript ^5.7.2** + **React ^19.1.0** con **npm** (build
  script: `npm run build` en `web/`, según `web/package.json`)
- Sin dependencias externas de Go: el módulo usa solo stdlib (confirmado: no hay `cli/go.sum`)

### Versiones relevantes

- Go: **1.26** (`cli/go.mod`)
- GoReleaser Action: TBD — ver Open questions §4 (pinnar versión exacta en el workflow)
- Vite: **^6.0.0** (`web/package.json`)
- React: **^19.1.0** (`web/package.json`)
- TypeScript: **^5.7.2** (`web/package.json`)
- npm: versión del sistema CI (usar `npm ci` en el workflow para reproducibilidad)

No usar librerías, APIs, flags o patrones que no estén documentados oficialmente o que no
estén ya presentes en el proyecto, salvo que este spec lo autorice explícitamente.

### Patrones existentes a respetar

- **Un solo binario con assets embebidos**: `embed.FS` en `cli/internal/webui` y
  `cli/internal/scaffold/assets/`; la distribución es un solo archivo ejecutable
  (`.claude/rules/architecture/distribution-packaging.md`).
- **Orden de build obligatorio**: `npm run build` (web/) → copiar `web/dist/` a
  `cli/internal/webui/dist/` → `go -C cli generate ./internal/scaffold` → `go build`. El
  workflow de Actions y los `before.hooks` de GoReleaser deben respetar este orden. Violarlo
  produce un binario con assets desactualizados o con `TestAssetsMatchKit` roto.
- **CLI-owns-writes**: el binario es el único escritor del estado en `.vector/`; el script
  `install.sh` solo escribe el binario en el directorio de instalación del usuario.
- **Sin dependencias externas en Go**: el módulo usa solo stdlib; los builds usan
  `CGO_ENABLED=0` para distribución estática.
- **`TestAssetsMatchKit`**: el test en `cli/internal/scaffold/scaffold_test.go` detecta drift
  entre `kit/` y `assets/`; `go generate` debe correr antes de `go test` en el pipeline.
- **Naming kebab-case** para flags del installer (`--dry-run`, `--force`, `--version`).
- **Artefactos de git en inglés**: commits, branch names, workflow names, etc.

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `cli/cmd/vector/main.go` existe con `const version = "0.0.1-dev"` (actualmente línea
      26; verificar la línea exacta antes de modificar).
- [x] Build de `web/` funcional: `cd web && npm ci && npm run build` produce `web/dist/`
      (assets actuales visibles en `cli/internal/webui/dist/`).
- [x] `cli/internal/webui/dist/` existe con contenido válido (verificado: contiene
      `index.html` y assets en `assets/`; el embed es válido).
- [x] `go -C cli generate ./internal/scaffold` corre sin errores.
- [x] `go -C cli build ./cmd/vector` produce un binario válido.
- [x] GitHub Actions está habilitado para el repo `github.com/mcampbellr/vector` (privado).
- [x] `secrets.GITHUB_TOKEN` está disponible en el contexto de Actions (provisto
      automáticamente por GitHub).
- [ ] **Tag inicial** (ej. `v0.1.0`): no existe aún. El implementador NO lo crea como parte
      de este spec; lo crea el usuario cuando el pipeline esté listo. TBD — ver Open questions
      §3.
- [ ] **Repo público**: GATING para el `curl|sh` anónimo; no bloqueante para construir la
      infra. TBD — ver Open questions §1.
- [ ] **LICENSE**: requerida antes del lanzamiento público; no es un entregable de este spec.
      TBD — ver Open questions §1.

Si alguna dependencia marcada `[x]` no existe al iniciar, el agente debe detenerse y reportar
exactamente qué falta. No debe inventar contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

**Pipeline de release activado por tag + instalador que consume GitHub Releases API.** El
pipeline corre en CI (GitHub Actions) orquestado por GoReleaser. El instalador es un script
bash stateless que consulta la API de GitHub Releases para resolver la versión, descarga el
asset correcto para la plataforma detectada, verifica SHA256, e instala el binario localmente
sin elevar privilegios.

### Capas afectadas

- presentation (web/board): **no** — sin cambios de UI.
- application/CLI (`cli/cmd/vector`): **sí** — cambio mínimo en `main.go`: `const → var
  version`.
- kit (`kit/`): **no** — sin cambios de commands ni agentes.
- pipeline/CI: **sí** — `.github/workflows/release.yml` nuevo.
- tooling/release: **sí** — `.goreleaser.yml` nuevo.
- distribución/docs: **sí** — `scripts/install.sh` y `docs/install.md` nuevos.

### Flujo esperado del pipeline de release

1. El usuario crea y pushea un tag `v<semver>` (ej. `v0.1.0`).
2. GitHub Actions dispara el workflow `release.yml`.
3. **Step 1 — checkout**: `actions/checkout@v4` con `fetch-depth: 0` (GoReleaser necesita el
   historial completo para generar el changelog).
4. **Step 2 — setup-go**: `actions/setup-go@v5` con `go-version-file: cli/go.mod`.
5. **Step 3 — setup-node**: `actions/setup-node@v4` (versión LTS o según `.nvmrc` si existe).
6. **Step 4 — web build**: `cd web && npm ci && npm run build`.
7. **Step 5 — copy dist**: `cp -r web/dist/* cli/internal/webui/dist/` (reemplaza el contenido
   del directorio de embed; el directorio ya existe con assets previos).
8. **Step 6 — go generate**: `go -C cli generate ./internal/scaffold` — sincroniza `kit/` →
   `assets/`; garantiza que `TestAssetsMatchKit` pase.
9. **Step 7 — goreleaser**: `goreleaser/goreleaser-action@vX` (versión pineada TBD) con
   `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`. GoReleaser compila los 4 binarios con
   `-ldflags "-X main.version={{.Version}}"`, crea los tar.gz, genera `checksums.txt`, y
   publica el GitHub Release.
10. El GitHub Release queda con 4 tar.gz + `checksums.txt` en
    `github.com/mcampbellr/vector/releases`.

### Flujo esperado del instalador

1. El usuario ejecuta `curl -fsSL https://<url>/scripts/install.sh | sh` (anónimo cuando el
   repo sea público; autenticado mientras sea privado).
2. El script detecta OS (`uname -s`) y arch (`uname -m`), normalizando `aarch64 → arm64`,
   `x86_64 → amd64`.
3. Si la plataforma no es soportada (cualquier OS != Darwin/Linux, o Windows), aborta con
   mensaje claro en stderr, exit 1.
4. Si `--version` no se pasa, consulta `GET
   https://api.github.com/repos/mcampbellr/vector/releases/latest` y extrae `tag_name` con
   `grep`/`sed` (sin `jq`).
5. Construye la URL del asset:
   `https://github.com/mcampbellr/vector/releases/download/<tag>/vector_<version_sin_v>_<os>_<arch>.tar.gz`
   (ej. tag `v0.1.0` → `vector_0.1.0_darwin_arm64.tar.gz`).
6. Descarga el asset y `checksums.txt` en un directorio temporal (`mktemp -d`); el script
   registra un trap `EXIT` para limpiar el temporal en cualquier salida.
7. Verifica SHA256: `shasum -a 256 --check` en Darwin; `sha256sum --check` en Linux. Si falla,
   limpia el temporal y aborta.
8. Crea el directorio de destino si no existe (`mkdir -p`). Comprueba permisos de escritura.
9. Instala el binario con `install -m 0755 <binary> "$INSTALL_DIR/vector"`.
10. Si `$INSTALL_DIR` no está en `$PATH`, imprime una sugerencia de export sin editar
    `.bashrc`/`.zshrc`.
11. Verifica la instalación con `"$INSTALL_DIR/vector" version`.

### Ubicación de archivos nuevos

```txt
vector/                         ← raíz del repo
  scripts/
    install.sh                  ← NUEVO
  .goreleaser.yml               ← NUEVO
  .github/
    workflows/
      release.yml               ← NUEVO
  cli/
    cmd/
      vector/
        main.go                 ← MODIFICAR (const → var version)
  docs/
    install.md                  ← NUEVO
```

---

## 6. Archivos a crear o modificar

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `scripts/install.sh` | NUEVO | Script bash 3.2+ de instalación de un paso: detecta plataforma, descarga binario de GitHub Releases, verifica SHA256, instala en `~/.local/bin/vector` o `$VECTOR_INSTALL_DIR` | Sin análogo de scripts shell en este repo; `cli/internal/webui/webui.go` ilustra el patrón de embed que el binario instalado sirve (contexto de arquitectura) |
| `.goreleaser.yml` | NUEVO | Configuración GoReleaser v2: 4 targets (darwin/linux × amd64/arm64), ldflags de versión, `checksums.txt` SHA256, publicación en `mcampbellr/vector` de GitHub | Sin análogo en este repo; seguir documentación oficial GoReleaser v2 |
| `.github/workflows/release.yml` | NUEVO | Workflow de Actions activado por `v*` tags; pasos en orden estricto: checkout → setup-go → setup-node → web build → copy dist → go generate → goreleaser | Sin análogo en este repo; patrón estándar `goreleaser/goreleaser-action` |
| `cli/cmd/vector/main.go` | MODIFICAR | Línea 26: cambiar `const version = "0.0.1-dev"` a `var version = "dev"` para permitir inyección con ldflags en tiempo de compilación | Mismo archivo; la variable `version` es referenciada en el subcomando `version` del switch de `main()` |
| `docs/install.md` | NUEVO | Documentación de instalación en español: requisitos, comando de instalación, flags de `install.sh`, nota sobre repo privado, verificación post-instalación, sugerencia de PATH | `docs/vision.md` (estructura y tono del archivo de documentación) |

### Detalle por archivo

#### scripts/install.sh

Acción: NUEVO

Nota sobre ubicación: el script vive en `scripts/install.sh`. La URL raw de GitHub para
descarga directa (`https://raw.githubusercontent.com/mcampbellr/vector/main/scripts/install.sh`)
requiere que el repo sea público; mientras sea privado, la validación se hace localmente o
con auth. Ver Open questions §2.

Debe implementar:

- Shebang `#!/usr/bin/env bash` y `set -euo pipefail`. `DEBUG=1` activa `set -x` antes del
  resto del script.
- Trap `EXIT` que borra el directorio temporal creado con `mktemp -d`.
- Detección de OS: `uname -s`. Soportado: `Darwin`, `Linux`. Cualquier otro (incluidos
  `MINGW*`, `MSYS*`, `CYGWIN*`, `Windows*`) → abortar con mensaje en stderr: `"Error:
  Windows is not supported in V1. Only macOS (darwin) and Linux are supported."`, exit 1.
- Detección y normalización de arch: `uname -m` → `x86_64` → `amd64`; `arm64` / `aarch64`
  → `arm64`. Cualquier otro → abortar: `"Unsupported architecture: <value>. Supported: amd64
  (x86_64), arm64 (aarch64)."`, exit 1.
- Parseo de flags de CLI con un loop sobre `$@`: `--version <tag>`, `--dry-run`, `--force`.
- Variable `INSTALL_DIR`: default `"$HOME/.local/bin"`; overridable con `$VECTOR_INSTALL_DIR`.
- Resolución de versión latest: si `--version` no se pasa, `curl -fsSL` (o `wget -qO-` como
  fallback si `curl` no está) a
  `https://api.github.com/repos/mcampbellr/vector/releases/latest` y extraer `tag_name` con
  `grep '"tag_name"'` + `sed` (sin `jq`). Si el valor extraído es vacío → abortar.
- Construcción del nombre del asset: OS en minúscula (`darwin`/`linux`), arch normalizado;
  versión sin prefijo `v` (e.g., tag `v0.1.0` → `version="0.1.0"` via `${tag#v}`). Nombre
  final: `vector_${version}_${os}_${arch}.tar.gz`. El implementador debe verificar que este
  template coincida exactamente con el `name_template` definido en `.goreleaser.yml`.
- Descarga del asset y de `checksums.txt` en `$TMPDIR` (resultado de `mktemp -d`).
- Verificación SHA256: filtrar la línea de `checksums.txt` que corresponde al archivo
  descargado (`grep "<filename>" checksums.txt`), luego verificar con `shasum -a 256 --check`
  (Darwin) o `sha256sum --check` (Linux). Si falla → limpiar y abortar.
- Modo `--dry-run`: imprimir todos los pasos prefijados con `[dry-run]` sin descargar ni
  instalar. Respetar el flag en cada operación efectiva.
- Modo `--force`: saltear la comprobación de versión instalada; instalar aunque la misma
  versión ya esté presente.
- `mkdir -p "$INSTALL_DIR"` antes de intentar instalar.
- Verificar `[ -w "$INSTALL_DIR" ]` después de `mkdir -p`; si no hay permisos → abortar.
- Copiar el binario extraído con `install -m 0755 vector "$INSTALL_DIR/vector"`.
- Post-instalación: si `$INSTALL_DIR` no está en `$PATH` → imprimir:
  `'Add ~/.local/bin to your PATH: export PATH="$HOME/.local/bin:$PATH"'` (sugerencia; no
  editar ningún archivo de shell).
- Verificación final: ejecutar `"$INSTALL_DIR/vector" version`; si devuelve `"dev"` → emitir
  un warning (no un error; el binario es válido).
- Compatibilidad bash 3.2+: sin arrays asociativos (`declare -A`), sin herestrings complejos,
  sin GNU-specific flags de `sed`/`grep` (`-E` de `grep` está en POSIX; `-r` de `sed` no
  está en BSD sed → usar `-E` si hace falta extended regex).

No debe incluir:

- Compilación desde fuentes.
- Soporte Windows en la lógica principal (solo abortar).
- Uso de `sudo` en ningún punto.
- `jq`, `python`, `ruby`, ni ninguna herramienta no disponible en un sistema limpio.
- Edición automática de `.bashrc`, `.zshrc` ni `.profile`.
- GPG verification.

#### .goreleaser.yml

Acción: NUEVO

Debe implementar:

- `version: 2` (GoReleaser v2 syntax).
- `project_name: vector`.
- `before.hooks`: como salvaguarda del orden de build (cinturón de seguridad; los pasos
  principales están en el workflow de Actions):
  ```yaml
  before:
    hooks:
      - sh -c "cd web && npm ci && npm run build && cp -r web/dist/* cli/internal/webui/dist/"
      - go -C cli generate ./internal/scaffold
  ```
- `builds`: bloque para el binario `vector`:
  - `dir: cli` (el módulo Go vive en `cli/`).
  - `main: ./cmd/vector`.
  - `binary: vector`.
  - `goos: [darwin, linux]`.
  - `goarch: [amd64, arm64]`.
  - `ldflags: ["-s -w -X main.version={{.Version}}"]`. `-s -w` reduce el tamaño del
    binario; `-X main.version={{.Version}}` inyecta la versión del tag.
  - `env: [CGO_ENABLED=0]` (compilación estática; sin dependencias de libc del sistema).
- `archives`:
  - `format: tar.gz` para todos.
  - `name_template: "vector_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`. Esto produce nombres
    como `vector_0.1.0_darwin_arm64.tar.gz` (GoReleaser serializa `{{ .Os }}` en minúscula
    por defecto). El implementador debe verificar que el resultado sea exactamente
    `vector_<VERSION>_<os>_<arch>.tar.gz` y que `install.sh` construya el mismo string.
  - `files`: solo el binario (sin docs, scripts ni configs dentro del tar.gz).
- `checksum`:
  - `name_template: "checksums.txt"`.
  - `algorithm: sha256`.
- `release`:
  - `github.owner: mcampbellr`.
  - `github.name: vector`.
  - `draft: false`.
  - `prerelease: auto` (tags con `-` como `v0.1.0-beta` se marcan como prerelease).
- Sin Homebrew, Docker ni snapcraft en V1.

Restricciones:

- El `name_template` del archive y el nombre que construye `install.sh` deben ser
  idénticos. El implementador valida esto manualmente antes de entregar.
- No añadir configuraciones de distribución adicionales (Homebrew tap, Docker Hub, etc.).
- No configurar Windows targets.

#### .github/workflows/release.yml

Acción: NUEVO

Debe implementar:

- `name: Release`.
- `on: push: tags: ['v*']`.
- `permissions: contents: write` (necesario para crear GitHub Releases).
- Un solo job `release` en `runs-on: ubuntu-latest`.
- Pasos en orden estricto (el orden es una restricción funcional, no de estilo):

  1. `name: Checkout` — `actions/checkout@v4` con `fetch-depth: 0`.
  2. `name: Set up Go` — `actions/setup-go@v5` con `go-version-file: cli/go.mod`.
  3. `name: Set up Node` — `actions/setup-node@v4`; si no existe `.nvmrc` en el repo,
     usar `node-version: 'lts/*'`. El implementador verifica si `.nvmrc` existe.
  4. `name: Build web` — `run: cd web && npm ci && npm run build`.
  5. `name: Copy web dist to embed dir` — `run: cp -r web/dist/* cli/internal/webui/dist/`.
  6. `name: Run go generate` — `run: go -C cli generate ./internal/scaffold`.
  7. `name: Run tests` — `run: go -C cli test ./...` (verifica `TestAssetsMatchKit` y todos
     los tests de Go).
  8. `name: Release with GoReleaser` — `uses: goreleaser/goreleaser-action@vX` (pinnar
     versión TBD — ver Open questions §4); con `args: release --clean` y `env: GITHUB_TOKEN:
     ${{ secrets.GITHUB_TOKEN }}`.

- `env` a nivel de job (si se usa en múltiples steps): `GITHUB_TOKEN:
  ${{ secrets.GITHUB_TOKEN }}`.

Restricciones:

- El step de web build (4) debe ser anterior al step de go generate (6) y al de GoReleaser
  (8). Si se invierte el orden, el binario publicado tendrá assets de web desactualizados.
- `fetch-depth: 0` es obligatorio para el changelog automático de GoReleaser.
- No añadir steps de deploy a ambientes, push a registries, ni notificaciones en este workflow.

#### cli/cmd/vector/main.go

Acción: MODIFICAR

Cambios requeridos:

- Verificar la línea exacta de `const version = "0.0.1-dev"` antes de editar (actualmente
  línea 26 según inspección; confirmar).
- Cambiar `const version = "0.0.1-dev"` por `var version = "dev"`. El valor `"dev"` es el
  fallback para builds locales sin ldflags. GoReleaser inyecta el valor real del tag en
  releases con `-X main.version={{.Version}}`.
- No cambiar ninguna otra línea de `main.go`.

Restricciones:

- Solo cambiar `const` → `var` y el valor default del string. Sin lógica adicional.
- No modificar la función `main()`, los subcomandos, los flags existentes, ni los imports.
- No modificar `kitVersion` en `internal/config` — ese es un campo de estado del board,
  completamente independiente de la versión del binario.

#### docs/install.md

Acción: NUEVO

Debe implementar (en español):

- **Sección de requisitos**: macOS 12+ o Linux (Ubuntu 20.04+ o equivalente); bash; curl o
  wget. Sin Go requerido en la máquina del usuario.
- **Sección de instalación**: el comando `curl -fsSL <URL> | sh` con nota explícita y
  destacada: este comando **solo funciona cuando el repo `mcampbellr/vector` es público**
  (pendiente — ver sección Open questions). Mientras el repo sea privado, la instalación se
  hace localmente o mediante download autenticado.
- **Sección de flags de `install.sh`**: tabla con `--version <tag>`, `--dry-run`, `--force`,
  y la variable de entorno `VECTOR_INSTALL_DIR`.
- **Sección de verificación post-instalación**: `vector version`.
- **Sección de PATH**: instrucción manual para añadir `~/.local/bin` al PATH si no está
  presente.
- **Referencia cruzada a `README.md`**: nota de que la sección de instalación del README se
  actualizará en el spec `rewrite-public-readme-humanized` cuando el repo sea público.

No debe incluir:

- Instrucciones para Windows.
- Instrucciones de compilación desde fuentes.
- Uso de `sudo`.

---

## 7. API Contract

El único contrato externo de esta feature es la **GitHub Releases API de solo lectura**. No
se introduce ni modifica ningún endpoint HTTP interno de Vector.

### Endpoint: Resolución de versión latest

```
GET https://api.github.com/repos/mcampbellr/vector/releases/latest
```

- **Auth**: mientras el repo sea privado, este endpoint devuelve `404` para requests anónimas.
  Requiere `Authorization: Bearer <token>` para acceso privado. Cuando el repo sea público,
  no requiere auth.
- **Campo relevante** en la respuesta JSON: `tag_name` (string, ej. `"v0.1.0"`).
- **Errores esperados**:
  - `404 Not Found`: repo privado sin auth, o sin releases publicados. Mensaje en el
    instalador: `"Could not resolve latest version. If the repo is private, it may not be
    publicly accessible yet."`.
  - `403 Forbidden`: rate limit de la API de GitHub (60 req/h sin auth). Mensaje: `"GitHub
    API rate limit hit. Try again later or use --version <tag>."`.
  - `tag_name` vacío en la respuesta: abortar con `"Could not resolve latest version from
    GitHub API."`.

### Endpoint: Descarga de asset y checksums

```
GET https://github.com/mcampbellr/vector/releases/download/<tag>/<asset>.tar.gz
GET https://github.com/mcampbellr/vector/releases/download/<tag>/checksums.txt
```

- **Auth**: mismo comportamiento que la API mientras el repo sea privado.
- **Esquema de naming de assets** (debe coincidir entre `.goreleaser.yml` y `install.sh`):
  `vector_<VERSION>_<OS>_<ARCH>.tar.gz` donde `<VERSION>` es el tag sin prefijo `v` (ej.
  `v0.1.0` → `0.1.0`), `<OS>` es `darwin` o `linux` en minúscula, `<ARCH>` es `amd64` o
  `arm64`. Ejemplo: `vector_0.1.0_darwin_arm64.tar.gz`.
- **`checksums.txt`**: una línea por asset con formato `<sha256>  <filename>` (dos espacios,
  compatible con `shasum --check` y `sha256sum --check`). Generado por GoReleaser.
- **`404` en el asset de la plataforma**: si GoReleaser no publicó el asset para esa plataforma,
  el instalador aborta con `"No prebuilt binary found for <os>/<arch> in release <tag>."`.

### Nota sobre privacidad del repo

Mientras `github.com/mcampbellr/vector` sea privado, toda request anónima a estos endpoints
devuelve `404` o `403`. El `curl|sh` público no puede funcionar. La validación privada del
pipeline requiere autenticación manual. Ver §14 para restricciones de seguridad sobre el
manejo del token.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] `bash -n scripts/install.sh` sale con código 0 (syntax check sin errores).
- [ ] `goreleaser check` ejecutado desde la raíz del repo con `.goreleaser.yml` en su lugar
      no reporta errores ni warnings.
- [ ] Un push del tag `v0.1.0` activa el workflow `release.yml` y el GitHub Release resultante
      contiene exactamente 4 assets (`vector_0.1.0_darwin_amd64.tar.gz`,
      `vector_0.1.0_darwin_arm64.tar.gz`, `vector_0.1.0_linux_amd64.tar.gz`,
      `vector_0.1.0_linux_arm64.tar.gz`) más `checksums.txt`.
- [ ] `vector version` ejecutado con el binario producido por GoReleaser devuelve `v0.1.0` (o
      la versión del tag pushado), no `"dev"`.
- [ ] `vector version` ejecutado con un binario compilado localmente sin ldflags devuelve
      `"dev"` (el fallback funciona correctamente).
- [ ] En un sistema macOS arm64, el script detecta OS `darwin` y arch `arm64`, y descarga
      `vector_X.Y.Z_darwin_arm64.tar.gz`.
- [ ] En un sistema Linux x86_64, el script normaliza `x86_64 → amd64` y descarga
      `vector_X.Y.Z_linux_amd64.tar.gz`.
- [ ] `uname -m` devolviendo `aarch64` es normalizado a `arm64` por el script.
- [ ] Si el SHA256 del archivo descargado no coincide con `checksums.txt`, el script aborta
      con el mensaje de error definido y elimina el directorio temporal.
- [ ] `--dry-run` imprime los pasos con prefijo `[dry-run]` sin descargar ni instalar.
- [ ] `--force` reinstala el binario aunque `vector version` ya reporte la versión correcta.
- [ ] `--version v0.1.0` instala esa versión específica sin consultar la API de latest.
- [ ] `VECTOR_INSTALL_DIR=/tmp/test_bin bash scripts/install.sh --version v0.1.0` instala el
      binario en `/tmp/test_bin/vector`.
- [ ] El binario queda instalado con modo `0755` (`ls -la` confirma).
- [ ] Si `$INSTALL_DIR` no está en `$PATH`, el script emite la sugerencia de export sin editar
      ningún archivo de shell.
- [ ] `go -C cli test ./...` sigue verde: `TestAssetsMatchKit` y todos los demás tests pasan.
- [ ] **Realidad "build now / publish later"**: mientras el repo sea privado, un `curl|sh`
      anónimo falla con `404`/`403` de GitHub — este es el comportamiento esperado y correcto.
      Una validación autenticada del mismo flujo (con `Authorization: Bearer $TOKEN` en la
      request manual de descarga) sí debe funcionar. El criterio de éxito del `curl|sh`
      público está bloqueado en TBD — ver Open questions §1 (repo público + LICENSE).

### Tests requeridos

Agregar o actualizar tests para:

- [ ] `bash -n scripts/install.sh` como smoke test de sintaxis (puede incluirse en un
      Makefile, en el workflow de PR, o en el README como instrucción de validación local).
- [ ] `goreleaser check` en CI (añadible a un workflow de lint de PR separado, sin hacer
      release).
- [ ] Compilación con ldflags: `go -C cli build -ldflags "-X main.version=v0.0.0-test" -o
      /tmp/vector-test ./cmd/vector && /tmp/vector-test version` → debe imprimir `v0.0.0-test`.
- [ ] Test de regresión: `go -C cli test ./...` verde tras el cambio `const → var`.

### Comandos de verificación

```bash
# Syntax check del instalador
bash -n scripts/install.sh

# Verificar configuración de GoReleaser sin publicar
goreleaser check

# Build local con inyección de versión (verifica el cambio const→var)
go -C cli build -ldflags "-X main.version=v0.1.0-test" -o /tmp/vector-test ./cmd/vector
/tmp/vector-test version
# Esperado: "v0.1.0-test"

# Build sin ldflags (verifica fallback "dev")
go -C cli build -o /tmp/vector-dev ./cmd/vector
/tmp/vector-dev version
# Esperado: "dev"

# Test suite Go (no debe haber regresiones)
go -C cli test ./...

# Snapshot local de GoReleaser (compila los 4 binarios sin publicar; requiere goreleaser instalado)
goreleaser release --snapshot --clean
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

La "UX" de esta feature es la experiencia del developer que instala Vector o que ejecuta el
pipeline de release. No hay UI web ni formularios.

### Salida del instalador

El script emite progress lines a stdout prefijadas con `==>` para el flujo normal:

- `==> Detected: darwin arm64`
- `==> Resolving latest version...` → `==> Latest version: v0.1.0`
- `==> Using pinned version: v0.1.0` (cuando se usa `--version`)
- `==> Downloading vector_0.1.0_darwin_arm64.tar.gz...`
- `==> Downloading checksums.txt...`
- `==> Verifying checksum...` → `==> Checksum OK`
- `==> Installing vector to /home/user/.local/bin/vector...`
- `==> vector v0.1.0 installed successfully`

Los errores van a stderr y son accionables (incluyen la URL o el path relacionado):

- No `"failed"` genérico, sino `"Failed to download asset: HTTP 404. Check that release
  v0.1.0 exists at https://github.com/mcampbellr/vector/releases"`.

En modo `--dry-run`, cada línea de acción se prefixa con `[dry-run]`:

- `[dry-run] Would download: vector_0.1.0_darwin_arm64.tar.gz`
- `[dry-run] Would install to: /home/user/.local/bin/vector`

Si `$INSTALL_DIR` no está en `$PATH` (post-instalación exitosa):

```
Add ~/.local/bin to your PATH: export PATH="$HOME/.local/bin:$PATH"
```

En Windows (o OS no soportado):

```
Error: Windows is not supported in V1. Only macOS (darwin) and Linux are supported.
```

(Salida a stderr, exit 1.)

### Experiencia del pipeline (GitHub Actions)

- Cada step del workflow tiene `name:` descriptivo para que el log de Actions sea legible sin
  leer el YAML.
- GoReleaser genera el changelog automático a partir del historial de commits del tag. El
  `fetch-depth: 0` en el checkout es la condición de habilitación.
- El log del workflow de Actions confirma qué binarios se compilaron y qué assets se
  publicaron.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas; el agente no debe cuestionarlas ni cambiarlas:

1. **Scope completo = installer + pipeline end-to-end**: el spec cubre `install.sh`,
   `.goreleaser.yml`, GitHub Actions workflow e inyección de versión. No se divide en fases.
   Razón: las piezas son co-dependientes; un release sin installer o un installer sin pipeline
   no son útiles por separado.

2. **Modelo "build now, publish later"**: toda la infraestructura se construye ahora, pero el
   `curl|sh` anónimo público solo funciona cuando el repo sea público. Hacer el repo público y
   añadir LICENSE es la decisión del usuario, no un entregable de este spec. Razón: el usuario
   quiere el pipeline listo para poder publicar cuando decida; no hay razón para bloquear la
   build infra en una decisión legal/comercial.

3. **Binarios precompilados, sin compilación desde fuentes**: el installer descarga el binario
   correcto para la plataforma. No requiere Go en la máquina del usuario. Razón: frictionless
   install; este es el objetivo de distribución día 0 declarado en
   `.claude/rules/architecture/distribution-packaging.md`.

4. **Plataformas V1 = darwin + linux, amd64 + arm64 (4 binarios)**: Windows excluido
   explícitamente. El installer aborta en Windows con mensaje claro. Razón: simplificar V1;
   Windows requiere `.exe`, PowerShell script, y complejidad adicional que se resuelve después.

5. **Ubicación de instalación = `~/.local/bin/vector`** (0755), overridable por
   `$VECTOR_INSTALL_DIR`. Sin `sudo`, sin `/usr/local/bin`. Razón: no requerir permisos de
   administrador; `~/.local/bin` es la convención estándar de herramientas de usuario en
   Linux y macOS.

6. **Versión injectable via ldflags**: `go build -ldflags "-X main.version={{.Version}}"`.
   Fallback `"dev"` para builds locales. Razón: el binario de release debe autoidentificar su
   versión; el hardcode actual (`"0.0.1-dev"`) hace imposible distinguir versiones.

7. **Detección de latest via GitHub Releases API**: `GET .../releases/latest`, campo
   `tag_name`; `--version <tag>` pasa la versión directamente. Sin `jq`. Razón: la API de
   GitHub es la fuente canónica de releases; parsear con `grep`/`sed` evita dependencias en
   el sistema del usuario.

8. **SHA256 sin GPG en V1**: `checksums.txt` con SHA256 verificado con `shasum`/`sha256sum`.
   Sin GPG signing. Razón: GPG añade complejidad significativa (keyserver, trust chain, gestión
   de clave); SHA256 sobre HTTPS es suficiente para V1.

9. **Bash 3.2+ compatible**: sin arrays asociativos ni GNU-isms. Razón: macOS incluye bash 3.2
   por defecto; el script debe funcionar sin `brew install bash`.

10. **`scripts/install.sh` (no raíz)**: el script vive en `scripts/install.sh` en lugar de
    `install.sh` en la raíz. Razón: mantiene la raíz limpia; agrupa scripts utilitarios.
    Tradeoff: la raw URL incluye el subdirectorio; si en el futuro se necesita una URL más
    corta, se puede añadir un redirect o symlink. Mientras el repo sea privado, la URL raw no
    es accesible públicamente de todas formas.

11. **`go -C cli` como prefijo de comandos Go**: todos los comandos Go en el workflow y en
    los comandos de verificación usan `go -C cli <cmd>` en lugar de `cd cli && go <cmd>`.
    Razón: Go 1.21+ soporta `-C`; alinea con Go 1.26 declarado en go.mod y con patrones del
    workflow. El implementador verifica compatibilidad con el runner de Actions.

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, no
implementarla.

---

## 11. Edge cases

### Plataforma no soportada

- **Windows** (`uname -s` devuelve `MINGW*`, `MSYS*`, `CYGWIN*`, o cualquier string que no
  sea `Darwin` ni `Linux`): el script sale inmediatamente con exit 1 y mensaje en stderr:
  `"Error: Windows is not supported in V1. Only macOS (darwin) and Linux are supported."`. No
  intenta continuar.
- **OS desconocido** (ej. FreeBSD, OpenBSD): mismo comportamiento — abortar con el mismo
  mensaje indicando que solo Darwin y Linux son soportados.

### Normalización de arquitectura

- `uname -m` → `aarch64`: normalizar a `arm64` antes de construir la URL del asset.
- `uname -m` → `x86_64`: normalizar a `amd64`.
- `uname -m` → cualquier otro valor (ej. `i686`, `s390x`): abortar con `"Unsupported
  architecture: <value>. Supported: amd64 (x86_64), arm64 (aarch64)."`, exit 1.

### GitHub API y descarga

- **`404` en `/releases/latest`**: repo privado sin auth, o sin releases. Mensaje: `"Could
  not resolve latest version. If the repo is private, it may not be publicly accessible
  yet."`. Abortar.
- **`403` en `/releases/latest`**: rate limit de la API de GitHub (60 req/h sin auth).
  Mensaje: `"GitHub API rate limit hit. Try again later or use --version <tag>."`. Abortar.
- **`tag_name` vacío en la respuesta**: el script debe detectar que la extracción del campo
  produjo string vacío y abortar con `"Could not resolve latest version from GitHub API."`.
- **Descarga parcial → fallo de checksum**: el trap `EXIT` garantiza la limpieza del directorio
  temporal. Mensaje: `"Checksum verification failed for <filename>. The download may be
  corrupt. Try again."`. Abortar con exit 1.
- **`checksums.txt` ausente** (404 al descargar): abortar con `"Could not download
  checksums.txt. Cannot verify integrity."`. No instalar un binario sin verificar.
- **Asset de la plataforma no existe en el release**: `404` en el URL del tar.gz. Mensaje:
  `"No prebuilt binary found for <os>/<arch> in release <tag>."`. Abortar.
- **Repo privado + request anónima**: se manifiesta como `404` en la API y en los assets.
  El script trata ambos con mensajes que mencionan explícitamente la posibilidad de que el
  repo sea privado.
- **Fallo de transporte (no-HTTP): host irresoluble / offline / DNS**: `curl` sale con un
  código distinto de un status HTTP (p. ej. `curl: (6) Could not resolve host`). El script
  detecta el exit code de `curl` y aborta con `"Failed to reach GitHub. Check your network
  connection and try again."` (exit 1), en vez de dejar que el usuario vea el error crudo de
  `curl`. Aplica tanto a la llamada de la API como a la descarga del binario/checksums.
- **Timeout de conexión / descarga colgada**: todas las invocaciones de `curl` usan
  `--connect-timeout 10 --max-time 300` (10 s para establecer conexión, 300 s tope total de la
  transferencia). Al superarse (`curl: (28) Operation timed out`), abortar con `"Connection to
  GitHub timed out. Try again later."` (exit 1).
- **`5xx` de la API o de los assets de GitHub** (500/502/503): con `curl -f` el status ≥400
  produce exit no-cero; el script lo distingue de un `404` (recurso ausente) y aborta con
  `"GitHub returned a server error (<code>). Try again later or use --version <tag>."` (exit 1).
- Sin reintentos automáticos en V1: ante cualquiera de estos fallos, el usuario re-ejecuta el
  script (ver §17). El mensaje siempre es accionable e incluye la URL o el path relacionado,
  consistente con el principio de §15.

### Permisos e instalación

- **Sin permisos de escritura en `$INSTALL_DIR`**: verificar `[ -w "$INSTALL_DIR" ]` después
  de `mkdir -p`. Si falla → abortar con `"No write permission in $INSTALL_DIR. Set
  VECTOR_INSTALL_DIR to a writable path."`.
- **`$VECTOR_INSTALL_DIR` apunta a un archivo en lugar de un directorio**: abortar con
  mensaje claro antes de intentar `mkdir -p`.
- **`~/.local/bin` no en `$PATH`**: no es un error; es una advertencia post-instalación
  exitosa. El script instala correctamente y luego imprime la sugerencia de export.

### Versión no inyectada

- Binario compilado sin `-X main.version=...` → `vector version` devuelve `"dev"`. Válido
  para desarrollo local. El instalador, tras instalar, corre `"$INSTALL_DIR/vector" version`;
  si devuelve `"dev"`, emite: `"Warning: installed binary reports version 'dev'. This may
  indicate a local build, not a release binary."`. No es un error fatal; el script sale con
  código 0.

### Pipeline GoReleaser

- **`before.hooks` falla** (ej. `npm ci` falla): GoReleaser aborta el release completo; el
  GitHub Release no se crea. El workflow falla en ese step con el log de error del hook.
- **`TestAssetsMatchKit` falla** (drift entre `kit/` y `assets/` sin haber corrido
  `go generate`): `go -C cli test ./...` falla en el workflow antes de que GoReleaser corra.
  El release no se publica hasta que el drift esté resuelto.
- **Tag sin formato semver** (ej. `v1` sin minor/patch): GoReleaser puede fallar o producir
  nombres de asset inesperados. Usar siempre `v<MAJOR>.<MINOR>.<PATCH>`.
- **GITHUB_TOKEN sin permisos de escritura**: el release falla al intentar crear el GitHub
  Release. Verificar que el workflow tenga `permissions: contents: write`.

---

## 12. Estados de UI requeridos

No aplica — esta feature no introduce ni modifica componentes de la UI web ni del board
kanban. La "interfaz" es la salida del script bash en la terminal.

Estados del flujo del instalador en terminal (para referencia del implementador):

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| detecting | `==> Detected: <os> <arch>` | Esperar |
| resolving-version | `==> Resolving latest version...` | Esperar (Ctrl+C para cancelar) |
| version-resolved | `==> Latest version: <tag>` | Esperar |
| downloading | `==> Downloading vector_X_Y_Z.tar.gz...` | Esperar |
| verifying | `==> Verifying checksum...` → `==> Checksum OK` | Esperar |
| installing | `==> Installing vector to <path>...` | Esperar |
| success | `==> vector vX.Y.Z installed successfully` | Ejecutar `vector --help` |
| error | Mensaje accionable en stderr, exit 1 | Leer el error, corregir y reintentar |
| dry-run | `[dry-run] Would download/install...` | Revisar los pasos sin efectos |

---

## 13. Validaciones

### Validaciones del script instalador

| Campo / Input | Regla | Mensaje de error |
|---|---|---|
| OS (`uname -s`) | Debe ser exactamente `Darwin` o `Linux` | `"Error: Windows is not supported in V1. Only macOS (darwin) and Linux are supported."` (stderr, exit 1) |
| Arch (`uname -m`) | Debe ser `x86_64`, `amd64`, `arm64`, o `aarch64` | `"Unsupported architecture: <value>. Supported: amd64 (x86_64), arm64 (aarch64)."` (stderr, exit 1) |
| `--version <tag>` | Opcional; si se pasa, se usa directamente sin validación de formato en V1 | — |
| Checksum SHA256 | El SHA256 del tar.gz descargado debe coincidir con la entrada en `checksums.txt` | `"Checksum verification failed for <filename>. The download may be corrupt. Try again."` (stderr, exit 1) |
| Permisos de escritura en `$INSTALL_DIR` | `$INSTALL_DIR` debe ser escribible tras `mkdir -p` | `"No write permission in <path>. Set VECTOR_INSTALL_DIR to a writable path."` (stderr, exit 1) |
| `tag_name` de la API | No debe ser string vacío tras extracción | `"Could not resolve latest version from GitHub API."` (stderr, exit 1) |
| `checksums.txt` descargado | Debe descargarse con éxito antes de verificar | `"Could not download checksums.txt. Cannot verify integrity."` (stderr, exit 1) |
| `$VECTOR_INSTALL_DIR` apunta a un archivo | El destino, si existe, debe ser un directorio (no un archivo regular) | `"VECTOR_INSTALL_DIR (<path>) is a file, not a directory."` (stderr, exit 1) |

### Validaciones de GoReleaser

- `goreleaser check` valida la sintaxis y coherencia del `.goreleaser.yml` antes de ejecutar
  (requiere `goreleaser` instalado localmente).
- El `name_template` del archive y el formato de URL que construye `install.sh` deben ser
  idénticos. El implementador valida esto manualmente comparando la salida de `goreleaser
  release --snapshot` con la URL que construye el script.

---

## 14. Seguridad y permisos

- **HTTPS obligatorio**: todas las descargas usan HTTPS. En `curl`, usar `--proto '=https'
  --tlsv1.2` (o al menos `-fsSL`) para forzar HTTPS y fallar en redirecciones. No permitir
  fallback a HTTP.
- **SHA256 antes de instalar**: la verificación ocurre antes de extraer o instalar el binario.
  Si la verificación falla, el directorio temporal se limpia y el script aborta. Nunca instalar
  un binario sin verificar.
- **Sin sudo**: el script no eleva privilegios. Instala solo en directorios del usuario. Si el
  usuario no tiene permisos de escritura en el directorio de destino, aborta con instrucción.
- **Sin token escrito a disco**: durante la validación privada del pipeline, si el usuario usa
  un token de GitHub (ej. via `GITHUB_TOKEN` env var para curl), el script no lo escribe en
  ningún archivo ni lo imprime en logs (salvo en `DEBUG=1`, donde el usuario entiende
  explícitamente que activa traza completa). El workflow de Actions usa `secrets.GITHUB_TOKEN`
  que GitHub gestiona internamente; el token no aparece en los logs de Actions.
- **`install.sh` no se auto-actualiza**: el script no descarga versiones más nuevas de sí
  mismo ni establece ningún mecanismo de actualización automática.
- **Validación privada sin comprometer tokens**: al validar el pipeline mientras el repo es
  privado, el usuario usa un PAT (Personal Access Token) en la variable de entorno; ese token
  no debe aparecer en historial de comandos (preferir `read -s TOKEN` o exportar desde un
  gestor de secretos) ni en logs de CI.
- **`CGO_ENABLED=0` en los builds de GoReleaser**: compilación estática; elimina la dependencia
  de libc del sistema de destino, reduciendo la superficie de ataque.

---

## 15. Observabilidad y logging

- **Salida del instalador**: progress lines a stdout; errores a stderr. `DEBUG=1` activa
  `set -x` para traza completa del script. Los errores incluyen la URL o el path relacionado
  para facilitar el debugging.
- **Pipeline en GitHub Actions**: cada step tiene `name:` descriptivo. Los logs de GoReleaser
  muestran los binarios compilados, los assets publicados y los checksums calculados.
- **`vector version`**: el principal observable de que la inyección de versión funcionó. El
  binario instalado desde un release debe reportar la versión exacta del tag.
- **`goreleaser release --snapshot --clean`**: para debug local del pipeline sin publicar;
  genera los binarios con un nombre de versión derivado del commit hash. Requiere `goreleaser`
  instalado localmente.
- No registrar tokens, credenciales ni información sensible en ningún log. En `DEBUG=1`, el
  usuario es responsable de que la traza no se almacene en sistemas de logging externos.

---

## 16. i18n / textos visibles

La prosa del spec y de `docs/install.md` está en español. Los strings visibles al usuario del
**instalador** están en **inglés** (consistente con la ayuda del CLI de Vector y con la
convención del proyecto). No hay sistema de traducciones; los strings están hardcodeados en
el script bash.

Strings de usuario del instalador (`scripts/install.sh`):

| Clave conceptual | Texto exacto (English) |
|---|---|
| platform-detected | `Detected: <os> <arch>` |
| resolving-version | `Resolving latest version...` |
| version-resolved | `Latest version: <tag>` |
| version-pinned | `Using pinned version: <tag>` |
| downloading-asset | `Downloading vector_<ver>_<os>_<arch>.tar.gz...` |
| downloading-checksums | `Downloading checksums.txt...` |
| verifying-checksum | `Verifying checksum...` |
| checksum-ok | `Checksum OK` |
| installing | `Installing vector to <path>...` |
| success | `vector <version> installed successfully` |
| path-suggestion | `Add ~/.local/bin to your PATH: export PATH="$HOME/.local/bin:$PATH"` |
| dry-run-prefix | `[dry-run]` (prefixa cada línea de acción en modo dry-run) |
| warn-dev-version | `Warning: installed binary reports version 'dev'. This may indicate a local build, not a release binary.` |
| err-unsupported-os | `Error: Windows is not supported in V1. Only macOS (darwin) and Linux are supported.` |
| err-unsupported-arch | `Unsupported architecture: <value>. Supported: amd64 (x86_64), arm64 (aarch64).` |
| err-api-404 | `Could not resolve latest version. If the repo is private, it may not be publicly accessible yet.` |
| err-api-403 | `GitHub API rate limit hit. Try again later or use --version <tag>.` |
| err-checksum-fail | `Checksum verification failed for <filename>. The download may be corrupt. Try again.` |
| err-no-checksums | `Could not download checksums.txt. Cannot verify integrity.` |
| err-asset-404 | `No prebuilt binary found for <os>/<arch> in release <tag>.` |
| err-no-perms | `No write permission in <path>. Set VECTOR_INSTALL_DIR to a writable path.` |
| err-no-version | `Could not resolve latest version from GitHub API.` |
| err-transport-fail | `Failed to reach GitHub. Check your network connection and try again.` |
| err-timeout | `Connection to GitHub timed out. Try again later.` |
| err-github-5xx | `GitHub returned a server error (<code>). Try again later or use --version <tag>.` |
| err-install-dir-is-file | `VECTOR_INSTALL_DIR (<path>) is a file, not a directory.` |

El contenido de `docs/install.md` y la prosa de este spec están en español. Los comandos
bash, flags, nombres de archivos y demás artefactos técnicos permanecen en inglés.

---

## 17. Performance

- **Tamaño del binario**: usar `-s -w` en ldflags (`-s` elimina tabla de símbolos, `-w`
  elimina debug info DWARF) para reducir el tamaño del binario publicado. El tamaño final
  depende principalmente del embed de `web/dist/` y de los assets de `kit/`; Vite production
  build ya minifica y hace tree-shaking (regla existente en `web/CLAUDE.md`).
- **`CGO_ENABLED=0`**: compilación estática; el binario no necesita libc del sistema. Elimina
  un vector de incompatibilidad en distribución.
- **Paralelismo en GoReleaser**: GoReleaser compila los 4 binarios en paralelo por defecto.
  No configurar `parallelism: 1` salvo que haya problemas de recursos en el runner de Actions.
- **`npm ci` en CI**: usa el lockfile para installs reproducibles y más rápidas que
  `npm install`.
- **Descarga selectiva en el instalador**: el script descarga solo el asset de la plataforma
  detectada (no los 4 tar.gz); `checksums.txt` es el único archivo adicional.
- **Sin caching de binarios instalados**: el instalador es stateless; no cachea descargas
  previas. Si se quiere evitar re-descargas, el usuario usa `--version` con la versión ya
  instalada y sin `--force`.

---

## 18. Restricciones

El agente no debe:

- Compilar desde fuentes en `install.sh` (no requerir Go en la máquina del usuario).
- Soportar Windows en V1: ni añadir `.exe`, ni scripts PowerShell, ni manifests de Scoop o
  Chocolatey.
- Usar `sudo` en ningún punto del instalador.
- Añadir dependencias al script más allá de `curl`/`wget`, `tar`, `shasum`/`sha256sum`,
  `grep`, `sed`, `mktemp`, `uname`, `install` (todos disponibles en macOS y Linux estándar).
- Usar `jq` (no se puede asumir instalado en el sistema del usuario).
- Modificar `README.md` (ese cambio pertenece al spec `rewrite-public-readme-humanized`).
- Crear ni proponer el contenido de `LICENSE` (decisión del usuario). TBD — ver Open
  questions §1.
- Pushear tags, crear GitHub Releases ni hacer el repo público como parte de la
  implementación de este spec.
- Editar manualmente `cli/internal/scaffold/assets/` (esos archivos se regeneran con
  `go generate`).
- Usar syntax bash 4+ exclusiva (`declare -A` para arrays asociativos, `mapfile`, etc.):
  compatibilidad con bash 3.2 es obligatoria.
- Hardcodear la versión del binario en el script (siempre resolución dinámica via API o
  `--version`).
- Modificar `.vector/config.json`, `kitVersion`, ni ningún archivo de estado del board
  (este spec solo toca `main.go` en el módulo Go y archivos nuevos de pipeline/scripts/docs).
- Añadir Homebrew tap, Docker images ni snapcraft en `.goreleaser.yml`.
- Configurar GPG signing en V1.
- Refactorizar código de `main.go` más allá del cambio `const → var version`.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `scripts/install.sh` — script bash 3.2+; `bash -n` limpio; plataforma no soportada
      (incluye Windows) abortada con mensaje accionable; normalización `aarch64→arm64` y
      `x86_64→amd64` funcionando; SHA256 verificado antes de instalar; flags `--version`,
      `--dry-run`, `--force` funcionando; `VECTOR_INSTALL_DIR` respetado; binary instalado
      con modo 0755; sugerencia de PATH emitida si `$INSTALL_DIR` no está en `$PATH`.
- [ ] `.goreleaser.yml` — `goreleaser check` pasa sin errores; 4 targets (darwin/linux ×
      amd64/arm64); `CGO_ENABLED=0`; ldflags inyectan `main.version`; `checksums.txt` SHA256
      generado; `name_template` del archive alineado con lo que construye `install.sh`;
      `release.github.owner: mcampbellr`.
- [ ] `.github/workflows/release.yml` — activado por `v*` tags; pasos en orden correcto (web
      build → copy dist → go generate → go test → goreleaser); `fetch-depth: 0`;
      `permissions: contents: write`; `GITHUB_TOKEN` configurado.
- [ ] `cli/cmd/vector/main.go` — `const version` cambiado a `var version = "dev"` en la
      línea verificada; `go build -ldflags "-X main.version=vX.Y.Z" ./cmd/vector && vector
      version` devuelve `vX.Y.Z`; build sin ldflags devuelve `"dev"`.
- [ ] `docs/install.md` — documentación en español; mención explícita de la restricción del
      repo privado; tabla de flags del script; instrucción de PATH; referencia cruzada a
      `README.md`.
- [ ] `go -C cli test ./...` verde (incluyendo `TestAssetsMatchKit` y regresiones del cambio
      `const→var`).
- [ ] Criterios de éxito de §8 completados y verificados localmente.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Verifiqué la línea exacta de `const version` en `cli/cmd/vector/main.go` antes de
      modificar (actualmente línea 26; confirmar).
- [ ] El `name_template` del archive en `.goreleaser.yml` produce el mismo string que
      construye `install.sh` (ej. `vector_0.1.0_darwin_arm64.tar.gz`). Validado manualmente.
- [ ] El script es compatible con bash 3.2+: sin `declare -A`, sin `-r` de `sed` GNU, sin
      herestrings avanzados, sin `mapfile`.
- [ ] El script no usa `jq`, no usa `sudo`, y tiene un trap `EXIT` que limpia el temporal.
- [ ] `bash -n scripts/install.sh` sale con código 0.
- [ ] `goreleaser check` pasa sin errores.
- [ ] Los pasos del workflow están en el orden correcto: checkout → setup-go → setup-node →
      web build → copy dist → go generate → go test → goreleaser.
- [ ] El workflow incluye `fetch-depth: 0` en el checkout y `permissions: contents: write`.
- [ ] La inyección de versión funciona: `go -C cli build -ldflags "-X main.version=vX.Y.Z"
      ./cmd/vector && vector version` devuelve `vX.Y.Z`.
- [ ] El fallback `"dev"` funciona: `go -C cli build ./cmd/vector && vector version` devuelve
      `"dev"`.
- [ ] `go -C cli test ./...` verde tras el cambio `const → var`.
- [ ] No toqué `README.md`, ni creé `LICENSE`, ni edité assets de scaffold manualmente, ni
      modifiqué `kitVersion` del config.
- [ ] No configuré Windows, GPG, Homebrew, Docker en `.goreleaser.yml`.
- [ ] `docs/install.md` menciona explícitamente que el `curl|sh` público requiere que el repo
      sea público (TBD — ver Open questions §1).
- [ ] No hice push de tags, no creé GitHub Releases, no hice el repo público como parte de
      este spec.
- [ ] No dejé logs temporales ni TODOs sin justificar.
- [ ] `release.github.owner` en `.goreleaser.yml` es `mcampbellr` (remote de GitHub), no
      `mariocampbell` (módulo Go).

---

## Open questions

1. **Repo público + LICENSE**: hacer el repo `github.com/mcampbellr/vector` público y añadir
   un archivo `LICENSE` son los requisitos de habilitación del `curl|sh` anónimo público.
   ¿Cuándo se toma esta decisión? ¿Qué licencia (MIT, Apache 2.0, otra)? Hasta que esto
   ocurra, el pipeline es "armado pero no en vivo" para el público, y `docs/install.md` debe
   advertirlo explícitamente. **Este punto bloquea el criterio de éxito del `curl|sh`
   anónimo** y la utilidad pública de `docs/install.md`.

2. **URL estable de `install.sh` para el usuario final**: mientras el repo sea privado, la
   raw URL de GitHub (`https://raw.githubusercontent.com/mcampbellr/vector/main/scripts/install.sh`)
   no funciona anónimamente. Opciones para la URL pública estable al lanzamiento: (a) raw
   GitHub URL directa (simple; cambia si se mueve el archivo o el repo); (b) dominio
   personalizado con redirect (`vector.sh/install` o similar; más estable pero requiere
   infraestructura adicional). ¿Se necesita planificar la URL estable antes del lanzamiento o
   se decide cuando el repo sea público?

3. **Primer tag de release y alineación de semver**: ¿cuál es el primer tag de release
   (`v0.1.0`, `v0.0.1`, otro)? El `kitVersion` en `.vector/config.json` es actualmente
   `"0.0.1-dev"` (string no-semver). ¿El primer release tag debe alinearse con esa versión
   (ej. `v0.0.1`) o empezar en `v0.1.0` marcando el primer release público? Esto afecta la
   decisión del usuario sobre qué tag pushear para activar el pipeline por primera vez.

4. **Pin de versión del GoReleaser Action**: el workflow usa `goreleaser/goreleaser-action@vX`.
   Recomendación: pinnar al major estable actual (`@v6`) en vez de `latest`, para builds
   reproducibles. La versión exacta del Action (y la versión de GoReleaser que instala) se fija
   al implementar, verificándola contra la release page de GoReleaser. Decisión menor, no
   bloquea el resto del spec.

> **Nota sobre discrepancia de identidad**: el módulo Go tiene path `github.com/mariocampbell/vector`
> (verificado en `cli/go.mod` y en los imports de `main.go`) pero el remote de GitHub es
> `https://github.com/mcampbellr/vector.git` (verificado en `.git/config`). Son distintos. El
> implementador no debe cambiar el módulo Go; GoReleaser usa el owner del remote de GitHub
> (`mcampbellr`). Verificar `git remote -v` antes de configurar `release.github.owner` en
> `.goreleaser.yml`.
