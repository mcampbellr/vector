# Add offline subagent evaluation harness with CI regression gate

## Why

There is no mechanism today to detect that a prompt change in one of Vector's three **judge**
subagents (`vector-spec-validator`, `vector-feasibility-reviewer`, `vector-comment-evaluator`) —
or in `citation-discipline.md` — degrades their behaviour, other than ad-hoc manual review. Vector
consumes no LLM of its own (it orchestrates the *user's* Claude Code), so the harness must be a
purely **offline** measurement tool over recorded transcripts: deterministic, zero API cost, and
able to run inside the existing `go -C cli test ./...` CI step without ever invoking a real agent.

## What changes

- New `cli/internal/eval/` package (`evaluator.go`, `citations.go`, `rubrics.go`) holding all
  scoring logic, shared verbatim between the human subcommand and the CI gate test (single
  `eval.Run`, zero drift).
- New `vector eval` subcommand (`cli/cmd/vector/eval.go`), registered in `newRootCmd`: ASCII table
  by default, `--json` for the machine-readable report, `--role` (repeatable, default all three),
  `--update-baselines` to refresh baselines after human review.
- CI gate via `go test` (`cli/cmd/vector/eval_test.go`) — picked up automatically by the existing
  `go -C cli test ./...` step; **no change to `ci.yml`**.
- Gate logic = **both** criteria with warm-up: fail if, for any role, the current score regresses
  vs a committed baseline (`score < baseline.score`) **or** falls below an absolute per-role
  threshold (`score < threshold`, inclusive). A committed `gate-mode.json` toggle (env override
  `VECTOR_EVAL_MODE`) keeps the gate non-blocking during warm-up.
- Captured-and-curated fixtures under `cli/cmd/vector/testdata/eval/<role>/<scenario>.json`, with
  hand-labelled `groundTruth.expectedVerdict`; curation process documented in
  `cli/internal/eval/README.md`.
- Citation-fidelity check: extracts `path:line` claims from each recorded output and verifies
  existence + line-in-range against the **current** repo tree, operationalising
  `citation-discipline.md`. Per-role×model win-rate is tabulated for model-routing evidence.

## Scope

- **In**: the `internal/eval` package, the `vector eval` subcommand, the `eval_test.go` CI gate,
  captured-and-curated fixtures + baselines + thresholds + gate-mode toggle for the three judge
  roles, the citation-fidelity check, per-role×model win-rate, and the curation README.
- **Out**: invoking real Claude Code / any paid API anywhere in the harness; the other 10 kit
  subagents (refiners, composer, implementers, writers, ui-ux-designer); semantic prose comparison;
  cross-run win-rate history persistence; modifying `ci.yml`; changing any existing command's
  `--json` (`TestJSONGoldenUnchanged` stays inviolate); reading/writing the user project's
  `.vector/` state; new external dependencies; calibrating the exact threshold numbers (placeholders
  only, recalibrated after warm-up).

Authored spec: `.vector/specs/add-subagent-evaluation-harness/spec.md`.
