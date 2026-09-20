import type { ConnectionState } from '../../api/useBoard'
import { compactRelativeTime } from '../../lib/format'
import { useNow } from '../../lib/useNow'
import { BoardTabs } from './BoardTabs'
import type { BoardView } from './BoardView'
import { PaletteTrigger } from './PaletteTrigger'
import { ThemeControl } from './ThemeControl'
import styles from './BoardHeader.module.css'

interface BoardHeaderProps {
  repo: string
  specCount: number
  updatedAt: string
  connection: ConnectionState
  view: BoardView
  onChangeView: (view: BoardView) => void
  onOpenPalette: () => void
}

const CONNECTION_LABEL: Record<ConnectionState, string> = {
  loading: 'connecting…',
  live: 'live',
  reconnecting: 'reconnecting…',
  error: 'offline',
}

// BoardHeader is the single 56px status line of the panel: identity on the left,
// the view tabs in the middle, connection and controls on the right. The
// gradient logo tile is gone — the product name is mono, the repo is the only
// thing set in sans, and freshness is one compact unit (`9h`, not `updated 9 hr
// ago`).
export function BoardHeader({
  repo,
  specCount,
  updatedAt,
  connection,
  view,
  onChangeView,
  onOpenPalette,
}: BoardHeaderProps) {
  const now = useNow()
  const freshness = updatedAt ? compactRelativeTime(updatedAt, now) : ''

  return (
    <header className={styles.header}>
      <div className={styles.brand}>
        <span className={styles.product}>vector</span>
        <span className={styles.separator}>/</span>
        <span className={styles.repo}>{repo}</span>
        <span className={styles.freshness}>
          {specCount} {specCount === 1 ? 'spec' : 'specs'}
          {freshness ? ` · ${freshness}` : ''}
        </span>
      </div>
      <BoardTabs view={view} onChange={onChangeView} />
      <div className={styles.actions}>
        {/* The dot always carries the colour; the label is tinted only when the
            user has to act or distrust what they see. */}
        <span className={`${styles.connection} ${styles[connection]}`}>
          <span className={styles.dot} />
          {CONNECTION_LABEL[connection]}
        </span>
        <PaletteTrigger onOpen={onOpenPalette} />
        <ThemeControl />
      </div>
    </header>
  )
}
