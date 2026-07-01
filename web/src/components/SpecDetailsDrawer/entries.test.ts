import { describe, expect, it } from 'vitest'
import type { Card } from '../../types/board'
import { basename, entriesFor } from './entries'

// makeCard builds a minimal Card with sane defaults; override per test.
function makeCard(overrides: Partial<Card>): Card {
  return {
    id: 'spec-id',
    title: 'A spec',
    status: 'draft',
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

describe('entriesFor', () => {
  it('lists the authored spec doc for a draft card with no OpenSpec artifacts', () => {
    const entries = entriesFor(makeCard({ specDoc: '.vector/specs/my-spec/spec.md' }))
    expect(entries).toEqual([{ key: 'spec', label: 'spec.md' }])
  })

  // Regression: a non-draft card carries OpenSpec artifacts, but specDoc still
  // points at the authored spec (propose never rewrites it). The spec doc must
  // stay listed alongside the OpenSpec artifacts — they are distinct files.
  it('keeps the spec doc for a non-draft card that also has OpenSpec artifacts', () => {
    const entries = entriesFor(
      makeCard({
        status: 'in-progress',
        specDoc: '.vector/specs/my-spec/spec.md',
        hasOpenSpec: true,
        artifacts: { proposal: true, design: true, tasks: true },
      }),
    )
    expect(entries).toEqual([
      { key: 'spec', label: 'spec.md' },
      { key: 'proposal', label: 'proposal.md' },
      { key: 'design', label: 'design.md' },
      { key: 'tasks', label: 'tasks.md' },
    ])
  })

  it('lists only the present OpenSpec artifacts', () => {
    const entries = entriesFor(
      makeCard({
        status: 'open',
        specDoc: 'openspec/specs/x/spec.md',
        hasOpenSpec: true,
        artifacts: { proposal: true, design: false, tasks: true },
      }),
    )
    expect(entries.map((entry) => entry.key)).toEqual(['spec', 'proposal', 'tasks'])
  })

  it('returns no entries when there is no spec doc and no artifacts', () => {
    expect(entriesFor(makeCard({}))).toEqual([])
  })

  it('adds a download entry per sketch', () => {
    const entries = entriesFor(
      makeCard({
        specDoc: '.vector/specs/my-spec/spec.md',
        sketches: [{ name: 'board.excalidraw', createdAt: '2026-06-27T00:00:00Z' }],
      }),
    )
    expect(entries).toEqual([
      { key: 'spec', label: 'spec.md' },
      { key: 'sketch', label: 'board.excalidraw', download: true },
    ])
  })

  it('emits one entry per sketch when several are attached', () => {
    const entries = entriesFor(
      makeCard({
        sketches: [
          { name: 'board.excalidraw', createdAt: '2026-06-27T00:00:00Z' },
          { name: 'drawer.excalidraw', createdAt: '2026-06-27T01:00:00Z' },
        ],
      }),
    )
    expect(entries).toEqual([
      { key: 'sketch', label: 'board.excalidraw', download: true },
      { key: 'sketch', label: 'drawer.excalidraw', download: true },
    ])
  })

  it('adds no sketch entries when the card has none', () => {
    const entries = entriesFor(makeCard({ specDoc: '.vector/specs/my-spec/spec.md' }))
    expect(entries.some((entry) => entry.key === 'sketch')).toBe(false)
  })
})

describe('basename', () => {
  it('returns the trailing path segment', () => {
    expect(basename('.vector/specs/my-spec/spec.md')).toBe('spec.md')
  })

  it('falls back to spec.md for an empty pointer', () => {
    expect(basename('')).toBe('spec.md')
  })
})
