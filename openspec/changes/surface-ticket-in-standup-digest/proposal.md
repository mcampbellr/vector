# Surface external ticket in standup digest

## Why

A spec can already be linked to an external ticket (`SpecState.Ticket`, set by `/vector:link`
and the raw/sync auto-detection), but the standup digest only ever names a spec by its internal
slug. A dev running `/vector:standup` has to translate slugs back to the ticket the team tracks
(e.g. `ACME-123`) by hand. Surfacing the ticket key **next to** the slug lets the spec be located
in the external tracker without dropping the slug that joins everything internally.

## What changes

- **Projection field**: `standup.SpecActivity` gains a nullable `Ticket *state.Ticket`
  (`omitempty`). The pure `Project` function stays store-free and leaves it nil.
- **Projection enrichment**: `enrichProjection` (`cli/cmd/vector/standup.go`) sets
  `sa.Ticket = spec.Ticket` from the spec it already reads, so both `vector standup --json` (the
  agent input) and the commit path carry the ticket. Nil for unlinked specs.
- **Persisted digest field**: `state.StandupSpecDigest` gains a nullable `Ticket *Ticket`
  (`omitempty`), copied at commit in `runStandupCommit`.
- **Agent prose**: `vector-standup-writer` receives `ticket` per `perSpec` entry and, when
  present, leads with `ticket.key` next to the slug in both the per-spec summaries and the global
  paragraph — key only (no URL, no provider label). The output `perSpec[].id` stays the slug.
- **Board Standup view**: `StandupSpecRow` renders a ticket badge next to the slug, mirroring
  `SpecCard`'s existing badge; `web/src/types/standup.ts` gains the optional `ticket?` field.

The ticket is **additive** everywhere: when a spec has no linked ticket, the digest is unchanged
(slug only), and all fields are `omitempty`, so existing consumers and old digests are unaffected.

## Capabilities

### Modified Capabilities
- `standup-digest`: the digest now surfaces a spec's linked external ticket key alongside its
  slug in the per-spec summaries, the global paragraph, and the board Standup view.

<!-- No new capability: this enriches the existing standup-digest. activity-worklog is untouched. -->

## Impact

- `cli/internal/standup/standup.go` (`SpecActivity.Ticket`), `cli/cmd/vector/standup.go`
  (`enrichProjection`, `runStandupCommit`), `cli/internal/state/standup.go`
  (`StandupSpecDigest.Ticket`).
- `kit/agents/vector-standup-writer.md` (input example + key-next-to-slug rule, slug verbatim in `id`).
- `web/src/types/standup.ts` (`ticket?: Ticket`), `web/src/components/StandupView/StandupSpecRow.tsx`
  (ticket badge).
- Tests: `cli/internal/standup/standup_test.go` (Project stays store-free → Ticket nil),
  new `cli/cmd/vector/standup_test.go` (enrichment + commit round-trip).
- No new dependencies, events, logging channels, or HTTP contract change (response is additive).
  `GET /api/standup` gains an optional `perSpec[].ticket`; `StandupSchemaVersion` is **not** bumped
  (additive + omitempty).

Authored spec: `.vector/specs/surface-ticket-in-standup-digest/spec.md`.
