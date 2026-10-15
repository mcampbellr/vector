import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Card, Column } from '../../types/board'
import { KanbanBoard } from './KanbanBoard'

afterEach(cleanup)

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

function makeColumn(overrides: Partial<Column>): Column {
  return {
    status: 'open',
    label: 'Open',
    cards: [],
    count: 0,
    ...overrides,
  }
}

describe('KanbanBoard', () => {
  it('delegates card selection through the onSelectCard prop', () => {
    const card = makeCard({ id: 'add-dark-mode', title: 'Dark mode' })
    const onSelectCard = vi.fn()
    render(
      <KanbanBoard
        columns={[makeColumn({ cards: [card], count: 1 })]}
        epics={[]}
        onSelectCard={onSelectCard}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: /^Open details for Dark mode/ }))
    expect(onSelectCard).toHaveBeenCalledWith(card)
  })

  it('owns no selection state and renders no details drawer', () => {
    const card = makeCard({ id: 'add-dark-mode', title: 'Dark mode' })
    render(
      <KanbanBoard
        columns={[makeColumn({ cards: [card], count: 1 })]}
        epics={[]}
        onSelectCard={() => {}}
      />,
    )

    // Clicking a card only delegates — no drawer (role="dialog") appears.
    fireEvent.click(screen.getByRole('button', { name: /^Open details for Dark mode/ }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})

describe('KanbanBoard freshness tick', () => {
  it('resolves one tick and propagates it to every card through its column', () => {
    // The board owns the single timer; the cards take `now` as a prop. If the
    // tick stopped reaching them, the age would not render at all and the
    // card's label would fall back to the bare title.
    render(
      <KanbanBoard
        columns={[
          makeColumn({ cards: [makeCard({ id: 'a', title: 'First' })], count: 1 }),
          makeColumn({
            status: 'review',
            label: 'Review',
            cards: [makeCard({ id: 'b', title: 'Second', status: 'review' })],
            count: 1,
          }),
        ]}
        epics={[]}
        onSelectCard={() => {}}
      />,
    )

    // The age itself is relative to the real clock here, so this asserts that it
    // arrived, not what it says — cardAge.test.ts pins the values.
    for (const title of ['First', 'Second']) {
      const card = screen.getByRole('button', { name: new RegExp(`^Open details for ${title}`) })
      expect(card.getAttribute('aria-label')).toContain(', updated ')
    }
  })
})
