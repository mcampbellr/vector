import { Search } from 'lucide-react'
import styles from './BoardHeader.module.css'

interface PaletteTriggerProps {
  onOpen: () => void
}

// PaletteTrigger is the header control that opens the command palette: the magnifier
// plus the keycap that names the shortcut, so the trigger teaches the key instead of
// hiding it in a tooltip. It owns no open/close logic — that lives in App via
// useCommandPaletteTrigger.
export function PaletteTrigger({ onOpen }: PaletteTriggerProps) {
  return (
    <button
      type="button"
      className={styles.paletteTrigger}
      onClick={onOpen}
      aria-label="Open command palette"
      title="Search specs (/)"
    >
      <Search size={13} strokeWidth={2} />
      <span className={styles.keycap}>/</span>
    </button>
  )
}
