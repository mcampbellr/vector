import { useState } from 'react'
import { useBoard } from './api/useBoard'
import { BoardHeader } from './components/BoardHeader/BoardHeader'
import { KanbanBoard } from './components/KanbanBoard/KanbanBoard'
import { StandupView } from './components/StandupView'
import { TokenSavingsMeter } from './components/TokenSavingsMeter/TokenSavingsMeter'
import styles from './App.module.css'

type View = 'board' | 'standup'

export function App() {
  const { board, connection, error } = useBoard()
  const [view, setView] = useState<View>('board')

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
      <nav className={styles.tabs}>
        <button
          type="button"
          className={`${styles.tab} ${view === 'board' ? styles.tabActive : ''}`}
          onClick={() => setView('board')}
        >
          Board
        </button>
        <button
          type="button"
          className={`${styles.tab} ${view === 'standup' ? styles.tabActive : ''}`}
          onClick={() => setView('standup')}
        >
          Standup
        </button>
      </nav>
      {view === 'board' ? (
        <div className={styles.content}>
          <aside className={styles.rail}>
            <TokenSavingsMeter savings={board.tokenSavings} />
          </aside>
          <KanbanBoard columns={board.columns} />
        </div>
      ) : (
        <div className={styles.content}>
          <StandupView />
        </div>
      )}
    </div>
  )
}
