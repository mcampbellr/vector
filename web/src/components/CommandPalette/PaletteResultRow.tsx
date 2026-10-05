import type { Card } from '../../types/board'
import { StatusPill } from '../StatusPill/StatusPill'
import { PriorityFlag } from '../PriorityFlag/PriorityFlag'
import styles from './CommandPalette.module.css'

interface PaletteResultRowProps {
  id: string
  card: Card
  highlighted: boolean
  onSelect: (card: Card) => void
}

// PaletteResultRow is one option in the palette's listbox: title, plain-text
// slug, status pill and priority flag. Purely presentational — highlight and
// selection are owned by CommandPalette (focus never leaves the search input,
// per the combobox pattern).
export function PaletteResultRow({ id, card, highlighted, onSelect }: PaletteResultRowProps) {
  return (
    <li
      id={id}
      role="option"
      aria-selected={highlighted}
      className={highlighted ? styles.rowHighlighted : styles.row}
      onClick={() => onSelect(card)}
    >
      <span className={styles.rowTitle}>{card.title}</span>
      <span className={styles.rowSlug}>{card.id}</span>
      <StatusPill status={card.status} />
      <PriorityFlag priority={card.priority} />
    </li>
  )
}
