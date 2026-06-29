import { Sparkles } from 'lucide-react'
import type { Board } from '../../types/board'
import { formatCompact } from '../../lib/format'
import { SpecModelBreakdown } from './SpecModelBreakdown'
import { TokenBreakdownRow } from './TokenBreakdownRow'
import styles from './TokenBreakdownView.module.css'

interface TokenBreakdownViewProps {
  board: Board
}

// TokenBreakdownView is the token-native savings view: how many tokens each spec
// moved off the expensive baseline and which cheap models handled it. It is a
// pure projection of the already-loaded board (same useBoard() SSE — no fetch),
// denominated only in tokens (no dollars). Loading/error/reconnect are handled
// by App.tsx; this view is never mounted without a board, so its only tab-local
// state is empty (no routes) vs the rows.
export function TokenBreakdownView({ board }: TokenBreakdownViewProps) {
  const { tokenSavings } = board
  const totalTokens = tokenSavings.tokensIn + tokenSavings.tokensOut

  const rows = board.columns
    .flatMap((column) => column.cards)
    .filter((card) => card.routes > 0)
    .sort((a, b) => b.tokensIn + b.tokensOut - (a.tokensIn + a.tokensOut))

  if (tokenSavings.routes === 0) {
    return (
      <section className={styles.view}>
        <header className={styles.header}>
          <h2 className={styles.title}>
            <Sparkles size={18} strokeWidth={2} />
            Token Breakdown
          </h2>
        </header>
        <div className={styles.state}>
          <p className={styles.empty}>No routed token events yet</p>
          <p className={styles.hint}>
            Run /vector:raw and follow-up commands to log cheap-agent routing here.
          </p>
        </div>
      </section>
    )
  }

  return (
    <section className={styles.view}>
      <header className={styles.header}>
        <h2 className={styles.title}>
          <Sparkles size={18} strokeWidth={2} />
          Token Breakdown
        </h2>
        <div className={styles.aggregate}>
          <span className={styles.headline}>{formatCompact(totalTokens)}</span>
          <span className={styles.headlineLabel}>tokens saved</span>
          <span className={styles.routeCount}>
            across {tokenSavings.routes} {tokenSavings.routes === 1 ? 'route' : 'routes'}
          </span>
        </div>
      </header>

      {tokenSavings.byModel.length > 0 && (
        <div className={styles.globalBreakdown}>
          <SpecModelBreakdown models={tokenSavings.byModel} />
        </div>
      )}

      <table className={styles.table}>
        <thead>
          <tr>
            <th scope="col" className={styles.specHead}>
              Spec
            </th>
            <th scope="col" className={styles.numHead}>
              Tokens in
            </th>
            <th scope="col" className={styles.numHead}>
              Tokens out
            </th>
            <th scope="col" className={styles.numHead}>
              Routes
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((card) => (
            <TokenBreakdownRow key={card.id} card={card} />
          ))}
        </tbody>
      </table>
    </section>
  )
}
