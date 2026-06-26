# Add spec ↔ external ticket linking

## Why

A spec on the board has no link to the operational ticket (Jira, Linear, GitHub) it implements,
so traceability lives only in the dev's head. The state already carries `Ticket{Provider,Key,URL,Auto}`
and the board already renders it, but nothing **writes** it. We need the write path — manual,
and best-effort automatic — without imposing any convention on the user's repo.

## What changes

- New `/vector:link [id] [ticket]` project command + `vector spec link <id> <ref> [--provider]`
  binary subcommand (CLI-owns-writes). Parses the ref, infers the provider (or `--provider`),
  persists `auto:false`.
- **Auto-link on `/vector:raw`**: a ticket detected in the raw idea text seeds the draft
  (`auto:true`) via a new `CreateSpecParams.Ticket`.
- **Auto-link on `vector sync`**: per change, `detectTicket` reads the spec doc's `ticket:`
  frontmatter first, then a conservative prose scan of the change artifacts (`auto:true`).
- **Provider inference** (jira / linear / github) by URL or shorthand; ambiguous key without URL →
  manual requires `--provider`, auto does **not** guess.
- **Idempotency + precedence**: re-linking the same `provider+key` is a no-op; auto never
  overwrites a manual link.

## Scope

- In: `Store.LinkSpec`, `CreateSpecParams.Ticket`, the `vector spec link` subcommand, the
  `parseRef`/`inferProvider`/`detectTicket` helpers, threading into `runSync`/`runSpecCreate`, the
  `/vector:link` command + embedded asset, and the `docs/domain-contract.md` §5 update.
- Out: validating against the external tracker, multiple tickets per spec, `unlink`, bidirectional
  sync, HTTP write surface, any change to the spec state machine.

Authored spec: `.vector/specs/add-ticket-linking/spec.md`.
