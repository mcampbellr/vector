import { useState } from 'react'
import type { Card, Column } from '../../types/board'
import { BoardColumn } from '../BoardColumn/BoardColumn'
import { SpecDetailsDrawer } from '../SpecDetailsDrawer'
import styles from './KanbanBoard.module.css'

interface KanbanBoardProps {
  columns: Column[]
}

// KanbanBoard owns the selected-card UI state for the details drawer. Selection
// is local UI state, not domain state — the board stays a read-only projection
// (architecture/state-model.md). One drawer is rendered at the board level for
// the currently selected card.
export function KanbanBoard({ columns }: KanbanBoardProps) {
  const [selectedCard, setSelectedCard] = useState<Card | null>(null)

  return (
    <div className={styles.board}>
      {columns.map((column) => (
        <BoardColumn key={column.status} column={column} onSelectCard={setSelectedCard} />
      ))}
      {selectedCard && (
        <SpecDetailsDrawer card={selectedCard} onClose={() => setSelectedCard(null)} />
      )}
    </div>
  )
}
