import type { Card } from '../../types/board'
import { formatCompact } from '../../lib/format'
import { SpecModelBreakdown } from './SpecModelBreakdown'
import styles from './TokenBreakdownView.module.css'

interface TokenBreakdownRowProps {
  card: Card
}

// TokenBreakdownRow renders one routed spec: a totals row (id, title, tokens in,
// tokens out, routes) plus — when the spec has a per-model breakdown — a second,
// muted row spanning the table with its `model→baseline` rollup. Tokens only.
export function TokenBreakdownRow({ card }: TokenBreakdownRowProps) {
  const models = card.byModel ?? []

  return (
    <>
      <tr className={styles.specRow}>
        <td className={styles.specCell}>
          <span className={styles.specTitle}>{card.title || card.id}</span>
          <span className={styles.specId}>{card.id}</span>
        </td>
        <td className={styles.numCell}>{formatCompact(card.tokensIn)}</td>
        <td className={styles.numCell}>{formatCompact(card.tokensOut)}</td>
        <td className={styles.numCell}>{card.routes}</td>
      </tr>
      {models.length > 0 && (
        <tr className={styles.breakdownRow}>
          <td className={styles.breakdownCell} colSpan={4}>
            <SpecModelBreakdown models={models} />
          </td>
        </tr>
      )}
    </>
  )
}
