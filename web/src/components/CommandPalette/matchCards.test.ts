import { describe, expect, it } from 'vitest'
import type { Card } from '../../types/board'
import { matchCards } from './matchCards'

function makeCard(overrides: Partial<Card>): Card {
  return {
    id: 'spec-id',
    title: 'A spec',
    status: 'open',
    priority: 'normal',
    hasOpenSpec: false,
    savedUsd: 0,
    routes: 0,
    tokensIn: 0,
    tokensOut: 0,
    updatedAt: '2026-06-27T00:00:00Z',
    ...overrides,
  }
}

const darkMode = makeCard({ id: 'add-dark-mode', title: 'Dark mode', priority: 'high' })
const embedFix = makeCard({
  id: 'fix-broken-embed',
  title: 'Fix broken embed (v2.*) [beta]',
  priority: 'urgent',
  status: 'in-progress',
})
const palette = makeCard({
  id: 'add-board-command-palette',
  title: 'Command palette',
  priority: 'normal',
  status: 'review',
})
const cards = [darkMode, embedFix, palette]

describe('matchCards', () => {
  it('matches case-insensitively by title', () => {
    expect(matchCards(cards, 'DARK Mode', [])).toEqual([darkMode])
  })

  it('matches by id/slug', () => {
    expect(matchCards(cards, 'broken-embed', [])).toEqual([embedFix])
  })

  it('matches by priority-as-text', () => {
    expect(matchCards(cards, 'urgent', [])).toEqual([embedFix])
  })

  it('matches by status-as-text', () => {
    expect(matchCards(cards, 'review', [])).toEqual([palette])
  })

  it('treats regex metacharacters as literal text without throwing', () => {
    expect(matchCards(cards, '(v2.*)', [])).toEqual([embedFix])
    expect(matchCards(cards, '[beta]', [])).toEqual([embedFix])
    expect(() => matchCards(cards, '\\', [])).not.toThrow()
    expect(matchCards(cards, '\\', [])).toEqual([])
    // A lone `.` is literal: only the one haystack containing a dot matches.
    expect(matchCards(cards, '.*', [])).toEqual([embedFix])
    expect(matchCards(cards, '(', [])).toEqual([embedFix])
  })

  it('returns every card for an empty or whitespace-only query', () => {
    expect(matchCards(cards, '', [])).toEqual(cards)
    expect(matchCards(cards, '   ', [])).toEqual(cards)
  })

  it('applies the multi-select priority filter', () => {
    expect(matchCards(cards, '', ['high'])).toEqual([darkMode])
    expect(matchCards(cards, '', ['high', 'urgent'])).toEqual([darkMode, embedFix])
    expect(matchCards(cards, 'embed', ['high'])).toEqual([])
  })

  it('preserves input order without relevance sorting', () => {
    // 'mode' hits darkMode by title; 'command' hits palette; a query hitting
    // several cards keeps board order.
    expect(matchCards(cards, 'add-', [])).toEqual([darkMode, palette])
  })
})
