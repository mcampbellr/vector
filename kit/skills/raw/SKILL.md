---
name: raw
description: Turn a raw idea or feature request into a structured Vector spec and register it on the board (status open). Use when the user runs /vector:raw or wants to capture a new spec/idea into Vector. Equivalent to /idea, but persisted by the Vector CLI.
---

# /vector:raw — capture a spec

Take the user's raw text and register a new spec in Vector. **You never write
Vector's state files yourself** — the `vector` binary is the sole writer. Your job
is to refine the input and call the binary.

> Token routing: this is light refinement (no architecture decisions). It can run
> on a cheap tier. Don't over-think it — capture, don't design.

## Steps

1. **Read the raw input** ($ARGUMENTS, or the user's latest message if empty).
2. **Derive a concise title** (≤ ~8 words) and a **kebab-case id** (slug of the title).
3. **Detect a ticket reference** if present (e.g. `MH-1438`, a Jira/Linear/GitHub URL).
   - If found, note it — after creating the spec, link it with `/vector:link <id> <ticket>`.
   - If not found, do nothing; the user can link later.
4. **Pick a priority** only if the input clearly implies one (urgent/high/normal/low);
   otherwise omit (defaults to `normal`).
5. **Author the spec body** as markdown using the template below — concise capture,
   not full design. Deeper design happens later at `/vector:apply`.
6. **Create the spec** by piping the body to the binary via stdin:

   ```bash
   vector spec create \
     --title "<title>" \
     --id "<slug>" \
     [--repo "<repo-name>"] \
     [--priority "<priority>"] \
     --body-file - <<'SPEC'
   <the authored markdown body>
   SPEC
   ```

   Add `--json` if you need to parse the result programmatically.
7. **Report** the created id and that it's on the board as `open`. If you detected a
   ticket, tell the user you can link it (or run `/vector:link`).

If `vector` is not found, the plugin/binary isn't installed — tell the user to run the
Vector install script; do not attempt to write `.vector/` files by hand.

## Spec body template (≈ /idea, lightweight)

```markdown
# <title>

## Problem / motivation
<what the user actually wants and why; 1–3 sentences>

## Context
<relevant constraints, current behavior, links/tickets if any>

## Proposed approach
<the rough direction — bullet points are fine; this is capture, not a final design>

## Scope
- In: <…>
- Out: <…>

## Open questions
- <unknowns to resolve before/at apply>
```

## Notes

- The id must be kebab-case and is reused as the OpenSpec change name when applied.
- Keep the body honest: mark unknowns as open questions instead of inventing detail.
