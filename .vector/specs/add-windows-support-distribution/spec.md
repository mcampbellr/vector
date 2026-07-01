# Spec: Agregar soporte Windows a GoReleaser e instalador PowerShell

## 1. Objetivo

Construir soporte de distribución Windows de primera clase para Vector: extender el pipeline de
GoReleaser para producir binarios `windows/amd64` y `windows/arm64` empaquetados en `.zip`, y
crear `scripts/install.ps1` — un instalador PowerShell 5.1+ equivalente idiomático a
`scripts/install.sh` — de modo que un usuario Windows pueda instalar Vector con un solo one-liner
sin instalar Go, Node ni ningún toolchain adicional.

Esta feature permite que un **desarrollador en Windows** pueda **instalar Vector con un one-liner
PowerShell** y obtener el mismo binario compilado y verificado que los usuarios de macOS y Linux,
cerrando la brecha de distribución sin impactar el pipeline Unix ni el código Go.

El problema concreto: hoy `.goreleaser.yml` declara `goos: [darwin, linux]` (líneas 30-32) y el
comentario de cabecera en línea 4 dice explícitamente "no Windows". `scripts/install.sh` aborta
en línea 105 con `"Windows is not supported in V1."`. No existe ningún path de instalación para
usuarios Windows.

## 2. Alcance

### Incluido en esta fase

- Extensión de `builds.goos` en `.goreleaser.yml` para incluir `windows` (cross-compilación con
  `CGO_ENABLED=0` ya configurado en línea 27).
- Refactorización del bloque `archives` de `.goreleaser.yml` en dos secciones discriminadas por
  `goos`: id `unix` (formatos `[tar.gz]`, goos `[darwin, linux]`) e id `windows` (formatos
  `[zip]`, goos `[windows]`). El `name_template` permanece invariante.
- Adición de `release.extra_files` en `.goreleaser.yml` para publicar `scripts/install.ps1` como
  asset del release (necesario para la URL `releases/latest/download/install.ps1`).
- Actualización del comentario de cabecera de `.goreleaser.yml` (línea 4) para eliminar "no
  Windows" y ajustar el conteo de binarios de 4 a 6.
- Creación de `scripts/install.ps1`: instalador PowerShell 5.1+ con paridad de funcionalidad con
  `install.sh`: verificación de versión PS, detección de arch, resolución de versión vía GitHub
  API, descarga con timeout, verificación SHA256, extracción `.zip`, instalación, hint de PATH,
  limpieza en `try/finally`.
- Flags: `--version <tag>`, `--dry-run`, `--force`. Env vars: `VECTOR_INSTALL_DIR`,
  `GITHUB_TOKEN`, `DEBUG`.
- Actualización de `README.md`: subsección Windows con el one-liner `irm | iex` (solo para
  latest), método de dos pasos para pinear versión, hint de PATH, y actualización de la lista de
  plataformas soportadas.
- Revisión de `.github/workflows/release.yml` (sin cambios esperados).

### Fuera de scope

- `winget`, Chocolatey, Scoop ni ningún package manager (roadmap V2).
- Windows 7 y PowerShell < 5.1.
- Firma de código del binario Windows (code signing / Windows Defender SmartScreen).
- Validación del formato del tag `--version` (pass-through; GitHub API rechaza si el release no
  existe, igual que `install.sh`).
- Modificaciones a `scripts/install.sh` (la detección/error de Windows en línea 105 permanece; el
  bash installer no corre en Windows nativo).
- Localización o traducción de mensajes del instalador.
- Modificación de la lógica del CLI Go, del estado del board ni de ningun componente web.

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Build pipeline: **GoReleaser v2** (declarado en `.goreleaser.yml` línea 8: `version: 2`; acción
  `goreleaser/goreleaser-action@v6` en `release.yml` línea 52)
- Lenguaje del binario: **Go 1.26** (declarado en `cli/go.mod`; cross-compilación vía
  `GOOS=windows CGO_ENABLED=0` sin toolchain C ni mingw)
- Instalador Unix: **bash 3.2+** (`scripts/install.sh`, 367 líneas — referencia de flujo y
  mensajes)
- Instalador Windows: **PowerShell 5.1+** (`scripts/install.ps1`, nuevo; disponible en Windows
  10 / Windows Server 2016+)
- CI: **GitHub Actions** (`ubuntu-latest`), sin cambios requeridos para cross-compilar Windows

### Versiones relevantes

- Go: **1.26** (`cli/go.mod`; setup-go usa `go-version-file: cli/go.mod` en `release.yml`
  línea 27)
- GoReleaser: **v2** (`version: 2` en `.goreleaser.yml` línea 8; `~> v2` en `release.yml`
  línea 54)
- PowerShell mínimo soportado: **5.1** (`Expand-Archive` y `Get-FileHash` son nativos desde 5.1;
  Windows 10 incluye 5.1 por defecto)

### Patrones existentes a respetar

- **`name_template` invariante**: `vector_{{ .Version }}_{{ .Os }}_{{ .Arch }}` (`.goreleaser.yml`
  línea 40). Se mantiene idéntico para Windows, produciendo
  `vector_<VER>_windows_amd64.zip` y `vector_<VER>_windows_arm64.zip`.
- **`CGO_ENABLED=0`**: `.goreleaser.yml` línea 27. Obligatorio para cross-compilación sin
  dependencias C; no se cambia.
- **`binary: vector`**: `.goreleaser.yml` línea 25. GoReleaser añade `.exe` automáticamente en
  builds Windows; el campo `binary` no cambia.
- **checksums.txt**: formato `<SHA256_UPPERCASE>  <FILENAME>` (dos espacios), generado con
  `algorithm: sha256` (`.goreleaser.yml` líneas 42-44). El PS script verifica contra este
  formato con `Get-FileHash -Algorithm SHA256`.
- **No deps externas**: `install.sh` parsea JSON con `grep`/`sed` sin `jq`; `install.ps1` usa
  `ConvertFrom-Json` nativo de PS 5.1. Solo cmdlets nativos.
- **Mensajes del instalador en inglés**: convención de `install.sh`; `install.ps1` sigue la
  misma convención (formato `==> mensaje` para progreso, `Error: mensaje` para errores).
- **Comercialización día 0**: instalador de un paso como requisito de primera clase
  (`architecture/distribution-packaging.md`).
- **`-UseBasicParsing`**: flag obligatorio en `Invoke-WebRequest` para compatibilidad con Windows
  Server Core (sin motor de IE/MSHTML).

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [x] `.goreleaser.yml` en la raíz del repo con pipeline Unix funcional (builds darwin+linux,
      archives tar.gz, checksums, release a GitHub)
- [x] `scripts/install.sh` (367 líneas) como referencia de flujo y mensajes del instalador
- [x] `.github/workflows/release.yml` con pipeline de release completo (build web → embed →
      scaffold → tests → GoReleaser)
- [x] `cli/go.mod` declarando Go 1.26 (compatible con cross-compilación Windows; `CGO_ENABLED=0`
      ya configurado)
- [x] Binario `vector` que ya compila con `CGO_ENABLED=0` (sin dependencias CGO; pre-requisito
      para cross-compilación a Windows sin toolchain C)
- [x] `README.md` con sección `## Installation` y subsección `### Install script` existentes

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta. No
debe inventar contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

**Extensión del pipeline existente**: GoReleaser es agnóstico de plataforma. Agregar `windows` al
bloque `builds.goos` es suficiente para que el mismo workflow de CI produzca 6 binarios (2 nuevos
para Windows, de los 4 actuales). El runner `ubuntu-latest` incluye el toolchain de Windows de Go
y cross-compila sin cambios. Las dos secciones `archives` discriminan por `goos` para entregar
`.tar.gz` a Unix y `.zip` a Windows.

`install.ps1` sigue el mismo flujo funcional que `install.sh` adaptado idiomáticamente a
PowerShell: mismos flags, mismas env vars, mismo orden de pasos, mismos mensajes de progreso.

### Capas afectadas

- `cli/` (código fuente Go): **no** — Go compila para Windows sin modificaciones con
  `CGO_ENABLED=0`.
- Release pipeline (`.goreleaser.yml`): **sí** — extensión de `goos`, refactor de `archives`,
  adición de `extra_files`.
- CI (`.github/workflows/release.yml`): **revisar, sin cambios esperados** — GoReleaser en
  `ubuntu-latest` cross-compila todos los targets.
- `scripts/`: **sí** — archivo nuevo `install.ps1`; `install.sh` no se modifica.
- `README.md`: **sí** — subsección Windows en la sección Installation.

### Flujo de `install.ps1`

1. Verificar `$PSVersionTable.PSVersion` ≥ 5.1 al inicio; abortar con mensaje claro si falla.
2. Si `$env:DEBUG -eq '1'`, activar `Set-PSDebug -Trace 1`.
3. Parsear parámetros: `--version <tag>`, `--dry-run`, `--force`. Abortar si `--version` viene
   sin argumento.
4. Detectar arquitectura vía `$env:PROCESSOR_ARCHITECTURE`: `AMD64`→`amd64`, `ARM64`→`arm64`;
   abortar con error accionable para cualquier otro valor (coherente con `install.sh` línea 105).
5. Crear directorio temporal; registrar limpieza en bloque `finally`.
6. Resolver versión: si `--version` especificado, usarla directamente; si no, llamar a
   `https://api.github.com/repos/mcampbellr/vector/releases/latest` y parsear `tag_name` con
   `ConvertFrom-Json` (en bloque `try/catch` para capturar respuestas no JSON). Soportar
   `GITHUB_TOKEN` vía header `Authorization: Bearer`.
7. Construir nombre del asset: `vector_<VER>_windows_<ARCH>.zip` donde `<VER>` es el tag sin
   la `v` inicial.
8. Preparar directorio de instalación: `$env:VECTOR_INSTALL_DIR` o
   `$env:LOCALAPPDATA\Programs\Vector`. Crear si no existe; verificar permisos de escritura.
9. Corto-circuitar si la misma versión ya está instalada (comparar output de `vector.exe version`
   con el tag resuelto), a menos que `--force`.
10. Descargar asset `.zip` vía `Invoke-WebRequest -UseBasicParsing -TimeoutSec 300`; soportar
    `GITHUB_TOKEN`. Mapear HTTP 401, 403, 404, 429, 5xx a mensajes accionables.
11. Descargar `checksums.txt` de la misma release.
12. Verificar SHA256 con `Get-FileHash -Algorithm SHA256`; comparar contra la línea de
    `checksums.txt` que matchea el filename. Abortar si no coinciden.
13. Extraer `.zip` con `Expand-Archive -Force` al directorio temporal.
14. Verificar que `vector.exe` existe en el directorio extraído; abortar si falta.
15. Copiar `vector.exe` al directorio de instalación.
16. Imprimir mensaje de éxito con la versión instalada.
17. Si el directorio de instalación no está en `$env:PATH`, imprimir hint de PATH (no modificar
    el PATH automáticamente).
18. Bloque `finally`: eliminar el directorio temporal.

### Ubicación de archivos nuevos

```txt
scripts/
  install.sh        # existente — no se modifica
  install.ps1       # NUEVO
```

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| `.goreleaser.yml` | MODIFICAR | Agregar `windows` a `goos`; refactorizar `archives` en dos secciones (unix/windows); agregar `extra_files` con `install.ps1`; actualizar comentario cabecera. | — |
| `scripts/install.ps1` | NUEVO | Instalador PowerShell 5.1+ equivalente a `install.sh`: detección de arch, resolución de versión, SHA256, extracción, instalación, hint de PATH. | `scripts/install.sh` (flujo, mensajes y flags de referencia) |
| `README.md` | MODIFICAR | Agregar subsección Windows con one-liner `irm \| iex`, hint de PATH y actualización de plataformas soportadas. | Subsección "Install script" existente en `README.md` |
| `.github/workflows/release.yml` | REVISAR | Confirmar que no requiere cambios (GoReleaser cross-compila Windows en `ubuntu-latest`). | — |

### Detalle por archivo

#### .goreleaser.yml

Acción: MODIFICAR

Cambios requeridos:

1. **Comentario de cabecera (líneas 2 y 4)**: actualizar para reflejar el soporte Windows.
   - Línea 2: cambiar `"Builds 4 precompiled binaries (darwin/linux × amd64/arm64)"` a
     `"Builds 6 precompiled binaries (darwin/linux/windows × amd64/arm64)"`.
   - Línea 4: cambiar
     `"# checksums.txt. No Homebrew/Docker/snapcraft, no Windows, no GPG (V1 scope)."`
     a
     `"# checksums.txt. No Homebrew/Docker/snapcraft, no GPG (V1 scope)."`.

2. **Bloque `builds.goos`** (actualmente líneas 30-32): agregar `windows`:
   ```yaml
   goos:
     - darwin
     - linux
     - windows
   ```

3. **Bloque `archives`** (actualmente líneas 37-40): reemplazar la sección única por dos
   entradas discriminadas por `goos`:
   ```yaml
   archives:
     - id: unix
       goos: [darwin, linux]
       formats: [tar.gz]
       name_template: "vector_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
     - id: windows
       goos: [windows]
       formats: [zip]
       name_template: "vector_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
   ```

4. **Bloque `release`** (actualmente líneas 46-50): agregar `extra_files` para incluir
   `install.ps1` como asset del release (necesario para la URL
   `releases/latest/download/install.ps1`):
   ```yaml
   release:
     github:
       owner: mcampbellr
       name: vector
     prerelease: auto
     extra_files:
       - glob: scripts/install.ps1
   ```

Restricciones:
- No cambiar `checksum.algorithm` ni el `name_template`.
- No cambiar `builds.binary` (sigue siendo `vector`; GoReleaser añade `.exe` automáticamente
  en Windows).
- No modificar `before.hooks`, `ldflags`, `snapshot` ni el bloque `checksum`.
- No agregar Homebrew, Docker, snapcraft ni otros targets.

#### scripts/install.ps1

Acción: NUEVO

Debe implementar:

- **Verificación PS 5.1+** al inicio (antes de cualquier otra operación):
  ```powershell
  if ($PSVersionTable.PSVersion.Major -lt 5 -or
      ($PSVersionTable.PSVersion.Major -eq 5 -and $PSVersionTable.PSVersion.Minor -lt 1)) {
      Write-Error "Error: PowerShell 5.1+ required."; exit 1
  }
  ```
- **DEBUG**: si `$env:DEBUG -eq '1'`, activar `Set-PSDebug -Trace 1`.
- **Parseo de parámetros**: `--version <tag>`, `--dry-run`, `--force`. Abortar con mensaje
  accionable si `--version` viene sin argumento.
- **Funciones helper**: `Write-Info` (`==> <msg>`), `Write-Err` (aborta con `Error: <msg>`),
  `Invoke-Dry` (imprime `[dry-run] <msg>` y retorna `$true` en dry-run).
- **Detección de arch**: `$env:PROCESSOR_ARCHITECTURE` → `AMD64`→`"amd64"`,
  `ARM64`→`"arm64"`; abortar con mensaje accionable para cualquier otro valor.
- **Resolución de versión**: si `--version` especificado, usar directamente. Si no, llamar a
  `https://api.github.com/repos/mcampbellr/vector/releases/latest` con
  `Invoke-WebRequest -UseBasicParsing`, parsear `tag_name` con `ConvertFrom-Json` dentro de un
  bloque `try/catch` (ver edge case de JSON malformado). Si `$env:GITHUB_TOKEN` está presente,
  añadir header `Authorization: Bearer $env:GITHUB_TOKEN`.
- **Nombre del asset**: `"vector_$($TAG.TrimStart('v'))_windows_$ARCH.zip"`.
- **Directorio de instalación**: `$env:VECTOR_INSTALL_DIR` si está definido, o
  `"$env:LOCALAPPDATA\Programs\Vector"` como default. Crear con `New-Item -ItemType Directory
  -Force`. Verificar permisos de escritura.
- **Corto-circuito**: si `vector.exe` ya existe en el directorio y reporta la misma versión,
  abortar con info (salvo `--force`). Análogo a `maybe_skip_if_present` de `install.sh`
  líneas 338-350.
- **Descarga**: `Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $dest -TimeoutSec 300`.
  Mapear códigos HTTP: 401 → token inválido, 403 → rate limit, 404 → asset no encontrado,
  429 → rate limit HTTP, 5xx → server error.
- **Verificación SHA256**:
  ```powershell
  $computed = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash
  # comparar contra la línea de checksums.txt que matchea el filename
  ```
  Abortar si no coinciden.
- **Extracción**: `Expand-Archive -Path $assetPath -DestinationPath $tmpDir -Force`.
- **Verificación de binario**: confirmar que `vector.exe` existe en `$tmpDir` tras la
  extracción; abortar con mensaje accionable si falta.
- **Instalación**: `Copy-Item -Path "$tmpDir\vector.exe" -Destination $installDir -Force`.
- **Hint de PATH**: si `$installDir` no está en `$env:PATH`, imprimir instrucción de
  `SetEnvironmentVariable`; no modificar el PATH automáticamente.
- **Limpieza**: directorio temporal eliminado en bloque `finally` con
  `Remove-Item -Recurse -Force`.
- **Dry-run**: todas las acciones destructivas se verifican con `Invoke-Dry`; sin dry-run
  ejecutan la operación real.

Debe seguir como referencia:
- `scripts/install.sh` (orden del flujo, nombres de env vars, flags, mensajes de progreso y
  error, patrón de corto-circuito si ya instalado)

No debe incluir:
- Modificación automática del PATH del sistema.
- Dependencias externas (solo cmdlets nativos de PS 5.1+: `Invoke-WebRequest`,
  `ConvertFrom-Json`, `Get-FileHash`, `Expand-Archive`).
- Soporte para PowerShell < 5.1 ni Windows 7.
- Secuencias de escape ANSI/color en los mensajes.

#### README.md

Acción: MODIFICAR

Cambios requeridos:

- En la sección `## Installation`, agregar una subsección `### Windows` con:
  - El one-liner idiomático como método principal (instala la última versión disponible):
    ```powershell
    powershell -c "irm https://github.com/mcampbellr/vector/releases/latest/download/install.ps1 | iex"
    ```
  - Nota sobre la variante en dos pasos para entornos que restringen `iex`, o para inspeccionar
    el script antes de ejecutarlo:
    ```powershell
    # Alternative: download and inspect before running
    irm https://github.com/mcampbellr/vector/releases/latest/download/install.ps1 -OutFile install.ps1
    .\install.ps1
    ```
  - Para instalar una versión específica, usar el método de dos pasos — el one-liner `irm | iex`
    no admite argumentos (en PowerShell 5.1, `-Command "string"` descarta los tokens tras la
    comilla de cierre y no los reenvía a la expresión evaluada):
    ```powershell
    irm https://github.com/mcampbellr/vector/releases/latest/download/install.ps1 -OutFile install.ps1
    .\install.ps1 --version v0.1.0
    ```
  - Mención del directorio de instalación default: `%LOCALAPPDATA%\Programs\Vector\vector.exe`.
  - Hint para agregar al PATH.
- Actualizar la línea `"Supported platforms: macOS and Linux on amd64 and arm64."` para incluir
  Windows (texto actual en `README.md` línea 55).

Restricciones:
- No cambiar el flujo ni los comandos de instalación Unix.
- No documentar winget (fuera de scope).
- No crear secciones nuevas de referencia de CLI.

#### .github/workflows/release.yml

Acción: REVISAR

Verificar que:
- `goreleaser/goreleaser-action@v6` en `ubuntu-latest` soporta cross-compilación a
  `windows/amd64` y `windows/arm64` con `CGO_ENABLED=0`. Go incluye el toolchain de Windows;
  no requiere mingw ni cgo.
- No hay pasos que asuman exclusivamente Linux/macOS para producir artefactos.
- `go-version-file: cli/go.mod` (línea 27 de `release.yml`) declara Go 1.26; compatible sin
  cambios.

Sin cambios funcionales esperados en este archivo. Nota para el implementador: el comentario de
cabecera de `release.yml` en línea 5 dice `"(4 platform archives + checksums.txt)"` — tras este
cambio serán 6 archives. Actualizar ese comentario (cambio cosmético, no funcional) al mismo
tiempo que se edita `.goreleaser.yml`.

---

## 7. API Contract

No aplica — este spec no introduce ni modifica ningún endpoint HTTP del servidor Vector.

El contrato relevante es externo: el **naming de assets en GitHub Releases** y el **formato de
`checksums.txt`**:

- **Nombre de asset Windows**: `vector_<VERSION>_windows_<ARCH>.zip` donde `<VERSION>` es el
  tag sin la `v` inicial y `<ARCH>` es `amd64` o `arm64`. Producido por el `name_template` de
  GoReleaser (`.goreleaser.yml` línea 40), que GoReleaser aplica a todos los builds incluyendo
  los nuevos targets Windows.
- **Nombre de asset instalador**: `install.ps1` (asset adicional vía `extra_files`). URL de
  descarga: `https://github.com/mcampbellr/vector/releases/latest/download/install.ps1`.
- **Formato de `checksums.txt`**: `<SHA256_UPPERCASE>  <FILENAME>` (dos espacios), generado por
  GoReleaser con `algorithm: sha256` (`.goreleaser.yml` línea 44). Los archives Windows
  aparecen en este archivo con su nombre completo (incluida extensión `.zip`).
- **`install.ps1` NO está en `checksums.txt`**: solo los archives binarios están checksumados
  por GoReleaser; `install.ps1` es un script de texto distribuido como `extra_files`.

No se infieren campos adicionales ni se renombran propiedades del contrato existente.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] Hacer push de un tag `v*` dispara el pipeline de release y GoReleaser produce 6 archives
      (darwin_amd64, darwin_arm64, linux_amd64, linux_arm64, windows_amd64, windows_arm64) más
      `checksums.txt` e `install.ps1` como assets del release.
- [ ] Los archives Unix son `.tar.gz`; los Windows son `.zip`. Todos nombrados
      `vector_<VER>_<OS>_<ARCH>.<ext>`.
- [ ] `checksums.txt` incluye entradas para los 6 archives (Unix y Windows).
- [ ] `install.ps1` está disponible en
      `https://github.com/mcampbellr/vector/releases/latest/download/install.ps1`.
- [ ] El one-liner `powershell -c "irm https://github.com/mcampbellr/vector/releases/latest/download/install.ps1 | iex"`
      ejecutado en Windows instala `vector.exe` en `%LOCALAPPDATA%\Programs\Vector\`.
- [ ] `install.ps1 --dry-run` imprime todas las acciones con prefijo `[dry-run]` sin descargar
      ni instalar nada.
- [ ] `install.ps1 --version v0.1.0` instala exactamente esa versión (invocado vía dos pasos).
- [ ] `install.ps1 --force` reinstala aunque la misma versión esté presente.
- [ ] `install.ps1` con `$env:GITHUB_TOKEN` seteado incluye el header `Authorization: Bearer`
      en todas las peticiones HTTP.
- [ ] En arquitectura no soportada (ej. x86/32-bit), `install.ps1` aborta con mensaje
      accionable sin instalar nada.
- [ ] En PowerShell < 5.1, `install.ps1` aborta inmediatamente con `"PowerShell 5.1+ required."`.
- [ ] Si el SHA256 no coincide, `install.ps1` aborta antes de copiar el binario.
- [ ] El comentario de cabecera del `.goreleaser.yml` ya no contiene "no Windows".
- [ ] `goreleaser check` pasa sin errores sobre el `.goreleaser.yml` modificado.
- [ ] `README.md` incluye la subsección Windows con el one-liner y el hint de PATH.
- [ ] `go -C cli test ./...` sigue verde (sin cambios en código Go).

### Tests requeridos

- [ ] **Validación de GoReleaser**: `goreleaser check` — el `.goreleaser.yml` modificado pasa
      la validación de configuración sin errores ni warnings.
- [ ] **Cross-compilación amd64**: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go -C cli build
      -o /dev/null ./cmd/vector` — sin errores.
- [ ] **Cross-compilación arm64**: `GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go -C cli build
      -o /dev/null ./cmd/vector` — sin errores.
- [ ] **`install.ps1 --dry-run`**: ejecutado en PowerShell 5.1+, no descarga ni instala;
      imprime acciones con prefijo `[dry-run]`.
- [ ] **Suite Go**: `go -C cli test ./...` sin cambios — debe seguir verde.
- [ ] **Snapshot de GoReleaser** (opcional, requiere Node para el build web):
      `goreleaser release --snapshot --clean` produce los 6 archives sin errores de config.

### Comandos de verificación

```bash
# Validar configuración de GoReleaser (no requiere build)
goreleaser check

# Cross-compilación Windows desde macOS/Linux
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go -C cli build -o /dev/null ./cmd/vector
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go -C cli build -o /dev/null ./cmd/vector

# Suite de tests Go (sin cambios; debe seguir verde)
go -C cli test ./...
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

No hay formularios ni UI gráfica. Los criterios de UX aplican al comportamiento del instalador
PowerShell como interfaz de línea de comandos.

### Mensajes de progreso

Cada paso principal imprime un mensaje con el prefijo `==>` (misma convención que `install.sh`
función `info()` en línea 38):

- `==> Detecting architecture...` / `==> Detected: windows <ARCH>`
- `==> Resolving latest version...` / `==> Latest version: <TAG>` (o
  `==> Using pinned version: <TAG>` con `--version`)
- `==> Downloading <ASSET>...`
- `==> Downloading checksums.txt...`
- `==> Verifying checksum...` / `==> Checksum OK`
- `==> Installing to <DIR>...`
- `==> vector <VER> installed successfully`

### Errores

Todos los errores abortan con `Write-Error` usando el prefijo `Error:` seguido de un mensaje
accionable en inglés. Nunca dejar el script en estado inconsistente sin limpiar el temp dir.

### Dry-run

Con `--dry-run`, cada operación destructiva (descarga, extracción, copia del binario) se
reemplaza por una línea `[dry-run] Would <acción>`. El script termina sin efecto en disco.

### Flags

- `--version <tag>`: pin a una versión específica. Pass-through; GitHub API rechaza si no existe.
  Solo disponible vía el método de dos pasos (el one-liner `irm | iex` no reenvía argumentos).
- `--dry-run`: simular sin efectos en disco.
- `--force`: omitir el corto-circuito de "ya instalado".

### PATH hint

Al terminar la instalación, si el directorio de instalación no está en `$env:PATH`, imprimir:

```
Add <DIR> to your PATH:
  [System.Environment]::SetEnvironmentVariable("Path", "$env:Path;<DIR>", "User")
```

No modificar el PATH automáticamente.

### Accesibilidad

Mensajes en texto plano sin secuencias de escape ANSI ni color. Compatible con terminales
Windows clásicas (cmd.exe, PowerShell ISE, Windows Terminal).

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas; el agente no debe cuestionarlas ni cambiarlas:

- **Windows amd64 + arm64**: paridad con Unix. Go cross-compila ambas arquitecturas sin hardware
  ARM real (`architecture/distribution-packaging.md`).
- **Formato `.zip` para Windows, `.tar.gz` para Unix**: convención nativa de cada plataforma.
  Implementado con dos secciones `archives` en GoReleaser, discriminadas por `goos`. El campo
  `name_template` permanece invariante en ambas.
- **Fallback de arch en `install.ps1`: abortar con error accionable**. Si
  `$env:PROCESSOR_ARCHITECTURE` no es `AMD64` ni `ARM64`, el script aborta. No se asume
  `amd64` por defecto. Coherente con `install.sh` línea 105.
- **`--version` pass-through**: no se valida el formato del tag. GitHub API rechaza con 404 si
  el release no existe. Mismo comportamiento que `install.sh` (línea 63-66). El flag solo es
  utilizable vía el método de dos pasos (ver §6, §9).
- **PowerShell < 5.1: abortar** con mensaje `"PowerShell 5.1+ required."`. Windows 7 no es
  un objetivo soportado.
- **One-liner idiomático en README para instalar latest**: `powershell -c "irm https://github.com/mcampbellr/vector/releases/latest/download/install.ps1 | iex"`.
  En PowerShell 5.1, `-Command "string"` descarta los tokens tras la comilla de cierre; por eso
  el one-liner solo sirve para latest. Para `--version` se usa el método de dos pasos.
  El one-liner requiere que `install.ps1` esté publicado como release asset via
  `release.extra_files` en GoReleaser.
- **Verificación SHA256 con `Get-FileHash -Algorithm SHA256`** (nativo PS 5.1+), preferido sobre
  `certutil` — output más limpio y sin dependencia de certificados del sistema.
- **`Invoke-WebRequest` con `-UseBasicParsing`**: obligatorio para compatibilidad con Windows
  Server Core (sin motor de IE/MSHTML); también acelera la descarga.
- **Directorio de instalación predeterminado**: `$env:LOCALAPPDATA\Programs\Vector` — no requiere
  permisos de administrador (UAC); equivalente conceptual al `~/.local/bin` de Unix.
- **No modificar el PATH automáticamente**: solo hint al usuario. Modificar el PATH del sistema
  requiere permisos especiales y puede tener efectos colaterales.
- **`.github/workflows/release.yml` sin cambios funcionales**: GoReleaser cross-compila para
  Windows en `ubuntu-latest` con `CGO_ENABLED=0`; no requiere runner Windows ni herramientas
  adicionales. El comentario de cabecera (4→6 archives) sí debe actualizarse.
- **No winget en este spec**: roadmap V2.
- **`scripts/install.sh` no se modifica**: es un instalador Unix; el mensaje de línea 105
  sobre Windows permanece correcto (los usuarios de Windows usan `install.ps1`).

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, no
implementarla.

---

## 11. Edge cases

### Detección de arquitectura en `install.ps1`

- `$env:PROCESSOR_ARCHITECTURE = "AMD64"` → mapear a `"amd64"`.
- `$env:PROCESSOR_ARCHITECTURE = "ARM64"` → mapear a `"arm64"`.
- Cualquier otro valor (incluido `"x86"`, `"IA64"`, o variable no definida) → abortar con:
  `"Unsupported architecture: <VALUE>. Supported: amd64 (AMD64), arm64 (ARM64)."`. No asumir
  ningún valor por defecto.

### PowerShell versión

- Si `PSVersion.Major < 5` o (`Major == 5` y `Minor < 1`) → abortar inmediatamente con
  `"PowerShell 5.1+ required."` antes de cualquier otra operación.

### Flag `--version`

- `--version v0.1.0` → usar el tag tal como se pasa; si el release no existe, GitHub API
  devuelve 404 y el script aborta con mensaje accionable.
- `--version` sin argumento → abortar con
  `"--version requires a tag argument (e.g. --version v0.1.0)."` (análogo a `install.sh`
  línea 65).

### Red y HTTP

- Timeout de descarga: `Invoke-WebRequest -TimeoutSec 300`; si falla, abortar con
  `"Connection to GitHub timed out. Try again later."`.
- HTTP 401: `"GitHub returned 401 Unauthorized. Check your GITHUB_TOKEN."`.
- HTTP 403: `"GitHub API rate limit hit. Try again later or use --version <tag>."`.
- HTTP 429: `"GitHub rate limit hit (429). Try again later or use --version <tag>."`.
- HTTP 404 para el asset: `"No prebuilt binary found for windows/<ARCH> in release <TAG>."`.
- HTTP 404 para `checksums.txt`: `"Could not download checksums.txt. Cannot verify integrity."`.
- HTTP 5xx: `"GitHub returned a server error (<CODE>). Try again later."`.

### Parseo de respuesta JSON

- Si la GitHub API devuelve contenido no JSON (p. ej. página de error de CDN o respuesta de
  proxy), `ConvertFrom-Json` lanza un error terminante. Envolver el parseo en `try/catch`:
  cualquier excepción durante el parseo → abortar con
  `"Failed to parse GitHub API response. Try again later."`.

### Verificación SHA256

- Si el filename del asset no aparece en ninguna línea de `checksums.txt` → abortar con
  `"Checksum verification failed for <FILENAME>. The download may be corrupt. Try again."`.
- Si el hash calculado con `Get-FileHash` no coincide con el esperado → mismo mensaje.

### Directorio de instalación

- `$env:VECTOR_INSTALL_DIR` apunta a un archivo (no directorio) → abortar con
  `"VECTOR_INSTALL_DIR (<PATH>) is a file, not a directory."`.
- Sin permisos de escritura en el directorio → abortar con
  `"No write permission in <DIR>. Set VECTOR_INSTALL_DIR to a writable path."`.

### Ya instalado

- Si `vector.exe` ya existe y reporta la misma versión que la resuelta, y no se pasa `--force`
  → corto-circuitar con `"vector <VER> is already installed (use --force to reinstall)"`.
- `--force` reinstala independientemente de la versión instalada.

### Binario ausente en el zip

- Si `Expand-Archive` no produce `vector.exe` en el directorio extraído → abortar con
  `"Archive <ASSET> did not contain a 'vector.exe' binary."`.

### Limpieza del directorio temporal

- El bloque `try/finally` garantiza que el directorio temporal se elimina incluso si el script
  aborta por error en cualquier punto. Análogo al `trap cleanup EXIT` de `install.sh`
  (líneas 48-53).

### GoReleaser con tres GOOS

- Las dos secciones `archives` deben cubrir exactamente los tres `goos` del bloque `builds`
  sin overlap: `[darwin, linux]` en id `unix` y `[windows]` en id `windows`.
- `goreleaser check` detectará cualquier gap o solapamiento de configuración.

---

## 12. Estados de UI requeridos

No aplica — este spec no introduce ni modifica componentes de interfaz gráfica. El board kanban,
la StandupView y el timeline no se ven afectados por este cambio.

Los únicos "estados" visibles son los mensajes de progreso y error del instalador CLI, definidos
en §9 (Criterios de UX) y §11 (Edge cases).

---

## 13. Validaciones

### Validaciones del instalador PowerShell (`install.ps1`)

| Condición | Regla | Mensaje de error |
|---|---|---|
| Versión de PowerShell | `PSVersion.Major >= 5` y, si `Major == 5`, `Minor >= 1` | `"PowerShell 5.1+ required."` |
| Arquitectura | `$env:PROCESSOR_ARCHITECTURE` es `AMD64` o `ARM64` | `"Unsupported architecture: <VALUE>. Supported: amd64 (AMD64), arm64 (ARM64)."` |
| `--version` con valor vacío | Si el flag está presente, debe tener argumento | `"--version requires a tag argument (e.g. --version v0.1.0)."` |
| SHA256 del asset | Hash calculado == entrada en `checksums.txt` (uppercase, dos espacios) | `"Checksum verification failed for <FILENAME>. The download may be corrupt. Try again."` |
| Binary en zip | `vector.exe` existe en `$tmpDir` tras `Expand-Archive` | `"Archive <ASSET> did not contain a 'vector.exe' binary."` |
| Directorio de instalación | Es directorio y tiene permisos de escritura | Ver §11 (edge cases de directorio) |

### Validaciones de GoReleaser

- `goreleaser check` debe pasar sin errores ni warnings sobre el `.goreleaser.yml` modificado.
- Las dos secciones `archives` cubren exactamente los tres `goos` del bloque `builds` sin
  overlap ni gap.

---

## 14. Seguridad y permisos

- **`GITHUB_TOKEN` nunca en logs**: el token se pasa como header HTTP
  (`Authorization: Bearer $env:GITHUB_TOKEN`); no se imprime en ningún mensaje, tampoco con
  `DEBUG=1`. Coherente con `install.sh` (líneas 141-145).
- **HTTPS obligatorio**: todas las peticiones usan `https://`; `Invoke-WebRequest` con
  `-UseBasicParsing` no permite downgrade de protocolo.
- **Verificación SHA256 es obligatoria** y no salteable. El script aborta si la verificación
  falla antes de copiar `vector.exe` al directorio de instalación.
- **Sin permisos de administrador**: la instalación en `%LOCALAPPDATA%\Programs\Vector` no
  requiere elevación (UAC).
- **No modificar el PATH del sistema automáticamente**: requeriría permisos elevados o
  modificar el perfil del usuario sin consentimiento explícito.
- **Limpieza de temp dir garantizada** en `finally`: no quedan archivos de descarga (incluido
  el `.zip` con el binario) tras la ejecución, exitosa o no.
- **`install.ps1` en el release como texto plano**: el script es inspeccionable antes de
  ejecutar (la variante en dos pasos del README lo facilita explícitamente).

---

## 15. Observabilidad y logging

- **`DEBUG=1`** (`$env:DEBUG -eq '1'`): activa `Set-PSDebug -Trace 1` para trazar cada línea
  ejecutada. Análogo al `set -x` de `install.sh` (líneas 25-27).
- **Progreso**: mensajes `==>` a stdout en cada paso principal.
- **Errores**: mensajes `Error:` vía `Write-Error` a stderr antes de `exit 1`.
- **Nada sensible en logs**: `GITHUB_TOKEN`, hashes intermedios y rutas de archivos temporales
  no se imprimen en condiciones normales; solo el hash final si la verificación falla
  (para diagnóstico).
- No se añade logging adicional al pipeline de CI.

---

## 16. i18n / textos visibles

No hay sistema de traducciones. Todos los textos del instalador `install.ps1` son strings
hardcodeados en inglés, igual que `install.sh`. Esta convención está establecida en el proyecto:
los mensajes del instalador están en inglés independientemente del idioma de la prosa del spec
(`SPEC_LANGUAGE` aplica a la documentación del spec, no a los mensajes del instalador).

Textos requeridos en `install.ps1`:

| Contexto | Texto (en inglés) |
|---|---|
| Inicio — detección de arch | `==> Detecting architecture...` |
| Arch detectada | `==> Detected: windows <ARCH>` |
| Resolviendo versión | `==> Resolving latest version...` |
| Versión resuelta | `==> Latest version: <TAG>` |
| Versión pinada | `==> Using pinned version: <TAG>` |
| Ya instalado | `==> vector <VER> is already installed (use --force to reinstall)` |
| Descargando asset | `==> Downloading <ASSET>...` |
| Descargando checksums | `==> Downloading checksums.txt...` |
| Verificando | `==> Verifying checksum...` |
| Verificación OK | `==> Checksum OK` |
| Instalando | `==> Installing to <DIR>...` |
| Éxito | `==> vector <VER> installed successfully` |
| Hint de PATH | `Add <DIR> to your PATH: [System.Environment]::SetEnvironmentVariable("Path", "$env:Path;<DIR>", "User")` |
| Error PS version | `PowerShell 5.1+ required.` |
| Error arch | `Unsupported architecture: <VALUE>. Supported: amd64 (AMD64), arm64 (ARM64).` |
| Error `--version` sin arg | `--version requires a tag argument (e.g. --version v0.1.0).` |
| Error 401 Unauthorized | `GitHub returned 401 Unauthorized. Check your GITHUB_TOKEN.` |
| Error 403 rate limit | `GitHub API rate limit hit. Try again later or use --version <tag>.` |
| Error 429 rate limit | `GitHub rate limit hit (429). Try again later or use --version <tag>.` |
| Error 404 asset | `No prebuilt binary found for windows/<ARCH> in release <TAG>.` |
| Error 404 checksums | `Could not download checksums.txt. Cannot verify integrity.` |
| Error timeout | `Connection to GitHub timed out. Try again later.` |
| Error red | `Failed to reach GitHub. Check your network connection and try again.` |
| Error JSON parse | `Failed to parse GitHub API response. Try again later.` |
| Error SHA256 | `Checksum verification failed for <FILENAME>. The download may be corrupt. Try again.` |
| Error binary ausente | `Archive <ASSET> did not contain a 'vector.exe' binary.` |
| Error dir es archivo | `VECTOR_INSTALL_DIR (<PATH>) is a file, not a directory.` |
| Error sin permisos | `No write permission in <DIR>. Set VECTOR_INSTALL_DIR to a writable path.` |

---

## 17. Performance

- **Overhead en CI mínimo**: GoReleaser cross-compila en paralelo; agregar 2 binarios Windows
  incrementa el tiempo de compilación marginalmente (estimado < 30 s en `ubuntu-latest`). El
  pipeline de release no cambia su estructura.
- **`-UseBasicParsing` en `Invoke-WebRequest`**: evita inicializar el motor de IE/MSHTML, lo que
  acelera las descargas y es obligatorio en Windows Server Core.
- **`-TimeoutSec 300`**: idéntico al `--max-time 300` de curl en `install.sh` (línea 145).
  Evita que el script cuelgue indefinidamente.
- **Sin reintentos automáticos**: igual que `install.sh`. El usuario puede reintentar
  manualmente si la descarga falla por red.
- **Extracción en directorio temporal**: `Expand-Archive` escribe en `$tmpDir` antes de copiar
  `vector.exe` al destino final, evitando sobrescribir un binario en producción durante una
  extracción parcial.
- **`Get-FileHash` local**: la verificación SHA256 opera sobre el archivo ya descargado; no
  requiere petición de red adicional.

---

## 18. Restricciones

El agente no debe:

- Agregar winget, Chocolatey, Scoop ni ningún package manager (fuera de scope).
- Modificar `.github/workflows/release.yml` (solo revisar; sin cambios esperados).
- Modificar `scripts/install.sh` (no es parte del scope).
- Asumir `amd64` por defecto si la arquitectura no se detecta en Windows — abortar siempre.
- Agregar firma de código (code signing) al binario Windows.
- Modificar el esquema del estado del board, el JSON de specs ni ningún componente del CLI Go.
- Instalar dependencias externas de PowerShell (solo cmdlets nativos de PS 5.1+).
- Modificar el PATH del sistema automáticamente desde el script (solo hint).
- Cambiar el `name_template` del asset de GoReleaser.
- Agregar soporte de GOOS distintos de `darwin`, `linux`, `windows`.
- Validar el formato del tag `--version` (pass-through).
- Cambiar `checksum.algorithm` ni el formato de `checksums.txt`.
- Cambiar `builds.binary` (sigue siendo `vector`; GoReleaser maneja `.exe` automáticamente).
- Documentar el one-liner `irm | iex` con argumentos (no funciona en PS 5.1; usar dos pasos).

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] `.goreleaser.yml` modificado: `goos` incluye `windows`; dos secciones `archives` (id
      `unix` con `[tar.gz]` y goos `[darwin, linux]`; id `windows` con `[zip]` y goos
      `[windows]`); `release.extra_files` incluye `scripts/install.ps1`; comentario de
      cabecera actualizado (líneas 2 y 4).
- [ ] `scripts/install.ps1` creado con feature parity de `install.sh`: verificación PS 5.1+,
      detección de arch con abort en arch desconocida, resolución de versión vía GitHub API
      (con try/catch en el parseo JSON), descarga con timeout y mapeo HTTP 401/403/404/429/5xx,
      verificación SHA256 obligatoria, extracción, instalación, hint de PATH, limpieza en
      `try/finally`, flags `--version`/`--dry-run`/`--force`, env vars
      `VECTOR_INSTALL_DIR`/`GITHUB_TOKEN`/`DEBUG`.
- [ ] `README.md` actualizado: subsección `### Windows` con one-liner `irm | iex` (solo
      latest), método de dos pasos con nota explicativa (para inspección y para `--version`),
      directorio default, hint de PATH; plataformas soportadas actualizadas para incluir Windows.
- [ ] `.github/workflows/release.yml` revisado y confirmado sin cambios funcionales; comentario
      de cabecera actualizado (4→6 archives).
- [ ] `goreleaser check` pasa sin errores sobre el `.goreleaser.yml` modificado.
- [ ] Cross-compilación manual `GOOS=windows GOARCH=amd64/arm64 CGO_ENABLED=0` sin errores.
- [ ] `go -C cli test ./...` sigue verde.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] `.goreleaser.yml` tiene `windows` en `goos`, dos secciones `archives` con `goos`
      correctos (sin overlap ni gap), y `extra_files` apuntando a `scripts/install.ps1`.
- [ ] El comentario de cabecera del `.goreleaser.yml` ya no contiene "no Windows" y el conteo
      de binarios es correcto.
- [ ] `builds.binary` sigue siendo `vector` (sin `.exe`; GoReleaser lo añade automáticamente).
- [ ] `install.ps1` verifica PowerShell 5.1+ como primera operación.
- [ ] `install.ps1` aborta con error accionable en arquitectura no soportada; no asume `amd64`.
- [ ] `install.ps1` implementa todos los flags: `--version`, `--dry-run`, `--force`.
- [ ] `install.ps1` soporta `VECTOR_INSTALL_DIR`, `GITHUB_TOKEN` y `DEBUG` como env vars.
- [ ] HTTP 401, 403, 404, 429 y 5xx tienen mensajes de error accionables distintos.
- [ ] El parseo de `ConvertFrom-Json` está envuelto en `try/catch` con mensaje accionable.
- [ ] La verificación SHA256 usa `Get-FileHash -Algorithm SHA256` y es obligatoria antes de
      copiar `vector.exe`.
- [ ] `Invoke-WebRequest` incluye `-UseBasicParsing` en todas las llamadas.
- [ ] El README documenta el one-liner `irm | iex` SOLO para instalar latest; la variante
      `--version` usa el método de dos pasos con nota explicativa.
- [ ] El directorio de instalación predeterminado es `$env:LOCALAPPDATA\Programs\Vector`.
- [ ] No se modifica el PATH automáticamente — solo hint.
- [ ] La limpieza del directorio temporal ocurre en bloque `finally`.
- [ ] `goreleaser check` pasa.
- [ ] Cross-compilación `GOOS=windows` es exitosa para `amd64` y `arm64`.
- [ ] `go -C cli test ./...` sigue verde.
- [ ] El comentario de `release.yml` línea 5 fue actualizado de 4 a 6 archives.
- [ ] No se implementó nada fuera de scope (no winget, no firma de código, no soporte PS < 5.1,
      no modificaciones a `install.sh`).
- [ ] No quedan placeholders `[...]` literales en el spec.
