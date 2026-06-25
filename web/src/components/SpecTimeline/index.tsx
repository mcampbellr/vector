import { useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { useSpecActivity } from '../../api/useStandup'
import { TimelineEntry } from './TimelineEntry'
import styles from './SpecTimeline.module.css'

interface SpecTimelineProps {
  specId: string
  /** How many events to show before "show more". Answers the design's open
   *  question on truncation (default 20). */
  limit?: number
}

// SpecTimeline is the per-card activity history: a read-only, expandable list fed
// lazily by GET /api/activity. It is collapsed by default and only fetches once
// opened, so it never blocks the board (the timeline is lazy per card).
export function SpecTimeline({ specId, limit = 20 }: SpecTimelineProps) {
  const [open, setOpen] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const { data, loading, error, reload } = useSpecActivity(open ? specId : null)

  const events = data?.events ?? []
  const shown = expanded ? events : events.slice(0, limit)
  const hidden = events.length - shown.length

  return (
    <div className={styles.timeline}>
      <button
        type="button"
        className={styles.toggle}
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        {open ? <ChevronDown size={13} strokeWidth={2} /> : <ChevronRight size={13} strokeWidth={2} />}
        Activity
      </button>

      {open && (
        <div className={styles.panel}>
          {loading && <p className={styles.muted}>loading activity…</p>}
          {error && (
            <p className={styles.error}>
              error loading activity: {error}{' '}
              <button type="button" className={styles.retry} onClick={reload}>
                retry
              </button>
            </p>
          )}
          {!loading && !error && events.length === 0 && (
            <p className={styles.muted}>no activity in this window</p>
          )}
          {!loading && !error && events.length > 0 && (
            <>
              <ol className={styles.list}>
                {shown.map((event, i) => (
                  <TimelineEntry key={`${event.ts}-${event.type}-${i}`} event={event} />
                ))}
              </ol>
              {hidden > 0 && (
                <button type="button" className={styles.more} onClick={() => setExpanded(true)}>
                  show more ({hidden})
                </button>
              )}
            </>
          )}
        </div>
      )}
    </div>
  )
}
