/**
 * Copies text to the clipboard, resolving to whether it actually worked.
 *
 * `navigator.clipboard` is exposed only in a secure context — HTTPS or
 * localhost. `vector serve --host <lan-or-tailnet-ip>` exists precisely so the
 * board can be opened from another machine, and that is always plain HTTP, so
 * the async API is simply absent there. The board's whole interaction model is
 * "the datum is the copy button" (the slug and the verb), so falling through to
 * the legacy `execCommand` path is the difference between copying working and
 * silently doing nothing.
 *
 * Callers must gate their "copied" confirmation on the returned boolean: the
 * board must never claim it copied something it did not.
 */
export async function copyText(text: string): Promise<boolean> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // Denied permission, or an API present but unusable — try the legacy path
      // before giving up.
    }
  }
  return copyViaExecCommand(text)
}

function copyViaExecCommand(text: string): boolean {
  const field = document.createElement('textarea')
  field.value = text
  // Readonly keeps the iOS keyboard shut; the 1px fixed box keeps the field out
  // of view without the scroll jump an off-screen focused element causes.
  field.setAttribute('readonly', '')
  field.setAttribute('aria-hidden', 'true')
  field.style.cssText =
    'position:fixed;top:0;left:0;width:1px;height:1px;padding:0;border:none;opacity:0'
  document.body.appendChild(field)

  const selection = document.getSelection()
  const previousRange = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null
  // select() focuses the scratch field in real engines; without restoring this,
  // a keyboard user who copies loses their place and Tab restarts from the top.
  const previouslyFocused = document.activeElement

  try {
    field.select()
    field.setSelectionRange(0, text.length)
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    field.remove()
    // Restore whatever the user had selected/focused before we hijacked it.
    if (selection && previousRange) {
      selection.removeAllRanges()
      selection.addRange(previousRange)
    }
    if (previouslyFocused instanceof HTMLElement) previouslyFocused.focus()
  }
}
