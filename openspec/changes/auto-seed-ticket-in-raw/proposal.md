# Auto-seed the detected ticket link in /vector:raw

## Why

`/vector:raw` already detects a ticket reference in the raw idea but only *notes* it and tells
the user to run `/vector:link` afterwards. The binary already supports seeding the link at
creation: `vector spec create` accepts `--ticket '{provider,key,url,auto}'`, and `parseTicketFlag`
(`cli/cmd/vector/ticket.go:296`) is documented as *"the --ticket JSON passed by /vector:raw when
it detects a ticket in the raw idea text"*. So a raw input that clearly carries a ticket should
come out **linked at create time**, with no second command. The gap is purely the missing wiring
in the command template (`kit/commands/vector/raw.md`): step 9's `vector spec create` omits
`--ticket`.

## What changes

Markdown-only change to `kit/commands/vector/raw.md` (+ its regenerated scaffold asset copy):

- **Detect a ticket in the `RAW_IDEA`** with detection rules matching `vector sync`'s
  `detectTicket`, by confidence tier:
  1. **URL** of a recognized tracker (provider by host via `inferProvider`; unknown host → skip;
     two distinct ticket URLs → discard) — replicates `ticketFromProse`.
  2. **Shorthand `<provider>:<key>`** — a NEW extension for free-prose (sync resolves shorthand
     only in frontmatter via `parseRef`→`splitShorthand`; `ticketFromProse` is URL-only).
  3. **Cue-word bare key** (`ticketCueRe`, line-start, tolerating ws/`>`/`**`) — gated on
     `defaultTicketProvider`.
  4. **Configured-prefix bare key** (`ticketKeyPrefixes`) — gated on `defaultTicketProvider`.
  Same-tier distinct keys → discard; `ADR`/`RFC` prefixes skipped.
- **Seed at create time**: build `{"provider","key","url","auto":true}` and pass it as `--ticket`
  to `vector spec create` (step 9).
- **Report** `linked <KEY> (<provider>)` when seeded; the `/vector:link` hint only shows for the
  ambiguous / no-detection case (step 11).
- **Never block creation on linking**: an invalid `--ticket` (malformed JSON, uninferable
  provider) falls back to creating the card without `--ticket` and shows the hint (today's
  behavior).

No Go changes — the binary already supports `--ticket`. `/vector:sync` and `/vector:link` are
untouched.

## Capabilities

### Modified Capabilities
- `raw-spec-authoring`: `/vector:raw` now auto-seeds a confidently-detected ticket link at create
  time (`auto:true`), instead of deferring every link to a manual `/vector:link`.

## Out of scope

- Any Go change (`ticket.go`, `store.go`, the `Ticket`/`TicketProvider` types).
- Changing `/vector:sync`'s `detectTicket` (already auto-links) or `/vector:link`.
- Prompting for `--provider` in the raw flow (ambiguous keys go to `/vector:link`).
- Validating tickets against external trackers (no API calls).
- Multiple tickets per spec.

## Note on this repo

This repo's `.vector/config.json` has no `defaultTicketProvider`, so bare-key cue/prefix cases do
not auto-seed here (gated, like sync) — only URL and shorthand do — until that provider is
configured.
