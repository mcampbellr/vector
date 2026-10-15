import type { Artifacts } from '../../types/board'
import styles from './SpecCard.module.css'

interface CardArtifactMeterProps {
  artifacts?: Artifacts
  /** True when the age renders to its left and owns the row's `margin-left: auto`. */
  afterAge?: boolean
}

// CardArtifactMeter renders proposal/design/tasks as three 13×3px segments
// instead of three uppercase labels: same datum, 48px, lit = done. It is always
// present — a card with no artifacts shows three unlit segments, because hiding
// the meter made the absence invisible and broke the column's vertical read.
export function CardArtifactMeter({ artifacts, afterAge = false }: CardArtifactMeterProps) {
  const segments: ReadonlyArray<[label: string, on: boolean]> = [
    ['proposal', artifacts?.proposal ?? false],
    ['design', artifacts?.design ?? false],
    ['tasks', artifacts?.tasks ?? false],
  ]

  const present = segments.filter(([, on]) => on).map(([label]) => label)
  const missing = segments.filter(([, on]) => !on).map(([label]) => label)
  const tooltip =
    present.length === 0
      ? 'no artifacts yet'
      : missing.length === 0
        ? present.join(' · ')
        : `${present.join(' · ')} — no ${missing.join(', ')}`

  return (
    <span
      className={`${styles.meter}${afterAge ? ` ${styles.meterAfterAge}` : ''}`}
      title={tooltip}
      aria-label={`Artifacts: ${tooltip}`}
    >
      {segments.map(([label, on]) => (
        <span key={label} className={`${styles.segment}${on ? ` ${styles.segmentOn}` : ''}`} />
      ))}
    </span>
  )
}
