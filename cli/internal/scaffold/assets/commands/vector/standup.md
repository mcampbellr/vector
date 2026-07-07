---
name: "Vector: Standup"
description: Project the activity since your last standup, generate a natural-language scrum digest (global + per-spec) via a cheap Haiku agent, persist it, and surface it on the board. The binary owns every write; the prose comes from the agent.
category: Workflow
tags: [vector, standup, digest, activity, ceremony]
---

Produce the **scrum-standup digest** for the period since your last standup: what advanced,
what reached review, what is blocked. The binary projects and persists; the prose is generated
by a cheap agent. **You never write Vector's state yourself** — you pipe the digest through
`vector standup commit`, which advances the marker (CLI-owns-writes).

**Input**: `$ARGUMENTS` (optional window: `24h` | `today` | `7d`). Empty → since the last
standup marker.

> Token routing: the projection is free (binary), and the prose is cheap bounded work
> (structured JSON → a few short paragraphs) → the **Haiku** `vector-standup-writer` agent
> (`product/token-routing.md`). The binary never calls an LLM.

## 1. Project the period

Run `vector standup --json` (append `--since <window>` if the user passed `24h`/`today`/`7d`).
It returns `{since, perSpec[], totals}` projected from the activity log since the marker (or
the window), plus an optional top-level `language` field (the repo's configured prose language;
absent when none is set). An invalid window errors with `invalid --since: use 24h, today or 7d`
— surface it and stop.

If `perSpec` is empty, tell the user "no activity since last standup" and stop — there is
nothing to digest (the marker only advances on a real commit in step 3, so nothing is lost).

## 2. Generate the digest (Haiku)

Pass the **exact JSON** from step 1 to the `vector-standup-writer` subagent (model: Haiku). Do
not re-read the activity log yourself or summarize before handing it off — the agent's whole job
is the prose. If step 1's JSON carried a non-empty `language`, prepend the directive
`Write the prose in: <language>` to the agent prompt (above the pasted JSON); if `language` is
absent or empty, add no directive and the agent falls back to the conversation language. It returns:

```json
{ "global": "<1–3 paragraphs>", "perSpec": [ { "id": "...", "summary": "<1–2 sentences>" } ] }
```

## 2a. Validate the digest (shape-gate)

Before piping the digest to the binary, validate that the agent output is well-formed **and**
follows the deterministic per-spec format. A valid response meets **all** of:

- Parseable as JSON.
- `global` is a non-empty string.
- `perSpec` is an array (may be `[]` when the projection's `perSpec` was empty).
- Every `perSpec[]` entry has an `id` that matches an `id` from step 1's projection (same set,
  no invented specs), and a non-empty `summary`.
- Every `summary` is a **single plain paragraph in the template shape**: it contains no markdown
  bullets or list markers (`- `, `* `, `•`, `1.`), no emojis, and **begins with that spec's
  identifier** — the projection's `ticket.key` when the spec has one, otherwise its slug `id`.

**If valid on attempt 1:** proceed to §3.

**If invalid on attempt 1:** notify the user on stdout:

```
subagent returned invalid JSON — retrying (attempt 2/2)…
```

Then re-spawn the `vector-standup-writer` subagent (same Haiku tier) with the same projection
JSON **plus** a correction directive prepended to the prompt (above the JSON):

```
The previous attempt returned malformed JSON or a summary that broke the format.
Return ONLY a valid JSON object matching exactly:
{"global": "<string>", "perSpec": [{"id": "<string>", "summary": "<string>"}]}
Each summary must be one plain paragraph that starts with the spec's identifier
(ticket key if present, else the slug), with no bullets, markdown or emojis.
No preface, no code fences, no trailing text.
```

**If valid on attempt 2:** proceed to §3.

**If invalid on attempt 2:** report to the user and abort. Do **not** pipe anything to the binary;
the marker does not advance:

```
standup digest failed: the subagent returned invalid JSON twice; nothing was written and the marker was not advanced. Re-run /vector:standup to retry.
```

## 3. Persist via the binary

Pipe the agent's JSON to `vector standup commit --digest-file -` (pass the same `--since` you
used in step 1 so the persisted projection matches the window). The binary validates the JSON,
rebuilds the structural fields from a fresh projection, writes
`.vector/local/standup.json`, and **advances the marker** to now. Invalid JSON →
`invalid digest json`: nothing is written and the marker does not move (re-run the agent).

## 4. Report

Print the **global digest** and the counts (e.g. `5 specs, 12 changes since the last standup`),
then point the user at the board: "open the Standup view to see the per-spec breakdown and each
card's activity timeline." On macOS you can offer `| pbcopy` to copy the digest.

## Notes

- The window keywords are `24h`, `today`, `7d`; anything else is rejected with an actionable
  message. Absolute timestamps are out of scope for V1.
- `.vector/local/standup.json` and `activity.jsonl` are **personal and gitignored** — the digest
  is per-dev, not committed or shared.
- The marker advances even on a period with no new activity once committed; the full history is
  retained in `activity.jsonl`, so advancing never destroys anything.
- If `vector` is not found, it isn't installed — tell the user; never edit `.vector/` by hand.
