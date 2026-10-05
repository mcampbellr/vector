import { describe, expect, it } from 'vitest'
import { shortTicketRef } from './shortTicketRef'

describe('shortTicketRef', () => {
  it('reduces an owner/repo#number reference to the number', () => {
    expect(shortTicketRef('mcampbellr/cdr-monorepo#174')).toBe('#174')
  })

  it('prefixes a bare number, keeping its leading zeros', () => {
    expect(shortTicketRef('004')).toBe('#004')
  })

  it('keeps a provider key that is already short whole', () => {
    expect(shortTicketRef('MH-1814')).toBe('MH-1814')
    expect(shortTicketRef('ENRI-C1')).toBe('ENRI-C1')
  })

  it('ignores trailing whitespace around the number', () => {
    expect(shortTicketRef('owner/repo#299  ')).toBe('#299')
  })
})
