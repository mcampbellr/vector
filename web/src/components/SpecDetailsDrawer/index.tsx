import { Suspense, lazy, useEffect } from 'react'
import { Tag, X } from 'lucide-react'
import type { Card } from '../../types/board'
import { useSpecSummary } from '../../api/useSpecSummary'
import { StatusPill } from '../StatusPill/StatusPill'
import { PriorityFlag } from '../PriorityFlag/PriorityFlag'
import { nextCommandFor } from '../SpecCard/nextCommandFor'
import { RelatedChips } from '../SpecCard/RelatedChips'
import { AttentionCategoryChip } from '../SpecCard/AttentionCategoryChip'
import { SpecTimeline } from '../SpecTimeline'
import { CopyableCommand } from './CopyableCommand'
import { UsefulCommands } from './UsefulCommands'
import { SpecArtifactBrowser } from './SpecArtifactBrowser'
import { CopyableSlug } from '../CopyableSlug/CopyableSlug'
import { relativeTime } from '../../lib/format'
import { useNow } from '../../lib/useNow'
import styles from './SpecDetailsDrawer.module.css'

// MarkdownView (and its react-markdown dependency) is code-split out of the
// board bundle; the drawer only pulls it in when it renders an attention detail.
const MarkdownView = lazy(() => import('./MarkdownView'))

interface SpecDetailsDrawerProps {
  card: Card
  onClose: () => void
}

// SpecDetailsDrawer is the right-side panel that opens when a board card is
// clicked. It surfaces the AI "what was done" summary, the activity timeline,
// the next command, and context-aware copyable useful commands. It mutates
// nothing — the board stays read-only (architecture/state-model.md). The summary
// is fetched lazily: the drawer only mounts when a card is selected, so the fetch
// fires on open and is torn down on close.
export function SpecDetailsDrawer({ card, onClose }: SpecDetailsDrawerProps) {
  const { data: summary, loading, error } = useSpecSummary(card.id)
  const now = useNow(30_000)

  // Close on Escape — the drawer is a modal surface (role="dialog").
  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const hasSummary = !!summary?.summary
  const nextCommand = nextCommandFor(card.status, card.id)

  return (
    <div className={styles.overlay} onClick={onClose}>
      <aside
        className={styles.drawer}
        role="dialog"
        aria-modal="true"
        aria-label={`Details for ${card.title}`}
        onClick={(event) => event.stopPropagation()}
      >
        <header className={styles.header}>
          <div className={styles.headerMain}>
            <h2 className={styles.title}>{card.title}</h2>
            <CopyableSlug slug={card.id} />
          </div>
          <button type="button" className={styles.close} aria-label="Close details" onClick={onClose}>
            <X size={16} strokeWidth={2.5} />
          </button>
        </header>

        <div className={styles.metaRow}>
          <StatusPill status={card.status} />
          <PriorityFlag priority={card.priority} />
          {card.ticket && (
            <a
              className={styles.ticket}
              href={card.ticket.url}
              target="_blank"
              rel="noreferrer"
              title={card.ticket.url}
            >
              <Tag size={11} strokeWidth={2} />
              {card.ticket.key}
            </a>
          )}
        </div>

        {(card.attentionSummary || card.attentionReason) && (
          <section className={styles.section}>
            <div className={styles.attentionHeader}>
              <AttentionCategoryChip category={card.attentionCategory} />
              <p className={styles.attentionSummaryText}>
                {card.attentionSummary ?? card.attentionReason}
              </p>
            </div>
            {card.attentionDetail && (
              <Suspense fallback={<p className={styles.muted}>loading detail…</p>}>
                <div className={styles.markdown}>
                  <MarkdownView source={card.attentionDetail} />
                </div>
              </Suspense>
            )}
          </section>
        )}

        {card.relatedTo && card.relatedTo.length > 0 && (
          <section className={styles.section}>
            <h3 className={styles.sectionTitle}>Related</h3>
            <RelatedChips related={card.relatedTo} />
          </section>
        )}

        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Summary</h3>
          {loading && <p className={styles.muted}>loading summary…</p>}
          {error && <p className={styles.error}>could not load summary: {error}</p>}
          {!loading && !error && !hasSummary && (
            <p className={styles.muted}>
              No summary yet. It is generated after the next lifecycle command on this spec.
            </p>
          )}
          {!loading && !error && hasSummary && (
            <>
              <p className={styles.summaryProse}>{summary!.summary}</p>
              <p className={styles.summaryMeta}>
                {summary!.action ? `after /vector:${summary!.action}` : ''}
                {summary!.generatedAt ? ` · ${relativeTime(summary!.generatedAt, now)}` : ''}
              </p>
            </>
          )}
        </section>

        {nextCommand && (
          <section className={styles.section}>
            <h3 className={styles.sectionTitle}>Next command</h3>
            <div className={styles.cmdList}>
              <CopyableCommand label="Run next" command={nextCommand} />
            </div>
          </section>
        )}

        <UsefulCommands card={card} />

        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Activity</h3>
          <SpecTimeline specId={card.id} />
        </section>

        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Files</h3>
          <SpecArtifactBrowser card={card} />
        </section>
      </aside>
    </div>
  )
}
