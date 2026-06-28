# Design — add-agent-prose-language

Source spec: `.vector/specs/add-agent-prose-language/spec.md` (20-section spec authored by
`/vector:raw`, validated PASS). This file captures the load-bearing decisions; the spec doc is
the full reference.

## Key decisions (LOCKED)

1. **Language lives in `config.json`, not a per-run flag** — it is a repo/team attribute. The
   only way to set it is `vector init --language` / `vector update --language`.
2. **`SchemaVersion` stays 1** — the field is additive and backward-compatible (`omitempty` +
   zero-value). No migration code; an older config without the field behaves exactly as today.
3. **The command obtains the language from `vector standup --json`**, as a new top-level
   `language` field resolved from config by the binary — NOT by the command reading
   `.vector/config.json` directly, and NOT via a new `vector config get` subcommand. Keeps
   CLI-owns-writes intact and the command thin.
4. **Free pass-through value** — any non-empty trimmed string (BCP-47 or plain name). No
   allow-list to maintain.
5. **Language reaches the agent as a prompt directive** (`Write the prose in: <language>`), not
   as structured data in the agent's input contract. Keeps the agent decoupled from config and
   the directive optional.
6. **Only standup is wired this phase**; the config field is generic for future reuse.
7. **Spec ids always verbatim**, untranslated, in any language.

## Architecture

Config-driven + prompt directive. Layer touch:

- `cli/internal/config` — new `Language string json:"language,omitempty"` on `Config`, plus
  `ResolvedLanguage()` (trim). Pattern twin: `ApplyMode` / `ResolvedApplyMode`.
- `cli/cmd/vector/main.go` — `--language` flag on `runInit` and `runUpdate` (set `cfg.Language`
  on persist; `init --force` preserves an existing language when the flag is absent). `usage()`
  updated. `runStandup` is **not** here — only the `case "standup"` dispatch (main.go:45).
- `cli/cmd/vector/standup.go` — in `runStandup` (line 20), after `enrichProjection` (line 45),
  load the config and assign `proj.Language = cfg.ResolvedLanguage()` before serializing
  `--json`. The assignment lives in `runStandup`, **not** inside `enrichProjection`, so the
  `standup` package never imports `config`. A `config.Load` error is **ignored** for the
  language (projection must not fail over a dispensable field → empty language → agent fallback).
- `cli/internal/standup/standup.go` — new `Language string json:"language,omitempty"` on
  `Projection`, populated by the caller, not the projection builder.
- `kit/commands/vector/standup.md` — step 2 reads `language` from the projection JSON and, when
  present, prepends `Write the prose in: <language>` to the agent prompt.
- `kit/agents/vector-standup-writer.md` (+ regenerated `cli/internal/scaffold/assets/agents/…`
  copy) — language hard rule switched to command-provided with conversation fallback.

## Flow

1. `vector init --language es` (or `vector update --language es`) → binary persists
   `"language": "es"`.
2. `/vector:standup` → `vector standup --json` → binary loads config, adds
   `"language": "es"` top-level to the projection.
3. Command reads `language`, prepends `Write the prose in: es` to the writer prompt.
4. Haiku agent emits the digest in Spanish; ids stay verbatim.
5. Command persists via `vector standup commit` (unchanged) and reports.

## Risks / edge cases

- `init --force` without `--language` must not silently wipe a configured language → preserve it.
- `flag.String` can't distinguish `--language ""` from absent, so there is no flag path to
  *clear* a language (edit config or re-init). Open question, non-blocking.
- Legacy config (no field) loads clean; verified `SchemaVersion` stays 1.

## Verification

`go -C cli generate ./...` (regenerate scaffold copy), `gofmt -l cli`, `go -C cli vet ./...`,
`go -C cli test ./...`, `go -C cli build ./...` — all green; digest stays valid JSON.
