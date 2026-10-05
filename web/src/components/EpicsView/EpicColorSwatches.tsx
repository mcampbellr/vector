import type { CSSProperties } from 'react'
import { EPIC_COLORS } from '../../types/board'
import type { EpicColor } from '../../types/board'
import styles from './EpicsView.module.css'

interface EpicColorSwatchesProps {
  /** Radio-group name, unique per form instance. */
  name: string
  value: EpicColor | ''
  onChange: (color: EpicColor | '') => void
  disabled: boolean
}

// EpicColorSwatches picks the epic's chip colour from the fixed palette (the
// same tokens the CLI validates), plus "none" for the neutral chip. A radio
// group, so arrow keys move between swatches.
export function EpicColorSwatches({ name, value, onChange, disabled }: EpicColorSwatchesProps) {
  return (
    <div className={styles.swatches} role="radiogroup" aria-label="Color">
      <label className={styles.swatch} title="none">
        <input
          type="radio"
          name={name}
          value=""
          checked={value === ''}
          disabled={disabled}
          onChange={() => onChange('')}
        />
        <span className={`${styles.swatchDot} ${styles.swatchNone}`} aria-hidden="true" />
        <span className={styles.visuallyHidden}>none</span>
      </label>
      {EPIC_COLORS.map((color) => (
        <label key={color} className={styles.swatch} title={color}>
          <input
            type="radio"
            name={name}
            value={color}
            checked={value === color}
            disabled={disabled}
            onChange={() => onChange(color)}
          />
          <span
            className={styles.swatchDot}
            style={{ '--swatch-color': `var(--epic-${color})` } as CSSProperties}
            aria-hidden="true"
          />
          <span className={styles.visuallyHidden}>{color}</span>
        </label>
      ))}
    </div>
  )
}
