import { Search } from 'lucide-react'
import styles from './BoardHeader.module.css'

interface PaletteTriggerProps {
  onOpen: () => void
}

// PaletteTrigger is the header button that opens the command palette. It owns
// no open/close logic — that lives in App via useCommandPaletteTrigger.
export function PaletteTrigger({ onOpen }: PaletteTriggerProps) {
  return (
    <button
      type="button"
      className={styles.paletteTrigger}
      onClick={onOpen}
      aria-label="Open command palette"
      title="Search specs (/)"
    >
      <Search size={16} strokeWidth={2} />
    </button>
  )
}
