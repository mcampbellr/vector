import { GitBranch } from 'lucide-react'
import type { RelatedItem } from '../../types/board'
import { relationChips } from './relationChips'
import styles from './SpecCard.module.css'

interface RelatedChipsProps {
  related: RelatedItem[]
}

// RelatedChips renders the cause→bug relations of a spec read-only (mirrors the
// ticket chip; no editing — all writes go through the binary). Each chip names the
// cause it points to; the aria-label spells out kind, ref and how it was found.
export function RelatedChips({ related }: RelatedChipsProps) {
  const chips = relationChips(related)
  if (chips.length === 0) return null
  return (
    <div className={styles.related}>
      {chips.map((chip) => (
        <span key={chip.key} className={styles.relation} aria-label={chip.ariaLabel} title={chip.title}>
          <GitBranch size={11} strokeWidth={2} />
          {chip.label}
        </span>
      ))}
    </div>
  )
}
