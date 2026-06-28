# add-vector-bug-command

Add `/vector:bug`: the bug-framed counterpart of `/vector:raw`. It refines a raw bug report (Haiku), deduces the root cause via `git blame`/`git log` (mapping suspect commits to a Vector spec or external ticket), and registers the bug as a `draft` card with a new persisted `relatedTo[]` state field (`{kind: spec|ticket, ref, source: blame|manual}`) — surfaced read-only on the board, API, and standup. Authoring only; ends in `draft` (the `fix-…` OpenSpec change is `/vector:propose`, the fix is `/vector:apply`).
