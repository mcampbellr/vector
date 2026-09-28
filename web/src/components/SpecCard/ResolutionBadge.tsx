import { Ban } from 'lucide-react'
import type { Resolution } from '../../types/board'
import { RESOLUTION_LABELS, isDroppedResolution } from '../../lib/resolution'
import styles from './SpecCard.module.css'

interface ResolutionBadgeProps {
  resolution?: Resolution
  note?: string
}

// ResolutionBadge marks a card closed without delivering the work (obsolete,
// duplicate, superseded) — those do not count toward epic progress. A `done` or
// absent resolution renders nothing: that is the normal closed card.
export function ResolutionBadge({ resolution, note }: ResolutionBadgeProps) {
  if (!isDroppedResolution(resolution)) return null
  const label = RESOLUTION_LABELS[resolution]
  const description = `Closed as ${label}${note ? ` — ${note}` : ''} (not counted as done)`
  return (
    <span className={`${styles.glyph} ${styles.resolution}`} title={description} aria-label={description}>
      <Ban size={11} strokeWidth={2} />
      {label}
    </span>
  )
}
