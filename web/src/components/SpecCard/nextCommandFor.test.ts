import { describe, expect, it } from 'vitest'
import type { Card, PullRequest } from '../../types/board'
import { nextCommandFor } from './nextCommandFor'

const pr: PullRequest = { url: 'https://github.com/o/r/pull/7', number: 7, draft: false, openedAt: '2026-06-27T00:00:00Z' }

describe('nextCommandFor', () => {
  const cases: Array<{ name: string; card: Pick<Card, 'status' | 'id' | 'pr'>; want: string | null }> = [
    { name: 'open → apply', card: { status: 'open', id: 'x' }, want: '/vector:apply x' },
    { name: 'in-progress → apply', card: { status: 'in-progress', id: 'x' }, want: '/vector:apply x' },
    { name: 'needs-attention → apply', card: { status: 'needs-attention', id: 'x' }, want: '/vector:apply x' },
    { name: 'in-progress with a PR still → apply', card: { status: 'in-progress', id: 'x', pr }, want: '/vector:apply x' },
    { name: 'review without a PR → ship', card: { status: 'review', id: 'x' }, want: '/vector:ship x' },
    { name: 'review with a PR → close', card: { status: 'review', id: 'x', pr }, want: '/vector:close x' },
    { name: 'review with a draft PR → close', card: { status: 'review', id: 'x', pr: { ...pr, draft: true } }, want: '/vector:close x' },
    { name: 'closed → null', card: { status: 'closed', id: 'x' }, want: null },
    { name: 'closed with a PR → null', card: { status: 'closed', id: 'x', pr }, want: null },
  ]

  for (const { name, card, want } of cases) {
    it(name, () => {
      expect(nextCommandFor(card)).toBe(want)
    })
  }
})
