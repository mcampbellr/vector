# Spec: Post-action spec summaries and details drawer

## 1. Goal

Build two coupled deliverables behind one card:

1. **Post-action summary pipeline (cli + kit)** — after every domain action a spec transitions
   through (`/vector:propose`, `/vector:apply`, `/vector:status`, `/vector:close`,
   `/vector:archive`), Vector spawns a cheap **Haiku** agent (`vector-summary-writer`) that writes
   a short "what was done" prose summary for that spec. The summary is **persisted locally**
   (`.vector/local/summaries.json`, gitignored) and **reused by `/vector:standup`** to make the
   digest more meaningful.
2. **Spec details drawer (web)** — board cards become **clickable**. The inline "Next command" and
   "Activity" affordances move off the card face into a right-side **drawer** that surfaces: the
   spec's AI summary, its activity timeline, and a set of **copyable** useful commands (e.g. a
   `/vector:link` command when the spec has no ticket).

This feature lets a developer (a) read an AI summary of what's been done on a spec without running
a standup, and (b) keep the board cards compact while reaching all spec detail and next-step
commands from one panel. It also establishes the **post-action agent** as a reusable Vector
primitive: Vector orchestrates, then summarizes with Haiku, and downstream commands (standup, and
future daily) consume those summaries.

The decisions already made by the user (Section 10): **one combined spec**; the summary agent runs
after **every domain transition**; the summary is stored **locally/gitignored**; the card face keeps
**metadata only** (title, ticket, status pill, priority, estimate, savings) and becomes clickable.

## 2. Scope

### Included in this phase

**A. Post-action summary pipeline (cli)**

- **Local persistence**: a new `state.SpecSummary` record and a `summaries.json` store at
  `.vector/local/summaries.json` (a map `specID → SpecSummary`), with `Store` methods
  `ReadSummaries()`, `ReadSummary(id)`, and `WriteSummary(id, summary, action, now)` — atomic,
  serialized through the store mutex, mirroring `cli/internal/state/standup.go`.
- **New subcommand `vector spec summarize <id>`** (dispatched from `runSpec`,
  `cli/cmd/vector/main.go:539`):
  - default / `--json`: emits the **summarize projection** JSON for the agent — the spec's
    `title`/`status`/`ticket`, its recent activity timeline, and the **prior summary** (for
    incremental context). Mirrors `vector standup --json`.
  - `commit --action <name> --summary-file -|path`: persists the agent's prose to
    `summaries.json` under that spec id, recording the triggering `action`. On invalid/empty input
    it **writes nothing** (mirrors `runStandupCommit`).
- **Standup reuse**: `standup.SpecActivity` gains a `PriorSummary string` (omitempty) field;
  `enrichProjection` (`cli/cmd/vector/standup.go:90`) fills it from `store.ReadSummary(sa.ID)`, so
  the `vector-standup-writer` agent receives the existing per-spec summary as context.
- **HTTP read endpoint**: `GET /api/summary?spec=<id>` (`cli/internal/board/server.go`) serves the
  persisted `SpecSummary`; `{}` (200) when a spec has no summary yet. The board `Source` interface
  (`cli/internal/board/board.go:130`) gains `ReadSummary(id string) (*state.SpecSummary, error)`.

**B. Post-action agent (kit)**

- **New agent** `kit/agents/vector-summary-writer.md` (model: **Haiku**, read-only): input = the
  summarize projection JSON; output = a short prose summary of what was done on the spec. Mirrors
  `kit/agents/vector-standup-writer.md` in structure and hard rules.
- **Command files gain a post-action summary step** — `apply.md`, `propose.md`, `status.md`,
  `close.md`, `archive.md` (under `kit/commands/vector/`) each end with: run
  `vector spec summarize <id> --json` → spawn `vector-summary-writer` on that JSON → pipe its prose
  to `vector spec summarize <id> commit --action <command> --summary-file -`. The embedded scaffold
  copies under `cli/internal/scaffold/assets/` are regenerated from `kit/` via `go generate`
  (`cli/internal/scaffold/scaffold.go:13`), never hand-edited.
- **Standup agent uses the summary** — `kit/agents/vector-standup-writer.md` gains a `priorSummary`
  input field and a rule to use it as context, so the reuse in `/vector:standup` is real, not
  best-effort.

**C. Spec details drawer (web)**

- **New component** `web/src/components/SpecDetailsDrawer/` (right-side panel) surfacing: header
  (title, status pill, ticket badge), the **AI summary** (new `useSpecSummary` hook →
  `GET /api/summary`), the **activity timeline** (reusing the existing `SpecTimeline`/
  `useSpecActivity` path), the **next command**, and a **Useful commands** section of copyable
  slash commands.
- **Card refactor**: `SpecCard.tsx` drops the inline `NextCommand` and `SpecTimeline`; the
  `<article>` becomes clickable and calls an `onSelect(card)` callback. Header/footer metadata is
  unchanged.
- **One drawer at board level**: `KanbanBoard` owns the selected-card state, threads `onSelect`
  down through `BoardColumn` → `SpecCard`, and renders a single `SpecDetailsDrawer`.
- **New types/hook**: `SpecSummary` type mirroring the Go struct, and `useSpecSummary(specId)` in
  `web/src/api/`.
- **Tests**: Go tests for the summarize subcommand (projection + commit round-trip), the
  `summaries.json` store, the `PriorSummary` enrichment, and `GET /api/summary`; web typecheck +
  build green.

### Out of scope

- **Write endpoints / real mutations from the web.** The board stays **read-only** (no POST/PUT).
  "Assign ticket from the drawer" is a **copyable `/vector:link` command**, not a web form — chosen
  by the user. Drag-and-drop and write endpoints remain a separate, deferred slice
  (`cli/CLAUDE.md`, Pendiente).
- **Triggering the summary agent from `/vector:raw`, `/vector:sync`, or `/vector:link`.** `raw`
  creates a draft (no work done yet), `sync` is a bulk import, and `link` is metadata-only — none
  produce "what was done" prose. (Open question if `link` should refresh the summary.)
- **Committing the summary to git.** It lives in `.vector/local/` (gitignored), per the user's
  decision; it does not travel between machines and is regenerated locally.
- **Crediting the Haiku summary call in the Token Savings Meter.** There is no defined baseline for
  this route; no `agent.routed` event is emitted for it here (future consideration).
- **A drawer on the Standup view's `StandupSpecRow`** or any surface other than board kanban cards.
- **Changing the card's visual style** beyond removing the two inline affordances and adding the
  click handler (header/footer/pills/flags unchanged).
- **Changing the `vector standup commit` contract**, the activity-log schema, or the state machine.

The agent must not implement anything outside the scope above, even if it seems related.

---

## 3. Project technologies and conventions

### Stack

- Language: **Go** (single module, stdlib only) for `cli/`; **TypeScript + React 19 + Vite** for
  `web/`; **Markdown** for `kit/` commands/agents.
- Package manager: `go` toolchain for `cli/`; `npm` for `web/`.
- UI: **CSS Modules** + CSS variables (`web/src/styles/tokens.css`); icons via `lucide-react`. No
  component library (light-bundle rule).
- State (web): SSE/`fetch` projection of `cli/`'s HTTP API; the frontend owns no canonical state.
- API client (web): hand-mirrored types in `web/src/types/*` (no typegen yet).
- Testing: Go `testing` (table-driven); `web/` typecheck + build as the gate.
- Agent tier: **Haiku** for `vector-summary-writer` (cheap prose; `product/token-routing.md`).

### Relevant versions

- Go: `TBD — ver Open questions` (confirm in `cli/go.mod`; not changed here).
- React: 19 (`web/package.json`; not changed here).

No new libraries, APIs, flags, or patterns beyond those already in the project.

### Existing patterns to respect

- **Local-store pattern**: `standup.json` and `activity.jsonl` live under `.vector/local/`
  (gitignored via `.gitignore:25`). `summaries.json` follows the same shape, atomic write
  (`writeFileAtomic`) and mutex (`cli/internal/state/standup.go`, `WriteStandup`).
- **Two-step agent pipeline**: `vector standup --json` (projection) → agent → `vector standup commit`
  (`cli/cmd/vector/standup.go`). `vector spec summarize` mirrors it exactly.
- **Agent contract**: the agent only transforms its input JSON; it never calls the binary or writes
  state (`kit/agents/vector-standup-writer.md`, Hard rules). `vector-summary-writer` follows this.
- **Command files call the binary** for every state write; the binary is the sole writer
  (`kit/commands/vector/apply.md` §5–6).
- **Board `Source` interface** is the read seam web consumes (`cli/internal/board/board.go:130`);
  handlers return `{error}` JSON on failure and `{}` for "nothing yet" (`server.go:55`,
  `handleStandup`).
- **Web**: one component per file/folder, semantic names (`SpecDetailsDrawer`, `SummarySection`,
  `UsefulCommands`), strong typing from the API contract, lazy fetch on open (the `SpecTimeline`
  pattern), copy-button feedback (the `NextCommand` pattern). See `standards/typescript-react.md`.

---

## 4. Prerequisites

Before starting, the following must already exist (all verified present):

- [x] Local store under `.vector/local/` with atomic write + mutex (dir created in `Open()` at
      `cli/internal/state/store.go:25`; `writeFileAtomic` at `cli/internal/state/store.go:468`;
      `standup.go:71` `WriteStandup` as the mirror).
- [x] `standup.Project` / `standup.Timeline` projections and `standup.SpecActivity`
      (`cli/internal/standup/standup.go:42,49`).
- [x] `enrichProjection` store-backed enrichment (`cli/cmd/vector/standup.go:90`).
- [x] `vector standup` / `standup commit` two-step pipeline as the mirror
      (`cli/cmd/vector/standup.go`).
- [x] `runSpec` subcommand dispatch (`cli/cmd/vector/main.go:535`) and `vector spec worklog` as the
      "post-implement, before-transition" seam already wired into `apply.md` (`standup.go:207`).
- [x] Board `Source` interface + HTTP server with `/api/standup` and `/api/activity`
      (`cli/internal/board/board.go:130`, `cli/internal/board/server.go`).
- [x] `kit/agents/vector-standup-writer.md` as the agent template; the five transition command files
      (`apply|propose|status|close|archive.md`) and their scaffold mirrors
      (`cli/internal/scaffold/assets/commands/vector/`).
- [x] Web: `SpecCard` rendering inline `NextCommand` + `SpecTimeline`
      (`web/src/components/SpecCard/SpecCard.tsx:61-63`), `nextCommandFor`
      (`SpecCard/nextCommandFor.ts`), `NextCommand` copy pattern, `TimelineEntry`, `useSpecActivity`
      (`web/src/api/useStandup.ts`), and the `Card`/`Ticket` types (`web/src/types/board.ts`).

If a prerequisite is missing, stop and report exactly what is absent. Do not invent contracts.

---

## 5. Architecture

### Pattern to use

The **orchestrate-then-summarize** primitive, mirroring the existing standup pipeline:
command performs the domain action → command runs `vector spec summarize <id> --json` to get a
read-only projection → command spawns the **Haiku** `vector-summary-writer` agent → command pipes
the prose to `vector spec summarize <id> commit`, which persists it locally. The binary never calls
a model; the agent never writes state. The web drawer is a pure **projection** of read-only
endpoints; it owns no canonical state and performs no mutation.

### Affected layers

- **presentation (web)**: yes — new `SpecDetailsDrawer`; `SpecCard` loses two inline affordances and
  gains a click handler; `KanbanBoard`/`BoardColumn` thread selection; new hook/types.
- **application/use-cases (cli command)**: yes — new `runSpecSummarize` (+ `commit`);
  `enrichProjection` fills `PriorSummary`; new `GET /api/summary` handler.
- **domain (cli state)**: yes — new `state.SpecSummary` + `summaries.json` store methods. The state
  machine, `SpecState`, and the committed `state.json` are **unchanged** (the summary is local).
- **data/infrastructure (cli projection)**: yes — `standup.SpecActivity` gains `PriorSummary`. The
  pure `Project`/`Timeline` functions stay store-free; enrichment stays in the caller.
- **kit**: yes — new agent file; five command files (+ their scaffold mirrors) gain a final step.
- **shared/common**: no.

### Expected flow (pipeline)

1. The user runs a transition command, e.g. `/vector:apply <id>`. The command implements the work,
   logs it (`vector spec worklog`), and transitions the state (existing behavior).
2. As its **final step**, the command runs `vector spec summarize <id> --json`, which projects the
   spec's recent activity + metadata + prior summary.
3. The command spawns `vector-summary-writer` (Haiku) on that JSON; the agent returns short prose.
4. The command runs `vector spec summarize <id> commit --action apply --summary-file -`, which writes
   the prose to `.vector/local/summaries.json` under `<id>`.
5. On the next `/vector:standup`, `enrichProjection` loads each spec's `PriorSummary`, and
   `vector-standup-writer` uses it as context for a more meaningful digest.

### Expected flow (drawer)

1. The user clicks a `SpecCard`. `KanbanBoard` sets the selected card and renders `SpecDetailsDrawer`.
2. The drawer lazily fetches `GET /api/summary?spec=<id>` (AI summary) and the activity timeline
   (`useSpecActivity`) only once open.
3. It renders header + summary + timeline + next command + useful copyable commands.
4. The user copies a command (e.g. `/vector:link <id> <TICKET>` when the spec has no ticket) to paste
   into Claude Code, or closes the drawer (button / Esc / overlay click).

### Location of new files

```txt
cli/
  cmd/vector/summarize.go            # runSpecSummarize + commit (or add to an existing cmd file)
  internal/state/summary.go          # SpecSummary + summaries.json store methods
kit/
  agents/vector-summary-writer.md    # the Haiku post-action agent
web/src/components/SpecDetailsDrawer/
  index.tsx
  SpecDetailsDrawer.module.css
  SummarySection.tsx                 # only if the markup grows past a few lines
  UsefulCommands.tsx                 # the copyable command list (likely its own file)
web/src/api/useSpecSummary.ts        # or co-locate in the existing useStandup.ts
```

No new Go packages; reuse `internal/state`, `internal/standup`, `internal/board`.

---

## 6. Files to create or modify

| Path | Action | Purpose | Project example to follow |
|---|---|---|---|
| `cli/internal/state/summary.go` | NUEVO | `SpecSummary` struct + `summaries.json` store: `ReadSummaries`/`ReadSummary`/`WriteSummary` | `cli/internal/state/standup.go` |
| `cli/cmd/vector/summarize.go` | NUEVO | `runSpecSummarize` (projection `--json`) + commit (`--action`, `--summary-file`) | `cli/cmd/vector/standup.go` (`runStandup`/`runStandupCommit`) |
| `cli/cmd/vector/main.go` | MODIFY | Add `case "summarize": return runSpecSummarize(args[1:])` to `runSpec` (line 539); add usage line | `cli/cmd/vector/main.go:544-561` (the existing cases) |
| `cli/internal/standup/standup.go` | MODIFY | Add `PriorSummary string \`json:"priorSummary,omitempty"\`` to `SpecActivity` | `cli/internal/standup/standup.go:49-54` (existing `Title`/`LastStatus`) |
| `cli/cmd/vector/standup.go` | MODIFY | In `enrichProjection` set `sa.PriorSummary` from `store.ReadSummary(sa.ID)` | `cli/cmd/vector/standup.go:90-108` (the `Ticket` enrichment) |
| `cli/internal/board/board.go` | MODIFY | Add `ReadSummary(id string) (*state.SpecSummary, error)` to the `Source` interface | `cli/internal/board/board.go:130-133` |
| `cli/internal/board/server.go` | MODIFY | Add `handleSummary` + route `/api/summary`; `{}` when none | `cli/internal/board/server.go:55-74` (`handleStandup`) |
| `kit/agents/vector-summary-writer.md` | NUEVO | Haiku agent: summarize projection → short "what was done" prose | `kit/agents/vector-standup-writer.md` |
| `kit/commands/vector/apply.md` | MODIFY | Add a final "post-action summary" step (summarize → agent → commit) | `kit/commands/vector/apply.md` §5 (worklog step) |
| `kit/commands/vector/propose.md` | MODIFY | Same post-action summary step after the transition | `kit/commands/vector/apply.md` (the new step) |
| `kit/commands/vector/status.md` | MODIFY | Same post-action summary step | idem |
| `kit/commands/vector/close.md` | MODIFY | Same post-action summary step | idem |
| `kit/commands/vector/archive.md` | MODIFY | Same post-action summary step | idem |
| `kit/agents/vector-standup-writer.md` | MODIFY | Add `priorSummary` to the input example + a rule to use it as context when present | `kit/agents/vector-standup-writer.md` (existing Input/Hard-rules sections) |
| `cli/internal/scaffold/assets/**` | REGEN | **Not hand-edited** — regenerated from `kit/` by `go generate ./internal/scaffold/...` (see `cli/internal/scaffold/scaffold.go:13`, which `rm -rf assets` then re-copies) | — |
| `web/src/types/board.ts` | MODIFY | Add `SpecSummary` interface mirroring the Go struct | `web/src/types/standup.ts` (`StandupSpecDigest`) |
| `web/src/api/useSpecSummary.ts` | NUEVO | `useSpecSummary(specId)` → `GET /api/summary`; lazy | `web/src/api/useStandup.ts` (`useSpecActivity`) |
| `web/src/components/SpecDetailsDrawer/index.tsx` | NUEVO | The drawer container (header + summary + timeline + commands) | `web/src/components/SpecCard/SpecCard.tsx`, `SpecTimeline/index.tsx` |
| `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css` | NUEVO | Drawer layout/overlay/responsive styles | `web/src/components/SpecCard/SpecCard.module.css` |
| `web/src/components/SpecDetailsDrawer/UsefulCommands.tsx` | NUEVO | Copyable, context-aware command list | `web/src/components/SpecCard/NextCommand.tsx` (copy pattern) |
| `web/src/components/SpecCard/SpecCard.tsx` | MODIFY | Remove inline `NextCommand` + `SpecTimeline`; add `onSelect` click handler | `web/src/components/SpecCard/SpecCard.tsx:61-63` |
| `web/src/components/BoardColumn/BoardColumn.tsx` | MODIFY | Thread `onSelect` from board to each card | `web/src/components/BoardColumn/BoardColumn.tsx:21` |
| `web/src/components/KanbanBoard/KanbanBoard.tsx` | MODIFY | Own `selectedCard` state; render one `SpecDetailsDrawer` | `web/src/components/KanbanBoard/KanbanBoard.tsx` |

### Detail per file

#### cli/internal/state/summary.go

Action: NUEVO. Mirror `standup.go`:

- `type SpecSummary struct { SchemaVersion int; ID string; Summary string; Action string;
  GeneratedAt time.Time }` (JSON-tagged).
- `summariesPath()` → `filepath.Join(s.root, "local", "summaries.json")`.
- `ReadSummaries()` → `map[string]SpecSummary` (missing file → empty map, not an error).
- `ReadSummary(id)` → `*SpecSummary` (nil when absent).
- `WriteSummary(id, summary, action string, now time.Time)` → reads the map, sets the entry, writes
  atomically under the store mutex. Stamps `SchemaVersion` and `GeneratedAt` (UTC).

Restrictions: do not touch `state.json` or the state machine. The summary is derived/local.

#### cli/cmd/vector/summarize.go

Action: NUEVO. Mirror `runStandup`/`runStandupCommit`:

- `runSpecSummarize(args)`: if `args[0] == "commit"` → commit path; else projection path.
- Projection path: resolve the spec, build the agent input — `{ id, title, status, ticket,
  priorSummary, events: [...] }` where `events` is `standup.Timeline(events, id, from)` over a recent
  window (default `24h`, `TBD — ver Open questions` for the exact default), and `priorSummary` is
  `store.ReadSummary(id)`. Emit as indented JSON (no `--json` flag needed if it is the only output,
  but accept `--json` for symmetry).
- Commit path: flags `--action <name>` (the triggering command; required) and
  `--summary-file -|path` (required). Read the prose; if empty or unreadable, **write nothing** and
  return a clear error (mirror `runStandupCommit`'s "invalid json → write nothing"). Else
  `store.WriteSummary(id, prose, action, time.Now())`.
- Reuse `openStore`, `readBody`, `leadingID`, `resolveActor` (already in the package).

#### cli/internal/standup/standup.go

Action: MODIFY. Add `PriorSummary string \`json:"priorSummary,omitempty"\`` to `SpecActivity`
(line 49 area). Do **not** give `Project` store access; the field is filled by the caller.

#### cli/cmd/vector/standup.go

Action: MODIFY. In `enrichProjection` (line 90), after reading `spec`, set
`if sum := store.ReadSummary(sa.ID); sum != nil { sa.PriorSummary = sum.Summary }`. Do not change the
`agentDigest` shape or `runStandupCommit` persistence.

#### cli/internal/board/board.go & server.go

Action: MODIFY.

- `board.go`: add `ReadSummary(id string) (*state.SpecSummary, error)` to the `Source` interface
  (line 130). `*state.Store` already satisfies it once `summary.go` adds the method.
- `server.go`: register `mux.HandleFunc("/api/summary", s.handleSummary)` in `Routes` (line 34 area);
  add `handleSummary` that validates the `spec` query param (400 if missing), calls
  `s.src.ReadSummary(spec)`, writes `{}` (200) when nil, else the marshaled `SpecSummary`. Reuse
  `writeJSONError`. Mirror `handleStandup`/`handleActivity`.

#### kit/agents/vector-summary-writer.md

Action: NUEVO. Mirror `vector-standup-writer.md`:

- Front-matter: model **Haiku**, read-only (tools: `Read`).
- Input: the summarize projection JSON (`id`, `title`, `status`, `ticket?`, `priorSummary?`,
  `events[]`).
- Output: a **single short prose summary** (1–2 sentences) of what was done on the spec this period,
  incorporating `priorSummary` as prior context when present. Plain text (or
  `{ "summary": "..." }` — pick one and keep `runSpecSummarize commit` parsing in sync;
  `TBD — ver Open questions` on the exact wire shape).
- Hard rules: transform input only; never call the binary; never invent activity not in `events`;
  prose follows the conversation language; emit valid output even on empty `events` ("no recorded
  work yet for this period").

#### kit/commands/vector/{apply,propose,status,close,archive}.md (+ scaffold mirrors)

Action: MODIFY. Append a final step after the transition/report:

```
## Post-action summary (cheap Haiku agent)

After the transition, refresh the spec's summary so the drawer and `/vector:standup` reflect what
was just done:

1. `vector spec summarize <id> --json` — get the projection.
2. Spawn `vector-summary-writer` (Haiku) on that JSON; it returns a short "what was done" summary.
3. `vector spec summarize <id> commit --action <command> --summary-file -` — persist the prose.

This is read-only-then-persist via the binary (the binary stays the sole writer). Skip only if the
run genuinely changed nothing.
```

`<command>` is the originating command (`apply`/`propose`/`status`/`close`/`archive`). Do **not**
hand-edit the scaffold copies under `cli/internal/scaffold/assets/`: after editing the `kit/` files,
run `go generate ./internal/scaffold/...` in `cli/` — `scaffold.go:13` wipes and re-copies the assets
from `kit/`, so any manual edit there is overwritten.

#### kit/agents/vector-standup-writer.md

Action: MODIFY. The standup-reuse purpose (Section 1) depends on the agent actually consuming the new
field:

- Add `priorSummary` to the per-spec Input example.
- Add a Hard rule: "When a `priorSummary` is present, use it as context for what was already known,
  and describe what changed since — do not contradict or blindly repeat it." Keep the output JSON
  shape (`{ global, perSpec:[{id, summary}] }`) unchanged.

#### web/src/types/board.ts

Action: MODIFY. Add:

```ts
export interface SpecSummary {
  id: string
  summary: string
  action: string
  generatedAt: string
}
```

#### web/src/api/useSpecSummary.ts

Action: NUEVO. Mirror `useSpecActivity`: `useSpecSummary(specId: string | null)` →
`GET /api/summary?spec=<id>`, lazy (null skips the fetch), returning `{ data, loading, error,
reload }`. Reuse the `useFetchJSON` helper (export it from `useStandup.ts` or co-locate the hook
there to avoid duplication).

#### web/src/components/SpecDetailsDrawer/index.tsx

Action: NUEVO. Props `{ card: Card; onClose: () => void }`. Renders a right-side panel with an
overlay. Sections:

- **Header**: `card.title`, `StatusPill`, ticket badge (reuse `SpecCard`'s badge markup), a close
  button (`aria-label="Close details"`).
- **Summary**: `useSpecSummary(card.id)` → render `summary` prose; loading spinner; error + retry;
  empty state when `{}` ("No summary yet — run a domain command or `/vector:standup`").
- **Activity**: reuse the existing `SpecTimeline` component (pass `specId={card.id}`), or the same
  `useSpecActivity` + `TimelineEntry` it uses — do not duplicate the timeline logic.
- **Next command**: reuse `nextCommandFor(card.status, card.id)` + the `NextCommand` copy affordance.
- **Useful commands**: `UsefulCommands` (below).

UX: open on mount; close on the button, `Escape`, and overlay click; `role="dialog"`,
`aria-modal="true"`; focus the close button on open. Lazy — fetches only while mounted/open.

#### web/src/components/SpecDetailsDrawer/UsefulCommands.tsx

Action: NUEVO. A context-aware list of **copyable** slash commands, each with the `NextCommand`
copy-button pattern (clipboard + check feedback + "paste into Claude Code" title). Include:

- `/vector:link <id> <TICKET-KEY>` — **only when `!card.ticket`** (the "assign a ticket from here"
  case the user asked for).
- `/vector:status <id> <status>`, `/vector:close <id>`, `/vector:archive <id>` — shown when legal for
  the current status (`TBD — ver Open questions` on the exact per-status set; default: show
  status/close for non-terminal, archive for `closed`).

No execution, no mutation — copy only.

#### web/src/components/SpecCard/SpecCard.tsx

Action: MODIFY. Remove the inline `<NextCommand .../>` and `<SpecTimeline .../>` (lines 61–63) and
their imports. Make the `<article>` clickable: add `onClick={() => onSelect(card)}`,
`role="button"`, `tabIndex={0}`, and an `onKeyDown` for Enter/Space. Add `onSelect: (card: Card) =>
void` to `SpecCardProps`. Keep header/footer (title, ticket, status pill, UAT, priority, estimate,
savings) unchanged.

#### web/src/components/BoardColumn/BoardColumn.tsx & KanbanBoard/KanbanBoard.tsx

Action: MODIFY.

- `BoardColumn`: accept `onSelect` and pass it to each `SpecCard`.
- `KanbanBoard`: `const [selected, setSelected] = useState<Card | null>(null)`; pass
  `onSelect={setSelected}` down; render `{selected && <SpecDetailsDrawer card={selected}
  onClose={() => setSelected(null)} />}`. One drawer instance, one open card at a time.

Restrictions: do not introduce a state library or a router; selection is local UI state (the board
remains a projection — selection is not domain state).

---

## 7. API Contract

There is no `docs/api-contract.md` for these internal projections; the Go structs are the source of
truth and `web/src/types/*` mirrors them by hand. Changes:

- **New** `GET /api/summary?spec=<id>` → `SpecSummary` JSON, or `{}` (200) when the spec has no
  summary. `400` on a missing `spec` param; `500` on a store read error (mirrors `handleActivity`).
  Response shape (the Go struct is the source of truth; `web/src/types/board.ts` mirrors it):

  ```json
  { "id": "add-spec-summary-drawer", "summary": "…what was done…", "action": "apply", "generatedAt": "2026-06-26T12:00:00Z" }
  ```
- The `standup` projection (`vector standup --json`) and the agent stdin gain an optional
  `perSpec[].priorSummary` (additive, `omitempty`).
- `GET /api/standup` / `GET /api/activity` are **unchanged**.

The CLI surface adds `vector spec summarize <id> [--json]` and `vector spec summarize <id> commit
--action <name> --summary-file -|path`.

### Endpoints involved

- GET /api/summary?spec=<id> (new)
- GET /api/activity?spec=<id>&since=<dur> (existing, consumed by the drawer)

---

## 8. Success criteria

The implementation is correct when:

- [ ] `state.SpecSummary` + `summaries.json` store (`ReadSummaries`/`ReadSummary`/`WriteSummary`)
      exist, atomic + mutex, missing-file → empty, mirroring `standup.go`.
- [ ] `vector spec summarize <id> --json` emits a projection with `id/title/status/ticket?/
      priorSummary?/events[]`; `commit --action <name> --summary-file -` persists the prose and
      writes nothing on empty/invalid input.
- [ ] `runSpec` dispatches `summarize`; `vector spec` usage lists it.
- [ ] `standup.SpecActivity.PriorSummary` is filled by `enrichProjection`; `vector standup --json`
      carries it; `Project` stays store-free.
- [ ] `GET /api/summary?spec=<id>` serves the summary (or `{}`); `Source` has `ReadSummary`.
- [ ] `vector-summary-writer` (Haiku) agent + the five command files (and their scaffold mirrors)
      run the summarize → agent → commit step after their transition.
- [ ] Board cards are clickable; `NextCommand` + `SpecTimeline` no longer render on the card face;
      the drawer shows summary + activity + next command + copyable useful commands.
- [ ] The drawer shows `/vector:link` only when the spec has no ticket; commands copy to clipboard
      with feedback; nothing mutates from the web.
- [ ] No regression: standup commit, marker advance, `/api/activity`, the state machine, and other
      board views are intact.
- [ ] `go vet`/`go test` green; web typecheck + build succeed; binary rebuilt + reinstalled.

### Required tests

- [ ] `summaries.json` round-trip: `WriteSummary` then `ReadSummary`/`ReadSummaries`; missing file →
      empty map.
- [ ] `runSpecSummarize commit`: valid prose persists; empty/invalid input writes nothing.
- [ ] `enrichProjection`: a spec with a stored summary gets `PriorSummary`; one without stays empty.
- [ ] `GET /api/summary`: present → 200 with body; absent → `{}`; missing param → 400.
- [ ] (web) typecheck passes with the new types/hook; build succeeds; drawer opens/closes and renders
      the empty-summary state.

### Verification commands

```bash
go -C cli vet ./...
go -C cli test ./...
npm --prefix web run typecheck
npm --prefix web run build
```

The phase is not complete if any of these fail.

---

## 9. UX criteria

### Card

- The whole card is a clickable control (`role="button"`, keyboard-activable). No inline next-command
  or activity sections remain on the face; metadata (title, ticket, status, priority, estimate,
  savings) is unchanged.
- A subtle hover/focus affordance signals clickability (CSS only; no new icon required).

### Drawer

- Opens as a right-side panel over a dim overlay; one open at a time.
- **Close**: header close button, `Escape`, and overlay click all close it.
- **Summary**: shows the AI prose; while loading, a spinner; on error, an inline message + `Retry`;
  when `{}`, an empty hint ("No summary yet — run a domain command or `/vector:standup`").
- **Activity**: the existing lazy timeline (loading/empty/error/retry as today).
- **Next command** + **Useful commands**: each command is copyable with check feedback and a "paste
  into Claude Code to run" tooltip. `/vector:link` appears only when the spec has no ticket.
- Content scrolls inside the drawer; the close button has a comfortable hit target on mobile.

### Accessibility

- `role="dialog"`, `aria-modal="true"`, labelled by the spec title; focus moves to the drawer on
  open and is restored to the card on close; copy buttons have `aria-label`s.

---

## 10. Decisions made

Settled by the user — do not re-litigate:

- **One combined spec** covering the post-action pipeline (cli + kit) and the drawer (web).
- The summary agent runs after **every domain transition** (`propose`, `apply`, `status`, `close`,
  `archive`). `raw`/`sync`/`link` are excluded (no "what was done" content).
- The summary is **stored locally and gitignored** (`.vector/local/summaries.json`); it does not
  travel via git and is regenerated locally.
- The card face keeps **metadata only** and becomes clickable; `NextCommand` + `SpecTimeline` move
  into the drawer.
- "Assign ticket from the drawer" is a **copyable `/vector:link` command**, not a web mutation. The
  board stays **read-only**.
- The summary agent is **Haiku** (cheap), following the existing standup-pipeline pattern.
- Reuse existing seams: the standup two-step pipeline, `enrichProjection`, the `Source` interface,
  the `NextCommand`/`SpecTimeline` web patterns. No new types or layers beyond those listed.

If the agent sees a seemingly better alternative, report it as an observation; do not implement it.

---

## 11. Edge cases

### Spec has no summary yet

- `ReadSummary` → nil; `GET /api/summary` → `{}`. Drawer shows the empty hint; standup
  `PriorSummary` stays empty (current standup behavior). Common case; must stay graceful.

### Domain action on a spec with no new activity

- The summarize projection's `events` is empty. The agent returns a short "no recorded work yet for
  this period" line; commit still persists it (or, if the command judges nothing changed, it skips
  the step — `apply.md` already allows skipping the worklog on a no-op run).

### Agent returns empty/invalid output

- `vector spec summarize commit` **writes nothing** and errors clearly (mirror
  `runStandupCommit` "invalid digest json"). The previous summary (if any) is preserved.

### Summary fetch fails (drawer)

- The summary section shows an inline error + `Retry`; the rest of the drawer (activity, commands)
  still renders. No new global failure mode.

### Card clicked while a drawer is open

- Selecting another card swaps the drawer's `card`; `Escape`/overlay/close clears it. Re-clicking the
  same card is idempotent.

### Concurrent summarize writes

- `WriteSummary` is serialized through the store mutex and atomic (like `WriteStandup`); the
  last writer wins for a given spec id, never a partial file.

### Standup after a summary exists

- `enrichProjection` includes `PriorSummary`; `vector-standup-writer` uses it as context. If the
  agent ignores it, the digest is no worse than today (additive, non-breaking).

### Clipboard unavailable

- Reuse `NextCommand`'s guard (`if (!navigator.clipboard) return`); the copy button no-ops safely.

### Board offline / `vector serve` down

- `GET /api/summary` simply doesn't resolve; the drawer's summary section shows its error/empty state
  (same handling as `useSpecActivity`). No new dependency.

### Timeout (drawer fetch)

- `useFetchJSON` (`web/src/api/useStandup.ts`) sets no explicit request timeout; a stalled
  `GET /api/summary` resolves as a fetch error. The summary section shows the inline error + `Retry`
  (identical to the offline case). No new behavior is introduced — same as the existing
  `useSpecActivity` path.

### Double submit

- N/A — every server endpoint is GET (read-only); the summarize pipeline is CLI-sequential with no
  concurrent web form; copy-to-clipboard is idempotent. There is no mutation to double-submit.

### HTTP error codes (400/401/403/404/409/422/429/500)

- Local, auth-free, read-only server. `/api/summary`: 400 (missing `spec`), 500 (store read error);
  "no summary" is `{}` (200), not 404. No auth/conflict/validation codes apply (mirrors
  `handleStandup`/`handleActivity`).

---

## 12. Required UI states

| State | What is shown | What the user can do |
|---|---|---|
| card idle | metadata only, clickable | click/Enter to open the drawer |
| drawer open, summary loading | spinner in the summary section | wait / read activity / close |
| drawer open, summary present | AI prose summary | read; copy commands; close |
| drawer open, summary empty (`{}`) | "No summary yet — run a domain command or `/vector:standup`" | copy commands; close |
| drawer open, summary error | inline error + `Retry` | retry; read activity; close |
| activity loading/empty/error | delegated to the existing `SpecTimeline` states | as today |
| offline (`vector serve` down) | summary section shows the inline fetch error + `Retry` (no new request type) | retry / close |
| command copied | check icon + copied state (~1.5s) | paste into Claude Code |

---

## 13. Validations

Read-only + copy-to-clipboard; no user-facing forms.

| Field | Rule | Message |
|---|---|---|
| `summarize commit --action` | required, non-empty | usage error |
| `summarize commit --summary-file` | required; empty/unreadable → write nothing | clear CLI error |
| `GET /api/summary?spec` | required | `400 missing spec query parameter` |

Server-side: `summarize commit` keeps the "empty/invalid input → write nothing" rule.

---

## 14. Security and permissions

- Summaries are non-secret prose about the user's own specs; the Haiku agent receives only the
  projection JSON (no credentials). `.vector/local/summaries.json` is gitignored and personal.
- Copyable commands contain only the spec id / ticket key — no secrets. Execution happens in Claude
  Code, outside Vector.
- The local server stays auth-free and read-only; no new write surface is exposed.

---

## 15. Observability and logging

- No new event types in `activity.jsonl`; the summary is a derived local artifact, not an event.
  (`agent.routed` crediting for the Haiku call is out of scope.)
- Reuse existing error paths: `enrichProjection` already `continue`s on a failed `ReadSpec`; the
  summarize commit returns a clear error and writes nothing on bad input.

---

## 16. i18n / visible text

Vector has no i18n layer; CLI/board strings are English in code. The agent prose follows the
**conversation language** (as `vector-standup-writer.md` already does). New visible strings:

| Key | Text |
|---|---|
| drawer.summary.empty | "No summary yet — run a domain command or /vector:standup." |
| drawer.summary.error | "Failed to load summary." + "Retry" |
| drawer.close | aria-label "Close details" |
| commands.copyHint | title "Paste into Claude Code to run." |

Exact wording is `TBD — ver Open questions` if the user wants different copy.

---

## 17. Performance

- `useSpecSummary` and the activity timeline fetch **lazily** only when a drawer is open — the board
  never loads per-spec summaries for every card.
- `ReadSummary` is a single map lookup over one small local file; `WriteSummary` is one atomic write.
- The Haiku summary call is cheap by design (`product/token-routing.md`); it runs once per domain
  action, not per board render.
- The drawer is unmounted on close (hooks clean up; `useSpecActivity` already uses cancellation).

---

## 18. Restrictions

The agent must not:

- Add web write endpoints or mutate state from the drawer (board stays read-only).
- Commit the summary to git or store it in `state.json` (it is local/gitignored).
- Give `standup.Project`/`Timeline` store access or change their signatures.
- Change the `agentDigest`/`vector standup commit` contract or the activity-log schema.
- Trigger the summary agent from `raw`/`sync`/`link` (out of scope).
- Add new dependencies, a state library, or a router in `web/`.
- Use `any` / `interface{}` outside justified (de)serialization boundaries.
- Refactor unrelated code or change other board/standup views beyond what is listed.
- Let the scaffold mirrors drift from the kit originals.
- Ignore lint/vet/typecheck/test failures.

---

## 19. Deliverables

On completion:

- [ ] `state.SpecSummary` + `summaries.json` store with tests.
- [ ] `vector spec summarize <id>` (projection + commit) wired into `runSpec`, with tests.
- [ ] `SpecActivity.PriorSummary` + `enrichProjection` fill, with a test.
- [ ] `GET /api/summary` + `Source.ReadSummary`, with a handler test.
- [ ] `vector-summary-writer.md` (Haiku) created; the five command files run the post-action summary
      step; `vector-standup-writer.md` updated to consume `priorSummary`; scaffold assets regenerated
      via `go generate ./internal/scaffold/...` (not hand-edited).
- [ ] `SpecDetailsDrawer` + `UsefulCommands` + `useSpecSummary` + `SpecSummary` type; `SpecCard`
      clickable with inline affordances removed; `KanbanBoard` owns the single drawer.
- [ ] Gate green: `go vet`, `go test`, web typecheck, web build.
- [ ] Binary rebuilt + reinstalled to `~/.local/bin/vector` (dogfooding uses the PATH binary).

---

## 20. Final checklist for the agent

- [ ] Read this whole spec.
- [ ] Confirmed the reuse seams (standup two-step pipeline, `enrichProjection`, `Source` interface,
      `NextCommand`/`SpecTimeline` web patterns) — no new types/layers beyond those listed.
- [ ] `summaries.json` is local/gitignored and atomic+mutex like `standup.json`.
- [ ] `vector spec summarize` mirrors `vector standup`/`standup commit` (write nothing on bad input).
- [ ] The five command files **and** their scaffold mirrors gained the post-action step identically.
- [ ] `vector-summary-writer` is Haiku, read-only, transform-only.
- [ ] The drawer is read-only; `/vector:link` shows only when the spec has no ticket; nothing mutates
      from the web.
- [ ] Card face is metadata-only and clickable; inline `NextCommand`/`SpecTimeline` removed.
- [ ] Added Go tests (store round-trip, summarize commit, `PriorSummary`, `/api/summary`); web
      typecheck + build green.
- [ ] Ran `go vet`, `go test`, web typecheck, web build.
- [ ] Rebuilt and reinstalled the `vector` binary.
- [ ] Left no temporary logs or unjustified TODOs.
