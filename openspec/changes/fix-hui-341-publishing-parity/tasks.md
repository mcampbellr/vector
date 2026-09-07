## 0. Pre-implementation parity verification

- [ ] 0.1 For each parity-related Publishing surface, decide and record whether an exact-surface, exact-branch comparison with Brian is feasible before code changes. When feasible, run the relevant comparison skill; use `design-compare` for same-surface visual comparison.
- [ ] 0.2 Before implementation review, locate and review that run's same-scroll screenshots and Markdown gap report. Use confirmed gaps as the first review input, then reconcile them with current code and Brian.
- [ ] 0.3 Link the actual generated screenshot and Markdown-report locations with the surface, branch/reference, viewport, scroll offsets, confirmed gaps, and reconciliation decision. If unavailable, record the attempted run, concrete reason, substitute evidence from current code and Brian, and the resulting limitation; do not claim unproduced visual-comparison coverage.

## 1. Release-board parity

- [ ] 1.1 Replace any remaining Redis plan-queue path with the hash/diff release board and render comparison/hash-duration plus required grouping/data columns.
- [ ] 1.2 Implement accessible per-row worksheet expansion with final before/after detail, “What changed”, and “Who sees it”.
- [ ] 1.3 Restore filters and explicit empty, idle, loading, success, error, forbidden, disabled, timeout/still-working, retry, hold, restore, dependency, and safety-note copy through `assembly.publishing.*`.
- [ ] 1.4 Surface detailed reach/client/dispatch information consistently in row, detail, and release confirmation views.
- [ ] 1.5 Surface SCORM status and recursive-gate warnings and prevent failed gates from appearing eligible.

## 2. Review and selection workflow

- [ ] 2.1 Implement one retargetable in-app review drawer/modal/portal from the reachable Publishing row action, with loading/retry and unsaved-edit discard protection.
- [ ] 2.2 Render final-form review variants for video pairs, reorder states, quiz Q&A, email, and field/media/HTML/JSON detail; buffer editable unchanged text locally.
- [ ] 2.3 Synchronize review/approval indicators with board state and gate row/bulk/select-all/Send-to-live selection on sign-off, hold, dependency, and SCORM eligibility.
- [ ] 2.4 Implement no-reload Check now, Full re-check, two-step Send to live, and duplicate-submission prevention.

## 3. Persistence, refresh, and release safeguards

- [ ] 3.1 Complete snapshot, materialized board, run, run-row pre-image, review, hold, and changelog persistence for the approved adapted model.
- [ ] 3.2 Implement publish as one transaction copying selected staging rows and recording every touched pre-image; implement explicitly confirmed rollback by reapplying those pre-images.
- [ ] 3.3 Invalidate/rebuild materialized board state after publish and rollback as well as edit, scheduled refresh, and explicit refresh/recheck.
- [ ] 3.4 Centralize dependency closure and `include_closure` handling for recheck, plan, and publish, with direct/implied rows and deterministic omission errors.
- [ ] 3.5 Implement recursive SCORM traversal and normalized blocker detail across gate, board, detail, reach, plan, and publish.

## 4. API contracts and authorization

- [ ] 4.1 Reconcile `GET summary`, `GET board`, and `POST refresh` refresh/recheck states and stable error responses.
- [ ] 4.2 Reconcile reach, entity, review, confirm, edit, hold/restore, and gate/scorm contracts, tenant scope, and authorization.
- [ ] 4.3 Reconcile plan, publish, changelog, runs, and rollback contracts for history/versioning, transactional behavior, gates, and error mapping.
- [ ] 4.4 Derive actor, tenant, client, and role solely from authentication context; ensure logging excludes catalog payloads, client directories, credentials, and tokens.
- [ ] 4.5 Regenerate OpenAPI for changed routes.

## 5. Coverage matrix and verification

- [ ] 5.1 Add UI coverage for worksheet expansion, review retargeting/edits, sign-off selection, SCORM/reach details, filters, holds, dependencies, and all board states.
- [ ] 5.2 Add unit and request-level contract coverage for refresh/recheck, review/confirm, hold/restore, release, reach, history/changelog, rollback/versioning, closure, recursive SCORM, and 4xx/5xx errors.
- [ ] 5.3 Exercise the HUI-341 happy path: hash/diff → review → approval → eligible selection → publish → refreshed board/history → confirmed rollback → refreshed board/history.
- [ ] 5.4 Exercise empty catalog, non-admin rejection, forbidden UI, concurrent edit/version conflict, dependency conflict, hold/restore, failed nested SCORM, timeout, retry, and failure mapping.
- [ ] 5.5 Record the final matrix showing each HUI-341 criterion and each draft finding as covered by an implementation task and an executable verification.
- [ ] 5.6 Verify the parity-verification protocol for every changed parity surface: pre-change evidence is linked and reviewed first, or the unavailable-run fallback is complete and its limitation is explicit.
