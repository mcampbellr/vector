import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { EpicSummary } from '../../types/board'
import { EpicChip } from './EpicChip'

afterEach(cleanup)

const mobile: EpicSummary = {
  id: 'app-mobile',
  title: 'App Mobile',
  color: 'violet',
  total: 2,
  done: 1,
  byStatus: { open: 1, closed: 1 },
  updatedAt: '2026-06-27T00:00:00Z',
}

describe('EpicChip', () => {
  it('shows the epic title tinted with its palette token', () => {
    render(<EpicChip epicId="app-mobile" epic={mobile} />)

    const chip = screen.getByTitle('Epic: App Mobile')
    expect(chip.textContent).toBe('App Mobile')
    expect(chip.style.getPropertyValue('--epic-chip-color')).toBe('var(--epic-violet)')
  })

  it('falls back to the neutral colour for an epic without one', () => {
    render(<EpicChip epicId="web" epic={{ ...mobile, id: 'web', title: 'Web', color: undefined }} />)
    expect(screen.getByTitle('Epic: Web').style.getPropertyValue('--epic-chip-color')).toBe(
      'var(--color-text-secondary)',
    )
  })

  it('still shows a dangling epic id when the epic is not on the board', () => {
    render(<EpicChip epicId="deleted-epic" />)
    expect(screen.getByTitle('Epic: deleted-epic').textContent).toBe('deleted-epic')
  })
})
