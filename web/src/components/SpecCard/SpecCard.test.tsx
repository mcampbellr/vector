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

describe('SpecCard needs-attention', () => {
  it('renders the category chip and a truncatable summary with a full title', () => {
    render(
      <SpecCard
        card={makeCard({
          status: 'needs-attention',
          attentionCategory: 'dependency',
          attentionSummary: 'Zoho api_names pending settings-read credentials',
          attentionReason: 'Zoho api_names pending settings-read credentials',
        })}
        onSelect={() => {}}
      />,
    )

    expect(screen.getByText('Dependency')).toBeTruthy()
    const summary = screen.getByTitle('Zoho api_names pending settings-read credentials')
    expect(summary.textContent).toContain('Zoho api_names pending')
  })

  it('omits the chip for an unknown/absent category but still shows the summary', () => {
    render(
      <SpecCard
        card={makeCard({ status: 'needs-attention', attentionSummary: 'waiting on a decision' })}
        onSelect={() => {}}
      />,
    )

    expect(screen.queryByText('Dependency')).toBeNull()
    expect(screen.getByTitle('waiting on a decision')).toBeTruthy()
  })

  it('falls back to attentionReason when the structured fields are absent', () => {
    render(
      <SpecCard
        card={makeCard({ status: 'needs-attention', attentionReason: 'blocked on the DTO rename' })}
        onSelect={() => {}}
      />,
    )

    expect(screen.getByText('blocked on the DTO rename')).toBeTruthy()
    // No category chip on a purely-legacy card.
    for (const label of ['Dependency', 'Env', 'Decision', 'External', 'Other']) {
      expect(screen.queryByText(label)).toBeNull()
    }
  })

  it('renders nothing attention-related when the card is not blocked', () => {
    render(<SpecCard card={makeCard({})} onSelect={() => {}} />)
    expect(screen.queryByText('Dependency')).toBeNull()
  })
})

describe('SpecCard ticket badge', () => {
  it('renders the full ticket key intact alongside a very long title', () => {
    render(
      <SpecCard
        card={makeCard({
          title:
            'Reparar clipping del badge de ticket en el encabezado de la tarjeta de spec del board kanban',
          ticket: {
            provider: 'linear',
            key: 'MH-1814',
            url: 'https://linear.app/acme/issue/MH-1814',
          },
        })}
        onSelect={() => {}}
      />,
    )

    // The key must exist as a single, complete text node — not split at the
    // hyphen (MH-) nor clipped away. CSS-level visual truncation can't be
    // asserted in jsdom, so we assert full DOM presence + the title fallback.
    const badge = screen.getByTitle('https://linear.app/acme/issue/MH-1814')
    expect(badge.textContent).toContain('MH-1814')
    expect(screen.getByText('MH-1814')).toBeTruthy()
  })
})

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
