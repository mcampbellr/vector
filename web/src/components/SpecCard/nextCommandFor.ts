import type { Card } from '../../types/board'

/**
 * Returns the slash command the user should run next for a card, or null when
 * no command applies (closed = terminal state, even with a recorded PR).
 *
 * A card in review needs shipping until /vector:ship records its PR; once the
 * PR exists the next step is /vector:close (run after the merge). The PR is the
 * only signal: a draft PR still reads as shipped, and merge is not detected.
 */
/**
 * The bare verb of that command (`apply`, `ship`, `close`), which is what the
 * card's button prints — the whole line only rides in its tooltip. Shared so the
 * button and the status row's width budget (rowThreeFit.ts) read the same word
 * instead of each re-deriving it.
 */
export function nextVerbFor(card: Pick<Card, 'status' | 'id' | 'pr'>): string | null {
  const command = nextCommandFor(card)
  if (command === null) return null
  return command.split(' ')[0].replace('/vector:', '')
}

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
