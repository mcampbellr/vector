import { Tag } from 'lucide-react'
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

// StandupSpecRow renders one work item as a self-standing block: the identifier
// leads (ticket key as a link when linked, else the slug), the status shows as a
// pill plus a left accent bar keyed off data-status, and the summary is the
// engineering-standup paragraph. One work item, one block — never grouped.
export function StandupSpecRow({ spec }: StandupSpecRowProps) {
  const ticket = spec.ticket?.key ? spec.ticket : undefined
  return (
    <article className={styles.specRow} data-status={spec.status}>
      <header className={styles.specHead}>
        <div className={styles.specHeading}>
          <div className={styles.specIdRow}>
            {ticket ? (
              <>
                <a
                  className={styles.ticket}
                  href={ticket.url || undefined}
                  target="_blank"
                  rel="noreferrer"
                  title={ticket.url || ticket.key}
                >
                  <Tag size={11} strokeWidth={2.25} aria-hidden />
                  {ticket.key}
                </a>
                <span className={styles.specId}>{spec.id}</span>
              </>
            ) : (
              <span className={styles.specIdLead}>{spec.id}</span>
            )}
          </div>
          {spec.title && spec.title !== spec.id && <h3 className={styles.specTitle}>{spec.title}</h3>}
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
