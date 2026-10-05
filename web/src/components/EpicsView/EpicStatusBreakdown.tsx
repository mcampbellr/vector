import type { EpicSummary } from '../../types/board'
import { EPIC_STATUS_ORDER } from './helpers'
import styles from './EpicsView.module.css'

interface EpicStatusBreakdownProps {
  byStatus: EpicSummary['byStatus']
}

// EpicStatusBreakdown lists how the epic's specs spread across the lifecycle —
// only the statuses that have specs, in lifecycle order, in the board's
// lowercase status vocabulary.
export function EpicStatusBreakdown({ byStatus }: EpicStatusBreakdownProps) {
  const entries = EPIC_STATUS_ORDER.filter((status) => (byStatus[status] ?? 0) > 0)
  if (entries.length === 0) return null

  return (
    <ul className={styles.breakdown} aria-label="Specs by status">
      {entries.map((status) => (
        <li key={status} className={styles.breakdownItem}>
          <span className={styles.breakdownStatus}>{status}</span>
          <span className={styles.breakdownCount}>{byStatus[status]}</span>
        </li>
      ))}
    </ul>
  )
}
