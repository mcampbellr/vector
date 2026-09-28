import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Card } from '../../types/board'
import { SpecDetailsDrawer } from './index'
import { jsonResponse, stubFetch } from '../../test/fetchStub'

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
        epics={[]}
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
        epics={[]}
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

describe('SpecDetailsDrawer focus and epic controls', () => {
  const epics = [
    {
      id: 'app-mobile',
      title: 'App Mobile',
      total: 0,
      done: 0,
      byStatus: {},
      updatedAt: '2026-06-27T00:00:00Z',
    },
  ]

  afterEach(() => vi.unstubAllGlobals())

  it('assigns the spec to an epic through the API', async () => {
    const { requests } = stubFetch(async () => jsonResponse(200, { id: 'spec-id', epic: 'app-mobile', changed: true }))
    render(<SpecDetailsDrawer card={makeCard({ status: 'open' })} epics={epics} onClose={() => {}} />)

    const select = screen.getByRole('combobox', { name: 'Epic' }) as HTMLSelectElement
    expect(select.value).toBe('')
    fireEvent.change(select, { target: { value: 'app-mobile' } })

    expect(requests).toEqual([{ url: '/api/specs/spec-id/epic', method: 'POST', body: { epic: 'app-mobile' } }])
  })

  it('clears the epic with null and shows a failure inline', async () => {
    const { requests } = stubFetch(async () => jsonResponse(400, { error: 'epic "app-mobile" does not exist' }))
    render(
      <SpecDetailsDrawer card={makeCard({ status: 'open', epic: 'app-mobile' })} epics={epics} onClose={() => {}} />,
    )

    fireEvent.change(screen.getByRole('combobox', { name: 'Epic' }), { target: { value: '' } })
    expect(requests[0].body).toEqual({ epic: null })
    expect((await screen.findByRole('alert')).textContent).toContain('does not exist')
  })

  it('offers the focus toggle in the meta row', () => {
    render(<SpecDetailsDrawer card={makeCard({ status: 'open', focus: true })} epics={epics} onClose={() => {}} />)
    expect(screen.getByRole('button', { name: 'Unfocus spec' })).toBeTruthy()
  })
})
