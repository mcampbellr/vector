# Tasks — token-meter-accuracy

## 1. Datos — evento + RouteAgent

- [ ] 1.1 Añadir `Precision string \`json:"precision,omitempty"\`` a `AgentRoutedData` en
  `cli/internal/state/event.go` con comentario: `// "actual" = token counts from the harness;
  "estimated" = self-reported by the orchestrating command (default).`
- [ ] 1.2 Actualizar `RouteAgent` en `cli/internal/state/standup.go`: añadir parámetro
  `precision string` (después de `tokensOut int`); normalizar `""` → `"estimated"`; validar
  que el valor sea `"actual"` o `"estimated"` (error `"invalid precision %q: must be actual or
  estimated"`); asignar `data.Precision = precision`.
- [ ] 1.3 Añadir tests de `RouteAgent` en `cli/internal/state/store_test.go` (o equivalente):
  `TestRouteAgent_PrecisionDefault` (sin pasar precision → `"estimated"`),
  `TestRouteAgent_PrecisionActual` (`"actual"` persiste),
  `TestRouteAgent_PrecisionInvalid` (`"bogus"` → error).

## 2. Proyección — board rollup

- [ ] 2.1 Añadir `Precision string \`json:"precision,omitempty"\`` a `TokenSavings` en
  `cli/internal/board/board.go`.
- [ ] 2.2 Implementar lógica de peor caso en `rollupSavings`: variable local `hasEstimated bool`;
  por cada evento `EvtAgentRouted` decodificado, si `data.Precision != "actual"` → `hasEstimated
  = true`; al finalizar: si `s.Routes > 0 && !hasEstimated` → `s.Precision = "actual"`; si
  `s.Routes > 0 && hasEstimated` → `s.Precision = "estimated"`; si `s.Routes == 0` → `s.Precision
  = ""`.
- [ ] 2.3 Añadir tests de `rollupSavings` en `cli/internal/board/board_test.go`:
  `TestRollupSavings_AllActual`, `TestRollupSavings_AllEstimated`, `TestRollupSavings_Mixed`,
  `TestRollupSavings_OldEvents` (campo vacío → `"estimated"`), `TestRollupSavings_Empty`
  (sin eventos → `""`). Usar patrón table-driven del proyecto.

## 3. CLI — flag `--precision`

- [ ] 3.1 Añadir `precision := fs.String("precision", "", "data quality: actual|estimated (default: estimated)")` en `runSpecRoute` (`cli/cmd/vector/route.go`); pasar `*precision` a
  `store.RouteAgent(...)` en la posición correcta del parámetro.
- [ ] 3.2 Añadir `"precision": ...` al mapa del output `--json` en `runSpecRoute`.

## 4. Kit commands

- [ ] 4.1 Actualizar el paso de recording de token routing en `kit/commands/vector/raw.md`:
  añadir guía de cuándo pasar `--precision actual` (señal real del harness, TBD) vs. omitirlo
  (estimación — default correcto y honesto). No cambiar otros pasos.
- [ ] 4.2 Aplicar el mismo cambio en `kit/commands/vector/bug.md` paso equivalente.
- [ ] 4.3 Regenerar las copias vendored `cli/internal/scaffold/assets/commands/vector/raw.md` y
  `cli/internal/scaffold/assets/commands/vector/bug.md` vía `go generate` (o copia manual
  equivalente al patrón del proyecto).

## 5. Web — badge Estimated

- [ ] 5.1 Localizar el componente Token Savings Meter en `web/src/` buscando usos de
  `tokenSavings` / `totalSavedUsd` (ruta exacta TBD — Open question §2 del spec).
- [ ] 5.2 Actualizar el tipo TypeScript del board para incluir `precision?: string` en el objeto
  `tokenSavings` (derivar del contrato de la API, no duplicar a mano).
- [ ] 5.3 Añadir badge condicional `Estimated` en el componente: renderizar solo cuando
  `tokenSavings.precision === "estimated"`; estilo neutral (slate/gris, sin icono de alerta);
  `aria-label` accesible; contraste WCAG AA.

## 6. Documentación

- [ ] 6.1 Actualizar `docs/domain-contract.md` §3 (shape de `agent.routed`) con el campo
  `precision: "actual" | "estimated"` y su semántica (real del harness vs. estimación del
  command; ausente en eventos anteriores → `"estimated"`).

## 7. Verificación

- [ ] 7.1 `gofmt -l cli && go -C cli vet ./... && go -C cli test ./...` verdes sin regresiones.
- [ ] 7.2 `cd web && npm run build` exitoso (necesario para el embed).
- [ ] 7.3 Verificar con `curl /api/board` que `tokenSavings.precision` aparece en la respuesta.
- [ ] 7.4 Confirmar que los kit commands sin `--precision` siguen produciendo `"estimated"` y que
  los tests existentes de `rollupSavings` y `Build` no tienen regresiones.
