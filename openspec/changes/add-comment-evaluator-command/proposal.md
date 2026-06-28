# Add /vector:comment command to evaluate PR comments

## Why

The user relies on the global `/pr-comment` skill to critically assess a review/ticket comment
against the real PR diff and implement it only when it earns it. That skill is somnio-specific
(pnpm, `graphify-out/`, fixed worktree layout, max-5-files, GitHub-only via `gh`) and lives
outside Vector. Vector needs the same capability, but **agnostic to the user's code** and
**distributable** inside the binary, so any Vector-managed repo gets it via `vector init`. Being
inside Vector, it can also tie the resulting work to a spec card so it shows in standup/timeline.

## What changes

- **Project command `/vector:comment`** (`kit/commands/vector/comment.md`): parses the comment,
  resolves the branch, gets the diff, resolves the associated spec card, delegates the critical
  evaluation, reports the verdict in the project's configured language (`config.language`, with conversation-language fallback), and implements conditionally — mirroring the
  `/pr-comment` flow, with the somnio-specific parts generalized.
- **Evaluator agent** (`kit/agents/vector-comment-evaluator.md`), tier **Sonnet**, read-only:
  gathers its own evidence and returns a structured verdict (`VÁLIDO Y VALIOSO` / `VÁLIDO PERO
  MARGINAL` / `INVÁLIDO O SIN VALOR` + `N/10` + `file:line` evidence + AI-red-flags + remediation).
- **Agnostic diff resolution**: the command **asks the user** whether to use `gh` (when present
  and a PR exists) or local `git diff <base>..HEAD`. No GitHub assumption.
- **Verification detection**: discover `build`/`lint`/`test` from `.vector/config.json` + repo
  manifests; **ask the user** if not detected. No hardcoded `pnpm`/`npm`.
- **Spec-aware integration**: on implement, resolve the spec card and append a `work.logged`
  event via the existing `vector spec worklog`; offer `review → in-progress` (via existing
  `vector spec status`) only when the card is in `review`. **No new Go code, events, or endpoints.**
- **Humanized reply** (English by default) when not implementing; copied to clipboard, never posted.
- **Vendoring**: the command and agent are embedded into the binary via `go generate`
  (`cli/internal/scaffold`), so `vector init` seeds them.

## Capabilities

### New Capabilities

- `comment-evaluation`: critically evaluate a PR/ticket comment against the real diff with a
  skeptical Sonnet agent, report a structured verdict, and implement only when valid and low-risk.
- `comment-spec-linking`: when implementing a comment-driven change, link the work to its spec
  card by appending `work.logged` (and optionally stepping `review → in-progress`) via the binary.

### Modified Capabilities

<!-- None: the integration reuses existing `vector spec worklog`/`status`/`list`; no Go code,
events, endpoints, or UI change. -->

## Impact

- `kit/`: new `commands/vector/comment.md` and `agents/vector-comment-evaluator.md`.
- `cli/internal/scaffold/assets/`: embedded copies regenerated via `go generate` (no manual edit);
  `scaffold_test.go` only if it enumerates the command set.
- **No** changes to `cli/` Go code, `cli/internal/state`, `cli/internal/board`, or `web/`.
- No new dependencies. Reuses `vector spec worklog` (`cli/cmd/vector/standup.go:199`),
  `vector spec status` (`cli/cmd/vector/spec_transitions.go:143`), and `vector spec list`.

Authored spec: `.vector/specs/add-comment-evaluator-command/spec.md`.
