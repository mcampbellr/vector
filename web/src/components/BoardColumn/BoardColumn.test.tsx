import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { Card, Column } from '../../types/board'
import { BoardColumn } from './BoardColumn'

afterEach(cleanup)

// makeCard builds a minimal Card so a column can hold real cards; override per test.
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

// makeColumn builds a Column with sane defaults; override per test. Own fixture
// (not makeCard) — the column shape is what BoardColumn renders.
function makeColumn(overrides: Partial<Column>): Column {
  return {
    status: 'open',
    label: 'Open',
    cards: [],
    count: 0,
    ...overrides,
  }
}

describe('BoardColumn', () => {
  it('renders the sticky header with label and count for a column with cards', () => {
    const cards = [
      makeCard({ id: 'a', title: 'Spec A' }),
      makeCard({ id: 'b', title: 'Spec B' }),
      makeCard({ id: 'c', title: 'Spec C' }),
    ]
    const { container } = render(
      <BoardColumn column={makeColumn({ label: 'In progress', cards, count: cards.length })} onSelectCard={() => {}} />,
    )

    const header = container.querySelector('header')
    expect(header).not.toBeNull()
    expect(header?.querySelector('h2')?.textContent).toBe('In progress')
    expect(header?.querySelector('span')?.textContent).toBe('3')
    expect(screen.queryByText('No specs')).toBeNull()
  })

  it('shows the empty state for a column with no cards', () => {
    render(<BoardColumn column={makeColumn({ label: 'Review', cards: [], count: 0 })} onSelectCard={() => {}} />)

    expect(screen.getByRole('heading', { level: 2 }).textContent).toBe('Review')
    expect(screen.getByText('No specs')).toBeTruthy()
  })
})
