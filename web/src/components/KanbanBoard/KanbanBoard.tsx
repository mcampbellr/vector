import type { Column } from '../../types/board'
import { BoardColumn } from '../BoardColumn/BoardColumn'
import styles from './KanbanBoard.module.css'

interface KanbanBoardProps {
  columns: Column[]
}

export function KanbanBoard({ columns }: KanbanBoardProps) {
  return (
    <div className={styles.board}>
      {columns.map((column) => (
        <BoardColumn key={column.status} column={column} />
      ))}
    </div>
  )
}
