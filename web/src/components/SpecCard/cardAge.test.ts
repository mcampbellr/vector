import { describe, expect, it } from 'vitest'
import { cardAge } from './cardAge'
import type { Status } from '../../types/board'

const NOW = Date.parse('2026-10-05T17:20:00Z')
const MIN = 60_000
const HOUR = 3_600_000
const DAY = 86_400_000

/** A spec updated `elapsed` ms before NOW. */
function at(elapsed: number, status: Status = 'in-progress') {
  return cardAge(new Date(NOW - elapsed).toISOString(), status, NOW, { timeZone: 'UTC' })
}

describe('cardAge · card text', () => {
  it('prints one unit per step', () => {
    expect(at(20 * MIN)?.text).toBe('now')
    expect(at(5 * HOUR)?.text).toBe('5h')
    expect(at(3 * DAY)?.text).toBe('3d')
    expect(at(9 * DAY)?.text).toBe('1w')
    expect(at(41 * DAY)?.text).toBe('1mo')
    expect(at(400 * DAY)?.text).toBe('1y')
  })

  it('floors at every boundary instead of rounding up', () => {
    expect(at(59 * MIN)?.text).toBe('now')
    expect(at(60 * MIN)?.text).toBe('1h')
    expect(at(23 * HOUR + 59 * MIN)?.text).toBe('23h')
    // The case that drove the floor decision: 6d 23h is 6d, never 7d.
    expect(at(6 * DAY + 23 * HOUR)?.text).toBe('6d')
    expect(at(7 * DAY)?.text).toBe('1w')
    expect(at(29 * DAY)?.text).toBe('4w')
    expect(at(30 * DAY)?.text).toBe('1mo')
    expect(at(365 * DAY)?.text).toBe('1y')
  })

  it('reads the last days before a year as 12mo, following the design logic', () => {
    // A month is 30 days flat. The design's prose said the range topped out at
    // `11mo` while its own logic floored days/30; this follows the logic.
    expect(at(364 * DAY)?.text).toBe('12mo')
  })

  it('never prints seconds or minutes, unlike the board header', () => {
    expect(at(45_000)?.text).toBe('now')
    expect(at(20 * MIN)?.text).toBe('now')
  })
})

describe('cardAge · tone', () => {
  it('stays quiet under a week and escalates past it', () => {
    expect(at(3 * DAY)?.tone).toBe('quiet')
    expect(at(6 * DAY + 23 * HOUR)?.tone).toBe('quiet')
    expect(at(7 * DAY)?.tone).toBe('stale')
    expect(at(29 * DAY)?.tone).toBe('stale')
    expect(at(30 * DAY)?.tone).toBe('very-stale')
  })

  it('escalates for every in-flight status', () => {
    for (const status of ['in-progress', 'needs-attention', 'review'] as const) {
      expect(at(41 * DAY, status)?.tone).toBe('very-stale')
    }
  })

  it('never escalates an open or closed card, at any age', () => {
    // Backlog waiting is its job, and finished work does not rot.
    const open = at(41 * DAY, 'open')
    expect(open?.text).toBe('1mo')
    expect(open?.tone).toBe('neutral')
    expect(open?.tooltip).not.toContain('Stale')

    const closed = at(400 * DAY, 'closed')
    expect(closed?.text).toBe('1y')
    expect(closed?.tone).toBe('neutral')
    expect(closed?.tooltip).not.toContain('Stale')
  })
})

describe('cardAge · tooltip and aria lead', () => {
  it('builds the three leads', () => {
    expect(at(3 * DAY)?.tooltip).toContain('Updated 3 days ago')
    expect(at(9 * DAY)?.tooltip).toContain('Stale — updated 9 days ago')
    expect(at(41 * DAY)?.tooltip).toContain('Very stale — updated 41 days ago')
  })

  it('derives the lead from the same floored values as the card text', () => {
    // relativeTime() rounds, so reusing it here would pair `6d` with "7 days
    // ago" inside one tooltip. These three cases are exactly that trap.
    const sixDays = at(6 * DAY + 20 * HOUR)
    expect(sixDays?.text).toBe('6d')
    expect(sixDays?.relative).toBe('6 days ago')

    const almostADay = at(23 * HOUR + 40 * MIN)
    expect(almostADay?.text).toBe('23h')
    expect(almostADay?.relative).toBe('23 hr ago')

    const almostAMonth = at(29 * DAY + 14 * HOUR)
    expect(almostAMonth?.text).toBe('4w')
    expect(almostAMonth?.tooltip).toContain('Stale — updated 29 days ago')
  })

  it('keeps sub-hour precision in the lead even though the card says now', () => {
    const fresh = at(45_000)
    expect(fresh?.text).toBe('now')
    expect(fresh?.relative).toBe('45 sec ago')
    expect(fresh?.tooltip).toContain('Updated 45 sec ago')
  })

  it('singularizes one day', () => {
    expect(at(DAY)?.relative).toBe('1 day ago')
  })

  it('appends the absolute timestamp in a fixed zone', () => {
    const tooltip = at(3 * DAY)?.tooltip ?? ''
    // Recent ICU separates the meridiem with a narrow no-break space, so the
    // assertion normalizes it rather than depending on the host's ICU build.
    expect(tooltip.replace(/ /g, ' ')).toBe('Updated 3 days ago · Oct 2, 2026, 5:20 PM')
  })
})

describe('cardAge · unusable input', () => {
  it('returns null rather than printing a meaningless age', () => {
    expect(cardAge('', 'in-progress', NOW)).toBeNull()
    expect(cardAge('invalid', 'in-progress', NOW)).toBeNull()
    // Go's zero time.Time. Parseable, so it would otherwise print as `55y`.
    expect(cardAge('0001-01-01T00:00:00Z', 'in-progress', NOW)).toBeNull()
  })

  it('returns null for a non-finite now', () => {
    expect(cardAge('2026-10-01T00:00:00Z', 'in-progress', Number.NaN)).toBeNull()
    expect(cardAge('2026-10-01T00:00:00Z', 'in-progress', Number.POSITIVE_INFINITY)).toBeNull()
  })

  it('clamps a future timestamp to now instead of printing a negative age', () => {
    const skewed = cardAge(new Date(NOW + 2 * HOUR).toISOString(), 'in-progress', NOW, {
      timeZone: 'UTC',
    })
    expect(skewed?.text).toBe('now')
    expect(skewed?.tone).toBe('quiet')
  })
})
