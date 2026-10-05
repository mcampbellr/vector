import type { Card, Column, EpicSummary } from '../types/board'

/** Which specs the board shows: every spec, only specs without an epic, or only
 *  the specs of one epic. */
export type EpicFilter = { kind: 'all' } | { kind: 'none' } | { kind: 'epic'; epicId: string }

export const ALL_EPICS: EpicFilter = { kind: 'all' }

/** The URL query parameter that persists the filter (`?epic=<id>` or
 *  `?epic=none`), so a filtered board survives reloads and can be shared. */
export const EPIC_FILTER_PARAM = 'epic'

const NO_EPIC_VALUE = 'none'

export function parseEpicFilter(search: string): EpicFilter {
  const value = new URLSearchParams(search).get(EPIC_FILTER_PARAM)
  if (!value) return ALL_EPICS
  if (value === NO_EPIC_VALUE) return { kind: 'none' }
  return { kind: 'epic', epicId: value }
}

/** The query-param value for a filter; null means "remove the param". */
export function serializeEpicFilter(filter: EpicFilter): string | null {
  switch (filter.kind) {
    case 'all':
      return null
    case 'none':
      return NO_EPIC_VALUE
    case 'epic':
      return filter.epicId
  }
}

export function matchesEpicFilter(card: Card, filter: EpicFilter): boolean {
  switch (filter.kind) {
    case 'all':
      return true
    case 'none':
      return !card.epic
    case 'epic':
      return card.epic === filter.epicId
  }
}

/** Narrows every column to the matching cards, keeping each column (and its
 *  order) so the board layout stays stable; counts follow the filtered cards. */
export function filterColumnsByEpic(columns: Column[], filter: EpicFilter): Column[] {
  if (filter.kind === 'all') return columns
  return columns.map((column) => {
    const cards = column.cards.filter((card) => matchesEpicFilter(card, filter))
    return { ...column, cards, count: cards.length }
  })
}

/** A filter naming an epic that is not on the board (deleted, or a stale URL)
 *  falls back to showing everything instead of an inexplicably empty board. */
export function resolveEpicFilter(filter: EpicFilter, epics: EpicSummary[]): EpicFilter {
  if (filter.kind === 'epic' && !epics.some((epic) => epic.id === filter.epicId)) return ALL_EPICS
  return filter
}
