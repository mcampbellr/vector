import { cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useFileContent } from './useFileContent'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function stubFetch(impl: () => Promise<Response>) {
  vi.stubGlobal('fetch', vi.fn(impl))
}

describe('useFileContent error kinds', () => {
  it('classifies a 404 as not-found', async () => {
    stubFetch(async () => new Response(JSON.stringify({ error: 'artifact "spec" for spec "a" not found' }), { status: 404 }))
    const { result } = renderHook(() => useFileContent('a', 'spec'))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.errorKind).toBe('not-found')
    expect(result.current.error).toBe('artifact "spec" for spec "a" not found')
  })

  it('classifies a rejected fetch as unreachable', async () => {
    stubFetch(() => Promise.reject(new TypeError('Failed to fetch')))
    const { result } = renderHook(() => useFileContent('a', 'spec'))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.errorKind).toBe('unreachable')
  })

  it('classifies other HTTP errors as failed', async () => {
    stubFetch(async () => new Response(JSON.stringify({ error: 'could not read artifact' }), { status: 500 }))
    const { result } = renderHook(() => useFileContent('a', 'spec'))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.errorKind).toBe('failed')
  })

  it('clears the error kind on success', async () => {
    stubFetch(async () => new Response('# Spec', { status: 200 }))
    const { result } = renderHook(() => useFileContent('a', 'spec'))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toBe('# Spec')
    expect(result.current.errorKind).toBeNull()
  })
})
