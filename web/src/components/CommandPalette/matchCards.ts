import type { Card, Priority } from '../../types/board'

// matchCards is the palette's pure filter: case-insensitive literal substring
// over a title+id+priority+status haystack — never a RegExp, so user-typed
// metacharacters (. * ( [) stay literal text. An empty query filters nothing
// by text; active priorities additionally narrow the set. Input order is
// preserved — no relevance scoring.
export function matchCards(cards: Card[], query: string, priorities: Priority[]): Card[] {
  const normalized = query.trim().toLowerCase()
  return cards.filter((card) => {
    if (priorities.length > 0 && !priorities.includes(card.priority)) return false
    if (normalized === '') return true
    const haystack = `${card.title} ${card.id} ${card.priority} ${card.status}`.toLowerCase()
    return haystack.includes(normalized)
  })
}
