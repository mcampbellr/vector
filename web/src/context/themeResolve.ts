// Pure theme-resolution logic — no React, no DOM. Kept separate from
// ThemeContext so the only branching logic is unit-testable in the DOM-free
// Vitest setup (mirrors the SpecDetailsDrawer/entries.ts pattern).

export type ThemeMode = 'light' | 'dark' | 'system'

/** The concrete theme actually applied to the document. */
export type ResolvedTheme = 'light' | 'dark'

const THEME_MODES: ThemeMode[] = ['light', 'dark', 'system']

/** Resolves a mode to the concrete theme. `system` follows the OS
 *  (`prefersDark`); explicit modes ignore it. */
export function resolveTheme(mode: ThemeMode, prefersDark: boolean): ResolvedTheme {
  if (mode === 'system') return prefersDark ? 'dark' : 'light'
  return mode
}

/** Validates a persisted value; anything outside the enum falls back to
 *  `system` (corrupt/absent storage must not crash the app). */
export function parseStoredMode(raw: string | null): ThemeMode {
  return raw !== null && (THEME_MODES as string[]).includes(raw) ? (raw as ThemeMode) : 'system'
}
