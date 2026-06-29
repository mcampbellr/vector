import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  SIZE_STORAGE_KEY,
  clampSize,
  loadPersistedSize,
  savePersistedSize,
  type ModalSize,
} from './FilePreviewModal'

// happy-dom defaults: innerWidth 1024, innerHeight 768.
// → ceiling ~972.8 x 729.6, default floor 720 x 660.48.
const FLOOR: ModalSize = { width: 720, height: 768 * 0.86 }

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  localStorage.clear()
})

describe('clampSize', () => {
  it('returns a value below the floor clamped up to the floor', () => {
    const result = clampSize({ width: 100, height: 100 }, FLOOR)
    expect(result.width).toBe(FLOOR.width)
    expect(result.height).toBeCloseTo(FLOOR.height)
  })

  it('returns a value above the ceiling clamped down to the ceiling', () => {
    const result = clampSize({ width: 5000, height: 5000 }, FLOOR)
    expect(result.width).toBeCloseTo(1024 * 0.95)
    expect(result.height).toBeCloseTo(768 * 0.95)
  })

  it('passes a value within bounds through unchanged', () => {
    const result = clampSize({ width: 800, height: 700 }, FLOOR)
    expect(result).toEqual({ width: 800, height: 700 })
  })
})

describe('loadPersistedSize', () => {
  it('returns the stored size when valid and within bounds', () => {
    savePersistedSize({ width: 850, height: 700 })
    expect(loadPersistedSize(FLOOR)).toEqual({ width: 850, height: 700 })
  })

  it('returns null when nothing is stored', () => {
    expect(loadPersistedSize(FLOOR)).toBeNull()
  })

  it('returns null on malformed JSON without throwing', () => {
    localStorage.setItem(SIZE_STORAGE_KEY, '{ not json')
    expect(loadPersistedSize(FLOOR)).toBeNull()
  })

  it('returns null when fields are missing or the wrong type', () => {
    localStorage.setItem(SIZE_STORAGE_KEY, JSON.stringify({ width: '720' }))
    expect(loadPersistedSize(FLOOR)).toBeNull()
  })

  it('returns null on non-finite numbers', () => {
    localStorage.setItem(SIZE_STORAGE_KEY, JSON.stringify({ width: Infinity, height: 700 }))
    expect(loadPersistedSize(FLOOR)).toBeNull()
  })

  it('re-clamps a stored size that exceeds the current viewport', () => {
    localStorage.setItem(SIZE_STORAGE_KEY, JSON.stringify({ width: 5000, height: 5000 }))
    const result = loadPersistedSize(FLOOR)
    expect(result?.width).toBeCloseTo(1024 * 0.95)
    expect(result?.height).toBeCloseTo(768 * 0.95)
  })
})
