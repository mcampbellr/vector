import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Board, Card } from './types/board'
import { App } from './App'
import { ThemeProvider } from './context/ThemeContext'

// The drawer fetches its post-action summary unconditionally on mount
// (SpecDetailsDrawer/index.tsx:35) — stub it so the test renders offline.
vi.mock('./api/useSpecSummary', () => ({
  useSpecSummary: () => ({ data: null, loading: false, error: null }),
}))

// Standup and tokens views fetch their own data; the palette/drawer wiring
// under test does not depend on their internals.
vi.mock('./components/StandupView', () => ({
  StandupView: () => <div>standup-view-stub</div>,
}))
vi.mock('./components/TokenBreakdownView', () => ({
  TokenBreakdownView: () => <div>tokens-view-stub</div>,
}))

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

function makeBoard(): Board {
  return {
    schemaVersion: 1,
    repo: 'vector',
    generatedAt: '2026-06-27T00:00:00Z',
    updatedAt: '2026-06-27T00:00:00Z',
    columns: [
      {
        status: 'open',
        label: 'Open',
        count: 1,
        cards: [makeCard({ id: 'add-dark-mode', title: 'Dark mode' })],
      },
    ],
    tokenSavings: {
      totalSavedUsd: 0,
      totalSpentUsd: 0,
      baselineUsd: 0,
      routes: 0,
      tokensIn: 0,
      tokensOut: 0,
      byModel: [],
    },
    totals: { specs: 1 },
  }
}

vi.mock('./api/useBoard', () => ({
  useBoard: () => ({ board: makeBoard(), connection: 'live', error: null }),
}))

afterEach(cleanup)

function openPaletteWithSlash() {
  fireEvent.keyDown(window, { key: '/' })
}

// App renders ThemeControl, which requires the ThemeProvider (as in main.tsx).
function renderApp() {
  return render(
    <ThemeProvider>
      <App />
    </ThemeProvider>,
  )
}

describe('App command palette wiring', () => {
  it('opens the palette with / from the standup and tokens views', () => {
    renderApp()

    fireEvent.click(screen.getByRole('button', { name: 'Standup' }))
    openPaletteWithSlash()
    expect(screen.getByRole('dialog', { name: 'Command palette' })).toBeTruthy()
    fireEvent.keyDown(screen.getByRole('combobox'), { key: 'Escape' })

    fireEvent.click(screen.getByRole('button', { name: 'Tokens' }))
    openPaletteWithSlash()
    expect(screen.getByRole('dialog', { name: 'Command palette' })).toBeTruthy()
  })

  it('jumps to the details drawer from standup without changing the active tab', () => {
    renderApp()

    fireEvent.click(screen.getByRole('button', { name: 'Standup' }))
    openPaletteWithSlash()
    const input = screen.getByRole('combobox')
    fireEvent.change(input, { target: { value: 'dark' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    // Drawer opened for the selected spec; palette closed; still on standup.
    expect(screen.getByRole('dialog', { name: 'Details for Dark mode' })).toBeTruthy()
    expect(screen.queryByRole('dialog', { name: 'Command palette' })).toBeNull()
    expect(screen.getByText('standup-view-stub')).toBeTruthy()
  })

  it('ignores / while focus is inside an input, including the palette itself', () => {
    renderApp()

    openPaletteWithSlash()
    const input = screen.getByRole('combobox')
    expect(document.activeElement).toBe(input)

    // Typing / inside the palette input neither reopens nor interferes.
    fireEvent.keyDown(input, { key: '/' })
    expect(screen.getAllByRole('combobox')).toHaveLength(1)
    expect(screen.getAllByRole('dialog', { name: 'Command palette' })).toHaveLength(1)
  })

  it('Escape closes only the palette when the drawer is open behind it', () => {
    renderApp()

    // Open the drawer via the palette.
    openPaletteWithSlash()
    let input = screen.getByRole('combobox')
    fireEvent.change(input, { target: { value: 'dark' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(screen.getByRole('dialog', { name: 'Details for Dark mode' })).toBeTruthy()

    // Reopen the palette on top of the drawer.
    openPaletteWithSlash()
    input = screen.getByRole('combobox')

    // First Escape: palette closes, drawer stays (stopPropagation on the panel).
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(screen.queryByRole('dialog', { name: 'Command palette' })).toBeNull()
    expect(screen.getByRole('dialog', { name: 'Details for Dark mode' })).toBeTruthy()

    // Second Escape (on window): the drawer's own listener closes it.
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(screen.queryByRole('dialog', { name: 'Details for Dark mode' })).toBeNull()
  })
})
