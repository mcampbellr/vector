import { Flag } from 'lucide-react'
import type { Priority, Status } from '../../types/board'
import styles from './SpecCard.module.css'

const LOUD: Record<Priority, string | null> = {
  urgent: 'Urgent',
  high: 'High',
  normal: null,
  low: null,
}

const CLASSES: Record<'urgent' | 'high', string> = {
  urgent: styles.priorityUrgent,
  high: styles.priorityHigh,
}

interface CardPriorityFlagProps {
  priority: Priority
  status: Status
}

// CardPriorityFlag is the board-face flag, deliberately narrower than the shared
// PriorityFlag used by the drawer and the palette: only urgent and high print.
// Normal is most of the board, so labelling it is pure noise and it dims the
// urgent alarm; low prints nothing either. A closed card prints nothing at all —
// a closed urgent is no longer a call to action.
export function CardPriorityFlag({ priority, status }: CardPriorityFlagProps) {
  const label = LOUD[priority]
  if (label === null || status === 'closed') return null
  const loud = priority as 'urgent' | 'high'

  return (
    <span
      className={`${styles.priority} ${CLASSES[loud]}`}
      title={`Priority: ${label}`}
    >
      <Flag size={11} strokeWidth={2} fill="currentColor" />
      {label}
    </span>
  )
}
