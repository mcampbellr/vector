# Tasks — detect-ticket-from-worktree-name

## 1. Config (`cli/internal/config`)

- [x] 1.1 `WorktreeTicketKeys(repoRoot string) map[string]string`: deriva la raíz de worktrees del
      prefijo literal de `changesTemplate()` antes de `[branch]`; si el template no trae `[branch]`,
      retorna mapa vacío. Molde `ChangesDirs`/`changesTemplate`.
- [x] 1.2 Scan multinivel acotado bajo la raíz con cota en constante nombrada `worktreeMaxDepth`
      (comentario del porqué); tolera grupos `feat`/`chore`/`fix`/`docs`/… y branches de un nivel
      (`develop`). Read-only; **no** `filepath.Glob` de un solo nivel.
- [x] 1.3 Por basename candidato: extraer `<KEY>` con la forma universal `[A-Za-z][A-Za-z0-9]*-\d+`;
      `<KEY>-<resto>` → `slug = resto`; `<KEY>` puro → no se indexa; descarta denylist (`ADR`/`RFC`);
      normaliza la parte de proyecto a mayúsculas. Mapa `slug→key`; slug duplicado con keys distintas → omitido.
- [x] 1.4 Errores de I/O: permiso en subdirectorio → omite ese subárbol (índice sigue); error al
      derivar la raíz (config malformada) → se propaga. No tocar `ChangesDirs`/`compileTemplate`.

## 2. Detección (`cli/cmd/vector/ticket.go`)

- [x] 2.1 `detectTicket` gana `branchKey string` como último parámetro. Tras los tres fallbacks
      actuales, si nada matcheó, `defaultProvider != ""`, `branchKey != ""` y `!denylistedKey(branchKey)`
      → `&state.Ticket{Provider: defaultProvider, Key: branchKey, URL: "", Auto: true}`.
- [x] 2.2 No cambiar la lógica de los fallbacks previos: artefacto (frontmatter/URL/cue/prefijo) siempre
      gana sobre el branch.

## 3. Binario / threading (`cli/cmd/vector/main.go`)

- [x] 3.1 `runSync`: antes del loop, si `cfg.ResolvedDefaultTicketProvider() != ""`, computar
      `idx := cfg.WorktreeTicketKeys(root)` una vez (mapa vacío si no aplica).
- [x] 3.2 Pasar `idx[c.Name]` como `branchKey` a `detectTicket` (create y reconcile). Sin cambios al
      contrato JSON ni al orden de precedencia.

## 4. Tests

- [x] 4.1 `config_test.go` — `WorktreeTicketKeys`: layout multinivel (feat/chore) → mapa correcto; un
      nivel (`develop`); denylist `ADR`/`RFC`; upper-normalize; slug duplicado con keys distintas → omitido;
      template sin `[branch]` → vacío.
- [x] 4.2 `ticket_test.go` — actualizar las ~10 llamadas existentes a `detectTicket(c, root, …)` con `""`
      como 5º arg (`branchKey`); + nuevos: branchKey linkea como último fallback; artefacto gana sobre branch;
      branchKey denylisted → nil; sin provider → nil.
- [x] 4.3 Integración `runSync`: tempdir con worktrees fake + config con default provider → linkea por branch.

## 5. Docs

- [x] 5.1 `docs/domain-contract.md` §5: añadir "nombre del worktree por slug" como último recurso del
      orden de precedencia de `detectTicket`.

## 6. Verificación

- [x] 6.1 `gofmt -l cli && go -C cli vet ./... && go -C cli test -race ./...` verdes.
- [x] 6.2 Sin regresiones en `create`/`sync`/`propose`/`apply`/`serve`/`link`.
