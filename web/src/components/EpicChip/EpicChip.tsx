import type { CSSProperties } from 'react'
import type { EpicSummary } from '../../types/board'
import styles from './EpicChip.module.css'

interface EpicChipProps {
  /** The card's epic id — shown as-is when the epic is not (or no longer) on the
   *  board, so a dangling reference is still visible. */
  epicId: string
  epic?: EpicSummary
}

// EpicChip names the epic a spec belongs to: a colour dot plus the epic title,
// tinted with the epic's palette token (`--epic-<color>`, light/dark aware).
// An epic without a colour falls back to the neutral text ramp.
export function EpicChip({ epicId, epic }: EpicChipProps) {
  const title = epic?.title ?? epicId
  const style = {
    '--epic-chip-color': epic?.color ? `var(--epic-${epic.color})` : 'var(--color-text-secondary)',
  } as CSSProperties

  return (
    <span className={styles.chip} style={style} title={`Epic: ${title}`} data-epic={epicId}>
      <span className={styles.dot} aria-hidden="true" />
      <span className={styles.label}>{title}</span>
    </span>
  )
}
