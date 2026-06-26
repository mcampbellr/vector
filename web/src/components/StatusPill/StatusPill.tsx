import type { Status } from '../../types/board'
import styles from './StatusPill.module.css'

const LABELS: Record<Status, string> = {
  draft: 'Draft',
  open: 'Open',
  'in-progress': 'Progress',
  'needs-attention': 'Attention',
  review: 'Review',
  closed: 'Done',
}

interface StatusPillProps {
  status: Status
}

export function StatusPill({ status }: StatusPillProps) {
  return <span className={`${styles.pill} ${styles[status]}`}>{LABELS[status]}</span>
}
