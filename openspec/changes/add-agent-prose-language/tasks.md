# Tasks — add-agent-prose-language

## 1. Config (`cli/internal/config`)

- [x] 1.1 Añadir `Language string \`json:"language,omitempty"\`` al struct `Config` (junto a los
      campos opcionales como `ApplyMode`). `SchemaVersion` se mantiene en 1.
- [x] 1.2 Añadir `func (c *Config) ResolvedLanguage() string` → `strings.TrimSpace(c.Language)`.
- [x] 1.3 Tests: round-trip de `Language` (set/omitido); carga de un config legacy sin el campo →
      `Language == ""` sin error; `ResolvedLanguage()` recorta espacios.

## 2. Binario — flags init/update (`cli/cmd/vector/main.go`)

- [x] 2.1 `runInit`: flag `--language` (`fs.String("language", "", "...")`). Al persistir, si el
      valor recortado no está vacío, `cfg.Language = strings.TrimSpace(*language)`.
- [x] 2.2 `runInit --force` sin `--language`: preservar el `Language` existente (no borrarlo).
- [x] 2.3 `runUpdate`: mismo flag `--language`; si trae valor, set antes de `config.Write`; sin
      flag, no toca `Language`.
- [x] 2.4 `usage()`: añadir `[--language lang]` a las líneas de `init` y `update`; reflejar el
      idioma en el reporte de `init` cuando se fijó.
- [x] 2.5 Tests: `init --language es` escribe el campo; sin flag lo omite; `--force` sin flag
      preserva; `--force --language en` sobrescribe; `update --language` fija/cambia preservando
      el resto.

## 3. Binario — proyección de idioma (`cli/cmd/vector/standup.go` + `cli/internal/standup`)

- [x] 3.1 `cli/internal/standup/standup.go`: añadir `Language string \`json:"language,omitempty"\``
      al struct `Projection` (poblado por el caller, no por el builder).
- [x] 3.2 `runStandup` (línea 20): tras `enrichProjection` (línea 45), `config.Load` +
      `proj.Language = cfg.ResolvedLanguage()` antes de serializar `--json`. La asignación va en
      `runStandup`, no en `enrichProjection` (no acoplar `standup` a `config`).
- [x] 3.3 Error de `config.Load` se ignora para el idioma → `proj.Language` vacío, agente al
      fallback. `runStandupCommit` no se toca.
- [x] 3.4 Tests: `vector standup --json` incluye `language` cuando el config lo declara y lo
      omite cuando no.

## 4. Kit — comando y agente

- [x] 4.1 `kit/commands/vector/standup.md`: paso 1 nota el posible campo `language`; paso 2, si
      está presente, antepone `Write the prose in: <language>` al prompt del subagente; si
      ausente, no añade directiva.
- [x] 4.2 `kit/agents/vector-standup-writer.md`: reemplazar la hard rule de idioma por
      *"Write the prose in the language provided by the command; if none is provided, match the
      conversation language. Keep spec ids verbatim."*
- [x] 4.3 Regenerar la copia embebida (`go -C cli generate ./...`) y verificar que
      `cli/internal/scaffold/assets/agents/vector-standup-writer.md` coincide con la fuente.

## 5. Docs

- [x] 5.1 `README.md`: documentar `vector init --language` / `vector update --language` y el
      campo `language` del config (semántica "ausente = idioma de la conversación").

## 6. Gate

- [x] 6.1 `go -C cli generate ./...`, `gofmt -l cli` (vacío), `go -C cli vet ./...`,
      `go -C cli test ./...`, `go -C cli build ./...` — todos verdes.
- [x] 6.2 Verificación funcional: repo con `"language": "es"` → `/vector:standup` produce digest
      en español aunque la conversación esté en inglés; repo sin el campo → comportamiento actual.
