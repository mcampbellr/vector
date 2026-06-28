# Tasks — detect-external-blocker-in-apply

## 1. Command (kit)

- [x] 1.1 `apply.md` §6: añadir sub-paso "Detect external-dependency blocker" con las 3 señales
      (TODO runtime con dep externa; artefacto outbound de pedido a humano; item tasks/aceptación
      pendiente de dato real) y el guard determinista test-only/cosmético.
- [x] 1.2 `apply.md` §6: regla de routing — cualquier señal ⇒ `vector spec status <id>
      needs-attention --reason "<motivo>"` automático e independiente de `applyMode`; ninguna ⇒
      comportamiento actual (`review` o dejar para `/vector:close`).
- [x] 1.3 `apply.md` §6: forma de la razón (qué falta + cómo/quién desbloquea + ref de PR) y nota
      que documenta la heurística (auditable) y la diferencia con el hard-stop de §4.
- [x] 1.4 `apply.md` §7: surfacear el bloqueo + `reason` cuando se ruteó a `needs-attention`;
      "ready for review" cuando limpio.

## 2. Embed / scaffold

- [x] 2.1 `go -C cli generate ./...` para regenerar
      `cli/internal/scaffold/assets/commands/vector/apply.md`; verificar que coincide con la fuente.

## 3. Verificación

- [x] 3.1 Gate: `gofmt -l cli` vacío, `go -C cli vet ./...`, `go -C cli test ./...`,
      `go -C cli build ./...` verdes (no-regresión del binario).
- [x] 3.2 Casos de ejemplo en el PR: TODO runtime → `needs-attention`; artefacto outbound →
      `needs-attention`; item tasks pendiente → `needs-attention`; TODO test-only/cosmético →
      `review`; run limpio → `review`.
