import { afterEach, describe, expect, it, vi } from 'vitest'
import { copyText } from './copyText'

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard')

function setClipboard(value: unknown) {
  Object.defineProperty(navigator, 'clipboard', { value, configurable: true, writable: true })
}

afterEach(() => {
  if (originalClipboard) Object.defineProperty(navigator, 'clipboard', originalClipboard)
  else setClipboard(undefined)
  vi.restoreAllMocks()
  document.body.innerHTML = ''
})

describe('copyText', () => {
  it('uses the async Clipboard API when the page is a secure context', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    setClipboard({ writeText })

    await expect(copyText('/vector:apply add-dark-mode')).resolves.toBe(true)
    expect(writeText).toHaveBeenCalledWith('/vector:apply add-dark-mode')
  })

  // The board served over plain HTTP from `vector serve --host <ip>` has no
  // navigator.clipboard at all; without the fallback every copy is a silent no-op.
  it('falls back to execCommand when navigator.clipboard is absent', async () => {
    setClipboard(undefined)
    const execCommand = vi.fn().mockReturnValue(true)
    document.execCommand = execCommand as unknown as typeof document.execCommand

    await expect(copyText('fix-raw-tags')).resolves.toBe(true)
    expect(execCommand).toHaveBeenCalledWith('copy')
  })

  it('falls back to execCommand when the async write is rejected', async () => {
    setClipboard({ writeText: vi.fn().mockRejectedValue(new Error('denied')) })
    const execCommand = vi.fn().mockReturnValue(true)
    document.execCommand = execCommand as unknown as typeof document.execCommand

    await expect(copyText('fix-raw-tags')).resolves.toBe(true)
    expect(execCommand).toHaveBeenCalledWith('copy')
  })

  it('reports failure rather than letting a caller claim a copy that never happened', async () => {
    setClipboard(undefined)
    document.execCommand = vi.fn().mockReturnValue(false) as unknown as typeof document.execCommand

    await expect(copyText('fix-raw-tags')).resolves.toBe(false)
  })

  it('leaves no scratch field behind in the document', async () => {
    setClipboard(undefined)
    document.execCommand = vi.fn().mockReturnValue(true) as unknown as typeof document.execCommand

    await copyText('fix-raw-tags')
    expect(document.querySelector('textarea')).toBeNull()
  })
})
