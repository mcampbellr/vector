import { Pin } from 'lucide-react'
import type { Card } from '../../types/board'
import { StatusPill } from '../StatusPill/StatusPill'
import styles from './EpicsView.module.css'

interface EpicSpecRowProps {
  card: Card
  onSelect: (card: Card) => void
}

// EpicSpecRow is one spec inside an epic: a button that opens the same details
// drawer as a board card.
export function EpicSpecRow({ card, onSelect }: EpicSpecRowProps) {
  return (
    <li>
      <button
        type="button"
        className={styles.specRow}
        aria-label={`Open details for ${card.title}`}
        onClick={() => onSelect(card)}
      >
        <StatusPill status={card.status} />
        <span className={styles.specTitle}>{card.title || card.id}</span>
        {card.focus && (
          <span className={styles.specFocus} title="Focused" aria-label="Focused">
            <Pin size={11} strokeWidth={2} fill="currentColor" />
          </span>
        )}
        <span className={styles.specId}>{card.id}</span>
      </button>
    </li>
  )
}
