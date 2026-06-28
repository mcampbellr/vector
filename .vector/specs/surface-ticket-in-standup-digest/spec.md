# Spec: Surface external ticket in standup digest

## 1. Goal

Build the enrichment that makes Vector's standup digest **surface a spec's linked external
ticket key alongside its slug**, wherever the digest identifies a spec.

This feature lets a developer running `/vector:standup` report progress using the ticket the
team already tracks (e.g. `ACME-123`) **next to** the internal slug (e.g. `add-standup-digest`),
so a spec is easy to locate in the external tracker without dropping the slug that joins
everything internally. When a spec has no linked ticket, the digest is unchanged (slug only).

The decision is **keep both** (ticket key + slug), with the ticket shown as **the key only**
(no provider label, no URL in prose), in the **per-spec summaries, the global paragraph, and
the board's Standup view**.

## 2. Scope

### Included in this phase

- **Projection field**: `standup.SpecActivity` gains a `Ticket *state.Ticket` field
  (`cli/internal/standup/standup.go`), `omitempty`.
- **Projection enrichment**: `enrichProjection` (`cli/cmd/vector/standup.go:88`) populates
  `sa.Ticket` from the spec it already reads from the store (`store.ReadSpec(sa.ID)`), so both
  `vector standup --json` (the agent input) and the commit path carry the ticket.
- **Persisted digest field**: `state.StandupSpecDigest` gains a `Ticket *Ticket` field
  (`cli/internal/state/standup.go`), `omitempty`, copied at commit
  (`runStandupCommit`, `cli/cmd/vector/standup.go:162`).
- **Agent prose**: `vector-standup-writer` (`kit/agents/vector-standup-writer.md`) receives the
  `ticket` in each `perSpec` entry and, when present, leads with `ticket.key` next to the slug
  in both the per-spec summaries and the global paragraph; the slug stays.
- **Board Standup view**: `StandupSpecRow` (`web/src/components/StandupView/StandupSpecRow.tsx`)
  renders a ticket badge next to the slug, mirroring `SpecCard`'s existing badge; `standup.ts`
  types gain the `ticket?` field.
- **Tests**: projection-enrichment and commit round-trip carry the ticket; web typecheck/build green.

### Out of scope

- Changing the spec's slug `id` — it remains the join key the commit uses to map prose to specs
  (`cli/cmd/vector/standup.go:158`). The ticket is **additive**, never a replacement of `id`.
- Auto-linking tickets (already owned by `/vector:link` and the raw/sync auto-detection at
  `cli/cmd/vector/main.go:309`).
- Including the ticket **URL** in the agent prose (the URL lives only in the board badge `title`,
  as `SpecCard` already does).
- Provider labels in prose (no `Jira ACME-123` — key only).
- Supporting multiple tickets per spec (`SpecState.Ticket` is a single pointer) or re-linking.
- Changing the `vector standup commit` contract, the marker logic, or the activity-log schema.
- Surfacing the ticket in any other command (`/vector:daily`, etc.) — standup only.

The agent must not implement anything outside the scope above, even if it seems related.

---

## 3. Project technologies and conventions

### Stack

- Language: Go (single module, stdlib only) for `cli/`; TypeScript + React 19 + Vite for `web/`.
- Package manager: `go` toolchain for `cli/`; `npm` for `web/`.
- State management (web): SSE projection of `cli/`'s HTTP API; no canonical client state.
- API client (web): hand-mirrored types in `web/src/types/*` (no typegen yet).
- Testing: Go `testing` (table-driven); `web/` typecheck + build as the gate.
- Agent: the `vector-standup-writer` Haiku subagent (`kit/agents/vector-standup-writer.md`).

### Relevant versions

- Go: `TBD — ver Open questions` (confirm exact version in `cli/go.mod`; not changed here).
- React: 19 (`web/package.json`; not changed here).

No new libraries, APIs, flags, or patterns beyond those already in the project.

### Existing patterns to respect

- `state.Ticket{Provider, Key, URL, Auto}` (`cli/internal/state/types.go:113`, the `Ticket`
  struct) is the existing shape; reuse it, do not redefine.
- `SpecActivity` is the projection vehicle; `enrichProjection` is the existing store-backed
  enrichment seam (it already reads each `spec` to fill `Title`/`LastStatus`).
- The agent only transforms its input JSON; it never calls the binary or writes state
  (`kit/agents/vector-standup-writer.md`, Hard rules).
- `SpecActivity.ID` / `StandupSpecDigest.ID` is the slug and the commit join key — unchanged.
- Web mirrors the Go contract by hand in `web/src/types/standup.ts`; `SpecCard` already renders
  a ticket badge (`web/src/components/SpecCard/SpecCard.tsx:20`) — mirror it, don't invent new UX.

---

## 4. Prerequisites

Before starting this phase the following must already exist (all verified present):

- [x] `state.Ticket` struct and `SpecState.Ticket` pointer (`cli/internal/state/types.go`).
- [x] `standup.SpecActivity` projection and `standup.Project` (`cli/internal/standup/standup.go:48`).
- [x] `enrichProjection` store-backed enrichment (`cli/cmd/vector/standup.go:88`).
- [x] `state.StandupSpecDigest` persisted shape and `WriteStandup`/`ReadStandup`
      (`cli/internal/state/standup.go`).
- [x] `vector-standup-writer` agent with the current input/output contract
      (`kit/agents/vector-standup-writer.md`).
- [x] `GET /api/standup` handler serving the persisted digest (`cli/internal/board/server.go:55`).
- [x] `StandupSpecRow` + `standup.ts` types in `web/`; `SpecCard` ticket badge as the reference.
- [x] `Store.LinkSpec` validates a ticket has a non-empty `Key` (so a persisted `Ticket` is never
      keyless — confirmed by `cli/internal/state/store_test.go:355`).

If a prerequisite is missing, stop and report exactly what is absent. Do not invent contracts.

---

## 5. Architecture

### Pattern to use

Read-only, deterministic projection enriched in the command layer → JSON handed to the agent →
agent produces prose → binary persists the rebuilt structural digest. The ticket is an additive
field threaded along the existing path; no new layer, no signature change to `Project`.

### Affected layers

- presentation (web): yes — `StandupSpecRow` renders a ticket badge; `standup.ts` adds the type.
- application/use-cases (cli command): yes — `enrichProjection` fills `Ticket`; `runStandupCommit`
  copies it into the persisted digest.
- domain (cli state): yes — `StandupSpecDigest` gains a `Ticket` field. `SpecState`/`Ticket`
  unchanged.
- data/infrastructure (cli projection): yes — `SpecActivity` gains a `Ticket` field. The pure
  `Project` function is **not** given store access; enrichment stays in the caller.
- shared/common: no.

### Expected flow

1. `vector standup --json` runs `standup.Project(events, from)` then `enrichProjection(store, &proj)`.
2. `enrichProjection` reads each `spec` from the store (it already does) and sets
   `sa.Ticket = spec.Ticket` (nil when unlinked).
3. The `/vector:standup` command pipes the enriched JSON to `vector-standup-writer`.
4. The agent, when `ticket` is present, leads each per-spec summary and any global mention with
   `ticket.key` next to the slug; otherwise slug only. Its output `perSpec[].id` stays the slug.
5. `vector standup commit` re-projects, re-enriches, and copies `sa.Ticket` into
   `StandupSpecDigest.Ticket`, then persists `.vector/local/standup.json`.
6. `GET /api/standup` serves the digest; the board's `StandupSpecRow` renders the ticket badge
   and shows the agent's prose (which already names the key).

### Location of new files

No new files. All changes are edits to existing files listed in section 6.

---

## 6. Files to create or modify

| Path | Action | Purpose | Project example to follow |
|---|---|---|---|
| `cli/internal/standup/standup.go` | MODIFY | Add `Ticket *state.Ticket` to `SpecActivity` (omitempty) | `cli/internal/standup/standup.go` (the existing `Title`/`LastStatus` fields on the same struct) |
| `cli/cmd/vector/standup.go` | MODIFY | `enrichProjection` sets `sa.Ticket = spec.Ticket`; `runStandupCommit` copies `Ticket` into `StandupSpecDigest` | `cli/cmd/vector/standup.go` (the `Title`/`LastStatus` enrichment already in `enrichProjection`) |
| `cli/internal/state/standup.go` | MODIFY | Add `Ticket *Ticket` to `StandupSpecDigest` (omitempty) | `cli/internal/state/standup.go` (the existing `StandupSpecDigest` fields) |
| `kit/agents/vector-standup-writer.md` | MODIFY | Add `ticket` to the input example; add the rule to lead with `ticket.key` next to the slug when present, key-only, slug verbatim in `id` | `kit/agents/vector-standup-writer.md` (the existing Input/Hard-rules/Output sections) |
| `web/src/types/standup.ts` | MODIFY | Add `ticket?: Ticket` to `StandupSpecDigest` (import `Ticket` from `./board`) | `web/src/types/board.ts` (`Ticket` + `Card.ticket`) |
| `web/src/components/StandupView/StandupSpecRow.tsx` | MODIFY | Render a ticket badge next to the slug when `spec.ticket` is set | `web/src/components/SpecCard/SpecCard.tsx:20`–`23` |
| `cli/internal/standup/standup_test.go` | MODIFY | Keep projection tests green; `Project` leaves `Ticket` nil (no store), asserted unchanged | `cli/internal/standup/standup_test.go` (the existing table cases) |
| `cli/cmd/vector/standup_test.go` | NUEVO | Assert `enrichProjection` sets `Ticket` and commit round-trips it into the persisted digest | `cli/internal/board/board_test.go` (table style) |

### Detail per file

#### cli/internal/standup/standup.go

Action: MODIFY

- Add `Ticket *state.Ticket \`json:"ticket,omitempty"\`` to `SpecActivity` (after `LastChanged`/
  `Title`). The `state` package is already imported.
- Do **not** change `Project` — it stays store-free; the field is left nil there and filled by
  the caller's `enrichProjection`.
- Do not change `ID` or any join behavior.

#### cli/cmd/vector/standup.go

Action: MODIFY

- In `enrichProjection` (line 88), after reading `spec`, set `sa.Ticket = spec.Ticket`
  (assign unconditionally; `spec.Ticket` is nil for unlinked specs, which is the correct value).
- In `runStandupCommit` (the `perSpec` assembly at line 162), add `Ticket: sa.Ticket` to the
  `state.StandupSpecDigest{...}` literal.

Constraints:

- Do not change the `agentDigest` input shape (the agent still sends only `id` + `summary`); the
  ticket reaches the persisted digest from the fresh projection, not from the agent payload.
- Do not change the marker logic or the "write nothing on invalid JSON" behavior.

#### cli/internal/state/standup.go

Action: MODIFY

- Add `Ticket *Ticket \`json:"ticket,omitempty"\`` to `StandupSpecDigest`. `Ticket` is defined in
  the same `state` package.

Restrictions:

- Do not bump `StandupSchemaVersion` unless a reviewer requires it — the field is additive and
  `omitempty`, so old digests deserialize unchanged. `TBD — ver Open questions` if review
  disagrees.

#### kit/agents/vector-standup-writer.md

Action: MODIFY

Required changes:

- Extend the Input example so each `perSpec` entry can include:
  `"ticket": { "provider": "jira", "key": "ACME-123", "url": "https://...", "auto": false }`.
- Add a Hard rule: "When a `ticket` is present, lead the spec's mention with its **key** (e.g.
  `ACME-123`) shown **next to** the slug, in both the per-spec summary and any global-paragraph
  mention — e.g. `ACME-123 (add-standup-digest) reached review`. Use the **key only** — never the
  URL or provider name. When there is no `ticket`, use the slug alone (current behavior)."
- Reaffirm: the output `perSpec[].id` stays the **slug** verbatim — it is the commit join key;
  never put the ticket key in `id`.

Restrictions:

- Do not change the output JSON shape (`{ global, perSpec:[{id, summary}] }`).
- Do not invent tickets not present in the input.

#### web/src/types/standup.ts

Action: MODIFY

- Add `ticket?: Ticket` to `StandupSpecDigest`, importing `Ticket` from `./board`.

#### web/src/components/StandupView/StandupSpecRow.tsx

Action: MODIFY

- When `spec.ticket` is set, render a ticket badge next to the existing `specId` span, mirroring
  `SpecCard.tsx:20`–`23` (badge text = `spec.ticket.key`, `title` = `spec.ticket.url`). Reuse the
  existing badge styling pattern; do not introduce a new component file for a one-line badge here
  unless the markup grows (see `standards/typescript-react.md`).

Restrictions:

- Keep the slug (`spec.id`) visible — both are shown.
- Do not change the row's other content (title, status pill, change count, timeline).

---

## 7. API Contract

No HTTP contract change. The integration surfaces are:

- `vector standup --json` → the `standup.Projection` JSON (gains an optional `perSpec[].ticket`).
- The agent stdin/stdout contract (input gains optional `ticket`; output shape unchanged).
- `GET /api/standup` → the persisted `StandupDigest` (gains an optional `perSpec[].ticket`).

All three are additive and `omitempty`, so existing consumers are unaffected. There is no
`docs/api-contract.md` for this internal projection; the Go structs are the source of truth and
`web/src/types/standup.ts` mirrors them by hand.

### Endpoints involved

- GET /api/standup (response shape extended, additive)

---

## 8. Success criteria

The implementation is correct when:

- [ ] `standup.SpecActivity` has a nullable `Ticket` field; `Project` is unchanged and store-free.
- [ ] `enrichProjection` sets `sa.Ticket` from `spec.Ticket` (nil for unlinked specs).
- [ ] `vector standup --json` emits `perSpec[].ticket` for linked specs and omits it otherwise.
- [ ] `state.StandupSpecDigest` has a nullable `Ticket`; `runStandupCommit` round-trips it.
- [ ] The agent's prose leads with `ticket.key` next to the slug (per-spec + global) when present,
      slug-only otherwise; output `perSpec[].id` is always the slug.
- [ ] `GET /api/standup` serves the ticket; `StandupSpecRow` renders the badge next to the slug.
- [ ] No regression: marker advance, "write nothing on invalid JSON", empty-period handling,
      `/api/activity`, and the board's other views are intact.
- [ ] No Go vet/test failures; no TypeScript errors; web build succeeds (needed for the embed).

### Required tests

- [ ] `enrichProjection`: a linked spec gets `Ticket` set; an unlinked spec keeps `Ticket == nil`.
- [ ] `runStandupCommit` round-trip: the persisted `StandupSpecDigest` carries the ticket; re-read
      via `ReadStandup` returns it.
- [ ] Empty period: no specs, no ticket, digest unchanged (existing case still green).
- [ ] (web) typecheck passes with the new `ticket?` field and the badge render.

### Verification commands

```bash
go -C cli vet ./...
go -C cli test ./...
npm --prefix web run typecheck   # web/package.json → "typecheck": "tsc -b --noEmit"
npm --prefix web run build       # web/package.json → "build": "tsc -b && vite build"
```

The phase is not complete if any of these fail.

---

## 9. UX criteria

This is a CLI/agent + board feature; the UX is the digest text and the board badge.

### Prose (agent)

- When a spec has a ticket: the mention leads with the **key** next to the slug, e.g.
  `ACME-123 (add-standup-digest) reached review`. Key only — no URL, no provider name.
- When a spec has no ticket: slug only, exactly as today.
- Both the per-spec summary and the global paragraph follow this rule.

### Board Standup view

- `StandupSpecRow` shows a ticket badge next to the slug when `spec.ticket` is set, matching the
  look of `SpecCard`'s badge (key as text, URL as the `title` tooltip).
- The slug stays visible alongside the badge (keep both).
- No ticket → no badge; the row is unchanged.

### Accessibility

- The badge has the ticket URL as its `title` (as `SpecCard` does). It is text, not an icon-only
  control, so it is legible to assistive tech.

---

## 10. Decisions made

These are settled; the agent must not re-litigate them:

- **Keep both** ticket key + slug, everywhere the digest names a spec (chosen by the user).
- **Key only** in prose — no URL, no provider label (the URL lives in the board badge `title`).
- Applies to **per-spec summaries, the global paragraph, and the board Standup view**.
- The spec **slug `id` is immutable** and remains the commit join key; the ticket is additive.
- Enrichment lives in **`enrichProjection`** (the command layer that already reads the store), not
  in the pure `Project` function — `Project` keeps no store dependency.
- The ticket reaches the persisted digest from the **fresh projection at commit**, not from the
  agent payload — the `agentDigest` input shape (`id` + `summary`) does not change.
- Reuse the existing `state.Ticket` struct and `SpecCard` badge pattern; no new types or UX.

If the agent sees a seemingly better alternative, report it as an observation, do not implement it.

---

## 11. Edge cases

### No linked ticket

- `spec.Ticket == nil` → `SpecActivity.Ticket` / `StandupSpecDigest.Ticket` stay nil, omitted from
  JSON. Agent uses the slug alone; board renders no badge. (The common case; must stay unchanged.)

### Ticket present

- `Store.LinkSpec` rejects a keyless ticket (`store_test.go:355`), so a persisted `Ticket` always
  has a `Key`. The agent and badge can rely on `key` being non-empty when `ticket` is present.

### Spec read fails during enrichment

- `enrichProjection` already `continue`s when `store.ReadSpec` errors; in that case `Ticket` stays
  nil (slug-only). Do not change this fallback.

### Same ticket key on multiple specs

- Allowed (no uniqueness constraint). Each spec reports its own ticket independently.

### Empty period

- `perSpec` empty → global is `no activity since last standup`, no tickets involved (existing
  behavior, unchanged).

### Agent receives a malformed/partial ticket

- The agent must not crash the prose: if `ticket` is absent or missing `key`, fall back to the
  slug. Emit valid output JSON regardless.

### Board offline / local server not running

- `GET /api/standup` is served by the local `vector serve` binary. If it is down, the existing
  `useStandup` error/empty handling in `StandupView` applies unchanged — this change adds no new
  fetch and no new failure mode. The ticket badge simply never renders (no data).

### Fetch timeout (board UI)

- N/A — this change introduces no new request. The badge is derived from the already-fetched
  `/api/standup` payload; timeout behavior is whatever `useStandup`/`useSpecActivity` already do.

### Double-submit

- N/A — `GET /api/standup` is read-only; there is no form or mutation in this feature.

### HTTP error codes (400/401/403/404/409/422/429/500)

- N/A — the binary is a **local, auth-free, read-only** server. There is no authentication
  (401/403), no write conflict (409), and no body validation (400/422/429) on this path. A
  missing/empty digest is served as `{}` (`server.go:63`), not a 404. The board's generic
  fetch-error state covers any transport-level failure; no per-code handling is added.

---

## 12. Required UI states

The digest is read-only; the board's `StandupView` already owns loading/success/error/empty
states for the whole digest. This change only adds an inline badge inside a row.

| State | What is shown | What the user can do |
|---|---|---|
| spec with ticket | slug + ticket badge (key, URL as tooltip) | hover to read the URL; read the prose naming the key |
| spec without ticket | slug only (no badge) | read the prose (slug only) |
| empty period | "no activity since last standup" | nothing to act on |
| idle / loading / success / error | N/A — owned by `StandupView` (see Section 9); this change is an inline badge inside an already-rendered row, not a new fetch or screen | (delegated to `StandupView`) |
| offline | N/A — no new request; badge renders from the already-fetched `/api/standup` payload | (delegated to `StandupView`) |

---

## 13. Validations

### Data validations (cli)

| Field | Rule | Message |
|---|---|---|
| `SpecActivity.Ticket` | nullable; copied verbatim from `spec.Ticket` | — (internal; nil = no ticket) |
| `StandupSpecDigest.Ticket` | nullable; copied from the projection at commit | — (internal) |

No new user-facing validation. Ticket completeness is already guaranteed at link time by
`Store.LinkSpec`.

### Server validations

None changed. `vector standup commit` keeps its "invalid digest json → write nothing" rule.

---

## 14. Security and permissions

- Ticket keys/URLs (Jira/Linear/GitHub) are non-secret references; no tokens or secrets are
  exposed by surfacing them.
- The Haiku agent receives only the projection JSON (public references); no credentials.
- `.vector/local/standup.json` and `activity.jsonl` stay personal and gitignored — unchanged.

---

## 15. Observability and logging

- No new events. The ticket already exists in `SpecState.Ticket`; this feature only reads/threads
  it. The append-only activity log is untouched.
- Use only existing error paths (`enrichProjection` already swallows a failed `ReadSpec` by
  continuing). Do not add new logging channels.

---

## 16. i18n / visible text

Vector has no i18n layer; CLI/board strings are English in code. The agent prose follows the
**conversation language** (existing rule in `vector-standup-writer.md`), but the ticket **key**
and the slug are emitted verbatim regardless of language.

| Key | Text |
|---|---|
| (none new) | The ticket badge shows `spec.ticket.key` verbatim; no translatable string is added |

---

## 17. Performance

- `enrichProjection` already calls `store.ReadSpec` per spec; reading `spec.Ticket` is a struct
  field access — zero additional I/O.
- The projection is bounded by the number of active specs in the window; no new allocation of
  note. The agent input grows by one small object per linked spec.

---

## 18. Restrictions

The agent must not:

- Change the `SpecActivity.ID` / `StandupSpecDigest.ID` slug or the commit join key.
- Put the ticket key into the `id` field of the agent output.
- Give `Project` store access or change its signature.
- Change the `agentDigest` input shape or the `vector standup commit` contract.
- Add new dependencies, events, or logging channels.
- Include the ticket URL or provider name in the prose (key only).
- Add a new web component file for the inline badge unless the markup genuinely grows.
- Refactor unrelated code or change other board views.
- Ignore lint/typecheck/test failures.

---

## 19. Deliverables

On completion:

- [ ] `SpecActivity.Ticket` and `StandupSpecDigest.Ticket` fields added (omitempty).
- [ ] `enrichProjection` fills `Ticket`; `runStandupCommit` round-trips it.
- [ ] `vector-standup-writer.md` updated: input example + key-next-to-slug rule + slug-verbatim id.
- [ ] `web/src/types/standup.ts` and `StandupSpecRow.tsx` render the badge next to the slug.
- [ ] Go tests for enrichment + commit round-trip; existing tests still green.
- [ ] Gate green: `go vet`, `go test`, web typecheck, web build.
- [ ] Binary rebuilt + reinstalled to `~/.local/bin/vector` (dogfooding uses the PATH binary).

---

## 20. Final checklist for the agent

Before delivering, verify:

- [ ] Read this whole spec.
- [ ] Confirmed `state.Ticket` and `enrichProjection` are the reuse seams (no new types/layers).
- [ ] Confirmed `SpecActivity.ID` / `StandupSpecDigest.ID` slug and join key are unchanged.
- [ ] Updated the agent prompt: key-next-to-slug, key-only, slug verbatim in `id`.
- [ ] Only modified the files listed in section 6 (or justified any exception).
- [ ] Followed real project examples (`enrichProjection`, `SpecCard` badge).
- [ ] Added table-driven Go tests for ticket enrichment + commit round-trip.
- [ ] No unauthorized dependencies; no settled decisions changed.
- [ ] Ran `go vet`, `go test`, web typecheck, web build.
- [ ] Rebuilt and reinstalled the `vector` binary.
- [ ] Left no temporary logs or unjustified TODOs.
