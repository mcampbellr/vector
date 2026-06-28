# add-comment-evaluator-command

Add `/vector:comment`: an agnosticized, distributable port of the `/pr-comment` skill that critically evaluates a PR/ticket comment against the real diff (Sonnet evaluator) and implements it only when valid and low-risk, logging `work.logged` against the associated spec card.
