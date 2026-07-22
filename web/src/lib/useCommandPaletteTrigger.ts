import { useCallback, useEffect, useRef, useState } from 'react'

interface CommandPaletteTrigger {
  isOpen: boolean
  open: () => void
  close: () => void
}

// isTextEntrySurface guards the `/` shortcut: while the user is typing in any
// input, textarea or contenteditable (including the palette's own input), the
// key must insert a literal slash instead of opening the palette.
function isTextEntrySurface(element: Element | null): boolean {
  if (!(element instanceof HTMLElement)) return false
  return (
    element.tagName === 'INPUT' ||
    element.tagName === 'TEXTAREA' ||
    element.isContentEditable === true
  )
}

// useCommandPaletteTrigger owns the palette's open/closed state and its `/`
// shortcut: one window keydown listener (cleaned up on unmount), plus focus
// capture on open and restore on close so keyboard users land back where they
// were. No filtering or rendering logic lives here.
export function useCommandPaletteTrigger(): CommandPaletteTrigger {
  const [isOpen, setIsOpen] = useState(false)
  const previousFocusRef = useRef<HTMLElement | null>(null)

  const open = useCallback(() => {
    previousFocusRef.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null
    setIsOpen(true)
  }, [])

  const close = useCallback(() => {
    setIsOpen(false)
    const previous = previousFocusRef.current
    if (previous && previous.isConnected) previous.focus()
  }, [])

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key !== '/') return
      const target = event.target instanceof Element ? event.target : null
      if (isTextEntrySurface(target) || isTextEntrySurface(document.activeElement)) return
      event.preventDefault()
      open()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [open])

  return { isOpen, open, close }
}
