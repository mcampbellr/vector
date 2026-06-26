import type { StandupSpecDigest } from '../../types/standup'
import type { Status } from '../../types/board'
import { StatusPill } from '../StatusPill/StatusPill'
import { SpecTimeline } from '../SpecTimeline'
import styles from './StandupView.module.css'

interface StandupSpecRowProps {
  spec: StandupSpecDigest
}

const KNOWN_STATUSES: Status[] = ['draft', 'open', 'in-progress', 'needs-attention', 'review', 'closed']

function isStatus(value: string): value is Status {
  return (KNOWN_STATUSES as string[]).includes(value)
}

export function StandupSpecRow({ spec }: StandupSpecRowProps) {
  return (
    <article className={styles.specRow}>
      <header className={styles.specHead}>
        <div className={styles.specHeading}>
          <h3 className={styles.specTitle}>{spec.title || spec.id}</h3>
          <span className={styles.specId}>{spec.id}</span>
        </div>
        <div className={styles.specMeta}>
          {isStatus(spec.status) && <StatusPill status={spec.status} />}
          <span className={styles.changeCount}>
            {spec.changeCount} {spec.changeCount === 1 ? 'change' : 'changes'}
          </span>
        </div>
      </header>
      {spec.summary && <p className={styles.specSummary}>{spec.summary}</p>}
      <SpecTimeline specId={spec.id} />
    </article>
  )
}
