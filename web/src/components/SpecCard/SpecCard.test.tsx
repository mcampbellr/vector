import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Card } from '../../types/board'
import { SpecCard } from './SpecCard'

afterEach(cleanup)

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
        onSelect={() => {}}
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
        onSelect={() => {}}
      />,
    )

    expect(screen.queryByText('Dependency')).toBeNull()
    expect(screen.getByTitle('waiting on a decision')).toBeTruthy()
  })

  it('falls back to attentionReason when the structured fields are absent', () => {
    render(
      <SpecCard
        card={makeCard({ status: 'needs-attention', attentionReason: 'blocked on the DTO rename' })}
        onSelect={() => {}}
      />,
    )

    expect(screen.getByText('blocked on the DTO rename')).toBeTruthy()
    // No category label on a purely-legacy card.
    for (const label of ['Dependency', 'Env', 'Decision', 'External', 'Other']) {
      expect(screen.queryByText(label)).toBeNull()
    }
  })

  it('renders nothing attention-related when the card is not blocked', () => {
    render(<SpecCard card={makeCard({})} onSelect={() => {}} />)
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
        onSelect={() => {}}
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
        onSelect={() => {}}
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
      render(<SpecCard card={makeCard({ priority })} onSelect={() => {}} />)
      expect(screen.getByTitle(`Priority: ${priority === 'urgent' ? 'Urgent' : 'High'}`)).toBeTruthy()
      cleanup()
    }

    for (const priority of ['normal', 'low'] as const) {
      render(<SpecCard card={makeCard({ priority })} onSelect={() => {}} />)
      expect(screen.queryByTitle(/^Priority: /)).toBeNull()
      cleanup()
    }
  })

  it('drops the flag and the verb on a closed card — it is no longer a call to action', () => {
    render(<SpecCard card={makeCard({ status: 'closed', priority: 'urgent' })} onSelect={() => {}} />)

    expect(screen.queryByTitle('Priority: Urgent')).toBeNull()
    expect(screen.queryByLabelText(/^Copy next command/)).toBeNull()
  })

  it('collapses the next command to its verb, keeping the full line in the tooltip', () => {
    render(<SpecCard card={makeCard({ status: 'review', id: 'fix-raw-tags' })} onSelect={() => {}} />)

    const verb = screen.getByTitle('/vector:close fix-raw-tags')
    expect(verb.textContent).toBe('close')
  })

  it('renders the quick-win glyph with no label', () => {
    render(<SpecCard card={makeCard({ quickWin: true })} onSelect={() => {}} />)

    const glyph = screen.getByLabelText('Quick win')
    expect(glyph).toBeTruthy()
    expect(glyph.textContent).toBe('')
  })

  it('omits the quick-win glyph when quickWin is absent', () => {
    render(<SpecCard card={makeCard({})} onSelect={() => {}} />)

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
    render(<SpecCard card={makeCard({ status: 'review', id: 'fix-raw-tags' })} onSelect={onSelect} />)

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
    render(<SpecCard card={card} onSelect={onSelect} />)

    fireEvent.keyDown(screen.getByRole('button', { name: `Open details for ${card.title}` }), {
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
        onSelect={() => {}}
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
        onSelect={() => {}}
      />,
    )

    const summary = screen.getByTitle('staging snapshot is stale')
    expect(summary.className).not.toMatch(/attentionSummaryLegacy/)
  })
})

describe('SpecCard artifact meter', () => {
  it('is always present, unlit, when the spec carries no artifacts', () => {
    render(<SpecCard card={makeCard({})} onSelect={() => {}} />)

    expect(screen.getByLabelText('Artifacts: no artifacts yet')).toBeTruthy()
  })

  it('names the missing artifact when only some are present', () => {
    render(
      <SpecCard
        card={makeCard({ artifacts: { proposal: true, design: true, tasks: false } })}
        onSelect={() => {}}
      />,
    )

    expect(screen.getByLabelText('Artifacts: proposal · design — no tasks')).toBeTruthy()
  })
})
