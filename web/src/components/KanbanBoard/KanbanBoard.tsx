import type { Card, Column } from '../../types/board'
import { BoardColumn } from '../BoardColumn/BoardColumn'
import styles from './KanbanBoard.module.css'

interface KanbanBoardProps {
  columns: Column[]
  onSelectCard: (card: Card) => void
}

// KanbanBoard is a pure read-only projection of the board columns
// (architecture/state-model.md). Selection state and the details drawer live
// in App — elevated so the command palette and the standup/tokens views can
// open a spec too; the board only delegates clicks through onSelectCard.
export function KanbanBoard({ columns, onSelectCard }: KanbanBoardProps) {
  return (
    <div className={styles.board}>
      {columns.map((column) => (
        <BoardColumn key={column.status} column={column} onSelectCard={onSelectCard} />
      ))}
    </div>
  )
}
