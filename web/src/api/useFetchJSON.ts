import { useCallback, useEffect, useState } from 'react'

export interface AsyncState<T> {
  data: T | null
  loading: boolean
  error: string | null
  reload: () => void
}

// parseError pulls the {error} field the local API returns on 4xx/5xx, falling
// back to the status text.
async function parseError(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string }
    if (body.error) return body.error
  } catch {
    /* non-JSON body — fall through */
  }
  return `request failed (${res.status})`
}

// useFetchJSON fetches JSON from a URL, or stays idle when url is null (so a
// caller can keep the request lazy — e.g. a drawer that only fetches on open).
// It is the shared GET helper behind the standup, activity, and summary hooks.
export function useFetchJSON<T>(url: string | null): AsyncState<T> {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState<boolean>(url !== null)
  const [error, setError] = useState<string | null>(null)
  const [nonce, setNonce] = useState(0)

  const reload = useCallback(() => setNonce((n) => n + 1), [])

  useEffect(() => {
    if (url === null) {
      setData(null)
      setLoading(false)
      setError(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    fetch(url)
      .then(async (res) => {
        if (!res.ok) throw new Error(await parseError(res))
        return (await res.json()) as T
      })
      .then((json) => {
        if (!cancelled) {
          setData(json)
          setLoading(false)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'request failed')
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [url, nonce])

  return { data, loading, error, reload }
}
