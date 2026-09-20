import type { CSSProperties } from 'react'
import type { Card, Column } from '../../types/board'
import { SpecCard } from '../SpecCard/SpecCard'
import { statusRailColor } from '../../lib/statusRailColor'
import styles from './BoardColumn.module.css'

interface BoardColumnProps {
  column: Column
  onSelectCard: (card: Card) => void
}

// BoardColumn is the Ledger column: a mono header (status key · count · rule ·
// rail dot) that stays put, over its own vertical scroller. The header sits
// outside the scroll area, so it needs no sticky trick, and the scroller carries
// overscroll-behavior:contain so reaching the end does not drag the board row.
export function BoardColumn({ column, onSelectCard }: BoardColumnProps) {
  const cards = column.cards ?? []
  const dotStyle: CSSProperties = { background: statusRailColor(column.status) }

  return (
    <section className={styles.column}>
      <header className={styles.header}>
        {/* The key, not the label: the board speaks in lowercase English, the
            same vocabulary as the slash commands and the state JSON. */}
        <h2 className={styles.key}>{column.status}</h2>
        <span className={styles.count}>{column.count}</span>
        <span className={styles.rule} />
        <span className={styles.dot} style={dotStyle} />
      </header>
      <div className={styles.cards}>
        {cards.length === 0 ? (
          <p className={styles.empty}>— no specs</p>
        ) : (
          cards.map((card) => <SpecCard key={card.id} card={card} onSelect={onSelectCard} />)
        )}
      </div>
    </section>
  )
}
