import { useCallback, useEffect, useRef, useState } from 'react'
import { copyText } from './copyText'

/**
 * Copy-and-confirm for the board's copy affordances: the card's slug and verb
 * and the drawer's slug and command list all behave identically — copy, then
 * replace the datum with "copied" for a moment, in place, with no global toast.
 *
 * `copied` is gated on the copy actually succeeding (see `copyText`), so the
 * board never claims it copied something it did not. The reset timer is tracked
 * so a second click restarts it instead of letting the first one cut the second
 * confirmation short, and it is cleared on unmount — board cards unmount on
 * their own whenever the SSE stream pushes a new board.
 */
export function useCopyFeedback(resetAfterMs = 1500) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current)
    },
    [],
  )

  const copy = useCallback(
    (text: string) => {
      copyText(text)
        .then((ok) => {
          if (!ok) return
          setCopied(true)
          if (timer.current) clearTimeout(timer.current)
          timer.current = setTimeout(() => setCopied(false), resetAfterMs)
        })
        .catch(() => {
          // copyText swallows its own failures and reports them as `false`; this
          // only guards against an unexpected throw leaving an unhandled rejection.
        })
    },
    [resetAfterMs],
  )

  return { copied, copy }
}
