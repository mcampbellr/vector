import type { Priority } from '../../types/board'
import styles from './CommandPalette.module.css'

const PRIORITIES: Priority[] = ['urgent', 'high', 'normal', 'low']

interface PalettePriorityFilterProps {
  selected: Priority[]
  onToggle: (priority: Priority) => void
}

// PalettePriorityFilter is the palette's multi-select refinement: 0–4 active
// priority chips, where zero active means no priority filter. Priority is the
// only filter axis — status is deliberately excluded (board columns already
// are the status axis).
export function PalettePriorityFilter({ selected, onToggle }: PalettePriorityFilterProps) {
  return (
    <div className={styles.priorityFilter}>
      {PRIORITIES.map((priority) => (
        <button
          key={priority}
          type="button"
          className={selected.includes(priority) ? styles.chipActive : styles.chip}
          aria-pressed={selected.includes(priority)}
          onClick={() => onToggle(priority)}
        >
          {priority}
        </button>
      ))}
    </div>
  )
}
