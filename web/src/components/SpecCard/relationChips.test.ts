import { describe, expect, it } from 'vitest'
import type { RelatedItem } from '../../types/board'
import { relationChips } from './relationChips'

describe('relationChips', () => {
  it('yields no chips for missing or empty relations', () => {
    expect(relationChips(undefined)).toEqual([])
    expect(relationChips([])).toEqual([])
  })

  it('renders one chip per relation with a spelled-out accessible label', () => {
    const related: RelatedItem[] = [
      { kind: 'spec', ref: 'add-login', source: 'blame' },
      { kind: 'ticket', ref: 'jira:ACME-7', source: 'manual' },
    ]
    const chips = relationChips(related)

    expect(chips).toHaveLength(2)
    expect(chips[0]).toEqual({
      key: 'spec:add-login',
      label: 'add-login',
      ariaLabel: 'Caused by spec add-login (blame)',
      title: 'Caused by spec add-login · blame',
    })
    expect(chips[1].label).toBe('jira:ACME-7')
    expect(chips[1].ariaLabel).toContain('ticket jira:ACME-7')
  })

  it('keys are stable per kind+ref so duplicate refs across kinds do not collide', () => {
    const chips = relationChips([
      { kind: 'spec', ref: 'x', source: 'manual' },
      { kind: 'ticket', ref: 'x', source: 'manual' },
    ])
    expect(new Set(chips.map((c) => c.key)).size).toBe(2)
  })
})
