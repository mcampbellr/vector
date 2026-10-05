# Design — extend-since-duration-windows

## Pattern: shared parser, one generalization point

`ParseSince` stays the single source of truth for the `--since` / `since` syntax. Its body goes
from a 3-literal switch to a one-literal branch (`today`) plus a generic `<N><unit>` branch. No
consumer (CLI, board) needs to know the format changed — both already call the function with the
raw user string and receive `(time.Time, error)`.

Signature is **unchanged**:

```go
func ParseSince(window string, now time.Time) (time.Time, error)
```

Pure and deterministic: `now` is the only time input; the function never reads the system clock.

## Parsing algorithm

1. Normalize `now` to UTC.
2. If `window == "today"` → return UTC midnight of `now` (unchanged from today).
3. Otherwise, treat the **last byte** as the unit candidate:
   - `h` → `unitDur = time.Hour`
   - `d` → `unitDur = 24 * time.Hour`
   - any other byte → `(time.Time{}, ErrInvalidSince)`
4. Apply `strconv.Atoi` to the remaining prefix to get `N`.
   - `Atoi` failure (non-numeric, empty, whitespace, decimal, sign) → `ErrInvalidSince`.
   - `N < 1` → `ErrInvalidSince` (rejects `0d`/`0h` and, via the `≥ 1` rule, negatives).
5. Return `now.Add(-time.Duration(N) * unitDur)`.

`strconv`/`errors`/`time` are already imported — **no new dependency, no regex**.

## Key decisions (from spec §10 — fixed, not to be revisited)

- **`Nd` + `Nh`, not just `Nd`** — same implementation cost, covers more real use cases.
- **No `Nw`/`Nm`/`Ns`** — keeps the parser simple; weeks/months are not fixed durations.
- **`today` stays the only keyword** — calendar-relative semantics, not a fixed subtraction; not
  forced into the `<N><unit>` shape.
- **Generalize the shared parser, don't duplicate per consumer** — avoids CLI/board drift.
- **`36h` becomes valid in `/api/activity`** — direct consequence of the shared parser; the test
  presupposing the opposite is updated in the same change.
- **No upper bound on N** — YAGNI; a cap can be added later if abuse appears.
- **Strict format (lowercase, no spaces)** — consistent with the existing case-sensitive
  literals.
- **`strconv.Atoi` only, stdlib** — `<digits><letter>` resolved via last-byte slicing + `Atoi`.
- **Signature unchanged** — do not break the two existing consumers or the documented purity
  contract.

## Layers affected

- domain/standup (`standup.go`): **yes** — `ParseSince` body + `ErrInvalidSince` message.
- application/CLI (`standup.go` cmd): **yes** — `resolveSince` error string + `--since` help
  text in two commands. `resolveSince` logic is otherwise unchanged.
- domain/board (`server.go`): **no code change** — `handleActivity` already delegates; only its
  test updates.
- web/board UI: **no** — no component change; `useSpecActivity` default `since='7d'` survives.
- data/state: **no** — pure input parsing; nothing persisted is touched.

## Risks

- **Behavior change on `/api/activity?since=36h`** (400 → 200): intentional and documented; the
  test `TestHandleActivityInvalidSince` is updated in the same change so the suite stays green.
- **Pre-existing test row `{"36h", time.Time{}, true}`** in `standup_test.go:108` would fail
  after generalization — it must be flipped to a valid row (or moved into the valid block) as
  part of this change.
- **Integer overflow on absurd N**: not a real risk within any realistic range (`time.Duration`
  is int64 ns); no explicit guard added, per spec §17.
