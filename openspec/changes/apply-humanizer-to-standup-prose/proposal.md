# Humanize the standup digest prose

## Why

The standup digest is written by the `vector-standup-writer` Haiku agent. Its prose is correct
but carries the usual AI tells: inflated significance, superficial `-ing` tails, copula avoidance
(`serves as`/`stands as`), AI vocabulary (`crucial`, `pivotal`, `leverage`), rule-of-three
padding, and negative parallelisms. A dev who wants it to read naturally has to hand the digest
to a separate `/humanizer` skill afterwards.

The user's `/humanizer` skill already catalogs 29 anti-AI-writing patterns, but it lives in their
personal dotfiles (`~/.dotfiles/claude/.claude/skills/humanizer/SKILL.md`) and is **not**
distributed with Vector — an end-user repo never has it. The fix is to vendor a distilled,
standup-relevant subset of those patterns straight into the agent, so the digest comes out
human-sounding on the first generation, in whatever language the repo declares.

## What changes

- **New "Prose quality" section inside `kit/agents/vector-standup-writer.md`** — a compact
  (~20–30 line) distillation of the `/humanizer` patterns most relevant to short standup prose,
  written in English as actionable directives (what to avoid / what to prefer), not the full
  29-section catalog. It covers: no significance inflation, no superficial `-ing` tails, plain
  vocabulary, direct copula (`is`/`are`/`has`), no forced rule-of-three or synonym cycling, no
  negative parallelisms or tailing negations, plain style (no em-dash punch, emojis, boldface,
  curly quotes), no filler/hedging, no generic upbeat closers, and varied sentence rhythm.
- **Always on** — the guidance is part of the agent's system prompt. No per-run flag, no config
  field. The digest is humanized by default.
- **Subtractive, not additive** — the change takes the *removal* patterns and rhythm variation
  from `/humanizer` but **excludes** its "PERSONALITY AND SOUL" (inject opinions/feelings)
  section, because that would conflict with the agent's existing `Never invent work` hard rule.
  The prose gets more human by removing AI noise, not by adding judgments the events don't support.
- **Language-agnostic** — the guidance is written in English (pattern names like *rule of three*,
  *em-dash overuse*) but the patterns are universal and apply to the prose in whatever language
  the agent produces. It interoperates with the `language` field from `add-agent-prose-language`:
  the digest comes out in the declared language **and** humanized in that language; with no
  declared language, in the conversation language, humanized all the same.
- **Embedded scaffold copy regenerated** — `cli/internal/scaffold/assets/agents/vector-standup-writer.md`
  is refreshed via `go generate` (never hand-edited).

`/humanizer` is used as a **reference** for the patterns, not a runtime dependency: the guidance
is vendored inside the agent so the distributed binary is self-contained.

## Capabilities

### Modified Capabilities
- `standup-digest`: the digest pipeline now produces humanized prose (free of common AI tells) by
  default, in the configured prose language, without a second tool pass.

## Out of scope

- Modifying the user's `/humanizer` skill (it lives outside Vector, in personal dotfiles).
- Wiring humanization into agents other than `vector-standup-writer` (raw spec author, validator).
- A per-run flag (`vector standup --humanize`) or config field (`humanizeStandup`) — always on.
- A post-generation pass agent that rewrites the JSON; all humanization is prompt-driven.
- Changing the agent's output shape (`{global, perSpec[]}`) or the standup projection contract.
- Extracting the guidance into a shared `kit/rules/prose/…` file for reuse (possible future
  extension; this change keeps it embedded in the standup-writer).
