# Tasks — rewrite-public-readme-humanized

## 1. Lectura y verificación

- [ ] 1.1 Leer el `README.md` actual (raíz) y el spec completo.
- [ ] 1.2 `ls kit/commands/vector/*.md` → confirmar el recuento (11 esperados) y leer cada archivo
      para descripciones reales de los commands.
- [ ] 1.3 `ls docs/{vision,domain-contract,plugin-and-commands,commercialization}.md
      docs/assets/kanban-reference.png` → confirmar que cada path existe.
- [ ] 1.4 Confirmar Go 1.26+ en `cli/go.mod` para los pasos de instalación.

## 2. Redacción (9 secciones, inglés)

- [ ] 2.1 Header + tagline; sin badges/shields de CI no configurado.
- [ ] 2.2 `What is Vector` (2–3 párrafos) + imagen del board con alt text descriptivo.
- [ ] 2.3 `Why Vector` (token routing, board kanban, agnóstico al stack, integración Claude Code).
- [ ] 2.4 `Installation` (Go 1.26+, `git clone` + `go build` desde `cli/`, `vector init`;
      `curl | install.sh` como "coming soon").
- [ ] 2.5 `Quickstart` (flujo mínimo de 3–4 pasos hasta ver la card en el board).
- [ ] 2.6 `Key Concepts` (spec, OpenSpec, board, token routing, `/vector:*`, `vector init` con
      contexto + links a `docs/`).
- [ ] 2.7 `Commands Reference` (tabla de los 11 commands con descripciones verificadas).
- [ ] 2.8 `Walkthrough — End-to-End Flow` (narrativo: init → raw → propose → apply → board).
- [ ] 2.9 `Contributing / License` (licencia TBD, no inventada) + `Further Reading` (links verificados).

## 3. Humanización

- [ ] 3.1 Pasar cada sección por `/humanizer`; eliminar señales de prosa generada por IA.

## 4. Verificación final (§8 del spec)

- [ ] 4.1 `grep -c` de "captura inicial" / "Nada implementado" / "Sin código aún" → 0.
- [ ] 4.2 Paths referenciados existen; imagen referenciada con alt text; `wc -l > 100`.
- [ ] 4.3 Eliminado el bloque español residual de `--language`; ningún archivo fuera de `README.md`
      modificado.
