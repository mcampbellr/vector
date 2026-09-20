import type { Status } from '../types/board'

const RAIL_VAR: Record<Status, string> = {
  draft: 'var(--rail-draft)',
  open: 'var(--rail-open)',
  'in-progress': 'var(--rail-in-progress)',
  'needs-attention': 'var(--rail-needs-attention)',
  review: 'var(--rail-review)',
  closed: 'var(--rail-closed)',
}

/**
 * The CSS colour expression for a status rail — the 2px stripe on a card and the
 * 5px dot on a column header. Returned as a `var(--rail-*)` reference (not a
 * literal) so the value keeps following the active theme; it is applied inline
 * because the same six rails are consumed by two different CSS Modules.
 */
export function statusRailColor(status: Status): string {
  return RAIL_VAR[status]
}
