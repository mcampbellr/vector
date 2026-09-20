import type { CSSProperties, KeyboardEvent } from 'react'
import { ClipboardCheck, Clock, Layers, Zap } from 'lucide-react'
import type { Card } from '../../types/board'
import { CardArtifactMeter } from './CardArtifactMeter'
import { CardAttentionRow } from './CardAttentionRow'
import { CardPriorityFlag } from './CardPriorityFlag'
import { CardSlugButton } from './CardSlugButton'
import { CardVerbButton } from './CardVerbButton'
import { clipTitle } from './clipTitle'
import { shortTicketRef } from './shortTicketRef'
import { formatCompact, formatEstimate } from '../../lib/format'
import { statusRailColor } from '../../lib/statusRailColor'
import styles from './SpecCard.module.css'

interface SpecCardProps {
  card: Card
  onSelect: (card: Card) => void
}

// SpecCard is the board face for a spec, in Ledger form: a status rail plus
// three fixed rows — title · identity · status — and a fourth row only when the
// spec needs attention. The status pill is gone (the column already names the
// status), the artifact labels collapsed into a three-segment meter and the next
// command into its verb. The timeline, AI summary and useful commands stay in
// the details drawer, opened by clicking the card.
//
// The card is an article[role=button][tabindex=0], not a <button>: buttons do
// not nest and there are two inside (the slug and the verb).
export function SpecCard({ card, onSelect }: SpecCardProps) {
  function handleKeyDown(event: KeyboardEvent<HTMLElement>) {
    // Only the card itself opens the drawer. A keydown on the slug or the verb
    // bubbles up here, and preventDefault() from an ancestor cancels the nested
    // button's own activation — without this guard, Enter/Space on either of
    // them copies nothing and opens the drawer instead.
    if (event.target !== event.currentTarget) return
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      onSelect(card)
    }
  }

  // Without a ticket ref the title gets the full width back, so it fits ~62
  // characters over its two lines instead of ~54.
  const titleShown = clipTitle(card.title, card.ticket ? 54 : 62)
  const railStyle: CSSProperties = { background: statusRailColor(card.status) }
  const sketchCount = card.sketches?.length ?? 0

  return (
    <article
      className={styles.card}
      role="button"
      tabIndex={0}
      aria-label={`Open details for ${card.title}`}
      onClick={() => onSelect(card)}
      onKeyDown={handleKeyDown}
    >
      <div className={styles.rail} style={railStyle} />
      <div className={styles.body}>
        <div className={styles.titleRow}>
          <div className={styles.titleBox}>
            <h3 className={styles.title} title={card.title}>
              {titleShown}
            </h3>
          </div>
          {card.ticket && (
            <span className={styles.ticket} title={card.ticket.url || card.ticket.key}>
              {shortTicketRef(card.ticket.key)}
            </span>
          )}
        </div>

        <div className={styles.identityRow}>
          <CardSlugButton slug={card.id} />
          <CardArtifactMeter artifacts={card.artifacts} />
        </div>

        <div className={styles.statusRow}>
          <CardPriorityFlag priority={card.priority} status={card.status} />
          {card.quickWin && (
            <span className={styles.glyph} title="Quick win" aria-label="Quick win">
              <Zap size={11} strokeWidth={2} />
            </span>
          )}
          {card.status === 'review' && card.needsUat && (
            <span
              className={styles.glyph}
              title="Requires manual UAT before closing"
              aria-label="Requires manual UAT before closing"
            >
              <ClipboardCheck size={11} strokeWidth={2} />
              uat
            </span>
          )}
          {sketchCount > 0 && (
            <span
              className={`${styles.glyph} ${styles.glyphDim}`}
              title={`${sketchCount} Excalidraw sketch${sketchCount > 1 ? 'es' : ''} attached`}
              aria-label={`${sketchCount} Excalidraw sketch${sketchCount > 1 ? 'es' : ''} attached`}
            >
              <Layers size={11} strokeWidth={2} />
              {sketchCount}
            </span>
          )}
          {/* Planned before spent: the glyph is the only thing telling them apart. */}
          {card.estimateMinutes ? (
            <span className={`${styles.glyph} ${styles.glyphDim}`} title="Estimate">
              <Clock size={11} strokeWidth={2} />
              {formatEstimate(card.estimateMinutes)}
            </span>
          ) : null}
          {card.routes > 0 && (
            <span className={styles.tokens} title={`Tokens spent · ${card.routes} cheap-agent routes`}>
              {formatCompact(card.tokensIn + card.tokensOut)} tok
            </span>
          )}
          <CardVerbButton status={card.status} id={card.id} />
        </div>

        <CardAttentionRow card={card} />
      </div>
    </article>
  )
}
