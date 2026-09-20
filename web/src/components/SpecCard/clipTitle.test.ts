import { describe, expect, it } from 'vitest'
import { clipTitle } from './clipTitle'

describe('clipTitle', () => {
  it('leaves a title that fits within the budget untouched', () => {
    expect(clipTitle('Remote consent prefill link', 62)).toBe('Remote consent prefill link')
  })

  it('cuts at a word boundary and appends an ellipsis', () => {
    const clipped = clipTitle(
      'Notas clínicas: ventana de edición de 24h tras completar la cita + autoguardado',
      54,
    )
    expect(clipped.endsWith('…')).toBe(true)
    expect(clipped.length).toBeLessThanOrEqual(55)
    expect(clipped).not.toContain('autoguardado')
    // The boundary is a whole word — no dangling partial word before the dots.
    expect(clipped.slice(0, -1).split(' ').at(-1)).toBe('tras')
  })

  it('cuts right after a word longer than 18 chars, which eats a whole line', () => {
    expect(
      clipTitle('App móvil: cablear PersistQueryClientProvider — la caché offline nunca se conecta', 54),
    ).toBe('App móvil: cablear PersistQueryClientProvider…')
  })

  // Regression: the design's own clip() applies the long-word rule before the
  // length check, so these — package names, file names, identifiers, i.e. most
  // of this repo's titles — lost their verb and object despite fitting easily.
  it('does not truncate a title that fits just because it holds a long word', () => {
    for (const title of [
      'Add google-services.json to the Android build',
      'Bump @tanstack/react-query to v6',
      'Fix PersistQueryClientProvider wiring',
      'Refactor useCommandPaletteTrigger hook',
    ]) {
      expect(clipTitle(title, 62)).toBe(title)
    }
  })

  it('keeps a long word that is the last one instead of cutting to nothing', () => {
    expect(clipTitle('Ship google-services.json', 62)).toBe('Ship google-services.json')
  })

  it('clips mid-word when the last space is too early to be worth honouring', () => {
    // The only space sits at index 1, so honouring it would throw away 59 of
    // the 60 available characters: the cut happens mid-word instead.
    const clipped = clipTitle(`a ${'b'.repeat(80)}`, 60)
    expect(clipped).toBe(`a ${'b'.repeat(58)}…`)
  })

  it('strips trailing punctuation before the ellipsis', () => {
    expect(clipTitle('Bound staff appointment loads, and coalesce reconnect recovery', 30)).toBe(
      'Bound staff appointment…',
    )
  })
})
