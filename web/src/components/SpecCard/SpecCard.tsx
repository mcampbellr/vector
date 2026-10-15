import type { CSSProperties, KeyboardEvent } from 'react'
import { ClipboardCheck, Clock, Layers, Zap } from 'lucide-react'
import type { Card, EpicSummary } from '../../types/board'
import { EpicChip } from '../EpicChip/EpicChip'
import { FocusToggle } from '../FocusToggle/FocusToggle'
import { CardAgeStamp } from './CardAgeStamp'
import { CardArtifactMeter } from './CardArtifactMeter'
import { CardAttentionRow } from './CardAttentionRow'
import { CardPriorityFlag } from './CardPriorityFlag'
import { CardSlugButton } from './CardSlugButton'
import { CardVerbButton } from './CardVerbButton'
import { ResolutionBadge } from './ResolutionBadge'
import { cardAge } from './cardAge'
import type { CardAge } from './cardAge'
import { clipTitle } from './clipTitle'
import { nextVerbFor } from './nextCommandFor'
import { rowThreeFit } from './rowThreeFit'
import { shortTicketRef } from './shortTicketRef'
import { statusRailColor } from '../../lib/statusRailColor'
import styles from './SpecCard.module.css'

interface SpecCardProps {
  card: Card
  /** The card's epic, resolved from Board.epics by the column (absent when the
   *  spec has no epic or the epic is not on the board). */
  epic?: EpicSummary
  /** One tick for the whole board, resolved in KanbanBoard and passed down, so
   *  the age stays current without 95 cards each owning a timer. */
  now: number
  onSelect: (card: Card) => void
}

// SpecCard is the board face for a spec, in Ledger form: a status rail plus
// three fixed rows — title · identity (slug, epic chip, artifacts) · status
// (focus pin first) — and a fourth row only when the spec needs attention. The status pill is gone (the column already names the
// status), the artifact labels collapsed into a three-segment meter and the next
// command into its verb. The timeline, AI summary and useful commands stay in
// the details drawer, opened by clicking the card.
//
// The card is an article[role=button][tabindex=0], not a <button>: buttons do
// not nest and there are three inside (the slug, the focus pin and the verb).
export function SpecCard({ card, epic, now, onSelect }: SpecCardProps) {
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
  const age = cardAge(card.updatedAt, card.status, now)
  const focusVisible = card.focus === true || card.focusInherited === true

  // Row 3 budgets itself: it overflowed in the worst case and clipped the verb.
  const fit = rowThreeFit({
    priority: card.priority,
    status: card.status,
    resolution: card.resolution,
    quickWin: card.quickWin === true,
    uat: card.needsUat === true,
    focusVisible,
    sketchCount,
    estimateMinutes: card.estimateMinutes,
    tokens: card.tokensIn + card.tokensOut,
    routes: card.routes,
    verb: nextVerbFor(card),
  })

  // The age rides along in the label: a screen reader gets the card's freshness
  // without reaching the stamp, which is not focusable.
  const ariaLabel = age
    ? `Open details for ${card.title}, updated ${age.relative}${ariaToneSuffix(age.tone)}`
    : `Open details for ${card.title}`

  return (
    <article
      className={styles.card}
      role="button"
      tabIndex={0}
      aria-label={ariaLabel}
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
          {card.epic && (
            <span className={styles.epicSlot}>
              <EpicChip epicId={card.epic} epic={epic} />
            </span>
          )}
          {age && <CardAgeStamp age={age} />}
          <CardArtifactMeter artifacts={card.artifacts} afterAge={age !== null} />
        </div>

        <div className={styles.statusRow}>
          {/* Focus leads the row: it outranks priority in the column order. */}
          <span className={styles.focusSlot}>
            <FocusToggle
              specId={card.id}
              focused={card.focus === true}
              canFocus={card.status !== 'closed'}
              variant="card"
              inheritedFrom={card.focusInherited ? (epic?.title ?? card.epic) : undefined}
            />
          </span>
          <CardPriorityFlag priority={card.priority} status={card.status} />
          <ResolutionBadge resolution={card.resolution} note={card.resolutionNote} />
          {card.quickWin && (
            <span className={styles.glyph} title="Quick win" aria-label="Quick win">
              <Zap size={11} strokeWidth={2} />
            </span>
          )}
          {/* Glyph-only, like quick win: the word `uat` cost width the verb
              needed, and the meaning is already in the title and the label. */}
          {card.status === 'review' && card.needsUat && (
            <span
              className={styles.glyph}
              title="Requires manual UAT before closing"
              aria-label="Requires manual UAT before closing"
            >
              <ClipboardCheck size={11} strokeWidth={2} />
            </span>
          )}
          {fit.showSketches && sketchCount > 0 && (
            <span
              className={`${styles.glyph} ${styles.glyphDim}`}
              title={`${sketchCount} Excalidraw sketch${sketchCount > 1 ? 'es' : ''} attached`}
              aria-label={`${sketchCount} Excalidraw sketch${sketchCount > 1 ? 'es' : ''} attached`}
            >
              <Layers size={11} strokeWidth={2} />
              {sketchCount}
            </span>
          )}
          {/* Planned before spent: the glyph is the only thing telling them apart.
              Both are sheddable — the drawer shows either in full. */}
          {fit.showEstimate && (
            <span className={`${styles.glyph} ${styles.glyphDim}`} title="Estimate">
              <Clock size={11} strokeWidth={2} />
              {fit.estimateText}
            </span>
          )}
          {fit.showTokens && (
            <span className={styles.tokens} title={`Tokens spent · ${card.routes} cheap-agent routes`}>
              {fit.tokensText}
            </span>
          )}
          <CardVerbButton card={card} />
        </div>

        <CardAttentionRow card={card} />
      </div>
    </article>
  )
}

function ariaToneSuffix(tone: CardAge['tone']): string {
  if (tone === 'stale') return ', stale'
  if (tone === 'very-stale') return ', very stale'
  return ''
}
