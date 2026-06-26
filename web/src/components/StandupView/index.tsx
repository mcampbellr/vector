import { CalendarClock } from 'lucide-react'
import { useStandup } from '../../api/useStandup'
import { relativeTime } from '../../lib/format'
import { useNow } from '../../lib/useNow'
import { StandupSpecRow } from './StandupSpecRow'
import styles from './StandupView.module.css'

// StandupView is the dedicated board view for the persisted standup digest: the
// global ceremony paragraph plus a per-spec breakdown with each card's activity
// timeline. It is a projection of GET /api/standup (the digest is generated and
// persisted by /vector:standup; the board only reads it).
export function StandupView() {
  const { data, loading, error, reload } = useStandup()
  const now = useNow(30000)

  if (loading) {
    return <div className={styles.state}>loading standup…</div>
  }

  if (error) {
    return (
      <div className={styles.state}>
        <p className={styles.error}>error loading standup: {error}</p>
        <button type="button" className={styles.retry} onClick={reload}>
          retry
        </button>
      </div>
    )
  }

  const hasContent = Boolean(data?.global) || (data?.perSpec?.length ?? 0) > 0
  if (!data || !hasContent) {
    return (
      <div className={styles.state}>
        <p className={styles.empty}>no activity since last standup</p>
        <p className={styles.hint}>Run /vector:standup to generate a digest.</p>
      </div>
    )
  }

  return (
    <section className={styles.view}>
      <header className={styles.header}>
        <h2 className={styles.title}>Standup</h2>
        {data.since && (
          <span className={styles.period}>
            <CalendarClock size={13} strokeWidth={2} />
            since {relativeTime(data.since, now)}
          </span>
        )}
      </header>

      {data.global && <p className={styles.global}>{data.global}</p>}

      {data.totals && (
        <p className={styles.totals}>
          {data.totals.specs} {data.totals.specs === 1 ? 'spec' : 'specs'} · {data.totals.changes}{' '}
          {data.totals.changes === 1 ? 'change' : 'changes'}
        </p>
      )}

      <div className={styles.specs}>
        {(data.perSpec ?? []).map((spec) => (
          <StandupSpecRow key={spec.id} spec={spec} />
        ))}
      </div>
    </section>
  )
}
