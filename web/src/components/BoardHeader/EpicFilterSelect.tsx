import type { ChangeEvent } from 'react'
import { Layers } from 'lucide-react'
import type { EpicSummary } from '../../types/board'
import type { EpicFilter } from '../../lib/epicFilter'
import styles from './BoardHeader.module.css'

interface EpicFilterSelectProps {
  epics: EpicSummary[]
  filter: EpicFilter
  onChange: (filter: EpicFilter) => void
}

const ALL_VALUE = '__all__'
const NONE_VALUE = '__none__'

function toValue(filter: EpicFilter): string {
  switch (filter.kind) {
    case 'all':
      return ALL_VALUE
    case 'none':
      return NONE_VALUE
    case 'epic':
      return `epic:${filter.epicId}`
  }
}

function fromValue(value: string): EpicFilter {
  if (value === ALL_VALUE) return { kind: 'all' }
  if (value === NONE_VALUE) return { kind: 'none' }
  return { kind: 'epic', epicId: value.slice('epic:'.length) }
}

// EpicFilterSelect narrows the kanban to one epic (or to specs with no epic).
// Epics are listed in the board's display order (epic order, then title); a
// focused epic is labelled so.
// A native <select>: keyboard, screen-reader and touch behaviour come for free,
// and it fits the 56px header. The active filter is tinted so a narrowed board
// never passes for the whole board.
export function EpicFilterSelect({ epics, filter, onChange }: EpicFilterSelectProps) {
  function handleChange(event: ChangeEvent<HTMLSelectElement>) {
    onChange(fromValue(event.target.value))
  }

  const active = filter.kind !== 'all'

  return (
    <label className={`${styles.epicFilter}${active ? ` ${styles.epicFilterActive}` : ''}`}>
      <Layers size={13} strokeWidth={2} aria-hidden="true" />
      <select
        className={styles.epicFilterSelect}
        aria-label="Filter the board by epic"
        value={toValue(filter)}
        onChange={handleChange}
      >
        <option value={ALL_VALUE}>all epics</option>
        <option value={NONE_VALUE}>no epic</option>
        {epics.map((epic) => (
          <option key={epic.id} value={`epic:${epic.id}`}>
            {epic.focus ? `${epic.title} (focused)` : epic.title}
          </option>
        ))}
      </select>
    </label>
  )
}
