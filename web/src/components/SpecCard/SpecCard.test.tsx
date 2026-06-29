import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { Card } from '../../types/board'
import { SpecCard } from './SpecCard'

afterEach(cleanup)

// makeCard builds a minimal Card; override per test.
function makeCard(overrides: Partial<Card>): Card {
  return {
    id: 'spec-id',
    title: 'A spec',
    status: 'in-progress',
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

describe('SpecCard quick-win badge', () => {
  it('renders the read-only Quick Win badge when quickWin is set', () => {
    render(<SpecCard card={makeCard({ quickWin: true })} onSelect={() => {}} />)

    const badge = screen.getByLabelText('Quick win')
    expect(badge).toBeTruthy()
    expect(badge.textContent).toContain('Quick Win')
  })

  it('omits the badge when quickWin is absent', () => {
    render(<SpecCard card={makeCard({})} onSelect={() => {}} />)

    expect(screen.queryByLabelText('Quick win')).toBeNull()
  })
})
