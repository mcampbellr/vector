import { useMemo } from 'react'
import type { Card, Column, EpicSummary } from '../../types/board'
import { BoardColumn } from '../BoardColumn/BoardColumn'
import styles from './KanbanBoard.module.css'

interface KanbanBoardProps {
  columns: Column[]
  epics: EpicSummary[]
  onSelectCard: (card: Card) => void
}

// KanbanBoard is a projection of the board columns (architecture/state-model.md);
// the only writes it can trigger are the per-card focus toggles, which go to the
// API and come back through the SSE board push. Selection state and the details drawer live
// in App — elevated so the command palette and the standup/tokens views can
// open a spec too; the board only delegates clicks through onSelectCard.
export function KanbanBoard({ columns, epics, onSelectCard }: KanbanBoardProps) {
  const epicsById = useMemo(() => new Map(epics.map((epic) => [epic.id, epic])), [epics])

  return (
    <div className={styles.board}>
      {columns.map((column) => (
        <BoardColumn
          key={column.status}
          column={column}
          epicsById={epicsById}
          onSelectCard={onSelectCard}
        />
      ))}
    </div>
  )
}
