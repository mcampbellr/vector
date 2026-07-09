import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Card } from '../../types/board'
import { SpecDetailsDrawer } from './index'

// The drawer fetches its post-action summary on mount; stub it so the test
// renders offline. SpecTimeline is collapsed by default (passes a null id), so
// it never fetches.
vi.mock('../../api/useSpecSummary', () => ({
  useSpecSummary: () => ({ data: null, loading: false, error: null }),
}))

afterEach(cleanup)

function makeCard(overrides: Partial<Card>): Card {
  return {
    id: 'spec-id',
    title: 'A spec',
    status: 'needs-attention',
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

describe('SpecDetailsDrawer needs-attention', () => {
  it('renders the chip, full summary and markdown detail (emphasis, list, link)', async () => {
    render(
      <SpecDetailsDrawer
        card={makeCard({
          attentionCategory: 'external',
          attentionSummary: 'Zoho api_names pending credentials',
          attentionDetail: 'See **PR #367** and the [ticket](https://x/MH-1582) with:\n\n- fill the TODO\n- confirm creds',
        })}
        onClose={() => {}}
      />,
    )

    expect(screen.getByText('External')).toBeTruthy()
    expect(screen.getByText('Zoho api_names pending credentials')).toBeTruthy()

    // Markdown is lazy (React.lazy + Suspense) → await the rendered nodes.
    expect(await screen.findByText('PR #367')).toBeTruthy() // <strong>
    const link = await screen.findByRole('link', { name: 'ticket' })
    expect(link.getAttribute('href')).toBe('https://x/MH-1582')
    expect(screen.getByText('fill the TODO')).toBeTruthy() // <li>
  })

  it('falls back to plain-text attentionReason when there is no detail', () => {
    render(
      <SpecDetailsDrawer
        card={makeCard({ attentionReason: 'blocked on the DTO rename' })}
        onClose={() => {}}
      />,
    )

    expect(screen.getByText('blocked on the DTO rename')).toBeTruthy()
    // Purely-legacy card: no category chip is rendered.
    for (const label of ['Dependency', 'Env', 'Decision', 'External', 'Other']) {
      expect(screen.queryByText(label)).toBeNull()
    }
  })
})
