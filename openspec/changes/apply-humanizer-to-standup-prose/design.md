# Design — apply-humanizer-to-standup-prose

Source spec: `.vector/specs/apply-humanizer-to-standup-prose/spec.md` (20-section spec authored by
`/vector:raw`, validated PASS). This file captures the load-bearing decisions; the spec doc is
the full reference.

## Key decisions (LOCKED)

1. **Integration = condense into the agent's system prompt** — not a shared rule file, not a
   post-pass agent. The distilled guidance is a static section of `vector-standup-writer.md`.
   Rationale: simplicity and token routing — the agent runs on Haiku, so a single call with a
   bounded prompt increment beats a second call (post-pass) or per-run injection from the command.
2. **Always on** — no per-run flag, no config field. Humanization is the agent's default behavior.
3. **Language-agnostic** — guidance in English, universal patterns; applied to the prose in
   whatever language the agent produces. Interoperates with `language` from
   `add-agent-prose-language` (humanizes in that language).
4. **Subtractive, not additive** — take the removal patterns and rhythm variation from
   `/humanizer`; **exclude** "PERSONALITY AND SOUL" (inject opinions/feelings) because it would
   conflict with the agent's `Never invent work` hard rule.
5. **`/humanizer` is reference, not a dependency** — the guidance is vendored inside the agent;
   Vector never reads the personal skill at runtime.
6. **Only `vector-standup-writer` this phase** — reuse by other prose agents is a later extension.
7. **Coexistence with `add-agent-prose-language`** — both specs edit the same agent file. This
   change **adds** the Prose-quality section and **respects** the language rule that the sibling
   introduces; it does not duplicate or rewrite that rule. If the sibling has not been applied
   yet, this change adds only the Prose-quality section (the language rule is the sibling's job).

## Architecture

Prompt-side guidance vendored into the agent. Layer touch:

- `kit/agents/vector-standup-writer.md` — new `## Prose quality — write like a human` section
  after the "Hard rules" block and before "Output — exact shape". Distilled directives + an
  explicit note that the guidance is subtractive (no injected opinions) and language-agnostic.
  `model: haiku`, `tools: Read`, the Input block, the other Hard rules, the Output shape, and the
  empty-period literal (`no activity since last standup`) all stay intact.
- `cli/internal/scaffold/assets/agents/vector-standup-writer.md` — **regenerated** copy via the
  `//go:generate` directive in `cli/internal/scaffold/scaffold.go:13`
  (`cp -R ../../../kit/{commands,agents,vector} assets/`). Never hand-edited; must end up
  byte-identical to the source.
- `README.md` — optional one-line note if it already documents `/vector:standup`; omit otherwise
  (no new flags/config to document).

No Go logic, no config, no projection, no API changes. The standup projection/commit pipeline and
the binary are untouched; the binary never calls an LLM.

## Token / cost

The only cost is a bounded, one-time increase in the agent's system prompt (~20–30 lines). The
agent stays on Haiku — no tier change, no second call, no extra I/O. A post-pass agent was
rejected for the ~2x token/latency cost (`product/token-routing.md`).

## Verification

Qualitative + parity:
- Source↔embed parity: `diff kit/agents/vector-standup-writer.md cli/internal/scaffold/assets/agents/vector-standup-writer.md`
  must be empty after `go generate`.
- Manual ES/EN: run `/vector:standup` with `language: "es"` and without; confirm the prose is free
  of the §6 tells.
- Edge cases: empty period returns the exact `no activity since last standup` literal (not
  "humanized" into something else); minimal activity stays tight with no invented sentiment.
- Output stays valid JSON with shape `{global, perSpec[]}` (enforced by `vector standup commit`).

## Risks

- **Drift** between source and embedded copy if `go generate` is skipped — mitigated by the parity
  check in the gate.
- **Collision** with `add-agent-prose-language` edits to the same file — mitigated by decision 7
  (additive, respect the language rule).
- **Residual tells** — graceful degradation: the digest stays valid and useful; quality is a goal,
  not a commit-blocking gate.
