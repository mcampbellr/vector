# Accept generic duration windows (Nd/Nh) in standup `--since`

## Why

`standup.ParseSince` (`cli/internal/standup/standup.go:27-39`) is a 3-branch switch that only
accepts the fixed literals `24h`, `7d`, and `today`, and rejects everything else with
`ErrInvalidSince`. A dev running `vector standup`, `vector standup commit`, or querying the
board's `GET /api/activity` cannot pick an arbitrary window (e.g. "last 3 days", "last 36
hours") — they are stuck with the two hardcoded shortcuts.

The parser is **shared** by both consumers — the `--since` flag of `vector standup` /
`vector standup commit` (`cli/cmd/vector/standup.go`) and the `since` query param of
`GET /api/activity` (`cli/internal/board/server.go:93`). Generalizing it in one place benefits
both automatically, with no per-consumer duplication.

This is a planned extension of the accepted-values surface, not a bug fix: the original switch
was fixed-literal because `24h`/`7d`/`today` were the only windows the two consumers needed at
the time.

## What changes

- **Generalize `standup.ParseSince`** to accept:
  - `today` — unchanged (UTC midnight of the current day).
  - `<N>h` — N hours ago, N integer ≥ 1 (e.g. `12h`, `24h`, `36h`).
  - `<N>d` — N days ago, N integer ≥ 1 (e.g. `1d`, `3d`, `5d`, `7d`).
  - The old fixed literals `24h`/`7d` **stop being special-cased**: they fall naturally out of
    the generic parser (`24h` → `now - 24h`, `7d` → `now - 7*24h`), fully backward-compatible.
- **Strict format validation**: N must be a positive integer (≥ 1). Reject `0d`/`0h`, negatives
  (`-5d`), non-integers (`1.5d`), missing number (`d`), unknown units (`5w`/`5m`/`5s`),
  uppercase unit (`5D`), and any surrounding/internal whitespace (`" 5d "`, `"5 d"`).
- **No upper bound**: `999d`, `365d` are valid.
- **No new units**: `Nw`/`Nm`/`Ns` are not supported.
- **`ErrInvalidSince`** message updated to document the new syntax.
- **`resolveSince`** (CLI) user-facing error message updated.
- **`--since` help text** updated in `newStandupCmd` and `newStandupCommitCmd`.
- **`GET /api/activity`** inherits the new syntax with **no code change** — it already delegates
  to `standup.ParseSince`. This makes `since=36h` valid (today it is a 400).
- **`TestHandleActivityInvalidSince`** updated: swap the now-valid `36h` invalid-case for a
  genuinely invalid value (`36x`/`abc`/`5w`) and add a valid-case assertion (`5d`/`36h` → 200).
- **Table-driven unit tests for `ParseSince`**: valid (`1d`, `5d`, `7d`, `24h`, `36h`, `today`),
  invalid (`0d`, `-5d`, `1.5d`, `d`, `5w`, `5D`, `" 5d "`, `"5 d"`), and explicit regression
  (`24h` == `now - 24h`, `7d` == `now - 7*24h`).

## Capabilities

### Modified Capabilities
- `standup-digest`: the `--since` / `since` window syntax accepted by `standup.ParseSince`
  widens from three fixed literals to `today` / `<N>h` / `<N>d`. Existing values keep their
  exact behavior; `36h` (and any integer window) become valid across both the CLI and
  `GET /api/activity`.

## Impact

- `cli/internal/standup/standup.go` — `ParseSince` body + `ErrInvalidSince` message.
- `cli/internal/standup/standup_test.go` — extend `TestParseSince`; flip the pre-existing
  `{"36h", ..., true}` invalid row to valid.
- `cli/cmd/vector/standup.go` — `resolveSince` error string + `--since` help in two commands.
- `cli/internal/board/server_test.go` — update `TestHandleActivityInvalidSince`.
- **No change** to `cli/internal/board/server.go` (inherits via delegation) or to any `web/`
  file (`useSpecActivity` default `since='7d'` survives unchanged).
- No new dependencies — Go stdlib only (`strconv.Atoi`, no regex).

Authored spec: `.vector/specs/extend-since-duration-windows/spec.md`.
