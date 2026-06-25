---
name: "Vector: Link"
description: Link a spec card to its external ticket (Jira/Linear/GitHub). Parses the ref, infers the provider (or asks when ambiguous), and persists the link via the binary. You never write Vector's state yourself.
category: Workflow
tags: [vector, ticket, link, traceability]
---

Link a Vector card to the operational **ticket** it implements (Jira, Linear, GitHub) so
traceability lives on the board, not in your head. **You never write Vector's state yourself** —
you resolve the ref and then call `vector spec link`, which persists the link (CLI-owns-writes).

**Input**: `$ARGUMENTS` — `<id> <ref>`. The `<id>` is the spec id; `<ref>` is a ticket URL,
a `<provider>:<key>` shorthand, or a bare key. If either is missing, ask
(`vector spec list` to pick the card).

> Token routing: orchestration only — parse the ref and call the binary. Don't over-think it.

## Hard rules

- **Linking is metadata, not a transition.** It never changes the card's lifecycle status.
- **One ticket per spec.** Re-linking the same `provider+key+url` is a no-op (the binary is
  idempotent). Linking a *different* ticket replaces the existing one — **confirm first** when the
  card already has a ticket (`AskUserQuestion`), since manual links are authoritative.
- **Never guess an ambiguous provider.** A bare key with no URL (e.g. `ACME-1`) needs an explicit
  provider; ask the user (`AskUserQuestion`) and pass `--provider`, never assume.
- **No external calls.** Vector does not validate the ticket against the tracker.

## Steps

1. **Read `<id>` and `<ref>`** from `$ARGUMENTS`. If `<id>` is missing, list cards
   (`vector spec list`) and ask which to link. If `<ref>` is missing, ask for the ticket.

2. **Check the existing link.** Read `.vector/specs/<id>/state.json`. If `ticket` is already set
   and the new ref differs, confirm the replacement with `AskUserQuestion` before proceeding.

3. **Resolve the provider.** A full URL (`https://…atlassian.net/…`, `linear.app`, `github.com`)
   or a `<provider>:<key>` shorthand is unambiguous — let the binary infer. A **bare key** is
   ambiguous: ask the user for the provider and pass `--provider <jira|linear|github|other>`.

4. **Link** — call the binary:
   ```bash
   vector spec link <id> <ref> [--provider <provider>] --json
   ```
   It parses the ref, infers/honors the provider, persists `ticket{provider,key,url,auto:false}`,
   and logs `spec.linked`. Parse the JSON (`changed:false` means it was already linked).

5. **Report**: the id, the linked `provider key`, the URL (or note it's keyed-only when empty),
   and whether anything changed. The board's card now shows the ticket.

## Notes

- **Auto vs manual**: `/vector:raw` and `vector sync` may *auto*-detect a ticket (`auto:true`);
  this command always writes a **manual** link (`auto:false`), which auto-detection never overwrites.
- An empty URL is valid (a bare key with no host): the board shows the key without a link.
- If `vector` is not found, it isn't installed — tell the user; do not edit `.vector/` by hand.
