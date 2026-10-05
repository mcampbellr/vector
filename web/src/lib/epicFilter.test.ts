import { describe, expect, it } from 'vitest'
import type { Card, Column, EpicSummary } from '../types/board'
import {
  ALL_EPICS,
  filterColumnsByEpic,
  parseEpicFilter,
  resolveEpicFilter,
  serializeEpicFilter,
} from './epicFilter'

function card(id: string, epic?: string): Card {
  return {
    id,
    title: id,
    status: 'open',
    priority: 'normal',
    hasOpenSpec: false,
    savedUsd: 0,
    routes: 0,
    tokensIn: 0,
    tokensOut: 0,
    updatedAt: '2026-06-27T00:00:00Z',
    epic,
  }
}

const columns: Column[] = [
  { status: 'open', label: 'Open', count: 3, cards: [card('a', 'mobile'), card('b'), card('c', 'web')] },
  { status: 'review', label: 'Review', count: 1, cards: [card('d', 'mobile')] },
]

const ids = (filtered: Column[]) => filtered.map((column) => column.cards.map((c) => c.id).join(','))

describe('epicFilter', () => {
  it('keeps every column and recounts the filtered cards', () => {
    const mobile = filterColumnsByEpic(columns, { kind: 'epic', epicId: 'mobile' })
    expect(ids(mobile)).toEqual(['a', 'd'])
    expect(mobile.map((column) => column.count)).toEqual([1, 1])

    expect(ids(filterColumnsByEpic(columns, { kind: 'none' }))).toEqual(['b', ''])
    expect(filterColumnsByEpic(columns, ALL_EPICS)).toBe(columns)
  })

  it('round-trips through the ?epic= query parameter', () => {
    expect(parseEpicFilter('')).toEqual(ALL_EPICS)
    expect(parseEpicFilter('?epic=none')).toEqual({ kind: 'none' })
    expect(parseEpicFilter('?epic=app-mobile')).toEqual({ kind: 'epic', epicId: 'app-mobile' })
    expect(serializeEpicFilter(ALL_EPICS)).toBeNull()
    expect(serializeEpicFilter({ kind: 'none' })).toBe('none')
    expect(serializeEpicFilter({ kind: 'epic', epicId: 'app-mobile' })).toBe('app-mobile')
  })

  it('falls back to all when the filtered epic is not on the board', () => {
    const epics: EpicSummary[] = [
      { id: 'mobile', title: 'Mobile', total: 2, done: 0, byStatus: {}, updatedAt: '' },
    ]
    const kept = { kind: 'epic', epicId: 'mobile' } as const
    expect(resolveEpicFilter(kept, epics)).toBe(kept)
    expect(resolveEpicFilter({ kind: 'epic', epicId: 'gone' }, epics)).toBe(ALL_EPICS)
    expect(resolveEpicFilter({ kind: 'none' }, [])).toEqual({ kind: 'none' })
  })
})
