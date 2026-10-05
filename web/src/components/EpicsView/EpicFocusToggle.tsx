import { Pin } from 'lucide-react'
import { setEpicFocus } from '../../api/boardWrites'
import { useWriteAction } from '../../api/useWriteAction'
import type { EpicSummary } from '../../types/board'
import styles from './EpicsView.module.css'

interface EpicFocusToggleProps {
  epic: EpicSummary
}

// EpicFocusToggle pins an epic (POST /api/epics/{id}/focus): every non-closed
// spec of a focused epic inherits focus — derived by the server, never copied
// onto the specs — so it sorts first in its column and in /vector:apply.
// Unfocusing the epic leaves individually focused specs focused. It renders
// from the board's `focus` field only; the SSE push re-renders it.
export function EpicFocusToggle({ epic }: EpicFocusToggleProps) {
  const { run, pending, error } = useWriteAction(setEpicFocus)
  const focused = epic.focus === true
  const label = focused ? `Unfocus epic ${epic.title}` : `Focus epic ${epic.title}`
  const hint = focused
    ? 'Focused epic: its open specs sort first in their column and in /vector:apply. Click to unfocus (individually focused specs stay focused).'
    : 'Focus this epic: all its open specs inherit focus and sort first.'

  return (
    <>
      <button
        type="button"
        className={`${styles.iconButton}${focused ? ` ${styles.iconButtonOn}` : ''}`}
        aria-pressed={focused}
        aria-label={label}
        title={hint}
        disabled={pending}
        aria-busy={pending || undefined}
        onClick={() => void run(epic.id, !focused)}
      >
        <Pin size={12} strokeWidth={2} fill={focused ? 'currentColor' : 'none'} />
      </button>
      {error && (
        <span className={styles.formError} role="alert">
          {error}
        </span>
      )}
    </>
  )
}
