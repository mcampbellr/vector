import type { MouseEvent } from 'react'
import { Pin, PinOff } from 'lucide-react'
import { setSpecFocus } from '../../api/boardWrites'
import { useWriteAction } from '../../api/useWriteAction'
import styles from './FocusToggle.module.css'

interface FocusToggleProps {
  specId: string
  focused: boolean
  /** Closed specs cannot be focused (the server refuses it); they can still be
   *  unfocused. */
  canFocus: boolean
  /** `card`: a compact pin in the card's status row. `drawer`: a labelled button
   *  with the full error message. */
  variant: 'card' | 'drawer'
  /** Title of the focused epic this spec inherits focus from (Card.focusInherited).
   *  Renders an outlined pin that stays visible; clicking still only sets the
   *  spec's OWN focus. */
  inheritedFrom?: string
}

// FocusToggle flips the developer's "work on this first" marker through
// POST /api/specs/{id}/focus. It renders from the board's `focus` field only —
// no optimistic copy — so what it shows is always what the state JSON says; the
// SSE push after the write re-renders it. While the request is in flight the
// button is disabled; a failure is shown inline next to it.
export function FocusToggle({ specId, focused, canFocus, variant, inheritedFrom }: FocusToggleProps) {
  const { run, pending, error } = useWriteAction(setSpecFocus)
  const inherited = !focused && inheritedFrom !== undefined
  const disabled = pending || (!focused && !canFocus)
  const label = focused
    ? 'Unfocus spec'
    : inherited
      ? `Focus spec (focus currently inherited from epic ${inheritedFrom})`
      : 'Focus spec'
  const hint = focused
    ? 'Focused: sorts first in its column and in /vector:apply. Click to unfocus.'
    : inherited
      ? `Focus inherited from epic “${inheritedFrom}”: sorts first while the epic is focused. Click to focus this spec on its own (unfocus the epic from the Epics view).`
      : canFocus
        ? 'Focus: work on this first — sorts ahead of priority in its column and in /vector:apply.'
        : 'Closed specs cannot be focused.'

  function handleClick(event: MouseEvent<HTMLButtonElement>) {
    // The card itself is clickable (it opens the drawer); the toggle must not.
    event.stopPropagation()
    void run(specId, !focused)
  }

  const stateClass = focused ? styles.on : inherited ? styles.inherited : styles.off
  const Icon = focused && variant === 'drawer' ? PinOff : Pin

  return (
    <span className={`${styles.wrapper} ${styles[variant]}`}>
      <button
        type="button"
        className={`${styles.toggle} ${stateClass}`}
        aria-pressed={focused}
        aria-label={label}
        title={hint}
        disabled={disabled}
        aria-busy={pending || undefined}
        onClick={handleClick}
      >
        <Icon
          size={variant === 'card' ? 11 : 13}
          strokeWidth={2}
          fill={focused && variant === 'card' ? 'currentColor' : 'none'}
        />
        {variant === 'drawer' && <span>{focused ? 'unfocus' : inherited ? 'focus (inherited from epic)' : 'focus'}</span>}
        {variant === 'card' && (focused || inherited) && <span>focus</span>}
      </button>
      {error && (
        <span className={styles.error} role="alert" title={error}>
          {variant === 'card' ? 'focus failed' : error}
        </span>
      )}
    </span>
  )
}
