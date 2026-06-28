// Standup contract — mirrors internal/state.StandupDigest and the standup
// handlers in internal/board/server.go (GET /api/standup, GET /api/activity).
// Hand-mirrored until the API gains a typegen step (standards/typescript-react.md).

import type { Status, Ticket } from './board'

/** One spec's line in the persisted standup digest. */
export interface StandupSpecDigest {
  id: string
  title: string
  status: Status | ''
  summary: string
  changeCount: number
  ticket?: Ticket
}

export interface StandupTotals {
  specs: number
  changes: number
  byStatus: Record<string, number>
}

/** GET /api/standup — the persisted last-standup digest. `{}` (all fields
 *  absent) when no standup has been committed yet. */
export interface StandupDigest {
  schemaVersion?: number
  generatedAt?: string
  since?: string
  markerAt?: string
  global?: string
  perSpec?: StandupSpecDigest[]
  totals?: StandupTotals
}

/** One flattened entry of a spec's activity timeline. Fields are present per
 *  event type: status.changed carries from/to/trigger/reason; work.logged
 *  carries filesTouched/tasksCompleted/note. */
export interface ActivityEvent {
  ts: string
  type: string
  from?: string
  to?: string
  trigger?: string
  reason?: string
  filesTouched?: string[]
  tasksCompleted?: string[]
  note?: string
}

/** GET /api/activity?spec=<id>&since=<dur> — a spec's projected timeline. */
export interface SpecActivity {
  spec: string
  since: string
  events: ActivityEvent[]
}
