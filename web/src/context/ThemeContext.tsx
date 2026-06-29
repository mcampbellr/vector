import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { parseStoredMode, resolveTheme, type ResolvedTheme, type ThemeMode } from './themeResolve'

const STORAGE_KEY = 'vector-theme'
const DARK_QUERY = '(prefers-color-scheme: dark)'

interface ThemeContextValue {
  mode: ThemeMode
  resolved: ResolvedTheme
  setMode: (mode: ThemeMode) => void
}

const ThemeContext = createContext<ThemeContextValue | null>(null)

/** Reads the persisted mode, guarded against unavailable/throwing storage
 *  (private mode, quota). Failure degrades to `system`. */
function readStoredMode(): ThemeMode {
  try {
    return parseStoredMode(window.localStorage.getItem(STORAGE_KEY))
  } catch {
    return 'system'
  }
}

/** Current OS preference, guarded for environments without `matchMedia`. */
function readPrefersDark(): boolean {
  try {
    return window.matchMedia(DARK_QUERY).matches
  } catch {
    return false
  }
}

interface ThemeProviderProps {
  children: ReactNode
}

// ThemeProvider owns the theme mode, mirrors the resolved theme onto
// <html data-theme>, persists the choice to localStorage, and tracks the OS
// live while in `system`. All visual change happens in CSS via the data-theme
// token overrides; components stay theme-agnostic.
export function ThemeProvider({ children }: ThemeProviderProps) {
  const [mode, setModeState] = useState<ThemeMode>(readStoredMode)
  const [prefersDark, setPrefersDark] = useState<boolean>(readPrefersDark)

  const resolved = resolveTheme(mode, prefersDark)

  // Apply the resolved theme to the document so the CSS token overrides win.
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', resolved)
  }, [resolved])

  // Persist the explicit mode choice (best-effort).
  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, mode)
    } catch {
      // storage unavailable — theme still works in-memory for the session
    }
  }, [mode])

  // Track the OS theme live, but only while following the system preference.
  useEffect(() => {
    if (mode !== 'system') return
    let media: MediaQueryList
    try {
      media = window.matchMedia(DARK_QUERY)
    } catch {
      return
    }
    setPrefersDark(media.matches)
    const onChange = (event: MediaQueryListEvent) => setPrefersDark(event.matches)
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [mode])

  const setMode = useCallback((next: ThemeMode) => setModeState(next), [])

  return (
    <ThemeContext.Provider value={{ mode, resolved, setMode }}>{children}</ThemeContext.Provider>
  )
}

export function useTheme(): ThemeContextValue {
  const value = useContext(ThemeContext)
  if (value === null) {
    throw new Error('useTheme must be used within a ThemeProvider')
  }
  return value
}
