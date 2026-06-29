# Instalación de Vector

Vector se distribuye como un **único binario** que incluye el CLI y el panel web embebido. La
instalación no requiere Go, Node ni ninguna otra cadena de herramientas en tu máquina.

## Requisitos

- **macOS 12+** o **Linux** (Ubuntu 20.04+ o equivalente).
- `bash`.
- `curl` o `wget`.
- **No** se requiere Go en la máquina del usuario: el instalador descarga binarios
  precompilados.

Plataformas soportadas: `darwin`/`linux` × `amd64`/`arm64`. Windows no está soportado en V1.

## Instalación

```sh
curl -fsSL https://github.com/mcampbellr/vector/releases/latest/download/install.sh | sh
```

> **Nota importante.** Este comando anónimo `curl … | sh` **solo funciona cuando el repositorio
> `mcampbellr/vector` es público**. Mientras el repo sea privado, las requests anónimas a la
> GitHub Releases API devuelven `404`/`403` y la instalación falla — es el comportamiento
> esperado ("build now, publish later"). Hasta entonces, instala localmente (compilando desde
> el repo) o mediante un download autenticado con un token de GitHub:
>
> ```sh
> GITHUB_TOKEN=<tu_token> bash scripts/install.sh --version v0.1.0
> ```
>
> La URL pública estable de `install.sh` se confirma al hacer el repo público (ver las
> preguntas abiertas del spec `one-step-installer-script`).

El instalador detecta tu sistema operativo y arquitectura, resuelve la última versión (o la
fijada con `--version`), descarga el binario de tu plataforma junto con `checksums.txt`,
**verifica el SHA256 antes de instalar**, y copia el binario a `~/.local/bin/vector` con modo
`0755`. No usa `sudo` ni edita tus archivos de shell.

## Flags y variables de entorno

| Flag / Variable | Descripción |
|---|---|
| `--version <tag>` | Instala una versión específica (ej. `--version v0.1.0`) sin consultar la API de latest. |
| `--dry-run` | Imprime cada paso con prefijo `[dry-run]` sin descargar ni instalar nada. |
| `--force` | Reinstala aunque la misma versión ya esté presente. |
| `VECTOR_INSTALL_DIR` | Directorio de instalación. Default: `$HOME/.local/bin`. |
| `GITHUB_TOKEN` | Token bearer opcional para download autenticado (necesario mientras el repo sea privado). |
| `DEBUG=1` | Activa `set -x` para una traza completa del script. |

## Verificación post-instalación

```sh
vector version
```

Debe imprimir la versión del tag instalado (ej. `vector v0.1.0`). Si imprime `vector dev`, el
binario es un build local sin versión inyectada, no un release.

## PATH

Si `~/.local/bin` no está en tu `$PATH`, el instalador no edita ningún archivo de shell —
únicamente sugiere el export. Añádelo manualmente a tu perfil de shell:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

(En `~/.zshrc`, `~/.bashrc` o `~/.profile`, según tu shell.)

## Referencia cruzada

La sección de instalación del `README.md` se actualizará en el spec
`rewrite-public-readme-humanized` cuando el repo sea público. Este documento es la fuente de
verdad de la instalación hasta entonces.
