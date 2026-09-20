import type { Card } from '../../types/board'
import { AttentionCategoryLabel, isKnownAttentionCategory } from './AttentionCategoryLabel'
import styles from './SpecCard.module.css'

interface CardAttentionRowProps {
  card: Card
}

// CardAttentionRow is the card's only optional row: it goes last, under a 1px
// rule, and closes the card at four rows. A structured card shows the category
// label plus a one-line summary; a legacy card (attentionReason only) shows the
// text in attention red — the red is what flags that the datum is unstructured.
export function CardAttentionRow({ card }: CardAttentionRowProps) {
  const text = card.attentionSummary ?? card.attentionReason
  if (!text) return null

  // Derived from whether a label will actually render, not merely from the field
  // being set: an unknown category renders no label, so the row must fall back to
  // the legacy red rather than end up with neither label nor warning colour.
  const structured = isKnownAttentionCategory(card.attentionCategory)

  return (
    <div className={styles.attentionRow}>
      <AttentionCategoryLabel category={card.attentionCategory} />
      <span
        className={`${styles.attentionSummary}${structured ? '' : ` ${styles.attentionSummaryLegacy}`}`}
        title={text}
      >
        {text}
      </span>
    </div>
  )
}
