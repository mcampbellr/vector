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
        onSelectCard={onSelectCard}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Open details for Dark mode' }))
    expect(onSelectCard).toHaveBeenCalledWith(card)
  })

  it('owns no selection state and renders no details drawer', () => {
    const card = makeCard({ id: 'add-dark-mode', title: 'Dark mode' })
    render(
      <KanbanBoard
        columns={[makeColumn({ cards: [card], count: 1 })]}
        onSelectCard={() => {}}
      />,
    )

    // Clicking a card only delegates — no drawer (role="dialog") appears.
    fireEvent.click(screen.getByRole('button', { name: 'Open details for Dark mode' }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
