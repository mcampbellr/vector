import styles from './SpecCard.module.css'

const CATEGORIES = ['dependency', 'env', 'decision', 'external', 'other'] as const
type AttentionCategory = (typeof CATEGORIES)[number]

const LABELS: Record<AttentionCategory, string> = {
  dependency: 'Dependency',
  env: 'Env',
  decision: 'Decision',
  external: 'External',
  other: 'Other',
}

const CLASSES: Record<AttentionCategory, string> = {
  dependency: styles.catDependency,
  env: styles.catEnv,
  decision: styles.catDecision,
  external: styles.catExternal,
  other: styles.catOther,
}

/** Whether a category string is one the board knows how to label. Exported so a
 *  caller can tell "structured" from "unlabelled" with the same rule the label
 *  itself applies — a newer CLI could emit a category this build does not know. */
export function isKnownAttentionCategory(category?: string): boolean {
  return Boolean(category) && (CATEGORIES as readonly string[]).includes(category as string)
}

interface AttentionCategoryLabelProps {
  category?: string
}

// AttentionCategoryLabel names why a spec is blocked. It is a coloured mono
// label, not a chip: the chip background added no information the coloured text
// doesn't carry, and five filled backgrounds per column brought the confetti
// back. Renders nothing for an absent or unknown category, so a legacy card that
// carries only attentionReason shows no label.
export function AttentionCategoryLabel({ category }: AttentionCategoryLabelProps) {
  if (!isKnownAttentionCategory(category)) return null
  const known = category as AttentionCategory
  return (
    <span className={`${styles.attentionCategory} ${CLASSES[known]}`}>{LABELS[known]}</span>
  )
}
