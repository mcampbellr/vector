# Tasks — surface-ticket-in-standup-digest

## 1. Projection (cli/internal/standup)

- [x] 1.1 Add `Ticket *state.Ticket \`json:"ticket,omitempty"\`` to `SpecActivity` (after `Title`/`LastChanged`). `state` is already imported.
- [x] 1.2 Leave `Project` untouched — store-free; `Ticket` stays nil there (filled by the caller).
- [x] 1.3 `standup_test.go`: assert `Project` leaves `Ticket == nil` (no store); existing table cases stay green.

## 2. Command layer (cli/cmd/vector/standup.go)

- [x] 2.1 In `enrichProjection`, after reading `spec`, set `sa.Ticket = spec.Ticket` unconditionally (nil for unlinked specs). Keep the existing `ReadSpec` error `continue` fallback.
- [x] 2.2 In `runStandupCommit`'s `perSpec` assembly, add `Ticket: sa.Ticket` to the `state.StandupSpecDigest{...}` literal. Do not change the `agentDigest` input shape or the marker logic.

## 3. Persisted digest (cli/internal/state/standup.go)

- [x] 3.1 Add `Ticket *Ticket \`json:"ticket,omitempty"\`` to `StandupSpecDigest`. Do not bump `StandupSchemaVersion` (additive + omitempty).

## 4. Agent prose (kit/agents/vector-standup-writer.md)

- [x] 4.1 Extend the Input example: each `perSpec` entry can include `"ticket": { "provider": "jira", "key": "ACME-123", "url": "https://...", "auto": false }`.
- [x] 4.2 Add the Hard rule: when `ticket` is present, lead the spec's mention with its **key** next to the slug (per-spec summary + global paragraph) — e.g. `ACME-123 (add-standup-digest) reached review`; key only, no URL/provider; slug-only when no ticket.
- [x] 4.3 Reaffirm: output `perSpec[].id` stays the **slug** verbatim (commit join key); never the ticket key. Output JSON shape unchanged.

## 5. Web (board Standup view)

- [x] 5.1 `web/src/types/standup.ts`: add `ticket?: Ticket` to `StandupSpecDigest`, importing `Ticket` from `./board`.
- [x] 5.2 `web/src/components/StandupView/StandupSpecRow.tsx`: when `spec.ticket` is set, render a ticket badge next to the `specId` span (text = `spec.ticket.key`, `title` = `spec.ticket.url`), mirroring `SpecCard.tsx:20`–`23`. Keep the slug visible; no new component file for a one-line badge.

## 6. Tests + verification

- [x] 6.1 New `cli/cmd/vector/standup_test.go` (table-driven): `enrichProjection` sets `Ticket` for a linked spec and leaves it nil for an unlinked one; `runStandupCommit` round-trips it into the persisted digest, re-read via `ReadStandup`.
- [x] 6.2 Empty-period case stays green (no specs, no ticket, digest unchanged).
- [x] 6.3 Gate green: `go -C cli vet ./...`, `go -C cli test ./...`, `npm --prefix web run typecheck`, `npm --prefix web run build`. No regressions in marker advance, "write nothing on invalid JSON", `/api/activity`, other board views.
- [x] 6.4 Rebuild + reinstall the `vector` binary to `~/.local/bin/vector` (dogfooding uses the PATH binary).
