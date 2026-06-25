import { Flag } from 'lucide-react'
import type { Priority } from '../../types/board'
import styles from './PriorityFlag.module.css'

const LABELS: Record<Priority, string> = {
  urgent: 'Urgent',
  high: 'High',
  normal: 'Normal',
  low: 'Low',
}

interface PriorityFlagProps {
  priority: Priority
}

export function PriorityFlag({ priority }: PriorityFlagProps) {
  return (
    <span className={`${styles.flag} ${styles[priority]}`} title={`Priority: ${LABELS[priority]}`}>
      <Flag size={13} strokeWidth={2} fill="currentColor" />
      {LABELS[priority]}
    </span>
  )
}
