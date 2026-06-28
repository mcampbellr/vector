# Design — add-vector-bug-command

## Context

`/vector:bug` mirrors `/vector:raw` (orchestrating project command + cheap research + Haiku refiner
+ Sonnet validation + binary-owned state writes) but adds **cause deduction**: it ties the bug to
the prior work that caused it. The persisted relation (`relatedTo[]`) is the durable product value;
everything else reuses existing Vector machinery. Source spec:
`.vector/specs/add-vector-bug-command/spec.md`.

## Key decisions (locked with the user)

- **Lifecycle = native `draft`** (like `/vector:raw`). The command authors and registers the card in
  `draft`; it does **not** create the OpenSpec change. Keeps the bug on the board and reuses
  `raw → propose → apply` without duplicating `/vector:propose` orchestration.
- **`relatedTo[]` = real Go state field**, not just prose or an event: persisted on `SpecState`,
  queryable, surfaced on board/API/web. The `spec.related` event is added *in addition*, for the
  timeline/standup. Rationale: the user wants a consultable record of the bug's cause.
- **`kind` in V1 = `spec` and `ticket`** only. The `git blame` commit is an *inference signal*, not
  a stored `kind`. A future `kind: commit`/`pr` is an open question, deliberately out of V1.
- **Infer, then ask.** `git blame`/`git log` deduce the cause; on ambiguity / multiple candidates /
  low confidence / no match → `AskUserQuestion` (candidates + "none" + manual entry). Never guess —
  agnosticism + no hallucinated links.
- **Own embedded refiner `vector-bug-refiner` (Haiku)**, not reuse of the global. Every kit agent is
  vendored/embedded except OpenSpec's own (`architecture/distribution-packaging.md`).
- **Validation reuses `vector-spec-validator` (Sonnet)** — the authored spec is a standard Vector
  spec; no new validator.
- **Token routing**: deduction/parse/resolution = main loop (cheap); refine = Haiku; validate =
  Sonnet; compose = main loop (`product/token-routing.md`).
- **Web read-only** for `relatedTo[]`: the panel only displays; all writes go through the binary
  (`architecture/state-model.md`, CLI-owns-writes).

## Data model

```go
type RelatedKind string   // "spec" | "ticket"
type RelatedSource string // "blame" | "manual"

type RelatedItem struct {
    Kind   RelatedKind   `json:"kind"`
    Ref    string        `json:"ref"`    // Vector spec id, or provider:key for tickets
    Source RelatedSource `json:"source"`
}

// SpecState gains:
RelatedTo []RelatedItem `json:"relatedTo,omitempty"`
```

- `omitempty` keeps existing specs byte-compatible (no `relatedTo` → unchanged read/serialize).
- Mirror the existing `Ticket` struct/persistence patterns in `types.go`/`store.go`.
- `RelateSpec(id, item)` is serialized by the `Store` mutex, idempotent on `{kind,ref}` (no dup, no
  double event), and does **not** touch the state machine (relating never changes `status`).
- Validation: `kind ∈ {spec,ticket}`; `ref` non-empty (if `kind=spec`, exists in `vector spec
  list`); `source ∈ {blame,manual}`, default `manual`.

## CLI contract (write source of truth)

- `vector spec create --title … --id <slug> --status draft [--related '<json-array>'] --body-file -
  [--json]` — creates the `draft` card, writes the doc, persists `relatedTo[]`. Invalid relation →
  **degrade**: create the card without relations and report (mirrors the `--ticket` fallback in
  `/vector:raw`); do not lose a valid card.
- `vector spec relate <id> --kind spec|ticket --ref <ref> [--source blame|manual] [--json]` —
  add/manage one relation (idempotent), append `spec.related`. Invalid input → reject the op.
- `vector spec list --json` — resolve spec candidates for cause mapping (new flag this phase).
- `vector spec route <id> --model … --baseline … --task … --tokens-in … --tokens-out …` — token
  meter (one call for the Haiku refine, one for the Sonnet validate).

## HTTP surface

No new write endpoints. `GET /api/board` gains a read-only `relatedTo` per card (projection change
in `cli/internal/board`); the SSE `/api/events` stream is unchanged. The command makes no HTTP
requests, so HTTP status codes (400/401/…/500) don't apply to its flow.

## Cause-deduction flow

1. Identify files/symbols cited in the raw report; run `git blame`/`git log -S`/`--grep` for suspect
   commits (scoped to those files; report progress; cap on huge repos and offer to continue without
   `relatedTo[]`).
2. Map each commit → a Vector spec (OpenSpec change name / id, resolved via `spec list --json`) or a
   ticket (commit trailer, e.g. `ACME-12`).
3. Unique + high confidence → seed `relatedTo[]` (`source: blame`). Ambiguous / multiple / low
   confidence / no match → `AskUserQuestion` (candidates + "none" + manual `source: manual`).
4. No `git` / non-git repo / files absent → skip deduction with a notice; author the bug without
   relations.

## Risks / edge cases

- **Re-invocation** with the same report → by design a *second distinct* `draft` card (no report
  dedup; each run is a new bug). User archives/closes duplicates. CLI runs sequentially → not
  concurrent.
- **Validator BLOCK** unresolved in ≤3 cycles → surface the report, do **not** register.
- **`git` timeout** on deep history → cut, report, continue without `relatedTo[]`; never block
  authoring.
- **Backward compatibility** — specs without `relatedTo` read/serialize identically; covered by a
  regression test.

## Out of scope

Implementing the fix (`/vector:apply`); creating the OpenSpec change (`/vector:propose`); storing
raw commits/PRs as a relation `kind` in V1; validating the related ticket/spec against the external
tracker (that stays in `/vector:link`); web editing of `relatedTo[]`; posting the bug to a tracker;
redesigning the spec template.
