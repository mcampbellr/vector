# Fix Assembly Station Publishing parity gaps (HUI-341)

## Why

Assembly Station Publishing still diverges from the verified HUI-341 behavior and Brian reference in the release board, review workflow, release safeguards, and backend contracts. The gaps can yield unclear or unsafe release decisions and stale post-release state.

## What Changes

- Restore the Publishing release board's expandable worksheet, review interaction, selection gates, SCORM/reach visibility, filters, hold/restore flow, safety copy, and all explicit loading/error/timeout states.
- Complete release-model behavior: post-publish/rollback invalidation, dependency closure and `include_closure`, recursive SCORM gating, and consistent refresh/recheck, review, confirmation, hold, publish, reach, history, rollback, and error contracts.
- Before code changes to each feasible parity surface, compare that exact surface/branch with Brian using the relevant comparison skill (`design-compare` for same-surface visual work), review its same-scroll screenshots and Markdown gap report first, and record linked evidence plus reconciliation; document an unavailable run and substitute evidence explicitly.
- Verify the full HUI-341 board, review, API, security, and regression acceptance matrix with unit, request-level, and UI coverage.

## Capabilities

### New Capabilities

- `assembly-publishing-parity`: Defined, testable Publishing behavior for the HUI-341 parity correction across the Assembly Station frontend and backend.

### Modified Capabilities

- None in this Vector repository; implementation happens in Somnio.

## Impact

Somnio's Assembly Station Publishing UI, release APIs, persistence, OpenAPI, and tests. No Vector runtime behavior changes beyond tracking this proposal.
