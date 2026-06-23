# Vector — Esquemas: `state.json` y `activity.jsonl`

> Definiciones canónicas (Go, lenguaje de impl del CLI). El CLI Go es el **único escritor**.
> Reconciliación con la decisión 1: el state lo **posee Vector** (`.vector/specs/<id>/state.json`)
> y *referencia* el change de OpenSpec una vez aplicado — porque un spec existe como `open`
> antes de que exista ningún change. No se anida dentro de `openspec/`.

## Layout de archivos

```
.vector/
├── specs/<id>/state.json     # committed, 1 archivo por spec (sharded → conflictos locales)
├── local/activity.jsonl      # gitignored, append-only, personal → /vector:daily + token meter
└── board.json                # gitignored, DERIVADO (lo regenera `vector serve`)
openspec/changes/<id>/        # proposal/design/tasks (lo crea /vector:apply); state lo referencia
```

`.gitignore`: `.vector/local/` y `.vector/board.json`.

## `state.json` — fuente de verdad por-spec (committed)

```go
package vector

type Status string

const (
	StatusOpen           Status = "open"
	StatusInProgress     Status = "in-progress"
	StatusNeedsAttention Status = "needs-attention"
	StatusReview         Status = "review"
	StatusClosed         Status = "closed"
	StatusArchived       Status = "archived"
)

type Priority string

const (
	PriorityUrgent Priority = "urgent"
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

type TicketProvider string

const (
	TicketJira   TicketProvider = "jira"
	TicketLinear TicketProvider = "linear"
	TicketGitHub TicketProvider = "github"
	TicketOther  TicketProvider = "other"
)

// SpecState lives at .vector/specs/<id>/state.json — committed, slow-changing.
// Token economics are NOT here (they churn per-run and are personal); they live
// in activity.jsonl and are rolled up into board.json.
type SpecState struct {
	SchemaVersion int      `json:"schemaVersion"`        // migration guard
	ID            string   `json:"id"`                   // kebab-case, == openspec change name when applied
	Title         string   `json:"title"`
	Status        Status   `json:"status"`
	Priority      Priority `json:"priority"`
	Repo          string   `json:"repo"`                 // for multi-repo boards
	Stage         string   `json:"stage,omitempty"`      // optional kanban column override
	Assignee      string   `json:"assignee,omitempty"`   // git handle → powers "my specs"
	Labels        []string `json:"labels,omitempty"`
	EstimateMin   int      `json:"estimateMinutes,omitempty"`

	Ticket   *Ticket    `json:"ticket,omitempty"`
	OpenSpec *OpenSpec  `json:"openspec,omitempty"`      // nil until /vector:apply
	Flag     *Attention `json:"needsAttention,omitempty"`// set iff Status == needs-attention

	// RFC3339 UTC. These timestamps give cycle-time analytics without a history
	// array — full transition history = git log of this file + activity.jsonl.
	CreatedAt  time.Time  `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`   // set on apply
	ReviewAt   *time.Time `json:"reviewAt,omitempty"`
	ClosedAt   *time.Time `json:"closedAt,omitempty"`
	ArchivedAt *time.Time `json:"archivedAt,omitempty"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type Ticket struct {
	Provider TicketProvider `json:"provider"`
	Key      string         `json:"key"`   // e.g. MH-1438
	URL      string         `json:"url"`
	Auto     bool           `json:"auto"`  // true if auto-detected from /vector:raw text
}

type OpenSpec struct {
	Change    string      `json:"change"`    // openspec/changes/<change>
	Artifacts ArtifactSet `json:"artifacts"`
}

type ArtifactSet struct {
	Proposal bool `json:"proposal"`
	Design   bool `json:"design"`
	Tasks    bool `json:"tasks"`
}

type Attention struct {
	Reason string    `json:"reason"`
	Since  time.Time `json:"since"`
	Source string    `json:"source,omitempty"` // "hook" | "command"
}
```

### Ejemplo `.vector/specs/new-patient-expediente/state.json`

```json
{
  "schemaVersion": 1,
  "id": "new-patient-expediente",
  "title": "New patient expediente",
  "status": "needs-attention",
  "priority": "high",
  "repo": "cdr",
  "assignee": "mariocampbell",
  "labels": ["backend", "bff"],
  "estimateMinutes": 90,
  "ticket": { "provider": "jira", "key": "MH-1438", "url": "https://…/MH-1438", "auto": true },
  "openspec": { "change": "new-patient-expediente", "artifacts": { "proposal": true, "design": true, "tasks": false } },
  "needsAttention": { "reason": "Ambiguous money-assembler DTO contract", "since": "2026-06-22T15:04:05Z", "source": "hook" },
  "createdAt": "2026-06-22T14:00:00Z",
  "startedAt": "2026-06-22T14:40:00Z",
  "updatedAt": "2026-06-22T15:04:05Z"
}
```

## `activity.jsonl` — log personal (gitignored, append-only)

Una línea = un evento. Append-only ⇒ sin conflictos, crash-safe. Reconstruye `/vector:daily`
y el **Token Savings Meter**.

```go
type EventType string

const (
	EvtSpecCreated   EventType = "spec.created"
	EvtSpecLinked    EventType = "spec.linked"
	EvtStatusChanged EventType = "status.changed"
	EvtNoteAdded     EventType = "note.added"
	EvtReminderSet   EventType = "reminder.set"
	EvtSpecApplied   EventType = "spec.applied"
	EvtSpecClosed    EventType = "spec.closed"
	EvtSpecArchived  EventType = "spec.archived"
	EvtBoardMoved    EventType = "board.moved"
	EvtAgentRouted   EventType = "agent.routed" // Token Savings Meter
)

// Event is one line of .vector/local/activity.jsonl.
type Event struct {
	V      int             `json:"v"`
	TS     time.Time       `json:"ts"`
	Type   EventType       `json:"type"`
	SpecID string          `json:"specId,omitempty"`
	Repo   string          `json:"repo"`
	Actor  string          `json:"actor"`         // git handle
	Data   json.RawMessage `json:"data,omitempty"`// typed per Type (decode via type switch)
}

// Typed payloads — no `any`.
type SpecCreatedData   struct{ Title, Source, Template string }
type StatusChangedData struct{ From, To Status; Trigger, Reason string } // Trigger: command|hook|apply
type NoteAddedData     struct{ Text string; Pinned bool }
type ReminderSetData   struct{ Text string; DueAt *time.Time }
type BoardMovedData    struct{ From, To string }

// AgentRoutedData is the commercialization wedge: every cheap-agent route logs
// what it would have cost on the baseline model.
type AgentRoutedData struct {
	Task      string  `json:"task"`
	Model     string  `json:"model"`     // cheap model actually used
	Baseline  string  `json:"baseline"`  // model that would've run otherwise
	TokensIn  int     `json:"tokensIn"`
	TokensOut int     `json:"tokensOut"`
	CostUSD   float64 `json:"costUsd"`
	SavedUSD  float64 `json:"savedUsd"`  // baselineCost - costUsd
}
```

### Ejemplo `.vector/local/activity.jsonl`

```jsonl
{"v":1,"ts":"2026-06-22T14:00:00Z","type":"spec.created","specId":"new-patient-expediente","repo":"cdr","actor":"mariocampbell","data":{"title":"New patient expediente","source":"raw","template":"idea"}}
{"v":1,"ts":"2026-06-22T14:01:10Z","type":"spec.linked","specId":"new-patient-expediente","repo":"cdr","actor":"mariocampbell","data":{"provider":"jira","key":"MH-1438","auto":true}}
{"v":1,"ts":"2026-06-22T14:40:00Z","type":"status.changed","specId":"new-patient-expediente","repo":"cdr","actor":"mariocampbell","data":{"from":"open","to":"in-progress","trigger":"apply"}}
{"v":1,"ts":"2026-06-22T14:55:30Z","type":"agent.routed","specId":"new-patient-expediente","repo":"cdr","actor":"mariocampbell","data":{"task":"summarize ADRs","model":"haiku","baseline":"opus","tokensIn":18200,"tokensOut":900,"costUsd":0.02,"savedUsd":0.31}}
{"v":1,"ts":"2026-06-22T15:04:05Z","type":"status.changed","specId":"new-patient-expediente","repo":"cdr","actor":"mariocampbell","data":{"from":"in-progress","to":"needs-attention","trigger":"hook","reason":"Ambiguous money-assembler DTO contract"}}
```

## `board.json` (derivado, no committed)

`vector serve` escanea `.vector/specs/*/state.json` → agrupa por `status` (columnas) y ordena
por `priority`+`updatedAt`; cruza `activity.jsonl` para el roll-up de tokens ahorrados. Se
regenera, nunca se edita a mano → cero conflictos.

## Decisiones de diseño (por qué)

- **Token economics fuera del state committed**: cambian por-run y son personales → en
  `activity.jsonl`; evita churn/conflictos en el archivo compartido.
- **Sin array de historial en state.json**: las transiciones se reconstruyen de `git log` del
  archivo + `activity.jsonl`. Timestamps de ciclo (`startedAt`/`reviewAt`/`closedAt`) bastan
  para analytics sin inflar el archivo.
- **`id` = slug kebab** (no ULID): legible en CLI (`/vector:status new-checkout-flow review`)
  y mapea 1:1 al nombre del change de OpenSpec.
- **Status en kebab-case** (`needs-attention`), display mapea a "Needs attention".
- **`schemaVersion`**: permite migraciones del CLI sin romper repos existentes.

## Pendiente

- ¿Validación: JSON Schema generado desde los structs Go para validar en el panel web (React)?
- ¿`board.json` necesita un shape público estable para el frontend, o el panel consume un
  endpoint del `vector serve` (Go API) en vez del archivo?
