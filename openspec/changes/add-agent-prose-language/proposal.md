# Configurable prose language for Vector agent output

## Why

The standup digest is written by the `vector-standup-writer` Haiku agent, whose only language
rule is *"match the conversation language"*. But the command spawns the subagent with only the
projection JSON — the agent never receives the conversation language, so it defaults to English.
There is no way to pin the output language per project: a Spanish-speaking team running
`/vector:standup` from an English conversation gets an English digest, with no override.

## What changes

- **`language` field in `.vector/config.json`** — optional string (BCP-47 tag like `es`/`es-MX`,
  or a plain name like `Spanish`/`español`). Absent/empty = current behavior. Added with
  `omitempty`; **`SchemaVersion` stays 1** (additive, backward-compatible: an older config
  without the field deserializes to `Language == ""`, no migration). Mirrors the existing
  `applyMode` optional-field pattern.
- **`--language <lang>` on `vector init` and `vector update`** — `init` sets it at bootstrap;
  `update` is the non-destructive way to set/change it on an already-initialized repo. Value is
  free pass-through (trimmed, non-empty), no validation against a list.
- **Binary surfaces the language via the projection** — `vector standup --json` resolves
  `language` from config and exposes it as a new top-level field on `standup.Projection`. The
  command already consumes that JSON, so it never reads `.vector/config.json` itself and the
  binary stays the sole config reader/writer (CLI-owns-writes).
- **`/vector:standup` honors the language** — the command reads `language` from the projection
  JSON and, when present, passes the directive `Write the prose in: <language>` to the subagent;
  when absent, it adds nothing and the agent falls back to the conversation language.
- **Agent rule updated** — `vector-standup-writer` (and its embedded scaffold copy) switches to:
  *"Write the prose in the language provided by the command; if none is provided, match the
  conversation language. Keep spec ids verbatim."*

The design is generic prose-language config (not standup-specific), so other prose-generating
agents (e.g. the raw spec author) can reuse it later. **Only standup is wired in this change.**

## Capabilities

### New Capabilities
- `agent-prose-language`: a repo declares the language Vector agents write prose in
  (`config.language`); the binary surfaces it through the standup projection and the
  `/vector:standup` command passes it to the writer agent, with a conversation-language fallback.

### Modified Capabilities
- `standup-digest`: the digest pipeline now honors the configured prose language instead of
  always defaulting to English.

## Out of scope

- Translating spec ids, titles, or any persisted state (ids stay verbatim).
- UI/board localization and CLI help i18n.
- Wiring agents other than `vector-standup-writer`.
- A per-run `--language` flag on `vector standup` / `/vector:standup` (language is a repo
  attribute, not a session one).
- Validating the language value against an allow-list.
