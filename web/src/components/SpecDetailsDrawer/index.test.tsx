import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Card, PullRequest } from '../../types/board'
import { SpecDetailsDrawer } from './index'
import { jsonResponse, stubFetch } from '../../test/fetchStub'

// The drawer fetches its post-action summary on mount; stub it so the test
// renders offline. SpecTimeline is collapsed by default (passes a null id), so
// it never fetches.
vi.mock('../../api/useSpecSummary', () => ({
  useSpecSummary: () => ({ data: null, loading: false, error: null }),
}))

afterEach(cleanup)

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard')

function setClipboard(value: unknown) {
  Object.defineProperty(navigator, 'clipboard', { value, configurable: true, writable: true })
}

afterEach(() => {
  if (originalClipboard) Object.defineProperty(navigator, 'clipboard', originalClipboard)
  else setClipboard(undefined)
})

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

describe('SpecDetailsDrawer resolution', () => {
  it('shows the resolution and its note for a closed spec', () => {
    render(
      <SpecDetailsDrawer
        card={makeCard({ status: 'closed', resolution: 'superseded', resolutionNote: 'replaced by checkout-v2' })}
        epics={[]}
        onClose={() => {}}
      />,
    )
    const section = screen.getByRole('region', { name: 'Resolution' })
    expect(section.textContent).toContain('Closed as superseded')
    expect(section.textContent).toContain('not counted toward epic progress')
    expect(section.textContent).toContain('replaced by checkout-v2')
  })

  it('has no resolution section when the spec has none', () => {
    render(<SpecDetailsDrawer card={makeCard({ status: 'open' })} epics={[]} onClose={() => {}} />)
    expect(screen.queryByRole('region', { name: 'Resolution' })).toBeNull()
  })
})

describe('SpecDetailsDrawer next command by PR presence', () => {
  const pr: PullRequest = { url: 'https://github.com/o/r/pull/7', number: 7, draft: false, openedAt: '2026-06-27T00:00:00Z' }

  it('suggests ship for a review spec with no recorded PR', () => {
    render(<SpecDetailsDrawer card={makeCard({ status: 'review', id: 'fix-raw-tags' })} epics={[]} onClose={() => {}} />)
    expect(screen.getByRole('button', { name: 'Copy command: /vector:ship fix-raw-tags' })).toBeTruthy()
  })

  it('suggests close once the review spec has a recorded PR', () => {
    render(<SpecDetailsDrawer card={makeCard({ status: 'review', id: 'fix-raw-tags', pr })} epics={[]} onClose={() => {}} />)
    expect(screen.getByRole('button', { name: 'Copy command: /vector:close fix-raw-tags' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Copy command: /vector:ship fix-raw-tags' })).toBeNull()
  })
})

describe('SpecDetailsDrawer close without PR', () => {
  const pr: PullRequest = { url: 'https://github.com/o/r/pull/7', number: 7, draft: false, openedAt: '2026-06-27T00:00:00Z' }

  it('keeps close copyable as an alternative for a review spec with no PR', () => {
    render(<SpecDetailsDrawer card={makeCard({ status: 'review', id: 'fix-raw-tags' })} epics={[]} onClose={() => {}} />)
    expect(screen.getByText('Close without PR')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Copy command: /vector:close fix-raw-tags' })).toBeTruthy()
  })

  it('drops the alternative once a PR is recorded — close is already the next command', () => {
    render(<SpecDetailsDrawer card={makeCard({ status: 'review', id: 'fix-raw-tags', pr })} epics={[]} onClose={() => {}} />)
    expect(screen.queryByText('Close without PR')).toBeNull()
    expect(screen.getAllByRole('button', { name: 'Copy command: /vector:close fix-raw-tags' })).toHaveLength(1)
  })
})

describe('SpecDetailsDrawer next step', () => {
  it('offers the shell line and the slash command as peers, with the $ out of the clipboard', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    setClipboard({ writeText })
    render(<SpecDetailsDrawer card={makeCard({ id: 'add-dark-mode', status: 'in-progress' })} epics={[]} onClose={() => {}} />)

    const section = screen.getByRole('region', { name: 'Next step' })
    expect(section.textContent).toContain('From a terminal')
    expect(section.textContent).toContain('Inside Claude Code')

    const shellCopy = screen.getByRole('button', { name: 'Copy shell command: vector open add-dark-mode' })
    fireEvent.click(shellCopy)
    expect(writeText).toHaveBeenCalledWith('vector open add-dark-mode')

    fireEvent.click(screen.getByRole('button', { name: 'Copy command: /vector:apply add-dark-mode' }))
    expect(writeText).toHaveBeenLastCalledWith('/vector:apply add-dark-mode')
  })

  it('keeps the terminal line for a closed spec and says why Claude has nothing left', () => {
    render(<SpecDetailsDrawer card={makeCard({ id: 'add-dark-mode', status: 'closed' })} epics={[]} onClose={() => {}} />)

    const section = screen.getByRole('region', { name: 'Next step' })
    expect(screen.getByRole('button', { name: 'Copy shell command: vector open add-dark-mode' })).toBeTruthy()
    expect(screen.queryByText('Inside Claude Code')).toBeNull()
    expect(section.textContent).toContain('opens a plain shell in its worktree')
  })
})
