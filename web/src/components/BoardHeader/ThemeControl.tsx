import { Monitor, Moon, Sun } from 'lucide-react'
import { useTheme } from '../../context/ThemeContext'
import type { ThemeMode } from '../../context/themeResolve'
import styles from './BoardHeader.module.css'

// Cycle order through the tri-state control.
const NEXT_MODE: Record<ThemeMode, ThemeMode> = {
  light: 'dark',
  dark: 'system',
  system: 'light',
}

const MODE_LABEL: Record<ThemeMode, string> = {
  light: 'Light',
  dark: 'Dark',
  system: 'System',
}

// ThemeControl is the tri-state Light → Dark → System cycle button in the board
// header: one icon reflecting the current mode, an accessible name announcing it,
// and a visible focus ring. It drives the ThemeProvider only — no styling logic.
export function ThemeControl() {
  const { mode, setMode } = useTheme()
  const label = `Theme: ${MODE_LABEL[mode]} (click to change)`

  return (
    <button
      type="button"
      className={styles.themeControl}
      onClick={() => setMode(NEXT_MODE[mode])}
      aria-label={label}
      title={label}
    >
      {mode === 'light' && <Sun size={16} strokeWidth={2} />}
      {mode === 'dark' && <Moon size={16} strokeWidth={2} />}
      {mode === 'system' && <Monitor size={16} strokeWidth={2} />}
    </button>
  )
}
