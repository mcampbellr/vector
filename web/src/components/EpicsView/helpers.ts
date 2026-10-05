import type { Card, Column, EpicMemberStatus } from '../../types/board'

/** Display order of an epic's status breakdown: the board columns, then archived. */
export const EPIC_STATUS_ORDER: EpicMemberStatus[] = [
  'open',
  'in-progress',
  'needs-attention',
  'review',
  'closed',
  'archived',
]

/** Groups the board's cards by epic id, keeping the server's column order (and,
 *  within a column, its focus → priority → recency order). Cards without an
 *  epic are collected under the '' key. */
export function groupCardsByEpic(columns: Column[]): Map<string, Card[]> {
  const groups = new Map<string, Card[]>()
  for (const column of columns) {
    for (const card of column.cards) {
      const key = card.epic ?? ''
      const group = groups.get(key)
      if (group) group.push(card)
      else groups.set(key, [card])
    }
  }
  return groups
}

/** Whole-percent progress (done of total), 0 for an empty epic. */
export function progressPercent(done: number, total: number): number {
  if (total <= 0) return 0
  return Math.round((done / total) * 100)
}

/** Parses the order field: '' → unordered (0); otherwise a whole number ≥ 0
 *  (validated again by the server). */
export function parseOrderInput(raw: string): number | null {
  const trimmed = raw.trim()
  if (trimmed === '') return 0
  if (!/^\d+$/.test(trimmed)) return null
  return Number(trimmed)
}
