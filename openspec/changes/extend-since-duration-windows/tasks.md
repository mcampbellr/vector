# Tasks — extend-since-duration-windows

## 1. Parser (`cli/internal/standup/standup.go`)

- [ ] 1.1 Replace the 3-branch switch in `ParseSince` (lines 27-39) with: `today` branch
      (unchanged UTC midnight) + generic `<N><unit>` branch — last byte as unit (`h` →
      `time.Hour`, `d` → `24*time.Hour`; else invalid), `strconv.Atoi` on the prefix, require
      `N >= 1`, return `now.Add(-time.Duration(N) * unitDur)`.
- [ ] 1.2 Any other case (unknown unit, `Atoi` fails, `N < 1`, empty string) →
      `(time.Time{}, ErrInvalidSince)`.
- [ ] 1.3 Update the `ErrInvalidSince` message (line 20) to document the new syntax, e.g.
      `"invalid since: use today or <N>h / <N>d (e.g. 12h, 5d)"`.
- [ ] 1.4 Update the function comment (lines 22-26) to describe the generic format (stop
      enumerating `24h`/`7d` as the only values). Keep the package purity/determinism contract.
- [ ] 1.5 No new dependency (no regex, no duration-parsing lib) — only `strconv`/`errors`/`time`.
      No `Nw`/`Nm`/`Ns`, no upper bound on N.

## 2. Parser tests (`cli/internal/standup/standup_test.go`)

- [ ] 2.1 Flip the pre-existing `{"36h", time.Time{}, true}` row (line 108) to a valid row
      (`{"36h", now.Add(-36 * time.Hour), false}`) or fold it into the valid block. Keep the
      other existing invalid rows (`""`, `"yesterday"`).
- [ ] 2.2 Extend `TestParseSince` (line 97) with valid cases: `1d`, `5d`, `7d`, `24h`, `36h`,
      `today`.
- [ ] 2.3 Add invalid cases: `0d`, `0h`, `-5d`, `1.5d`, `d`, `h`, `5w`, `5m`, `5D`, `" 5d "`,
      `"5 d"`, `""`.
- [ ] 2.4 Explicit regression: `ParseSince("24h", now) == now.Add(-24*time.Hour)` and
      `ParseSince("7d", now) == now.Add(-7*24*time.Hour)`. Assert with `errors.Is` only — no
      error-message text assertions.

## 3. CLI messages (`cli/cmd/vector/standup.go`)

- [ ] 3.1 `resolveSince` (line 98): update the error string to e.g.
      `"invalid --since: use today, <N>h or <N>d (e.g. 12h, 5d)"`. Do not change its logic — it
      keeps delegating to `standup.ParseSince` and keeps falling back to the persisted marker
      when `window == ""`.
- [ ] 3.2 `newStandupCmd` (line 78): update the `--since` help text to the new syntax
      (`today, <N>h or <N>d, e.g. 12h, 5d (default: since the last standup marker)`).
- [ ] 3.3 `newStandupCommitCmd` (line ~164): same treatment on its `--since` help text.
- [ ] 3.4 Do not rename `--since` or change its type; keep `runStandup` /
      `runStandupCommitBody` flow intact.

## 4. Board test (`cli/internal/board/server_test.go`)

- [ ] 4.1 `TestHandleActivityInvalidSince` (line 122): swap the invalid value from `36h` to a
      genuinely invalid one (`36x` / `abc` / `5w`) for the 400 case.
- [ ] 4.2 Add a valid-case assertion (`5d` and/or `36h` → 200).
- [ ] 4.3 Do **not** modify `cli/internal/board/server.go` — `handleActivity` inherits the new
      syntax via delegation.

## 5. Verification gate

- [ ] 5.1 `gofmt -l cli` (no output).
- [ ] 5.2 `go -C cli vet ./...`.
- [ ] 5.3 `go -C cli test ./...`.
- [ ] 5.4 `go -C cli build ./...`.
- [ ] 5.5 Confirm no `web/` file and no `server.go` were touched; no temp logs / unjustified
      TODOs left behind.
