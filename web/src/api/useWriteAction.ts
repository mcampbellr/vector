import { useCallback, useEffect, useRef, useState } from 'react'

export interface WriteAction<Args extends unknown[], Result> {
  /** Runs the write; resolves to the result, or null when it failed (the
   *  message is then in `error`). Concurrent calls are ignored while pending. */
  run: (...args: Args) => Promise<Result | null>
  pending: boolean
  error: string | null
  clearError: () => void
}

// useWriteAction wraps one board write with the UI state every write control
// needs: `pending` to disable the control while the request is in flight, and
// `error` to show the server's message inline. It never touches board data —
// the result reaches the screen through the SSE board push.
export function useWriteAction<Args extends unknown[], Result>(
  action: (...args: Args) => Promise<Result>,
): WriteAction<Args, Result> {
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const pendingRef = useRef(false)
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  const run = useCallback(
    async (...args: Args): Promise<Result | null> => {
      if (pendingRef.current) return null
      pendingRef.current = true
      setPending(true)
      setError(null)
      try {
        return await action(...args)
      } catch (caught: unknown) {
        if (mountedRef.current) setError(caught instanceof Error ? caught.message : 'request failed')
        return null
      } finally {
        pendingRef.current = false
        if (mountedRef.current) setPending(false)
      }
    },
    [action],
  )

  const clearError = useCallback(() => setError(null), [])

  return { run, pending, error, clearError }
}
