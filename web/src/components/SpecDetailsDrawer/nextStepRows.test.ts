import { describe, expect, it } from 'vitest'
import type { Status } from '../../types/board'
import { nextStepRows, openCommandFor } from './nextStepRows'

const ID = 'add-dark-mode'

describe('nextStepRows', () => {
  it('leads with the terminal row for every status', () => {
    const statuses: Status[] = ['open', 'in-progress', 'needs-attention', 'review', 'closed']
    for (const status of statuses) {
      const [first] = nextStepRows(status, ID)
      expect(first).toEqual({ label: 'From a terminal', command: `vector open ${ID}`, shell: true })
    }
  })

  it('pairs the terminal row with /vector:apply while the spec is workable', () => {
    for (const status of ['open', 'in-progress', 'needs-attention'] as Status[]) {
      expect(nextStepRows(status, ID)[1]).toEqual({
        label: 'Inside Claude Code',
        command: `/vector:apply ${ID}`,
        shell: false,
      })
    }
  })

  it('pairs it with /vector:close in review', () => {
    expect(nextStepRows('review', ID)[1]).toEqual({
      label: 'Inside Claude Code',
      command: `/vector:close ${ID}`,
      shell: false,
    })
  })

  it('keeps only the terminal row for a closed spec', () => {
    const rows = nextStepRows('closed', ID)
    expect(rows).toHaveLength(1)
    expect(rows[0].shell).toBe(true)
  })

  it('marks exactly one row as a shell line', () => {
    expect(nextStepRows('in-progress', ID).filter((row) => row.shell)).toHaveLength(1)
  })

  it('builds the open command without a leading slash', () => {
    expect(openCommandFor(ID)).toBe('vector open add-dark-mode')
    expect(openCommandFor(ID).startsWith('/')).toBe(false)
  })
})
