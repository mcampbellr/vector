import { useBoard } from './api/useBoard'
import { BoardHeader } from './components/BoardHeader/BoardHeader'
import { KanbanBoard } from './components/KanbanBoard/KanbanBoard'
import { TokenSavingsMeter } from './components/TokenSavingsMeter/TokenSavingsMeter'
import styles from './App.module.css'

export function App() {
  const { board, connection, error } = useBoard()

  if (!board) {
    return (
      <div className={styles.app}>
        <div className={styles.placeholder}>
          {error ? `Failed to load board: ${error}` : 'Loading board…'}
        </div>
      </div>
    )
  }

  return (
    <div className={styles.app}>
      <BoardHeader
        repo={board.repo}
        specCount={board.totals.specs}
        updatedAt={board.updatedAt}
        connection={connection}
      />
      <div className={styles.content}>
        <aside className={styles.rail}>
          <TokenSavingsMeter savings={board.tokenSavings} />
        </aside>
        <KanbanBoard columns={board.columns} />
      </div>
    </div>
  )
}
