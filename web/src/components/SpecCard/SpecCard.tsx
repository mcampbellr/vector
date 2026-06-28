import type { KeyboardEvent } from 'react'
import { ClipboardCheck, Clock, Sparkles, Tag } from 'lucide-react'
import type { Card } from '../../types/board'
import { StatusPill } from '../StatusPill/StatusPill'
import { PriorityFlag } from '../PriorityFlag/PriorityFlag'
import { ArtifactDot } from './ArtifactDot'
import { RelatedChips } from './RelatedChips'
import { CardNextCommand } from './CardNextCommand'
import { CopyableSlug } from '../CopyableSlug/CopyableSlug'
import { formatEstimate, formatUsd } from '../../lib/format'
import styles from './SpecCard.module.css'

interface SpecCardProps {
  card: Card
  onSelect: (card: Card) => void
}

// SpecCard is the board face for a spec: metadata (title, slug, ticket,
// artifacts, status, priority, estimate, savings) plus a quick-copy next
// command. The
// activity timeline, AI summary and useful commands remain in the details
// drawer, opened by clicking the card — keeping the face uncluttered
// (spec-details-drawer).
export function SpecCard({ card, onSelect }: SpecCardProps) {
  function handleKeyDown(event: KeyboardEvent<HTMLElement>) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      onSelect(card)
    }
  }

  return (
    <article
      className={styles.card}
      role="button"
      tabIndex={0}
      aria-label={`Open details for ${card.title}`}
      onClick={() => onSelect(card)}
      onKeyDown={handleKeyDown}
    >
      <header className={styles.head}>
        <h3 className={styles.title}>{card.title}</h3>
        {card.ticket && (
          <span className={styles.ticket} title={card.ticket.url}>
            <Tag size={11} strokeWidth={2} />
            {card.ticket.key}
          </span>
        )}
      </header>

      <CopyableSlug slug={card.id} />

      {card.attentionReason && <p className={styles.attention}>{card.attentionReason}</p>}

      {card.relatedTo && card.relatedTo.length > 0 && <RelatedChips related={card.relatedTo} />}

      {card.artifacts && (
        <div className={styles.artifacts}>
          <ArtifactDot label="proposal" on={card.artifacts.proposal} />
          <ArtifactDot label="design" on={card.artifacts.design} />
          <ArtifactDot label="tasks" on={card.artifacts.tasks} />
        </div>
      )}

      <footer className={styles.meta}>
        <StatusPill status={card.status} />
        {card.status === 'review' && card.needsUat && (
          <span className={styles.uat} title="Requires manual UAT" aria-label="Requires manual UAT">
            <ClipboardCheck size={12} strokeWidth={2} />
            UAT
          </span>
        )}
        <PriorityFlag priority={card.priority} />
        {card.estimateMinutes ? (
          <span className={styles.estimate}>
            <Clock size={13} strokeWidth={2} />
            {formatEstimate(card.estimateMinutes)}
          </span>
        ) : null}
        {card.routes > 0 && (
          <span className={styles.savings} title={`${card.routes} cheap-agent routes`}>
            <Sparkles size={12} strokeWidth={2} />
            {formatUsd(card.savedUsd)} saved
          </span>
        )}
      </footer>

      <CardNextCommand status={card.status} id={card.id} />
    </article>
  )
}
