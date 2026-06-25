import type { ActivityEvent } from '../../types/standup'
import { relativeTime } from '../../lib/format'
import { useNow } from '../../lib/useNow'
import styles from './SpecTimeline.module.css'

interface TimelineEntryProps {
  event: ActivityEvent
}

// describe renders the human label for one activity event by type, grounded only
// in the fields the event carries (status.changed vs work.logged).
function describe(event: ActivityEvent): { label: string; detail: string | null } {
  if (event.type === 'status.changed') {
    const transition = `${event.from ?? '?'} → ${event.to ?? '?'}`
    return { label: transition, detail: event.reason ?? null }
  }
  if (event.type === 'work.logged') {
    const parts: string[] = []
    if (event.tasksCompleted?.length) parts.push(event.tasksCompleted.join(', '))
    if (event.filesTouched?.length) {
      const count = event.filesTouched.length
      parts.push(`${count} file${count === 1 ? '' : 's'}`)
    }
    return { label: event.note || 'work logged', detail: parts.length ? parts.join(' · ') : null }
  }
  return { label: event.type, detail: null }
}

export function TimelineEntry({ event }: TimelineEntryProps) {
  const now = useNow(30000)
  const { label, detail } = describe(event)
  const kind = event.type === 'work.logged' ? styles.work : styles.transition

  return (
    <li className={styles.entry}>
      <span className={`${styles.marker} ${kind}`} aria-hidden="true" />
      <div className={styles.body}>
        <span className={styles.label}>{label}</span>
        {detail && <span className={styles.detail}>{detail}</span>}
      </div>
      <time className={styles.time} dateTime={event.ts}>
        {relativeTime(event.ts, now)}
      </time>
    </li>
  )
}
