// Board contract — mirrors internal/board/board.go (the cli/ → web/ API shape).
// The frontend owns no canonical state; this is the single shape it renders.
// When the API gains a typegen step, replace this hand-mirror with the generated
// types (standards/typescript-react.md).

export type Status =
  | 'draft'
  | 'open'
  | 'in-progress'
  | 'needs-attention'
  | 'review'
  | 'closed'

export type Priority = 'urgent' | 'high' | 'normal' | 'low'

export interface Ticket {
  provider: string
  key: string
  url: string
}

export interface Artifacts {
  proposal: boolean
  design: boolean
  tasks: boolean
}

export type RelatedKind = 'spec' | 'ticket'
export type RelatedSource = 'blame' | 'manual'

/** A cause→bug relation shown read-only on the card; mirrors Go board.RelatedItem.
 *  `ref` is a Vector spec id (kind 'spec') or a provider:key (kind 'ticket'). */
export interface RelatedItem {
  kind: RelatedKind
  ref: string
  source: RelatedSource
}

/** An Excalidraw wireframe attached to a spec; mirrors Go state.SketchRef.
 *  Served download-only via GET /api/file?spec=<id>&artifact=sketch. */
export interface SketchRef {
  name: string
  createdAt: string
}

export interface Card {
  id: string
  title: string
  status: Status
  priority: Priority
  repo?: string
  stage?: string
  assignee?: string
  labels?: string[]
  estimateMinutes?: number
  ticket?: Ticket
  relatedTo?: RelatedItem[]
  hasOpenSpec: boolean
  /** repo-relative path to the authored spec doc; mirrors Go Card.SpecDoc. */
  specDoc?: string
  artifacts?: Artifacts
  attentionReason?: string
  needsUat?: boolean
  /** /vector:quick one-run change; rendered as a read-only badge. */
  quickWin?: boolean
  /** Attached Excalidraw wireframes; each is a download-only artifact entry. */
  sketches?: SketchRef[]
  savedUsd: number
  routes: number
  tokensIn: number
  tokensOut: number
  /** This spec's own per-model token breakdown; absent when it has no routes. */
  byModel?: ModelRollup[]
  updatedAt: string
}

/** GET /api/summary?spec=<id> — a spec's persisted post-action summary. `{}`
 *  (all fields absent) when none has been generated yet. Mirrors
 *  internal/state.SpecSummary. */
export interface SpecSummary {
  schemaVersion?: number
  id?: string
  summary?: string
  action?: string
  generatedAt?: string
}

export interface Column {
  status: Status
  label: string
  cards: Card[]
  count: number
}

export interface ModelRollup {
  model: string
  baseline: string
  routes: number
  tokensIn: number
  tokensOut: number
  savedUsd: number
}

export interface TokenSavings {
  totalSavedUsd: number
  totalSpentUsd: number
  baselineUsd: number
  routes: number
  tokensIn: number
  tokensOut: number
  byModel: ModelRollup[]
  /**
   * Data quality of the rolled-up savings.
   *   "actual"    — every contributing event was harness-reported (exact).
   *   "estimated" — at least one event was self-reported by a command.
   *   absent / "" — no routes recorded (meter empty; no badge shown).
   */
  precision?: 'actual' | 'estimated'
}

export interface Totals {
  specs: number
}

export interface Board {
  schemaVersion: number
  repo: string
  generatedAt: string
  updatedAt: string
  columns: Column[]
  tokenSavings: TokenSavings
  totals: Totals
}
