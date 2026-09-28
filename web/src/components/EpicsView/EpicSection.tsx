import { useState } from 'react'
import { Filter, Pencil } from 'lucide-react'
import type { Card, EpicSummary } from '../../types/board'
import { EpicChip } from '../EpicChip/EpicChip'
import { EpicForm } from './EpicForm'
import { EpicSpecRow } from './EpicSpecRow'
import { EpicStatusBreakdown } from './EpicStatusBreakdown'
import { progressPercent } from './helpers'
import styles from './EpicsView.module.css'

interface EpicSectionProps {
  epic: EpicSummary
  /** The epic's cards on the board (archived specs are counted in the epic's
   *  totals but have no card). */
  cards: Card[]
  onSelectCard: (card: Card) => void
  onShowOnBoard: (epicId: string) => void
}

// EpicSection is one epic in the epics view: identity, progress (done of total,
// done = closed + archived), status breakdown and its specs. "show on board"
// narrows the kanban to this epic; "edit" swaps the header for the edit form.
export function EpicSection({ epic, cards, onSelectCard, onShowOnBoard }: EpicSectionProps) {
  const [editing, setEditing] = useState(false)
  const percent = progressPercent(epic.done, epic.total)
  const archived = epic.byStatus.archived ?? 0

  return (
    <section className={styles.epic} aria-label={`Epic ${epic.title}`}>
      {editing ? (
        <EpicForm epic={epic} onDone={() => setEditing(false)} onCancel={() => setEditing(false)} />
      ) : (
        <header className={styles.epicHeader}>
          <div className={styles.epicIdentity}>
            <EpicChip epicId={epic.id} epic={epic} />
            <span className={styles.epicId}>{epic.id}</span>
          </div>
          <div className={styles.epicActions}>
            <button
              type="button"
              className={styles.secondaryButton}
              onClick={() => onShowOnBoard(epic.id)}
              title="Filter the board to this epic"
            >
              <Filter size={12} strokeWidth={2} />
              show on board
            </button>
            <button
              type="button"
              className={styles.iconButton}
              onClick={() => setEditing(true)}
              aria-label={`Edit epic ${epic.title}`}
              title="Edit epic"
            >
              <Pencil size={12} strokeWidth={2} />
            </button>
          </div>
        </header>
      )}

      {!editing && epic.description && <p className={styles.epicDescription}>{epic.description}</p>}

      <div className={styles.progress}>
        <div
          className={styles.progressTrack}
          role="progressbar"
          aria-label={`${epic.title} progress`}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={percent}
        >
          <div className={styles.progressFill} style={{ width: `${percent}%` }} />
        </div>
        <span className={styles.progressText}>
          {epic.done} of {epic.total} done
        </span>
      </div>

      <EpicStatusBreakdown byStatus={epic.byStatus} />

      {cards.length > 0 ? (
        <ul className={styles.specList}>
          {cards.map((card) => (
            <EpicSpecRow key={card.id} card={card} onSelect={onSelectCard} />
          ))}
        </ul>
      ) : (
        <p className={styles.muted}>
          {epic.total === 0
            ? `No specs yet — assign one from its details drawer, or run vector spec epic <spec-id> ${epic.id}`
            : 'Every spec of this epic is archived.'}
        </p>
      )}
      {cards.length > 0 && archived > 0 && (
        <p className={styles.muted}>
          + {archived} archived {archived === 1 ? 'spec' : 'specs'}
        </p>
      )}
    </section>
  )
}
