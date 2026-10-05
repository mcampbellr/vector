import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FilePreviewError } from './FilePreviewError'

afterEach(cleanup)

describe('FilePreviewError', () => {
  it('shows a terminal not-available state without Retry on not-found', () => {
    render(<FilePreviewError kind="not-found" message="artifact not found" onRetry={vi.fn()} />)
    expect(screen.getByText(/file not available/)).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
  })

  it('names the server and offers Retry when unreachable', () => {
    const onRetry = vi.fn()
    render(<FilePreviewError kind="unreachable" message="Failed to fetch" onRetry={onRetry} />)
    expect(screen.getByText(/could not reach the vector server/)).toBeTruthy()
    screen.getByRole('button', { name: 'Retry' }).click()
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('keeps the server message for other failures', () => {
    render(<FilePreviewError kind="failed" message="could not read artifact" onRetry={vi.fn()} />)
    expect(screen.getByText('could not load file: could not read artifact')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy()
  })
})
