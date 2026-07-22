import { useEffect, useState } from 'react'
import { useBoard } from './api/useBoard'
import type { Card } from './types/board'
import { BoardHeader } from './components/BoardHeader/BoardHeader'
import { KanbanBoard } from './components/KanbanBoard/KanbanBoard'
import { StandupView } from './components/StandupView'
import { TokenBreakdownView } from './components/TokenBreakdownView'
import { CommandPalette } from './components/CommandPalette'
import { SpecDetailsDrawer } from './components/SpecDetailsDrawer'
import { useCommandPaletteTrigger } from './lib/useCommandPaletteTrigger'
import styles from './App.module.css'

type View = 'board' | 'standup' | 'tokens'

export function App() {
  const { board, connection, error } = useBoard()
  const [view, setView] = useState<View>('board')
  // Selection and the palette live here — the only common ancestor of the
  // header, the three views, the palette and the drawer — so jump-to-spec
  // works identically from board, standup and tokens.
  const [selectedCard, setSelectedCard] = useState<Card | null>(null)
  const { isOpen: paletteOpen, open: openPalette, close: closePalette } = useCommandPaletteTrigger()

  // Reflect the active project in the browser tab so it's identifiable when
  // several boards are open at once. `board.repo` is the repo directory name.
  useEffect(() => {
    document.title = board ? `${board.repo} · Vector board` : 'Vector board'
  }, [board])

  if (!board) {
    return (
      <div className={styles.app}>
        <div className={styles.placeholder}>
          {error ? `Failed to load board: ${error}` : 'Loading board…'}
        </div>
      </div>
    )
  }

  const cards = board.columns.flatMap((column) => column.cards)

  return (
    <div className={styles.app}>
      <BoardHeader
        repo={board.repo}
        specCount={board.totals.specs}
        updatedAt={board.updatedAt}
        connection={connection}
        onOpenPalette={openPalette}
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
        <button
          type="button"
          className={`${styles.tab} ${view === 'tokens' ? styles.tabActive : ''}`}
          onClick={() => setView('tokens')}
        >
          Tokens
        </button>
      </nav>
      {view === 'board' && (
        <div className={styles.content}>
          <KanbanBoard columns={board.columns} onSelectCard={setSelectedCard} />
        </div>
      )}
      {view === 'standup' && (
        <div className={styles.content}>
          <StandupView />
        </div>
      )}
      {view === 'tokens' && (
        <div className={styles.content}>
          <TokenBreakdownView board={board} />
        </div>
      )}
      {paletteOpen && (
        <CommandPalette cards={cards} onSelectCard={setSelectedCard} onClose={closePalette} />
      )}
      {selectedCard && (
        <SpecDetailsDrawer card={selectedCard} onClose={() => setSelectedCard(null)} />
      )}
    </div>
  )
}
