import type { ModelRollup } from '../../types/board'
import { formatCompact } from '../../lib/format'
import styles from './TokenBreakdownView.module.css'

interface SpecModelBreakdownProps {
  models: ModelRollup[]
}

// SpecModelBreakdown renders a per-model token rollup as a secondary, muted list:
// one entry per `model→baseline` pair with its routed-token volume and route
// count. Reused for both the global aggregate breakdown and each spec's own.
// Tokens only — no dollars.
export function SpecModelBreakdown({ models }: SpecModelBreakdownProps) {
  if (models.length === 0) return null

  return (
    <ul className={styles.modelList}>
      {models.map((model) => (
        <li key={`${model.model}-${model.baseline}`} className={styles.modelItem}>
          <span className={styles.modelPair}>
            {model.model} <span className={styles.modelArrow}>→</span> {model.baseline}
          </span>
          <span className={styles.modelTokens}>
            {formatCompact(model.tokensIn + model.tokensOut)} tok · {model.routes}×
          </span>
        </li>
      ))}
    </ul>
  )
}
