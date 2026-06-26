import { Sparkles, TrendingDown } from 'lucide-react'
import type { TokenSavings } from '../../types/board'
import { formatCompact, formatUsd } from '../../lib/format'
import styles from './TokenSavingsMeter.module.css'

interface TokenSavingsMeterProps {
  savings: TokenSavings
}

// The commercialization wedge: how much the cheap-agent routing saved versus
// running everything on the baseline model. Rolled up from agent.routed events.
export function TokenSavingsMeter({ savings }: TokenSavingsMeterProps) {
  const reductionPct =
    savings.baselineUsd > 0 ? Math.round((savings.totalSavedUsd / savings.baselineUsd) * 100) : 0
  const spentPct = savings.baselineUsd > 0 ? (savings.totalSpentUsd / savings.baselineUsd) * 100 : 0

  if (savings.routes === 0) {
    return (
      <section className={styles.meter}>
        <div className={styles.heading}>
          <Sparkles size={16} strokeWidth={2} />
          Token Savings Meter
        </div>
        <p className={styles.emptyState}>
          No agent routing recorded yet. As Vector routes trivial work to cheaper agents, the
          savings show up here.
        </p>
      </section>
    )
  }

  return (
    <section className={styles.meter}>
      <div className={styles.heading}>
        <Sparkles size={16} strokeWidth={2} />
        Token Savings Meter
      </div>

      <div className={styles.headline}>
        <div className={styles.amount}>
          <span className={styles.saved}>{formatUsd(savings.totalSavedUsd)}</span>
          <span className={styles.savedLabel}>saved</span>
        </div>
        <div className={styles.reduction}>
          <TrendingDown size={14} strokeWidth={2.5} />
          {reductionPct}% cheaper
        </div>
      </div>

      <div className={styles.bar} title={`Spent ${formatUsd(savings.totalSpentUsd)} of a ${formatUsd(savings.baselineUsd)} baseline`}>
        <div className={styles.barSpent} style={{ width: `${spentPct}%` }} />
      </div>
      <div className={styles.barLegend}>
        <span>{formatUsd(savings.totalSpentUsd)} actual</span>
        <span>{formatUsd(savings.baselineUsd)} baseline</span>
      </div>

      <dl className={styles.stats}>
        <div className={styles.stat}>
          <dt>Routes</dt>
          <dd>{savings.routes}</dd>
        </div>
        <div className={styles.stat}>
          <dt>Tokens in</dt>
          <dd>{formatCompact(savings.tokensIn)}</dd>
        </div>
        <div className={styles.stat}>
          <dt>Tokens out</dt>
          <dd>{formatCompact(savings.tokensOut)}</dd>
        </div>
      </dl>

      {savings.byModel.length > 0 && (
        <ul className={styles.models}>
          {savings.byModel.map((model) => (
            <li key={`${model.model}-${model.baseline}`} className={styles.model}>
              <span className={styles.route}>
                {model.model} <span className={styles.arrow}>vs</span> {model.baseline}
              </span>
              <span className={styles.modelSaved}>
                {formatUsd(model.savedUsd)} · {model.routes}×
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
