# Tasks — apply-humanizer-to-standup-prose

## 1. Agent — Prose quality section (`kit/agents/vector-standup-writer.md`)

- [x] 1.1 Add a new `## Prose quality — write like a human` section after the "Hard rules" block
      and before "Output — exact shape", with a compact (~20–30 line) distillation of the
      `/humanizer` patterns most relevant to standup prose:
      - No significance/legacy inflation (`marks a pivotal moment`, `key milestone`,
        `represents a shift`, `sets the stage for`).
      - No superficial `-ing` tails (`…, reflecting steady progress`, `…, showcasing the work`).
      - Plain vocabulary — avoid `crucial, pivotal, leverage, robust, seamless, delve,
        underscore, showcase, vibrant, foster, intricate, testament`.
      - Direct copula: `is`/`are`/`has`, not `serves as`/`stands as`/`boasts`/`features`.
      - No forced rule-of-three; no synonym cycling (elegant variation) of the same spec.
      - No negative parallelisms (`not just X, it's Y`) or tailing negations (`no blockers`
        tacked on — write it as a real clause, e.g. `nothing is blocked`).
      - Plain style: no em-dashes for punch (commas/periods), no emojis, no boldface, no curly
        quotes; present tense; lead with the outcome, then the substance.
      - No filler/hedging (`at this point in time`, `it's worth noting that`) and no generic
        upbeat closers (`good momentum`, `on track for great things`).
      - Vary sentence rhythm (mix short and longer sentences).
- [x] 1.2 Add an explicit note that the guidance is **subtractive**: it humanizes by removing AI
      tells and varying rhythm, **not** by adding opinions, feelings, or judgments (that would
      violate `Never invent work`). Do not use `/humanizer`'s "add soul / have opinions" advice.
- [x] 1.3 Add a note that the guidance is **language-agnostic**: the patterns apply to the prose
      in whatever language the agent produces (es/en/other), not only English.
- [x] 1.4 Keep intact: front-matter (`model: haiku`, `tools: Read`), the Input block, the other
      Hard rules, the Output shape (`{global, perSpec[]}`), and the `no activity since last
      standup` literal. Do not duplicate or rewrite the language rule from
      `add-agent-prose-language` if it is already present — complement it.

## 2. Scaffold — regenerate embedded copy

- [x] 2.1 Run `go -C cli generate ./internal/scaffold` (or `go -C cli generate ./...`) to refresh
      `cli/internal/scaffold/assets/agents/vector-standup-writer.md`. Do not hand-edit the copy.
- [x] 2.2 Verify the embedded copy is byte-identical to the source:
      `diff kit/agents/vector-standup-writer.md cli/internal/scaffold/assets/agents/vector-standup-writer.md`
      must produce no output.

## 3. Docs (optional)

- [~] 3.1 (skipped — README does not document /vector:standup) If `README.md` already documents `/vector:standup`, add one line: the digest is
      humanized automatically (always on, no config). If the README is vision-stage and does not
      document the standup, skip this — do not create a section just for it.

## 4. Gate

- [x] 4.1 `go -C cli generate ./...`, `diff` source↔embed empty, `gofmt -l cli` (empty),
      `go -C cli vet ./...`, `go -C cli test ./...`, `go -C cli build ./...` — all green.
- [ ] 4.2 Functional check: `/vector:standup` with `language: "es"` → Spanish, humanized digest
      (no §1.1 tells); without a declared language → conversation language, humanized. Empty
      period → exact `no activity since last standup` literal; minimal activity → tight summary
      with no invented sentiment. Output stays valid JSON `{global, perSpec[]}`.
