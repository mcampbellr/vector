# Design — add-windows-support-distribution

## Decisiones clave

- **Extensión del pipeline existente, no un pipeline paralelo**: GoReleaser es agnóstico de
  plataforma. Agregar `windows` a `builds.goos` basta para que el mismo workflow en
  `ubuntu-latest` cross-compile 2 binarios Windows adicionales (6 en total) sin runner Windows,
  sin mingw y sin cgo (`CGO_ENABLED=0` ya está). (§5 del spec.)
- **`.zip` para Windows, `.tar.gz` para Unix**: convención nativa de cada plataforma. Se
  implementa con dos secciones `archives` discriminadas por `goos` (id `unix` →
  `[darwin, linux]`; id `windows` → `[windows]`), cubriendo exactamente los tres `goos` del
  bloque `builds` **sin overlap ni gap** (`goreleaser check` lo detecta).
- **`name_template` invariante**: `vector_{{ .Version }}_{{ .Os }}_{{ .Arch }}` se mantiene
  idéntico en ambas secciones, produciendo `vector_<VER>_windows_amd64.zip` y
  `vector_<VER>_windows_arm64.zip`. Contrato de naming compartido entre GoReleaser y el string
  que arma `install.ps1`; deben ser idénticos.
- **`install.ps1` como espejo idiomático de `install.sh`**: mismo orden de pasos, mismos flags
  (`--version`/`--dry-run`/`--force`), mismas env vars (`VECTOR_INSTALL_DIR`/`GITHUB_TOKEN`/
  `DEBUG`), mismos mensajes de progreso (`==>`) y error (`Error:`), mismo patrón de
  corto-circuito "ya instalado". Solo cmdlets nativos de PS 5.1+ (`Invoke-WebRequest`,
  `ConvertFrom-Json`, `Get-FileHash`, `Expand-Archive`) — sin `jq`, sin deps externas.
- **`install.ps1` publicado como release asset vía `release.extra_files`**: necesario para que
  la URL `releases/latest/download/install.ps1` exista y el one-liner `irm | iex` funcione. El
  script es texto plano, **no** entra en `checksums.txt` (solo los archives binarios se
  checksumean).
- **One-liner `irm | iex` solo para latest**: en PowerShell 5.1 `-Command "string"` descarta los
  tokens tras la comilla de cierre, así que el one-liner no puede reenviar `--version`. Para
  pinear versión (o inspeccionar el script) se documenta el método de dos pasos
  (`irm -OutFile install.ps1` → `.\install.ps1 --version v0.1.0`).
- **Fallback de arch = abortar, nunca asumir `amd64`**: si `$env:PROCESSOR_ARCHITECTURE` no es
  `AMD64` ni `ARM64`, el script aborta con mensaje accionable (coherente con `install.sh`
  línea 105).
- **Verificación SHA256 obligatoria y no salteable** con `Get-FileHash -Algorithm SHA256`
  (nativo PS 5.1+, preferido sobre `certutil`), comparada contra la línea de `checksums.txt`
  (formato `<SHA256_UPPERCASE>  <FILENAME>`, dos espacios) antes de copiar `vector.exe`.
- **`-UseBasicParsing` obligatorio** en todas las llamadas `Invoke-WebRequest`: compatibilidad
  con Windows Server Core (sin motor IE/MSHTML) y descarga más rápida.
- **Directorio default `$env:LOCALAPPDATA\Programs\Vector`**: no requiere elevación (UAC);
  equivalente conceptual al `~/.local/bin` de Unix. **No** se modifica el PATH automáticamente;
  solo se imprime el hint.
- **`release.yml` sin cambios funcionales**: GoReleaser ya cross-compila Windows en
  `ubuntu-latest`. Solo cambia el comentario de cabecera (4→6 archives), cosmético.
- **`install.sh` intacto**: es el instalador Unix; su error de Windows en línea 105 sigue siendo
  correcto (los usuarios Windows usan `install.ps1`).

## Superficie

- `.goreleaser.yml` (MODIFICAR): `builds.goos` += `windows`; `archives` → dos secciones
  (`unix`/`windows`) por `goos`; `release.extra_files` += `scripts/install.ps1`; comentario de
  cabecera (líneas 2 y 4). No tocar `before.hooks`, `ldflags`, `snapshot`, `checksum.algorithm`,
  `builds.binary` (sigue `vector`; GoReleaser añade `.exe`), ni el `name_template`.
- `scripts/install.ps1` (NUEVO): verificación PS 5.1+ como primera operación; `DEBUG=1` →
  `Set-PSDebug -Trace 1`; parseo de flags; detección de arch; tmp dir + cleanup en `finally`;
  resolución de versión (GitHub API o `--version`, `try/catch` en el parseo JSON, header
  `Authorization: Bearer` si hay `GITHUB_TOKEN`); nombre de asset
  `vector_<VER sin v>_windows_<ARCH>.zip`; install dir + chequeo de permisos; corto-circuito si
  ya instalado (salvo `--force`); descarga con timeout y mapeo HTTP 401/403/404/429/5xx;
  descarga de `checksums.txt`; verificación SHA256; `Expand-Archive -Force`; verificación de
  `vector.exe`; `Copy-Item`; hint de PATH; dry-run vía helper.
- `README.md` (MODIFICAR): subsección `### Windows` (one-liner latest, método de dos pasos,
  install dir default, hint de PATH); línea de plataformas soportadas (línea 55) += Windows.
- `.github/workflows/release.yml` (REVISAR): confirmar cross-compilación Windows sin cambios;
  actualizar comentario de cabecera (línea 5: 4→6 archives).

## Open questions

Ninguna bloqueante. Las decisiones de §10 del spec ya están tomadas (arch, formatos de archive,
fallback de arch, pass-through de `--version`, PS mínimo, one-liner, SHA256, install dir, PATH
hint). El snapshot completo de GoReleaser (`goreleaser release --snapshot --clean`) es opcional
en verificación porque requiere Node para el build web; la validación mínima es `goreleaser
check` + cross-compilación manual.
