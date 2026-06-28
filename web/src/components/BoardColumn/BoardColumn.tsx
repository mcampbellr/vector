import type { Card, Column } from '../../types/board'
import { SpecCard } from '../SpecCard/SpecCard'
import styles from './BoardColumn.module.css'

interface BoardColumnProps {
  column: Column
  onSelectCard: (card: Card) => void
}

export function BoardColumn({ column, onSelectCard }: BoardColumnProps) {
  const cards = column.cards ?? []
  return (
    <section className={styles.column}>
      <header className={styles.header}>
        <h2 className={styles.title}>{column.label}</h2>
        <span className={styles.count}>{column.count}</span>
      </header>
      <div className={styles.cards}>
        {cards.length === 0 ? (
          <p className={styles.empty}>No specs</p>
        ) : (
          cards.map((card) => <SpecCard key={card.id} card={card} onSelect={onSelectCard} />)
        )}
      </div>
    </section>
  )
}
