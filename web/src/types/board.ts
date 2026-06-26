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
  hasOpenSpec: boolean
  artifacts?: Artifacts
  attentionReason?: string
  needsUat?: boolean
  savedUsd: number
  routes: number
  updatedAt: string
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
