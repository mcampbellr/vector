import type { Card, Priority } from '../../types/board'

// Diacritic-insensitive fold: NFD-decompose then strip combining marks
// (U+0300–U+036F) so "é" → "e", "ï" → "i", etc., before the case fold.
// Applied to both the query and the haystack so "movil" matches "móvil",
// "MÓVIL" matches "movil", and "naive"/"naïve" match each other.
function foldDiacritics(value: string): string {
  return value.normalize('NFD').replace(/[\u0300-\u036f]/g, '')
}

// matchCards is the palette's pure filter: diacritic- and case-insensitive
// literal substring over a title+id+priority+status haystack — never a
// RegExp, so user-typed metacharacters (. * ( [) stay literal text. An empty
// query filters nothing by text; active priorities additionally narrow the
// set. Input order is preserved — no relevance scoring.
export function matchCards(cards: Card[], query: string, priorities: Priority[]): Card[] {
  const normalized = foldDiacritics(query.trim().toLowerCase())
  return cards.filter((card) => {
    if (priorities.length > 0 && !priorities.includes(card.priority)) return false
    if (normalized === '') return true
    const haystack = foldDiacritics(
      `${card.title} ${card.id} ${card.priority} ${card.status}`.toLowerCase(),
    )
    return haystack.includes(normalized)
  })
}
