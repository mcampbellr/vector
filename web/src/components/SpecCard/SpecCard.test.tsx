import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Card, PullRequest } from '../../types/board'
import { SpecCard } from './SpecCard'

afterEach(cleanup)

// A fixed tick for every render: the card's age is derived, not live, so the
// tests pin `now` instead of leaning on the clock. Three days after the
// fixture's updatedAt, which keeps the age quiet (`3d`, no escalation) and out
// of the way of the assertions below.
const NOW = Date.parse('2026-06-30T00:00:00Z')

// makeCard builds a minimal Card; override per test.
function makeCard(overrides: Partial<Card>): Card {
  return {
    id: 'spec-id',
    title: 'A spec',
    status: 'in-progress',
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

describe('SpecCard needs-attention', () => {
  it('renders the category label and a truncatable summary with a full title', () => {
    render(
      <SpecCard
        card={makeCard({
          status: 'needs-attention',
          attentionCategory: 'dependency',
          attentionSummary: 'Zoho api_names pending settings-read credentials',
          attentionReason: 'Zoho api_names pending settings-read credentials',
        })}
        now={NOW} onSelect={() => {}}
      />,
    )

    expect(screen.getByText('Dependency')).toBeTruthy()
    const summary = screen.getByTitle('Zoho api_names pending settings-read credentials')
    expect(summary.textContent).toContain('Zoho api_names pending')
  })

  it('omits the label for an unknown/absent category but still shows the summary', () => {
    render(
      <SpecCard
        card={makeCard({ status: 'needs-attention', attentionSummary: 'waiting on a decision' })}
        now={NOW} onSelect={() => {}}
      />,
    )

    expect(screen.queryByText('Dependency')).toBeNull()
    expect(screen.getByTitle('waiting on a decision')).toBeTruthy()
  })

  it('falls back to attentionReason when the structured fields are absent', () => {
    render(
      <SpecCard
        card={makeCard({ status: 'needs-attention', attentionReason: 'blocked on the DTO rename' })}
        now={NOW} onSelect={() => {}}
      />,
    )

    expect(screen.getByText('blocked on the DTO rename')).toBeTruthy()
    // No category label on a purely-legacy card.
    for (const label of ['Dependency', 'Env', 'Decision', 'External', 'Other']) {
      expect(screen.queryByText(label)).toBeNull()
    }
  })

  it('renders nothing attention-related when the card is not blocked', () => {
    render(<SpecCard card={makeCard({})} now={NOW} onSelect={() => {}} />)
    expect(screen.queryByText('Dependency')).toBeNull()
  })
})

describe('SpecCard ticket ref', () => {
  it('keeps a short provider key whole alongside a very long title', () => {
    render(
      <SpecCard
        card={makeCard({
          title:
            'Reparar clipping del badge de ticket en el encabezado de la tarjeta de spec del board kanban',
          ticket: {
            provider: 'linear',
            key: 'MH-1814',
            url: 'https://linear.app/acme/issue/MH-1814',
          },
        })}
        now={NOW} onSelect={() => {}}
      />,
    )

    // The key exists as a single, complete text node — not split at the hyphen
    // (MH-) nor clipped away. It sits at a fixed intrinsic width, so it can
    // never squeeze the title the way the old badge did.
    const ref = screen.getByTitle('https://linear.app/acme/issue/MH-1814')
    expect(ref.textContent).toBe('MH-1814')
  })

  it('collapses an owner/repo#number key to the number, with the title clipped to two lines', () => {
    const title = 'Published treatment mentions are not actionable, and never were'
    render(
      <SpecCard
        card={makeCard({
          title,
          ticket: {
            provider: 'github',
            key: 'mcampbellr/cdr-monorepo#174',
            url: 'https://github.com/mcampbellr/cdr-monorepo/issues/174',
          },
        })}
        now={NOW} onSelect={() => {}}
      />,
    )

    expect(screen.getByText('#174')).toBeTruthy()
    // The untruncated title stays reachable through the tooltip.
    const heading = screen.getByRole('heading', { level: 3 })
    expect(heading.getAttribute('title')).toBe(title)
    expect(heading.textContent!.endsWith('…')).toBe(true)
  })
})

describe('SpecCard status row', () => {
  it('prints the flag only for urgent and high', () => {
    for (const priority of ['urgent', 'high'] as const) {
      render(<SpecCard card={makeCard({ priority })} now={NOW} onSelect={() => {}} />)
      expect(screen.getByTitle(`Priority: ${priority === 'urgent' ? 'Urgent' : 'High'}`)).toBeTruthy()
      cleanup()
    }

    for (const priority of ['normal', 'low'] as const) {
      render(<SpecCard card={makeCard({ priority })} now={NOW} onSelect={() => {}} />)
      expect(screen.queryByTitle(/^Priority: /)).toBeNull()
      cleanup()
    }
  })

  it('drops the flag and the verb on a closed card — it is no longer a call to action', () => {
    render(<SpecCard card={makeCard({ status: 'closed', priority: 'urgent' })} now={NOW} onSelect={() => {}} />)

    expect(screen.queryByTitle('Priority: Urgent')).toBeNull()
    expect(screen.queryByLabelText(/^Copy next command/)).toBeNull()
  })

  it('collapses the next command to its verb, keeping the full line in the tooltip', () => {
    render(<SpecCard card={makeCard({ status: 'review', id: 'fix-raw-tags' })} now={NOW} onSelect={() => {}} />)

    const verb = screen.getByTitle('/vector:ship fix-raw-tags')
    expect(verb.textContent).toBe('ship')
  })

  it('switches the review verb to close once a PR is recorded', () => {
    const pr: PullRequest = { url: 'https://github.com/o/r/pull/7', number: 7, draft: false, openedAt: '2026-06-27T00:00:00Z' }
    render(<SpecCard card={makeCard({ status: 'review', id: 'fix-raw-tags', pr })} now={NOW} onSelect={() => {}} />)

    const verb = screen.getByTitle('/vector:close fix-raw-tags')
    expect(verb.textContent).toBe('close')
  })

  it('renders the quick-win glyph with no label', () => {
    render(<SpecCard card={makeCard({ quickWin: true })} now={NOW} onSelect={() => {}} />)

    const glyph = screen.getByLabelText('Quick win')
    expect(glyph).toBeTruthy()
    expect(glyph.textContent).toBe('')
  })

  it('omits the quick-win glyph when quickWin is absent', () => {
    render(<SpecCard card={makeCard({})} now={NOW} onSelect={() => {}} />)

    expect(screen.queryByLabelText('Quick win')).toBeNull()
  })
})

describe('SpecCard keyboard interaction', () => {
  // The card is article[role=button][tabindex=0] with two real buttons inside.
  // Without a target guard its onKeyDown calls preventDefault() on keydowns that
  // bubble up from those buttons, which cancels their native activation: Enter
  // on the slug would copy nothing and open the drawer instead.
  it('does not open the drawer when Enter reaches the card from a nested button', () => {
    const onSelect = vi.fn()
    render(<SpecCard card={makeCard({ status: 'review', id: 'fix-raw-tags' })} now={NOW} onSelect={onSelect} />)

    for (const name of [/^Copy spec id/, /^Copy next command/]) {
      const inner = screen.getByLabelText(name)
      const event = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
      inner.dispatchEvent(event)

      expect(onSelect).not.toHaveBeenCalled()
      // The nested button's own activation must survive to run.
      expect(event.defaultPrevented).toBe(false)
    }
  })

  it('still opens the drawer on Enter over the card itself', () => {
    const onSelect = vi.fn()
    const card = makeCard({})
    render(<SpecCard card={card} now={NOW} onSelect={onSelect} />)

    fireEvent.keyDown(screen.getByRole('button', { name: new RegExp(`^Open details for ${card.title}`) }), {
      key: 'Enter',
    })
    expect(onSelect).toHaveBeenCalledWith(card)
  })
})

describe('SpecCard attention row colour', () => {
  // An unknown category renders no label, so the row must fall back to the
  // legacy red instead of ending up with neither label nor warning colour.
  it('treats an unknown category as unstructured', () => {
    render(
      <SpecCard
        card={makeCard({
          status: 'needs-attention',
          attentionCategory: 'brand-new-category' as never,
          attentionSummary: 'waiting on something new',
        })}
        now={NOW} onSelect={() => {}}
      />,
    )

    const summary = screen.getByTitle('waiting on something new')
    expect(summary.className).toMatch(/attentionSummaryLegacy/)
  })

  it('keeps a known category out of the legacy red', () => {
    render(
      <SpecCard
        card={makeCard({
          status: 'needs-attention',
          attentionCategory: 'env',
          attentionSummary: 'staging snapshot is stale',
        })}
        now={NOW} onSelect={() => {}}
      />,
    )

    const summary = screen.getByTitle('staging snapshot is stale')
    expect(summary.className).not.toMatch(/attentionSummaryLegacy/)
  })
})

describe('SpecCard artifact meter', () => {
  it('is always present, unlit, when the spec carries no artifacts', () => {
    render(<SpecCard card={makeCard({})} now={NOW} onSelect={() => {}} />)

    expect(screen.getByLabelText('Artifacts: no artifacts yet')).toBeTruthy()
  })

  it('names the missing artifact when only some are present', () => {
    render(
      <SpecCard
        card={makeCard({ artifacts: { proposal: true, design: true, tasks: false } })}
        now={NOW} onSelect={() => {}}
      />,
    )

    expect(screen.getByLabelText('Artifacts: proposal · design — no tasks')).toBeTruthy()
  })
})

describe('SpecCard focus and epic', () => {
  it('renders the epic chip with the resolved epic title', () => {
    render(
      <SpecCard
        card={makeCard({ epic: 'app-mobile' })}
        epic={{
          id: 'app-mobile',
          title: 'App Mobile',
          color: 'blue',
          total: 1,
          done: 0,
          byStatus: { 'in-progress': 1 },
          updatedAt: '2026-06-27T00:00:00Z',
        }}
        now={NOW} onSelect={() => {}}
      />,
    )

    expect(screen.getByTitle('Epic: App Mobile').textContent).toBe('App Mobile')
  })

  it('renders no epic chip for a spec without an epic', () => {
    render(<SpecCard card={makeCard({})} now={NOW} onSelect={() => {}} />)
    expect(screen.queryByTitle(/^Epic: /)).toBeNull()
  })

  it('shows a pressed focus pin on a focused card and an unpressed one otherwise', () => {
    render(<SpecCard card={makeCard({ focus: true })} now={NOW} onSelect={() => {}} />)
    expect(screen.getByRole('button', { name: 'Unfocus spec' }).getAttribute('aria-pressed')).toBe('true')
    cleanup()

    render(<SpecCard card={makeCard({})} now={NOW} onSelect={() => {}} />)
    expect(screen.getByRole('button', { name: 'Focus spec' }).getAttribute('aria-pressed')).toBe('false')
  })

  it('toggles focus without opening the drawer', () => {
    const fetchMock = vi.fn(async () => new Response('{"id":"spec-id","focus":true,"changed":true}', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)
    const onSelect = vi.fn()
    render(<SpecCard card={makeCard({})} now={NOW} onSelect={onSelect} />)

    fireEvent.click(screen.getByRole('button', { name: 'Focus spec' }))
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(onSelect).not.toHaveBeenCalled()
    vi.unstubAllGlobals()
  })

  it('badges a dropped resolution and not a done one', () => {
    const { unmount } = render(
      <SpecCard
        card={makeCard({ status: 'closed', resolution: 'duplicate', resolutionNote: 'dup of login-v2' })}
        now={NOW} onSelect={() => {}}
      />,
    )
    const badge = screen.getByLabelText('Closed as duplicate — dup of login-v2 (not counted as done)')
    expect(badge.textContent).toBe('duplicate')
    unmount()

    render(<SpecCard card={makeCard({ status: 'closed', resolution: 'done' })} now={NOW} onSelect={() => {}} />)
    expect(screen.queryByLabelText(/Closed as/)).toBeNull()
  })

  it('shows an inherited focus pin naming the epic', () => {
    render(
      <SpecCard
        card={makeCard({ epic: 'app-mobile', focusInherited: true })}
        epic={{ id: 'app-mobile', title: 'App Mobile', total: 1, done: 0, byStatus: {}, updatedAt: '', focus: true }}
        now={NOW} onSelect={() => {}}
      />,
    )
    expect(screen.getByRole('button', { name: 'Focus spec (focus currently inherited from epic App Mobile)' })).toBeTruthy()
  })
})

// The fixture's own updatedAt. NOW sits 3 days after it, so the default age is
// a quiet `3d`; the tests that need another step move `now`, not the card.
describe('SpecCard age stamp', () => {
  const updatedAt = updatedAtFixture
  const at = (iso: string) => Date.parse(iso)

  it('prints the age in card units with the full tooltip', () => {
    render(
      <SpecCard
        card={makeCard({ updatedAt })}
        now={NOW}
        onSelect={() => {}}
      />,
    )

    const stamp = screen.getByText('3d')
    expect(stamp.getAttribute('title')?.replace(/ /g, ' ')).toContain('Updated 3 days ago · ')
  })

  it('escalates a stale card and keeps the glyph for the very stale one', () => {
    const { unmount } = render(
      <SpecCard card={makeCard({ updatedAt })} now={at('2026-07-06T00:00:00Z')} onSelect={() => {}} />,
    )
    // 9 days: stale, no glyph.
    expect(screen.getByText('1w').querySelector('svg')).toBeNull()
    unmount()

    render(
      <SpecCard card={makeCard({ updatedAt })} now={at('2026-08-07T00:00:00Z')} onSelect={() => {}} />,
    )
    // 41 days: very stale, and the only step that spends a hue.
    expect(screen.getByText('1mo').querySelector('svg')).toBeTruthy()
  })

  it('never escalates a closed card, whatever its age', () => {
    render(
      <SpecCard
        card={makeCard({ status: 'closed', updatedAt })}
        now={at('2026-08-07T00:00:00Z')}
        onSelect={() => {}}
      />,
    )

    const stamp = screen.getByText('1mo')
    expect(stamp.querySelector('svg')).toBeNull()
    expect(stamp.getAttribute('title')).not.toContain('Stale')
  })

  it('carries the age and its tone in the card label, for the keyboard path', () => {
    render(
      <SpecCard card={makeCard({ updatedAt })} now={at('2026-07-06T00:00:00Z')} onSelect={() => {}} />,
    )

    expect(
      screen.getByRole('button', { name: 'Open details for A spec, updated 9 days ago, stale' }),
    ).toBeTruthy()
  })

  it('renders no age and leaves the label untouched when the timestamp is unusable', () => {
    render(
      <SpecCard card={makeCard({ updatedAt: '0001-01-01T00:00:00Z' })} now={NOW} onSelect={() => {}} />,
    )

    // Go's zero time: the card is whole, just without the datum.
    expect(screen.getByRole('button', { name: 'Open details for A spec' })).toBeTruthy()
    const meter = screen.getByLabelText(/^Artifacts:/)
    expect(meter.className).not.toMatch(/meterAfterAge/)
  })

  it('hands the row-3 auto margin to the age when one renders', () => {
    render(<SpecCard card={makeCard({ updatedAt })} now={NOW} onSelect={() => {}} />)

    const meter = screen.getByLabelText(/^Artifacts:/)
    expect(meter.className).toMatch(/meterAfterAge/)
  })
})

describe('SpecCard row 3 fit', () => {
  it('uses the compact card formats for the estimate and the tokens', () => {
    render(
      <SpecCard
        card={makeCard({ estimateMinutes: 120, routes: 2, tokensIn: 7_000, tokensOut: 5_400 })}
        now={NOW}
        onSelect={() => {}}
      />,
    )

    // `2h`, not the drawer's `2 h`; `12k tok`, not `12.4k tok`.
    expect(screen.getByTitle('Estimate').textContent).toBe('2h')
    expect(screen.getByTitle(/Tokens spent/).textContent).toBe('12k tok')
  })

  it('sheds the tokens before anything else when the row runs out of width', () => {
    render(
      <SpecCard
        card={makeCard({
          status: 'review',
          priority: 'urgent',
          needsUat: true,
          focus: true,
          sketches: [
            { name: 'a.excalidraw', createdAt: updatedAtFixture },
            { name: 'b.excalidraw', createdAt: updatedAtFixture },
          ],
          estimateMinutes: 90,
          routes: 3,
          tokensIn: 7_000,
          tokensOut: 5_400,
        })}
        now={NOW}
        onSelect={() => {}}
      />,
    )

    expect(screen.queryByTitle(/Tokens spent/)).toBeNull()
    // The protected data and the sheddable ones above tokens all stay.
    expect(screen.getByTitle('Estimate').textContent).toBe('90m')
    expect(screen.getByLabelText('2 Excalidraw sketches attached')).toBeTruthy()
    expect(screen.getByTitle('/vector:ship spec-id')).toBeTruthy()
  })

  it('drops the uat label but keeps its glyph and its meaning', () => {
    render(
      <SpecCard
        card={makeCard({ status: 'review', needsUat: true })}
        now={NOW}
        onSelect={() => {}}
      />,
    )

    const uat = screen.getByLabelText('Requires manual UAT before closing')
    expect(uat.textContent).toBe('')
    expect(uat.querySelector('svg')).toBeTruthy()
  })
})

const updatedAtFixture = '2026-06-27T00:00:00Z'
