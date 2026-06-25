import { Layers } from 'lucide-react'
import type { ConnectionState } from '../../api/useBoard'
import { relativeTime } from '../../lib/format'
import { useNow } from '../../lib/useNow'
import styles from './BoardHeader.module.css'

interface BoardHeaderProps {
  repo: string
  specCount: number
  updatedAt: string
  connection: ConnectionState
}

const CONNECTION_LABEL: Record<ConnectionState, string> = {
  loading: 'connecting…',
  live: 'live',
  reconnecting: 'reconnecting…',
  error: 'offline',
}

export function BoardHeader({ repo, specCount, updatedAt, connection }: BoardHeaderProps) {
  const now = useNow()
  const freshness = updatedAt ? relativeTime(updatedAt, now) : null

  return (
    <header className={styles.header}>
      <div className={styles.brand}>
        <span className={styles.logo}>
          <Layers size={18} strokeWidth={2.2} />
        </span>
        <div>
          <h1 className={styles.title}>{repo}</h1>
          <p className={styles.subtitle}>
            {specCount} {specCount === 1 ? 'spec' : 'specs'}
            {freshness ? ` · updated ${freshness}` : ''}
          </p>
        </div>
      </div>
      <span className={`${styles.status} ${styles[connection]}`}>
        <span className={styles.dot} />
        {CONNECTION_LABEL[connection]}
      </span>
    </header>
  )
}
