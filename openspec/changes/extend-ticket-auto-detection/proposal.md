# Extend ticket auto-detection to bare keys with configurable defaults

## Why

The sibling change `add-ticket-linking` shipped `detectTicket` (`cli/cmd/vector/ticket.go`), but it
**discards any ticket key without a URL** because it cannot infer the provider. UAT against the
`somnio` repo showed that repo writes its tickets as bare Jira keys anchored to a label in prose —
`Ticket: MH-1592.`, `**Ticket:** MH-1552`, `> Ticket: MH-1611 · Epic MH-1528` — with no URL and no
`ticket:` frontmatter. Result: `vector sync` linked **zero** of them. To make auto-linking useful in
single-tracker repos, detection needs a deterministic way to resolve bare keys, driven by config —
without an LLM in the `sync` path (token-routing) and without imposing conventions on the user's repo.

## What changes

- **Config**: two new optional fields in `.vector/config.json`:
  - `defaultTicketProvider` (`jira|linear|github|other`) — fallback provider for ambiguous keys
    detected in sync/raw and for manual `vector spec link` of a bare key. Invalid value → error at
    `config.Load`.
  - `ticketKeyPrefixes` (`[]string`, e.g. `["MH"]`) — project key prefixes that identify a bare key
    as a ticket with high confidence anywhere in prose.
- **Smarter (still deterministic) detection** in `detectTicket`, as a fallback **after** the `ticket:`
  frontmatter scan and the prose-URL scan (precedence order unchanged), and **only** when
  `defaultTicketProvider` is set. It recognizes a key by **either**:
  - a **cue word** anchored at line start (tolerating `>` blockquote and `**bold**`): `Ticket:`,
    `Issue:`, `Ref:`, `Tracking:`, or a provider name (`Jira:`/`Linear:`/`GitHub:`), taking the first
    `[A-Za-z][A-Za-z0-9]*-\d+` key after the cue; or
  - a **known project prefix** (`^<PREFIX>-\d+` from `ticketKeyPrefixes`) anywhere in prose.
- **Denylist** of prefixes that are never tickets: `ADR`, `RFC` (built-in).
- **Manual link of a bare key**: `vector spec link <id> <key>` with no `--provider` consults
  `defaultTicketProvider` instead of erroring on ambiguity.

## Scope

- In: the two config fields + validation, `ticketFromContext` (cues + prefixes + denylist) wired into
  `detectTicket`, `runSpecLink` consulting the default provider, threading config into the callers,
  `TicketProvider.Valid()` if missing, Go tests, and the `docs/domain-contract.md` §5 update.
- Out: building a canonical URL from the key (base-url per provider — future), validating against the
  external tracker, multiple tickets per spec, recognizing non-ticket labels (`Epic:`/`Story:`),
  auto-discovering the project prefix, `unlink`, and any spec state-machine change.

Authored spec: `.vector/specs/extend-ticket-auto-detection/spec.md`.
