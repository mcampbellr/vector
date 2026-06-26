import { ClipboardCheck, Clock, Sparkles, Tag } from 'lucide-react'
import type { Card } from '../../types/board'
import { StatusPill } from '../StatusPill/StatusPill'
import { PriorityFlag } from '../PriorityFlag/PriorityFlag'
import { ArtifactDot } from './ArtifactDot'
import { NextCommand } from './NextCommand'
import { SpecTimeline } from '../SpecTimeline'
import { formatEstimate, formatUsd } from '../../lib/format'
import styles from './SpecCard.module.css'

interface SpecCardProps {
  card: Card
}

export function SpecCard({ card }: SpecCardProps) {
  return (
    <article className={styles.card}>
      <header className={styles.head}>
        <h3 className={styles.title}>{card.title}</h3>
        {card.ticket && (
          <span className={styles.ticket} title={card.ticket.url}>
            <Tag size={11} strokeWidth={2} />
            {card.ticket.key}
          </span>
        )}
      </header>

      {card.attentionReason && <p className={styles.attention}>{card.attentionReason}</p>}

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

      <NextCommand status={card.status} id={card.id} />

      <SpecTimeline specId={card.id} />
    </article>
  )
}
