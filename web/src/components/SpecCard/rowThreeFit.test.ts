import { describe, expect, it } from 'vitest'
import { formatCardEstimate, formatCardTokens, rowThreeFit } from './rowThreeFit'
import type { RowThreeInput } from './rowThreeFit'

// The worst case the design measured: urgent + uat + 2 sketches + 90 min +
// 12.4k tok + verb. It is the case that overflowed and clipped the verb before
// the compact formats, so it is the one the budget has to clear.
const WORST: RowThreeInput = {
  priority: 'urgent',
  status: 'review',
  quickWin: false,
  uat: true,
  focusVisible: false,
  sketchCount: 2,
  estimateMinutes: 90,
  tokens: 12_400,
  routes: 3,
  verb: 'close',
}

describe('formatCardEstimate', () => {
  it('collapses whole hours and keeps everything else in minutes', () => {
    expect(formatCardEstimate(45)).toBe('45m')
    expect(formatCardEstimate(90)).toBe('90m')
    expect(formatCardEstimate(120)).toBe('2h')
    expect(formatCardEstimate(150)).toBe('150m')
  })
})

describe('formatCardTokens', () => {
  it('steps from raw to thousands to millions', () => {
    expect(formatCardTokens(840)).toBe('840 tok')
    expect(formatCardTokens(3_100)).toBe('3.1k tok')
    expect(formatCardTokens(12_400)).toBe('12k tok')
    expect(formatCardTokens(1_200_000)).toBe('1.2M tok')
  })

  it('keeps 999_999 in thousands instead of rounding it up to a million', () => {
    // Intl's compact notation prints `1M` here, which reads as a millions-scale
    // number one token short of a million. The millions step starts at 1_000_000.
    expect(formatCardTokens(999_999)).toBe('1000k tok')
  })
})

describe('rowThreeFit', () => {
  it('fits the design worst case with nothing shed', () => {
    const fit = rowThreeFit(WORST)
    expect(fit.shed).toEqual([])
    expect(fit.showEstimate).toBe(true)
    expect(fit.showSketches).toBe(true)
    expect(fit.showTokens).toBe(true)
    expect(fit.estimateText).toBe('90m')
    expect(fit.tokensText).toBe('12k tok')
  })

  it('sheds only the tokens once the focus pin joins the worst case', () => {
    const fit = rowThreeFit({ ...WORST, focusVisible: true })
    expect(fit.shed).toEqual(['tokens'])
    expect(fit.showTokens).toBe(false)
    expect(fit.showSketches).toBe(true)
    expect(fit.showEstimate).toBe(true)
  })

  it('sheds in order: tokens, then sketches, then the estimate', () => {
    // A card width small enough that every sheddable datum has to go.
    const fit = rowThreeFit({ ...WORST, focusVisible: true, cardWidth: 150 })
    expect(fit.shed).toEqual(['tokens', 'sketches', 'estimate'])
  })

  it('never sheds a protected datum, even when the row cannot fit at all', () => {
    const fit = rowThreeFit({
      ...WORST,
      focusVisible: true,
      resolution: 'superseded',
      quickWin: true,
      cardWidth: 80,
    })
    // Everything sheddable is gone and the row still does not fit; the protected
    // data stay regardless. The fit rule narrows the row, it never hides a
    // control or an alarm.
    expect(fit.shed).toEqual(['tokens', 'sketches', 'estimate'])
    expect(fit.showTokens).toBe(false)
    expect(fit.showSketches).toBe(false)
    expect(fit.showEstimate).toBe(false)
  })

  it('claims no width for data the row does not print', () => {
    // normal priority, no uat outside review, no sketches, no estimate and no
    // routes: nothing but the verb prints, so nothing can overflow.
    const fit = rowThreeFit({
      priority: 'normal',
      status: 'open',
      quickWin: false,
      uat: true,
      focusVisible: false,
      sketchCount: 0,
      tokens: 9_999,
      routes: 0,
      verb: 'apply',
    })
    expect(fit.shed).toEqual([])
    expect(fit.tokensText).toBe('')
    expect(fit.estimateText).toBe('')
    expect(fit.showTokens).toBe(false)
  })

  it('drops the priority flag on a closed card, freeing its width', () => {
    // A closed urgent prints no flag (CardPriorityFlag), so the row that would
    // otherwise shed keeps its tokens.
    const closed = rowThreeFit({
      ...WORST,
      status: 'closed',
      focusVisible: true,
      uat: false,
      verb: null,
    })
    expect(closed.shed).toEqual([])
    expect(closed.showTokens).toBe(true)
  })
})
