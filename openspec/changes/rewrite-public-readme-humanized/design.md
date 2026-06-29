# Design — rewrite-public-readme-humanized

## Decisiones clave

- **Inglés para el entregable, español para el spec**: el `README.md` es artefacto
  público/comercial → inglés (decisión explícita del usuario). El spec sigue la convención del
  repo (docs en español). Distinción intencional, no error (ver spec §16).
- **Patrón editorial, sin software**: no hay capas, estado ni API. El trabajo es
  leer → verificar contra el repo → redactar por sección → `/humanizer` → ensamblar → verificar
  paths. Ningún contenido aspiracional.
- **`/humanizer` obligatorio**: cada bloque de prosa pasa por el skill antes de entregar (sin
  em-dash overuse, rule-of-three, vocabulario AI genérico, paralelismos negativos). No opcional.
- **Solo pasos de instalación reales**: una única vía documentada (clonar + `go build` desde
  `cli/`), no `go install <module-url>` (módulo no publicado, remoto TBD). `curl | install.sh`
  solo como "coming soon".
- **Sin invenciones**: licencia = "TBD" (no existe `LICENSE`, no se crea); descripciones de
  commands verificadas contra `kit/commands/vector/*.md`, no inventadas; cada path referenciado
  validado con `ls` antes de incluirlo.
- **Un solo archivo**: `README.md` (raíz). El bloque español de `add-agent-prose-language`
  (`--language`) se elimina por completo en la reescritura; no se documenta el flag en el README
  público.

## Superficie

- `README.md` (raíz) — único archivo modificado. Reemplazo casi total del contenido actual.
- Lecturas de referencia (no se modifican): `kit/commands/vector/*.md` (11 commands),
  `docs/{vision,domain-contract,plugin-and-commands,commercialization}.md`,
  `docs/assets/kanban-reference.png`, `cli/go.mod` (versión de Go).

## Flujo

leer README actual + fuentes verificadas → redactar las 9 secciones en inglés con datos
verificados → `/humanizer` por sección → ensamblar → correr verificación de §8 (paths existen,
texto antiguo ausente, `wc -l` > 100, inglés) → escribir `README.md`.

## Open questions (no bloquean este change)

- Licencia exacta (sección dice TBD), URL pública del repo (placeholder `github.com/<org>/vector`
  marcado TBD), recuento exacto de commands (re-verificar con `ls kit/commands/vector/*.md`),
  follow-up del instalador one-liner como `/vector:raw` aparte.
