# Strengthen Excalidraw wireframe confirmation to an explicit two-option AskUserQuestion

## Change Type
chore

## What Changes
In `kit/commands/vector/raw.md` step 12.c and `kit/commands/vector/research.md` step 15.c,
replace the vague "ask via `AskUserQuestion` whether to generate…" phrasing with explicit,
mandatory two-option language: a proper selection question with two concrete options —
**Generate wireframe** (proceed) and **Skip** (end cleanly) — never a free-text prompt.

## Why
At runtime the vague instruction rendered as a free-text prompt instead of a bounded choice.
Explicit option labels force a select-style question, consistent with other spec clarifications
the user is meant to answer.

## Files to Touch
- kit/commands/vector/raw.md (step 12.c)
- kit/commands/vector/research.md (step 15.c)

## Acceptance
- Steps 12.c / 15.c explicitly list the two options (Generate wireframe / Skip) and forbid free text.
- No changes to the UI heuristic, async spawn, or routing steps.
- No changes to precondition checks (--no-sketch, sketchEnabled).

## Follow-up (embedded commands)
- go generate ./internal/scaffold (from cli/) → refresh assets
- reinstall binary + `vector update` to reseed .claude/commands/vector/