# Tasks — dispatcher-json-retry

## 1. Standup command

- [ ] 1.1 Leer `kit/commands/vector/standup.md` completo para entender la numeración actual de pasos y el estilo.
- [ ] 1.2 Insertar el bloque "Validate the digest (shape-gate)" entre §2 y §3: check de parseabilidad + `global` non-empty + `perSpec` array.
- [ ] 1.3 Añadir el re-spawn del agente con la directive de corrección explícita (`shape exacto + No preface, no code fences, no trailing text`).
- [ ] 1.4 Añadir el segundo shape-gate (intento 2): si válido → continuar §3; si inválido → reportar mensaje de fallo accionable y abortar sin pipear nada al binario.
- [ ] 1.5 Añadir la notificación al usuario durante el retry: `subagent returned invalid JSON — retrying (attempt 2/2)…`.
- [ ] 1.6 Verificar que §1, §3 y §4 no sean modificados y que el comportamiento en el happy path sea idéntico al actual.

## 2. Apply command

- [ ] 2.1 Leer `kit/commands/vector/apply.md` §7 completo para entender el paso 2 ("Pass that exact JSON…") y el paso 3 ("Pipe its JSON to `vector spec summarize …`").
- [ ] 2.2 Insertar el shape-gate (intento 1) entre el paso 2 y el paso 3 de §7: check de parseabilidad + `summary` non-empty.
- [ ] 2.3 Añadir el re-spawn del agente con la directive de corrección para `vector-summary-writer`.
- [ ] 2.4 Añadir el segundo shape-gate (intento 2): si válido → continuar paso 3; si inválido → skip no-gate (no pipear al binario).
- [ ] 2.5 Actualizar §8 para mencionar, si el summary fue saltado por doble fallo, la nota `summary skipped: subagent returned invalid JSON twice` sin interrumpir el reporte principal.
- [ ] 2.6 Verificar que ningún paso fuera de §7 y §8 sea modificado, y que el doble fallo en summary nunca sea gate (apply siempre completa la transición).

## 3. Assets embebidos

- [ ] 3.1 Copiar `kit/commands/vector/standup.md` modificado a `cli/internal/scaffold/assets/commands/vector/standup.md`.
- [ ] 3.2 Copiar `kit/commands/vector/apply.md` modificado a `cli/internal/scaffold/assets/commands/vector/apply.md`.
- [ ] 3.3 Confirmar que los dos assets están byte-a-byte en sync con sus contrapartes de `kit/` (diff vacío).

## 4. Verificación de no-regresión

- [ ] 4.1 Ejecutar `gofmt -l cli && go -C cli vet ./... && go -C cli test ./...` y confirmar que pasan verdes (sin cambios al binario, solo verificación de que no se rompió nada).
- [ ] 4.2 Revisión manual: simular JSON inválido del `vector-standup-writer` → confirmar que el marcador no avanza y el mensaje de error es el definido en design.md.
- [ ] 4.3 Revisión manual: simular JSON inválido del `vector-summary-writer` → confirmar que apply completa la transición y §8 menciona el skip.
- [ ] 4.4 Revisión de no-regresión: ejecutar un flujo completo de `/vector:standup` y `/vector:apply` con output válido del agente → ambos completan sin cambios observables respecto al comportamiento actual.
