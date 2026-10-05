## Context

This is a parity bug fix, not a Publishing redesign. The source scope is HUI-341, the captured Brian behavior, and the Vector draft coverage matrix. The approved adaptation retains one shared published set in `publish_snapshot`; it excludes learner-view switching, release channels, per-client pointers, and QA-client rollout.

## Goals / Non-Goals

**Goals:** restore the complete release-board and review workflow; make release eligibility explainable and enforced; reconcile API contracts and mutation invalidation; prove every HUI-341 criterion with tests.

**Non-Goals:** change non-Publishing surfaces, create a named review window or URL-only surface, trust actor/tenant/client data supplied by the request, or log catalog payloads/client directories.

## Decisions

### Evidence-first parity verification

For every parity-related surface, decide before code changes whether an exact-surface, exact-branch comparison with Brian is feasible. When feasible, run the relevant comparison skill first; same-surface visual work uses `design-compare`. The implementer must locate and review the run's same-scroll screenshots and Markdown gap report before reviewing implementation options. Confirmed gaps are the first review input and must be reconciled against both current code and Brian before a fix is selected.

The implementation record links the actual tool-emitted locations for screenshots and the Markdown gap report, alongside surface, branch/reference, viewport, scroll offsets, confirmed gaps, and the reconciliation decision. No artifact path is assumed by this change. If a run cannot be produced, the record must state the attempted comparison and concrete unavailability reason, link the substitute current-code/Brian evidence, mark the limitation, and explain the reconciliation; it may not represent visual-comparison coverage as complete.

### Single reachable review surface

Use one in-app, retargetable drawer/modal/portal opened from the existing Publishing row action. It owns buffered edits and requests confirmation before a retarget discards them. This preserves Brian-equivalent behavior while preventing duplicate windows and stale state.

### Explainable release eligibility

Model eligibility from approval/sign-off, hold, dependency closure, and recursive SCORM results. Board rows expose the reason for ineligibility; plan/publish independently validate the same gates so client state cannot bypass them.

### Refresh materialized state after every release mutation

Run the same invalidation/rebuild path after publish and rollback as after edit, alongside scheduled and explicit refresh. Publish stores each touched staging pre-image transactionally; rollback reapplies pre-images and then refreshes the materialized board/history.

### Shared closure and recursive gate evaluators

One closure evaluator distinguishes direct from implied selected rows and enforces `include_closure`. One recursive SCORM traversal returns normalized blocker path/reason for board, detail, reach, plan, and publish responses. Duplicating either rule by endpoint would drift behavior.

### Contract-first adapters

Keep the adapted snapshot model behind stable summary, board, refresh/recheck, reach, entity, review, confirm, edit, hold, gate, plan, publish, changelog, runs, and rollback contracts. Authorization derives entirely from authenticated context; errors have stable status/body mappings and OpenAPI stays generated from the implementation.

## Risks / Trade-offs

- [Stale eligibility after a mutation] → centralize invalidation and assert the next summary/board response.
- [Unintended dependency release] → make omitted closure a deterministic validation error and expose implied rows before confirmation.
- [Deep SCORM blocker hidden in UI] → return normalized recursive gate details and test nested content.
- [Unsaved review edits lost] → warn on retarget and retain edits until confirmed discard or save.
- [Visual parity regression] → test worksheet expansion, copy, accessible keyboard interaction, and all explicit state variants.
- [Parity fix based on stale or unverified assumptions] → require pre-change exact-surface evidence, evidence-first review, and an auditable unavailable-run fallback.

## Migration Plan

Ship persistence/API changes behind transactional migrations if required; generate OpenAPI; deploy the release board with contract/UI coverage; verify hash/diff → review → approval → publish → refresh/history → confirmed rollback → refresh/history. Roll back by applying recorded pre-images through the explicit rollback path, never by repointing published state.

## Open Questions

None: the draft locks the approved adapted snapshot model and the stated out-of-scope items.
