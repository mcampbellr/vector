import { useMemo, useState } from 'react'
import { Layers, Plus } from 'lucide-react'
import type { Board, Card } from '../../types/board'
import type { EpicFilter } from '../../lib/epicFilter'
import { EpicForm } from './EpicForm'
import { EpicSection } from './EpicSection'
import { groupCardsByEpic } from './helpers'
import styles from './EpicsView.module.css'

interface EpicsViewProps {
  board: Board
  onSelectCard: (card: Card) => void
  /** Applies a board filter and switches to the kanban. */
  onShowOnBoard: (filter: EpicFilter) => void
}

// EpicsView is the epics tab: every epic with its progress, status breakdown and
// specs, plus the "new epic" form. It is a projection of the already-loaded
// board (same SSE stream, no fetch); its only writes are the epic form's create
// and edit. Clicking a spec opens the shared details drawer; "show on board"
// narrows the kanban to that epic.
export function EpicsView({ board, onSelectCard, onShowOnBoard }: EpicsViewProps) {
  const [creating, setCreating] = useState(false)
  const cardsByEpic = useMemo(() => groupCardsByEpic(board.columns), [board.columns])
  const unassigned = cardsByEpic.get('') ?? []

  return (
    <section className={styles.view} aria-label="Epics">
      <header className={styles.header}>
        <h2 className={styles.title}>
          <Layers size={18} strokeWidth={2} />
          Epics
        </h2>
        {!creating && (
          <button type="button" className={styles.primaryButton} onClick={() => setCreating(true)}>
            <Plus size={13} strokeWidth={2} />
            new epic
          </button>
        )}
      </header>

      {creating && (
        <div className={styles.newEpic}>
          <EpicForm onDone={() => setCreating(false)} onCancel={() => setCreating(false)} />
        </div>
      )}

      {board.epics.length === 0 && !creating && (
        <div className={styles.state}>
          <p className={styles.empty}>No epics yet</p>
          <p className={styles.hint}>
            An epic groups related specs (e.g. “App Mobile”). Create one with “new epic”, or from
            the terminal:
          </p>
          <code className={styles.command}>vector epic create --title "App Mobile"</code>
          <p className={styles.hint}>
            Then assign specs from their details drawer, or with{' '}
            <code className={styles.inlineCode}>vector spec epic --epic app-mobile &lt;spec-id&gt;…</code>.
          </p>
        </div>
      )}

      <div className={styles.epicList}>
        {board.epics.map((epic) => (
          <EpicSection
            key={epic.id}
            epic={epic}
            cards={cardsByEpic.get(epic.id) ?? []}
            onSelectCard={onSelectCard}
            onShowOnBoard={(epicId) => onShowOnBoard({ kind: 'epic', epicId })}
          />
        ))}
      </div>

      {board.epics.length > 0 && unassigned.length > 0 && (
        <p className={styles.unassigned}>
          {unassigned.length} {unassigned.length === 1 ? 'spec has' : 'specs have'} no epic.{' '}
          <button
            type="button"
            className={styles.linkButton}
            onClick={() => onShowOnBoard({ kind: 'none' })}
          >
            show them on the board
          </button>
        </p>
      )}
    </section>
  )
}
