import { describe, expect, it } from 'vitest'
import type { PullRequest, Status } from '../../types/board'
import { nextStepRows, openCommandFor } from './nextStepRows'

const ID = 'add-dark-mode'
const PR: PullRequest = { url: 'https://github.com/o/r/pull/7', number: 7, draft: false, openedAt: '2026-06-27T00:00:00Z' }

function card(status: Status, id = ID, pr?: PullRequest) {
  return { status, id, pr }
}

describe('nextStepRows', () => {
  it('leads with the terminal row for every status', () => {
    const statuses: Status[] = ['open', 'in-progress', 'needs-attention', 'review', 'closed']
    for (const status of statuses) {
      const [first] = nextStepRows(card(status))
      expect(first).toEqual({ label: 'From a terminal', command: `vector open ${ID}`, shell: true })
    }
  })

  it('pairs the terminal row with /vector:apply while the spec is workable', () => {
    for (const status of ['open', 'in-progress', 'needs-attention'] as Status[]) {
      expect(nextStepRows(card(status))[1]).toEqual({
        label: 'Inside Claude Code',
        command: `/vector:apply ${ID}`,
        shell: false,
      })
    }
  })

  it('pairs it with /vector:ship in review until a PR is recorded', () => {
    expect(nextStepRows(card('review'))[1]).toEqual({
      label: 'Inside Claude Code',
      command: `/vector:ship ${ID}`,
      shell: false,
    })
  })

  it('pairs it with /vector:close once the review spec has a PR', () => {
    expect(nextStepRows(card('review', ID, PR))[1]).toEqual({
      label: 'Inside Claude Code',
      command: `/vector:close ${ID}`,
      shell: false,
    })
  })

  it('keeps only the terminal row for a closed spec', () => {
    const rows = nextStepRows(card('closed'))
    expect(rows).toHaveLength(1)
    expect(rows[0].shell).toBe(true)
  })

  it('marks exactly one row as a shell line', () => {
    expect(nextStepRows(card('in-progress')).filter((row) => row.shell)).toHaveLength(1)
  })

  it('builds the open command without a leading slash', () => {
    expect(openCommandFor(ID)).toBe('vector open add-dark-mode')
    expect(openCommandFor(ID).startsWith('/')).toBe(false)
  })
})
