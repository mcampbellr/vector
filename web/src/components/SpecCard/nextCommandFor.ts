import type { Card } from '../../types/board'

/**
 * Returns the slash command the user should run next for a card, or null when
 * no command applies (closed = terminal state, even with a recorded PR).
 *
 * A card in review needs shipping until /vector:ship records its PR; once the
 * PR exists the next step is /vector:close (run after the merge). The PR is the
 * only signal: a draft PR still reads as shipped, and merge is not detected.
 */
export function nextCommandFor(card: Pick<Card, 'status' | 'id' | 'pr'>): string | null {
  switch (card.status) {
    case 'open':
    case 'in-progress':
    case 'needs-attention':
      return `/vector:apply ${card.id}`
    case 'review':
      return card.pr ? `/vector:close ${card.id}` : `/vector:ship ${card.id}`
    case 'closed':
      return null
  }
}
