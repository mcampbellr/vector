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
// header. The icon shows the *current* mode, not the next one, and the tooltip
// says where the click leads — one button instead of a three-slot segmented
// control that does not fit a 56px header. It drives the ThemeProvider only —
// no styling logic.
export function ThemeControl() {
  const { mode, setMode } = useTheme()
  const next = NEXT_MODE[mode]
  const label = `${MODE_LABEL[mode]} · click → ${MODE_LABEL[next]}`

  return (
    <button
      type="button"
      className={styles.themeControl}
      onClick={() => setMode(next)}
      aria-label={`Theme: ${label}`}
      title={label}
    >
      {mode === 'light' && <Sun size={14} strokeWidth={2} />}
      {mode === 'dark' && <Moon size={14} strokeWidth={2} />}
      {mode === 'system' && <Monitor size={14} strokeWidth={2} />}
    </button>
  )
}
