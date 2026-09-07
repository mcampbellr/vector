# Bug / Fix: Publishing parity check — Assembly Station (HUI-341)

## Outcome
Restore behavioral parity for the Publishing surface only. This card covers every remaining Publishing finding from HUI-341 across the Assembly Station frontend and backend. It does not change any non-Publishing surface, create an OpenSpec change, or alter Somnio state.

## Source of truth and constraints
- Behavioral/API reference: Brian's implementation at `a2ed66f` and the captured Publishing contracts.
- Internal storage remains the approved adaptation: `publish_snapshot` represents published state; it must not introduce client release pointers, channels, or QA-client rollout.
- One shared published set; learner view continues to read staging (the learner flip is out of scope).
- Existing clickable Publishing navigation remains the entry point. Review is opened from the board's row action; no URL-only surface or named browser window.
- All endpoint authorization uses authenticated request context; no tenant, actor, client, or role is trusted from request body/query. Do not log catalog payloads or client directories.

## Acceptance criteria carried from HUI-341

### Board and release workflow
- [ ] Publishing replaces the Redis plan-queue model completely; no dual plan-queue and diff/release model remains.
- [ ] Opening Publishing renders a hash/diff release board with comparison/hash-duration status, grouping by Courses, Videos, Roleplay, Badges, and Everything else.
- [ ] The board provides CHANGE, WHAT, KIND, TAB, WHAT CHANGED, ROWS, REACHES, OWN EDITION, IN SCORM, and REVIEWED data; each visible value is sourced from the corresponding contract.
- [ ] Per-row Review, bulk selection, select-all, two-step Send to live, Check now, and Full re-check work without a full page reload.
- [ ] Board filtering, empty, idle, loading, success, error, forbidden, in-flight disabled, timeout/still-working, retry, hold, restore, and dependency states have explicit user-facing copy. Empty shows “No pending changes” and preserves Check now; forbidden is never a blank page.
- [ ] Text is localized through `packages/i18n` under `assembly.publishing.*`, using the captured source copy.

### Review behavior
- [ ] Review is a single in-app drawer/modal/portal, not a named browser window or a separate route.
- [ ] Review displays one pending change in final form by kind: video pair, both reorder states, rendered quiz Q&A, rendered email, and applicable field/media/HTML/JSON detail.
- [ ] Unchanged plain text, including “About this”, is editable in the review surface; edits are buffered locally and no request is sent per keystroke.
- [ ] Selecting Review for another row retargets the same instance, shows loading/retry as needed, and warns before discarding unsaved edits; it never opens a second instance or leaks stale data.
- [ ] Review/approval state transitions update board selection eligibility and reviewed indicators without stale client state.

### Backend model and API contract
- [ ] Snapshot, materialized board, run, run-row pre-image, review, hold, and changelog persistence are present and consistent with the approved adapted model.
- [ ] Catalog hashing/diff uses the publishable registry and write-counter gate; a scheduled incremental refresh runs, and refresh is also triggered after publish, edit, and rollback.
- [ ] `GET summary`, `GET board`, and `POST refresh` support refresh/recheck semantics expected by the frontend, including stable status/error responses.
- [ ] `POST reach`, `GET entity`, `GET review`, `POST/DELETE confirm`, `POST edit`, `POST/DELETE hold`, and `GET gate/scorm` honor the captured response contracts, authorization, tenant scope, and errors.
- [ ] `POST plan`, `POST publish`, `GET changelog`, `GET runs`, and `POST runs/:id/rollback` honor version/run history, review/hold/confirmation gates, transactional semantics, and error mapping.
- [ ] Publish copies selected staging rows into the published snapshot in one transaction and records a pre-image for every touched row. Rollback reapplies those pre-images; it does not rehash or repoint state.
- [ ] Dependency closure is calculated and exposed/validated consistently wherever a selected change requires related rows; `include_closure` behavior cannot silently omit dependencies.
- [ ] SCORM status/gates are recursively evaluated through applicable nested content and appear consistently in board, detail, reach, and send eligibility contracts.
- [ ] OpenAPI is regenerated for changed publishing routes; unit and request-level coverage exercise every changed controller/DTO/service contract.

## Coverage matrix for remaining gaps

| Area | Finding to close | Acceptance evidence |
|---|---|---|
| Frontend: expandable worksheet | Restore an expandable worksheet/table row for every pending change, with readable before/after fields and explicit “What changed” and “Who sees it”. | Expanding/collapsing a row is keyboard-safe, keeps context, and renders prose plus source data without IDs-only output. |
| Frontend: ReviewWindow parity | Reconcile the review window/drawer behavior to Brian's reference: one retargetable in-app instance, final-form renderers, editable unchanged text, verdict/approval feedback. | Review different rows successively; no new window, stale content, or silent loss of draft edits. |
| Frontend: selection gates | Make selection and Send to live conditional on required review approval/sign-off and non-held/non-blocked status. | Rows that lack sign-off, are held, or fail a gate are visibly ineligible with the reason; eligible rows remain selectable. |
| Frontend: SCORM | Surface SCORM indicators, recursive gate warnings, and their effect on release eligibility. | A row shows current SCORM state, warning/error copy, and cannot bypass a failed gate. |
| Frontend: reach and dispatch | Restore detailed reach: affected clients/audience, client-specific implications, and dispatch/release outcome detail. | Row/detail/release confirmation agree on reach, client counts/names where authorized, and dispatch result. |
| Frontend: filters and states | Complete filters, all state copy, hold/restore flow, dependencies, safety notes, errors, and bounded retry/timeout behavior. | Each state is testable and understandable without developer tooling; actions prevent duplicate submits. |
| Backend: invalidation | Invalidate/rebuild materialized board state after publish and rollback, in addition to edit and on-demand/scheduled refresh. | The next summary/board response reflects the mutation without an application restart or stale cached eligibility. |
| Backend: closure | Implement and test dependency closure plus `include_closure` request/response semantics for plan/publish/recheck. | Directly selected and implied dependencies are distinguishable; omitted closure causes a deterministic validation error, not partial release. |
| Backend: recursive SCORM | Evaluate SCORM gates recursively across dependent/nested assets and return normalized blocker detail. | Deeply nested failing assets block release and identify the path/reason in gate, detail, and plan responses. |
| Backend: contracts | Close contract gaps for refresh/recheck, review/confirm, hold/restore, release, reach, history/changelog, rollback/versioning, and 4xx/5xx error bodies. | Contract tests cover success, forbidden, validation, conflict/concurrency, timeout, and failure results; OpenAPI matches. |

## Regression and security criteria
- [ ] Happy path covers hash/diff → Review → approval → eligible selection → publish → refreshed board/history → rollback → refreshed board/history.
- [ ] Test empty catalog, non-admin rejection, forbidden UI, concurrent edit/version conflict, dependency conflict, hold/restore, failed recursive SCORM, timeout, and backend 4xx/5xx mapping.
- [ ] Rollback requires explicit confirmation. Every publish, edit, refresh, hold, restore, and rollback derives actor/tenant/client scope from authentication context.
- [ ] No full catalog payloads, client directories, credentials, or tokens are logged.

## Explicitly out of scope
- Learner-view conversion from staging to published snapshot.
- Per-client pointers, release channels, or QA-client staged rollout.
- Catalog/Experts/Preview parity gaps outside Publishing.
- Named-window `SteerView` behavior, OpenSpec artifacts, branches, worktrees, commits, PRs, Jira changes, or any Somnio state change.
