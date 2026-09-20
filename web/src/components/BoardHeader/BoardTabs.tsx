import type { BoardView } from './BoardView'
import { BOARD_VIEWS } from './BoardView'
import styles from './BoardHeader.module.css'

interface BoardTabsProps {
  view: BoardView
  onChange: (view: BoardView) => void
}

// BoardTabs is the view switch, living inside the 56px header rather than on a
// row of its own: three mono labels, the active one on a raised surface. The
// labels are the view ids verbatim — the board speaks lowercase English.
export function BoardTabs({ view, onChange }: BoardTabsProps) {
  return (
    <nav className={styles.tabs} aria-label="Views">
      {BOARD_VIEWS.map((candidate) => (
        <button
          key={candidate}
          type="button"
          className={`${styles.tab}${candidate === view ? ` ${styles.tabActive}` : ''}`}
          aria-current={candidate === view ? true : undefined}
          onClick={() => onChange(candidate)}
        >
          {candidate}
        </button>
      ))}
    </nav>
  )
}
