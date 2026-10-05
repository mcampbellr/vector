import { useEffect, useMemo, useState } from 'react'
import { useBoard } from './api/useBoard'
import type { Card } from './types/board'
import { BoardHeader } from './components/BoardHeader/BoardHeader'
import type { BoardView } from './components/BoardHeader/BoardView'
import { KanbanBoard } from './components/KanbanBoard/KanbanBoard'
import { EpicsView } from './components/EpicsView'
import { StandupView } from './components/StandupView'
import { TokenBreakdownView } from './components/TokenBreakdownView'
import { CommandPalette } from './components/CommandPalette'
import { SpecDetailsDrawer } from './components/SpecDetailsDrawer'
import { useCommandPaletteTrigger } from './lib/useCommandPaletteTrigger'
import { filterColumnsByEpic, resolveEpicFilter } from './lib/epicFilter'
import type { EpicFilter } from './lib/epicFilter'
import { useEpicFilter } from './lib/useEpicFilter'
import styles from './App.module.css'

export function App() {
  const { board, connection, error } = useBoard()
  const [view, setView] = useState<BoardView>('board')
  // Selection and the palette live here — the only common ancestor of the
  // header, the views, the palette and the drawer — so jump-to-spec works
  // identically from every view. The selection is a snapshot; the drawer renders
  // the live card of the same id (below), so a write made from the drawer shows
  // up there as soon as the SSE board push lands.
  const [selectedCard, setSelectedCard] = useState<Card | null>(null)
  const { isOpen: paletteOpen, open: openPalette, close: closePalette } = useCommandPaletteTrigger()
  const [epicFilter, setEpicFilter] = useEpicFilter()

  // Reflect the active project in the browser tab so it's identifiable when
  // several boards are open at once. `board.repo` is the repo directory name.
  useEffect(() => {
    document.title = board ? `${board.repo} · Vector board` : 'Vector board'
  }, [board])

  const epics = useMemo(() => board?.epics ?? [], [board])
  const effectiveFilter = resolveEpicFilter(epicFilter, epics)
  const filteredColumns = useMemo(
    () => (board ? filterColumnsByEpic(board.columns, effectiveFilter) : []),
    [board, effectiveFilter],
  )

  if (!board) {
    return (
      <div className={styles.app}>
        <div className={styles.placeholder}>
          {error ? `failed to load board — ${error}` : 'loading board…'}
        </div>
      </div>
    )
  }

  const cards = board.columns.flatMap((column) => column.cards)
  const liveSelectedCard = selectedCard
    ? (cards.find((card) => card.id === selectedCard.id) ?? selectedCard)
    : null

  function showOnBoard(filter: EpicFilter) {
    setEpicFilter(filter)
    setView('board')
  }

  return (
    <div className={styles.app}>
      <BoardHeader
        repo={board.repo}
        specCount={board.totals.specs}
        updatedAt={board.updatedAt}
        connection={connection}
        view={view}
        onChangeView={setView}
        onOpenPalette={openPalette}
        epics={epics}
        epicFilter={effectiveFilter}
        onChangeEpicFilter={setEpicFilter}
      />
      {view === 'board' && (
        <div className={styles.content}>
          <KanbanBoard columns={filteredColumns} epics={epics} onSelectCard={setSelectedCard} />
        </div>
      )}
      {view === 'epics' && (
        <div className={styles.content}>
          <EpicsView board={board} onSelectCard={setSelectedCard} onShowOnBoard={showOnBoard} />
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
      {liveSelectedCard && (
        <SpecDetailsDrawer
          card={liveSelectedCard}
          epics={epics}
          onClose={() => setSelectedCard(null)}
        />
      )}
    </div>
  )
}
