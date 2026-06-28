# Design — auto-seed-ticket-in-raw

Source spec: `.vector/specs/auto-seed-ticket-in-raw/spec.md` (20-section spec authored by
`/vector:raw`, validated PASS). This file captures the load-bearing decisions; the spec doc is
the full reference.

## Key decisions (LOCKED)

1. **Markdown-only change** — `kit/commands/vector/raw.md` + regenerated
   `cli/internal/scaffold/assets/commands/vector/raw.md`. No Go: the binary already supports
   `--ticket` (`parseTicketFlag` ticket.go:296, `runSpecCreate` main.go:685, `Ticket` types.go:113).
2. **Detection rules match `vector sync`** for URL, cue-word, and prefix tiers; replicate the
   semantics (the markdown flow can't call the Go helpers).
3. **Shorthand-in-prose is a NEW extension** — `detectTicket`'s prose path (`ticketFromProse`)
   scans URLs only (`ticketURLRe`); sync resolves `<provider>:<key>` shorthand only in frontmatter
   (`ticketFromFrontmatter`→`parseRef`→`splitShorthand`). The command scans the `RAW_IDEA` for
   shorthands with `splitShorthand` semantics.
4. **Precedence by confidence; same-tier distinct keys → discard** (no guessing which is real).
5. **Bare-key tiers gated on `defaultTicketProvider`** (opt-in, like sync). Without it, a bare key
   is not seeded — silent skip + `/vector:link` hint.
6. **`auto:true`** marks auto-detected provenance (distinct from `/vector:link`).
7. **Never block creation on linking** — invalid `--ticket` → create without it + hint.
8. **No `--provider` prompt** in raw; ambiguous keys go to `/vector:link`.

## Detection precedence (replicating ticket.go semantics)

| Tier | Signal | Source parity | Gate |
|---|---|---|---|
| 1 | URL of known tracker (host→provider) | `ticketFromProse` / `inferProvider` | none |
| 2 | `<provider>:<key>` shorthand in prose | **NEW** (`splitShorthand` semantics) | none |
| 3 | cue-word bare key (`ticketCueRe`, line-start, tolerates ws/`>`/`**`) | `ticketFromContext` | `defaultTicketProvider` |
| 4 | configured-prefix bare key (`ticketKeyPrefixes`) | `ticketFromContext` | `defaultTicketProvider` |

Denylist `ADR`/`RFC` (`denylistedKey`); same-tier distinct keys → discard (`pickSingleKey` /
`ticketFromProse` conflict semantics). `ticketFromContext` already returns `Auto:true`.

## Flow

1. `/vector:raw` refines/composes (steps 1–7).
2. Detection runs over `RAW_IDEA` by tier → `TICKET_JSON = {provider,key,url,auto:true}` or unset.
3. Validate (step 8).
4. `vector spec create … [--ticket "$TICKET_JSON"] … --status draft --body-file -`.
5. Binary validates (`parseTicketFlag`), persists `Ticket`, emits `spec.linked`.
6. Report `linked <KEY> (<provider>)` or the hint.

## Risks

- Overconfident detection of a bare key that is a code/doc example → mitigated by cue/prefix
  gating + ADR/RFC denylist + the `defaultTicketProvider` opt-in.
- Divergence from sync if the markdown rules drift → the spec pins the exact precedence and helper
  parity.
- Invalid `--ticket` must not abort creation → explicit fallback.

## Verification

`go -C cli generate ./...` (regenerate scaffold copy), `gofmt -l cli`, `go -C cli vet ./...`,
`go -C cli test ./...` (ticket_test.go + scaffold tests), `go -C cli build ./...` — all green.
Example-based checks for each tier + the malformed-`--ticket` fallback.
