import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FocusToggle } from './FocusToggle'
import { deferredResponse, jsonResponse, stubFetch } from '../../test/fetchStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe('FocusToggle', () => {
  it('posts the inverse of the current value and disables itself while pending', async () => {
    const deferred = deferredResponse()
    const { requests } = stubFetch(deferred.promise)
    render(<FocusToggle specId="add-login" focused={false} canFocus variant="drawer" />)

    const button = screen.getByRole('button', { name: 'Focus spec' })
    expect(button.getAttribute('aria-pressed')).toBe('false')
    fireEvent.click(button)

    expect(requests).toEqual([{ url: '/api/specs/add-login/focus', method: 'POST', body: { focus: true } }])
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(true))

    await act(async () => {
      deferred.resolve(jsonResponse(200, { id: 'add-login', focus: true, changed: true }))
    })
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false))
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('unfocuses a focused spec', () => {
    const { requests } = stubFetch(async () => jsonResponse(200, { id: 'x', focus: false, changed: true }))
    render(<FocusToggle specId="x" focused canFocus variant="card" />)

    const button = screen.getByRole('button', { name: 'Unfocus spec' })
    expect(button.getAttribute('aria-pressed')).toBe('true')
    expect(button.textContent).toBe('focus')
    fireEvent.click(button)
    expect(requests[0].body).toEqual({ focus: false })
  })

  it('shows the server error inline', async () => {
    stubFetch(async () => jsonResponse(409, { error: 'spec "x" is closed: only an active spec can be focused' }))
    render(<FocusToggle specId="x" focused={false} canFocus variant="drawer" />)

    fireEvent.click(screen.getByRole('button', { name: 'Focus spec' }))
    expect((await screen.findByRole('alert')).textContent).toContain('only an active spec can be focused')
  })

  it('keeps a compact message on the card with the full error in the tooltip', async () => {
    stubFetch(async () => jsonResponse(500, { error: 'disk full' }))
    render(<FocusToggle specId="x" focused={false} canFocus variant="card" />)

    fireEvent.click(screen.getByRole('button', { name: 'Focus spec' }))
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toBe('focus failed')
    expect(alert.getAttribute('title')).toBe('disk full')
  })

  it('cannot focus a closed spec', () => {
    const { fetchMock } = stubFetch(async () => jsonResponse(200, {}))
    render(<FocusToggle specId="x" focused={false} canFocus={false} variant="drawer" />)

    const button = screen.getByRole('button', { name: 'Focus spec' }) as HTMLButtonElement
    expect(button.disabled).toBe(true)
    fireEvent.click(button)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('does not let the click reach the card behind it', () => {
    stubFetch(async () => jsonResponse(200, { id: 'x', focus: true, changed: true }))
    const onCardClick = vi.fn()
    render(
      <div onClick={onCardClick}>
        <FocusToggle specId="x" focused={false} canFocus variant="card" />
      </div>,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Focus spec' }))
    expect(onCardClick).not.toHaveBeenCalled()
  })
})
