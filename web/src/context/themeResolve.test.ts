import { describe, expect, it } from 'vitest'
import { parseStoredMode, resolveTheme, type ThemeMode } from './themeResolve'

describe('resolveTheme', () => {
  it('follows the OS only in system mode', () => {
    expect(resolveTheme('system', true)).toBe('dark')
    expect(resolveTheme('system', false)).toBe('light')
  })

  it('ignores prefersDark for explicit modes', () => {
    expect(resolveTheme('light', true)).toBe('light')
    expect(resolveTheme('light', false)).toBe('light')
    expect(resolveTheme('dark', true)).toBe('dark')
    expect(resolveTheme('dark', false)).toBe('dark')
  })
})

describe('parseStoredMode', () => {
  it('passes through every valid mode', () => {
    const modes: ThemeMode[] = ['light', 'dark', 'system']
    for (const mode of modes) {
      expect(parseStoredMode(mode)).toBe(mode)
    }
  })

  it('falls back to system for null and garbage', () => {
    expect(parseStoredMode(null)).toBe('system')
    expect(parseStoredMode('')).toBe('system')
    expect(parseStoredMode('Dark')).toBe('system')
    expect(parseStoredMode('blue')).toBe('system')
  })
})
