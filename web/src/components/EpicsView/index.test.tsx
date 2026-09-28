import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Board, Card, EpicSummary } from '../../types/board'
import { EpicsView } from './index'
import { jsonResponse, stubFetch } from '../../test/fetchStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

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

const mobile: EpicSummary = {
  id: 'app-mobile',
  title: 'App Mobile',
  description: 'iOS and Android clients',
  color: 'blue',
  total: 3,
  done: 2,
  byStatus: { 'in-progress': 1, closed: 1, archived: 1 },
  updatedAt: '2026-06-27T00:00:00Z',
}

function makeBoard(epics: EpicSummary[]): Board {
  return {
    schemaVersion: 2,
    repo: 'vector',
    generatedAt: '2026-06-27T00:00:00Z',
    updatedAt: '2026-06-27T00:00:00Z',
    columns: [
      {
        status: 'in-progress',
        label: 'In progress',
        count: 1,
        cards: [makeCard({ id: 'push', title: 'Push notifications', status: 'in-progress', epic: 'app-mobile', focus: true })],
      },
      {
        status: 'closed',
        label: 'Closed',
        count: 2,
        cards: [
          makeCard({ id: 'login', title: 'Login screen', status: 'closed', epic: 'app-mobile' }),
          makeCard({ id: 'docs', title: 'Docs', status: 'closed' }),
        ],
      },
    ],
    epics,
    tokenSavings: {
      totalSavedUsd: 0,
      totalSpentUsd: 0,
      baselineUsd: 0,
      routes: 0,
      tokensIn: 0,
      tokensOut: 0,
      byModel: [],
    },
    totals: { specs: 4 },
  }
}

describe('EpicsView', () => {
  it('tells the user how to create an epic when there is none', () => {
    render(<EpicsView board={makeBoard([])} onSelectCard={() => {}} onShowOnBoard={() => {}} />)

    expect(screen.getByText('No epics yet')).toBeTruthy()
    expect(screen.getByText('vector epic create --title "App Mobile"')).toBeTruthy()
    expect(screen.getByRole('button', { name: /new epic/ })).toBeTruthy()
  })

  it('shows each epic with its progress, status breakdown and specs', () => {
    render(<EpicsView board={makeBoard([mobile])} onSelectCard={() => {}} onShowOnBoard={() => {}} />)

    const section = screen.getByRole('region', { name: 'Epic App Mobile' })
    expect(section.textContent).toContain('2 of 3 done')
    expect(section.textContent).toContain('iOS and Android clients')
    expect(screen.getByRole('progressbar', { name: 'App Mobile progress' }).getAttribute('aria-valuenow')).toBe('67')

    const breakdown = screen.getByRole('list', { name: 'Specs by status' })
    expect(breakdown.textContent).toBe('in-progress1closed1archived1')

    expect(screen.getByRole('button', { name: 'Open details for Push notifications' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Open details for Login screen' })).toBeTruthy()
    // A spec of no epic is not listed under it…
    expect(screen.queryByRole('button', { name: 'Open details for Docs' })).toBeNull()
    // …but the view points at the unassigned ones.
    expect(screen.getByText(/1 spec has no epic/)).toBeTruthy()
    expect(screen.getByText('+ 1 archived spec')).toBeTruthy()
  })

  it('opens the drawer for a spec and applies the board filter for an epic', () => {
    const onSelectCard = vi.fn()
    const onShowOnBoard = vi.fn()
    render(<EpicsView board={makeBoard([mobile])} onSelectCard={onSelectCard} onShowOnBoard={onShowOnBoard} />)

    fireEvent.click(screen.getByRole('button', { name: 'Open details for Push notifications' }))
    expect(onSelectCard).toHaveBeenCalledWith(expect.objectContaining({ id: 'push' }))

    fireEvent.click(screen.getByRole('button', { name: /show on board/ }))
    expect(onShowOnBoard).toHaveBeenCalledWith({ kind: 'epic', epicId: 'app-mobile' })

    fireEvent.click(screen.getByRole('button', { name: 'show them on the board' }))
    expect(onShowOnBoard).toHaveBeenCalledWith({ kind: 'none' })
  })

  it('creates an epic from the new epic form', async () => {
    const { requests } = stubFetch(async () =>
      jsonResponse(201, { schemaVersion: 1, id: 'web', title: 'Web', color: 'teal', createdAt: '', updatedAt: '' }),
    )
    render(<EpicsView board={makeBoard([])} onSelectCard={() => {}} onShowOnBoard={() => {}} />)

    fireEvent.click(screen.getByRole('button', { name: /new epic/ }))
    const form = screen.getByRole('form', { name: 'New epic' })
    fireEvent.change(screen.getByLabelText('title'), { target: { value: '  Web  ' } })
    fireEvent.click(screen.getByRole('radio', { name: 'teal' }))
    fireEvent.submit(form)

    await waitFor(() => expect(screen.queryByRole('form', { name: 'New epic' })).toBeNull())
    expect(requests).toEqual([{ url: '/api/epics', method: 'POST', body: { title: 'Web', color: 'teal' } }])
  })

  it('keeps the form open with the server error when creation fails', async () => {
    stubFetch(async () => jsonResponse(409, { error: 'epic "web" already exists' }))
    render(<EpicsView board={makeBoard([])} onSelectCard={() => {}} onShowOnBoard={() => {}} />)

    fireEvent.click(screen.getByRole('button', { name: /new epic/ }))
    fireEvent.change(screen.getByLabelText('title'), { target: { value: 'Web' } })
    fireEvent.submit(screen.getByRole('form', { name: 'New epic' }))

    expect((await screen.findByRole('alert')).textContent).toBe('epic "web" already exists')
    expect(screen.getByRole('form', { name: 'New epic' })).toBeTruthy()
  })

  it('edits an epic through PATCH', async () => {
    const { requests } = stubFetch(async () => jsonResponse(200, { ...mobile, schemaVersion: 1, createdAt: '' }))
    render(<EpicsView board={makeBoard([mobile])} onSelectCard={() => {}} onShowOnBoard={() => {}} />)

    fireEvent.click(screen.getByRole('button', { name: 'Edit epic App Mobile' }))
    fireEvent.change(screen.getByLabelText('title'), { target: { value: 'Mobile' } })
    fireEvent.submit(screen.getByRole('form', { name: 'Edit epic App Mobile' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toEqual({
      url: '/api/epics/app-mobile',
      method: 'PATCH',
      body: { title: 'Mobile', description: 'iOS and Android clients', color: 'blue', order: 0 },
    })
  })

  it('shows the order, the dropped count and edits the order through PATCH', async () => {
    const ordered: EpicSummary = { ...mobile, order: 2, dropped: 1 }
    const { requests } = stubFetch(async () => jsonResponse(200, { ...ordered, schemaVersion: 1, createdAt: '' }))
    render(<EpicsView board={makeBoard([ordered])} onSelectCard={() => {}} onShowOnBoard={() => {}} />)

    const section = screen.getByRole('region', { name: 'Epic App Mobile' })
    expect(section.textContent).toContain('#2')
    expect(section.textContent).toContain('2 of 3 done · 1 dropped')

    fireEvent.click(screen.getByRole('button', { name: 'Edit epic App Mobile' }))
    const orderInput = screen.getByLabelText('order') as HTMLInputElement
    expect(orderInput.value).toBe('2')
    fireEvent.change(orderInput, { target: { value: 'x' } })
    const submit = screen.getByRole('button', { name: 'save' }) as HTMLButtonElement
    expect(submit.disabled).toBe(true)
    fireEvent.change(orderInput, { target: { value: '1' } })
    fireEvent.submit(screen.getByRole('form', { name: 'Edit epic App Mobile' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].body).toEqual({ title: 'App Mobile', description: 'iOS and Android clients', color: 'blue', order: 1 })
  })

  it('focuses an epic from its pin and marks inherited focus on its specs', async () => {
    const focusedEpic: EpicSummary = { ...mobile, focus: true }
    const board = makeBoard([focusedEpic])
    board.columns[0].cards[0] = makeCard({
      id: 'push',
      title: 'Push notifications',
      status: 'in-progress',
      epic: 'app-mobile',
      focusInherited: true,
    })
    const { requests } = stubFetch(async () => jsonResponse(200, { id: 'app-mobile', focus: false, changed: true }))
    render(<EpicsView board={board} onSelectCard={() => {}} onShowOnBoard={() => {}} />)

    expect(screen.getByLabelText('Focus inherited from the epic')).toBeTruthy()
    const pin = screen.getByRole('button', { name: 'Unfocus epic App Mobile' })
    expect(pin.getAttribute('aria-pressed')).toBe('true')
    fireEvent.click(pin)
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toEqual({ url: '/api/epics/app-mobile/focus', method: 'POST', body: { focus: false } })
  })
})
