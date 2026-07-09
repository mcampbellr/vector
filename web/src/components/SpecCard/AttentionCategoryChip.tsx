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

interface AttentionCategoryChipProps {
  category?: string
}

// Small pill classifying why a spec is blocked (the needs-attention category).
// Renders nothing for an absent or unknown category, so a legacy card that only
// carries attentionReason shows no chip. Mirrors ArtifactDot's shape.
export function AttentionCategoryChip({ category }: AttentionCategoryChipProps) {
  if (!category || !(CATEGORIES as readonly string[]).includes(category)) return null
  const known = category as AttentionCategory
  return <span className={`${styles.categoryChip} ${CLASSES[known]}`}>{LABELS[known]}</span>
}
