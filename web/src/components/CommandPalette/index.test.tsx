import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Card } from '../../types/board'
import { CommandPalette } from './index'

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

const cards = [
  makeCard({ id: 'add-dark-mode', title: 'Dark mode', priority: 'high' }),
  makeCard({ id: 'fix-embed', title: 'Fix embed', priority: 'urgent', status: 'in-progress' }),
  makeCard({ id: 'add-palette', title: 'Command palette', priority: 'normal', status: 'review' }),
]

function renderPalette(overrides?: {
  onSelectCard?: (card: Card) => void
  onClose?: () => void
}) {
  const onSelectCard = overrides?.onSelectCard ?? vi.fn()
  const onClose = overrides?.onClose ?? vi.fn()
  const view = render(
    <CommandPalette cards={cards} onSelectCard={onSelectCard} onClose={onClose} />,
  )
  return { onSelectCard, onClose, view }
}

describe('CommandPalette', () => {
  it('focuses the search input on mount', () => {
    renderPalette()
    expect(document.activeElement).toBe(screen.getByRole('combobox'))
  })

  it('filters by title, id, priority-as-text and status-as-text, case-insensitively', () => {
    renderPalette()
    const input = screen.getByRole('combobox')

    fireEvent.change(input, { target: { value: 'DARK' } })
    expect(screen.getAllByRole('option')).toHaveLength(1)
    expect(screen.getByText('add-dark-mode')).toBeTruthy()

    fireEvent.change(input, { target: { value: 'fix-embed' } })
    expect(screen.getAllByRole('option')).toHaveLength(1)
    expect(screen.getByText('Fix embed')).toBeTruthy()

    fireEvent.change(input, { target: { value: 'urgent' } })
    expect(screen.getAllByRole('option')).toHaveLength(1)
    expect(screen.getByText('Fix embed')).toBeTruthy()

    fireEvent.change(input, { target: { value: 'review' } })
    expect(screen.getAllByRole('option')).toHaveLength(1)
    expect(screen.getByText('Command palette')).toBeTruthy()
  })

  it('refines the list with the priority chips', () => {
    renderPalette()
    expect(screen.getAllByRole('option')).toHaveLength(3)

    fireEvent.click(screen.getByRole('button', { name: 'high' }))
    expect(screen.getAllByRole('option')).toHaveLength(1)
    expect(screen.getByText('Dark mode')).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'urgent' }))
    expect(screen.getAllByRole('option')).toHaveLength(2)

    // Toggling both off restores the unfiltered list.
    fireEvent.click(screen.getByRole('button', { name: 'high' }))
    fireEvent.click(screen.getByRole('button', { name: 'urgent' }))
    expect(screen.getAllByRole('option')).toHaveLength(3)
  })

  it('moves the highlight with the arrows and selects with Enter', () => {
    const { onSelectCard, onClose } = renderPalette()
    const input = screen.getByRole('combobox')

    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    // No wraparound: a third ArrowDown stays on the last row.
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(input.getAttribute('aria-activedescendant')).toBe('palette-option-add-palette')

    fireEvent.keyDown(input, { key: 'ArrowUp' })
    expect(input.getAttribute('aria-activedescendant')).toBe('palette-option-fix-embed')

    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSelectCard).toHaveBeenCalledWith(cards[1])
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('selects a row on click, calling onSelectCard and onClose', () => {
    const { onSelectCard, onClose } = renderPalette()
    fireEvent.click(screen.getByText('Command palette'))
    expect(onSelectCard).toHaveBeenCalledWith(cards[2])
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('closes on Escape', () => {
    const { onClose } = renderPalette()
    fireEvent.keyDown(screen.getByRole('combobox'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('closes on overlay click but not on a click inside the panel', () => {
    const { onClose, view } = renderPalette()
    fireEvent.click(screen.getByRole('dialog'))
    expect(onClose).not.toHaveBeenCalled()

    const overlay = view.container.firstElementChild
    expect(overlay).not.toBeNull()
    fireEvent.click(overlay!)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('renders the No results empty state without crashing', () => {
    renderPalette()
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'zzz-no-match' } })
    expect(screen.getByText('No results')).toBeTruthy()
    expect(screen.queryAllByRole('option')).toHaveLength(0)
  })
})
