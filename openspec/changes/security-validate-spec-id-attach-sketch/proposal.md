# Slug-validate the spec id at the AttachSketch store boundary

## Why

The `/security-audit` run on 2026-07-01 flagged a LOW-severity, defense-in-depth finding in
`cli/internal/state/store.go`. `AttachSketch` sanitizes the sketch **file name**
(`sanitizeSketchName`) but joins the caller-supplied `id` into a filesystem path via
`sketchesDir(id)` **without validating** it against the canonical spec-slug pattern. If a future
caller reaches `AttachSketch` (or a sibling path-building writer) without a preceding `ReadSpec(id)`,
an `id` containing path separators or `..` could write outside the intended `.vector/specs/<id>/`
directory.

Traversal is blocked in practice today: the prior `ReadSpec(id)` fails on a non-existent/odd id, and
this is a local-only CLI surface — hence LOW. This is a missing boundary check, not a live exploit,
but the writer must not depend on an upstream read for its own path safety.

## What changes

- Validate `id` against the canonical spec-slug pattern (reuse `state.Slug` / the existing slug
  regex) at the store boundary, returning a clear error on mismatch — mirroring the existing sketch
  **name** check.
- Centralize the check into one `validateSpecID` guard so every path-building store method (not just
  `AttachSketch`) can share it, rather than a one-off inline check.
- Add unit tests: an `id` with `/`, with `..`, and an empty `id` are all rejected **before** any
  filesystem write; existing valid-id behavior is unchanged.

## Scope

- **In**: the `id`-boundary validation at `AttachSketch` (and co-located path-building writers) plus
  the rejection tests. No behavior change for valid ids.
- **Out**: reworking the sketch storage layout or the sketch **name** sanitizer (already correct).
  Just add the id-boundary validation + test.
