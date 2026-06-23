# Workflows — Convención de git

> Aplica a: todo trabajo con git en el repo Vector.

## Idioma (heredado del global del usuario)

- **Commits, títulos/cuerpos de PR, branches y cualquier artefacto de repo → en inglés**,
  sin importar el idioma de la conversación. No se redefine aquí; ver `~/.claude/CLAUDE.md`.

## Branches

- kebab-case, descriptivas, en inglés: `feat/board-state-sync`, `fix/init-backup-consent`.
- No commitear directamente sobre `main` para trabajo no trivial; ramificar primero.

## Commits

- Estilo del repo (Conventional Commits, observado en el historial inicial: `docs:`,
  `chore:`). Mantener el prefijo de tipo.
- Commitear o pushear **solo cuando el usuario lo pide** (regla del harness).

## Footer de Claude

- Mensajes de commit creados por Claude terminan con la línea `Co-Authored-By` indicada por el
  harness; los cuerpos de PR con el footer de Claude Code. No inventar otros footers.

## Detección de convención del usuario

- Esta rule gobierna el repo **de Vector**. La convención de commit/git del repo **del
  usuario** se detecta en `/vector init` y se registra en el estado; no se asume.
